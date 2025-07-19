package v1

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateShardConfig validates a ShardConfig object
func ValidateShardConfig(config *ShardConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if config == nil {
		return allErrs
	}
	
	// Validate spec
	specErrs := ValidateShardConfigSpec(&config.Spec)
	allErrs = append(allErrs, specErrs...)
	
	// Validate status
	statusErrs := ValidateShardConfigStatus(&config.Status)
	allErrs = append(allErrs, statusErrs...)
	
	return allErrs
}

// ValidateShardInstance validates a ShardInstance object
func ValidateShardInstance(instance *ShardInstance) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if instance == nil {
		return allErrs
	}
	
	// Validate spec
	specErrs := ValidateShardInstanceSpec(&instance.Spec)
	allErrs = append(allErrs, specErrs...)
	
	// Validate status
	statusErrs := ValidateShardInstanceStatus(&instance.Status)
	allErrs = append(allErrs, statusErrs...)
	
	return allErrs
}

// SetShardConfigDefaults sets default values for ShardConfig
func SetShardConfigDefaults(config *ShardConfig) {
	if config.Spec.MinShards == 0 {
		config.Spec.MinShards = 1
	}
	if config.Spec.MaxShards == 0 {
		config.Spec.MaxShards = 10
	}
	if config.Spec.ScaleUpThreshold == 0 {
		config.Spec.ScaleUpThreshold = 0.8
	}
	if config.Spec.ScaleDownThreshold == 0 {
		config.Spec.ScaleDownThreshold = 0.3
	}
	if config.Spec.HealthCheckInterval.Duration == 0 {
		config.Spec.HealthCheckInterval = metav1.Duration{Duration: 30 * time.Second}
	}
	if config.Spec.LoadBalanceStrategy == "" {
		config.Spec.LoadBalanceStrategy = ConsistentHashStrategy
	}
	if config.Spec.GracefulShutdownTimeout.Duration == 0 {
		config.Spec.GracefulShutdownTimeout = metav1.Duration{Duration: 300 * time.Second}
	}
}

// SetShardInstanceDefaults sets default values for ShardInstance
func SetShardInstanceDefaults(instance *ShardInstance) {
	if instance.Status.Phase == "" {
		instance.Status.Phase = ShardPhasePending
	}
	if instance.Status.HealthStatus == nil {
		instance.Status.HealthStatus = &HealthStatus{
			Healthy:    true,
			LastCheck:  metav1.Now(),
			ErrorCount: 0,
		}
	}
}

// ValidateShardConfigUpdate validates updates to ShardConfig
func ValidateShardConfigUpdate(newConfig, oldConfig *ShardConfig) field.ErrorList {
	allErrs := ValidateShardConfig(newConfig)

	// Add any update-specific validations here
	// For example, prevent certain fields from being changed

	return allErrs
}

// ValidateShardInstanceUpdate validates updates to ShardInstance
func ValidateShardInstanceUpdate(newInstance, oldInstance *ShardInstance) field.ErrorList {
	allErrs := ValidateShardInstance(newInstance)

	// Prevent ShardID from being changed
	if newInstance.Spec.ShardID != oldInstance.Spec.ShardID {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "shardId"), "shardId cannot be changed"))
	}

	return allErrs
}

// ValidateShardPhaseTransition validates phase transitions for ShardInstance
func ValidateShardPhaseTransition(oldPhase, newPhase ShardPhase) error {
	if !IsValidPhaseTransition(oldPhase, newPhase) {
		return fmt.Errorf("invalid phase transition from %s to %s", oldPhase, newPhase)
	}
	return nil
}

// ValidateHashRange validates a HashRange object
func ValidateHashRange(hashRange *HashRange) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if hashRange == nil {
		return allErrs
	}
	
	hashRangePath := field.NewPath("hashRange")
	
	if hashRange.Start > hashRange.End {
		allErrs = append(allErrs, field.Invalid(hashRangePath.Child("start"), hashRange.Start, "start must be less than or equal to end"))
	}
	
	return allErrs
}

// ValidateHealthStatus validates a HealthStatus object
func ValidateHealthStatus(healthStatus *HealthStatus) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if healthStatus == nil {
		return allErrs
	}
	
	healthPath := field.NewPath("healthStatus")
	
	if healthStatus.ErrorCount < 0 {
		allErrs = append(allErrs, field.Invalid(healthPath.Child("errorCount"), healthStatus.ErrorCount, "errorCount must be non-negative"))
	}
	
	return allErrs
}

// ValidateLoadMetrics validates a LoadMetrics object
func ValidateLoadMetrics(loadMetrics *LoadMetrics) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if loadMetrics == nil {
		return allErrs
	}
	
	metricsPath := field.NewPath("loadMetrics")
	
	if loadMetrics.ResourceCount < 0 {
		allErrs = append(allErrs, field.Invalid(metricsPath.Child("resourceCount"), loadMetrics.ResourceCount, "resourceCount must be non-negative"))
	}
	
	if loadMetrics.CPUUsage < 0 || loadMetrics.CPUUsage > 1.0 {
		allErrs = append(allErrs, field.Invalid(metricsPath.Child("cpuUsage"), loadMetrics.CPUUsage, "cpuUsage must be between 0.0 and 1.0"))
	}
	
	if loadMetrics.MemoryUsage < 0 || loadMetrics.MemoryUsage > 1.0 {
		allErrs = append(allErrs, field.Invalid(metricsPath.Child("memoryUsage"), loadMetrics.MemoryUsage, "memoryUsage must be between 0.0 and 1.0"))
	}
	
	if loadMetrics.ProcessingRate < 0 {
		allErrs = append(allErrs, field.Invalid(metricsPath.Child("processingRate"), loadMetrics.ProcessingRate, "processingRate must be non-negative"))
	}
	
	if loadMetrics.QueueLength < 0 {
		allErrs = append(allErrs, field.Invalid(metricsPath.Child("queueLength"), loadMetrics.QueueLength, "queueLength must be non-negative"))
	}
	
	return allErrs
}

// ValidateMigrationPlan validates a MigrationPlan object
func ValidateMigrationPlan(migrationPlan *MigrationPlan) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if migrationPlan == nil {
		return allErrs
	}
	
	planPath := field.NewPath("migrationPlan")
	
	if migrationPlan.SourceShard == "" {
		allErrs = append(allErrs, field.Required(planPath.Child("sourceShard"), "sourceShard is required"))
	}
	
	if migrationPlan.TargetShard == "" {
		allErrs = append(allErrs, field.Required(planPath.Child("targetShard"), "targetShard is required"))
	}
	
	if migrationPlan.SourceShard == migrationPlan.TargetShard {
		allErrs = append(allErrs, field.Invalid(planPath.Child("targetShard"), migrationPlan.TargetShard, "targetShard must be different from sourceShard"))
	}
	
	if len(migrationPlan.Resources) == 0 {
		allErrs = append(allErrs, field.Required(planPath.Child("resources"), "at least one resource must be specified"))
	}
	
	if migrationPlan.EstimatedTime.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(planPath.Child("estimatedTime"), migrationPlan.EstimatedTime, "estimatedTime must be non-negative"))
	}
	
	validPriorities := map[MigrationPriority]bool{
		MigrationPriorityHigh:   true,
		MigrationPriorityMedium: true,
		MigrationPriorityLow:    true,
	}
	if !validPriorities[migrationPlan.Priority] {
		allErrs = append(allErrs, field.NotSupported(planPath.Child("priority"), migrationPlan.Priority, []string{
			string(MigrationPriorityHigh),
			string(MigrationPriorityMedium),
			string(MigrationPriorityLow),
		}))
	}
	
	return allErrs
}

// ValidateShardConfigStatus validates the status of a ShardConfig
func ValidateShardConfigStatus(status *ShardConfigStatus) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if status == nil {
		return allErrs
	}
	
	statusPath := field.NewPath("status")
	
	if status.CurrentShards < 0 {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("currentShards"), status.CurrentShards, "currentShards must be non-negative"))
	}
	
	if status.HealthyShards < 0 {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("healthyShards"), status.HealthyShards, "healthyShards must be non-negative"))
	}
	
	if status.HealthyShards > status.CurrentShards {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("healthyShards"), status.HealthyShards, "healthyShards cannot be greater than currentShards"))
	}
	
	if status.TotalLoad < 0 {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("totalLoad"), status.TotalLoad, "totalLoad must be non-negative"))
	}
	
	return allErrs
}

// ValidateShardInstanceStatus validates the status of a ShardInstance
func ValidateShardInstanceStatus(status *ShardInstanceStatus) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if status == nil {
		return allErrs
	}
	
	statusPath := field.NewPath("status")
	
	// Validate phase
	validPhases := map[ShardPhase]bool{
		ShardPhasePending:    true,
		ShardPhaseRunning:    true,
		ShardPhaseDraining:   true,
		ShardPhaseFailed:     true,
		ShardPhaseTerminated: true,
	}
	if !validPhases[status.Phase] {
		allErrs = append(allErrs, field.NotSupported(statusPath.Child("phase"), status.Phase, []string{
			string(ShardPhasePending),
			string(ShardPhaseRunning),
			string(ShardPhaseDraining),
			string(ShardPhaseFailed),
			string(ShardPhaseTerminated),
		}))
	}
	
	if status.Load < 0 {
		allErrs = append(allErrs, field.Invalid(statusPath.Child("load"), status.Load, "load must be non-negative"))
	}
	
	// Validate health status if present
	if status.HealthStatus != nil {
		healthErrs := ValidateHealthStatus(status.HealthStatus)
		for _, err := range healthErrs {
			allErrs = append(allErrs, field.Invalid(statusPath.Child("healthStatus").Child(err.Field), err.BadValue, err.Detail))
		}
	}
	
	return allErrs
}

// ValidateShardConfigSpec validates the spec of a ShardConfig with enhanced checks
func ValidateShardConfigSpec(spec *ShardConfigSpec) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if spec == nil {
		return allErrs
	}
	
	specPath := field.NewPath("spec")
	
	// Enhanced validation for MinShards
	if spec.MinShards < 1 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("minShards"), spec.MinShards, "must be at least 1"))
	} else if spec.MinShards > 100 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("minShards"), spec.MinShards, "must not exceed 100"))
	}
	
	// Enhanced validation for MaxShards
	if spec.MaxShards < 1 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("maxShards"), spec.MaxShards, "must be at least 1"))
	} else if spec.MaxShards > 1000 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("maxShards"), spec.MaxShards, "must not exceed 1000"))
	}
	
	// Validate MinShards <= MaxShards
	if spec.MinShards > spec.MaxShards {
		allErrs = append(allErrs, field.Invalid(specPath.Child("minShards"), spec.MinShards, "must be less than or equal to maxShards"))
	}
	
	// Enhanced threshold validation
	if spec.ScaleUpThreshold < 0.1 || spec.ScaleUpThreshold > 1.0 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("scaleUpThreshold"), spec.ScaleUpThreshold, "must be between 0.1 and 1.0"))
	}
	
	if spec.ScaleDownThreshold < 0.1 || spec.ScaleDownThreshold > 1.0 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("scaleDownThreshold"), spec.ScaleDownThreshold, "must be between 0.1 and 1.0"))
	}
	
	// Ensure reasonable gap between thresholds
	if spec.ScaleDownThreshold >= spec.ScaleUpThreshold {
		allErrs = append(allErrs, field.Invalid(specPath.Child("scaleDownThreshold"), spec.ScaleDownThreshold, "must be less than scaleUpThreshold"))
	}
	
	if spec.ScaleUpThreshold-spec.ScaleDownThreshold < 0.1 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("scaleUpThreshold"), spec.ScaleUpThreshold, "must be at least 0.1 higher than scaleDownThreshold to prevent flapping"))
	}
	
	// Enhanced health check interval validation
	if spec.HealthCheckInterval.Duration < time.Second {
		allErrs = append(allErrs, field.Invalid(specPath.Child("healthCheckInterval"), spec.HealthCheckInterval, "must be at least 1 second"))
	} else if spec.HealthCheckInterval.Duration > 10*time.Minute {
		allErrs = append(allErrs, field.Invalid(specPath.Child("healthCheckInterval"), spec.HealthCheckInterval, "must not exceed 10 minutes"))
	}
	
	// Validate LoadBalanceStrategy
	validStrategies := map[LoadBalanceStrategy]bool{
		ConsistentHashStrategy: true,
		RoundRobinStrategy:     true,
		LeastLoadedStrategy:    true,
	}
	if !validStrategies[spec.LoadBalanceStrategy] {
		allErrs = append(allErrs, field.NotSupported(specPath.Child("loadBalanceStrategy"), spec.LoadBalanceStrategy, []string{
			string(ConsistentHashStrategy),
			string(RoundRobinStrategy),
			string(LeastLoadedStrategy),
		}))
	}
	
	// Enhanced graceful shutdown timeout validation
	if spec.GracefulShutdownTimeout.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("gracefulShutdownTimeout"), spec.GracefulShutdownTimeout, "must be non-negative"))
	} else if spec.GracefulShutdownTimeout.Duration > time.Hour {
		allErrs = append(allErrs, field.Invalid(specPath.Child("gracefulShutdownTimeout"), spec.GracefulShutdownTimeout, "must not exceed 1 hour"))
	}
	
	return allErrs
}

// ValidateShardInstanceSpec validates the spec of a ShardInstance with enhanced checks
func ValidateShardInstanceSpec(spec *ShardInstanceSpec) field.ErrorList {
	allErrs := field.ErrorList{}
	
	if spec == nil {
		return allErrs
	}
	
	specPath := field.NewPath("spec")
	
	// Enhanced ShardID validation
	if spec.ShardID == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("shardId"), "shardId is required"))
	} else {
		if len(spec.ShardID) > 63 {
			allErrs = append(allErrs, field.TooLong(specPath.Child("shardId"), spec.ShardID, 63))
		}
		// Validate ShardID format (DNS-1123 subdomain)
		if !isValidDNS1123Subdomain(spec.ShardID) {
			allErrs = append(allErrs, field.Invalid(specPath.Child("shardId"), spec.ShardID, "must be a valid DNS-1123 subdomain"))
		}
	}
	
	// Validate HashRange if present
	if spec.HashRange != nil {
		hashRangeErrs := ValidateHashRange(spec.HashRange)
		for _, err := range hashRangeErrs {
			allErrs = append(allErrs, field.Invalid(specPath.Child("hashRange").Child(err.Field), err.BadValue, err.Detail))
		}
	}
	
	// Validate resources
	if len(spec.Resources) > 10000 {
		allErrs = append(allErrs, field.TooMany(specPath.Child("resources"), len(spec.Resources), 10000))
	}
	
	return allErrs
}

// isValidDNS1123Subdomain validates if a string is a valid DNS-1123 subdomain
func isValidDNS1123Subdomain(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	
	// Simple validation - in real implementation, use k8s.io/apimachinery/pkg/util/validation
	for i, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.') {
			return false
		}
		if i == 0 && (r == '-' || r == '.') {
			return false
		}
		if i == len(value)-1 && (r == '-' || r == '.') {
			return false
		}
	}
	return true
}