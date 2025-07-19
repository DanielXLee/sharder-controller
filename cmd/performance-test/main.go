package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/controllers"
	"github.com/k8s-shard-controller/pkg/interfaces"
	"github.com/k8s-shard-controller/test/performance"
)

var (
	kubeconfig     = flag.String("kubeconfig", "", "Path to kubeconfig file")
	namespace      = flag.String("namespace", "shard-controller-system", "Namespace for testing")
	testType       = flag.String("test-type", "load", "Type of test: load, startup, migration, stability")
	duration       = flag.Duration("duration", 5*time.Minute, "Test duration")
	initialRPS     = flag.Int("initial-rps", 10, "Initial requests per second")
	maxRPS         = flag.Int("max-rps", 100, "Maximum requests per second")
	concurrency    = flag.Int("concurrency", 5, "Number of concurrent workers")
	resourceSize   = flag.Int("resource-size", 1024, "Size of test resources in bytes")
	shardCount     = flag.Int("shard-count", 5, "Number of shards to create for testing")
	loadPattern    = flag.String("load-pattern", "constant", "Load pattern: constant, linear_increase, spike, random, burst")
	outputFile     = flag.String("output", "", "Output file for results (optional)")
	verbose        = flag.Bool("verbose", false, "Verbose output")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Setup Kubernetes clients
	k8sClient, kubeClient, err := setupKubernetesClients()
	if err != nil {
		log.Fatalf("Failed to setup Kubernetes clients: %v", err)
	}

	// Run the specified test
	switch *testType {
	case "load":
		err = runLoadTest(ctx, k8sClient, kubeClient)
	case "startup":
		err = runStartupTest(ctx, k8sClient, kubeClient)
	case "migration":
		err = runMigrationTest(ctx, k8sClient, kubeClient)
	case "stability":
		err = runStabilityTest(ctx, k8sClient, kubeClient)
	case "benchmark":
		err = runBenchmarkSuite(ctx, k8sClient, kubeClient)
	default:
		log.Fatalf("Unknown test type: %s", *testType)
	}

	if err != nil {
		log.Fatalf("Test failed: %v", err)
	}

	log.Println("Performance test completed successfully")
}

func setupKubernetesClients() (client.Client, kubernetes.Interface, error) {
	var restConfig *rest.Config
	var err error

	if *kubeconfig != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		restConfig, err = config.GetConfig()
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	// Create controller-runtime client
	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create standard Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return k8sClient, kubeClient, nil
}

func runLoadTest(ctx context.Context, k8sClient client.Client, kubeClient kubernetes.Interface) error {
	log.Printf("Starting load test with pattern: %s, duration: %v, RPS: %d-%d",
		*loadPattern, *duration, *initialRPS, *maxRPS)

	// Setup shard manager (in real scenario, this would connect to existing deployment)
	shardManager, err := setupShardManager(k8sClient, kubeClient)
	if err != nil {
		return fmt.Errorf("failed to setup shard manager: %w", err)
	}

	// Create load generator configuration
	loadConfig := &performance.LoadGeneratorConfig{
		Pattern:           performance.LoadPattern(*loadPattern),
		Duration:          *duration,
		InitialRPS:        *initialRPS,
		MaxRPS:            *maxRPS,
		ResourceTypes:     []string{"test", "demo", "sample", "benchmark"},
		ResourceSizeBytes: *resourceSize,
		Concurrency:       *concurrency,
		Namespace:         *namespace,
	}

	// Create and start load generator
	loadGenerator := performance.NewLoadGenerator(loadConfig, k8sClient, kubeClient, shardManager)

	// Start system profiler
	profiler := performance.NewSystemProfiler(1 * time.Second)
	profiler.Start()

	// Run load test
	err = loadGenerator.Start(ctx)
	if err != nil {
		return fmt.Errorf("load test failed: %w", err)
	}

	// Get results
	metrics := loadGenerator.GetMetrics()
	profileReport := profiler.Stop()

	// Print results
	printLoadTestResults(metrics, profileReport)

	// Save results to file if specified
	if *outputFile != "" {
		err = saveResultsToFile(*outputFile, metrics, profileReport)
		if err != nil {
			log.Printf("Failed to save results to file: %v", err)
		}
	}

	return nil
}

func runStartupTest(ctx context.Context, k8sClient client.Client, kubeClient kubernetes.Interface) error {
	log.Printf("Starting shard startup performance test with %d shards", *shardCount)

	shardManager, err := setupShardManager(k8sClient, kubeClient)
	if err != nil {
		return fmt.Errorf("failed to setup shard manager: %w", err)
	}

	// Measure shard startup times
	startupTimes := make([]time.Duration, *shardCount)
	
	for i := 0; i < *shardCount; i++ {
		start := time.Now()
		
		// Create shard configuration
		shardConfig := createTestShardConfig()
		
		_, err := shardManager.CreateShard(ctx, shardConfig)
		if err != nil {
			return fmt.Errorf("failed to create shard %d: %w", i, err)
		}
		
		startupTimes[i] = time.Since(start)
		
		if *verbose {
			log.Printf("Shard %d startup time: %v", i, startupTimes[i])
		}
	}

	// Calculate statistics
	stats := calculateTimeStatistics(startupTimes)
	printStartupTestResults(stats)

	return nil
}

func runMigrationTest(ctx context.Context, k8sClient client.Client, kubeClient kubernetes.Interface) error {
	log.Printf("Starting resource migration performance test")

	// Test different resource counts
	resourceCounts := []int{10, 50, 100, 500, 1000}
	
	for _, count := range resourceCounts {
		log.Printf("Testing migration of %d resources", count)
		
		start := time.Now()
		
		// Create mock resources
		resources := createTestResources(count, *resourceSize)
		
		// Create mock resource migrator
		migrator := &performance.MockResourceMigrator{}
		
		// Create migration plan
		plan, err := migrator.CreateMigrationPlan(ctx, "source-shard", "target-shard", resources)
		if err != nil {
			return fmt.Errorf("failed to create migration plan: %w", err)
		}
		
		// Execute migration
		err = migrator.ExecuteMigration(ctx, plan)
		if err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
		
		duration := time.Since(start)
		throughput := float64(count) / duration.Seconds()
		
		log.Printf("Migrated %d resources in %v (%.2f resources/sec)", count, duration, throughput)
	}

	return nil
}

func runStabilityTest(ctx context.Context, k8sClient client.Client, kubeClient kubernetes.Interface) error {
	log.Printf("Starting system stability test for %v", *duration)

	// Create a longer-running context
	testCtx, testCancel := context.WithTimeout(ctx, *duration)
	defer testCancel()

	shardManager, err := setupShardManager(k8sClient, kubeClient)
	if err != nil {
		return fmt.Errorf("failed to setup shard manager: %w", err)
	}

	// Create moderate load generator
	loadConfig := &performance.LoadGeneratorConfig{
		Pattern:           performance.ConstantLoad,
		Duration:          *duration,
		InitialRPS:        *initialRPS,
		MaxRPS:            *maxRPS,
		ResourceTypes:     []string{"stability-test"},
		ResourceSizeBytes: *resourceSize,
		Concurrency:       *concurrency / 2, // Lower concurrency for stability
		Namespace:         *namespace,
	}

	loadGenerator := performance.NewLoadGenerator(loadConfig, k8sClient, kubeClient, shardManager)

	// Start system monitoring
	profiler := performance.NewSystemProfiler(5 * time.Second) // Sample every 5 seconds
	profiler.Start()

	// Run stability test
	err = loadGenerator.Start(testCtx)
	if err != nil {
		return fmt.Errorf("stability test failed: %w", err)
	}

	// Get results
	metrics := loadGenerator.GetMetrics()
	profileReport := profiler.Stop()

	// Print stability results
	printStabilityTestResults(metrics, profileReport)

	return nil
}

func runBenchmarkSuite(ctx context.Context, k8sClient client.Client, kubeClient kubernetes.Interface) error {
	log.Printf("Running comprehensive benchmark suite")

	// Run multiple benchmark scenarios
	scenarios := []struct {
		name     string
		testFunc func(context.Context, client.Client, kubernetes.Interface) error
	}{
		{"Load Test", runLoadTest},
		{"Startup Test", runStartupTest},
		{"Migration Test", runMigrationTest},
		{"Stability Test", runStabilityTest},
	}

	results := make(map[string]error)

	for _, scenario := range scenarios {
		log.Printf("Running %s...", scenario.name)
		start := time.Now()
		
		err := scenario.testFunc(ctx, k8sClient, kubeClient)
		duration := time.Since(start)
		
		results[scenario.name] = err
		
		if err != nil {
			log.Printf("%s failed in %v: %v", scenario.name, duration, err)
		} else {
			log.Printf("%s completed successfully in %v", scenario.name, duration)
		}
		
		// Brief pause between tests
		time.Sleep(5 * time.Second)
	}

	// Print summary
	log.Println("\n=== Benchmark Suite Summary ===")
	for name, err := range results {
		status := "PASS"
		if err != nil {
			status = "FAIL"
		}
		log.Printf("%s: %s", name, status)
	}

	return nil
}

// Helper functions

func setupShardManager(k8sClient client.Client, kubeClient kubernetes.Interface) (*controllers.ShardManager, error) {
	// Create mock dependencies for testing
	loadBalancer := &performance.MockLoadBalancer{}
	healthChecker := &performance.MockHealthChecker{}
	resourceMigrator := &performance.MockResourceMigrator{}
	configManager := &performance.MockConfigManager{}
	metricsCollector := &performance.MockMetricsCollector{}
	alertManager := &performance.MockAlertManager{}
	logger := &performance.MockLogger{}
	config := &performance.MockConfig{
		Namespace: *namespace,
		NodeName:  "performance-test-node",
	}

	return controllers.NewShardManager(
		k8sClient, kubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
}

func createTestShardConfig() *shardv1.ShardConfig {
	return &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           1,
			MaxShards:           100,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}
}

func createTestResources(count, sizeBytes int) []*interfaces.Resource {
	resources := make([]*interfaces.Resource, count)
	for i := 0; i < count; i++ {
		resources[i] = &interfaces.Resource{
			ID:   fmt.Sprintf("perf-test-resource-%d", i),
			Type: "performance-test",
			Data: make([]byte, sizeBytes),
			Metadata: map[string]string{
				"test_id":      fmt.Sprintf("%d", i),
				"created_at":   time.Now().Format(time.RFC3339),
				"size_bytes":   fmt.Sprintf("%d", sizeBytes),
			},
		}
	}
	return resources
}

func calculateTimeStatistics(times []time.Duration) TimeStatistics {
	if len(times) == 0 {
		return TimeStatistics{}
	}

	var total time.Duration
	min, max := times[0], times[0]

	for _, t := range times {
		total += t
		if t < min {
			min = t
		}
		if t > max {
			max = t
		}
	}

	avg := total / time.Duration(len(times))

	return TimeStatistics{
		Count:   len(times),
		Total:   total,
		Average: avg,
		Min:     min,
		Max:     max,
	}
}

type TimeStatistics struct {
	Count   int
	Total   time.Duration
	Average time.Duration
	Min     time.Duration
	Max     time.Duration
}

func printLoadTestResults(metrics *performance.LoadMetrics, profile performance.ProfileReport) {
	fmt.Println("\n=== Load Test Results ===")
	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Success Rate: %.2f%%\n", metrics.SuccessRate)
	fmt.Printf("Average Latency: %.2f ms\n", metrics.AvgLatencyMs)
	fmt.Printf("Min Latency: %.2f ms\n", metrics.MinLatencyMs)
	fmt.Printf("Max Latency: %.2f ms\n", metrics.MaxLatencyMs)
	fmt.Printf("Throughput: %.2f RPS\n", metrics.RequestsPerSec)
	fmt.Printf("System Profile: %s\n", profile.String())
}

func printStartupTestResults(stats TimeStatistics) {
	fmt.Println("\n=== Startup Test Results ===")
	fmt.Printf("Shards Created: %d\n", stats.Count)
	fmt.Printf("Total Time: %v\n", stats.Total)
	fmt.Printf("Average Startup Time: %v\n", stats.Average)
	fmt.Printf("Min Startup Time: %v\n", stats.Min)
	fmt.Printf("Max Startup Time: %v\n", stats.Max)
}

func printStabilityTestResults(metrics *performance.LoadMetrics, profile performance.ProfileReport) {
	fmt.Println("\n=== Stability Test Results ===")
	fmt.Printf("Test Duration: %v\n", profile.Duration)
	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Success Rate: %.2f%%\n", metrics.SuccessRate)
	fmt.Printf("System Stability: %s\n", profile.String())
	
	// Stability assessment
	if metrics.SuccessRate > 99.0 && profile.TotalGC < 100 {
		fmt.Println("System Stability: EXCELLENT")
	} else if metrics.SuccessRate > 95.0 && profile.TotalGC < 200 {
		fmt.Println("System Stability: GOOD")
	} else {
		fmt.Println("System Stability: NEEDS IMPROVEMENT")
	}
}

func saveResultsToFile(filename string, metrics *performance.LoadMetrics, profile performance.ProfileReport) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "Performance Test Results\n")
	fmt.Fprintf(file, "========================\n")
	fmt.Fprintf(file, "Timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Total Requests: %d\n", metrics.TotalRequests)
	fmt.Fprintf(file, "Success Rate: %.2f%%\n", metrics.SuccessRate)
	fmt.Fprintf(file, "Average Latency: %.2f ms\n", metrics.AvgLatencyMs)
	fmt.Fprintf(file, "Throughput: %.2f RPS\n", metrics.RequestsPerSec)
	fmt.Fprintf(file, "System Profile: %s\n", profile.String())

	return nil
}