package controllers

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/k8s-shard-controller/pkg/interfaces"
)

// ErrorHandlerIntegration provides integration between the error handler and other system components
type ErrorHandlerIntegration struct {
	errorHandler     *ErrorHandler
	shardManager     interfaces.ShardManager
	resourceMigrator interfaces.ResourceMigrator
	healthChecker    interfaces.HealthChecker
	loadBalancer     interfaces.LoadBalancer
	configManager    interfaces.ConfigManager
}

// NewErrorHandlerIntegration creates a new error handler integration
func NewErrorHandlerIntegration(
	errorHandler *ErrorHandler,
	shardManager interfaces.ShardManager,
	resourceMigrator interfaces.ResourceMigrator,
	healthChecker interfaces.HealthChecker,
	loadBalancer interfaces.LoadBalancer,
	configManager interfaces.ConfigManager,
) *ErrorHandlerIntegration {
	integration := &ErrorHandlerIntegration{
		errorHandler:     errorHandler,
		shardManager:     shardManager,
		resourceMigrator: resourceMigrator,
		healthChecker:    healthChecker,
		loadBalancer:     loadBalancer,
		configManager:    configManager,
	}

	// Register recovery handlers
	integration.registerRecoveryHandlers()

	return integration
}

// registerRecoveryHandlers registers all recovery handlers with the error handler
func (ehi *ErrorHandlerIntegration) registerRecoveryHandlers() {
	// Register retry handler
	retryHandler := NewRetryRecoveryHandler(ehi.shardManager)
	ehi.errorHandler.RegisterRecoveryHandler(RecoveryActionRetry, retryHandler)

	// Register restart handler
	restartHandler := NewRestartRecoveryHandler(ehi.shardManager)
	ehi.errorHandler.RegisterRecoveryHandler(RecoveryActionRestart, restartHandler)

	// Register scale handler
	scaleHandler := NewScaleRecoveryHandler(ehi.shardManager)
	ehi.errorHandler.RegisterRecoveryHandler(RecoveryActionScale, scaleHandler)

	// Register migrate handler
	migrateHandler := NewMigrateRecoveryHandler(ehi.resourceMigrator, ehi.shardManager)
	ehi.errorHandler.RegisterRecoveryHandler(RecoveryActionMigrate, migrateHandler)
}

// HandleShardStartupFailure handles shard startup failures
func (ehi *ErrorHandlerIntegration) HandleShardStartupFailure(ctx context.Context, shardID string, err error) error {
	errorCtx := &ErrorContext{
		ErrorType:  ErrorTypeShardStartup,
		Severity:   ErrorSeverityHigh,
		Component:  "shard_manager",
		Operation:  "create_shard",
		ShardID:    shardID,
		Timestamp:  time.Now(),
		Error:      err,
		RetryCount: 0,
		Metadata: map[string]interface{}{
			"shard_id": shardID,
		},
	}

	return ehi.errorHandler.HandleError(ctx, errorCtx)
}

// HandleShardRuntimeFailure handles shard runtime failures
func (ehi *ErrorHandlerIntegration) HandleShardRuntimeFailure(ctx context.Context, shardID string, err error) error {
	errorCtx := &ErrorContext{
		ErrorType:  ErrorTypeShardRuntime,
		Severity:   ErrorSeverityHigh,
		Component:  "shard_manager",
		Operation:  "shard_operation",
		ShardID:    shardID,
		Timestamp:  time.Now(),
		Error:      err,
		RetryCount: 0,
		Metadata: map[string]interface{}{
			"shard_id": shardID,
		},
	}

	return ehi.errorHandler.HandleError(ctx, errorCtx)
}

// HandleMigrationFailure handles resource migration failures
func (ehi *ErrorHandlerIntegration) HandleMigrationFailure(ctx context.Context, sourceShard, targetShard, resourceID string, err error) error {
	errorCtx := &ErrorContext{
		ErrorType:  ErrorTypeMigrationFailure,
		Severity:   ErrorSeverityMedium,
		Component:  "resource_migrator",
		Operation:  "migrate_resource",
		ShardID:    sourceShard,
		ResourceID: resourceID,
		Timestamp:  time.Now(),
		Error:      err,
		RetryCount: 0,
		Metadata: map[string]interface{}{
			"source_shard": sourceShard,
			"target_shard": targetShard,
			"resource_id":  resourceID,
		},
	}

	return ehi.errorHandler.HandleError(ctx, errorCtx)
}

// HandleNetworkPartition handles network partition scenarios
func (ehi *ErrorHandlerIntegration) HandleNetworkPartition(ctx context.Context, component string, err error) error {
	errorCtx := &ErrorContext{
		ErrorType:  ErrorTypeNetworkPartition,
		Severity:   ErrorSeverityHigh,
		Component:  component,
		Operation:  "network_communication",
		Timestamp:  time.Now(),
		Error:      err,
		RetryCount: 0,
		Metadata: map[string]interface{}{
			"network_issue": true,
		},
	}

	return ehi.errorHandler.HandleError(ctx, errorCtx)
}

// HandleConfigurationError handles configuration errors
func (ehi *ErrorHandlerIntegration) HandleConfigurationError(ctx context.Context, configType string, err error) error {
	errorCtx := &ErrorContext{
		ErrorType:  ErrorTypeConfigurationError,
		Severity:   ErrorSeverityCritical,
		Component:  "config_manager",
		Operation:  "load_config",
		Timestamp:  time.Now(),
		Error:      err,
		RetryCount: 0,
		Metadata: map[string]interface{}{
			"config_type": configType,
		},
	}

	return ehi.errorHandler.HandleError(ctx, errorCtx)
}

// HandleResourceExhaustion handles resource exhaustion scenarios
func (ehi *ErrorHandlerIntegration) HandleResourceExhaustion(ctx context.Context, component string, resourceType string, err error) error {
	errorCtx := &ErrorContext{
		ErrorType:  ErrorTypeResourceExhaustion,
		Severity:   ErrorSeverityHigh,
		Component:  component,
		Operation:  "resource_allocation",
		Timestamp:  time.Now(),
		Error:      err,
		RetryCount: 0,
		Metadata: map[string]interface{}{
			"resource_type": resourceType,
		},
	}

	return ehi.errorHandler.HandleError(ctx, errorCtx)
}

// HandleSystemOverload handles system overload scenarios
func (ehi *ErrorHandlerIntegration) HandleSystemOverload(ctx context.Context, totalLoad float64, threshold float64, shardCount int) error {
	err := fmt.Errorf("system overload detected: load=%.2f, threshold=%.2f, shards=%d", totalLoad, threshold, shardCount)

	errorCtx := &ErrorContext{
		ErrorType:  ErrorTypeSystemOverload,
		Severity:   ErrorSeverityCritical,
		Component:  "shard_manager",
		Operation:  "load_monitoring",
		Timestamp:  time.Now(),
		Error:      err,
		RetryCount: 0,
		Metadata: map[string]interface{}{
			"total_load":  totalLoad,
			"threshold":   threshold,
			"shard_count": shardCount,
		},
	}

	return ehi.errorHandler.HandleError(ctx, errorCtx)
}

// HandleLeaderElectionFailure handles leader election failures
func (ehi *ErrorHandlerIntegration) HandleLeaderElectionFailure(ctx context.Context, err error) error {
	errorCtx := &ErrorContext{
		ErrorType:  ErrorTypeLeaderElection,
		Severity:   ErrorSeverityHigh,
		Component:  "shard_manager",
		Operation:  "leader_election",
		Timestamp:  time.Now(),
		Error:      err,
		RetryCount: 0,
		Metadata: map[string]interface{}{
			"leader_election": true,
		},
	}

	return ehi.errorHandler.HandleError(ctx, errorCtx)
}

// MonitorAndHandleErrors continuously monitors for errors and handles them
func (ehi *ErrorHandlerIntegration) MonitorAndHandleErrors(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("Starting error monitoring and handling")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping error monitoring")
			return ctx.Err()
		case <-ticker.C:
			if err := ehi.performErrorDetection(ctx); err != nil {
				logger.Error(err, "Error during error detection")
			}
		}
	}
}

// performErrorDetection performs proactive error detection
func (ehi *ErrorHandlerIntegration) performErrorDetection(ctx context.Context) error {
	logger := log.FromContext(ctx)

	// Check for unhealthy shards
	if err := ehi.detectUnhealthyShards(ctx); err != nil {
		logger.Error(err, "Failed to detect unhealthy shards")
	}

	// Check for system overload
	if err := ehi.detectSystemOverload(ctx); err != nil {
		logger.Error(err, "Failed to detect system overload")
	}

	// Check for configuration issues
	if err := ehi.detectConfigurationIssues(ctx); err != nil {
		logger.Error(err, "Failed to detect configuration issues")
	}

	return nil
}

// detectUnhealthyShards detects and handles unhealthy shards
func (ehi *ErrorHandlerIntegration) detectUnhealthyShards(ctx context.Context) error {
	shards, err := ehi.shardManager.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}

	for _, shard := range shards {
		if !shard.IsHealthy() {
			// Handle unhealthy shard
			err := fmt.Errorf("shard %s is unhealthy: %s", shard.Spec.ShardID, shard.Status.HealthStatus.Message)
			if handleErr := ehi.HandleShardRuntimeFailure(ctx, shard.Spec.ShardID, err); handleErr != nil {
				log.Log.Error(handleErr, "Failed to handle unhealthy shard", "shardId", shard.Spec.ShardID)
			}
		}
	}

	return nil
}

// detectSystemOverload detects system overload conditions
func (ehi *ErrorHandlerIntegration) detectSystemOverload(ctx context.Context) error {
	shards, err := ehi.shardManager.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}

	if len(shards) == 0 {
		return nil
	}

	// Calculate total load
	totalLoad := 0.0
	for _, shard := range shards {
		totalLoad += shard.Status.Load
	}
	averageLoad := totalLoad / float64(len(shards))

	// Define overload threshold
	overloadThreshold := 0.9

	if averageLoad > overloadThreshold {
		return ehi.HandleSystemOverload(ctx, averageLoad, overloadThreshold, len(shards))
	}

	return nil
}

// detectConfigurationIssues detects configuration-related issues
func (ehi *ErrorHandlerIntegration) detectConfigurationIssues(ctx context.Context) error {
	// Try to load configuration
	_, err := ehi.configManager.LoadConfig(ctx)
	if err != nil {
		return ehi.HandleConfigurationError(ctx, "shard_config", err)
	}

	return nil
}

// GetErrorStatistics returns error statistics
func (ehi *ErrorHandlerIntegration) GetErrorStatistics() ErrorStatistics {
	activeErrors := ehi.errorHandler.GetActiveErrors()
	errorHistory := ehi.errorHandler.GetErrorHistory()

	stats := ErrorStatistics{
		ActiveErrorCount:  len(activeErrors),
		TotalErrorCount:   len(errorHistory),
		ErrorsByType:      make(map[ErrorType]int),
		ErrorsBySeverity:  make(map[ErrorSeverity]int),
		ErrorsByComponent: make(map[string]int),
	}

	// Count errors by type, severity, and component
	for _, errorCtx := range errorHistory {
		stats.ErrorsByType[errorCtx.ErrorType]++
		stats.ErrorsBySeverity[errorCtx.Severity]++
		stats.ErrorsByComponent[errorCtx.Component]++
	}

	// Calculate recent error rate (last hour)
	recentThreshold := time.Now().Add(-1 * time.Hour)
	recentErrors := 0
	for _, errorCtx := range errorHistory {
		if errorCtx.Timestamp.After(recentThreshold) {
			recentErrors++
		}
	}
	stats.RecentErrorRate = float64(recentErrors) / 60.0 // errors per minute

	return stats
}

// ErrorStatistics contains error statistics
type ErrorStatistics struct {
	ActiveErrorCount  int                   `json:"active_error_count"`
	TotalErrorCount   int                   `json:"total_error_count"`
	RecentErrorRate   float64               `json:"recent_error_rate"`
	ErrorsByType      map[ErrorType]int     `json:"errors_by_type"`
	ErrorsBySeverity  map[ErrorSeverity]int `json:"errors_by_severity"`
	ErrorsByComponent map[string]int        `json:"errors_by_component"`
}

// HealthCheckErrorDetector integrates with health checker to detect and handle errors
type HealthCheckErrorDetector struct {
	errorIntegration *ErrorHandlerIntegration
}

// NewHealthCheckErrorDetector creates a new health check error detector
func NewHealthCheckErrorDetector(errorIntegration *ErrorHandlerIntegration) *HealthCheckErrorDetector {
	return &HealthCheckErrorDetector{
		errorIntegration: errorIntegration,
	}
}

// OnShardFailed handles shard failure events from health checker
func (hced *HealthCheckErrorDetector) OnShardFailed(ctx context.Context, shardID string) error {
	err := fmt.Errorf("shard %s failed health check", shardID)
	return hced.errorIntegration.HandleShardRuntimeFailure(ctx, shardID, err)
}

// OnShardRecovered handles shard recovery events from health checker
func (hced *HealthCheckErrorDetector) OnShardRecovered(ctx context.Context, shardID string) error {
	logger := log.FromContext(ctx)
	logger.Info("Shard recovered from failure", "shardId", shardID)

	// Clear any active errors for this shard
	activeErrors := hced.errorIntegration.errorHandler.GetActiveErrors()
	for errorID, errorCtx := range activeErrors {
		if errorCtx.ShardID == shardID {
			hced.errorIntegration.errorHandler.removeActiveError(errorID)
			logger.Info("Cleared active error for recovered shard", "shardId", shardID, "errorId", errorID)
		}
	}

	return nil
}

// ConfigurationErrorDetector integrates with config manager to detect configuration errors
type ConfigurationErrorDetector struct {
	errorIntegration *ErrorHandlerIntegration
}

// NewConfigurationErrorDetector creates a new configuration error detector
func NewConfigurationErrorDetector(errorIntegration *ErrorHandlerIntegration) *ConfigurationErrorDetector {
	return &ConfigurationErrorDetector{
		errorIntegration: errorIntegration,
	}
}

// OnConfigurationError handles configuration error events
func (ced *ConfigurationErrorDetector) OnConfigurationError(ctx context.Context, configType string, err error) error {
	return ced.errorIntegration.HandleConfigurationError(ctx, configType, err)
}

// OnConfigurationChanged handles configuration change events
func (ced *ConfigurationErrorDetector) OnConfigurationChanged(ctx context.Context, configType string, changes map[string]interface{}) error {
	logger := log.FromContext(ctx)
	logger.Info("Configuration changed", "configType", configType, "changes", changes)

	// Validate new configuration
	config, err := ced.errorIntegration.configManager.LoadConfig(ctx)
	if err != nil {
		return ced.OnConfigurationError(ctx, configType, err)
	}

	// Validate configuration
	if err := ced.errorIntegration.configManager.ValidateConfig(config); err != nil {
		return ced.OnConfigurationError(ctx, configType, err)
	}

	return nil
}
