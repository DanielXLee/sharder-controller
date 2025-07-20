package performance

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

// PerformanceAnalyzer provides comprehensive performance analysis
type PerformanceAnalyzer struct {
	mu                sync.RWMutex
	measurements      map[string]*PerformanceMeasurement
	benchmarkResults  []*BenchmarkResult
	optimizer         *PerformanceOptimizer
	thresholds        *PerformanceThresholds
}

// PerformanceMeasurement tracks detailed performance metrics for an operation
type PerformanceMeasurement struct {
	OperationName     string
	SampleCount       int64
	TotalDuration     time.Duration
	MinDuration       time.Duration
	MaxDuration       time.Duration
	Durations         []time.Duration
	SuccessCount      int64
	ErrorCount        int64
	MemoryAllocations int64
	StartTime         time.Time
	EndTime           time.Time
}

// BenchmarkResult contains results from a specific benchmark
type BenchmarkResult struct {
	Name              string
	OperationsPerSec  float64
	AvgLatencyMs      float64
	P50LatencyMs      float64
	P95LatencyMs      float64
	P99LatencyMs      float64
	MemoryAllocMB     float64
	AllocsPerOp       int64
	Timestamp         time.Time
	Metadata          map[string]interface{}
}

// DetailedPerformanceReport contains comprehensive performance analysis
type DetailedPerformanceReport struct {
	GeneratedAt       time.Time
	TestDuration      time.Duration
	SystemInfo        SystemInfo
	Measurements      map[string]*PerformanceMeasurement
	BenchmarkResults  []*BenchmarkResult
	Recommendations   []OptimizationRecommendation
	OverallScore      float64
	BottleneckAnalysis *BottleneckAnalysis
}

// SystemInfo contains system information
type SystemInfo struct {
	GoVersion      string
	NumCPU         int
	GOMAXPROCS     int
	MemoryStats    runtime.MemStats
	OSInfo         string
	ArchInfo       string
}

// BottleneckAnalysis identifies performance bottlenecks
type BottleneckAnalysis struct {
	PrimaryBottleneck   string
	SecondaryBottleneck string
	CriticalOperations  []string
	ResourceConstraints []string
	ScalabilityIssues   []string
}

// NewPerformanceAnalyzer creates a new performance analyzer
func NewPerformanceAnalyzer() *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		measurements:     make(map[string]*PerformanceMeasurement),
		benchmarkResults: make([]*BenchmarkResult, 0),
		optimizer:        NewPerformanceOptimizer(),
		thresholds: &PerformanceThresholds{
			MaxAvgLatencyMs:     50.0,
			MaxErrorRate:        0.01,
			MaxMemoryUsageMB:    256.0,
			MaxCPUUsagePercent:  70.0,
			MinThroughputRPS:    100.0,
		},
	}
}

// StartMeasurement begins measuring performance for an operation
func (pa *PerformanceAnalyzer) StartMeasurement(operationName string) *PerformanceMeasurement {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	measurement := &PerformanceMeasurement{
		OperationName: operationName,
		StartTime:     time.Now(),
		MinDuration:   time.Duration(^uint64(0) >> 1), // Max duration
		Durations:     make([]time.Duration, 0),
	}

	pa.measurements[operationName] = measurement
	return measurement
}

// RecordOperation records a single operation measurement
func (pa *PerformanceAnalyzer) RecordOperation(operationName string, duration time.Duration, success bool, memAllocs int64) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	measurement, exists := pa.measurements[operationName]
	if !exists {
		measurement = &PerformanceMeasurement{
			OperationName: operationName,
			StartTime:     time.Now(),
			MinDuration:   duration,
			MaxDuration:   duration,
			Durations:     make([]time.Duration, 0),
		}
		pa.measurements[operationName] = measurement
	}

	// Update measurement
	measurement.SampleCount++
	measurement.TotalDuration += duration
	measurement.Durations = append(measurement.Durations, duration)
	measurement.MemoryAllocations += memAllocs
	measurement.EndTime = time.Now()

	if duration < measurement.MinDuration {
		measurement.MinDuration = duration
	}
	if duration > measurement.MaxDuration {
		measurement.MaxDuration = duration
	}

	if success {
		measurement.SuccessCount++
	} else {
		measurement.ErrorCount++
	}

	// Also record in optimizer
	pa.optimizer.RecordOperation(operationName, duration, success)
}

// AddBenchmarkResult adds a benchmark result
func (pa *PerformanceAnalyzer) AddBenchmarkResult(result *BenchmarkResult) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	result.Timestamp = time.Now()
	pa.benchmarkResults = append(pa.benchmarkResults, result)
}

// AnalyzeDetailedPerformance performs comprehensive performance analysis
func (pa *PerformanceAnalyzer) AnalyzeDetailedPerformance() *DetailedPerformanceReport {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	report := &DetailedPerformanceReport{
		GeneratedAt:       time.Now(),
		SystemInfo:        pa.getSystemInfo(),
		Measurements:      make(map[string]*PerformanceMeasurement),
		BenchmarkResults:  make([]*BenchmarkResult, len(pa.benchmarkResults)),
		Recommendations:   pa.optimizer.AnalyzePerformance(),
		BottleneckAnalysis: pa.analyzeBottlenecks(),
	}

	// Copy measurements
	for k, v := range pa.measurements {
		report.Measurements[k] = v
	}

	// Copy benchmark results
	copy(report.BenchmarkResults, pa.benchmarkResults)

	// Calculate overall score
	report.OverallScore = pa.calculateOverallScore()

	// Calculate test duration
	if len(pa.measurements) > 0 {
		var earliest, latest time.Time
		first := true
		for _, measurement := range pa.measurements {
			if first {
				earliest = measurement.StartTime
				latest = measurement.EndTime
				first = false
			} else {
				if measurement.StartTime.Before(earliest) {
					earliest = measurement.StartTime
				}
				if measurement.EndTime.After(latest) {
					latest = measurement.EndTime
				}
			}
		}
		report.TestDuration = latest.Sub(earliest)
	}

	return report
}

// getSystemInfo collects system information
func (pa *PerformanceAnalyzer) getSystemInfo() SystemInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemInfo{
		GoVersion:   runtime.Version(),
		NumCPU:      runtime.NumCPU(),
		GOMAXPROCS:  runtime.GOMAXPROCS(0),
		MemoryStats: memStats,
		OSInfo:      runtime.GOOS,
		ArchInfo:    runtime.GOARCH,
	}
}

// analyzeBottlenecks identifies performance bottlenecks
func (pa *PerformanceAnalyzer) analyzeBottlenecks() *BottleneckAnalysis {
	analysis := &BottleneckAnalysis{
		CriticalOperations:  make([]string, 0),
		ResourceConstraints: make([]string, 0),
		ScalabilityIssues:   make([]string, 0),
	}

	// Find slowest operations
	type operationLatency struct {
		name    string
		avgMs   float64
		samples int64
	}

	var operations []operationLatency
	for name, measurement := range pa.measurements {
		if measurement.SampleCount > 0 {
			avgMs := float64(measurement.TotalDuration.Nanoseconds()) / float64(measurement.SampleCount) / 1e6
			operations = append(operations, operationLatency{
				name:    name,
				avgMs:   avgMs,
				samples: measurement.SampleCount,
			})
		}
	}

	// Sort by average latency
	sort.Slice(operations, func(i, j int) bool {
		return operations[i].avgMs > operations[j].avgMs
	})

	// Identify primary and secondary bottlenecks
	if len(operations) > 0 {
		analysis.PrimaryBottleneck = operations[0].name
		if operations[0].avgMs > pa.thresholds.MaxAvgLatencyMs {
			analysis.CriticalOperations = append(analysis.CriticalOperations, operations[0].name)
		}
	}

	if len(operations) > 1 {
		analysis.SecondaryBottleneck = operations[1].name
		if operations[1].avgMs > pa.thresholds.MaxAvgLatencyMs {
			analysis.CriticalOperations = append(analysis.CriticalOperations, operations[1].name)
		}
	}

	// Check for resource constraints
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	memoryUsageMB := float64(memStats.Alloc) / 1024 / 1024
	if memoryUsageMB > pa.thresholds.MaxMemoryUsageMB {
		analysis.ResourceConstraints = append(analysis.ResourceConstraints, 
			fmt.Sprintf("High memory usage: %.2f MB", memoryUsageMB))
	}

	if memStats.NumGC > 100 {
		analysis.ResourceConstraints = append(analysis.ResourceConstraints, 
			fmt.Sprintf("Frequent garbage collection: %d cycles", memStats.NumGC))
	}

	// Check for scalability issues
	for _, measurement := range pa.measurements {
		if measurement.SampleCount > 1000 {
			errorRate := float64(measurement.ErrorCount) / float64(measurement.SampleCount)
			if errorRate > pa.thresholds.MaxErrorRate {
				analysis.ScalabilityIssues = append(analysis.ScalabilityIssues,
					fmt.Sprintf("High error rate in %s: %.2f%%", measurement.OperationName, errorRate*100))
			}
		}
	}

	return analysis
}

// calculateOverallScore calculates an overall performance score (0-100)
func (pa *PerformanceAnalyzer) calculateOverallScore() float64 {
	score := 100.0
	
	// Deduct points for slow operations
	for _, measurement := range pa.measurements {
		if measurement.SampleCount > 0 {
			avgMs := float64(measurement.TotalDuration.Nanoseconds()) / float64(measurement.SampleCount) / 1e6
			if avgMs > pa.thresholds.MaxAvgLatencyMs {
				score -= 10.0
			}
			
			errorRate := float64(measurement.ErrorCount) / float64(measurement.SampleCount)
			if errorRate > pa.thresholds.MaxErrorRate {
				score -= 15.0
			}
		}
	}

	// Deduct points for resource usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	memoryUsageMB := float64(memStats.Alloc) / 1024 / 1024
	if memoryUsageMB > pa.thresholds.MaxMemoryUsageMB {
		score -= 20.0
	}

	if score < 0 {
		score = 0
	}

	return score
}

// CalculatePercentiles calculates latency percentiles for an operation
func (pa *PerformanceAnalyzer) CalculatePercentiles(operationName string) map[string]float64 {
	pa.mu.RLock()
	defer pa.mu.RUnlock()

	measurement, exists := pa.measurements[operationName]
	if !exists || len(measurement.Durations) == 0 {
		return nil
	}

	// Sort durations
	durations := make([]time.Duration, len(measurement.Durations))
	copy(durations, measurement.Durations)
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	percentiles := map[string]float64{
		"p50": float64(durations[len(durations)*50/100].Nanoseconds()) / 1e6,
		"p90": float64(durations[len(durations)*90/100].Nanoseconds()) / 1e6,
		"p95": float64(durations[len(durations)*95/100].Nanoseconds()) / 1e6,
		"p99": float64(durations[len(durations)*99/100].Nanoseconds()) / 1e6,
	}

	return percentiles
}

// GenerateOptimizationPlan generates a detailed optimization plan
func (pa *PerformanceAnalyzer) GenerateOptimizationPlan() *OptimizationPlan {
	recommendations := pa.optimizer.AnalyzePerformance()
	bottlenecks := pa.analyzeBottlenecks()

	plan := &OptimizationPlan{
		GeneratedAt:     time.Now(),
		Priority:        "high",
		EstimatedEffort: "medium",
		ExpectedGains:   make(map[string]string),
		ActionItems:     make([]ActionItem, 0),
	}

	// Convert recommendations to action items
	for _, rec := range recommendations {
		actionItem := ActionItem{
			Title:       rec.Issue,
			Description: rec.Description,
			Solution:    rec.Solution,
			Priority:    string(rec.Severity),
			Component:   rec.Component,
			Impact:      rec.Impact,
		}
		plan.ActionItems = append(plan.ActionItems, actionItem)
	}

	// Add bottleneck-specific actions
	if bottlenecks.PrimaryBottleneck != "" {
		plan.ActionItems = append(plan.ActionItems, ActionItem{
			Title:       fmt.Sprintf("Optimize Primary Bottleneck: %s", bottlenecks.PrimaryBottleneck),
			Description: fmt.Sprintf("The operation '%s' is the primary performance bottleneck", bottlenecks.PrimaryBottleneck),
			Solution:    pa.getBottleneckSolution(bottlenecks.PrimaryBottleneck),
			Priority:    "critical",
			Component:   bottlenecks.PrimaryBottleneck,
			Impact:      "Significant performance improvement expected",
		})
	}

	// Set expected gains
	plan.ExpectedGains["latency"] = "20-40% reduction in average latency"
	plan.ExpectedGains["throughput"] = "15-30% increase in throughput"
	plan.ExpectedGains["memory"] = "10-25% reduction in memory usage"
	plan.ExpectedGains["stability"] = "Improved system stability and error rates"

	return plan
}

// getBottleneckSolution provides specific solutions for bottlenecks
func (pa *PerformanceAnalyzer) getBottleneckSolution(operationName string) string {
	solutions := map[string]string{
		"shard_manager":     "Implement caching for shard metadata, optimize shard selection algorithm, use concurrent processing",
		"load_balancer":     "Cache load calculations, optimize consistent hash implementation, pre-compute hash rings",
		"resource_migrator": "Implement parallel migration, optimize serialization, use streaming for large resources",
		"health_checker":    "Implement health check caching, use circuit breakers, optimize network timeouts",
	}

	if solution, exists := solutions[operationName]; exists {
		return solution
	}

	return "Profile the operation to identify specific bottlenecks, optimize algorithms, implement caching where appropriate"
}

// OptimizationPlan contains a detailed optimization plan
type OptimizationPlan struct {
	GeneratedAt     time.Time
	Priority        string
	EstimatedEffort string
	ExpectedGains   map[string]string
	ActionItems     []ActionItem
}

// ActionItem represents a specific optimization action
type ActionItem struct {
	Title       string
	Description string
	Solution    string
	Priority    string
	Component   string
	Impact      string
	Completed   bool
}

// String returns a string representation of the detailed performance report
func (dpr *DetailedPerformanceReport) String() string {
	result := fmt.Sprintf("Detailed Performance Analysis Report\n")
	result += fmt.Sprintf("Generated: %s\n", dpr.GeneratedAt.Format(time.RFC3339))
	result += fmt.Sprintf("Test Duration: %v\n", dpr.TestDuration)
	result += fmt.Sprintf("Overall Score: %.1f/100\n\n", dpr.OverallScore)

	result += fmt.Sprintf("System Information:\n")
	result += fmt.Sprintf("  Go Version: %s\n", dpr.SystemInfo.GoVersion)
	result += fmt.Sprintf("  CPU Cores: %d\n", dpr.SystemInfo.NumCPU)
	result += fmt.Sprintf("  Memory Allocated: %.2f MB\n", float64(dpr.SystemInfo.MemoryStats.Alloc)/1024/1024)
	result += fmt.Sprintf("  GC Cycles: %d\n\n", dpr.SystemInfo.MemoryStats.NumGC)

	if dpr.BottleneckAnalysis != nil {
		result += fmt.Sprintf("Bottleneck Analysis:\n")
		result += fmt.Sprintf("  Primary: %s\n", dpr.BottleneckAnalysis.PrimaryBottleneck)
		result += fmt.Sprintf("  Secondary: %s\n", dpr.BottleneckAnalysis.SecondaryBottleneck)
		if len(dpr.BottleneckAnalysis.CriticalOperations) > 0 {
			result += fmt.Sprintf("  Critical Operations: %v\n", dpr.BottleneckAnalysis.CriticalOperations)
		}
		result += "\n"
	}

	result += fmt.Sprintf("Operation Measurements:\n")
	for name, measurement := range dpr.Measurements {
		if measurement.SampleCount > 0 {
			avgMs := float64(measurement.TotalDuration.Nanoseconds()) / float64(measurement.SampleCount) / 1e6
			errorRate := float64(measurement.ErrorCount) / float64(measurement.SampleCount) * 100
			result += fmt.Sprintf("  %s: %d samples, avg %.2fms, error rate %.2f%%\n", 
				name, measurement.SampleCount, avgMs, errorRate)
		}
	}

	if len(dpr.Recommendations) > 0 {
		result += fmt.Sprintf("\nOptimization Recommendations:\n")
		for i, rec := range dpr.Recommendations {
			result += fmt.Sprintf("  %d. [%s] %s: %s\n", i+1, rec.Severity, rec.Issue, rec.Description)
		}
	}

	return result
}

// String returns a string representation of the optimization plan
func (op *OptimizationPlan) String() string {
	result := fmt.Sprintf("Optimization Plan\n")
	result += fmt.Sprintf("Generated: %s\n", op.GeneratedAt.Format(time.RFC3339))
	result += fmt.Sprintf("Priority: %s\n", op.Priority)
	result += fmt.Sprintf("Estimated Effort: %s\n\n", op.EstimatedEffort)

	result += fmt.Sprintf("Expected Gains:\n")
	for metric, gain := range op.ExpectedGains {
		result += fmt.Sprintf("  %s: %s\n", metric, gain)
	}
	result += "\n"

	result += fmt.Sprintf("Action Items:\n")
	for i, item := range op.ActionItems {
		status := "[ ]"
		if item.Completed {
			status = "[x]"
		}
		result += fmt.Sprintf("  %s %d. [%s] %s\n", status, i+1, item.Priority, item.Title)
		result += fmt.Sprintf("      Component: %s\n", item.Component)
		result += fmt.Sprintf("      Solution: %s\n", item.Solution)
		result += fmt.Sprintf("      Impact: %s\n\n", item.Impact)
	}

	return result
}