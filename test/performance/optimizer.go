package performance

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// PerformanceOptimizer analyzes system performance and suggests optimizations
type PerformanceOptimizer struct {
	mu              sync.RWMutex
	metrics         map[string]*ComponentMetrics
	recommendations []OptimizationRecommendation
	thresholds      *PerformanceThresholds
}

// ComponentMetrics tracks performance metrics for a system component
type ComponentMetrics struct {
	Name            string
	OperationCount  int64
	TotalLatency    time.Duration
	MaxLatency      time.Duration
	MinLatency      time.Duration
	ErrorCount      int64
	MemoryUsageMB   float64
	CPUUsagePercent float64
	LastUpdated     time.Time
}

// PerformanceThresholds defines acceptable performance limits
type PerformanceThresholds struct {
	MaxAvgLatencyMs    float64
	MaxErrorRate       float64
	MaxMemoryUsageMB   float64
	MaxCPUUsagePercent float64
	MinThroughputRPS   float64
}

// OptimizationRecommendation suggests performance improvements
type OptimizationRecommendation struct {
	Component   string
	Issue       string
	Severity    RecommendationSeverity
	Description string
	Solution    string
	Impact      string
}

type RecommendationSeverity string

const (
	SeverityCritical RecommendationSeverity = "critical"
	SeverityHigh     RecommendationSeverity = "high"
	SeverityMedium   RecommendationSeverity = "medium"
	SeverityLow      RecommendationSeverity = "low"
)

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer() *PerformanceOptimizer {
	return &PerformanceOptimizer{
		metrics: make(map[string]*ComponentMetrics),
		thresholds: &PerformanceThresholds{
			MaxAvgLatencyMs:    100.0, // 100ms
			MaxErrorRate:       0.01,  // 1%
			MaxMemoryUsageMB:   512.0, // 512MB
			MaxCPUUsagePercent: 80.0,  // 80%
			MinThroughputRPS:   10.0,  // 10 RPS
		},
	}
}

// RecordOperation records a performance metric for an operation
func (po *PerformanceOptimizer) RecordOperation(component string, latency time.Duration, success bool) {
	po.mu.Lock()
	defer po.mu.Unlock()

	metrics, exists := po.metrics[component]
	if !exists {
		metrics = &ComponentMetrics{
			Name:        component,
			MinLatency:  latency,
			MaxLatency:  latency,
			LastUpdated: time.Now(),
		}
		po.metrics[component] = metrics
	}

	metrics.OperationCount++
	metrics.TotalLatency += latency
	metrics.LastUpdated = time.Now()

	if latency > metrics.MaxLatency {
		metrics.MaxLatency = latency
	}
	if latency < metrics.MinLatency {
		metrics.MinLatency = latency
	}

	if !success {
		metrics.ErrorCount++
	}
}

// RecordSystemMetrics records system-level performance metrics
func (po *PerformanceOptimizer) RecordSystemMetrics(component string, memoryMB, cpuPercent float64) {
	po.mu.Lock()
	defer po.mu.Unlock()

	metrics, exists := po.metrics[component]
	if !exists {
		metrics = &ComponentMetrics{
			Name:        component,
			LastUpdated: time.Now(),
		}
		po.metrics[component] = metrics
	}

	metrics.MemoryUsageMB = memoryMB
	metrics.CPUUsagePercent = cpuPercent
	metrics.LastUpdated = time.Now()
}

// AnalyzePerformance analyzes current performance and generates recommendations
func (po *PerformanceOptimizer) AnalyzePerformance() []OptimizationRecommendation {
	po.mu.Lock()
	defer po.mu.Unlock()

	po.recommendations = nil

	for _, metrics := range po.metrics {
		po.analyzeComponentMetrics(metrics)
	}

	po.analyzeSystemWideMetrics()

	return po.recommendations
}

// analyzeComponentMetrics analyzes metrics for a specific component
func (po *PerformanceOptimizer) analyzeComponentMetrics(metrics *ComponentMetrics) {
	if metrics.OperationCount == 0 {
		return
	}

	// Analyze latency
	avgLatencyMs := float64(metrics.TotalLatency.Nanoseconds()) / float64(metrics.OperationCount) / 1e6
	if avgLatencyMs > po.thresholds.MaxAvgLatencyMs {
		severity := SeverityMedium
		if avgLatencyMs > po.thresholds.MaxAvgLatencyMs*2 {
			severity = SeverityHigh
		}
		if avgLatencyMs > po.thresholds.MaxAvgLatencyMs*5 {
			severity = SeverityCritical
		}

		po.recommendations = append(po.recommendations, OptimizationRecommendation{
			Component:   metrics.Name,
			Issue:       "High Latency",
			Severity:    severity,
			Description: fmt.Sprintf("Average latency is %.2fms, exceeding threshold of %.2fms", avgLatencyMs, po.thresholds.MaxAvgLatencyMs),
			Solution:    po.getLatencyOptimizationSolution(metrics.Name, avgLatencyMs),
			Impact:      "Reduced response times and better user experience",
		})
	}

	// Analyze error rate
	errorRate := float64(metrics.ErrorCount) / float64(metrics.OperationCount)
	if errorRate > po.thresholds.MaxErrorRate {
		severity := SeverityMedium
		if errorRate > po.thresholds.MaxErrorRate*5 {
			severity = SeverityHigh
		}
		if errorRate > po.thresholds.MaxErrorRate*10 {
			severity = SeverityCritical
		}

		po.recommendations = append(po.recommendations, OptimizationRecommendation{
			Component:   metrics.Name,
			Issue:       "High Error Rate",
			Severity:    severity,
			Description: fmt.Sprintf("Error rate is %.2f%%, exceeding threshold of %.2f%%", errorRate*100, po.thresholds.MaxErrorRate*100),
			Solution:    po.getErrorRateOptimizationSolution(metrics.Name, errorRate),
			Impact:      "Improved reliability and reduced failure rates",
		})
	}

	// Analyze memory usage
	if metrics.MemoryUsageMB > po.thresholds.MaxMemoryUsageMB {
		severity := SeverityMedium
		if metrics.MemoryUsageMB > po.thresholds.MaxMemoryUsageMB*1.5 {
			severity = SeverityHigh
		}
		if metrics.MemoryUsageMB > po.thresholds.MaxMemoryUsageMB*2 {
			severity = SeverityCritical
		}

		po.recommendations = append(po.recommendations, OptimizationRecommendation{
			Component:   metrics.Name,
			Issue:       "High Memory Usage",
			Severity:    severity,
			Description: fmt.Sprintf("Memory usage is %.2fMB, exceeding threshold of %.2fMB", metrics.MemoryUsageMB, po.thresholds.MaxMemoryUsageMB),
			Solution:    po.getMemoryOptimizationSolution(metrics.Name, metrics.MemoryUsageMB),
			Impact:      "Reduced memory footprint and better resource utilization",
		})
	}

	// Analyze CPU usage
	if metrics.CPUUsagePercent > po.thresholds.MaxCPUUsagePercent {
		severity := SeverityMedium
		if metrics.CPUUsagePercent > po.thresholds.MaxCPUUsagePercent*1.2 {
			severity = SeverityHigh
		}
		if metrics.CPUUsagePercent > 95.0 {
			severity = SeverityCritical
		}

		po.recommendations = append(po.recommendations, OptimizationRecommendation{
			Component:   metrics.Name,
			Issue:       "High CPU Usage",
			Severity:    severity,
			Description: fmt.Sprintf("CPU usage is %.2f%%, exceeding threshold of %.2f%%", metrics.CPUUsagePercent, po.thresholds.MaxCPUUsagePercent),
			Solution:    po.getCPUOptimizationSolution(metrics.Name, metrics.CPUUsagePercent),
			Impact:      "Improved CPU efficiency and system responsiveness",
		})
	}
}

// analyzeSystemWideMetrics analyzes system-wide performance patterns
func (po *PerformanceOptimizer) analyzeSystemWideMetrics() {
	var totalMemory, totalCPU float64
	var componentCount int

	for _, metrics := range po.metrics {
		if metrics.MemoryUsageMB > 0 {
			totalMemory += metrics.MemoryUsageMB
			componentCount++
		}
		if metrics.CPUUsagePercent > 0 {
			totalCPU += metrics.CPUUsagePercent
		}
	}

	// Check for memory leaks
	if componentCount > 0 {
		avgMemory := totalMemory / float64(componentCount)
		if avgMemory > po.thresholds.MaxMemoryUsageMB*0.8 {
			po.recommendations = append(po.recommendations, OptimizationRecommendation{
				Component:   "system",
				Issue:       "Potential Memory Leak",
				Severity:    SeverityHigh,
				Description: fmt.Sprintf("Average memory usage across components is %.2fMB, indicating potential memory leaks", avgMemory),
				Solution:    "Implement memory profiling, review object lifecycle management, add garbage collection tuning",
				Impact:      "Prevented memory exhaustion and improved system stability",
			})
		}
	}

	// Check for CPU bottlenecks
	if totalCPU > po.thresholds.MaxCPUUsagePercent*float64(componentCount)*0.8 {
		po.recommendations = append(po.recommendations, OptimizationRecommendation{
			Component:   "system",
			Issue:       "CPU Bottleneck",
			Severity:    SeverityHigh,
			Description: fmt.Sprintf("Total CPU usage across components is %.2f%%, indicating CPU bottlenecks", totalCPU),
			Solution:    "Optimize algorithms, implement caching, consider horizontal scaling, review goroutine usage",
			Impact:      "Improved system throughput and reduced response times",
		})
	}
}

// getLatencyOptimizationSolution provides latency optimization recommendations
func (po *PerformanceOptimizer) getLatencyOptimizationSolution(component string, latencyMs float64) string {
	solutions := []string{
		"Implement caching for frequently accessed data",
		"Optimize database queries and add proper indexing",
		"Use connection pooling for external services",
		"Implement asynchronous processing where possible",
		"Review and optimize critical path algorithms",
		"Consider using faster serialization formats",
		"Implement request batching to reduce overhead",
	}

	switch component {
	case "shard_manager":
		return "Optimize shard selection algorithm, implement shard metadata caching, use concurrent processing for shard operations"
	case "load_balancer":
		return "Cache load calculations, optimize consistent hash implementation, use pre-computed hash rings"
	case "resource_migrator":
		return "Implement parallel migration, optimize resource serialization, use streaming for large resources"
	case "health_checker":
		return "Implement health check caching, use circuit breakers, optimize network timeouts"
	default:
		return solutions[int(latencyMs)%len(solutions)]
	}
}

// getErrorRateOptimizationSolution provides error rate optimization recommendations
func (po *PerformanceOptimizer) getErrorRateOptimizationSolution(component string, errorRate float64) string {
	switch component {
	case "shard_manager":
		return "Implement retry logic with exponential backoff, add better error handling for Kubernetes API calls, implement circuit breakers"
	case "load_balancer":
		return "Add validation for shard health before assignment, implement fallback strategies, improve error logging"
	case "resource_migrator":
		return "Implement migration validation, add rollback mechanisms, improve error recovery procedures"
	case "health_checker":
		return "Implement health check retries, add timeout handling, improve network error detection"
	default:
		return "Implement comprehensive error handling, add retry mechanisms, improve input validation, add circuit breakers"
	}
}

// getMemoryOptimizationSolution provides memory optimization recommendations
func (po *PerformanceOptimizer) getMemoryOptimizationSolution(component string, memoryMB float64) string {
	switch component {
	case "shard_manager":
		return "Implement shard metadata cleanup, use object pooling for frequent allocations, optimize in-memory caching"
	case "load_balancer":
		return "Optimize hash ring storage, implement LRU cache for load calculations, reduce object allocations in hot paths"
	case "resource_migrator":
		return "Implement streaming for large resource migrations, use memory-mapped files for temporary storage, optimize resource serialization"
	default:
		return "Implement object pooling, optimize data structures, add memory profiling, implement garbage collection tuning"
	}
}

// getCPUOptimizationSolution provides CPU optimization recommendations
func (po *PerformanceOptimizer) getCPUOptimizationSolution(component string, cpuPercent float64) string {
	switch component {
	case "shard_manager":
		return "Optimize shard selection algorithms, implement concurrent processing, reduce lock contention"
	case "load_balancer":
		return "Optimize consistent hash calculations, implement algorithm caching, use more efficient data structures"
	case "resource_migrator":
		return "Implement parallel processing, optimize serialization algorithms, use worker pools"
	default:
		return "Optimize algorithms, implement caching, reduce computational complexity, use efficient data structures"
	}
}

// GetMetrics returns current performance metrics
func (po *PerformanceOptimizer) GetMetrics() map[string]*ComponentMetrics {
	po.mu.RLock()
	defer po.mu.RUnlock()

	result := make(map[string]*ComponentMetrics)
	for k, v := range po.metrics {
		result[k] = v
	}
	return result
}

// GeneratePerformanceReport generates a comprehensive performance report
func (po *PerformanceOptimizer) GeneratePerformanceReport() *PerformanceReport {
	po.mu.RLock()
	defer po.mu.RUnlock()

	report := &PerformanceReport{
		GeneratedAt:     time.Now(),
		ComponentCount:  len(po.metrics),
		Recommendations: po.recommendations,
		SystemMetrics:   po.getSystemMetrics(),
	}

	// Calculate overall health score
	report.OverallHealthScore = po.calculateHealthScore()

	return report
}

// getSystemMetrics collects current system metrics
func (po *PerformanceOptimizer) getSystemMetrics() SystemMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemMetrics{
		AllocatedMemoryMB: float64(memStats.Alloc) / 1024 / 1024,
		SystemMemoryMB:    float64(memStats.Sys) / 1024 / 1024,
		GCCount:           memStats.NumGC,
		GoroutineCount:    runtime.NumGoroutine(),
		CPUCount:          runtime.NumCPU(),
	}
}

// calculateHealthScore calculates an overall system health score (0-100)
func (po *PerformanceOptimizer) calculateHealthScore() float64 {
	if len(po.recommendations) == 0 {
		return 100.0
	}

	score := 100.0
	for _, rec := range po.recommendations {
		switch rec.Severity {
		case SeverityCritical:
			score -= 25.0
		case SeverityHigh:
			score -= 15.0
		case SeverityMedium:
			score -= 10.0
		case SeverityLow:
			score -= 5.0
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

// PerformanceReport contains a comprehensive performance analysis
type PerformanceReport struct {
	GeneratedAt        time.Time
	ComponentCount     int
	OverallHealthScore float64
	Recommendations    []OptimizationRecommendation
	SystemMetrics      SystemMetrics
}

// SystemMetrics contains system-level performance metrics
type SystemMetrics struct {
	AllocatedMemoryMB float64
	SystemMemoryMB    float64
	GCCount           uint32
	GoroutineCount    int
	CPUCount          int
}

// String returns a string representation of the performance report
func (pr *PerformanceReport) String() string {
	return fmt.Sprintf(
		"Performance Report (Generated: %s)\n"+
			"Overall Health Score: %.1f/100\n"+
			"Components Analyzed: %d\n"+
			"Recommendations: %d\n"+
			"System Memory: %.2f MB allocated, %.2f MB system\n"+
			"Goroutines: %d, GC Cycles: %d",
		pr.GeneratedAt.Format(time.RFC3339),
		pr.OverallHealthScore,
		pr.ComponentCount,
		len(pr.Recommendations),
		pr.SystemMetrics.AllocatedMemoryMB,
		pr.SystemMetrics.SystemMemoryMB,
		pr.SystemMetrics.GoroutineCount,
		pr.SystemMetrics.GCCount,
	)
}
