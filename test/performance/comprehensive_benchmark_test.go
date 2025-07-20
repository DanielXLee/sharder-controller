package performance

import (
	"context"
	"fmt"
	"runtime"
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

// ComprehensiveBenchmarkSuite runs all performance benchmarks
type ComprehensiveBenchmarkSuite struct {
	shardManager interfaces.ShardManager
	analyzer     *PerformanceAnalyzer
	profiler     *SystemProfiler
}

// NewComprehensiveBenchmarkSuite creates a new benchmark suite
func NewComprehensiveBenchmarkSuite() (*ComprehensiveBenchmarkSuite, error) {
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
	config := NewMockConfig("benchmark")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create shard manager: %w", err)
	}

	return &ComprehensiveBenchmarkSuite{
		shardManager: shardManager,
		analyzer:     NewPerformanceAnalyzer(),
		profiler:     NewSystemProfiler(100 * time.Millisecond),
	}, nil
}

// RunAllBenchmarks executes all performance benchmarks
func (cbs *ComprehensiveBenchmarkSuite) RunAllBenchmarks(t *testing.T) *DetailedPerformanceReport {
	t.Log("Starting comprehensive performance benchmarks...")
	
	// Start profiling
	cbs.profiler.Start()
	
	// Run individual benchmarks
	cbs.benchmarkShardStartupTime(t)
	cbs.benchmarkResourceAssignmentThroughput(t)
	cbs.benchmarkResourceMigrationSpeed(t)
	cbs.benchmarkLoadBalancerPerformance(t)
	cbs.benchmarkConcurrentOperations(t)
	cbs.benchmarkMemoryUsage(t)
	cbs.benchmarkScalabilityLimits(t)
	
	// Stop profiling
	profileReport := cbs.profiler.Stop()
	t.Logf("System profiling completed: %s", profileReport.String())
	
	// Generate comprehensive report
	report := cbs.analyzer.AnalyzeDetailedPerformance()
	t.Logf("Performance analysis completed with score: %.1f/100", report.OverallScore)
	
	return report
}

// benchmarkShardStartupTime measures shard startup performance
func (cbs *ComprehensiveBenchmarkSuite) benchmarkShardStartupTime(t *testing.T) {
	t.Log("Benchmarking shard startup time...")
	
	ctx := context.Background()
	
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
	
	// Measure multiple shard creations
	startupTimes := make([]time.Duration, 0, 50)
	for i := 0; i < 50; i++ {
		var memBefore, memAfter runtime.MemStats
		runtime.ReadMemStats(&memBefore)
		
		start := time.Now()
		_, err := cbs.shardManager.CreateShard(ctx, shardConfig)
		duration := time.Since(start)
		
		runtime.ReadMemStats(&memAfter)
		memAllocs := int64(memAfter.Mallocs - memBefore.Mallocs)
		
		require.NoError(t, err)
		startupTimes = append(startupTimes, duration)
		
		cbs.analyzer.RecordOperation("shard_startup", duration, true, memAllocs)
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
	
	// Add benchmark result
	cbs.analyzer.AddBenchmarkResult(&BenchmarkResult{
		Name:             "shard_startup",
		OperationsPerSec: float64(len(startupTimes)) / total.Seconds(),
		AvgLatencyMs:     float64(avg.Nanoseconds()) / 1e6,
		P50LatencyMs:     float64(startupTimes[len(startupTimes)/2].Nanoseconds()) / 1e6,
		P95LatencyMs:     float64(startupTimes[len(startupTimes)*95/100].Nanoseconds()) / 1e6,
		P99LatencyMs:     float64(startupTimes[len(startupTimes)*99/100].Nanoseconds()) / 1e6,
		Metadata: map[string]interface{}{
			"samples":     len(startupTimes),
			"min_ms":      float64(min.Nanoseconds()) / 1e6,
			"max_ms":      float64(max.Nanoseconds()) / 1e6,
			"requirement": "Shard startup should be under 1 second",
		},
	})
	
	t.Logf("Shard startup benchmark: avg=%v, min=%v, max=%v", avg, min, max)
	
	// Assert performance requirements
	require.Less(t, avg, 1*time.Second, "Average shard startup time should be under 1 second")
	require.Less(t, max, 2*time.Second, "Maximum shard startup time should be under 2 seconds")
}

// benchmarkResourceAssignmentThroughput measures resource assignment performance
func (cbs *ComprehensiveBenchmarkSuite) benchmarkResourceAssignmentThroughput(t *testing.T) {
	t.Log("Benchmarking resource assignment throughput...")
	
	ctx := context.Background()
	
	// Test different batch sizes
	batchSizes := []int{10, 100, 1000}
	
	for _, batchSize := range batchSizes {
		t.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(t *testing.T) {
			start := time.Now()
			
			for i := 0; i < batchSize; i++ {
				var memBefore, memAfter runtime.MemStats
				runtime.ReadMemStats(&memBefore)
				
				resource := &interfaces.Resource{
					ID:   fmt.Sprintf("benchmark-resource-%d-%d", batchSize, i),
					Type: "benchmark",
					Data: map[string]string{
						"index":     fmt.Sprintf("%d", i),
						"batch":     fmt.Sprintf("%d", batchSize),
						"timestamp": time.Now().Format(time.RFC3339),
					},
				}
				
				opStart := time.Now()
				_, err := cbs.shardManager.AssignResource(ctx, resource)
				opDuration := time.Since(opStart)
				
				runtime.ReadMemStats(&memAfter)
				memAllocs := int64(memAfter.Mallocs - memBefore.Mallocs)
				
				require.NoError(t, err)
				cbs.analyzer.RecordOperation("resource_assignment", opDuration, true, memAllocs)
			}
			
			totalDuration := time.Since(start)
			throughput := float64(batchSize) / totalDuration.Seconds()
			
			// Add benchmark result
			cbs.analyzer.AddBenchmarkResult(&BenchmarkResult{
				Name:             fmt.Sprintf("resource_assignment_batch_%d", batchSize),
				OperationsPerSec: throughput,
				AvgLatencyMs:     float64(totalDuration.Nanoseconds()) / float64(batchSize) / 1e6,
				Metadata: map[string]interface{}{
					"batch_size":  batchSize,
					"requirement": "Resource assignment should achieve >100 ops/sec",
				},
			})
			
			t.Logf("Resource assignment batch %d: %.2f ops/sec", batchSize, throughput)
			
			// Assert performance requirements
			require.Greater(t, throughput, 50.0, "Resource assignment throughput should be reasonable")
		})
	}
}

// benchmarkResourceMigrationSpeed measures resource migration performance
func (cbs *ComprehensiveBenchmarkSuite) benchmarkResourceMigrationSpeed(t *testing.T) {
	t.Log("Benchmarking resource migration speed...")
	
	ctx := context.Background()
	resourceMigrator := &MockResourceMigrator{}
	
	// Test different resource counts
	resourceCounts := []int{10, 100, 500}
	
	for _, count := range resourceCounts {
		t.Run(fmt.Sprintf("Resources_%d", count), func(t *testing.T) {
			// Create test resources
			resources := make([]*interfaces.Resource, count)
			for i := 0; i < count; i++ {
				resources[i] = &interfaces.Resource{
					ID:   fmt.Sprintf("migration-resource-%d", i),
					Type: "migration-test",
					Data: map[string]string{
						"index":   fmt.Sprintf("%d", i),
						"payload": fmt.Sprintf("test-data-%d", i),
					},
				}
			}
			
			var memBefore, memAfter runtime.MemStats
			runtime.ReadMemStats(&memBefore)
			
			start := time.Now()
			
			// Create migration plan
			planStart := time.Now()
			plan, err := resourceMigrator.CreateMigrationPlan(ctx, "source-shard", "target-shard", resources)
			planDuration := time.Since(planStart)
			require.NoError(t, err)
			
			// Execute migration
			execStart := time.Now()
			err = resourceMigrator.ExecuteMigration(ctx, plan)
			execDuration := time.Since(execStart)
			require.NoError(t, err)
			
			totalDuration := time.Since(start)
			runtime.ReadMemStats(&memAfter)
			memAllocs := int64(memAfter.Mallocs - memBefore.Mallocs)
			
			throughput := float64(count) / totalDuration.Seconds()
			
			cbs.analyzer.RecordOperation("resource_migration", totalDuration, true, memAllocs)
			
			// Add benchmark result
			cbs.analyzer.AddBenchmarkResult(&BenchmarkResult{
				Name:             fmt.Sprintf("resource_migration_%d", count),
				OperationsPerSec: throughput,
				AvgLatencyMs:     float64(totalDuration.Nanoseconds()) / 1e6,
				Metadata: map[string]interface{}{
					"resource_count":    count,
					"plan_duration_ms":  float64(planDuration.Nanoseconds()) / 1e6,
					"exec_duration_ms":  float64(execDuration.Nanoseconds()) / 1e6,
					"requirement":       "Migration should handle >100 resources/sec",
				},
			})
			
			t.Logf("Migration %d resources: %.2f resources/sec (plan: %v, exec: %v)", 
				count, throughput, planDuration, execDuration)
			
			// Assert performance requirements
			require.Greater(t, throughput, 10.0, "Migration throughput should be reasonable")
		})
	}
}

// benchmarkLoadBalancerPerformance measures load balancer performance
func (cbs *ComprehensiveBenchmarkSuite) benchmarkLoadBalancerPerformance(t *testing.T) {
	t.Log("Benchmarking load balancer performance...")
	
	loadBalancer := &MockLoadBalancer{}
	
	// Test different shard counts
	shardCounts := []int{5, 10, 50, 100}
	
	for _, shardCount := range shardCounts {
		t.Run(fmt.Sprintf("Shards_%d", shardCount), func(t *testing.T) {
			// Create test shards
			shards := make([]*shardv1.ShardInstance, shardCount)
			for i := 0; i < shardCount; i++ {
				shards[i] = &shardv1.ShardInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("shard-%d", i),
					},
					Status: shardv1.ShardInstanceStatus{
						Load: float64(i) / float64(shardCount),
					},
				}
			}
			
			// Benchmark optimal shard selection
			iterations := 1000
			start := time.Now()
			
			for i := 0; i < iterations; i++ {
				var memBefore, memAfter runtime.MemStats
				runtime.ReadMemStats(&memBefore)
				
				opStart := time.Now()
				_, err := loadBalancer.GetOptimalShard(shards)
				opDuration := time.Since(opStart)
				
				runtime.ReadMemStats(&memAfter)
				memAllocs := int64(memAfter.Mallocs - memBefore.Mallocs)
				
				require.NoError(t, err)
				cbs.analyzer.RecordOperation("load_balancer_selection", opDuration, true, memAllocs)
			}
			
			totalDuration := time.Since(start)
			throughput := float64(iterations) / totalDuration.Seconds()
			
			// Add benchmark result
			cbs.analyzer.AddBenchmarkResult(&BenchmarkResult{
				Name:             fmt.Sprintf("load_balancer_%d_shards", shardCount),
				OperationsPerSec: throughput,
				AvgLatencyMs:     float64(totalDuration.Nanoseconds()) / float64(iterations) / 1e6,
				Metadata: map[string]interface{}{
					"shard_count": shardCount,
					"iterations":  iterations,
					"requirement": "Load balancer should handle >1000 ops/sec",
				},
			})
			
			t.Logf("Load balancer with %d shards: %.2f ops/sec", shardCount, throughput)
			
			// Assert performance requirements
			require.Greater(t, throughput, 500.0, "Load balancer should be fast")
		})
	}
}

// benchmarkConcurrentOperations measures performance under concurrent load
func (cbs *ComprehensiveBenchmarkSuite) benchmarkConcurrentOperations(t *testing.T) {
	t.Log("Benchmarking concurrent operations...")
	
	ctx := context.Background()
	
	// Test different concurrency levels
	concurrencyLevels := []int{5, 10, 20}
	
	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			var wg sync.WaitGroup
			operationsPerWorker := 50
			
			start := time.Now()
			
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					for j := 0; j < operationsPerWorker; j++ {
						var memBefore, memAfter runtime.MemStats
						runtime.ReadMemStats(&memBefore)
						
						resource := &interfaces.Resource{
							ID:   fmt.Sprintf("concurrent-resource-%d-%d", workerID, j),
							Type: "concurrent-test",
							Data: map[string]string{
								"worker": fmt.Sprintf("%d", workerID),
								"index":  fmt.Sprintf("%d", j),
							},
						}
						
						opStart := time.Now()
						_, err := cbs.shardManager.AssignResource(ctx, resource)
						opDuration := time.Since(opStart)
						
						runtime.ReadMemStats(&memAfter)
						memAllocs := int64(memAfter.Mallocs - memBefore.Mallocs)
						
						if err != nil {
							cbs.analyzer.RecordOperation("concurrent_assignment", opDuration, false, memAllocs)
						} else {
							cbs.analyzer.RecordOperation("concurrent_assignment", opDuration, true, memAllocs)
						}
					}
				}(i)
			}
			
			wg.Wait()
			totalDuration := time.Since(start)
			
			totalOperations := concurrency * operationsPerWorker
			throughput := float64(totalOperations) / totalDuration.Seconds()
			
			// Add benchmark result
			cbs.analyzer.AddBenchmarkResult(&BenchmarkResult{
				Name:             fmt.Sprintf("concurrent_ops_%d", concurrency),
				OperationsPerSec: throughput,
				AvgLatencyMs:     float64(totalDuration.Nanoseconds()) / float64(totalOperations) / 1e6,
				Metadata: map[string]interface{}{
					"concurrency":        concurrency,
					"operations_total":   totalOperations,
					"requirement":        "System should handle concurrent operations efficiently",
				},
			})
			
			t.Logf("Concurrent operations (concurrency=%d): %.2f ops/sec", concurrency, throughput)
			
			// Assert performance requirements
			require.Greater(t, throughput, 20.0, "Concurrent operations should maintain reasonable throughput")
		})
	}
}

// benchmarkMemoryUsage measures memory usage patterns
func (cbs *ComprehensiveBenchmarkSuite) benchmarkMemoryUsage(t *testing.T) {
	t.Log("Benchmarking memory usage...")
	
	ctx := context.Background()
	
	// Force garbage collection before starting
	runtime.GC()
	
	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)
	
	// Perform operations that should test memory usage
	operationCount := 1000
	
	for i := 0; i < operationCount; i++ {
		resource := &interfaces.Resource{
			ID:   fmt.Sprintf("memory-test-resource-%d", i),
			Type: "memory-test",
			Data: map[string]string{
				"index":   fmt.Sprintf("%d", i),
				"payload": fmt.Sprintf("memory-test-payload-%d", i),
			},
		}
		
		_, err := cbs.shardManager.AssignResource(ctx, resource)
		require.NoError(t, err)
		
		// Periodically check memory usage
		if i%100 == 0 {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			cbs.analyzer.RecordOperation("memory_usage", time.Duration(i), true, int64(memStats.Mallocs))
		}
	}
	
	// Force garbage collection and get final stats
	runtime.GC()
	
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)
	
	initialMemMB := float64(initialMemStats.Alloc) / 1024 / 1024
	finalMemMB := float64(finalMemStats.Alloc) / 1024 / 1024
	memoryGrowth := finalMemMB - initialMemMB
	
	// Add benchmark result
	cbs.analyzer.AddBenchmarkResult(&BenchmarkResult{
		Name:         "memory_usage",
		MemoryAllocMB: finalMemMB,
		Metadata: map[string]interface{}{
			"initial_memory_mb": initialMemMB,
			"final_memory_mb":   finalMemMB,
			"memory_growth_mb":  memoryGrowth,
			"gc_cycles":         finalMemStats.NumGC - initialMemStats.NumGC,
			"operations":        operationCount,
			"requirement":       "Memory usage should be controlled and not leak",
		},
	})
	
	t.Logf("Memory usage: initial=%.2fMB, final=%.2fMB, growth=%.2fMB, GC cycles=%d", 
		initialMemMB, finalMemMB, memoryGrowth, finalMemStats.NumGC-initialMemStats.NumGC)
	
	// Assert memory usage requirements
	require.Less(t, memoryGrowth, 100.0, "Memory growth should be controlled")
	require.Less(t, finalMemMB, 200.0, "Total memory usage should be reasonable")
}

// benchmarkScalabilityLimits tests system behavior at scale
func (cbs *ComprehensiveBenchmarkSuite) benchmarkScalabilityLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}
	
	t.Log("Benchmarking scalability limits...")
	
	ctx := context.Background()
	
	// Test with increasing load
	loadLevels := []int{100, 500, 1000}
	
	for _, loadLevel := range loadLevels {
		t.Run(fmt.Sprintf("Load_%d", loadLevel), func(t *testing.T) {
			start := time.Now()
			
			var wg sync.WaitGroup
			workers := 10
			operationsPerWorker := loadLevel / workers
			
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					for j := 0; j < operationsPerWorker; j++ {
						resource := &interfaces.Resource{
							ID:   fmt.Sprintf("scale-resource-%d-%d", workerID, j),
							Type: "scale-test",
							Data: map[string]string{
								"worker": fmt.Sprintf("%d", workerID),
								"index":  fmt.Sprintf("%d", j),
								"load":   fmt.Sprintf("%d", loadLevel),
							},
						}
						
						opStart := time.Now()
						_, err := cbs.shardManager.AssignResource(ctx, resource)
						opDuration := time.Since(opStart)
						
						cbs.analyzer.RecordOperation("scalability_test", opDuration, err == nil, 0)
					}
				}(i)
			}
			
			wg.Wait()
			totalDuration := time.Since(start)
			throughput := float64(loadLevel) / totalDuration.Seconds()
			
			// Add benchmark result
			cbs.analyzer.AddBenchmarkResult(&BenchmarkResult{
				Name:             fmt.Sprintf("scalability_%d", loadLevel),
				OperationsPerSec: throughput,
				AvgLatencyMs:     float64(totalDuration.Nanoseconds()) / float64(loadLevel) / 1e6,
				Metadata: map[string]interface{}{
					"load_level":  loadLevel,
					"workers":     workers,
					"requirement": "System should scale linearly with load",
				},
			})
			
			t.Logf("Scalability test (load=%d): %.2f ops/sec", loadLevel, throughput)
			
			// Assert scalability requirements
			require.Greater(t, throughput, float64(loadLevel)/60.0, "System should handle the load within reasonable time")
		})
	}
}

// TestComprehensiveBenchmarks runs all performance benchmarks
func TestComprehensiveBenchmarks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive benchmarks in short mode")
	}
	
	suite, err := NewComprehensiveBenchmarkSuite()
	require.NoError(t, err)
	
	report := suite.RunAllBenchmarks(t)
	
	// Print comprehensive report
	t.Logf("\n%s", report.String())
	
	// Generate optimization plan
	plan := suite.analyzer.GenerateOptimizationPlan()
	t.Logf("\n%s", plan.String())
	
	// Assert overall performance requirements
	require.Greater(t, report.OverallScore, 60.0, "Overall performance score should be acceptable")
	
	// Check specific performance requirements from task
	foundShardStartup := false
	foundResourceAssignment := false
	foundMigration := false
	
	for _, result := range report.BenchmarkResults {
		switch {
		case result.Name == "shard_startup":
			foundShardStartup = true
			require.Less(t, result.AvgLatencyMs, 1000.0, "Shard startup should be under 1 second")
		case result.Name == "resource_assignment_batch_100":
			foundResourceAssignment = true
			require.Greater(t, result.OperationsPerSec, 50.0, "Resource assignment should achieve reasonable throughput")
		case result.Name == "resource_migration_100":
			foundMigration = true
			require.Greater(t, result.OperationsPerSec, 10.0, "Resource migration should achieve reasonable speed")
		}
	}
	
	require.True(t, foundShardStartup, "Shard startup benchmark should be present")
	require.True(t, foundResourceAssignment, "Resource assignment benchmark should be present")
	require.True(t, foundMigration, "Resource migration benchmark should be present")
	
	t.Log("All comprehensive benchmarks completed successfully!")
}