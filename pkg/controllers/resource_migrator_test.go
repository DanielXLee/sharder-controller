package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockShardManager is a mock implementation of ShardManager for testing
type MockShardManager struct {
	mock.Mock
}

func (m *MockShardManager) CreateShard(ctx context.Context, config *shardv1.ShardConfig) (*shardv1.ShardInstance, error) {
	args := m.Called(ctx, config)
	return args.Get(0).(*shardv1.ShardInstance), args.Error(1)
}

func (m *MockShardManager) DeleteShard(ctx context.Context, shardId string) error {
	args := m.Called(ctx, shardId)
	return args.Error(0)
}

func (m *MockShardManager) ScaleUp(ctx context.Context, targetCount int) error {
	args := m.Called(ctx, targetCount)
	return args.Error(0)
}

func (m *MockShardManager) ScaleDown(ctx context.Context, targetCount int) error {
	args := m.Called(ctx, targetCount)
	return args.Error(0)
}

func (m *MockShardManager) RebalanceLoad(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockShardManager) AssignResource(ctx context.Context, resource *interfaces.Resource) (string, error) {
	args := m.Called(ctx, resource)
	return args.String(0), args.Error(1)
}

func (m *MockShardManager) CheckShardHealth(ctx context.Context, shardId string) (*shardv1.HealthStatus, error) {
	args := m.Called(ctx, shardId)
	return args.Get(0).(*shardv1.HealthStatus), args.Error(1)
}

func (m *MockShardManager) HandleFailedShard(ctx context.Context, shardId string) error {
	args := m.Called(ctx, shardId)
	return args.Error(0)
}

func (m *MockShardManager) GetShardStatus(ctx context.Context, shardId string) (*shardv1.ShardInstanceStatus, error) {
	args := m.Called(ctx, shardId)
	return args.Get(0).(*shardv1.ShardInstanceStatus), args.Error(1)
}

func (m *MockShardManager) ListShards(ctx context.Context) ([]*shardv1.ShardInstance, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*shardv1.ShardInstance), args.Error(1)
}

// Test helper functions

func createTestShard(id string, phase shardv1.ShardPhase, load float64) *shardv1.ShardInstance {
	return &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
		},
		Spec: shardv1.ShardInstanceSpec{
			ShardID: id,
		},
		Status: shardv1.ShardInstanceStatus{
			Phase: phase,
			Load:  load,
			HealthStatus: &shardv1.HealthStatus{
				Healthy:   phase == shardv1.ShardPhaseRunning,
				LastCheck: metav1.Now(),
			},
		},
	}
}

func createTestShardStatus(phase shardv1.ShardPhase, load float64) *shardv1.ShardInstanceStatus {
	return &shardv1.ShardInstanceStatus{
		Phase: phase,
		Load:  load,
		HealthStatus: &shardv1.HealthStatus{
			Healthy:   phase == shardv1.ShardPhaseRunning,
			LastCheck: metav1.Now(),
		},
	}
}

func createTestResources(count int) []*interfaces.Resource {
	resources := make([]*interfaces.Resource, count)
	for i := 0; i < count; i++ {
		resources[i] = &interfaces.Resource{
			ID:   fmt.Sprintf("resource-%d", i),
			Type: "test-resource",
			Data: map[string]string{
				"index": fmt.Sprintf("%d", i),
			},
		}
	}
	return resources
}

// Test cases

func TestNewResourceMigrator(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	assert.NotNil(t, migrator)
	assert.Equal(t, mockShardManager, migrator.shardManager)
	assert.Equal(t, 3, migrator.maxRetries)
	assert.Equal(t, time.Second*30, migrator.retryInterval)
	assert.Equal(t, time.Minute*10, migrator.migrationTimeout)
	assert.NotNil(t, migrator.activeMigrations)
	assert.NotNil(t, migrator.migrationHistory)
}

func TestCreateMigrationPlan_Success(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	sourceShard := "shard-1"
	targetShard := "shard-2"
	resources := createTestResources(5)
	
	// Mock shard status calls
	sourceStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.5)
	targetStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.3)
	
	mockShardManager.On("GetShardStatus", ctx, sourceShard).Return(sourceStatus, nil)
	mockShardManager.On("GetShardStatus", ctx, targetShard).Return(targetStatus, nil)
	
	plan, err := migrator.CreateMigrationPlan(ctx, sourceShard, targetShard, resources)
	
	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, sourceShard, plan.SourceShard)
	assert.Equal(t, targetShard, plan.TargetShard)
	assert.Equal(t, 5, len(plan.Resources))
	assert.Equal(t, shardv1.MigrationPriorityLow, plan.Priority)
	
	mockShardManager.AssertExpectations(t)
}

func TestCreateMigrationPlan_SameSourceAndTarget(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	shard := "shard-1"
	resources := createTestResources(1)
	
	plan, err := migrator.CreateMigrationPlan(ctx, shard, shard, resources)
	
	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "source and target shards cannot be the same")
}

func TestCreateMigrationPlan_NoResources(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	resources := []*interfaces.Resource{}
	
	plan, err := migrator.CreateMigrationPlan(ctx, "shard-1", "shard-2", resources)
	
	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "no resources specified")
}

func TestCreateMigrationPlan_SourceShardNotFound(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	resources := createTestResources(1)
	
	mockShardManager.On("GetShardStatus", ctx, "shard-1").Return((*shardv1.ShardInstanceStatus)(nil), fmt.Errorf("shard not found"))
	
	plan, err := migrator.CreateMigrationPlan(ctx, "shard-1", "shard-2", resources)
	
	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "failed to get source shard status")
	
	mockShardManager.AssertExpectations(t)
}

func TestCreateMigrationPlan_TargetShardOverloaded(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	resources := createTestResources(10)
	
	sourceStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.5)
	targetStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.9) // High load
	
	mockShardManager.On("GetShardStatus", ctx, "shard-1").Return(sourceStatus, nil)
	mockShardManager.On("GetShardStatus", ctx, "shard-2").Return(targetStatus, nil)
	
	plan, err := migrator.CreateMigrationPlan(ctx, "shard-1", "shard-2", resources)
	
	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "cannot accept")
	
	mockShardManager.AssertExpectations(t)
}

func TestCreateMigrationPlan_HighPriorityForFailedShard(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	resources := createTestResources(3)
	
	sourceStatus := createTestShardStatus(shardv1.ShardPhaseFailed, 0.5)
	targetStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.3)
	
	mockShardManager.On("GetShardStatus", ctx, "shard-1").Return(sourceStatus, nil)
	mockShardManager.On("GetShardStatus", ctx, "shard-2").Return(targetStatus, nil)
	
	plan, err := migrator.CreateMigrationPlan(ctx, "shard-1", "shard-2", resources)
	
	require.NoError(t, err)
	assert.Equal(t, shardv1.MigrationPriorityHigh, plan.Priority)
	
	mockShardManager.AssertExpectations(t)
}

func TestExecuteMigration_Success(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	plan := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1", "resource-2"},
		Priority:    shardv1.MigrationPriorityMedium,
	}
	
	// Mock the ListShards call that will be made during execution
	sourceShard := createTestShard("shard-1", shardv1.ShardPhaseRunning, 0.5)
	targetShard := createTestShard("shard-2", shardv1.ShardPhaseRunning, 0.3)
	shards := []*shardv1.ShardInstance{sourceShard, targetShard}
	
	mockShardManager.On("ListShards", mock.AnythingOfType("*context.timerCtx")).Return(shards, nil)
	
	err := migrator.ExecuteMigration(ctx, plan)
	
	assert.NoError(t, err)
	
	// Wait a bit for async execution to start
	time.Sleep(time.Millisecond * 200)
	
	// Check that migration is tracked (either active or completed)
	activeMigrations := migrator.GetActiveMigrations()
	history := migrator.GetMigrationHistory()
	totalMigrations := len(activeMigrations) + len(history)
	assert.Equal(t, 1, totalMigrations)
	
	// Wait for migration to complete if still active
	if len(activeMigrations) > 0 {
		time.Sleep(time.Second * 3)
		
		// Check migration history after completion
		history = migrator.GetMigrationHistory()
		assert.Equal(t, 1, len(history))
	}
	
	mockShardManager.AssertExpectations(t)
}

func TestExecuteMigration_DuplicateExecution(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	plan := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1"},
		Priority:    shardv1.MigrationPriorityLow,
	}
	
	// Mock the ListShards call
	sourceShard := createTestShard("shard-1", shardv1.ShardPhaseRunning, 0.5)
	targetShard := createTestShard("shard-2", shardv1.ShardPhaseRunning, 0.3)
	shards := []*shardv1.ShardInstance{sourceShard, targetShard}
	
	mockShardManager.On("ListShards", mock.AnythingOfType("*context.timerCtx")).Return(shards, nil)
	
	// Execute first migration
	err1 := migrator.ExecuteMigration(ctx, plan)
	assert.NoError(t, err1)
	
	// Try to execute the same plan again
	err2 := migrator.ExecuteMigration(ctx, plan)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "already being executed")
}

func TestGetMigrationStatus_NotFound(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	status, err := migrator.GetMigrationStatus(ctx, "non-existent-plan")
	
	assert.Error(t, err)
	assert.Empty(t, status)
	assert.Contains(t, err.Error(), "not found")
}

func TestRollbackMigration_ActiveMigration(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	plan := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1"},
		Priority:    shardv1.MigrationPriorityLow,
	}
	
	// Mock the ListShards call
	sourceShard := createTestShard("shard-1", shardv1.ShardPhaseRunning, 0.5)
	targetShard := createTestShard("shard-2", shardv1.ShardPhaseRunning, 0.3)
	shards := []*shardv1.ShardInstance{sourceShard, targetShard}
	
	mockShardManager.On("ListShards", mock.AnythingOfType("*context.timerCtx")).Return(shards, nil)
	
	// Start migration
	err := migrator.ExecuteMigration(ctx, plan)
	require.NoError(t, err)
	
	// Wait for migration to start and get plan ID
	var planID string
	var activeMigrations map[string]*MigrationExecution
	
	// Try multiple times to find the active migration
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		activeMigrations = migrator.GetActiveMigrations()
		if len(activeMigrations) > 0 {
			for id := range activeMigrations {
				planID = id
				break
			}
			break
		}
	}
	
	// If not found in active, check history (migration might have completed quickly)
	if planID == "" {
		history := migrator.GetMigrationHistory()
		require.True(t, len(history) > 0, "Migration should be either active or in history")
		for id := range history {
			planID = id
			break
		}
	}
	
	// Rollback active migration
	err = migrator.RollbackMigration(ctx, planID)
	assert.NoError(t, err)
	
	// Check that migration was cancelled
	status, err := migrator.GetMigrationStatus(ctx, planID)
	assert.NoError(t, err)
	assert.Equal(t, interfaces.MigrationStatusRolledBack, status)
}

func TestRollbackMigration_NotFound(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	err := migrator.RollbackMigration(ctx, "non-existent-plan")
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSetConfiguration(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	// Set new configuration
	newMaxRetries := 5
	newRetryInterval := time.Minute
	newMigrationTimeout := time.Minute * 20
	
	migrator.SetConfiguration(newMaxRetries, newRetryInterval, newMigrationTimeout)
	
	assert.Equal(t, newMaxRetries, migrator.maxRetries)
	assert.Equal(t, newRetryInterval, migrator.retryInterval)
	assert.Equal(t, newMigrationTimeout, migrator.migrationTimeout)
}

func TestCalculateBatchSize(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	testCases := []struct {
		totalResources int
		expectedBatch  int
	}{
		{5, 5},
		{10, 10},
		{50, 10},
		{100, 10},
		{200, 20},
	}
	
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("total_%d", tc.totalResources), func(t *testing.T) {
			batchSize := migrator.calculateBatchSize(tc.totalResources)
			assert.Equal(t, tc.expectedBatch, batchSize)
		})
	}
}

func TestDetermineMigrationPriority(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	testCases := []struct {
		name             string
		phase            shardv1.ShardPhase
		load             float64
		expectedPriority shardv1.MigrationPriority
	}{
		{"failed_shard", shardv1.ShardPhaseFailed, 0.5, shardv1.MigrationPriorityHigh},
		{"high_load", shardv1.ShardPhaseRunning, 0.9, shardv1.MigrationPriorityMedium},
		{"normal_load", shardv1.ShardPhaseRunning, 0.5, shardv1.MigrationPriorityLow},
		{"low_load", shardv1.ShardPhaseRunning, 0.2, shardv1.MigrationPriorityLow},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := createTestShardStatus(tc.phase, tc.load)
			priority := migrator.determineMigrationPriority(status)
			assert.Equal(t, tc.expectedPriority, priority)
		})
	}
}

func TestCanAcceptResources(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	testCases := []struct {
		name          string
		currentLoad   float64
		resourceCount int
		canAccept     bool
	}{
		{"low_load_few_resources", 0.3, 5, true},
		{"medium_load_few_resources", 0.5, 10, true},
		{"high_load_few_resources", 0.7, 5, true},
		{"high_load_many_resources", 0.7, 20, false},
		{"very_high_load", 0.9, 1, false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := createTestShardStatus(shardv1.ShardPhaseRunning, tc.currentLoad)
			canAccept := migrator.canAcceptResources(status, tc.resourceCount)
			assert.Equal(t, tc.canAccept, canAccept)
		})
	}
}

func TestEstimateMigrationTime(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	testCases := []struct {
		resourceCount int
		minDuration   time.Duration
	}{
		{1, time.Second * 10},
		{10, time.Second * 15},
		{100, time.Second * 60}, // 10 base + 50 for resources = 60 seconds
	}
	
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("resources_%d", tc.resourceCount), func(t *testing.T) {
			resources := createTestResources(tc.resourceCount)
			duration := migrator.estimateMigrationTime(resources)
			assert.True(t, duration.Duration >= tc.minDuration)
		})
	}
}

// Benchmark tests

func BenchmarkCreateMigrationPlan(b *testing.B) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	resources := createTestResources(100)
	
	sourceStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.5)
	targetStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.3)
	
	mockShardManager.On("GetShardStatus", ctx, "shard-1").Return(sourceStatus, nil)
	mockShardManager.On("GetShardStatus", ctx, "shard-2").Return(targetStatus, nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := migrator.CreateMigrationPlan(ctx, "shard-1", "shard-2", resources)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCalculateBatchSize(b *testing.B) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		migrator.calculateBatchSize(1000)
	}
}

// Test MigrationScheduler

func TestMigrationScheduler_AddPlan(t *testing.T) {
	scheduler := NewMigrationScheduler(2)
	
	highPriorityPlan := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1"},
		Priority:    shardv1.MigrationPriorityHigh,
	}
	
	lowPriorityPlan := &shardv1.MigrationPlan{
		SourceShard: "shard-3",
		TargetShard: "shard-4",
		Resources:   []string{"resource-2"},
		Priority:    shardv1.MigrationPriorityLow,
	}
	
	// Add low priority first
	scheduler.AddPlan(lowPriorityPlan)
	scheduler.AddPlan(highPriorityPlan)
	
	// High priority should come first
	nextPlan := scheduler.GetNextPlan()
	assert.Equal(t, shardv1.MigrationPriorityHigh, nextPlan.Priority)
	
	// Low priority should come second
	nextPlan = scheduler.GetNextPlan()
	assert.Equal(t, shardv1.MigrationPriorityLow, nextPlan.Priority)
}

func TestMigrationScheduler_MaxConcurrent(t *testing.T) {
	scheduler := NewMigrationScheduler(1)
	
	plan1 := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1"},
		Priority:    shardv1.MigrationPriorityMedium,
	}
	
	plan2 := &shardv1.MigrationPlan{
		SourceShard: "shard-3",
		TargetShard: "shard-4",
		Resources:   []string{"resource-2"},
		Priority:    shardv1.MigrationPriorityMedium,
	}
	
	scheduler.AddPlan(plan1)
	scheduler.AddPlan(plan2)
	
	// Should get first plan
	nextPlan := scheduler.GetNextPlan()
	assert.NotNil(t, nextPlan)
	
	// Should not get second plan due to max concurrent limit
	nextPlan = scheduler.GetNextPlan()
	assert.Nil(t, nextPlan)
	
	// Mark first as completed
	scheduler.MarkCompleted()
	
	// Now should get second plan
	nextPlan = scheduler.GetNextPlan()
	assert.NotNil(t, nextPlan)
}

func TestScheduleMigration(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	plan := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1"},
		Priority:    shardv1.MigrationPriorityMedium,
	}
	
	migrator.ScheduleMigration(plan)
	
	queued, running, maxConcurrent := migrator.GetSchedulerStatus()
	assert.Equal(t, 1, queued)
	assert.Equal(t, 0, running)
	assert.Equal(t, 3, maxConcurrent)
}

func TestValidateMigrationPlan_Success(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	plan := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1"},
		Priority:    shardv1.MigrationPriorityMedium,
	}
	
	sourceStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.5)
	targetStatus := createTestShardStatus(shardv1.ShardPhaseRunning, 0.3)
	
	mockShardManager.On("GetShardStatus", ctx, "shard-1").Return(sourceStatus, nil)
	mockShardManager.On("GetShardStatus", ctx, "shard-2").Return(targetStatus, nil)
	
	err := migrator.ValidateMigrationPlan(ctx, plan)
	
	assert.NoError(t, err)
	mockShardManager.AssertExpectations(t)
}

func TestValidateMigrationPlan_InvalidStates(t *testing.T) {
	ctx := context.Background()
	
	testCases := []struct {
		name         string
		plan         *shardv1.MigrationPlan
		sourcePhase  shardv1.ShardPhase
		targetPhase  shardv1.ShardPhase
		expectedErr  string
	}{
		{
			name: "nil_plan",
			plan: nil,
			expectedErr: "migration plan cannot be nil",
		},
		{
			name: "empty_source",
			plan: &shardv1.MigrationPlan{
				SourceShard: "",
				TargetShard: "shard-2",
				Resources:   []string{"resource-1"},
			},
			expectedErr: "source shard cannot be empty",
		},
		{
			name: "same_source_target",
			plan: &shardv1.MigrationPlan{
				SourceShard: "shard-1",
				TargetShard: "shard-1",
				Resources:   []string{"resource-1"},
			},
			expectedErr: "source and target shards cannot be the same",
		},
		{
			name: "no_resources",
			plan: &shardv1.MigrationPlan{
				SourceShard: "shard-1",
				TargetShard: "shard-2",
				Resources:   []string{},
			},
			expectedErr: "no resources specified",
		},
		{
			name: "source_not_running",
			plan: &shardv1.MigrationPlan{
				SourceShard: "shard-1",
				TargetShard: "shard-2",
				Resources:   []string{"resource-1"},
			},
			sourcePhase: shardv1.ShardPhaseFailed,
			targetPhase: shardv1.ShardPhaseRunning,
			expectedErr: "not in a valid state for migration",
		},
		{
			name: "target_not_running",
			plan: &shardv1.MigrationPlan{
				SourceShard: "shard-1",
				TargetShard: "shard-2",
				Resources:   []string{"resource-1"},
			},
			sourcePhase: shardv1.ShardPhaseRunning,
			targetPhase: shardv1.ShardPhaseFailed,
			expectedErr: "is not running",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fresh mock for each test case
			mockShardManager := &MockShardManager{}
			migrator := NewResourceMigrator(mockShardManager)
			
			if tc.plan != nil && tc.plan.SourceShard != "" && tc.plan.TargetShard != "" && tc.plan.SourceShard != tc.plan.TargetShard && len(tc.plan.Resources) > 0 {
				sourceStatus := createTestShardStatus(tc.sourcePhase, 0.5)
				targetStatus := createTestShardStatus(tc.targetPhase, 0.3)
				
				mockShardManager.On("GetShardStatus", ctx, tc.plan.SourceShard).Return(sourceStatus, nil)
				// Always mock target shard call since validation always checks both
				mockShardManager.On("GetShardStatus", ctx, tc.plan.TargetShard).Return(targetStatus, nil)
			}
			
			err := migrator.ValidateMigrationPlan(ctx, tc.plan)
			
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestGetMigrationProgress(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	plan := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1", "resource-2"},
		Priority:    shardv1.MigrationPriorityMedium,
	}
	
	// Mock the ListShards call
	sourceShard := createTestShard("shard-1", shardv1.ShardPhaseRunning, 0.5)
	targetShard := createTestShard("shard-2", shardv1.ShardPhaseRunning, 0.3)
	shards := []*shardv1.ShardInstance{sourceShard, targetShard}
	
	mockShardManager.On("ListShards", mock.AnythingOfType("*context.timerCtx")).Return(shards, nil)
	
	// Start migration
	err := migrator.ExecuteMigration(ctx, plan)
	require.NoError(t, err)
	
	// Wait for migration to start
	time.Sleep(time.Millisecond * 100)
	
	// Get plan ID from active migrations or history
	var planID string
	var activeMigrations map[string]*MigrationExecution
	
	// Try multiple times to find the migration
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		activeMigrations = migrator.GetActiveMigrations()
		if len(activeMigrations) > 0 {
			for id := range activeMigrations {
				planID = id
				break
			}
			break
		}
	}
	
	// If not found in active, check history (migration might have completed quickly)
	if planID == "" {
		history := migrator.GetMigrationHistory()
		require.True(t, len(history) > 0, "Migration should be either active or in history")
		for id := range history {
			planID = id
			break
		}
	}
	
	// Get migration progress
	progress, err := migrator.GetMigrationProgress(ctx, planID)
	require.NoError(t, err)
	assert.NotNil(t, progress)
	assert.Equal(t, planID, progress.PlanID)
	assert.Equal(t, 2, progress.TotalResources)
	assert.True(t, progress.Duration != nil)
}

func TestGetMigrationStatistics(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	// Initially should have zero stats
	stats := migrator.GetMigrationStatistics()
	assert.Equal(t, 0, stats.ActiveMigrations)
	assert.Equal(t, 0, stats.CompletedMigrations)
	assert.Equal(t, 0, stats.FailedMigrations)
	assert.Equal(t, 0, stats.TotalMigrations)
	assert.Equal(t, float64(0), stats.SuccessRate)
	
	// Add some mock migration history
	migrator.mu.Lock()
	migrator.migrationHistory["test-1"] = &MigrationExecution{
		Status:            interfaces.MigrationStatusCompleted,
		MigratedResources: 5,
	}
	migrator.migrationHistory["test-2"] = &MigrationExecution{
		Status:            interfaces.MigrationStatusFailed,
		MigratedResources: 0,
	}
	migrator.mu.Unlock()
	
	stats = migrator.GetMigrationStatistics()
	assert.Equal(t, 0, stats.ActiveMigrations)
	assert.Equal(t, 1, stats.CompletedMigrations)
	assert.Equal(t, 1, stats.FailedMigrations)
	assert.Equal(t, 2, stats.TotalMigrations)
	assert.Equal(t, 5, stats.TotalResourcesMigrated)
	assert.Equal(t, float64(50), stats.SuccessRate)
}

func TestCancelMigration(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	plan := &shardv1.MigrationPlan{
		SourceShard: "shard-1",
		TargetShard: "shard-2",
		Resources:   []string{"resource-1"},
		Priority:    shardv1.MigrationPriorityMedium,
	}
	
	// Mock the ListShards call
	sourceShard := createTestShard("shard-1", shardv1.ShardPhaseRunning, 0.5)
	targetShard := createTestShard("shard-2", shardv1.ShardPhaseRunning, 0.3)
	shards := []*shardv1.ShardInstance{sourceShard, targetShard}
	
	mockShardManager.On("ListShards", mock.AnythingOfType("*context.timerCtx")).Return(shards, nil)
	
	// Start migration
	err := migrator.ExecuteMigration(ctx, plan)
	require.NoError(t, err)
	
	// Wait for migration to start
	time.Sleep(time.Millisecond * 100)
	
	// Get plan ID from active migrations or history
	var planID string
	var activeMigrations map[string]*MigrationExecution
	
	// Try multiple times to find the migration
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		activeMigrations = migrator.GetActiveMigrations()
		if len(activeMigrations) > 0 {
			for id := range activeMigrations {
				planID = id
				break
			}
			break
		}
	}
	
	// If not found in active, check history (migration might have completed quickly)
	if planID == "" {
		history := migrator.GetMigrationHistory()
		require.True(t, len(history) > 0, "Migration should be either active or in history")
		for id := range history {
			planID = id
			break
		}
	}
	
	// Cancel migration (this will only work if it's still active)
	err = migrator.CancelMigration(ctx, planID)
	if len(activeMigrations) > 0 {
		// Migration was active, should be able to cancel
		assert.NoError(t, err)
		
		// Check status
		status, err := migrator.GetMigrationStatus(ctx, planID)
		assert.NoError(t, err)
		assert.Equal(t, interfaces.MigrationStatusRolledBack, status)
	} else {
		// Migration already completed, should get error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found or not active")
	}
}

func TestCancelMigration_NotFound(t *testing.T) {
	mockShardManager := &MockShardManager{}
	migrator := NewResourceMigrator(mockShardManager)
	
	ctx := context.Background()
	err := migrator.CancelMigration(ctx, "non-existent-plan")
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found or not active")
}