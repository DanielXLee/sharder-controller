package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/utils"
)

// HealthChecker implements the HealthChecker interface
type HealthChecker struct {
	client client.Client
	config config.HealthCheckConfig
	
	mu              sync.RWMutex
	shardStatuses   map[string]*shardv1.HealthStatus
	failureHistory  map[string][]time.Time // Track failure timestamps for each shard
	recoveryHistory map[string][]time.Time // Track recovery timestamps for each shard
	checkContext    context.Context
	checkCancel     context.CancelFunc
	isChecking      bool
	
	// Callbacks for shard state changes
	onShardFailedCallback    func(ctx context.Context, shardId string) error
	onShardRecoveredCallback func(ctx context.Context, shardId string) error
}

// NewHealthChecker creates a new HealthChecker
func NewHealthChecker(client client.Client, config config.HealthCheckConfig) (*HealthChecker, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	return &HealthChecker{
		client:          client,
		config:          config,
		shardStatuses:   make(map[string]*shardv1.HealthStatus),
		failureHistory:  make(map[string][]time.Time),
		recoveryHistory: make(map[string][]time.Time),
	}, nil
}

// SetFailureCallback sets the callback function for shard failures
func (hc *HealthChecker) SetFailureCallback(callback func(ctx context.Context, shardId string) error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.onShardFailedCallback = callback
}

// SetRecoveryCallback sets the callback function for shard recovery
func (hc *HealthChecker) SetRecoveryCallback(callback func(ctx context.Context, shardId string) error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.onShardRecoveredCallback = callback
}

// CheckHealth checks the health of a shard with enhanced failure detection
func (hc *HealthChecker) CheckHealth(ctx context.Context, shard *shardv1.ShardInstance) (*shardv1.HealthStatus, error) {
	if shard == nil {
		return nil, fmt.Errorf("shard cannot be nil")
	}

	logger := log.FromContext(ctx).WithValues("shardId", shard.Spec.ShardID)
	now := metav1.Now()
	shardId := shard.Spec.ShardID

	// Get previous health status
	hc.mu.RLock()
	prevStatus, exists := hc.shardStatuses[shardId]
	hc.mu.RUnlock()

	// Initialize health status
	healthStatus := &shardv1.HealthStatus{
		LastCheck: now,
		Healthy:   true,
		Message:   "Shard is healthy",
	}

	// Check 1: Shard phase validation
	if !utils.IsHealthyShardPhase(shard.Status.Phase) {
		healthStatus.Healthy = false
		healthStatus.Message = fmt.Sprintf("Shard is in unhealthy phase: %s", shard.Status.Phase)
		logger.V(1).Info("Shard phase is unhealthy", "phase", shard.Status.Phase)
	}

	// Check 2: Heartbeat freshness
	heartbeatAge := time.Since(shard.Status.LastHeartbeat.Time)
	if heartbeatAge > hc.config.Timeout {
		healthStatus.Healthy = false
		healthStatus.Message = fmt.Sprintf("Shard heartbeat is stale (age: %v, timeout: %v)", heartbeatAge, hc.config.Timeout)
		logger.V(1).Info("Shard heartbeat is stale", "age", heartbeatAge, "timeout", hc.config.Timeout)
	}

	// Check 3: Load metrics validation (if available)
	if shard.Status.Load > 1.0 {
		healthStatus.Healthy = false
		healthStatus.Message = fmt.Sprintf("Shard is overloaded (load: %.2f)", shard.Status.Load)
		logger.V(1).Info("Shard is overloaded", "load", shard.Status.Load)
	}

	// Handle error counting and thresholds
	if exists {
		if healthStatus.Healthy {
			// Shard is healthy now
			if !prevStatus.Healthy {
				// Shard recovered - reset error count and track recovery
				healthStatus.ErrorCount = 0
				hc.trackRecovery(shardId, now.Time)
				logger.Info("Shard recovered from failure")
			} else {
				// Shard remains healthy
				healthStatus.ErrorCount = 0
			}
		} else {
			// Shard is unhealthy
			if prevStatus.Healthy {
				// First failure - start counting
				healthStatus.ErrorCount = 1
				hc.trackFailure(shardId, now.Time)
				logger.Info("Shard first failure detected")
			} else {
				// Consecutive failure - increment count
				healthStatus.ErrorCount = prevStatus.ErrorCount + 1
				hc.trackFailure(shardId, now.Time)
				logger.Info("Shard consecutive failure", "errorCount", healthStatus.ErrorCount)
			}
		}
	} else {
		// First time checking this shard
		if !healthStatus.Healthy {
			healthStatus.ErrorCount = 1
			hc.trackFailure(shardId, now.Time)
			logger.Info("New shard detected as unhealthy")
		} else {
			healthStatus.ErrorCount = 0
			logger.Info("New shard detected as healthy")
		}
	}

	// Store the health status for next check
	hc.mu.Lock()
	hc.shardStatuses[shardId] = healthStatus
	hc.mu.Unlock()

	return healthStatus, nil
}

// trackFailure records a failure timestamp for a shard
func (hc *HealthChecker) trackFailure(shardId string, timestamp time.Time) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if hc.failureHistory[shardId] == nil {
		hc.failureHistory[shardId] = make([]time.Time, 0)
	}
	
	// Add new failure timestamp
	hc.failureHistory[shardId] = append(hc.failureHistory[shardId], timestamp)
	
	// Keep only recent failures (last hour)
	cutoff := timestamp.Add(-time.Hour)
	filtered := make([]time.Time, 0)
	for _, t := range hc.failureHistory[shardId] {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	hc.failureHistory[shardId] = filtered
}

// trackRecovery records a recovery timestamp for a shard
func (hc *HealthChecker) trackRecovery(shardId string, timestamp time.Time) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if hc.recoveryHistory[shardId] == nil {
		hc.recoveryHistory[shardId] = make([]time.Time, 0)
	}
	
	// Add new recovery timestamp
	hc.recoveryHistory[shardId] = append(hc.recoveryHistory[shardId], timestamp)
	
	// Keep only recent recoveries (last hour)
	cutoff := timestamp.Add(-time.Hour)
	filtered := make([]time.Time, 0)
	for _, t := range hc.recoveryHistory[shardId] {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	hc.recoveryHistory[shardId] = filtered
}

// StartHealthChecking starts periodic health checking
func (hc *HealthChecker) StartHealthChecking(ctx context.Context, interval time.Duration) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.isChecking {
		return fmt.Errorf("health checking is already running")
	}

	// Create a new check context
	checkCtx, cancel := context.WithCancel(ctx)
	hc.checkContext = checkCtx
	hc.checkCancel = cancel
	hc.isChecking = true

	// Start a goroutine to periodically check shard health
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-checkCtx.Done():
				return
			case <-ticker.C:
				hc.checkAllShards(checkCtx)
			}
		}
	}()

	return nil
}

// checkAllShards lists all shards and checks their health with automatic failure handling
func (hc *HealthChecker) checkAllShards(ctx context.Context) {
	logger := log.FromContext(ctx)
	
	shardList := &shardv1.ShardInstanceList{}
	if err := hc.client.List(ctx, shardList); err != nil {
		logger.Error(err, "Failed to list shards for health checking")
		return
	}

	logger.V(1).Info("Checking health of all shards", "shardCount", len(shardList.Items))

	for _, shard := range shardList.Items {
		shardCopy := shard.DeepCopy()
		if err := hc.checkAndUpdateShardHealth(ctx, shardCopy); err != nil {
			logger.Error(err, "Failed to check and update shard health", "shardId", shard.Spec.ShardID)
		}
	}
}

// checkAndUpdateShardHealth checks a single shard's health and handles state transitions
func (hc *HealthChecker) checkAndUpdateShardHealth(ctx context.Context, shard *shardv1.ShardInstance) error {
	logger := log.FromContext(ctx).WithValues("shardId", shard.Spec.ShardID)
	
	// Check the shard's health
	healthStatus, err := hc.CheckHealth(ctx, shard)
	if err != nil {
		return fmt.Errorf("failed to check health: %w", err)
	}

	// Get previous health status for comparison
	previouslyHealthy := shard.Status.HealthStatus != nil && shard.Status.HealthStatus.Healthy
	currentlyHealthy := healthStatus.Healthy

	// Update the shard's health status
	shard.Status.HealthStatus = healthStatus

	// Handle state transitions based on health changes and failure thresholds
	if err := hc.handleHealthStateTransition(ctx, shard, previouslyHealthy, currentlyHealthy); err != nil {
		logger.Error(err, "Failed to handle health state transition")
		return err
	}

	// Update the shard status in Kubernetes
	if err := hc.client.Status().Update(ctx, shard); err != nil {
		return fmt.Errorf("failed to update shard status: %w", err)
	}

	logger.V(1).Info("Updated shard health status", 
		"healthy", healthStatus.Healthy, 
		"errorCount", healthStatus.ErrorCount,
		"message", healthStatus.Message)

	return nil
}

// handleHealthStateTransition manages shard state transitions based on health status
func (hc *HealthChecker) handleHealthStateTransition(ctx context.Context, shard *shardv1.ShardInstance, previouslyHealthy, currentlyHealthy bool) error {
	logger := log.FromContext(ctx).WithValues("shardId", shard.Spec.ShardID)
	shardId := shard.Spec.ShardID

	// Case 1: Shard became unhealthy (failure detected)
	if previouslyHealthy && !currentlyHealthy {
		logger.Info("Shard transitioned from healthy to unhealthy")
		
		// Check if we've reached the failure threshold
		if shard.Status.HealthStatus.ErrorCount >= hc.config.FailureThreshold {
			logger.Info("Shard reached failure threshold, marking as failed", 
				"errorCount", shard.Status.HealthStatus.ErrorCount,
				"threshold", hc.config.FailureThreshold)
			
			// Transition to failed state
			if err := shard.TransitionTo(shardv1.ShardPhaseFailed); err != nil {
				return fmt.Errorf("failed to transition shard to failed state: %w", err)
			}
			
			// Call failure callback if set
			hc.mu.RLock()
			callback := hc.onShardFailedCallback
			hc.mu.RUnlock()
			
			if callback != nil {
				if err := callback(ctx, shardId); err != nil {
					logger.Error(err, "Failure callback returned error")
				}
			}
		}
	}

	// Case 2: Shard recovered (became healthy)
	if !previouslyHealthy && currentlyHealthy {
		logger.Info("Shard transitioned from unhealthy to healthy")
		
		// If shard was in failed state, transition back to running
		if shard.Status.Phase == shardv1.ShardPhaseFailed {
			logger.Info("Recovering failed shard to running state")
			
			if err := shard.TransitionTo(shardv1.ShardPhaseRunning); err != nil {
				return fmt.Errorf("failed to transition shard to running state: %w", err)
			}
			
			// Call recovery callback if set
			hc.mu.RLock()
			callback := hc.onShardRecoveredCallback
			hc.mu.RUnlock()
			
			if callback != nil {
				if err := callback(ctx, shardId); err != nil {
					logger.Error(err, "Recovery callback returned error")
				}
			}
		}
	}

	return nil
}

// StopHealthChecking stops periodic health checking
func (hc *HealthChecker) StopHealthChecking() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if !hc.isChecking {
		return nil
	}
	
	if hc.checkCancel != nil {
		hc.checkCancel()
	}
	
	hc.isChecking = false
	return nil
}

// OnShardFailed handles a failed shard
func (hc *HealthChecker) OnShardFailed(ctx context.Context, shardId string) error {
	shard := &shardv1.ShardInstance{}
	if err := hc.client.Get(ctx, client.ObjectKey{Name: shardId}, shard); err != nil {
		return err
	}

	shard.Status.Phase = shardv1.ShardPhaseFailed
	shard.Status.HealthStatus = &shardv1.HealthStatus{
		Healthy:    false,
		LastCheck:  shard.Status.LastHeartbeat,
		ErrorCount: hc.config.FailureThreshold,
		Message:    "Shard marked as failed",
	}

	return hc.client.Status().Update(ctx, shard)
}

// OnShardRecovered handles a recovered shard
func (hc *HealthChecker) OnShardRecovered(ctx context.Context, shardId string) error {
	shard := &shardv1.ShardInstance{}
	if err := hc.client.Get(ctx, client.ObjectKey{Name: shardId}, shard); err != nil {
		return err
	}

	shard.Status.Phase = shardv1.ShardPhaseRunning
	shard.Status.HealthStatus = &shardv1.HealthStatus{
		Healthy:    true,
		LastCheck:  shard.Status.LastHeartbeat,
		ErrorCount: 0,
		Message:    "Shard recovered",
	}

	return hc.client.Status().Update(ctx, shard)
}

// ReportHeartbeat updates the heartbeat timestamp for a shard
func (hc *HealthChecker) ReportHeartbeat(ctx context.Context, shardId string) error {
	shard := &shardv1.ShardInstance{}
	if err := hc.client.Get(ctx, client.ObjectKey{Name: shardId}, shard); err != nil {
		return fmt.Errorf("failed to get shard %s: %w", shardId, err)
	}

	// Update heartbeat timestamp
	shard.Status.LastHeartbeat = metav1.Now()

	// Update the shard status
	if err := hc.client.Status().Update(ctx, shard); err != nil {
		return fmt.Errorf("failed to update heartbeat for shard %s: %w", shardId, err)
	}

	return nil
}

// GetShardHealthStatus returns the current health status of a shard
func (hc *HealthChecker) GetShardHealthStatus(shardId string) (*shardv1.HealthStatus, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	status, exists := hc.shardStatuses[shardId]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent external modification
	statusCopy := &shardv1.HealthStatus{
		Healthy:    status.Healthy,
		LastCheck:  status.LastCheck,
		ErrorCount: status.ErrorCount,
		Message:    status.Message,
	}
	
	return statusCopy, true
}

// GetFailureHistory returns the failure history for a shard
func (hc *HealthChecker) GetFailureHistory(shardId string) []time.Time {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	history, exists := hc.failureHistory[shardId]
	if !exists {
		return nil
	}
	
	// Return a copy to prevent external modification
	historyCopy := make([]time.Time, len(history))
	copy(historyCopy, history)
	return historyCopy
}

// GetRecoveryHistory returns the recovery history for a shard
func (hc *HealthChecker) GetRecoveryHistory(shardId string) []time.Time {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	history, exists := hc.recoveryHistory[shardId]
	if !exists {
		return nil
	}
	
	// Return a copy to prevent external modification
	historyCopy := make([]time.Time, len(history))
	copy(historyCopy, history)
	return historyCopy
}

// GetHealthSummary returns a summary of all shard health statuses
func (hc *HealthChecker) GetHealthSummary() map[string]*shardv1.HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	summary := make(map[string]*shardv1.HealthStatus)
	for shardId, status := range hc.shardStatuses {
		summary[shardId] = &shardv1.HealthStatus{
			Healthy:    status.Healthy,
			LastCheck:  status.LastCheck,
			ErrorCount: status.ErrorCount,
			Message:    status.Message,
		}
	}
	
	return summary
}

// IsShardHealthy checks if a specific shard is currently healthy
func (hc *HealthChecker) IsShardHealthy(shardId string) bool {
	status, exists := hc.GetShardHealthStatus(shardId)
	return exists && status.Healthy
}

// GetUnhealthyShards returns a list of currently unhealthy shard IDs
func (hc *HealthChecker) GetUnhealthyShards() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	var unhealthy []string
	for shardId, status := range hc.shardStatuses {
		if !status.Healthy {
			unhealthy = append(unhealthy, shardId)
		}
	}
	
	return unhealthy
}

// CleanupShardData removes health data for a shard (called when shard is deleted)
func (hc *HealthChecker) CleanupShardData(shardId string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	delete(hc.shardStatuses, shardId)
	delete(hc.failureHistory, shardId)
	delete(hc.recoveryHistory, shardId)
}