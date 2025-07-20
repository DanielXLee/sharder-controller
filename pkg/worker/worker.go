package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	shardclient "github.com/k8s-shard-controller/pkg/client"
	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// WorkerShard implements the WorkerShard interface
type WorkerShard struct {
	config     *config.Config
	shardId    string
	client     client.Client
	kubeClient kubernetes.Interface
	logger     logr.Logger

	// Resource management
	mu                sync.RWMutex
	assignedResources map[string]*interfaces.Resource
	resourceQueue     chan *interfaces.Resource

	// State management
	running      bool
	draining     bool
	stopCh       chan struct{}
	doneCh       chan struct{}
	healthTicker *time.Ticker

	// Metrics
	loadMetrics *shardv1.LoadMetrics

	// Communication channels for inter-shard communication
	migrationCh chan *migrationRequest
}

// migrationRequest represents a resource migration request
type migrationRequest struct {
	resources   []*interfaces.Resource
	targetShard string
	responseCh  chan error
}

// NewWorkerShard creates a new WorkerShard
func NewWorkerShard(cfg *config.Config, shardId string) (*WorkerShard, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if shardId == "" {
		return nil, fmt.Errorf("shardId cannot be empty")
	}

	// Create client set
	clientSet, err := shardclient.NewClientSet(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client set: %w", err)
	}

	logger := log.Log.WithName("worker-shard").WithValues("shardId", shardId)

	return &WorkerShard{
		config:            cfg,
		shardId:           shardId,
		client:            clientSet.Client,
		kubeClient:        clientSet.KubeClient,
		logger:            logger,
		assignedResources: make(map[string]*interfaces.Resource),
		resourceQueue:     make(chan *interfaces.Resource, 1000), // Buffered channel for resource processing
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		migrationCh:       make(chan *migrationRequest, 100),
		loadMetrics: &shardv1.LoadMetrics{
			ResourceCount:  0,
			CPUUsage:       0.0,
			MemoryUsage:    0.0,
			ProcessingRate: 0.0,
			QueueLength:    0,
		},
	}, nil
}

// Start starts the worker shard
func (ws *WorkerShard) Start(ctx context.Context) error {
	ws.mu.Lock()
	if ws.running {
		ws.mu.Unlock()
		return fmt.Errorf("worker shard is already running")
	}
	ws.running = true
	ws.mu.Unlock()

	ws.logger.Info("Starting worker shard")

	// Initialize shard instance in Kubernetes
	if err := ws.initializeShardInstance(ctx); err != nil {
		return fmt.Errorf("failed to initialize shard instance: %w", err)
	}

	// Start health reporting
	ws.healthTicker = time.NewTicker(ws.config.HealthCheck.Interval)
	go ws.healthReportLoop(ctx)

	// Start resource processing loop
	go ws.resourceProcessingLoop(ctx)

	// Start migration handling loop
	go ws.migrationHandlingLoop(ctx)

	// Start metrics collection loop
	go ws.metricsCollectionLoop(ctx)

	ws.logger.Info("Worker shard started successfully")
	return nil
}

// Stop stops the worker shard gracefully
func (ws *WorkerShard) Stop() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !ws.running {
		return nil
	}

	ws.logger.Info("Stopping worker shard")

	// Stop health ticker
	if ws.healthTicker != nil {
		ws.healthTicker.Stop()
	}

	close(ws.stopCh)
	ws.running = false

	// Wait for worker to stop
	select {
	case <-ws.doneCh:
		ws.logger.Info("Worker shard stopped successfully")
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for worker shard to stop")
	}
}

// ProcessResource processes a resource (implements WorkerShard interface)
func (ws *WorkerShard) ProcessResource(ctx context.Context, resource *interfaces.Resource) error {
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}

	ws.mu.Lock()
	if !ws.running || ws.draining {
		ws.mu.Unlock()
		return fmt.Errorf("worker shard is not accepting new resources")
	}
	ws.mu.Unlock()

	// Add resource to processing queue
	select {
	case ws.resourceQueue <- resource:
		ws.logger.V(1).Info("Resource queued for processing", "resourceId", resource.ID)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("resource queue is full")
	}
}

// GetAssignedResources returns all assigned resources (implements WorkerShard interface)
func (ws *WorkerShard) GetAssignedResources(ctx context.Context) ([]*interfaces.Resource, error) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	resources := make([]*interfaces.Resource, 0, len(ws.assignedResources))
	for _, resource := range ws.assignedResources {
		resources = append(resources, resource)
	}

	return resources, nil
}

// ReportHealth reports the health status (implements WorkerShard interface)
func (ws *WorkerShard) ReportHealth(ctx context.Context) (*shardv1.HealthStatus, error) {
	ws.mu.RLock()
	running := ws.running
	draining := ws.draining
	resourceCount := len(ws.assignedResources)
	ws.mu.RUnlock()

	healthy := running && !draining
	message := "Shard is healthy"
	if draining {
		message = "Shard is draining"
	} else if !running {
		message = "Shard is not running"
	}

	return &shardv1.HealthStatus{
		Healthy:    healthy,
		LastCheck:  metav1.Now(),
		ErrorCount: 0,
		Message:    fmt.Sprintf("%s (resources: %d)", message, resourceCount),
	}, nil
}

// GetLoadMetrics returns current load metrics (implements WorkerShard interface)
func (ws *WorkerShard) GetLoadMetrics(ctx context.Context) (*shardv1.LoadMetrics, error) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	// Create a copy of current metrics
	metrics := &shardv1.LoadMetrics{
		ResourceCount:  ws.loadMetrics.ResourceCount,
		CPUUsage:       ws.loadMetrics.CPUUsage,
		MemoryUsage:    ws.loadMetrics.MemoryUsage,
		ProcessingRate: ws.loadMetrics.ProcessingRate,
		QueueLength:    ws.loadMetrics.QueueLength,
	}

	return metrics, nil
}

// MigrateResourcesTo migrates resources to another shard (implements WorkerShard interface)
func (ws *WorkerShard) MigrateResourcesTo(ctx context.Context, targetShard string, resources []*interfaces.Resource) error {
	if targetShard == "" {
		return fmt.Errorf("target shard cannot be empty")
	}
	if len(resources) == 0 {
		return nil // Nothing to migrate
	}

	ws.logger.Info("Starting resource migration", "targetShard", targetShard, "resourceCount", len(resources))

	// Create migration request
	req := &migrationRequest{
		resources:   resources,
		targetShard: targetShard,
		responseCh:  make(chan error, 1),
	}

	// Send migration request
	select {
	case ws.migrationCh <- req:
		// Wait for response
		select {
		case err := <-req.responseCh:
			if err != nil {
				ws.logger.Error(err, "Resource migration failed", "targetShard", targetShard)
				return err
			}
			ws.logger.Info("Resource migration completed successfully", "targetShard", targetShard)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AcceptMigratedResources accepts migrated resources from another shard (implements WorkerShard interface)
func (ws *WorkerShard) AcceptMigratedResources(ctx context.Context, resources []*interfaces.Resource) error {
	if len(resources) == 0 {
		return nil
	}

	ws.mu.Lock()
	if !ws.running || ws.draining {
		ws.mu.Unlock()
		return fmt.Errorf("worker shard is not accepting migrated resources")
	}

	ws.logger.Info("Accepting migrated resources", "resourceCount", len(resources))

	// Add resources to assigned resources
	for _, resource := range resources {
		ws.assignedResources[resource.ID] = resource
		ws.logger.V(1).Info("Accepted migrated resource", "resourceId", resource.ID)
	}

	// Update metrics
	ws.loadMetrics.ResourceCount = len(ws.assignedResources)
	ws.mu.Unlock()

	// Update shard instance status (without holding the mutex)
	if err := ws.updateShardStatus(ctx); err != nil {
		ws.logger.Error(err, "Failed to update shard status after accepting migrated resources")
	}

	return nil
}

// Drain starts the draining process (implements WorkerShard interface)
func (ws *WorkerShard) Drain(ctx context.Context) error {
	ws.mu.Lock()
	if ws.draining {
		ws.mu.Unlock()
		return fmt.Errorf("shard is already draining")
	}
	ws.draining = true
	ws.mu.Unlock()

	ws.logger.Info("Starting shard drain process")

	// Update shard phase to draining
	if err := ws.updateShardPhase(ctx, shardv1.ShardPhaseDraining); err != nil {
		ws.logger.Error(err, "Failed to update shard phase to draining")
	}

	// Stop accepting new resources and wait for current processing to complete
	// The actual resource migration will be handled by the shard manager

	return nil
}

// Shutdown performs graceful shutdown (implements WorkerShard interface)
func (ws *WorkerShard) Shutdown(ctx context.Context) error {
	ws.logger.Info("Starting graceful shutdown")

	// First drain the shard
	if err := ws.Drain(ctx); err != nil {
		ws.logger.Error(err, "Failed to drain shard during shutdown")
	}

	// Wait for all resources to be migrated or timeout
	timeout := ws.config.DefaultShardConfig.GracefulShutdownTimeout
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-shutdownCtx.Done():
			ws.logger.Info("Graceful shutdown timeout, forcing shutdown")
			return ws.Stop()
		case <-ticker.C:
			ws.mu.RLock()
			resourceCount := len(ws.assignedResources)
			ws.mu.RUnlock()

			if resourceCount == 0 {
				ws.logger.Info("All resources migrated, proceeding with shutdown")
				return ws.Stop()
			}
			ws.logger.V(1).Info("Waiting for resource migration to complete", "remainingResources", resourceCount)
		}
	}
}

// GetShardID returns the shard ID (implements WorkerShard interface)
func (ws *WorkerShard) GetShardID() string {
	return ws.shardId
}

// GetHashRange returns the hash range for this shard (implements WorkerShard interface)
func (ws *WorkerShard) GetHashRange() *shardv1.HashRange {
	// Get hash range from shard instance
	ctx := context.Background()
	shard := &shardv1.ShardInstance{}
	if err := ws.client.Get(ctx, client.ObjectKey{Name: ws.shardId, Namespace: ws.config.Namespace}, shard); err != nil {
		ws.logger.Error(err, "Failed to get shard instance for hash range")
		return nil
	}
	return shard.Spec.HashRange
}

// Private methods

// initializeShardInstance initializes the shard instance in Kubernetes
func (ws *WorkerShard) initializeShardInstance(ctx context.Context) error {
	shard := &shardv1.ShardInstance{}
	err := ws.client.Get(ctx, client.ObjectKey{Name: ws.shardId, Namespace: ws.config.Namespace}, shard)
	
	if err != nil {
		// If shard instance doesn't exist, create a basic one
		ws.logger.Info("ShardInstance not found, creating a basic one", "shardId", ws.shardId)
		
		// Create a basic shard instance
		shard = &shardv1.ShardInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ws.shardId,
				Namespace: ws.config.Namespace,
				Labels: map[string]string{
					"app":       "shard-worker",
					"component": "worker",
					"shard-id":  ws.shardId,
				},
			},
			Spec: shardv1.ShardInstanceSpec{
				ShardID: ws.shardId,
				HashRange: &shardv1.HashRange{
					Start: 0,
					End:   1000, // Default range, will be adjusted by manager
				},
				Resources: []string{}, // Basic resources list
			},
			Status: shardv1.ShardInstanceStatus{
				Phase:             shardv1.ShardPhasePending,
				AssignedResources: []string{},
				LastHeartbeat:     metav1.Now(),
			},
		}
		
		if err := ws.client.Create(ctx, shard); err != nil {
			return fmt.Errorf("failed to create shard instance: %w", err)
		}
		
		ws.logger.Info("Created basic ShardInstance", "shardId", ws.shardId)
		
		// Re-fetch the created shard to get the latest version
		if err := ws.client.Get(ctx, client.ObjectKey{Name: ws.shardId, Namespace: ws.config.Namespace}, shard); err != nil {
			return fmt.Errorf("failed to re-fetch created shard instance: %w", err)
		}
	}

	// Don't immediately transition to running - let the heartbeat loop handle it
	// This avoids potential race conditions during initialization
	ws.logger.Info("ShardInstance initialized, will transition to running in heartbeat loop")

	return nil
}

// healthReportLoop periodically reports health status
func (ws *WorkerShard) healthReportLoop(ctx context.Context) {
	defer close(ws.doneCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ws.stopCh:
			return
		case <-ws.healthTicker.C:
			if err := ws.updateHeartbeat(ctx); err != nil {
				ws.logger.Error(err, "Failed to update heartbeat")
			}
		}
	}
}

// resourceProcessingLoop processes resources from the queue
func (ws *WorkerShard) resourceProcessingLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ws.stopCh:
			return
		case resource := <-ws.resourceQueue:
			if err := ws.processResourceInternal(ctx, resource); err != nil {
				ws.logger.Error(err, "Failed to process resource", "resourceId", resource.ID)
			}
		}
	}
}

// migrationHandlingLoop handles resource migration requests
func (ws *WorkerShard) migrationHandlingLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ws.stopCh:
			return
		case req := <-ws.migrationCh:
			err := ws.handleMigrationRequest(ctx, req)
			req.responseCh <- err
		}
	}
}

// metricsCollectionLoop periodically collects and updates metrics
func (ws *WorkerShard) metricsCollectionLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Collect metrics every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ws.stopCh:
			return
		case <-ticker.C:
			ws.collectMetrics()
		}
	}
}

// processResourceInternal processes a single resource
func (ws *WorkerShard) processResourceInternal(ctx context.Context, resource *interfaces.Resource) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Add to assigned resources
	ws.assignedResources[resource.ID] = resource
	ws.logger.V(1).Info("Resource processed and assigned", "resourceId", resource.ID)

	// Update metrics
	ws.loadMetrics.ResourceCount = len(ws.assignedResources)
	ws.loadMetrics.QueueLength = len(ws.resourceQueue)

	return nil
}

// handleMigrationRequest handles a resource migration request
func (ws *WorkerShard) handleMigrationRequest(ctx context.Context, req *migrationRequest) error {
	ws.mu.Lock()
	// Remove resources from assigned resources
	for _, resource := range req.resources {
		delete(ws.assignedResources, resource.ID)
		ws.logger.V(1).Info("Resource migrated out", "resourceId", resource.ID, "targetShard", req.targetShard)
	}

	// Update metrics
	ws.loadMetrics.ResourceCount = len(ws.assignedResources)
	ws.mu.Unlock()

	// Update shard status (without holding the mutex)
	if err := ws.updateShardStatus(ctx); err != nil {
		ws.logger.Error(err, "Failed to update shard status after migration")
		// Don't return error to avoid breaking the migration process
	}

	return nil
}

// collectMetrics collects current metrics
func (ws *WorkerShard) collectMetrics() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Update basic metrics
	ws.loadMetrics.ResourceCount = len(ws.assignedResources)
	ws.loadMetrics.QueueLength = len(ws.resourceQueue)

	// TODO: Collect actual CPU and memory usage from system
	// For now, simulate based on resource count
	ws.loadMetrics.CPUUsage = float64(ws.loadMetrics.ResourceCount) / 1000.0
	if ws.loadMetrics.CPUUsage > 1.0 {
		ws.loadMetrics.CPUUsage = 1.0
	}

	ws.loadMetrics.MemoryUsage = float64(ws.loadMetrics.ResourceCount) / 800.0
	if ws.loadMetrics.MemoryUsage > 1.0 {
		ws.loadMetrics.MemoryUsage = 1.0
	}

	// Calculate processing rate (resources per second)
	ws.loadMetrics.ProcessingRate = float64(ws.loadMetrics.ResourceCount) / 60.0
}

// updateHeartbeat updates the heartbeat of the shard
func (ws *WorkerShard) updateHeartbeat(ctx context.Context) error {
	shard := &shardv1.ShardInstance{}
	if err := ws.client.Get(ctx, client.ObjectKey{Name: ws.shardId, Namespace: ws.config.Namespace}, shard); err != nil {
		return fmt.Errorf("failed to get shard instance: %w", err)
	}

	// Update heartbeat and health status
	shard.UpdateHeartbeat()

	// Update health status
	healthStatus, err := ws.ReportHealth(ctx)
	if err != nil {
		return fmt.Errorf("failed to get health status: %w", err)
	}
	shard.Status.HealthStatus = healthStatus

	// Update load
	ws.mu.RLock()
	load := ws.loadMetrics.CalculateOverallLoad()
	ws.mu.RUnlock()
	shard.UpdateLoad(load)

	if err := ws.client.Status().Update(ctx, shard); err != nil {
		return fmt.Errorf("failed to update shard status: %w", err)
	}

	return nil
}

// updateShardStatus updates the shard status in Kubernetes
// Note: This method should be called without holding the mutex
func (ws *WorkerShard) updateShardStatus(ctx context.Context) error {
	shard := &shardv1.ShardInstance{}
	if err := ws.client.Get(ctx, client.ObjectKey{Name: ws.shardId, Namespace: ws.config.Namespace}, shard); err != nil {
		return fmt.Errorf("failed to get shard instance: %w", err)
	}

	// Update assigned resources
	ws.mu.RLock()
	resourceIds := make([]string, 0, len(ws.assignedResources))
	for id := range ws.assignedResources {
		resourceIds = append(resourceIds, id)
	}
	ws.mu.RUnlock()

	shard.Status.AssignedResources = resourceIds

	if err := ws.client.Status().Update(ctx, shard); err != nil {
		return fmt.Errorf("failed to update shard status: %w", err)
	}

	return nil
}

// updateShardPhase updates the shard phase
func (ws *WorkerShard) updateShardPhase(ctx context.Context, phase shardv1.ShardPhase) error {
	shard := &shardv1.ShardInstance{}
	if err := ws.client.Get(ctx, client.ObjectKey{Name: ws.shardId, Namespace: ws.config.Namespace}, shard); err != nil {
		return fmt.Errorf("failed to get shard instance: %w", err)
	}

	if err := shard.TransitionTo(phase); err != nil {
		return fmt.Errorf("failed to transition shard phase: %w", err)
	}

	if err := ws.client.Status().Update(ctx, shard); err != nil {
		return fmt.Errorf("failed to update shard phase: %w", err)
	}

	return nil
}
