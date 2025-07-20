package performance

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/controllers"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// TestBasicPerformance tests basic performance characteristics
func TestBasicPerformance(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()

	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := NewMockConfig("test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(t, err)

	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           1,
			MaxShards:           100,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}

	// Test shard creation performance
	start := time.Now()
	_, err = shardManager.CreateShard(ctx, shardConfig)
	duration := time.Since(start)

	require.NoError(t, err)
	t.Logf("Shard creation took: %v", duration)

	// Should be reasonably fast in test environment
	require.Less(t, duration, 1*time.Second, "Shard creation should be fast")
}

// BenchmarkShardCreation benchmarks shard creation
func BenchmarkShardCreation(b *testing.B) {
	ctx := context.Background()

	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()

	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := NewMockConfig("test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(b, err)

	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           1,
			MaxShards:           100,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := shardManager.CreateShard(ctx, shardConfig)
		if err != nil {
			b.Fatalf("Failed to create shard: %v", err)
		}
	}
}

// BenchmarkResourceAssignment benchmarks resource assignment to shards
func BenchmarkResourceAssignment(b *testing.B) {
	ctx := context.Background()

	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()

	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := NewMockConfig("test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(b, err)

	// Create some test shards first
	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           1,
			MaxShards:           10,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}

	// Create a few shards for testing
	for i := 0; i < 3; i++ {
		_, err := shardManager.CreateShard(ctx, shardConfig)
		require.NoError(b, err)
	}

	// Create test resource
	resource := &interfaces.Resource{
		ID:   "test-resource",
		Type: "test",
		Data: map[string]string{"key": "value"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resource.ID = fmt.Sprintf("test-resource-%d", i) // Make each resource unique
		_, err := shardManager.AssignResource(ctx, resource)
		if err != nil {
			b.Fatalf("Failed to assign resource: %v", err)
		}
	}
}

// BenchmarkConcurrentResourceAssignment benchmarks concurrent resource assignment
func BenchmarkConcurrentResourceAssignment(b *testing.B) {
	ctx := context.Background()

	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()

	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := NewMockConfig("test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			resource := &interfaces.Resource{
				ID:   fmt.Sprintf("test-resource-%d", i),
				Type: "test",
				Data: map[string]string{"key": fmt.Sprintf("value-%d", i)},
			}

			_, err := shardManager.AssignResource(ctx, resource)
			if err != nil {
				b.Fatalf("Failed to assign resource: %v", err)
			}
			i++
		}
	})
}

// BenchmarkResourceMigration benchmarks resource migration between shards
func BenchmarkResourceMigration(b *testing.B) {
	ctx := context.Background()

	// Create mock resource migrator with realistic timing
	resourceMigrator := &MockResourceMigrator{}

	// Create test resources
	resources := make([]*interfaces.Resource, 100)
	for i := 0; i < 100; i++ {
		resources[i] = &interfaces.Resource{
			ID:   fmt.Sprintf("resource-%d", i),
			Type: "test",
			Data: map[string]string{"key": fmt.Sprintf("value-%d", i)},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		plan, err := resourceMigrator.CreateMigrationPlan(ctx, "shard-1", "shard-2", resources)
		if err != nil {
			b.Fatalf("Failed to create migration plan: %v", err)
		}

		err = resourceMigrator.ExecuteMigration(ctx, plan)
		if err != nil {
			b.Fatalf("Failed to execute migration: %v", err)
		}
	}
}

// BenchmarkLoadBalancerCalculation benchmarks load balancer calculations
func BenchmarkLoadBalancerCalculation(b *testing.B) {
	loadBalancer := &MockLoadBalancer{}

	// Create test shards
	shards := make([]*shardv1.ShardInstance, 10)
	for i := 0; i < 10; i++ {
		shards[i] = &shardv1.ShardInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("shard-%d", i),
			},
			Status: shardv1.ShardInstanceStatus{
				Load: float64(i) / 10.0,
			},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := loadBalancer.GetOptimalShard(shards)
		if err != nil {
			b.Fatalf("Failed to get optimal shard: %v", err)
		}
	}
}

// TestShardStartupTime tests shard startup performance
func TestShardStartupTime(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()

	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := NewMockConfig("test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(t, err)

	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           1,
			MaxShards:           100,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}

	// Test multiple shard creations and measure startup times
	startupTimes := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		start := time.Now()
		_, err := shardManager.CreateShard(ctx, shardConfig)
		startupTimes[i] = time.Since(start)
		require.NoError(t, err)
	}

	// Calculate statistics
	var total time.Duration
	var max time.Duration
	min := startupTimes[0]

	for _, duration := range startupTimes {
		total += duration
		if duration > max {
			max = duration
		}
		if duration < min {
			min = duration
		}
	}

	avg := total / time.Duration(len(startupTimes))

	t.Logf("Shard startup times - Avg: %v, Min: %v, Max: %v", avg, min, max)

	// Assert reasonable startup times (should be under 1 second in test environment)
	require.Less(t, avg, 1*time.Second, "Average startup time should be reasonable")
	require.Less(t, max, 2*time.Second, "Maximum startup time should be reasonable")
}

// TestResourceMigrationSpeed tests resource migration performance
func TestResourceMigrationSpeed(t *testing.T) {
	ctx := context.Background()
	resourceMigrator := &MockResourceMigrator{}

	// Test different resource counts
	resourceCounts := []int{10, 100, 1000}

	for _, count := range resourceCounts {
		t.Run(fmt.Sprintf("Resources_%d", count), func(t *testing.T) {
			// Create test resources
			resources := make([]*interfaces.Resource, count)
			for i := 0; i < count; i++ {
				resources[i] = &interfaces.Resource{
					ID:   fmt.Sprintf("resource-%d", i),
					Type: "test",
					Data: map[string]string{
						"key":  fmt.Sprintf("value-%d", i),
						"size": "1024", // 1KB resource
					},
				}
			}

			start := time.Now()
			plan, err := resourceMigrator.CreateMigrationPlan(ctx, "shard-1", "shard-2", resources)
			require.NoError(t, err)

			err = resourceMigrator.ExecuteMigration(ctx, plan)
			require.NoError(t, err)

			duration := time.Since(start)
			throughput := float64(count) / duration.Seconds()

			t.Logf("Migrated %d resources in %v (%.2f resources/sec)", count, duration, throughput)

			// Assert reasonable migration speed
			require.Greater(t, throughput, 100.0, "Migration throughput should be reasonable")
		})
	}
}

// TestMemoryUsageUnderLoad tests memory usage during high load
func TestMemoryUsageUnderLoad(t *testing.T) {
	ctx := context.Background()

	// Start system profiler
	profiler := NewSystemProfiler(100 * time.Millisecond)
	profiler.Start()

	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()

	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := NewMockConfig("test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(t, err)

	// Generate load
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				resource := &interfaces.Resource{
					ID:   fmt.Sprintf("resource-%d-%d", workerID, j),
					Type: "test",
					Data: map[string]string{"key": fmt.Sprintf("value-%d-%d", workerID, j)},
				}
				_, err := shardManager.AssignResource(ctx, resource)
				if err != nil {
					t.Errorf("Failed to assign resource: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Stop profiler and get report
	report := profiler.Stop()

	t.Logf("Memory usage report: %s", report.String())

	// Assert reasonable memory usage (should not exceed 100MB in test)
	require.Less(t, report.MaxMemAllocMB, 100.0, "Memory usage should be reasonable under load")

	// Check for memory leaks (end memory should not be significantly higher than start)
	memoryGrowth := report.EndMemAllocMB - report.StartMemAllocMB
	require.Less(t, memoryGrowth, 50.0, "Memory growth should be limited")
}

// TestConcurrentOperationsStability tests system stability under concurrent operations
func TestConcurrentOperationsStability(t *testing.T) {
	ctx := context.Background()

	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()

	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := NewMockConfig("test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(t, err)

	// Run concurrent operations
	var wg sync.WaitGroup
	errorCh := make(chan error, 100)

	// Resource assignment workers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				resource := &interfaces.Resource{
					ID:   fmt.Sprintf("resource-%d-%d", workerID, j),
					Type: "test",
					Data: map[string]string{"key": fmt.Sprintf("value-%d-%d", workerID, j)},
				}
				_, err := shardManager.AssignResource(ctx, resource)
				if err != nil {
					select {
					case errorCh <- err:
					default:
					}
				}
			}
		}(i)
	}

	// Health check workers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, err := shardManager.CheckShardHealth(ctx, fmt.Sprintf("shard-%d", workerID))
				if err != nil {
					select {
					case errorCh <- err:
					default:
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errorCh)

	// Check for errors
	errorCount := 0
	for err := range errorCh {
		t.Logf("Concurrent operation error: %v", err)
		errorCount++
	}

	// Allow some errors but not too many (should be stable)
	require.Less(t, errorCount, 10, "System should be stable under concurrent operations")
}

// TestLargeScaleShardManagement tests performance with large number of shards
func TestLargeScaleShardManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large scale test in short mode")
	}

	ctx := context.Background()

	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()

	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := NewMockConfig("test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(t, err)

	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           1,
			MaxShards:           1000,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}

	// Create many shards and measure performance
	shardCount := 100
	start := time.Now()

	for i := 0; i < shardCount; i++ {
		_, err := shardManager.CreateShard(ctx, shardConfig)
		require.NoError(t, err)
	}

	creationDuration := time.Since(start)

	// Test resource assignment with many shards
	start = time.Now()
	for i := 0; i < 1000; i++ {
		resource := &interfaces.Resource{
			ID:   fmt.Sprintf("resource-%d", i),
			Type: "test",
			Data: map[string]string{"key": fmt.Sprintf("value-%d", i)},
		}
		_, err := shardManager.AssignResource(ctx, resource)
		require.NoError(t, err)
	}
	assignmentDuration := time.Since(start)

	t.Logf("Created %d shards in %v (%.2f shards/sec)",
		shardCount, creationDuration, float64(shardCount)/creationDuration.Seconds())
	t.Logf("Assigned 1000 resources in %v (%.2f resources/sec)",
		assignmentDuration, 1000.0/assignmentDuration.Seconds())

	// Assert reasonable performance even at scale
	require.Less(t, creationDuration, 30*time.Second, "Large scale shard creation should complete in reasonable time")
	require.Less(t, assignmentDuration, 10*time.Second, "Large scale resource assignment should complete in reasonable time")
}
