package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// ShardManager implements the ShardManager interface with Leader Election
type ShardManager struct {
	client          client.Client
	kubeClient      kubernetes.Interface
	loadBalancer    interfaces.LoadBalancer
	healthChecker   interfaces.HealthChecker
	resourceMigrator interfaces.ResourceMigrator
	configManager   interfaces.ConfigManager
	metricsCollector interfaces.MetricsCollector
	alertManager    interfaces.AlertManager
	structuredLogger interfaces.StructuredLogger
	config          *config.Config
	
	// Leader election
	leaderElector   *leaderelection.LeaderElector
	isLeader        bool
	leaderMu        sync.RWMutex
	
	// State management
	mu             sync.RWMutex
	shards         map[string]*shardv1.ShardInstance
	running        bool
	stopCh         chan struct{}
	
	// Monitoring and decision making
	lastScaleDecision time.Time
	scaleDecisionMu   sync.RWMutex
}

// NewShardManager creates a new ShardManager with Leader Election
func NewShardManager(
	client client.Client,
	kubeClient kubernetes.Interface,
	loadBalancer interfaces.LoadBalancer,
	healthChecker interfaces.HealthChecker,
	resourceMigrator interfaces.ResourceMigrator,
	configManager interfaces.ConfigManager,
	metricsCollector interfaces.MetricsCollector,
	alertManager interfaces.AlertManager,
	structuredLogger interfaces.StructuredLogger,
	cfg *config.Config,
) (*ShardManager, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	if kubeClient == nil {
		return nil, fmt.Errorf("kubeClient cannot be nil")
	}
	if loadBalancer == nil {
		return nil, fmt.Errorf("loadBalancer cannot be nil")
	}
	if healthChecker == nil {
		return nil, fmt.Errorf("healthChecker cannot be nil")
	}
	// resourceMigrator can be nil initially and set later
	if configManager == nil {
		return nil, fmt.Errorf("configManager cannot be nil")
	}
	if metricsCollector == nil {
		return nil, fmt.Errorf("metricsCollector cannot be nil")
	}
	if alertManager == nil {
		return nil, fmt.Errorf("alertManager cannot be nil")
	}
	if structuredLogger == nil {
		return nil, fmt.Errorf("structuredLogger cannot be nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	sm := &ShardManager{
		client:          client,
		kubeClient:      kubeClient,
		loadBalancer:    loadBalancer,
		healthChecker:   healthChecker,
		resourceMigrator: resourceMigrator,
		configManager:   configManager,
		metricsCollector: metricsCollector,
		alertManager:    alertManager,
		structuredLogger: structuredLogger,
		config:          cfg,
		shards:          make(map[string]*shardv1.ShardInstance),
		stopCh:          make(chan struct{}),
	}

	// Initialize leader election
	if err := sm.initializeLeaderElection(); err != nil {
		return nil, fmt.Errorf("failed to initialize leader election: %w", err)
	}

	return sm, nil
}

// SetResourceMigrator sets the resource migrator (used to resolve circular dependency)
func (sm *ShardManager) SetResourceMigrator(resourceMigrator interfaces.ResourceMigrator) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.resourceMigrator = resourceMigrator
}

// initializeLeaderElection sets up leader election for the shard manager
func (sm *ShardManager) initializeLeaderElection() error {
	// Create resource lock for leader election
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "shard-manager-leader",
			Namespace: sm.config.Namespace,
		},
		Client: sm.kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: sm.config.NodeName,
		},
	}

	// Create leader election config
	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   30 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: sm.onStartedLeading,
			OnStoppedLeading: sm.onStoppedLeading,
			OnNewLeader:      sm.onNewLeader,
		},
	}

	// Create leader elector
	leaderElector, err := leaderelection.NewLeaderElector(leaderElectionConfig)
	if err != nil {
		return fmt.Errorf("failed to create leader elector: %w", err)
	}

	sm.leaderElector = leaderElector
	return nil
}

// Start starts the shard manager with leader election
func (sm *ShardManager) Start(ctx context.Context) error {
	sm.mu.Lock()
	if sm.running {
		sm.mu.Unlock()
		return fmt.Errorf("shard manager is already running")
	}
	sm.running = true
	sm.mu.Unlock()

	logger := log.FromContext(ctx)
	logger.Info("Starting Shard Manager with Leader Election")

	// Start leader election
	go sm.leaderElector.Run(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	return sm.Stop()
}

// Stop stops the shard manager
func (sm *ShardManager) Stop() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.running {
		return nil
	}

	close(sm.stopCh)
	sm.running = false
	return nil
}

// IsLeader returns whether this instance is the leader
func (sm *ShardManager) IsLeader() bool {
	sm.leaderMu.RLock()
	defer sm.leaderMu.RUnlock()
	return sm.isLeader
}

// onStartedLeading is called when this instance becomes the leader
func (sm *ShardManager) onStartedLeading(ctx context.Context) {
	sm.leaderMu.Lock()
	sm.isLeader = true
	sm.leaderMu.Unlock()

	logger := log.FromContext(ctx)
	logger.Info("Became leader, starting shard management operations")

	// Start management loops
	go sm.runScalingDecisionLoop(ctx)
	go sm.runMonitoringLoop(ctx)
	go sm.runHealthCheckLoop(ctx)
}

// onStoppedLeading is called when this instance stops being the leader
func (sm *ShardManager) onStoppedLeading() {
	sm.leaderMu.Lock()
	sm.isLeader = false
	sm.leaderMu.Unlock()

	logger := log.Log
	logger.Info("Stopped being leader, stopping shard management operations")
}

// onNewLeader is called when a new leader is elected
func (sm *ShardManager) onNewLeader(identity string) {
	logger := log.Log
	logger.Info("New leader elected", "leader", identity)
}

// CreateShard creates a new shard (delegates to enhanced version)
func (sm *ShardManager) CreateShard(ctx context.Context, config *shardv1.ShardConfig) (*shardv1.ShardInstance, error) {
	return sm.CreateShardWithLifecycle(ctx, config)
}

// DeleteShard deletes a shard
func (sm *ShardManager) DeleteShard(ctx context.Context, shardId string) error {
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shardId,
			Namespace: sm.config.Namespace,
		},
	}

	return sm.client.Delete(ctx, shard)
}

// ScaleUp scales up the number of shards
func (sm *ShardManager) ScaleUp(ctx context.Context, targetCount int) error {
	shards, err := sm.ListShards(ctx)
	if err != nil {
		return err
	}

	currentCount := len(shards)
	if currentCount >= targetCount {
		return nil // Already have enough shards
	}

	for i := currentCount; i < targetCount; i++ {
		config, err := sm.configManager.LoadConfig(ctx)
		if err != nil {
			return err
		}
		if _, err := sm.CreateShard(ctx, config); err != nil {
			return err
		}
	}

	return nil
}

// ScaleDown scales down the number of shards
func (sm *ShardManager) ScaleDown(ctx context.Context, targetCount int) error {
	shards, err := sm.ListShards(ctx)
	if err != nil {
		return err
	}

	currentCount := len(shards)
	if currentCount <= targetCount {
		return nil // Already have few enough shards
	}

	// For simplicity, we'll just delete the last shards
	// A more sophisticated implementation would choose which shards to delete based on load
	for i := currentCount; i > targetCount; i-- {
		shardToDelete := shards[i-1]
		if err := sm.DeleteShard(ctx, shardToDelete.Name); err != nil {
			return err
		}
	}

	return nil
}

// RebalanceLoad rebalances the load across shards
func (sm *ShardManager) RebalanceLoad(ctx context.Context) error {
	shards, err := sm.ListShards(ctx)
	if err != nil {
		return err
	}

	if sm.loadBalancer.ShouldRebalance(shards) {
		plan, err := sm.loadBalancer.GenerateRebalancePlan(shards)
		if err != nil {
			return err
		}

		return sm.resourceMigrator.ExecuteMigration(ctx, plan)
	}

	return nil
}

// AssignResource assigns a resource to a shard
func (sm *ShardManager) AssignResource(ctx context.Context, resource *interfaces.Resource) (string, error) {
	shards, err := sm.ListShards(ctx)
	if err != nil {
		return "", err
	}

	shard, err := sm.loadBalancer.AssignResourceToShard(resource, shards)
	if err != nil {
		return "", err
	}

	return shard.Spec.ShardID, nil
}

// CheckShardHealth checks the health of a shard
func (sm *ShardManager) CheckShardHealth(ctx context.Context, shardId string) (*shardv1.HealthStatus, error) {
	shard := &shardv1.ShardInstance{}
	if err := sm.client.Get(ctx, client.ObjectKey{Name: shardId, Namespace: sm.config.Namespace}, shard); err != nil {
		return nil, err
	}

	return sm.healthChecker.CheckHealth(ctx, shard)
}

// HandleFailedShard handles a failed shard
func (sm *ShardManager) HandleFailedShard(ctx context.Context, shardId string) error {
	return sm.healthChecker.OnShardFailed(ctx, shardId)
}

// GetShardStatus gets the status of a shard
func (sm *ShardManager) GetShardStatus(ctx context.Context, shardId string) (*shardv1.ShardInstanceStatus, error) {
	shard := &shardv1.ShardInstance{}
	if err := sm.client.Get(ctx, client.ObjectKey{Name: shardId, Namespace: sm.config.Namespace}, shard); err != nil {
		return nil, err
	}

	return &shard.Status, nil
}

// ListShards lists all shards
func (sm *ShardManager) ListShards(ctx context.Context) ([]*shardv1.ShardInstance, error) {
	shardList := &shardv1.ShardInstanceList{}
	if err := sm.client.List(ctx, shardList, client.InNamespace(sm.config.Namespace)); err != nil {
		return nil, err
	}

	shards := make([]*shardv1.ShardInstance, len(shardList.Items))
	for i := range shardList.Items {
		shards[i] = &shardList.Items[i]
	}

	return shards, nil
}

// runScalingDecisionLoop runs the scaling decision loop
func (sm *ShardManager) runScalingDecisionLoop(ctx context.Context) {
	logger := log.FromContext(ctx)
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sm.stopCh:
			return
		case <-ticker.C:
			if !sm.IsLeader() {
				continue
			}

			if err := sm.makeScalingDecision(ctx); err != nil {
				logger.Error(err, "Failed to make scaling decision")
			}
		}
	}
}

// runMonitoringLoop runs the monitoring loop
func (sm *ShardManager) runMonitoringLoop(ctx context.Context) {
	logger := log.FromContext(ctx)
	ticker := time.NewTicker(10 * time.Second) // Monitor every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sm.stopCh:
			return
		case <-ticker.C:
			if !sm.IsLeader() {
				continue
			}

			if err := sm.monitorShards(ctx); err != nil {
				logger.Error(err, "Failed to monitor shards")
			}
		}
	}
}

// runHealthCheckLoop runs the health check loop
func (sm *ShardManager) runHealthCheckLoop(ctx context.Context) {
	logger := log.FromContext(ctx)
	ticker := time.NewTicker(15 * time.Second) // Health check every 15 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sm.stopCh:
			return
		case <-ticker.C:
			if !sm.IsLeader() {
				continue
			}

			if err := sm.performHealthChecks(ctx); err != nil {
				logger.Error(err, "Failed to perform health checks")
			}
		}
	}
}

// makeScalingDecision makes scaling decisions based on current load and configuration
func (sm *ShardManager) makeScalingDecision(ctx context.Context) error {
	logger := log.FromContext(ctx)
	
	// Prevent too frequent scaling decisions
	sm.scaleDecisionMu.Lock()
	if time.Since(sm.lastScaleDecision) < 2*time.Minute {
		sm.scaleDecisionMu.Unlock()
		return nil
	}
	sm.lastScaleDecision = time.Now()
	sm.scaleDecisionMu.Unlock()

	// Load current configuration
	config, err := sm.configManager.LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get current shards
	shards, err := sm.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}

	// Calculate current metrics
	healthyShards := sm.countHealthyShards(shards)
	totalLoad := sm.calculateTotalLoad(shards)
	currentCount := len(shards)

	logger.Info("Scaling decision metrics", 
		"currentShards", currentCount,
		"healthyShards", healthyShards,
		"totalLoad", totalLoad,
		"scaleUpThreshold", config.Spec.ScaleUpThreshold,
		"scaleDownThreshold", config.Spec.ScaleDownThreshold)

	// Make scaling decision
	if totalLoad > config.Spec.ScaleUpThreshold && currentCount < config.Spec.MaxShards {
		// Scale up
		targetCount := currentCount + 1
		logger.Info("Scaling up", "from", currentCount, "to", targetCount)
		
		// Log scale event
		sm.structuredLogger.LogScaleEvent(ctx, "scale_up", currentCount, targetCount, "high_load", "initiated")
		
		if err := sm.ScaleUp(ctx, targetCount); err != nil {
			// Log error and send alert
			sm.structuredLogger.LogErrorEvent(ctx, "shard_manager", "scale_up", err, map[string]interface{}{
				"from_count": currentCount,
				"to_count":   targetCount,
			})
			sm.metricsCollector.RecordError("shard_manager", "scale_up_failed")
			return fmt.Errorf("failed to scale up: %w", err)
		}
		
		// Record successful scale operation
		sm.metricsCollector.RecordScaleOperation("scale_up", "success")
		sm.structuredLogger.LogScaleEvent(ctx, "scale_up", currentCount, targetCount, "high_load", "success")
		
		// Send alert for scale event
		if err := sm.alertManager.AlertScaleEvent(ctx, "scale_up", currentCount, targetCount, "high_load"); err != nil {
			logger.Error(err, "Failed to send scale up alert")
		}
		
	} else if totalLoad < config.Spec.ScaleDownThreshold && currentCount > config.Spec.MinShards {
		// Scale down
		targetCount := currentCount - 1
		logger.Info("Scaling down", "from", currentCount, "to", targetCount)
		
		// Log scale event
		sm.structuredLogger.LogScaleEvent(ctx, "scale_down", currentCount, targetCount, "low_load", "initiated")
		
		if err := sm.ScaleDownGracefully(ctx, targetCount); err != nil {
			// Log error and send alert
			sm.structuredLogger.LogErrorEvent(ctx, "shard_manager", "scale_down", err, map[string]interface{}{
				"from_count": currentCount,
				"to_count":   targetCount,
			})
			sm.metricsCollector.RecordError("shard_manager", "scale_down_failed")
			return fmt.Errorf("failed to scale down: %w", err)
		}
		
		// Record successful scale operation
		sm.metricsCollector.RecordScaleOperation("scale_down", "success")
		sm.structuredLogger.LogScaleEvent(ctx, "scale_down", currentCount, targetCount, "low_load", "success")
		
		// Send alert for scale event
		if err := sm.alertManager.AlertScaleEvent(ctx, "scale_down", currentCount, targetCount, "low_load"); err != nil {
			logger.Error(err, "Failed to send scale down alert")
		}
	}

	return nil
}

// monitorShards monitors shard status and updates internal state
func (sm *ShardManager) monitorShards(ctx context.Context) error {
	shards, err := sm.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Update internal shard map
	sm.shards = make(map[string]*shardv1.ShardInstance)
	for _, shard := range shards {
		sm.shards[shard.Spec.ShardID] = shard
		
		// Collect metrics for each shard
		if err := sm.metricsCollector.CollectShardMetrics(shard); err != nil {
			log.Log.Error(err, "Failed to collect metrics for shard", "shardId", shard.Spec.ShardID)
		}
	}

	// Record overall metrics
	healthyCount := sm.countHealthyShards(shards)
	totalLoad := sm.calculateTotalLoad(shards)
	
	sm.metricsCollector.RecordCustomMetric("shard_total_count", float64(len(shards)), nil)
	sm.metricsCollector.RecordCustomMetric("shard_healthy_count", float64(healthyCount), nil)
	sm.metricsCollector.RecordCustomMetric("shard_total_load", totalLoad, nil)

	// Collect system metrics
	if err := sm.metricsCollector.CollectSystemMetrics(shards); err != nil {
		log.Log.Error(err, "Failed to collect system metrics")
	}

	// Check for system overload
	if err := sm.checkSystemOverload(ctx, shards, totalLoad); err != nil {
		log.Log.Error(err, "Failed to check system overload")
	}

	return nil
}

// performHealthChecks performs health checks on all shards
func (sm *ShardManager) performHealthChecks(ctx context.Context) error {
	shards, err := sm.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}

	for _, shard := range shards {
		if err := sm.checkAndUpdateShardHealth(ctx, shard); err != nil {
			log.Log.Error(err, "Failed to check shard health", "shardId", shard.Spec.ShardID)
		}
	}

	return nil
}

// checkAndUpdateShardHealth checks and updates the health status of a single shard
func (sm *ShardManager) checkAndUpdateShardHealth(ctx context.Context, shard *shardv1.ShardInstance) error {
	healthStatus, err := sm.healthChecker.CheckHealth(ctx, shard)
	if err != nil {
		sm.structuredLogger.LogErrorEvent(ctx, "health_checker", "check_health", err, map[string]interface{}{
			"shard_id": shard.Spec.ShardID,
		})
		sm.metricsCollector.RecordError("health_checker", "check_failed")
		return fmt.Errorf("failed to check health for shard %s: %w", shard.Spec.ShardID, err)
	}

	// Log health event
	sm.structuredLogger.LogHealthEvent(ctx, shard.Spec.ShardID, healthStatus.Healthy, 0, healthStatus.Message)

	// Update shard status if health changed
	if shard.Status.HealthStatus == nil || 
		shard.Status.HealthStatus.Healthy != healthStatus.Healthy {
		
		previousHealthy := shard.Status.HealthStatus != nil && shard.Status.HealthStatus.Healthy
		shard.Status.HealthStatus = healthStatus
		
		// Update phase based on health
		if !healthStatus.Healthy && shard.Status.Phase == shardv1.ShardPhaseRunning {
			if err := shard.TransitionTo(shardv1.ShardPhaseFailed); err != nil {
				return fmt.Errorf("failed to transition shard to failed state: %w", err)
			}
			
			// Log shard failure event
			sm.structuredLogger.LogShardEvent(ctx, "shard_failed", shard.Spec.ShardID, map[string]interface{}{
				"reason": healthStatus.Message,
				"phase_transition": "running_to_failed",
			})
			
			// Send alert for shard failure
			if err := sm.alertManager.AlertShardFailure(ctx, shard.Spec.ShardID, healthStatus.Message); err != nil {
				log.Log.Error(err, "Failed to send shard failure alert", "shardId", shard.Spec.ShardID)
			}
			
			// Handle failed shard
			if err := sm.HandleFailedShard(ctx, shard.Spec.ShardID); err != nil {
				sm.structuredLogger.LogErrorEvent(ctx, "shard_manager", "handle_failed_shard", err, map[string]interface{}{
					"shard_id": shard.Spec.ShardID,
				})
				log.Log.Error(err, "Failed to handle failed shard", "shardId", shard.Spec.ShardID)
			}
		} else if healthStatus.Healthy && shard.Status.Phase == shardv1.ShardPhaseFailed {
			if err := shard.TransitionTo(shardv1.ShardPhaseRunning); err != nil {
				return fmt.Errorf("failed to transition shard to running state: %w", err)
			}
			
			// Log shard recovery event
			sm.structuredLogger.LogShardEvent(ctx, "shard_recovered", shard.Spec.ShardID, map[string]interface{}{
				"phase_transition": "failed_to_running",
				"was_healthy": previousHealthy,
			})
		}
		
		// Update the shard in Kubernetes
		if err := sm.client.Status().Update(ctx, shard); err != nil {
			sm.structuredLogger.LogErrorEvent(ctx, "shard_manager", "update_shard_status", err, map[string]interface{}{
				"shard_id": shard.Spec.ShardID,
			})
			return fmt.Errorf("failed to update shard status: %w", err)
		}
	}

	return nil
}

// ScaleDownGracefully scales down shards gracefully with resource migration
func (sm *ShardManager) ScaleDownGracefully(ctx context.Context, targetCount int) error {
	shards, err := sm.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}

	currentCount := len(shards)
	if currentCount <= targetCount {
		return nil // Already at target or below
	}

	// Select shards to remove (prefer least loaded, unhealthy shards)
	shardsToRemove := sm.selectShardsForRemoval(shards, currentCount-targetCount)

	for _, shard := range shardsToRemove {
		if err := sm.drainAndRemoveShard(ctx, shard); err != nil {
			return fmt.Errorf("failed to drain and remove shard %s: %w", shard.Spec.ShardID, err)
		}
	}

	return nil
}

// drainAndRemoveShard drains a shard and removes it gracefully
func (sm *ShardManager) drainAndRemoveShard(ctx context.Context, shard *shardv1.ShardInstance) error {
	logger := log.FromContext(ctx)
	logger.Info("Draining shard", "shardId", shard.Spec.ShardID)

	// Transition to draining state
	if err := shard.TransitionTo(shardv1.ShardPhaseDraining); err != nil {
		return fmt.Errorf("failed to transition shard to draining: %w", err)
	}

	// Update shard status
	if err := sm.client.Status().Update(ctx, shard); err != nil {
		return fmt.Errorf("failed to update shard status: %w", err)
	}

	// Migrate resources if any
	if len(shard.Status.AssignedResources) > 0 {
		if err := sm.migrateShardResources(ctx, shard); err != nil {
			return fmt.Errorf("failed to migrate resources: %w", err)
		}
	}

	// Wait for graceful shutdown timeout
	time.Sleep(30 * time.Second) // TODO: Use configurable timeout

	// Transition to terminated and delete
	if err := shard.TransitionTo(shardv1.ShardPhaseTerminated); err != nil {
		return fmt.Errorf("failed to transition shard to terminated: %w", err)
	}

	// Delete the shard
	if err := sm.DeleteShard(ctx, shard.Name); err != nil {
		return fmt.Errorf("failed to delete shard: %w", err)
	}

	logger.Info("Successfully drained and removed shard", "shardId", shard.Spec.ShardID)
	return nil
}

// migrateShardResources migrates resources from a shard to other healthy shards
func (sm *ShardManager) migrateShardResources(ctx context.Context, sourceShard *shardv1.ShardInstance) error {
	if len(sourceShard.Status.AssignedResources) == 0 {
		return nil
	}

	// Get healthy target shards
	allShards, err := sm.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}

	targetShards := make([]*shardv1.ShardInstance, 0)
	for _, shard := range allShards {
		if shard.Spec.ShardID != sourceShard.Spec.ShardID && 
			shard.IsHealthy() && shard.IsActive() {
			targetShards = append(targetShards, shard)
		}
	}

	if len(targetShards) == 0 {
		return fmt.Errorf("no healthy target shards available for migration")
	}

	// Create migration plan
	resources := make([]*interfaces.Resource, len(sourceShard.Status.AssignedResources))
	for i, resourceId := range sourceShard.Status.AssignedResources {
		resources[i] = &interfaces.Resource{
			ID:   resourceId,
			Type: "generic", // TODO: Determine actual resource type
		}
	}

	// Distribute resources among target shards
	for i, resource := range resources {
		targetShard := targetShards[i%len(targetShards)]
		
		startTime := time.Now()
		
		plan, err := sm.resourceMigrator.CreateMigrationPlan(
			ctx, 
			sourceShard.Spec.ShardID, 
			targetShard.Spec.ShardID, 
			[]*interfaces.Resource{resource},
		)
		if err != nil {
			sm.structuredLogger.LogErrorEvent(ctx, "resource_migrator", "create_migration_plan", err, map[string]interface{}{
				"source_shard": sourceShard.Spec.ShardID,
				"target_shard": targetShard.Spec.ShardID,
				"resource_id":  resource.ID,
			})
			return fmt.Errorf("failed to create migration plan: %w", err)
		}

		if err := sm.resourceMigrator.ExecuteMigration(ctx, plan); err != nil {
			duration := time.Since(startTime)
			
			// Log migration failure
			sm.structuredLogger.LogMigrationEvent(ctx, sourceShard.Spec.ShardID, targetShard.Spec.ShardID, 1, "failed", duration)
			sm.metricsCollector.RecordMigration(sourceShard.Spec.ShardID, targetShard.Spec.ShardID, "failed", duration)
			
			// Send migration failure alert
			if err := sm.alertManager.AlertMigrationFailure(ctx, sourceShard.Spec.ShardID, targetShard.Spec.ShardID, 1, err.Error()); err != nil {
				log.Log.Error(err, "Failed to send migration failure alert")
			}
			
			return fmt.Errorf("failed to execute migration: %w", err)
		}
		
		// Record successful migration
		duration := time.Since(startTime)
		sm.structuredLogger.LogMigrationEvent(ctx, sourceShard.Spec.ShardID, targetShard.Spec.ShardID, 1, "success", duration)
		sm.metricsCollector.RecordMigration(sourceShard.Spec.ShardID, targetShard.Spec.ShardID, "success", duration)
	}

	return nil
}

// selectShardsForRemoval selects shards for removal during scale down
func (sm *ShardManager) selectShardsForRemoval(shards []*shardv1.ShardInstance, count int) []*shardv1.ShardInstance {
	if count >= len(shards) {
		return shards
	}

	// Sort shards by priority for removal (unhealthy first, then least loaded)
	sortedShards := make([]*shardv1.ShardInstance, len(shards))
	copy(sortedShards, shards)

	// Simple selection: prefer unhealthy shards, then least loaded
	selected := make([]*shardv1.ShardInstance, 0, count)
	
	// First, select unhealthy shards
	for _, shard := range sortedShards {
		if len(selected) >= count {
			break
		}
		if !shard.IsHealthy() {
			selected = append(selected, shard)
		}
	}

	// Then, select least loaded healthy shards
	for _, shard := range sortedShards {
		if len(selected) >= count {
			break
		}
		if shard.IsHealthy() {
			// Check if already selected
			alreadySelected := false
			for _, s := range selected {
				if s.Spec.ShardID == shard.Spec.ShardID {
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				selected = append(selected, shard)
			}
		}
	}

	return selected
}

// countHealthyShards counts the number of healthy shards
func (sm *ShardManager) countHealthyShards(shards []*shardv1.ShardInstance) int {
	count := 0
	for _, shard := range shards {
		if shard.IsHealthy() {
			count++
		}
	}
	return count
}

// calculateTotalLoad calculates the total load across all shards
func (sm *ShardManager) calculateTotalLoad(shards []*shardv1.ShardInstance) float64 {
	if len(shards) == 0 {
		return 0
	}

	totalLoad := 0.0
	for _, shard := range shards {
		totalLoad += shard.Status.Load
	}
	
	return totalLoad / float64(len(shards)) // Return average load
}

// Enhanced CreateShard with proper lifecycle management
func (sm *ShardManager) CreateShardWithLifecycle(ctx context.Context, config *shardv1.ShardConfig) (*shardv1.ShardInstance, error) {
	logger := log.FromContext(ctx)
	
	// Generate unique shard ID
	shardId := fmt.Sprintf("shard-%d", time.Now().UnixNano())
	
	logger.Info("Creating new shard", "shardId", shardId)

	// Calculate hash range for the new shard
	existingShards, err := sm.ListShards(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing shards: %w", err)
	}

	hashRange := sm.calculateHashRangeForNewShard(existingShards)

	// Create shard instance
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shardId,
			Namespace: sm.config.Namespace,
			Labels: map[string]string{
				"app":                          "shard-controller",
				"shard.io/shard-id":           shardId,
				"shard.io/managed-by":         "shard-manager",
			},
		},
		Spec: shardv1.ShardInstanceSpec{
			ShardID:   shardId,
			HashRange: hashRange,
		},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhasePending,
			LastHeartbeat: metav1.Now(),
			HealthStatus: &shardv1.HealthStatus{
				Healthy:   false,
				LastCheck: metav1.Now(),
				Message:   "Shard is starting up",
			},
		},
	}

	// Create the shard in Kubernetes
	if err := sm.client.Create(ctx, shard); err != nil {
		return nil, fmt.Errorf("failed to create shard: %w", err)
	}

	// Record metrics
	sm.metricsCollector.RecordCustomMetric("shard_created_total", 1, map[string]string{
		"shard_id": shardId,
	})

	logger.Info("Successfully created shard", "shardId", shardId)
	return shard, nil
}

// checkSystemOverload checks for system overload and sends alerts if necessary
func (sm *ShardManager) checkSystemOverload(ctx context.Context, shards []*shardv1.ShardInstance, totalLoad float64) error {
	// Define overload threshold (configurable in production)
	overloadThreshold := 0.9
	
	// Check if system is overloaded
	if totalLoad > overloadThreshold {
		// Log system overload event
		sm.structuredLogger.LogSystemEvent(ctx, "system_overload", "critical", map[string]interface{}{
			"total_load":     totalLoad,
			"threshold":      overloadThreshold,
			"shard_count":    len(shards),
			"healthy_shards": sm.countHealthyShards(shards),
		})
		
		// Send system overload alert
		if err := sm.alertManager.AlertSystemOverload(ctx, totalLoad, overloadThreshold, len(shards)); err != nil {
			return fmt.Errorf("failed to send system overload alert: %w", err)
		}
		
		// Record overload metric
		sm.metricsCollector.RecordCustomMetric("system_overload_events_total", 1, map[string]string{
			"severity": "critical",
		})
	}
	
	// Check for high error rates in components
	if err := sm.checkComponentErrorRates(ctx); err != nil {
		return fmt.Errorf("failed to check component error rates: %w", err)
	}
	
	return nil
}

// checkComponentErrorRates checks for high error rates in system components
func (sm *ShardManager) checkComponentErrorRates(ctx context.Context) error {
	// This is a simplified implementation
	// In production, this would analyze actual error metrics from Prometheus
	
	components := []string{"shard_manager", "health_checker", "resource_migrator", "load_balancer"}
	errorRateThreshold := 0.05 // 5% error rate threshold
	
	for _, component := range components {
		// Simulate error rate calculation (in production, query from metrics)
		// This would typically query Prometheus for actual error rates
		errorRate := 0.02 // Placeholder - would be calculated from actual metrics
		
		if errorRate > errorRateThreshold {
			// Send high error rate alert
			if err := sm.alertManager.AlertHighErrorRate(ctx, component, errorRate, errorRateThreshold); err != nil {
				log.Log.Error(err, "Failed to send high error rate alert", "component", component)
			}
			
			// Log high error rate event
			sm.structuredLogger.LogSystemEvent(ctx, "high_error_rate", "warning", map[string]interface{}{
				"component":   component,
				"error_rate":  errorRate,
				"threshold":   errorRateThreshold,
			})
		}
	}
	
	return nil
}

// calculateHashRangeForNewShard calculates hash range for a new shard
func (sm *ShardManager) calculateHashRangeForNewShard(existingShards []*shardv1.ShardInstance) *shardv1.HashRange {
	// Simple implementation: divide hash space evenly
	// In a production system, this would be more sophisticated
	totalShards := len(existingShards) + 1
	rangeSize := uint32(0xFFFFFFFF) / uint32(totalShards)
	
	shardIndex := len(existingShards)
	start := uint32(shardIndex) * rangeSize
	end := start + rangeSize - 1
	
	if shardIndex == totalShards-1 {
		end = 0xFFFFFFFF // Last shard gets remainder
	}

	return &shardv1.HashRange{
		Start: start,
		End:   end,
	}
}