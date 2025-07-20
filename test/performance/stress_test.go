package performance

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
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

// StressTestConfig defines configuration for stress tests
type StressTestConfig struct {
	Duration             time.Duration
	ConcurrentWorkers    int
	ResourcesPerWorker   int
	ShardCount           int
	FailureInjectionRate float64 // 0.0 to 1.0
	LoadPattern          LoadPattern
}

// StressTestResult contains results from stress testing
type StressTestResult struct {
	Duration             time.Duration
	TotalOperations      int64
	SuccessfulOperations int64
	FailedOperations     int64
	OperationsPerSecond  float64
	AverageLatencyMs     float64
	MaxLatencyMs         float64
	MinLatencyMs         float64
	MemoryUsageMB        float64
	GoroutineCount       int
	ErrorRate            float64
}

// StressTestSuite runs comprehensive stress tests
type StressTestSuite struct {
	shardManager  interfaces.ShardManager
	loadGenerator *LoadGenerator
	optimizer     *PerformanceOptimizer
	profiler      *SystemProfiler
	config        *StressTestConfig

	// Metrics
	totalOps     int64
	successOps   int64
	failedOps    int64
	totalLatency int64
	maxLatency   int64
	minLatency   int64

	mu        sync.RWMutex
	latencies []time.Duration
}

// NewStressTestSuite creates a new stress test suite
func NewStressTestSuite(config *StressTestConfig) (*StressTestSuite, error) {
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
	mockConfig := NewMockConfig("stress-test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, mockConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create shard manager: %w", err)
	}

	// Create load generator config
	loadGenConfig := &LoadGeneratorConfig{
		Pattern:           config.LoadPattern,
		Duration:          config.Duration,
		InitialRPS:        100,
		MaxRPS:            1000,
		ResourceTypes:     []string{"test", "benchmark", "stress"},
		ResourceSizeBytes: 1024,
		Concurrency:       config.ConcurrentWorkers,
		Namespace:         "stress-test",
	}

	loadGenerator := NewLoadGenerator(loadGenConfig, fakeClient, fakeKubeClient, shardManager)
	optimizer := NewPerformanceOptimizer()
	profiler := NewSystemProfiler(500 * time.Millisecond)

	return &StressTestSuite{
		shardManager:  shardManager,
		loadGenerator: loadGenerator,
		optimizer:     optimizer,
		profiler:      profiler,
		config:        config,
		minLatency:    int64(^uint64(0) >> 1), // Max int64
	}, nil
}

// RunStressTest executes the stress test
func (sts *StressTestSuite) RunStressTest(ctx context.Context) (*StressTestResult, error) {
	fmt.Printf("Starting stress test with config: %+v\n", sts.config)

	// Start profiling
	sts.profiler.Start()

	// Setup initial shards
	err := sts.setupInitialShards(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to setup initial shards: %w", err)
	}

	// Start stress test workers
	var wg sync.WaitGroup
	startTime := time.Now()

	// Start resource assignment workers
	for i := 0; i < sts.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go sts.resourceAssignmentWorker(ctx, &wg, i)
	}

	// Start failure injection worker if enabled
	if sts.config.FailureInjectionRate > 0 {
		wg.Add(1)
		go sts.failureInjectionWorker(ctx, &wg)
	}

	// Start monitoring worker
	wg.Add(1)
	go sts.monitoringWorker(ctx, &wg)

	// Wait for test duration or context cancellation
	select {
	case <-ctx.Done():
	case <-time.After(sts.config.Duration):
	}

	// Stop all workers
	wg.Wait()

	// Stop profiling and collect results
	profileReport := sts.profiler.Stop()
	duration := time.Since(startTime)

	return sts.generateResult(duration, &profileReport), nil
}

// setupInitialShards creates initial shards for the test
func (sts *StressTestSuite) setupInitialShards(ctx context.Context) error {
	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           sts.config.ShardCount,
			MaxShards:           sts.config.ShardCount * 2,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 10 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}

	for i := 0; i < sts.config.ShardCount; i++ {
		_, err := sts.shardManager.CreateShard(ctx, shardConfig)
		if err != nil {
			return fmt.Errorf("failed to create shard %d: %w", i, err)
		}
	}

	return nil
}

// resourceAssignmentWorker performs resource assignment operations
func (sts *StressTestSuite) resourceAssignmentWorker(ctx context.Context, wg *sync.WaitGroup, workerID int) {
	defer wg.Done()

	for i := 0; i < sts.config.ResourcesPerWorker; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Create resource
		resource := &interfaces.Resource{
			ID:   fmt.Sprintf("stress-resource-%d-%d", workerID, i),
			Type: sts.getRandomResourceType(),
			Data: sts.generateResourceData(i),
			Metadata: map[string]string{
				"worker_id":    fmt.Sprintf("%d", workerID),
				"resource_num": fmt.Sprintf("%d", i),
				"timestamp":    time.Now().Format(time.RFC3339),
			},
		}

		// Measure operation
		start := time.Now()
		_, err := sts.shardManager.AssignResource(ctx, resource)
		latency := time.Since(start)

		// Record metrics
		sts.recordOperation(latency, err == nil)

		// Record in optimizer
		sts.optimizer.RecordOperation("shard_manager", latency, err == nil)

		// Add some randomness to avoid thundering herd
		if rand.Float64() < 0.1 {
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(10)))
		}
	}
}

// failureInjectionWorker injects random failures to test resilience
func (sts *StressTestSuite) failureInjectionWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if rand.Float64() < sts.config.FailureInjectionRate {
				// Simulate shard failure
				shardID := fmt.Sprintf("shard-%d", rand.Intn(sts.config.ShardCount))
				err := sts.shardManager.HandleFailedShard(ctx, shardID)
				if err != nil {
					fmt.Printf("Failed to handle shard failure for %s: %v\n", shardID, err)
				}
			}
		}
	}
}

// monitoringWorker monitors system performance during the test
func (sts *StressTestSuite) monitoringWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Record system metrics
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			memoryMB := float64(memStats.Alloc) / 1024 / 1024
			cpuPercent := 50.0 // Mock CPU usage

			sts.optimizer.RecordSystemMetrics("system", memoryMB, cpuPercent)

			// Log current status
			totalOps := atomic.LoadInt64(&sts.totalOps)
			successOps := atomic.LoadInt64(&sts.successOps)
			failedOps := atomic.LoadInt64(&sts.failedOps)

			fmt.Printf("Status: Total=%d, Success=%d, Failed=%d, Memory=%.2fMB\n",
				totalOps, successOps, failedOps, memoryMB)
		}
	}
}

// recordOperation records metrics for an operation
func (sts *StressTestSuite) recordOperation(latency time.Duration, success bool) {
	atomic.AddInt64(&sts.totalOps, 1)

	if success {
		atomic.AddInt64(&sts.successOps, 1)
	} else {
		atomic.AddInt64(&sts.failedOps, 1)
	}

	// Update latency metrics
	latencyNs := latency.Nanoseconds()
	atomic.AddInt64(&sts.totalLatency, latencyNs)

	// Update min latency
	for {
		currentMin := atomic.LoadInt64(&sts.minLatency)
		if latencyNs >= currentMin || atomic.CompareAndSwapInt64(&sts.minLatency, currentMin, latencyNs) {
			break
		}
	}

	// Update max latency
	for {
		currentMax := atomic.LoadInt64(&sts.maxLatency)
		if latencyNs <= currentMax || atomic.CompareAndSwapInt64(&sts.maxLatency, currentMax, latencyNs) {
			break
		}
	}

	// Store latency for percentile calculations
	sts.mu.Lock()
	sts.latencies = append(sts.latencies, latency)
	sts.mu.Unlock()
}

// generateResult creates the final test result
func (sts *StressTestSuite) generateResult(duration time.Duration, profileReport *ProfileReport) *StressTestResult {
	totalOps := atomic.LoadInt64(&sts.totalOps)
	successOps := atomic.LoadInt64(&sts.successOps)
	failedOps := atomic.LoadInt64(&sts.failedOps)
	totalLatency := atomic.LoadInt64(&sts.totalLatency)
	maxLatency := atomic.LoadInt64(&sts.maxLatency)
	minLatency := atomic.LoadInt64(&sts.minLatency)

	var avgLatencyMs float64
	if totalOps > 0 {
		avgLatencyMs = float64(totalLatency) / float64(totalOps) / float64(time.Millisecond)
	}

	var errorRate float64
	if totalOps > 0 {
		errorRate = float64(failedOps) / float64(totalOps) * 100
	}

	return &StressTestResult{
		Duration:             duration,
		TotalOperations:      totalOps,
		SuccessfulOperations: successOps,
		FailedOperations:     failedOps,
		OperationsPerSecond:  float64(totalOps) / duration.Seconds(),
		AverageLatencyMs:     avgLatencyMs,
		MaxLatencyMs:         float64(maxLatency) / float64(time.Millisecond),
		MinLatencyMs:         float64(minLatency) / float64(time.Millisecond),
		MemoryUsageMB:        profileReport.MaxMemAllocMB,
		GoroutineCount:       runtime.NumGoroutine(),
		ErrorRate:            errorRate,
	}
}

// getRandomResourceType returns a random resource type
func (sts *StressTestSuite) getRandomResourceType() string {
	types := []string{"test", "benchmark", "stress", "load", "performance"}
	return types[rand.Intn(len(types))]
}

// generateResourceData generates test data for resources
func (sts *StressTestSuite) generateResourceData(index int) map[string]string {
	return map[string]string{
		"index":     fmt.Sprintf("%d", index),
		"timestamp": time.Now().Format(time.RFC3339),
		"payload":   fmt.Sprintf("test-data-%d", index),
		"size":      "1024",
	}
}

// String returns a string representation of the stress test result
func (str *StressTestResult) String() string {
	return fmt.Sprintf(
		"Stress Test Results:\n"+
			"Duration: %v\n"+
			"Total Operations: %d\n"+
			"Successful: %d (%.2f%%)\n"+
			"Failed: %d (%.2f%%)\n"+
			"Operations/sec: %.2f\n"+
			"Latency - Avg: %.2fms, Min: %.2fms, Max: %.2fms\n"+
			"Memory Usage: %.2f MB\n"+
			"Goroutines: %d\n",
		str.Duration,
		str.TotalOperations,
		str.SuccessfulOperations, float64(str.SuccessfulOperations)/float64(str.TotalOperations)*100,
		str.FailedOperations, str.ErrorRate,
		str.OperationsPerSecond,
		str.AverageLatencyMs, str.MinLatencyMs, str.MaxLatencyMs,
		str.MemoryUsageMB,
		str.GoroutineCount,
	)
}

// TestStressTestBasic runs a basic stress test
func TestStressTestBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &StressTestConfig{
		Duration:             10 * time.Second,
		ConcurrentWorkers:    5,
		ResourcesPerWorker:   100,
		ShardCount:           3,
		FailureInjectionRate: 0.0, // No failures for basic test
		LoadPattern:          ConstantLoad,
	}

	suite, err := NewStressTestSuite(config)
	require.NoError(t, err)

	result, err := suite.RunStressTest(ctx)
	require.NoError(t, err)

	t.Logf("Stress test completed:\n%s", result.String())

	// Assert reasonable performance
	require.Greater(t, result.OperationsPerSecond, 10.0, "Should achieve reasonable throughput")
	require.Less(t, result.ErrorRate, 5.0, "Error rate should be low")
	require.Less(t, result.AverageLatencyMs, 1000.0, "Average latency should be reasonable")
	require.Less(t, result.MemoryUsageMB, 200.0, "Memory usage should be reasonable")
}

// TestStressTestWithFailures runs stress test with failure injection
func TestStressTestWithFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	config := &StressTestConfig{
		Duration:             15 * time.Second,
		ConcurrentWorkers:    8,
		ResourcesPerWorker:   50,
		ShardCount:           5,
		FailureInjectionRate: 0.1, // 10% failure injection rate
		LoadPattern:          RandomLoad,
	}

	suite, err := NewStressTestSuite(config)
	require.NoError(t, err)

	result, err := suite.RunStressTest(ctx)
	require.NoError(t, err)

	t.Logf("Stress test with failures completed:\n%s", result.String())

	// Assert system resilience
	require.Greater(t, result.OperationsPerSecond, 5.0, "Should maintain throughput under failures")
	require.Less(t, result.ErrorRate, 20.0, "Error rate should be manageable even with failures")
	require.Greater(t, float64(result.SuccessfulOperations)/float64(result.TotalOperations), 0.8, "Success rate should be reasonable")
}

// TestStressTestHighConcurrency runs stress test with high concurrency
func TestStressTestHighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config := &StressTestConfig{
		Duration:             20 * time.Second,
		ConcurrentWorkers:    20,
		ResourcesPerWorker:   25,
		ShardCount:           10,
		FailureInjectionRate: 0.05, // 5% failure injection rate
		LoadPattern:          BurstLoad,
	}

	suite, err := NewStressTestSuite(config)
	require.NoError(t, err)

	result, err := suite.RunStressTest(ctx)
	require.NoError(t, err)

	t.Logf("High concurrency stress test completed:\n%s", result.String())

	// Assert system handles high concurrency
	require.Greater(t, result.OperationsPerSecond, 15.0, "Should handle high concurrency")
	require.Less(t, result.ErrorRate, 15.0, "Error rate should be acceptable under high concurrency")
	require.Less(t, result.MemoryUsageMB, 500.0, "Memory usage should be controlled under high concurrency")
	require.Less(t, result.GoroutineCount, 1000, "Goroutine count should be reasonable")
}
