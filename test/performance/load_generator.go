package performance

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k8s-shard-controller/pkg/interfaces"
)

// LoadPattern defines different load generation patterns
type LoadPattern string

const (
	ConstantLoad    LoadPattern = "constant"
	LinearIncrease  LoadPattern = "linear_increase"
	SpikeLoad       LoadPattern = "spike"
	RandomLoad      LoadPattern = "random"
	BurstLoad       LoadPattern = "burst"
)

// LoadGeneratorConfig configures the load generator
type LoadGeneratorConfig struct {
	Pattern           LoadPattern
	Duration          time.Duration
	InitialRPS        int // Requests per second
	MaxRPS            int
	ResourceTypes     []string
	ResourceSizeBytes int
	Concurrency       int
	Namespace         string
}

// LoadGenerator generates synthetic load for performance testing
type LoadGenerator struct {
	config       *LoadGeneratorConfig
	client       client.Client
	kubeClient   kubernetes.Interface
	shardManager interfaces.ShardManager
	
	// Metrics
	totalRequests    int64
	successRequests  int64
	failedRequests   int64
	totalLatency     int64
	minLatency       int64
	maxLatency       int64
	
	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// NewLoadGenerator creates a new load generator
func NewLoadGenerator(
	config *LoadGeneratorConfig,
	client client.Client,
	kubeClient kubernetes.Interface,
	shardManager interfaces.ShardManager,
) *LoadGenerator {
	return &LoadGenerator{
		config:       config,
		client:       client,
		kubeClient:   kubeClient,
		shardManager: shardManager,
		stopCh:       make(chan struct{}),
		minLatency:   int64(^uint64(0) >> 1), // Max int64
	}
}

// Start starts the load generation
func (lg *LoadGenerator) Start(ctx context.Context) error {
	fmt.Printf("Starting load generator with pattern: %s, duration: %v, initial RPS: %d\n",
		lg.config.Pattern, lg.config.Duration, lg.config.InitialRPS)

	// Start worker goroutines
	for i := 0; i < lg.config.Concurrency; i++ {
		lg.wg.Add(1)
		go lg.worker(ctx, i)
	}

	// Start load pattern controller
	lg.wg.Add(1)
	go lg.loadController(ctx)

	// Wait for duration or context cancellation
	select {
	case <-ctx.Done():
		lg.Stop()
	case <-time.After(lg.config.Duration):
		lg.Stop()
	}

	lg.wg.Wait()
	return nil
}

// Stop stops the load generation
func (lg *LoadGenerator) Stop() {
	close(lg.stopCh)
}

// GetMetrics returns current performance metrics
func (lg *LoadGenerator) GetMetrics() *LoadMetrics {
	lg.mu.RLock()
	defer lg.mu.RUnlock()

	total := atomic.LoadInt64(&lg.totalRequests)
	success := atomic.LoadInt64(&lg.successRequests)
	failed := atomic.LoadInt64(&lg.failedRequests)
	totalLatency := atomic.LoadInt64(&lg.totalLatency)
	minLatency := atomic.LoadInt64(&lg.minLatency)
	maxLatency := atomic.LoadInt64(&lg.maxLatency)

	var avgLatency float64
	if total > 0 {
		avgLatency = float64(totalLatency) / float64(total) / float64(time.Millisecond)
	}

	return &LoadMetrics{
		TotalRequests:   total,
		SuccessRequests: success,
		FailedRequests:  failed,
		SuccessRate:     float64(success) / float64(total) * 100,
		AvgLatencyMs:    avgLatency,
		MinLatencyMs:    float64(minLatency) / float64(time.Millisecond),
		MaxLatencyMs:    float64(maxLatency) / float64(time.Millisecond),
		RequestsPerSec:  float64(total) / lg.config.Duration.Seconds(),
	}
}

// worker is a worker goroutine that generates load
func (lg *LoadGenerator) worker(ctx context.Context, workerID int) {
	defer lg.wg.Done()

	ticker := time.NewTicker(time.Second / time.Duration(lg.config.InitialRPS/lg.config.Concurrency))
	defer ticker.Stop()

	for {
		select {
		case <-lg.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			lg.generateRequest(ctx, workerID)
		}
	}
}

// loadController controls the load pattern
func (lg *LoadGenerator) loadController(ctx context.Context) {
	defer lg.wg.Done()

	switch lg.config.Pattern {
	case LinearIncrease:
		lg.linearIncreasePattern(ctx)
	case SpikeLoad:
		lg.spikePattern(ctx)
	case RandomLoad:
		lg.randomPattern(ctx)
	case BurstLoad:
		lg.burstPattern(ctx)
	default:
		// Constant load - no adjustment needed
	}
}

// generateRequest generates a single request
func (lg *LoadGenerator) generateRequest(ctx context.Context, workerID int) {
	start := time.Now()
	atomic.AddInt64(&lg.totalRequests, 1)

	// Create a synthetic resource
	resource := lg.createSyntheticResource(workerID)

	// Assign resource to shard
	_, err := lg.shardManager.AssignResource(ctx, resource)
	
	latency := time.Since(start).Nanoseconds()
	atomic.AddInt64(&lg.totalLatency, latency)

	// Update min/max latency
	for {
		currentMin := atomic.LoadInt64(&lg.minLatency)
		if latency >= currentMin || atomic.CompareAndSwapInt64(&lg.minLatency, currentMin, latency) {
			break
		}
	}
	for {
		currentMax := atomic.LoadInt64(&lg.maxLatency)
		if latency <= currentMax || atomic.CompareAndSwapInt64(&lg.maxLatency, currentMax, latency) {
			break
		}
	}

	if err != nil {
		atomic.AddInt64(&lg.failedRequests, 1)
	} else {
		atomic.AddInt64(&lg.successRequests, 1)
	}
}

// createSyntheticResource creates a synthetic resource for testing
func (lg *LoadGenerator) createSyntheticResource(workerID int) *interfaces.Resource {
	resourceType := lg.config.ResourceTypes[rand.Intn(len(lg.config.ResourceTypes))]
	
	return &interfaces.Resource{
		ID:   fmt.Sprintf("resource-%d-%d-%d", workerID, time.Now().UnixNano(), rand.Intn(10000)),
		Type: resourceType,
		Data: map[string]string{
			"payload": fmt.Sprintf("test-data-%d-bytes", lg.config.ResourceSizeBytes),
		},
		Metadata: map[string]string{
			"worker_id":    fmt.Sprintf("%d", workerID),
			"generated_at": time.Now().Format(time.RFC3339),
			"size_bytes":   fmt.Sprintf("%d", lg.config.ResourceSizeBytes),
		},
	}
}

// linearIncreasePattern implements linear increase load pattern
func (lg *LoadGenerator) linearIncreasePattern(ctx context.Context) {
	steps := 10
	stepDuration := lg.config.Duration / time.Duration(steps)
	rpsIncrement := (lg.config.MaxRPS - lg.config.InitialRPS) / steps

	for i := 0; i < steps; i++ {
		select {
		case <-lg.stopCh:
			return
		case <-ctx.Done():
			return
		case <-time.After(stepDuration):
			// Increase RPS
			currentRPS := lg.config.InitialRPS + (rpsIncrement * i)
			fmt.Printf("Increasing load to %d RPS\n", currentRPS)
		}
	}
}

// spikePattern implements spike load pattern
func (lg *LoadGenerator) spikePattern(ctx context.Context) {
	normalDuration := lg.config.Duration * 3 / 4
	spikeDuration := lg.config.Duration / 4

	// Normal load
	time.Sleep(normalDuration)

	// Spike load
	fmt.Printf("Starting load spike to %d RPS\n", lg.config.MaxRPS)
	time.Sleep(spikeDuration)
}

// randomPattern implements random load pattern
func (lg *LoadGenerator) randomPattern(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lg.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			randomRPS := lg.config.InitialRPS + rand.Intn(lg.config.MaxRPS-lg.config.InitialRPS)
			fmt.Printf("Random load adjustment to %d RPS\n", randomRPS)
		}
	}
}

// burstPattern implements burst load pattern
func (lg *LoadGenerator) burstPattern(ctx context.Context) {
	burstInterval := 30 * time.Second
	burstDuration := 5 * time.Second

	ticker := time.NewTicker(burstInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lg.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Printf("Starting burst load to %d RPS for %v\n", lg.config.MaxRPS, burstDuration)
			time.Sleep(burstDuration)
			fmt.Printf("Burst load ended, returning to normal\n")
		}
	}
}

// LoadMetrics contains performance metrics
type LoadMetrics struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	SuccessRate     float64
	AvgLatencyMs    float64
	MinLatencyMs    float64
	MaxLatencyMs    float64
	RequestsPerSec  float64
}

// String returns a string representation of the metrics
func (lm *LoadMetrics) String() string {
	return fmt.Sprintf(
		"Total: %d, Success: %d (%.2f%%), Failed: %d, "+
		"Avg Latency: %.2fms, Min: %.2fms, Max: %.2fms, RPS: %.2f",
		lm.TotalRequests, lm.SuccessRequests, lm.SuccessRate, lm.FailedRequests,
		lm.AvgLatencyMs, lm.MinLatencyMs, lm.MaxLatencyMs, lm.RequestsPerSec,
	)
}