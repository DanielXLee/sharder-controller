package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
)

func TestNewHealthChecker(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, err := NewHealthChecker(cl, cfg)
	require.NoError(t, err)
	assert.NotNil(t, hc)
}

func TestCheckHealth(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Test healthy shard
	healthyShard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "healthy-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "healthy-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Now(),
			HealthStatus:  &shardv1.HealthStatus{Healthy: true},
		},
	}
	healthStatus, err := hc.CheckHealth(context.Background(), healthyShard)
	assert.NoError(t, err)
	assert.True(t, healthStatus.Healthy)

	// Test unhealthy shard (stale heartbeat)
	unhealthyShard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "unhealthy-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "unhealthy-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
		},
	}
	healthStatus, err = hc.CheckHealth(context.Background(), unhealthyShard)
	assert.NoError(t, err)
	assert.False(t, healthStatus.Healthy)
}

func TestStartStopHealthChecking(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	err := hc.StartHealthChecking(context.Background(), 10*time.Millisecond)
	assert.NoError(t, err)
	assert.True(t, hc.isChecking)

	err = hc.StopHealthChecking()
	assert.NoError(t, err)
	assert.False(t, hc.isChecking)
}

func TestCheckHealthWithFailureThreshold(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck
	cfg.FailureThreshold = 3

	hc, _ := NewHealthChecker(cl, cfg)

	// Create a shard with stale heartbeat
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "test-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
		},
	}

	// First check - should be unhealthy with error count 1
	healthStatus, err := hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)
	assert.False(t, healthStatus.Healthy)
	assert.Equal(t, 1, healthStatus.ErrorCount)

	// Second check - should increment error count
	healthStatus, err = hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)
	assert.False(t, healthStatus.Healthy)
	assert.Equal(t, 2, healthStatus.ErrorCount)

	// Third check - should reach threshold
	healthStatus, err = hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)
	assert.False(t, healthStatus.Healthy)
	assert.Equal(t, 3, healthStatus.ErrorCount)
}

func TestCheckHealthRecovery(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "test-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
		},
	}

	// First check - unhealthy
	healthStatus, err := hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)
	assert.False(t, healthStatus.Healthy)
	assert.Equal(t, 1, healthStatus.ErrorCount)

	// Update heartbeat to make it healthy
	shard.Status.LastHeartbeat = metav1.Now()

	// Second check - should recover
	healthStatus, err = hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)
	assert.True(t, healthStatus.Healthy)
	assert.Equal(t, 0, healthStatus.ErrorCount)
}

func TestCheckHealthOverloadedShard(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Create a shard with high load
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "overloaded-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "overloaded-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Now(),
			Load:          1.5, // Overloaded
		},
	}

	healthStatus, err := hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)
	assert.False(t, healthStatus.Healthy)
	assert.Contains(t, healthStatus.Message, "overloaded")
}

func TestCheckHealthUnhealthyPhase(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Test different unhealthy phases
	phases := []shardv1.ShardPhase{
		shardv1.ShardPhasePending,
		shardv1.ShardPhaseFailed,
		shardv1.ShardPhaseDraining,
		shardv1.ShardPhaseTerminated,
	}

	for _, phase := range phases {
		shard := &shardv1.ShardInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "test-shard"},
			Spec:       shardv1.ShardInstanceSpec{ShardID: "test-shard"},
			Status: shardv1.ShardInstanceStatus{
				Phase:         phase,
				LastHeartbeat: metav1.Now(),
			},
		}

		healthStatus, err := hc.CheckHealth(context.Background(), shard)
		assert.NoError(t, err)
		assert.False(t, healthStatus.Healthy, "Phase %s should be unhealthy", phase)
		assert.Contains(t, healthStatus.Message, "unhealthy phase")
	}
}

func TestReportHeartbeat(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = shardv1.AddToScheme(scheme)
	
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "test-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(shard).Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Report heartbeat
	err := hc.ReportHeartbeat(context.Background(), "test-shard")
	assert.NoError(t, err)

	// Verify heartbeat was updated
	updatedShard := &shardv1.ShardInstance{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: "test-shard"}, updatedShard)
	assert.NoError(t, err)
	
	// Heartbeat should be recent (within last minute)
	assert.True(t, time.Since(updatedShard.Status.LastHeartbeat.Time) < time.Minute)
}

func TestGetShardHealthStatus(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Initially no status should exist
	status, exists := hc.GetShardHealthStatus("test-shard")
	assert.False(t, exists)
	assert.Nil(t, status)

	// Check health to create status
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "test-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Now(),
		},
	}

	_, err := hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)

	// Now status should exist
	status, exists = hc.GetShardHealthStatus("test-shard")
	assert.True(t, exists)
	assert.NotNil(t, status)
	assert.True(t, status.Healthy)
}

func TestFailureAndRecoveryHistory(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "test-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
		},
	}

	// Check health to trigger failure
	_, err := hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)

	// Verify failure history
	failures := hc.GetFailureHistory("test-shard")
	assert.Len(t, failures, 1)

	// Make shard healthy
	shard.Status.LastHeartbeat = metav1.Now()

	// Check health to trigger recovery
	_, err = hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)

	// Verify recovery history
	recoveries := hc.GetRecoveryHistory("test-shard")
	assert.Len(t, recoveries, 1)
}

func TestGetHealthSummary(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Create multiple shards with different health states
	healthyShard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "healthy-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "healthy-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Now(),
		},
	}

	unhealthyShard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "unhealthy-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "unhealthy-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
		},
	}

	// Check health for both shards
	_, err := hc.CheckHealth(context.Background(), healthyShard)
	assert.NoError(t, err)
	_, err = hc.CheckHealth(context.Background(), unhealthyShard)
	assert.NoError(t, err)

	// Get summary
	summary := hc.GetHealthSummary()
	assert.Len(t, summary, 2)
	assert.True(t, summary["healthy-shard"].Healthy)
	assert.False(t, summary["unhealthy-shard"].Healthy)
}

func TestGetUnhealthyShards(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Initially no unhealthy shards
	unhealthy := hc.GetUnhealthyShards()
	assert.Empty(t, unhealthy)

	// Add unhealthy shard
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "unhealthy-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "unhealthy-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
		},
	}

	_, err := hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)

	// Should now have one unhealthy shard
	unhealthy = hc.GetUnhealthyShards()
	assert.Len(t, unhealthy, 1)
	assert.Contains(t, unhealthy, "unhealthy-shard")
}

func TestCleanupShardData(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Add shard data
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "test-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Now(),
		},
	}

	_, err := hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)

	// Verify data exists
	_, exists := hc.GetShardHealthStatus("test-shard")
	assert.True(t, exists)

	// Cleanup
	hc.CleanupShardData("test-shard")

	// Verify data is gone
	_, exists = hc.GetShardHealthStatus("test-shard")
	assert.False(t, exists)
}

func TestCallbacks(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	var failureCalled, recoveryCalled bool
	var failedShardId, recoveredShardId string
	_ = failureCalled
	_ = recoveryCalled
	_ = failedShardId
	_ = recoveredShardId

	// Set callbacks
	hc.SetFailureCallback(func(ctx context.Context, shardId string) error {
		failureCalled = true
		failedShardId = shardId
		return nil
	})

	hc.SetRecoveryCallback(func(ctx context.Context, shardId string) error {
		recoveryCalled = true
		recoveredShardId = shardId
		return nil
	})

	// Test that callbacks are set
	hc.mu.RLock()
	assert.NotNil(t, hc.onShardFailedCallback)
	assert.NotNil(t, hc.onShardRecoveredCallback)
	hc.mu.RUnlock()

	// Note: Full callback testing would require integration with the state transition logic
	// which is tested in the checkAndUpdateShardHealth method
}

func TestIsShardHealthy(t *testing.T) {
	cl := fake.NewClientBuilder().Build()
	cfg := config.DefaultConfig().HealthCheck

	hc, _ := NewHealthChecker(cl, cfg)

	// Non-existent shard should be unhealthy
	assert.False(t, hc.IsShardHealthy("non-existent"))

	// Add healthy shard
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-shard"},
		Spec:       shardv1.ShardInstanceSpec{ShardID: "test-shard"},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhaseRunning,
			LastHeartbeat: metav1.Now(),
		},
	}

	_, err := hc.CheckHealth(context.Background(), shard)
	assert.NoError(t, err)

	assert.True(t, hc.IsShardHealthy("test-shard"))
}
