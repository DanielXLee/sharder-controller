package performance

import (
	"fmt"
	"time"
)

// PerformanceSummary contains a summary of all performance test results
type PerformanceSummary struct {
	TestDate              time.Time
	ShardStartupResults   *ShardStartupResults
	ResourceAssignResults *ResourceAssignmentResults
	MigrationResults      *MigrationResults
	StressTestResults     *StressTestResults
	BenchmarkResults      *BenchmarkResults
	OverallAssessment     *OverallAssessment
}

// ShardStartupResults contains shard startup performance metrics
type ShardStartupResults struct {
	AverageStartupTimeMs float64
	MinStartupTimeMs     float64
	MaxStartupTimeMs     float64
	SamplesCount         int
	RequirementMet       bool // Should be < 1000ms
}

// ResourceAssignmentResults contains resource assignment performance metrics
type ResourceAssignmentResults struct {
	ThroughputRPS    float64
	AverageLatencyMs float64
	RequirementMet   bool // Should be > 100 RPS
}

// MigrationResults contains resource migration performance metrics
type MigrationResults struct {
	ResourcesPerSecond float64
	AverageLatencyMs   float64
	RequirementMet     bool // Should handle > 100 resources/sec
}

// StressTestResults contains stress test performance metrics
type StressTestResults struct {
	TotalOperations  int64
	SuccessRate      float64
	ThroughputRPS    float64
	MaxMemoryUsageMB float64
	RequirementMet   bool // Should maintain > 95% success rate
}

// BenchmarkResults contains benchmark performance metrics
type BenchmarkResults struct {
	ShardCreationOpsPerSec      float64
	ResourceAssignmentOpsPerSec float64
	MemoryAllocationsPerOp      int64
	RequirementMet              bool
}

// OverallAssessment contains overall performance assessment
type OverallAssessment struct {
	Score                int    // 0-100
	Grade                string // A, B, C, D, F
	RequirementsMet      int
	TotalRequirements    int
	CriticalIssues       []string
	Recommendations      []string
	OptimizationPriority string
}

// NewPerformanceSummary creates a new performance summary
func NewPerformanceSummary() *PerformanceSummary {
	return &PerformanceSummary{
		TestDate: time.Now(),
	}
}

// GenerateReport generates a comprehensive performance report
func (ps *PerformanceSummary) GenerateReport() string {
	report := fmt.Sprintf("=== K8s Shard Controller Performance Report ===\n")
	report += fmt.Sprintf("Generated: %s\n\n", ps.TestDate.Format(time.RFC3339))

	// Shard Startup Performance
	if ps.ShardStartupResults != nil {
		report += fmt.Sprintf("🚀 Shard Startup Performance:\n")
		report += fmt.Sprintf("   Average: %.2fms (Requirement: <1000ms) %s\n",
			ps.ShardStartupResults.AverageStartupTimeMs,
			getStatusIcon(ps.ShardStartupResults.RequirementMet))
		report += fmt.Sprintf("   Range: %.2fms - %.2fms\n",
			ps.ShardStartupResults.MinStartupTimeMs,
			ps.ShardStartupResults.MaxStartupTimeMs)
		report += fmt.Sprintf("   Samples: %d\n\n", ps.ShardStartupResults.SamplesCount)
	}

	// Resource Assignment Performance
	if ps.ResourceAssignResults != nil {
		report += fmt.Sprintf("📋 Resource Assignment Performance:\n")
		report += fmt.Sprintf("   Throughput: %.2f RPS (Requirement: >100 RPS) %s\n",
			ps.ResourceAssignResults.ThroughputRPS,
			getStatusIcon(ps.ResourceAssignResults.RequirementMet))
		report += fmt.Sprintf("   Average Latency: %.2fms\n\n",
			ps.ResourceAssignResults.AverageLatencyMs)
	}

	// Migration Performance
	if ps.MigrationResults != nil {
		report += fmt.Sprintf("🔄 Resource Migration Performance:\n")
		report += fmt.Sprintf("   Speed: %.2f resources/sec (Requirement: >100 resources/sec) %s\n",
			ps.MigrationResults.ResourcesPerSecond,
			getStatusIcon(ps.MigrationResults.RequirementMet))
		report += fmt.Sprintf("   Average Latency: %.2fms\n\n",
			ps.MigrationResults.AverageLatencyMs)
	}

	// Stress Test Results
	if ps.StressTestResults != nil {
		report += fmt.Sprintf("💪 Stress Test Results:\n")
		report += fmt.Sprintf("   Total Operations: %d\n", ps.StressTestResults.TotalOperations)
		report += fmt.Sprintf("   Success Rate: %.2f%% (Requirement: >95%%) %s\n",
			ps.StressTestResults.SuccessRate,
			getStatusIcon(ps.StressTestResults.RequirementMet))
		report += fmt.Sprintf("   Throughput: %.2f RPS\n", ps.StressTestResults.ThroughputRPS)
		report += fmt.Sprintf("   Max Memory: %.2f MB\n\n", ps.StressTestResults.MaxMemoryUsageMB)
	}

	// Benchmark Results
	if ps.BenchmarkResults != nil {
		report += fmt.Sprintf("⚡ Benchmark Results:\n")
		report += fmt.Sprintf("   Shard Creation: %.2f ops/sec\n",
			ps.BenchmarkResults.ShardCreationOpsPerSec)
		report += fmt.Sprintf("   Resource Assignment: %.2f ops/sec\n",
			ps.BenchmarkResults.ResourceAssignmentOpsPerSec)
		report += fmt.Sprintf("   Memory Allocations: %d allocs/op\n\n",
			ps.BenchmarkResults.MemoryAllocationsPerOp)
	}

	// Overall Assessment
	if ps.OverallAssessment != nil {
		report += fmt.Sprintf("📊 Overall Assessment:\n")
		report += fmt.Sprintf("   Score: %d/100 (Grade: %s)\n",
			ps.OverallAssessment.Score, ps.OverallAssessment.Grade)
		report += fmt.Sprintf("   Requirements Met: %d/%d\n",
			ps.OverallAssessment.RequirementsMet, ps.OverallAssessment.TotalRequirements)

		if len(ps.OverallAssessment.CriticalIssues) > 0 {
			report += fmt.Sprintf("   Critical Issues:\n")
			for _, issue := range ps.OverallAssessment.CriticalIssues {
				report += fmt.Sprintf("     - %s\n", issue)
			}
		}

		if len(ps.OverallAssessment.Recommendations) > 0 {
			report += fmt.Sprintf("   Recommendations:\n")
			for _, rec := range ps.OverallAssessment.Recommendations {
				report += fmt.Sprintf("     - %s\n", rec)
			}
		}

		report += fmt.Sprintf("   Optimization Priority: %s\n",
			ps.OverallAssessment.OptimizationPriority)
	}

	return report
}

// CalculateOverallAssessment calculates the overall performance assessment
func (ps *PerformanceSummary) CalculateOverallAssessment() {
	score := 100
	requirementsMet := 0
	totalRequirements := 0
	var criticalIssues []string
	var recommendations []string

	// Check shard startup requirements
	if ps.ShardStartupResults != nil {
		totalRequirements++
		if ps.ShardStartupResults.RequirementMet {
			requirementsMet++
		} else {
			score -= 20
			criticalIssues = append(criticalIssues,
				fmt.Sprintf("Shard startup time (%.2fms) exceeds requirement (<1000ms)",
					ps.ShardStartupResults.AverageStartupTimeMs))
			recommendations = append(recommendations,
				"Optimize shard initialization process and reduce startup dependencies")
		}
	}

	// Check resource assignment requirements
	if ps.ResourceAssignResults != nil {
		totalRequirements++
		if ps.ResourceAssignResults.RequirementMet {
			requirementsMet++
		} else {
			score -= 25
			criticalIssues = append(criticalIssues,
				fmt.Sprintf("Resource assignment throughput (%.2f RPS) below requirement (>100 RPS)",
					ps.ResourceAssignResults.ThroughputRPS))
			recommendations = append(recommendations,
				"Implement caching for load balancer decisions and optimize shard selection algorithm")
		}
	}

	// Check migration requirements
	if ps.MigrationResults != nil {
		totalRequirements++
		if ps.MigrationResults.RequirementMet {
			requirementsMet++
		} else {
			score -= 20
			criticalIssues = append(criticalIssues,
				fmt.Sprintf("Migration speed (%.2f resources/sec) below requirement (>100 resources/sec)",
					ps.MigrationResults.ResourcesPerSecond))
			recommendations = append(recommendations,
				"Implement parallel migration and optimize resource serialization")
		}
	}

	// Check stress test requirements
	if ps.StressTestResults != nil {
		totalRequirements++
		if ps.StressTestResults.RequirementMet {
			requirementsMet++
		} else {
			score -= 30
			criticalIssues = append(criticalIssues,
				fmt.Sprintf("Stress test success rate (%.2f%%) below requirement (>95%%)",
					ps.StressTestResults.SuccessRate))
			recommendations = append(recommendations,
				"Improve error handling and implement circuit breakers for better resilience")
		}
	}

	// Determine grade and optimization priority
	var grade string
	var priority string

	switch {
	case score >= 90:
		grade = "A"
		priority = "Low"
	case score >= 80:
		grade = "B"
		priority = "Medium"
	case score >= 70:
		grade = "C"
		priority = "High"
	case score >= 60:
		grade = "D"
		priority = "Critical"
	default:
		grade = "F"
		priority = "Critical"
	}

	if score < 0 {
		score = 0
	}

	ps.OverallAssessment = &OverallAssessment{
		Score:                score,
		Grade:                grade,
		RequirementsMet:      requirementsMet,
		TotalRequirements:    totalRequirements,
		CriticalIssues:       criticalIssues,
		Recommendations:      recommendations,
		OptimizationPriority: priority,
	}
}

// getStatusIcon returns an icon based on requirement status
func getStatusIcon(met bool) string {
	if met {
		return "✅"
	}
	return "❌"
}

// ExportToMarkdown exports the performance summary to markdown format
func (ps *PerformanceSummary) ExportToMarkdown() string {
	md := "# K8s Shard Controller Performance Report\n\n"
	md += fmt.Sprintf("**Generated:** %s\n\n", ps.TestDate.Format("2006-01-02 15:04:05"))

	md += "## Performance Metrics\n\n"

	// Shard Startup Performance
	if ps.ShardStartupResults != nil {
		md += "### 🚀 Shard Startup Performance\n\n"
		md += "| Metric | Value | Requirement | Status |\n"
		md += "|--------|-------|-------------|--------|\n"
		md += fmt.Sprintf("| Average Startup Time | %.2fms | <1000ms | %s |\n",
			ps.ShardStartupResults.AverageStartupTimeMs,
			getStatusIcon(ps.ShardStartupResults.RequirementMet))
		md += fmt.Sprintf("| Min Startup Time | %.2fms | - | - |\n",
			ps.ShardStartupResults.MinStartupTimeMs)
		md += fmt.Sprintf("| Max Startup Time | %.2fms | - | - |\n",
			ps.ShardStartupResults.MaxStartupTimeMs)
		md += fmt.Sprintf("| Samples | %d | - | - |\n\n",
			ps.ShardStartupResults.SamplesCount)
	}

	// Resource Assignment Performance
	if ps.ResourceAssignResults != nil {
		md += "### 📋 Resource Assignment Performance\n\n"
		md += "| Metric | Value | Requirement | Status |\n"
		md += "|--------|-------|-------------|--------|\n"
		md += fmt.Sprintf("| Throughput | %.2f RPS | >100 RPS | %s |\n",
			ps.ResourceAssignResults.ThroughputRPS,
			getStatusIcon(ps.ResourceAssignResults.RequirementMet))
		md += fmt.Sprintf("| Average Latency | %.2fms | - | - |\n\n",
			ps.ResourceAssignResults.AverageLatencyMs)
	}

	// Overall Assessment
	if ps.OverallAssessment != nil {
		md += "## 📊 Overall Assessment\n\n"
		md += fmt.Sprintf("**Score:** %d/100 (Grade: %s)\n\n",
			ps.OverallAssessment.Score, ps.OverallAssessment.Grade)
		md += fmt.Sprintf("**Requirements Met:** %d/%d\n\n",
			ps.OverallAssessment.RequirementsMet, ps.OverallAssessment.TotalRequirements)

		if len(ps.OverallAssessment.CriticalIssues) > 0 {
			md += "### Critical Issues\n\n"
			for _, issue := range ps.OverallAssessment.CriticalIssues {
				md += fmt.Sprintf("- %s\n", issue)
			}
			md += "\n"
		}

		if len(ps.OverallAssessment.Recommendations) > 0 {
			md += "### Recommendations\n\n"
			for _, rec := range ps.OverallAssessment.Recommendations {
				md += fmt.Sprintf("- %s\n", rec)
			}
			md += "\n"
		}
	}

	return md
}
