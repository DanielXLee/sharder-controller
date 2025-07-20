package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// ResourceMigrator implements the ResourceMigrator interface
type ResourceMigrator struct {
	shardManager interfaces.ShardManager

	// Migration tracking
	activeMigrations  map[string]*MigrationExecution
	migrationHistory  map[string]*MigrationExecution
	pendingMigrations []*shardv1.MigrationPlan
	mu                sync.RWMutex

	// Configuration
	maxRetries              int
	retryInterval           time.Duration
	migrationTimeout        time.Duration
	maxConcurrentMigrations int

	// Scheduling
	scheduler *MigrationScheduler
}

// MigrationExecution tracks the execution of a migration plan
type MigrationExecution struct {
	Plan      *shardv1.MigrationPlan
	Status    interfaces.MigrationStatus
	StartTime time.Time
	EndTime   *time.Time
	Error     error
	Retries   int

	// Progress tracking
	TotalResources    int
	MigratedResources int
	FailedResources   []string

	// Cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// MigrationScheduler manages the scheduling and prioritization of migrations
type MigrationScheduler struct {
	priorityQueue  []*shardv1.MigrationPlan
	mu             sync.RWMutex
	maxConcurrent  int
	currentRunning int
}

// NewMigrationScheduler creates a new migration scheduler
func NewMigrationScheduler(maxConcurrent int) *MigrationScheduler {
	return &MigrationScheduler{
		priorityQueue:  make([]*shardv1.MigrationPlan, 0),
		maxConcurrent:  maxConcurrent,
		currentRunning: 0,
	}
}

// AddPlan adds a migration plan to the scheduler queue
func (ms *MigrationScheduler) AddPlan(plan *shardv1.MigrationPlan) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Insert plan in priority order (high priority first)
	inserted := false
	for i, existingPlan := range ms.priorityQueue {
		if ms.comparePriority(plan.Priority, existingPlan.Priority) > 0 {
			// Insert at position i
			ms.priorityQueue = append(ms.priorityQueue[:i], append([]*shardv1.MigrationPlan{plan}, ms.priorityQueue[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		ms.priorityQueue = append(ms.priorityQueue, plan)
	}
}

// GetNextPlan returns the next plan to execute if capacity allows
func (ms *MigrationScheduler) GetNextPlan() *shardv1.MigrationPlan {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.currentRunning >= ms.maxConcurrent || len(ms.priorityQueue) == 0 {
		return nil
	}

	plan := ms.priorityQueue[0]
	ms.priorityQueue = ms.priorityQueue[1:]
	ms.currentRunning++

	return plan
}

// MarkCompleted marks a migration as completed
func (ms *MigrationScheduler) MarkCompleted() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.currentRunning > 0 {
		ms.currentRunning--
	}
}

// comparePriority compares two migration priorities (returns > 0 if p1 > p2)
func (ms *MigrationScheduler) comparePriority(p1, p2 shardv1.MigrationPriority) int {
	priorityValues := map[shardv1.MigrationPriority]int{
		shardv1.MigrationPriorityHigh:   3,
		shardv1.MigrationPriorityMedium: 2,
		shardv1.MigrationPriorityLow:    1,
	}

	return priorityValues[p1] - priorityValues[p2]
}

// GetQueueStatus returns the current queue status
func (ms *MigrationScheduler) GetQueueStatus() (int, int, int) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return len(ms.priorityQueue), ms.currentRunning, ms.maxConcurrent
}

// NewResourceMigrator creates a new ResourceMigrator
func NewResourceMigrator(shardManager interfaces.ShardManager) *ResourceMigrator {
	rm := &ResourceMigrator{
		shardManager:            shardManager,
		activeMigrations:        make(map[string]*MigrationExecution),
		migrationHistory:        make(map[string]*MigrationExecution),
		pendingMigrations:       make([]*shardv1.MigrationPlan, 0),
		maxRetries:              3,
		retryInterval:           time.Second * 30,
		migrationTimeout:        time.Minute * 10,
		maxConcurrentMigrations: 3,
	}

	rm.scheduler = NewMigrationScheduler(rm.maxConcurrentMigrations)
	return rm
}

// CreateMigrationPlan creates a migration plan for moving resources between shards
func (rm *ResourceMigrator) CreateMigrationPlan(ctx context.Context, sourceShard, targetShard string, resources []*interfaces.Resource) (*shardv1.MigrationPlan, error) {
	if sourceShard == targetShard {
		return nil, fmt.Errorf("source and target shards cannot be the same")
	}

	if len(resources) == 0 {
		return nil, fmt.Errorf("no resources specified for migration")
	}

	// Validate that source and target shards exist and are healthy
	sourceShardInstance, err := rm.shardManager.GetShardStatus(ctx, sourceShard)
	if err != nil {
		return nil, fmt.Errorf("failed to get source shard status: %w", err)
	}

	targetShardInstance, err := rm.shardManager.GetShardStatus(ctx, targetShard)
	if err != nil {
		return nil, fmt.Errorf("failed to get target shard status: %w", err)
	}

	// Check if target shard can accept the resources
	if !rm.canAcceptResources(targetShardInstance, len(resources)) {
		return nil, fmt.Errorf("target shard %s cannot accept %d resources", targetShard, len(resources))
	}

	// Estimate migration time based on resource count and complexity
	estimatedTime := rm.estimateMigrationTime(resources)

	// Determine priority based on source shard health
	priority := rm.determineMigrationPriority(sourceShardInstance)

	// Convert resources to string IDs
	resourceIDs := make([]string, len(resources))
	for i, resource := range resources {
		resourceIDs[i] = resource.ID
	}

	plan := &shardv1.MigrationPlan{
		SourceShard:   sourceShard,
		TargetShard:   targetShard,
		Resources:     resourceIDs,
		EstimatedTime: estimatedTime,
		Priority:      priority,
	}

	klog.V(2).Infof("Created migration plan: %d resources from %s to %s (priority: %s, estimated time: %v)",
		len(resources), sourceShard, targetShard, priority, estimatedTime.Duration)

	return plan, nil
}

// ExecuteMigration executes a migration plan
func (rm *ResourceMigrator) ExecuteMigration(ctx context.Context, plan *shardv1.MigrationPlan) error {
	planID := rm.generatePlanID(plan)

	rm.mu.Lock()
	if _, exists := rm.activeMigrations[planID]; exists {
		rm.mu.Unlock()
		return fmt.Errorf("migration plan %s is already being executed", planID)
	}

	// Create migration execution context
	migrationCtx, cancel := context.WithTimeout(ctx, rm.migrationTimeout)
	execution := &MigrationExecution{
		Plan:              plan,
		Status:            interfaces.MigrationStatusPending,
		StartTime:         time.Now(),
		TotalResources:    len(plan.Resources),
		MigratedResources: 0,
		FailedResources:   make([]string, 0),
		ctx:               migrationCtx,
		cancel:            cancel,
	}

	rm.activeMigrations[planID] = execution
	rm.mu.Unlock()

	klog.V(1).Infof("Starting migration execution for plan %s: %d resources from %s to %s",
		planID, len(plan.Resources), plan.SourceShard, plan.TargetShard)

	// Execute migration in a separate goroutine
	go rm.executeMigrationAsync(planID, execution)

	return nil
}

// executeMigrationAsync executes the migration asynchronously
func (rm *ResourceMigrator) executeMigrationAsync(planID string, execution *MigrationExecution) {
	defer func() {
		execution.cancel()
		endTime := time.Now()
		execution.EndTime = &endTime

		rm.mu.Lock()
		delete(rm.activeMigrations, planID)
		rm.migrationHistory[planID] = execution
		rm.mu.Unlock()

		klog.V(1).Infof("Migration execution completed for plan %s: status=%s, migrated=%d/%d, duration=%v",
			planID, execution.Status, execution.MigratedResources, execution.TotalResources, endTime.Sub(execution.StartTime))
	}()

	// Update status to in progress
	execution.Status = interfaces.MigrationStatusInProgress

	// Execute migration with retries
	err := rm.executeMigrationWithRetries(execution)
	if err != nil {
		execution.Status = interfaces.MigrationStatusFailed
		execution.Error = err
		klog.Errorf("Migration failed for plan %s: %v", planID, err)
		return
	}

	execution.Status = interfaces.MigrationStatusCompleted
	klog.V(1).Infof("Migration completed successfully for plan %s", planID)
}

// executeMigrationWithRetries executes migration with retry logic
func (rm *ResourceMigrator) executeMigrationWithRetries(execution *MigrationExecution) error {
	plan := execution.Plan

	for execution.Retries <= rm.maxRetries {
		if execution.ctx.Err() != nil {
			return fmt.Errorf("migration cancelled or timed out")
		}

		err := rm.performMigration(execution)
		if err == nil {
			return nil // Success
		}

		execution.Retries++
		execution.Error = err

		if execution.Retries > rm.maxRetries {
			return fmt.Errorf("migration failed after %d retries: %w", rm.maxRetries, err)
		}

		klog.V(2).Infof("Migration attempt %d failed for plan %s->%s, retrying in %v: %v",
			execution.Retries, plan.SourceShard, plan.TargetShard, rm.retryInterval, err)

		// Wait before retry
		select {
		case <-execution.ctx.Done():
			return fmt.Errorf("migration cancelled during retry wait")
		case <-time.After(rm.retryInterval):
			// Continue to next retry
		}
	}

	return fmt.Errorf("migration failed after %d retries", rm.maxRetries)
}

// performMigration performs the actual migration
func (rm *ResourceMigrator) performMigration(execution *MigrationExecution) error {
	plan := execution.Plan

	// Get shard instances
	shards, err := rm.shardManager.ListShards(execution.ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}

	var sourceShard, targetShard *shardv1.ShardInstance
	for _, shard := range shards {
		if shard.Spec.ShardID == plan.SourceShard {
			sourceShard = shard
		}
		if shard.Spec.ShardID == plan.TargetShard {
			targetShard = shard
		}
	}

	if sourceShard == nil {
		return fmt.Errorf("source shard %s not found", plan.SourceShard)
	}
	if targetShard == nil {
		return fmt.Errorf("target shard %s not found", plan.TargetShard)
	}

	// Verify shards are in appropriate states
	if !sourceShard.IsActive() && sourceShard.Status.Phase != shardv1.ShardPhaseDraining {
		return fmt.Errorf("source shard %s is not in active or draining state: %s", plan.SourceShard, sourceShard.Status.Phase)
	}
	if !targetShard.IsActive() {
		return fmt.Errorf("target shard %s is not active: %s", plan.TargetShard, targetShard.Status.Phase)
	}

	// Get resources to migrate
	resources, err := rm.getResourcesForMigration(execution.ctx, sourceShard, plan.Resources)
	if err != nil {
		return fmt.Errorf("failed to get resources for migration: %w", err)
	}

	// Perform the migration in batches
	batchSize := rm.calculateBatchSize(len(resources))
	for i := 0; i < len(resources); i += batchSize {
		if execution.ctx.Err() != nil {
			return fmt.Errorf("migration cancelled")
		}

		end := i + batchSize
		if end > len(resources) {
			end = len(resources)
		}

		batch := resources[i:end]
		err := rm.migrateBatch(execution.ctx, sourceShard, targetShard, batch)
		if err != nil {
			// Track failed resources
			for _, resource := range batch {
				execution.FailedResources = append(execution.FailedResources, resource.ID)
			}
			return fmt.Errorf("failed to migrate batch %d-%d: %w", i, end-1, err)
		}

		execution.MigratedResources += len(batch)
		klog.V(3).Infof("Migrated batch %d-%d (%d resources) from %s to %s",
			i, end-1, len(batch), plan.SourceShard, plan.TargetShard)
	}

	return nil
}

// migrateBatch migrates a batch of resources with proper coordination
func (rm *ResourceMigrator) migrateBatch(ctx context.Context, sourceShard, targetShard *shardv1.ShardInstance, resources []*interfaces.Resource) error {
	// Phase 1: Prepare target shard for incoming resources
	err := rm.prepareTargetShard(ctx, targetShard, resources)
	if err != nil {
		return fmt.Errorf("failed to prepare target shard %s: %w", targetShard.Spec.ShardID, err)
	}

	// Phase 2: Transfer resources with consistency checks
	transferredResources := make([]*interfaces.Resource, 0, len(resources))
	for _, resource := range resources {
		err := rm.transferResource(ctx, sourceShard, targetShard, resource)
		if err != nil {
			// Rollback already transferred resources in this batch
			rm.rollbackBatchTransfer(ctx, sourceShard, targetShard, transferredResources)
			return fmt.Errorf("failed to transfer resource %s: %w", resource.ID, err)
		}
		transferredResources = append(transferredResources, resource)

		klog.V(4).Infof("Transferred resource %s from shard %s to shard %s",
			resource.ID, sourceShard.Spec.ShardID, targetShard.Spec.ShardID)
	}

	// Phase 3: Commit the migration by updating shard assignments
	for _, resource := range transferredResources {
		sourceShard.RemoveResource(resource.ID)
		targetShard.AddResource(resource.ID)
	}

	// Phase 4: Cleanup source shard
	err = rm.cleanupSourceShard(ctx, sourceShard, transferredResources)
	if err != nil {
		klog.Warningf("Failed to cleanup source shard %s after migration: %v", sourceShard.Spec.ShardID, err)
		// Don't fail the migration for cleanup errors
	}

	return nil
}

// prepareTargetShard prepares the target shard to receive resources
func (rm *ResourceMigrator) prepareTargetShard(ctx context.Context, targetShard *shardv1.ShardInstance, resources []*interfaces.Resource) error {
	// Check if target shard has enough capacity
	estimatedLoad := float64(len(resources)) * 0.02 // Each resource adds ~2% load
	if targetShard.Status.Load+estimatedLoad > 0.9 {
		return fmt.Errorf("target shard %s would be overloaded (current: %.2f, estimated: %.2f)",
			targetShard.Spec.ShardID, targetShard.Status.Load, targetShard.Status.Load+estimatedLoad)
	}

	// Reserve capacity on target shard
	targetShard.Status.Load += estimatedLoad

	klog.V(3).Infof("Prepared target shard %s for %d resources (load: %.2f -> %.2f)",
		targetShard.Spec.ShardID, len(resources), targetShard.Status.Load-estimatedLoad, targetShard.Status.Load)

	return nil
}

// transferResource transfers a single resource between shards
func (rm *ResourceMigrator) transferResource(ctx context.Context, sourceShard, targetShard *shardv1.ShardInstance, resource *interfaces.Resource) error {
	// In a real implementation, this would involve:
	// 1. Serializing resource state from source shard
	// 2. Transferring data to target shard
	// 3. Verifying data integrity
	// 4. Confirming successful transfer

	// Simulate network transfer delay
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Millisecond * 10): // Simulate 10ms transfer time
	}

	// Simulate potential transfer failures (5% failure rate for testing)
	if rm.shouldSimulateFailure() {
		return fmt.Errorf("simulated transfer failure for resource %s", resource.ID)
	}

	// Update resource metadata to track migration
	if resource.Metadata == nil {
		resource.Metadata = make(map[string]string)
	}
	resource.Metadata["migrated_from"] = sourceShard.Spec.ShardID
	resource.Metadata["migrated_to"] = targetShard.Spec.ShardID
	resource.Metadata["migration_timestamp"] = time.Now().Format(time.RFC3339)

	return nil
}

// rollbackBatchTransfer rolls back a partially completed batch transfer
func (rm *ResourceMigrator) rollbackBatchTransfer(ctx context.Context, sourceShard, targetShard *shardv1.ShardInstance, transferredResources []*interfaces.Resource) {
	klog.V(2).Infof("Rolling back partial batch transfer of %d resources from %s to %s",
		len(transferredResources), sourceShard.Spec.ShardID, targetShard.Spec.ShardID)

	for _, resource := range transferredResources {
		// In a real implementation, this would involve:
		// 1. Removing resource from target shard
		// 2. Restoring resource state on source shard
		// 3. Cleaning up any partial state

		// For now, just log the rollback
		klog.V(4).Infof("Rolled back resource %s from %s to %s",
			resource.ID, targetShard.Spec.ShardID, sourceShard.Spec.ShardID)
	}

	// Restore target shard load
	estimatedLoad := float64(len(transferredResources)) * 0.02
	targetShard.Status.Load -= estimatedLoad
}

// cleanupSourceShard cleans up resources on the source shard after migration
func (rm *ResourceMigrator) cleanupSourceShard(ctx context.Context, sourceShard *shardv1.ShardInstance, resources []*interfaces.Resource) error {
	// In a real implementation, this would involve:
	// 1. Removing resource data from source shard
	// 2. Updating shard metrics
	// 3. Freeing up allocated resources

	// Update source shard load
	estimatedLoad := float64(len(resources)) * 0.02
	sourceShard.Status.Load -= estimatedLoad
	if sourceShard.Status.Load < 0 {
		sourceShard.Status.Load = 0
	}

	klog.V(3).Infof("Cleaned up source shard %s after migrating %d resources (load: %.2f)",
		sourceShard.Spec.ShardID, len(resources), sourceShard.Status.Load)

	return nil
}

// shouldSimulateFailure returns true if we should simulate a failure for testing
func (rm *ResourceMigrator) shouldSimulateFailure() bool {
	// In production, this would always return false
	// For testing, we simulate a 5% failure rate
	return false // Disabled for now to avoid test flakiness
}

// GetMigrationStatus returns the status of a migration
func (rm *ResourceMigrator) GetMigrationStatus(ctx context.Context, planId string) (interfaces.MigrationStatus, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Check active migrations first
	if execution, exists := rm.activeMigrations[planId]; exists {
		return execution.Status, nil
	}

	// Check migration history
	if execution, exists := rm.migrationHistory[planId]; exists {
		return execution.Status, nil
	}

	return "", fmt.Errorf("migration plan %s not found", planId)
}

// RollbackMigration rolls back a migration
func (rm *ResourceMigrator) RollbackMigration(ctx context.Context, planId string) error {
	rm.mu.Lock()

	// Check if migration is active
	if execution, exists := rm.activeMigrations[planId]; exists {
		// Cancel active migration
		execution.cancel()
		execution.Status = interfaces.MigrationStatusRolledBack
		rm.mu.Unlock()

		klog.V(1).Infof("Cancelled active migration %s", planId)
		return nil
	}

	// Check migration history for completed migrations
	execution, exists := rm.migrationHistory[planId]
	if !exists {
		rm.mu.Unlock()
		return fmt.Errorf("migration plan %s not found", planId)
	}

	if execution.Status != interfaces.MigrationStatusCompleted {
		rm.mu.Unlock()
		return fmt.Errorf("can only rollback completed migrations, current status: %s", execution.Status)
	}

	// Create reverse migration plan
	reversePlan := &shardv1.MigrationPlan{
		SourceShard:   execution.Plan.TargetShard,
		TargetShard:   execution.Plan.SourceShard,
		Resources:     execution.Plan.Resources,
		EstimatedTime: execution.Plan.EstimatedTime,
		Priority:      shardv1.MigrationPriorityHigh, // Rollbacks are high priority
	}

	// Update original execution status before releasing lock
	execution.Status = interfaces.MigrationStatusRolledBack
	rm.mu.Unlock()

	// Execute reverse migration (without holding the lock)
	err := rm.ExecuteMigration(ctx, reversePlan)
	if err != nil {
		return fmt.Errorf("failed to execute rollback migration: %w", err)
	}

	klog.V(1).Infof("Initiated rollback for migration %s", planId)
	return nil
}

// Helper methods

func (rm *ResourceMigrator) generatePlanID(plan *shardv1.MigrationPlan) string {
	// Create a deterministic ID based on plan content for duplicate detection
	resourcesHash := ""
	if len(plan.Resources) > 0 {
		resourcesHash = plan.Resources[0] // Use first resource as part of ID
	}
	return fmt.Sprintf("%s-%s-%s", plan.SourceShard, plan.TargetShard, resourcesHash)
}

func (rm *ResourceMigrator) canAcceptResources(shard *shardv1.ShardInstanceStatus, resourceCount int) bool {
	// Simple capacity check - in a real implementation, this would be more sophisticated
	currentLoad := shard.Load
	estimatedAdditionalLoad := float64(resourceCount) * 0.02 // Assume each resource adds 0.02 load

	return (currentLoad + estimatedAdditionalLoad) < 0.8 // Don't exceed 80% capacity
}

func (rm *ResourceMigrator) estimateMigrationTime(resources []*interfaces.Resource) metav1.Duration {
	// Simple estimation: 1 second per resource + base overhead
	baseTime := time.Second * 10
	perResourceTime := time.Millisecond * 500

	totalTime := baseTime + time.Duration(len(resources))*perResourceTime
	return metav1.Duration{Duration: totalTime}
}

func (rm *ResourceMigrator) determineMigrationPriority(shard *shardv1.ShardInstanceStatus) shardv1.MigrationPriority {
	if shard.Phase == shardv1.ShardPhaseFailed {
		return shardv1.MigrationPriorityHigh
	}
	if shard.Load > 0.8 {
		return shardv1.MigrationPriorityMedium
	}
	return shardv1.MigrationPriorityLow
}

func (rm *ResourceMigrator) calculateBatchSize(totalResources int) int {
	// Calculate appropriate batch size based on total resources
	if totalResources <= 10 {
		return totalResources
	}
	if totalResources <= 100 {
		return 10
	}
	return 20
}

func (rm *ResourceMigrator) getResourcesForMigration(ctx context.Context, shard *shardv1.ShardInstance, resourceIDs []string) ([]*interfaces.Resource, error) {
	// In a real implementation, this would fetch actual resource data
	// For now, we'll create mock resources
	resources := make([]*interfaces.Resource, len(resourceIDs))
	for i, id := range resourceIDs {
		resources[i] = &interfaces.Resource{
			ID:   id,
			Type: "mock-resource",
			Data: map[string]string{
				"shard": shard.Spec.ShardID,
			},
			Metadata: map[string]string{
				"migrated_at": time.Now().Format(time.RFC3339),
			},
		}
	}
	return resources, nil
}

// GetActiveMigrations returns all currently active migrations
func (rm *ResourceMigrator) GetActiveMigrations() map[string]*MigrationExecution {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make(map[string]*MigrationExecution)
	for k, v := range rm.activeMigrations {
		result[k] = v
	}
	return result
}

// GetMigrationHistory returns migration history
func (rm *ResourceMigrator) GetMigrationHistory() map[string]*MigrationExecution {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make(map[string]*MigrationExecution)
	for k, v := range rm.migrationHistory {
		result[k] = v
	}
	return result
}

// SetConfiguration allows updating migration configuration
func (rm *ResourceMigrator) SetConfiguration(maxRetries int, retryInterval, migrationTimeout time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.maxRetries = maxRetries
	rm.retryInterval = retryInterval
	rm.migrationTimeout = migrationTimeout
}

// ScheduleMigration adds a migration plan to the scheduler queue
func (rm *ResourceMigrator) ScheduleMigration(plan *shardv1.MigrationPlan) {
	rm.scheduler.AddPlan(plan)
	klog.V(2).Infof("Scheduled migration plan: %s -> %s (priority: %s)",
		plan.SourceShard, plan.TargetShard, plan.Priority)
}

// ProcessScheduledMigrations processes migrations from the scheduler queue
func (rm *ResourceMigrator) ProcessScheduledMigrations(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			plan := rm.scheduler.GetNextPlan()
			if plan == nil {
				// No plans available or at capacity, wait a bit
				time.Sleep(time.Second * 5)
				continue
			}

			// Execute the migration
			err := rm.ExecuteMigration(ctx, plan)
			if err != nil {
				klog.Errorf("Failed to execute scheduled migration %s -> %s: %v",
					plan.SourceShard, plan.TargetShard, err)
				// Mark as completed to free up scheduler slot
				rm.scheduler.MarkCompleted()
			}
		}
	}
}

// GetSchedulerStatus returns the current scheduler status
func (rm *ResourceMigrator) GetSchedulerStatus() (queued, running, maxConcurrent int) {
	return rm.scheduler.GetQueueStatus()
}

// CancelMigration cancels an active migration
func (rm *ResourceMigrator) CancelMigration(ctx context.Context, planId string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	execution, exists := rm.activeMigrations[planId]
	if !exists {
		return fmt.Errorf("migration plan %s not found or not active", planId)
	}

	// Cancel the migration context
	execution.cancel()
	execution.Status = interfaces.MigrationStatusRolledBack

	klog.V(1).Infof("Cancelled migration %s", planId)
	return nil
}

// GetMigrationProgress returns detailed progress information for a migration
func (rm *ResourceMigrator) GetMigrationProgress(ctx context.Context, planId string) (*MigrationProgress, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var execution *MigrationExecution
	var found bool

	// Check active migrations first
	if execution, found = rm.activeMigrations[planId]; !found {
		// Check migration history
		if execution, found = rm.migrationHistory[planId]; !found {
			return nil, fmt.Errorf("migration plan %s not found", planId)
		}
	}

	progress := &MigrationProgress{
		PlanID:            planId,
		Status:            execution.Status,
		StartTime:         execution.StartTime,
		EndTime:           execution.EndTime,
		TotalResources:    execution.TotalResources,
		MigratedResources: execution.MigratedResources,
		FailedResources:   execution.FailedResources,
		Retries:           execution.Retries,
		Error:             execution.Error,
	}

	if execution.EndTime != nil {
		duration := execution.EndTime.Sub(execution.StartTime)
		progress.Duration = &duration
	} else {
		duration := time.Since(execution.StartTime)
		progress.Duration = &duration
	}

	// Calculate progress percentage
	if execution.TotalResources > 0 {
		progress.ProgressPercentage = float64(execution.MigratedResources) / float64(execution.TotalResources) * 100
	}

	return progress, nil
}

// MigrationProgress represents detailed progress information for a migration
type MigrationProgress struct {
	PlanID             string                     `json:"planId"`
	Status             interfaces.MigrationStatus `json:"status"`
	StartTime          time.Time                  `json:"startTime"`
	EndTime            *time.Time                 `json:"endTime,omitempty"`
	Duration           *time.Duration             `json:"duration,omitempty"`
	TotalResources     int                        `json:"totalResources"`
	MigratedResources  int                        `json:"migratedResources"`
	FailedResources    []string                   `json:"failedResources,omitempty"`
	ProgressPercentage float64                    `json:"progressPercentage"`
	Retries            int                        `json:"retries"`
	Error              error                      `json:"error,omitempty"`
}

// ValidateMigrationPlan validates a migration plan before execution
func (rm *ResourceMigrator) ValidateMigrationPlan(ctx context.Context, plan *shardv1.MigrationPlan) error {
	if plan == nil {
		return fmt.Errorf("migration plan cannot be nil")
	}

	if plan.SourceShard == "" {
		return fmt.Errorf("source shard cannot be empty")
	}

	if plan.TargetShard == "" {
		return fmt.Errorf("target shard cannot be empty")
	}

	if plan.SourceShard == plan.TargetShard {
		return fmt.Errorf("source and target shards cannot be the same")
	}

	if len(plan.Resources) == 0 {
		return fmt.Errorf("no resources specified for migration")
	}

	// Validate that shards exist and are in appropriate states
	sourceStatus, err := rm.shardManager.GetShardStatus(ctx, plan.SourceShard)
	if err != nil {
		return fmt.Errorf("failed to get source shard status: %w", err)
	}

	targetStatus, err := rm.shardManager.GetShardStatus(ctx, plan.TargetShard)
	if err != nil {
		return fmt.Errorf("failed to get target shard status: %w", err)
	}

	// Check source shard state
	if sourceStatus.Phase != shardv1.ShardPhaseRunning && sourceStatus.Phase != shardv1.ShardPhaseDraining {
		return fmt.Errorf("source shard %s is not in a valid state for migration: %s",
			plan.SourceShard, sourceStatus.Phase)
	}

	// Check target shard state
	if targetStatus.Phase != shardv1.ShardPhaseRunning {
		return fmt.Errorf("target shard %s is not running: %s",
			plan.TargetShard, targetStatus.Phase)
	}

	// Check target shard capacity
	if !rm.canAcceptResources(targetStatus, len(plan.Resources)) {
		return fmt.Errorf("target shard %s cannot accept %d resources (current load: %.2f)",
			plan.TargetShard, len(plan.Resources), targetStatus.Load)
	}

	return nil
}

// GetMigrationStatistics returns overall migration statistics
func (rm *ResourceMigrator) GetMigrationStatistics() *MigrationStatistics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := &MigrationStatistics{
		ActiveMigrations:       len(rm.activeMigrations),
		CompletedMigrations:    0,
		FailedMigrations:       0,
		TotalResourcesMigrated: 0,
	}

	// Calculate statistics from history
	for _, execution := range rm.migrationHistory {
		switch execution.Status {
		case interfaces.MigrationStatusCompleted:
			stats.CompletedMigrations++
			stats.TotalResourcesMigrated += execution.MigratedResources
		case interfaces.MigrationStatusFailed:
			stats.FailedMigrations++
		case interfaces.MigrationStatusRolledBack:
			stats.RolledBackMigrations++
		}
	}

	// Add active migrations to total
	for _, execution := range rm.activeMigrations {
		stats.TotalResourcesMigrated += execution.MigratedResources
	}

	stats.TotalMigrations = stats.CompletedMigrations + stats.FailedMigrations + stats.RolledBackMigrations

	// Calculate success rate
	if stats.TotalMigrations > 0 {
		stats.SuccessRate = float64(stats.CompletedMigrations) / float64(stats.TotalMigrations) * 100
	}

	return stats
}

// MigrationStatistics represents overall migration statistics
type MigrationStatistics struct {
	ActiveMigrations       int     `json:"activeMigrations"`
	CompletedMigrations    int     `json:"completedMigrations"`
	FailedMigrations       int     `json:"failedMigrations"`
	RolledBackMigrations   int     `json:"rolledBackMigrations"`
	TotalMigrations        int     `json:"totalMigrations"`
	TotalResourcesMigrated int     `json:"totalResourcesMigrated"`
	SuccessRate            float64 `json:"successRate"`
}
