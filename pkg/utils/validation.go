package utils

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
)

// IsShardHealthy checks if a shard is in a healthy and operational state.
func IsShardHealthy(status *shardv1.ShardInstanceStatus) bool {
	if status == nil {
		return false
	}

	// A shard is healthy if it's in the Running phase and its health status is marked as healthy.
	isPhaseRunning := status.Phase == shardv1.ShardPhaseRunning
	isHealthStatusOk := status.HealthStatus != nil && status.HealthStatus.Healthy

	return isPhaseRunning && isHealthStatusOk
}

// ValidateShardConfig validates a ShardConfig specification
func ValidateShardConfig(spec *shardv1.ShardConfigSpec) error {
	if spec.MinShards <= 0 {
		return fmt.Errorf("minShards must be greater than 0, got %d", spec.MinShards)
	}

	if spec.MaxShards < spec.MinShards {
		return fmt.Errorf("maxShards (%d) must be greater than or equal to minShards (%d)", spec.MaxShards, spec.MinShards)
	}

	if spec.ScaleUpThreshold <= 0 || spec.ScaleUpThreshold > 1 {
		return fmt.Errorf("scaleUpThreshold must be between 0 and 1, got %f", spec.ScaleUpThreshold)
	}

	if spec.ScaleDownThreshold <= 0 || spec.ScaleDownThreshold > 1 {
		return fmt.Errorf("scaleDownThreshold must be between 0 and 1, got %f", spec.ScaleDownThreshold)
	}

	if spec.ScaleDownThreshold >= spec.ScaleUpThreshold {
		return fmt.Errorf("scaleDownThreshold (%f) must be less than scaleUpThreshold (%f)", spec.ScaleDownThreshold, spec.ScaleUpThreshold)
	}

	if spec.HealthCheckInterval.Duration <= 0 {
		return fmt.Errorf("healthCheckInterval must be positive, got %v", spec.HealthCheckInterval.Duration)
	}

	if spec.GracefulShutdownTimeout.Duration <= 0 {
		return fmt.Errorf("gracefulShutdownTimeout must be positive, got %v", spec.GracefulShutdownTimeout.Duration)
	}

	// Validate load balance strategy
	switch spec.LoadBalanceStrategy {
	case shardv1.ConsistentHashStrategy, shardv1.RoundRobinStrategy, shardv1.LeastLoadedStrategy:
		// Valid strategies
	default:
		return fmt.Errorf("invalid loadBalanceStrategy: %s", spec.LoadBalanceStrategy)
	}

	return nil
}

// ValidateShardInstance validates a ShardInstance specification
func ValidateShardInstance(spec *shardv1.ShardInstanceSpec) error {
	if spec.ShardID == "" {
		return fmt.Errorf("shardId cannot be empty")
	}

	if spec.HashRange != nil {
		if spec.HashRange.Start > spec.HashRange.End {
			return fmt.Errorf("hashRange start (%d) must be less than or equal to end (%d)", spec.HashRange.Start, spec.HashRange.End)
		}
	}

	return nil
}

// ValidateConfig validates the controller configuration
func ValidateConfig(cfg *config.Config) error {
	if cfg.Namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}

	// Validate leader election config
	if cfg.LeaderElection.Enabled {
		if cfg.LeaderElection.LeaseDuration <= 0 {
			return fmt.Errorf("leaderElection.leaseDuration must be positive")
		}
		if cfg.LeaderElection.RenewDeadline <= 0 {
			return fmt.Errorf("leaderElection.renewDeadline must be positive")
		}
		if cfg.LeaderElection.RetryPeriod <= 0 {
			return fmt.Errorf("leaderElection.retryPeriod must be positive")
		}
		if cfg.LeaderElection.ResourceName == "" {
			return fmt.Errorf("leaderElection.resourceName cannot be empty")
		}
	}

	// Validate health check config
	if cfg.HealthCheck.Interval <= 0 {
		return fmt.Errorf("healthCheck.interval must be positive")
	}
	if cfg.HealthCheck.Timeout <= 0 {
		return fmt.Errorf("healthCheck.timeout must be positive")
	}
	if cfg.HealthCheck.FailureThreshold <= 0 {
		return fmt.Errorf("healthCheck.failureThreshold must be positive")
	}
	if cfg.HealthCheck.SuccessThreshold <= 0 {
		return fmt.Errorf("healthCheck.successThreshold must be positive")
	}
	if cfg.HealthCheck.Port <= 0 || cfg.HealthCheck.Port > 65535 {
		return fmt.Errorf("healthCheck.port must be between 1 and 65535")
	}

	// Validate metrics config
	if cfg.Metrics.Enabled {
		if cfg.Metrics.Port <= 0 || cfg.Metrics.Port > 65535 {
			return fmt.Errorf("metrics.port must be between 1 and 65535")
		}
		if cfg.Metrics.Path == "" {
			return fmt.Errorf("metrics.path cannot be empty")
		}
	}

	// Validate default shard config
	shardConfigSpec := convertToShardConfigSpec(cfg.DefaultShardConfig)
	if err := ValidateShardConfig(&shardConfigSpec); err != nil {
		return fmt.Errorf("invalid defaultShardConfig: %w", err)
	}

	// Validate log level
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
		// Valid log levels
	default:
		return fmt.Errorf("invalid logLevel: %s", cfg.LogLevel)
	}

	return nil
}

// ValidateHashRange validates that hash ranges don't overlap
func ValidateHashRanges(ranges []*shardv1.HashRange) error {
	if len(ranges) <= 1 {
		return nil
	}

	// Sort ranges by start value for easier validation
	sortedRanges := make([]*shardv1.HashRange, len(ranges))
	copy(sortedRanges, ranges)

	// Simple bubble sort for small arrays
	for i := 0; i < len(sortedRanges)-1; i++ {
		for j := 0; j < len(sortedRanges)-i-1; j++ {
			if sortedRanges[j].Start > sortedRanges[j+1].Start {
				sortedRanges[j], sortedRanges[j+1] = sortedRanges[j+1], sortedRanges[j]
			}
		}
	}

	// Check for overlaps
	for i := 0; i < len(sortedRanges)-1; i++ {
		current := sortedRanges[i]
		next := sortedRanges[i+1]

		if current.End >= next.Start {
			return fmt.Errorf("hash ranges overlap: [%d-%d] and [%d-%d]", 
				current.Start, current.End, next.Start, next.End)
		}
	}

	return nil
}

// ValidateLoadMetrics validates load metrics values
func ValidateLoadMetrics(metrics *shardv1.LoadMetrics) error {
	if metrics.ResourceCount < 0 {
		return fmt.Errorf("resourceCount cannot be negative: %d", metrics.ResourceCount)
	}

	if metrics.CPUUsage < 0 || metrics.CPUUsage > 1 {
		return fmt.Errorf("cpuUsage must be between 0 and 1: %f", metrics.CPUUsage)
	}

	if metrics.MemoryUsage < 0 || metrics.MemoryUsage > 1 {
		return fmt.Errorf("memoryUsage must be between 0 and 1: %f", metrics.MemoryUsage)
	}

	if metrics.ProcessingRate < 0 {
		return fmt.Errorf("processingRate cannot be negative: %f", metrics.ProcessingRate)
	}

	if metrics.QueueLength < 0 {
		return fmt.Errorf("queueLength cannot be negative: %d", metrics.QueueLength)
	}

	return nil
}

// ValidateMigrationPlan validates a migration plan
func ValidateMigrationPlan(plan *shardv1.MigrationPlan) error {
	if plan.SourceShard == "" {
		return fmt.Errorf("sourceShard cannot be empty")
	}

	if plan.TargetShard == "" {
		return fmt.Errorf("targetShard cannot be empty")
	}

	if plan.SourceShard == plan.TargetShard {
		return fmt.Errorf("sourceShard and targetShard cannot be the same")
	}

	if len(plan.Resources) == 0 {
		return fmt.Errorf("resources list cannot be empty")
	}

	if plan.EstimatedTime.Duration <= 0 {
		return fmt.Errorf("estimatedTime must be positive")
	}

	// Validate priority
	switch plan.Priority {
	case shardv1.MigrationPriorityHigh, shardv1.MigrationPriorityMedium, shardv1.MigrationPriorityLow:
		// Valid priorities
	default:
		return fmt.Errorf("invalid migration priority: %s", plan.Priority)
	}

	return nil
}

// IsValidShardPhase checks if a shard phase is valid
func IsValidShardPhase(phase shardv1.ShardPhase) bool {
	switch phase {
	case shardv1.ShardPhasePending, shardv1.ShardPhaseRunning, shardv1.ShardPhaseDraining, 
		 shardv1.ShardPhaseFailed, shardv1.ShardPhaseTerminated:
		return true
	default:
		return false
	}
}

// IsHealthyShardPhase checks if a shard phase indicates a healthy state
func IsHealthyShardPhase(phase shardv1.ShardPhase) bool {
	return phase == shardv1.ShardPhaseRunning
}

// IsTerminalShardPhase checks if a shard phase is terminal (no further transitions)
func IsTerminalShardPhase(phase shardv1.ShardPhase) bool {
	return phase == shardv1.ShardPhaseTerminated
}

// CalculateLoadScore calculates a load score from load metrics
func CalculateLoadScore(metrics *shardv1.LoadMetrics) float64 {
	// Weighted average of different load factors
	cpuWeight := 0.3
	memoryWeight := 0.3
	resourceWeight := 0.2
	queueWeight := 0.2

	// Normalize resource count and queue length (assuming max values)
	maxResources := 1000.0
	maxQueue := 100.0

	resourceScore := float64(metrics.ResourceCount) / maxResources
	if resourceScore > 1.0 {
		resourceScore = 1.0
	}

	queueScore := float64(metrics.QueueLength) / maxQueue
	if queueScore > 1.0 {
		queueScore = 1.0
	}

	return cpuWeight*metrics.CPUUsage + 
		   memoryWeight*metrics.MemoryUsage + 
		   resourceWeight*resourceScore + 
		   queueWeight*queueScore
}

// convertToShardConfigSpec converts config.ShardConfig to shardv1.ShardConfigSpec
func convertToShardConfigSpec(cfg config.ShardConfig) shardv1.ShardConfigSpec {
	return shardv1.ShardConfigSpec{
		MinShards:               cfg.MinShards,
		MaxShards:               cfg.MaxShards,
		ScaleUpThreshold:        cfg.ScaleUpThreshold,
		ScaleDownThreshold:      cfg.ScaleDownThreshold,
		HealthCheckInterval:     metav1.Duration{Duration: cfg.HealthCheckInterval},
		LoadBalanceStrategy:     cfg.LoadBalanceStrategy,
		GracefulShutdownTimeout: metav1.Duration{Duration: cfg.GracefulShutdownTimeout},
	}
}