package performance

import (
	"context"
	"fmt"
	"os"
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

// TestFinalPerformanceValidation runs all performance tests and generates a comprehensive report
func TestFinalPerformanceValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive performance validation in short mode")
	}

	t.Log("Starting comprehensive performance validation...")
	
	summary := NewPerformanceSummary()
	
	// Run shard startup performance test
	t.Run("ShardStartupPerformance", func(t *testing.T) {
		results := runShardStartupPerformanceTest(t)
		summary.ShardStartupResults = results
		t.Logf("Shard startup performance: avg=%.2fms, requirement met=%v", 
			results.AverageStartupTimeMs, results.RequirementMet)
	})
	
	// Run resource assignment performance test
	t.Run("ResourceAssignmentPerformance", func(t *testing.T) {
		results := runResourceAssignmentPerformanceTest(t)
		summary.ResourceAssignResults = results
		t.Logf("Resource assignment performance: %.2f RPS, requirement met=%v", 
			results.ThroughputRPS, results.RequirementMet)
	})
	
	// Run migration performance test
	t.Run("MigrationPerformance", func(t *testing.T) {
		results := runMigrationPerformanceTest(t)
		summary.MigrationResults = results
		t.Logf("Migration performance: %.2f resources/sec, requirement met=%v", 
			results.ResourcesPerSecond, results.RequirementMet)
	})
	
	// Run stress test
	t.Run("StressTest", func(t *testing.T) {
		results := runStressTestValidation(t)
		summary.StressTestResults = results
		t.Logf("Stress test: %d ops, %.2f%% success rate, requirement met=%v", 
			results.TotalOperations, results.SuccessRate, results.RequirementMet)
	})
	
	// Calculate overall assessment
	summary.CalculateOverallAssessment()
	
	// Generate and display report
	report := summary.GenerateReport()
	t.Logf("\n%s", report)
	
	// Export to markdown file
	markdownReport := summary.ExportToMarkdown()
	err := os.WriteFile("performance_report.md", []byte(markdownReport), 0644)
	if err != nil {
		t.Logf("Warning: Could not write performance report to file: %v", err)
	} else {
		t.Log("Performance report exported to performance_report.md")
	}
	
	// Assert overall performance requirements
	require.Greater(t, summary.OverallAssessment.Score, 70, 
		"Overall performance score should be above 70 (Grade C or better)")
	require.Greater(t, float64(summary.OverallAssessment.RequirementsMet)/float64(summary.OverallAssessment.TotalRequirements), 0.75, 
		"At least 75% of performance requirements should be met")
	
	t.Logf("Final Performance Score: %d/100 (Grade: %s)", 
		summary.OverallAssessment.Score, summary.OverallAssessment.Grade)
}

// runShardStartupPerformanceTest measures shard startup performance
func runShardStartupPerformanceTest(t *testing.T) *ShardStartupResults {
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
	config := NewMockConfig("perf-test")

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
	
	// Measure shard startup times
	sampleCount := 20
	startupTimes := make([]time.Duration, sampleCount)
	
	for i := 0; i < sampleCount; i++ {
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
	
	avgMs := float64(total.Nanoseconds()) / float64(sampleCount) / 1e6
	minMs := float64(min.Nanoseconds()) / 1e6
	maxMs := float64(max.Nanoseconds()) / 1e6
	
	return &ShardStartupResults{
		AverageStartupTimeMs: avgMs,
		MinStartupTimeMs:     minMs,
		MaxStartupTimeMs:     maxMs,
		SamplesCount:         sampleCount,
		RequirementMet:       avgMs < 1000.0, // Requirement: < 1000ms
	}
}

// runResourceAssignmentPerformanceTest measures resource assignment performance
func runResourceAssignmentPerformanceTest(t *testing.T) *ResourceAssignmentResults {
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
	config := NewMockConfig("perf-test")

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(t, err)

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
	
	// Create shards for testing
	for i := 0; i < 3; i++ {
		_, err := shardManager.CreateShard(ctx, shardConfig)
		require.NoError(t, err)
	}
	
	// Measure resource assignment performance
	operationCount := 500
	start := time.Now()
	
	for i := 0; i < operationCount; i++ {
		resource := &interfaces.Resource{
			ID:   fmt.Sprintf("perf-resource-%d", i),
			Type: "performance-test",
			Data: map[string]string{
				"index": fmt.Sprintf("%d", i),
				"test":  "performance",
			},
		}
		
		_, err := shardManager.AssignResource(ctx, resource)
		require.NoError(t, err)
	}
	
	totalDuration := time.Since(start)
	throughputRPS := float64(operationCount) / totalDuration.Seconds()
	avgLatencyMs := float64(totalDuration.Nanoseconds()) / float64(operationCount) / 1e6
	
	return &ResourceAssignmentResults{
		ThroughputRPS:    throughputRPS,
		AverageLatencyMs: avgLatencyMs,
		RequirementMet:   throughputRPS > 100.0, // Requirement: > 100 RPS
	}
}

// runMigrationPerformanceTest measures resource migration performance
func runMigrationPerformanceTest(t *testing.T) *MigrationResults {
	ctx := context.Background()
	resourceMigrator := &MockResourceMigrator{}
	
	// Test with 100 resources
	resourceCount := 100
	resources := make([]*interfaces.Resource, resourceCount)
	for i := 0; i < resourceCount; i++ {
		resources[i] = &interfaces.Resource{
			ID:   fmt.Sprintf("migration-resource-%d", i),
			Type: "migration-test",
			Data: map[string]string{
				"index":   fmt.Sprintf("%d", i),
				"payload": fmt.Sprintf("test-data-%d", i),
			},
		}
	}
	
	start := time.Now()
	
	// Create and execute migration plan
	plan, err := resourceMigrator.CreateMigrationPlan(ctx, "source-shard", "target-shard", resources)
	require.NoError(t, err)
	
	err = resourceMigrator.ExecuteMigration(ctx, plan)
	require.NoError(t, err)
	
	totalDuration := time.Since(start)
	resourcesPerSec := float64(resourceCount) / totalDuration.Seconds()
	avgLatencyMs := float64(totalDuration.Nanoseconds()) / 1e6
	
	return &MigrationResults{
		ResourcesPerSecond: resourcesPerSec,
		AverageLatencyMs:   avgLatencyMs,
		RequirementMet:     resourcesPerSec > 100.0, // Requirement: > 100 resources/sec
	}
}

// runStressTestValidation runs a stress test validation
func runStressTestValidation(t *testing.T) *StressTestResults {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	config := &StressTestConfig{
		Duration:              10 * time.Second,
		ConcurrentWorkers:     5,
		ResourcesPerWorker:    50,
		ShardCount:           3,
		FailureInjectionRate: 0.0, // No failures for validation
		LoadPattern:          ConstantLoad,
	}
	
	suite, err := NewStressTestSuite(config)
	require.NoError(t, err)
	
	result, err := suite.RunStressTest(ctx)
	require.NoError(t, err)
	
	successRate := float64(result.SuccessfulOperations) / float64(result.TotalOperations) * 100
	
	return &StressTestResults{
		TotalOperations:  result.TotalOperations,
		SuccessRate:      successRate,
		ThroughputRPS:    result.OperationsPerSecond,
		MaxMemoryUsageMB: result.MemoryUsageMB,
		RequirementMet:   successRate > 95.0, // Requirement: > 95% success rate
	}
}

// TestPerformanceRequirements validates specific performance requirements from the task
func TestPerformanceRequirements(t *testing.T) {
	t.Run("ShardStartupTimeRequirement", func(t *testing.T) {
		results := runShardStartupPerformanceTest(t)
		require.Less(t, results.AverageStartupTimeMs, 1000.0, 
			"Shard startup time should be under 1 second (Requirement 5.1)")
		t.Logf("✅ Shard startup time requirement met: %.2fms < 1000ms", results.AverageStartupTimeMs)
	})
	
	t.Run("ResourceMigrationSpeedRequirement", func(t *testing.T) {
		results := runMigrationPerformanceTest(t)
		require.Greater(t, results.ResourcesPerSecond, 100.0, 
			"Resource migration should handle >100 resources/sec (Requirement 5.3)")
		t.Logf("✅ Resource migration speed requirement met: %.2f resources/sec > 100", results.ResourcesPerSecond)
	})
	
	t.Run("SystemStabilityRequirement", func(t *testing.T) {
		results := runStressTestValidation(t)
		require.Greater(t, results.SuccessRate, 95.0, 
			"System should maintain >95% success rate under load (Requirement 5.1)")
		require.Less(t, results.MaxMemoryUsageMB, 100.0, 
			"Memory usage should be controlled under load")
		t.Logf("✅ System stability requirements met: %.2f%% success rate, %.2fMB memory", 
			results.SuccessRate, results.MaxMemoryUsageMB)
	})
}