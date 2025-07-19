package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// ErrorType represents different types of errors in the system
type ErrorType string

const (
	ErrorTypeShardStartup      ErrorType = "shard_startup"
	ErrorTypeShardRuntime      ErrorType = "shard_runtime"
	ErrorTypeMigrationFailure  ErrorType = "migration_failure"
	ErrorTypeNetworkPartition  ErrorType = "network_partition"
	ErrorTypeConfigurationError ErrorType = "configuration_error"
	ErrorTypeResourceExhaustion ErrorType = "resource_exhaustion"
	ErrorTypeSystemOverload    ErrorType = "system_overload"
	ErrorTypeLeaderElection    ErrorType = "leader_election"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	ErrorSeverityLow      ErrorSeverity = "low"
	ErrorSeverityMedium   ErrorSeverity = "medium"
	ErrorSeverityHigh     ErrorSeverity = "high"
	ErrorSeverityCritical ErrorSeverity = "critical"
)

// RecoveryAction represents the type of recovery action to take
type RecoveryAction string

const (
	RecoveryActionRetry           RecoveryAction = "retry"
	RecoveryActionRestart         RecoveryAction = "restart"
	RecoveryActionMigrate         RecoveryAction = "migrate"
	RecoveryActionScale           RecoveryAction = "scale"
	RecoveryActionManualIntervention RecoveryAction = "manual_intervention"
	RecoveryActionIgnore          RecoveryAction = "ignore"
)

// ErrorContext contains contextual information about an error
type ErrorContext struct {
	ErrorType     ErrorType                `json:"error_type"`
	Severity      ErrorSeverity            `json:"severity"`
	Component     string                   `json:"component"`
	Operation     string                   `json:"operation"`
	ShardID       string                   `json:"shard_id,omitempty"`
	ResourceID    string                   `json:"resource_id,omitempty"`
	Timestamp     time.Time                `json:"timestamp"`
	Error         error                    `json:"error"`
	Metadata      map[string]interface{}   `json:"metadata,omitempty"`
	RetryCount    int                      `json:"retry_count"`
	MaxRetries    int                      `json:"max_retries"`
	LastRetryTime time.Time                `json:"last_retry_time,omitempty"`
}

// RecoveryStrategy defines how to handle a specific type of error
type RecoveryStrategy struct {
	ErrorType      ErrorType      `json:"error_type"`
	MaxRetries     int            `json:"max_retries"`
	RetryDelay     time.Duration  `json:"retry_delay"`
	BackoffFactor  float64        `json:"backoff_factor"`
	MaxRetryDelay  time.Duration  `json:"max_retry_delay"`
	RecoveryAction RecoveryAction `json:"recovery_action"`
	Timeout        time.Duration  `json:"timeout"`
	RequiresManualIntervention bool `json:"requires_manual_intervention"`
}

// ErrorHandler provides unified error handling and recovery mechanisms
type ErrorHandler struct {
	mu                sync.RWMutex
	strategies        map[ErrorType]*RecoveryStrategy
	activeErrors      map[string]*ErrorContext
	errorHistory      []ErrorContext
	maxHistorySize    int
	
	// Dependencies
	alertManager     interfaces.AlertManager
	structuredLogger interfaces.StructuredLogger
	metricsCollector interfaces.MetricsCollector
	
	// Recovery handlers
	recoveryHandlers map[RecoveryAction]RecoveryHandler
	
	// Circuit breaker state
	circuitBreakers map[string]*CircuitBreaker
}

// RecoveryHandler defines the interface for recovery action handlers
type RecoveryHandler interface {
	Handle(ctx context.Context, errorCtx *ErrorContext) error
	CanHandle(errorCtx *ErrorContext) bool
}

// CircuitBreaker implements circuit breaker pattern for error handling
type CircuitBreaker struct {
	mu              sync.RWMutex
	failureCount    int
	lastFailureTime time.Time
	state           CircuitBreakerState
	threshold       int
	timeout         time.Duration
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerClosed    CircuitBreakerState = "closed"
	CircuitBreakerOpen      CircuitBreakerState = "open"
	CircuitBreakerHalfOpen  CircuitBreakerState = "half_open"
)

// NewErrorHandler creates a new error handler with default strategies
func NewErrorHandler(
	alertManager interfaces.AlertManager,
	structuredLogger interfaces.StructuredLogger,
	metricsCollector interfaces.MetricsCollector,
) *ErrorHandler {
	eh := &ErrorHandler{
		strategies:       make(map[ErrorType]*RecoveryStrategy),
		activeErrors:     make(map[string]*ErrorContext),
		errorHistory:     make([]ErrorContext, 0),
		maxHistorySize:   1000,
		alertManager:     alertManager,
		structuredLogger: structuredLogger,
		metricsCollector: metricsCollector,
		recoveryHandlers: make(map[RecoveryAction]RecoveryHandler),
		circuitBreakers:  make(map[string]*CircuitBreaker),
	}
	
	// Initialize default recovery strategies
	eh.initializeDefaultStrategies()
	
	return eh
}

// initializeDefaultStrategies sets up default recovery strategies for different error types
func (eh *ErrorHandler) initializeDefaultStrategies() {
	strategies := map[ErrorType]*RecoveryStrategy{
		ErrorTypeShardStartup: {
			ErrorType:      ErrorTypeShardStartup,
			MaxRetries:     3,
			RetryDelay:     30 * time.Second,
			BackoffFactor:  2.0,
			MaxRetryDelay:  5 * time.Minute,
			RecoveryAction: RecoveryActionRetry,
			Timeout:        10 * time.Minute,
			RequiresManualIntervention: false,
		},
		ErrorTypeShardRuntime: {
			ErrorType:      ErrorTypeShardRuntime,
			MaxRetries:     2,
			RetryDelay:     15 * time.Second,
			BackoffFactor:  1.5,
			MaxRetryDelay:  2 * time.Minute,
			RecoveryAction: RecoveryActionRestart,
			Timeout:        5 * time.Minute,
			RequiresManualIntervention: false,
		},
		ErrorTypeMigrationFailure: {
			ErrorType:      ErrorTypeMigrationFailure,
			MaxRetries:     3,
			RetryDelay:     1 * time.Minute,
			BackoffFactor:  2.0,
			MaxRetryDelay:  10 * time.Minute,
			RecoveryAction: RecoveryActionRetry,
			Timeout:        30 * time.Minute,
			RequiresManualIntervention: false,
		},
		ErrorTypeNetworkPartition: {
			ErrorType:      ErrorTypeNetworkPartition,
			MaxRetries:     5,
			RetryDelay:     30 * time.Second,
			BackoffFactor:  1.2,
			MaxRetryDelay:  5 * time.Minute,
			RecoveryAction: RecoveryActionRetry,
			Timeout:        15 * time.Minute,
			RequiresManualIntervention: false,
		},
		ErrorTypeConfigurationError: {
			ErrorType:      ErrorTypeConfigurationError,
			MaxRetries:     1,
			RetryDelay:     5 * time.Second,
			BackoffFactor:  1.0,
			MaxRetryDelay:  5 * time.Second,
			RecoveryAction: RecoveryActionManualIntervention,
			Timeout:        1 * time.Minute,
			RequiresManualIntervention: true,
		},
		ErrorTypeResourceExhaustion: {
			ErrorType:      ErrorTypeResourceExhaustion,
			MaxRetries:     2,
			RetryDelay:     2 * time.Minute,
			BackoffFactor:  2.0,
			MaxRetryDelay:  10 * time.Minute,
			RecoveryAction: RecoveryActionScale,
			Timeout:        20 * time.Minute,
			RequiresManualIntervention: false,
		},
		ErrorTypeSystemOverload: {
			ErrorType:      ErrorTypeSystemOverload,
			MaxRetries:     1,
			RetryDelay:     5 * time.Minute,
			BackoffFactor:  1.0,
			MaxRetryDelay:  5 * time.Minute,
			RecoveryAction: RecoveryActionScale,
			Timeout:        30 * time.Minute,
			RequiresManualIntervention: false,
		},
		ErrorTypeLeaderElection: {
			ErrorType:      ErrorTypeLeaderElection,
			MaxRetries:     5,
			RetryDelay:     10 * time.Second,
			BackoffFactor:  1.5,
			MaxRetryDelay:  2 * time.Minute,
			RecoveryAction: RecoveryActionRetry,
			Timeout:        10 * time.Minute,
			RequiresManualIntervention: false,
		},
	}
	
	for errorType, strategy := range strategies {
		eh.strategies[errorType] = strategy
	}
}

// HandleError processes an error and initiates appropriate recovery actions
func (eh *ErrorHandler) HandleError(ctx context.Context, errorCtx *ErrorContext) error {
	if errorCtx == nil {
		return fmt.Errorf("error context cannot be nil")
	}
	
	logger := log.FromContext(ctx).WithValues(
		"errorType", errorCtx.ErrorType,
		"component", errorCtx.Component,
		"operation", errorCtx.Operation,
		"severity", errorCtx.Severity,
	)
	
	// Set timestamp if not already set
	if errorCtx.Timestamp.IsZero() {
		errorCtx.Timestamp = time.Now()
	}
	
	// Generate unique error ID
	errorID := eh.generateErrorID(errorCtx)
	
	logger.Info("Handling error", "errorID", errorID, "error", errorCtx.Error.Error())
	
	// Record error metrics
	eh.recordErrorMetrics(errorCtx)
	
	// Log structured error event
	eh.logErrorEvent(ctx, errorCtx)
	
	// Check circuit breaker
	if eh.isCircuitBreakerOpen(errorCtx) {
		logger.Info("Circuit breaker is open, skipping recovery", "errorID", errorID)
		return eh.handleCircuitBreakerOpen(ctx, errorCtx)
	}
	
	// Get recovery strategy
	strategy := eh.getRecoveryStrategy(errorCtx.ErrorType)
	if strategy == nil {
		logger.Error(fmt.Errorf("no recovery strategy found"), "No recovery strategy", "errorType", errorCtx.ErrorType)
		return eh.handleUnknownError(ctx, errorCtx)
	}
	
	// Update error context with strategy info
	errorCtx.MaxRetries = strategy.MaxRetries
	
	// Store active error
	eh.storeActiveError(errorID, errorCtx)
	
	// Determine if immediate manual intervention is required
	if strategy.RequiresManualIntervention || eh.shouldTriggerManualIntervention(errorCtx) {
		return eh.triggerManualIntervention(ctx, errorCtx)
	}
	
	// Attempt recovery
	return eh.attemptRecovery(ctx, errorCtx, strategy)
}

// attemptRecovery attempts to recover from an error using the specified strategy
func (eh *ErrorHandler) attemptRecovery(ctx context.Context, errorCtx *ErrorContext, strategy *RecoveryStrategy) error {
	logger := log.FromContext(ctx).WithValues("errorType", errorCtx.ErrorType, "recoveryAction", strategy.RecoveryAction)
	
	// Check if we've exceeded max retries
	if errorCtx.RetryCount >= strategy.MaxRetries {
		logger.Info("Max retries exceeded, triggering manual intervention")
		return eh.triggerManualIntervention(ctx, errorCtx)
	}
	
	// Calculate retry delay with exponential backoff
	retryDelay := eh.calculateRetryDelay(strategy, errorCtx.RetryCount)
	
	// Wait for retry delay if this is a retry
	if errorCtx.RetryCount > 0 {
		logger.Info("Waiting before retry", "delay", retryDelay, "retryCount", errorCtx.RetryCount)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}
	
	// Increment retry count
	errorCtx.RetryCount++
	errorCtx.LastRetryTime = time.Now()
	
	// Get recovery handler
	handler := eh.getRecoveryHandler(strategy.RecoveryAction)
	if handler == nil {
		logger.Error(fmt.Errorf("no recovery handler found"), "No recovery handler", "recoveryAction", strategy.RecoveryAction)
		return eh.triggerManualIntervention(ctx, errorCtx)
	}
	
	// Create timeout context for recovery
	recoveryCtx, cancel := context.WithTimeout(ctx, strategy.Timeout)
	defer cancel()
	
	logger.Info("Attempting recovery", "recoveryAction", strategy.RecoveryAction, "retryCount", errorCtx.RetryCount)
	
	// Attempt recovery
	if err := handler.Handle(recoveryCtx, errorCtx); err != nil {
		logger.Error(err, "Recovery attempt failed")
		
		// Update circuit breaker
		eh.recordFailure(errorCtx)
		
		// Record failed recovery metrics
		eh.metricsCollector.RecordCustomMetric("error_recovery_failed_total", 1, map[string]string{
			"error_type":      string(errorCtx.ErrorType),
			"recovery_action": string(strategy.RecoveryAction),
			"component":       errorCtx.Component,
		})
		
		// Log recovery failure
		eh.structuredLogger.LogErrorEvent(recoveryCtx, "error_handler", "recovery_failed", err, map[string]interface{}{
			"error_type":      errorCtx.ErrorType,
			"recovery_action": strategy.RecoveryAction,
			"retry_count":     errorCtx.RetryCount,
			"max_retries":     strategy.MaxRetries,
		})
		
		// Retry if we haven't exceeded max retries
		if errorCtx.RetryCount < strategy.MaxRetries {
			return eh.attemptRecovery(ctx, errorCtx, strategy)
		}
		
		// Max retries exceeded, trigger manual intervention
		return eh.triggerManualIntervention(ctx, errorCtx)
	}
	
	// Recovery successful
	logger.Info("Recovery successful", "recoveryAction", strategy.RecoveryAction, "retryCount", errorCtx.RetryCount)
	
	// Update circuit breaker
	eh.recordSuccess(errorCtx)
	
	// Record successful recovery metrics
	eh.metricsCollector.RecordCustomMetric("error_recovery_success_total", 1, map[string]string{
		"error_type":      string(errorCtx.ErrorType),
		"recovery_action": string(strategy.RecoveryAction),
		"component":       errorCtx.Component,
	})
	
	// Log successful recovery
	eh.structuredLogger.LogSystemEvent(ctx, "error_recovery_success", "info", map[string]interface{}{
		"error_type":      errorCtx.ErrorType,
		"recovery_action": strategy.RecoveryAction,
		"retry_count":     errorCtx.RetryCount,
		"component":       errorCtx.Component,
	})
	
	// Remove from active errors
	eh.removeActiveError(eh.generateErrorID(errorCtx))
	
	// Add to history
	eh.addToHistory(*errorCtx)
	
	return nil
}

// triggerManualIntervention triggers manual intervention for critical errors
func (eh *ErrorHandler) triggerManualIntervention(ctx context.Context, errorCtx *ErrorContext) error {
	logger := log.FromContext(ctx).WithValues("errorType", errorCtx.ErrorType)
	
	logger.Error(errorCtx.Error, "Manual intervention required", 
		"component", errorCtx.Component,
		"operation", errorCtx.Operation,
		"retryCount", errorCtx.RetryCount)
	
	// Send critical alert
	alert := interfaces.Alert{
		Title:     fmt.Sprintf("Manual Intervention Required: %s", errorCtx.ErrorType),
		Message:   fmt.Sprintf("Component: %s, Operation: %s, Error: %s", errorCtx.Component, errorCtx.Operation, errorCtx.Error.Error()),
		Severity:  interfaces.AlertCritical,
		Component: errorCtx.Component,
		Timestamp: time.Now(),
		Labels: map[string]string{
			"error_type":  string(errorCtx.ErrorType),
			"component":   errorCtx.Component,
			"operation":   errorCtx.Operation,
			"shard_id":    errorCtx.ShardID,
			"severity":    string(errorCtx.Severity),
		},
		Annotations: map[string]interface{}{
			"retry_count":    errorCtx.RetryCount,
			"max_retries":    errorCtx.MaxRetries,
			"error_message":  errorCtx.Error.Error(),
			"metadata":       errorCtx.Metadata,
		},
	}
	
	if err := eh.alertManager.SendAlert(ctx, alert); err != nil {
		logger.Error(err, "Failed to send manual intervention alert")
	}
	
	// Record manual intervention metrics
	eh.metricsCollector.RecordCustomMetric("manual_intervention_triggered_total", 1, map[string]string{
		"error_type": string(errorCtx.ErrorType),
		"component":  errorCtx.Component,
		"severity":   string(errorCtx.Severity),
	})
	
	// Log manual intervention event
	eh.structuredLogger.LogSystemEvent(ctx, "manual_intervention_required", "critical", map[string]interface{}{
		"error_type":     errorCtx.ErrorType,
		"component":      errorCtx.Component,
		"operation":      errorCtx.Operation,
		"retry_count":    errorCtx.RetryCount,
		"max_retries":    errorCtx.MaxRetries,
		"error_message":  errorCtx.Error.Error(),
		"metadata":       errorCtx.Metadata,
	})
	
	return fmt.Errorf("manual intervention required for error: %w", errorCtx.Error)
}

// RegisterRecoveryHandler registers a recovery handler for a specific recovery action
func (eh *ErrorHandler) RegisterRecoveryHandler(action RecoveryAction, handler RecoveryHandler) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.recoveryHandlers[action] = handler
}

// SetRecoveryStrategy sets or updates a recovery strategy for an error type
func (eh *ErrorHandler) SetRecoveryStrategy(errorType ErrorType, strategy *RecoveryStrategy) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.strategies[errorType] = strategy
}

// GetActiveErrors returns a copy of currently active errors
func (eh *ErrorHandler) GetActiveErrors() map[string]*ErrorContext {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	
	result := make(map[string]*ErrorContext)
	for id, errorCtx := range eh.activeErrors {
		// Create a copy to prevent external modification
		result[id] = &ErrorContext{
			ErrorType:     errorCtx.ErrorType,
			Severity:      errorCtx.Severity,
			Component:     errorCtx.Component,
			Operation:     errorCtx.Operation,
			ShardID:       errorCtx.ShardID,
			ResourceID:    errorCtx.ResourceID,
			Timestamp:     errorCtx.Timestamp,
			Error:         errorCtx.Error,
			Metadata:      errorCtx.Metadata,
			RetryCount:    errorCtx.RetryCount,
			MaxRetries:    errorCtx.MaxRetries,
			LastRetryTime: errorCtx.LastRetryTime,
		}
	}
	
	return result
}

// GetErrorHistory returns the error history
func (eh *ErrorHandler) GetErrorHistory() []ErrorContext {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	
	// Return a copy to prevent external modification
	history := make([]ErrorContext, len(eh.errorHistory))
	copy(history, eh.errorHistory)
	return history
}

// Helper methods

func (eh *ErrorHandler) generateErrorID(errorCtx *ErrorContext) string {
	return fmt.Sprintf("%s-%s-%s-%d", 
		errorCtx.ErrorType, 
		errorCtx.Component, 
		errorCtx.Operation, 
		errorCtx.Timestamp.Unix())
}

func (eh *ErrorHandler) getRecoveryStrategy(errorType ErrorType) *RecoveryStrategy {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return eh.strategies[errorType]
}

func (eh *ErrorHandler) getRecoveryHandler(action RecoveryAction) RecoveryHandler {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return eh.recoveryHandlers[action]
}

func (eh *ErrorHandler) storeActiveError(errorID string, errorCtx *ErrorContext) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.activeErrors[errorID] = errorCtx
}

func (eh *ErrorHandler) removeActiveError(errorID string) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	delete(eh.activeErrors, errorID)
}

func (eh *ErrorHandler) addToHistory(errorCtx ErrorContext) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	
	eh.errorHistory = append(eh.errorHistory, errorCtx)
	
	// Trim history if it exceeds max size
	if len(eh.errorHistory) > eh.maxHistorySize {
		eh.errorHistory = eh.errorHistory[len(eh.errorHistory)-eh.maxHistorySize:]
	}
}

func (eh *ErrorHandler) calculateRetryDelay(strategy *RecoveryStrategy, retryCount int) time.Duration {
	delay := strategy.RetryDelay
	for i := 0; i < retryCount; i++ {
		delay = time.Duration(float64(delay) * strategy.BackoffFactor)
		if delay > strategy.MaxRetryDelay {
			delay = strategy.MaxRetryDelay
			break
		}
	}
	return delay
}

func (eh *ErrorHandler) shouldTriggerManualIntervention(errorCtx *ErrorContext) bool {
	// Check if error severity is critical
	if errorCtx.Severity == ErrorSeverityCritical {
		return true
	}
	
	// Check if this is a repeated error pattern
	if eh.isRepeatedErrorPattern(errorCtx) {
		return true
	}
	
	return false
}

func (eh *ErrorHandler) isRepeatedErrorPattern(errorCtx *ErrorContext) bool {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	
	// Check recent history for similar errors
	recentThreshold := time.Now().Add(-1 * time.Hour)
	similarErrorCount := 0
	
	for _, historyItem := range eh.errorHistory {
		if historyItem.Timestamp.After(recentThreshold) &&
			historyItem.ErrorType == errorCtx.ErrorType &&
			historyItem.Component == errorCtx.Component {
			similarErrorCount++
		}
	}
	
	// If we've seen this error type more than 5 times in the last hour, trigger manual intervention
	return similarErrorCount > 5
}

func (eh *ErrorHandler) handleUnknownError(ctx context.Context, errorCtx *ErrorContext) error {
	logger := log.FromContext(ctx)
	logger.Error(errorCtx.Error, "Unknown error type, no recovery strategy available", "errorType", errorCtx.ErrorType)
	
	// Send alert for unknown error
	alert := interfaces.Alert{
		Title:     fmt.Sprintf("Unknown Error Type: %s", errorCtx.ErrorType),
		Message:   fmt.Sprintf("No recovery strategy defined for error type %s in component %s", errorCtx.ErrorType, errorCtx.Component),
		Severity:  interfaces.AlertWarning,
		Component: errorCtx.Component,
		Timestamp: time.Now(),
		Labels: map[string]string{
			"error_type": string(errorCtx.ErrorType),
			"component":  errorCtx.Component,
		},
	}
	
	if err := eh.alertManager.SendAlert(ctx, alert); err != nil {
		logger.Error(err, "Failed to send unknown error alert")
	}
	
	return fmt.Errorf("unknown error type: %s", errorCtx.ErrorType)
}

func (eh *ErrorHandler) recordErrorMetrics(errorCtx *ErrorContext) {
	eh.metricsCollector.RecordError(errorCtx.Component, string(errorCtx.ErrorType))
	
	eh.metricsCollector.RecordCustomMetric("error_total", 1, map[string]string{
		"error_type": string(errorCtx.ErrorType),
		"component":  errorCtx.Component,
		"operation":  errorCtx.Operation,
		"severity":   string(errorCtx.Severity),
	})
}

func (eh *ErrorHandler) logErrorEvent(ctx context.Context, errorCtx *ErrorContext) {
	eh.structuredLogger.LogErrorEvent(ctx, errorCtx.Component, errorCtx.Operation, errorCtx.Error, map[string]interface{}{
		"error_type":   errorCtx.ErrorType,
		"severity":     errorCtx.Severity,
		"shard_id":     errorCtx.ShardID,
		"resource_id":  errorCtx.ResourceID,
		"retry_count":  errorCtx.RetryCount,
		"max_retries":  errorCtx.MaxRetries,
		"metadata":     errorCtx.Metadata,
	})
}

// Circuit Breaker Implementation

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		state:     CircuitBreakerClosed,
	}
}

// isCircuitBreakerOpen checks if the circuit breaker is open for a given error context
func (eh *ErrorHandler) isCircuitBreakerOpen(errorCtx *ErrorContext) bool {
	key := fmt.Sprintf("%s-%s", errorCtx.Component, errorCtx.ErrorType)
	
	eh.mu.RLock()
	cb, exists := eh.circuitBreakers[key]
	eh.mu.RUnlock()
	
	if !exists {
		// Create new circuit breaker
		cb = NewCircuitBreaker(5, 5*time.Minute) // 5 failures, 5 minute timeout
		eh.mu.Lock()
		eh.circuitBreakers[key] = cb
		eh.mu.Unlock()
	}
	
	return cb.IsOpen()
}

// recordFailure records a failure in the circuit breaker
func (eh *ErrorHandler) recordFailure(errorCtx *ErrorContext) {
	key := fmt.Sprintf("%s-%s", errorCtx.Component, errorCtx.ErrorType)
	
	eh.mu.RLock()
	cb, exists := eh.circuitBreakers[key]
	eh.mu.RUnlock()
	
	if exists {
		cb.RecordFailure()
	}
}

// recordSuccess records a success in the circuit breaker
func (eh *ErrorHandler) recordSuccess(errorCtx *ErrorContext) {
	key := fmt.Sprintf("%s-%s", errorCtx.Component, errorCtx.ErrorType)
	
	eh.mu.RLock()
	cb, exists := eh.circuitBreakers[key]
	eh.mu.RUnlock()
	
	if exists {
		cb.RecordSuccess()
	}
}

// handleCircuitBreakerOpen handles the case when circuit breaker is open
func (eh *ErrorHandler) handleCircuitBreakerOpen(ctx context.Context, errorCtx *ErrorContext) error {
	logger := log.FromContext(ctx)
	logger.Info("Circuit breaker is open, preventing further recovery attempts",
		"component", errorCtx.Component,
		"errorType", errorCtx.ErrorType)
	
	// Send circuit breaker alert
	alert := interfaces.Alert{
		Title:     fmt.Sprintf("Circuit Breaker Open: %s", errorCtx.Component),
		Message:   fmt.Sprintf("Circuit breaker is open for %s in component %s due to repeated failures", errorCtx.ErrorType, errorCtx.Component),
		Severity:  interfaces.AlertWarning,
		Component: errorCtx.Component,
		Timestamp: time.Now(),
		Labels: map[string]string{
			"error_type": string(errorCtx.ErrorType),
			"component":  errorCtx.Component,
			"state":      "circuit_breaker_open",
		},
	}
	
	if err := eh.alertManager.SendAlert(ctx, alert); err != nil {
		logger.Error(err, "Failed to send circuit breaker alert")
	}
	
	// Record circuit breaker metrics
	eh.metricsCollector.RecordCustomMetric("circuit_breaker_open_total", 1, map[string]string{
		"component":  errorCtx.Component,
		"error_type": string(errorCtx.ErrorType),
	})
	
	return fmt.Errorf("circuit breaker is open for %s in component %s", errorCtx.ErrorType, errorCtx.Component)
}

// Circuit Breaker methods

// IsOpen returns true if the circuit breaker is open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	switch cb.state {
	case CircuitBreakerOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = CircuitBreakerHalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
			return false
		}
		return true
	case CircuitBreakerHalfOpen:
		return false
	default:
		return false
	}
}

// RecordFailure records a failure and potentially opens the circuit breaker
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	
	if cb.state == CircuitBreakerHalfOpen {
		// Failed in half-open state, go back to open
		cb.state = CircuitBreakerOpen
	} else if cb.failureCount >= cb.threshold {
		// Threshold reached, open the circuit breaker
		cb.state = CircuitBreakerOpen
	}
}

// RecordSuccess records a success and potentially closes the circuit breaker
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failureCount = 0
	
	if cb.state == CircuitBreakerHalfOpen {
		// Success in half-open state, close the circuit breaker
		cb.state = CircuitBreakerClosed
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount returns the current failure count
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// Recovery Handlers

// RetryRecoveryHandler handles retry recovery actions
type RetryRecoveryHandler struct {
	shardManager interfaces.ShardManager
}

// NewRetryRecoveryHandler creates a new retry recovery handler
func NewRetryRecoveryHandler(shardManager interfaces.ShardManager) *RetryRecoveryHandler {
	return &RetryRecoveryHandler{
		shardManager: shardManager,
	}
}

// Handle implements the RecoveryHandler interface for retry actions
func (h *RetryRecoveryHandler) Handle(ctx context.Context, errorCtx *ErrorContext) error {
	logger := log.FromContext(ctx).WithValues("recoveryAction", "retry")
	
	switch errorCtx.ErrorType {
	case ErrorTypeShardStartup:
		return h.handleShardStartupRetry(ctx, errorCtx)
	case ErrorTypeMigrationFailure:
		return h.handleMigrationRetry(ctx, errorCtx)
	case ErrorTypeNetworkPartition:
		return h.handleNetworkPartitionRetry(ctx, errorCtx)
	case ErrorTypeLeaderElection:
		return h.handleLeaderElectionRetry(ctx, errorCtx)
	default:
		logger.Info("Generic retry for error type", "errorType", errorCtx.ErrorType)
		// Generic retry - just wait and return success to indicate retry should continue
		return nil
	}
}

// CanHandle returns true if this handler can handle the given error context
func (h *RetryRecoveryHandler) CanHandle(errorCtx *ErrorContext) bool {
	retryableErrors := []ErrorType{
		ErrorTypeShardStartup,
		ErrorTypeMigrationFailure,
		ErrorTypeNetworkPartition,
		ErrorTypeLeaderElection,
	}
	
	for _, errorType := range retryableErrors {
		if errorCtx.ErrorType == errorType {
			return true
		}
	}
	
	return false
}

func (h *RetryRecoveryHandler) handleShardStartupRetry(ctx context.Context, errorCtx *ErrorContext) error {
	if errorCtx.ShardID == "" {
		return fmt.Errorf("shard ID is required for shard startup retry")
	}
	
	// Check if shard exists and its current status
	status, err := h.shardManager.GetShardStatus(ctx, errorCtx.ShardID)
	if err != nil {
		return fmt.Errorf("failed to get shard status: %w", err)
	}
	
	// If shard is already running, consider retry successful
	if status.Phase == shardv1.ShardPhaseRunning {
		return nil
	}
	
	// If shard is in failed state, attempt to restart it
	if status.Phase == shardv1.ShardPhaseFailed {
		// This would typically involve restarting the shard pod
		// For now, we'll just check health to trigger recovery
		_, err := h.shardManager.CheckShardHealth(ctx, errorCtx.ShardID)
		return err
	}
	
	return fmt.Errorf("shard %s is in unexpected phase: %s", errorCtx.ShardID, status.Phase)
}

func (h *RetryRecoveryHandler) handleMigrationRetry(ctx context.Context, errorCtx *ErrorContext) error {
	// Migration retry would typically involve re-attempting the failed migration
	// This is a simplified implementation
	if errorCtx.ResourceID == "" {
		return fmt.Errorf("resource ID is required for migration retry")
	}
	
	// In a real implementation, this would re-attempt the specific migration
	// For now, we'll just return success to indicate the retry should continue
	return nil
}

func (h *RetryRecoveryHandler) handleNetworkPartitionRetry(ctx context.Context, errorCtx *ErrorContext) error {
	// Network partition retry would typically involve checking connectivity
	// and waiting for network to recover
	
	// Simulate network connectivity check
	time.Sleep(1 * time.Second)
	
	// In a real implementation, this would check actual network connectivity
	// For now, we'll assume network has recovered
	return nil
}

func (h *RetryRecoveryHandler) handleLeaderElectionRetry(ctx context.Context, errorCtx *ErrorContext) error {
	// Leader election retry would typically involve re-attempting leader election
	// This is handled by the leader election mechanism itself
	return nil
}

// RestartRecoveryHandler handles restart recovery actions
type RestartRecoveryHandler struct {
	shardManager interfaces.ShardManager
}

// NewRestartRecoveryHandler creates a new restart recovery handler
func NewRestartRecoveryHandler(shardManager interfaces.ShardManager) *RestartRecoveryHandler {
	return &RestartRecoveryHandler{
		shardManager: shardManager,
	}
}

// Handle implements the RecoveryHandler interface for restart actions
func (h *RestartRecoveryHandler) Handle(ctx context.Context, errorCtx *ErrorContext) error {
	logger := log.FromContext(ctx).WithValues("recoveryAction", "restart")
	
	if errorCtx.ShardID == "" {
		return fmt.Errorf("shard ID is required for restart recovery")
	}
	
	logger.Info("Attempting to restart shard", "shardId", errorCtx.ShardID)
	
	// Handle failed shard (this will trigger restart logic)
	if err := h.shardManager.HandleFailedShard(ctx, errorCtx.ShardID); err != nil {
		return fmt.Errorf("failed to handle failed shard for restart: %w", err)
	}
	
	// Wait a moment for restart to take effect
	time.Sleep(5 * time.Second)
	
	// Check if shard is now healthy
	healthStatus, err := h.shardManager.CheckShardHealth(ctx, errorCtx.ShardID)
	if err != nil {
		return fmt.Errorf("failed to check shard health after restart: %w", err)
	}
	
	if !healthStatus.Healthy {
		return fmt.Errorf("shard %s is still unhealthy after restart: %s", errorCtx.ShardID, healthStatus.Message)
	}
	
	logger.Info("Shard restart successful", "shardId", errorCtx.ShardID)
	return nil
}

// CanHandle returns true if this handler can handle the given error context
func (h *RestartRecoveryHandler) CanHandle(errorCtx *ErrorContext) bool {
	restartableErrors := []ErrorType{
		ErrorTypeShardRuntime,
	}
	
	for _, errorType := range restartableErrors {
		if errorCtx.ErrorType == errorType {
			return true
		}
	}
	
	return false
}

// ScaleRecoveryHandler handles scale recovery actions
type ScaleRecoveryHandler struct {
	shardManager interfaces.ShardManager
}

// NewScaleRecoveryHandler creates a new scale recovery handler
func NewScaleRecoveryHandler(shardManager interfaces.ShardManager) *ScaleRecoveryHandler {
	return &ScaleRecoveryHandler{
		shardManager: shardManager,
	}
}

// Handle implements the RecoveryHandler interface for scale actions
func (h *ScaleRecoveryHandler) Handle(ctx context.Context, errorCtx *ErrorContext) error {
	switch errorCtx.ErrorType {
	case ErrorTypeResourceExhaustion:
		return h.handleResourceExhaustionScale(ctx, errorCtx)
	case ErrorTypeSystemOverload:
		return h.handleSystemOverloadScale(ctx, errorCtx)
	default:
		return fmt.Errorf("scale recovery not supported for error type: %s", errorCtx.ErrorType)
	}
}

// CanHandle returns true if this handler can handle the given error context
func (h *ScaleRecoveryHandler) CanHandle(errorCtx *ErrorContext) bool {
	scalableErrors := []ErrorType{
		ErrorTypeResourceExhaustion,
		ErrorTypeSystemOverload,
	}
	
	for _, errorType := range scalableErrors {
		if errorCtx.ErrorType == errorType {
			return true
		}
	}
	
	return false
}

func (h *ScaleRecoveryHandler) handleResourceExhaustionScale(ctx context.Context, errorCtx *ErrorContext) error {
	logger := log.FromContext(ctx)
	
	// Get current shard count
	shards, err := h.shardManager.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}
	
	currentCount := len(shards)
	targetCount := currentCount + 1
	
	logger.Info("Scaling up due to resource exhaustion", "from", currentCount, "to", targetCount)
	
	// Scale up by one shard
	if err := h.shardManager.ScaleUp(ctx, targetCount); err != nil {
		return fmt.Errorf("failed to scale up: %w", err)
	}
	
	logger.Info("Scale up completed", "newShardCount", targetCount)
	return nil
}

func (h *ScaleRecoveryHandler) handleSystemOverloadScale(ctx context.Context, errorCtx *ErrorContext) error {
	logger := log.FromContext(ctx)
	
	// Get current shard count
	shards, err := h.shardManager.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list shards: %w", err)
	}
	
	currentCount := len(shards)
	// Scale up more aggressively for system overload
	targetCount := currentCount + 2
	
	logger.Info("Scaling up due to system overload", "from", currentCount, "to", targetCount)
	
	// Scale up by two shards
	if err := h.shardManager.ScaleUp(ctx, targetCount); err != nil {
		return fmt.Errorf("failed to scale up: %w", err)
	}
	
	logger.Info("Scale up completed", "newShardCount", targetCount)
	return nil
}

// MigrateRecoveryHandler handles migrate recovery actions
type MigrateRecoveryHandler struct {
	resourceMigrator interfaces.ResourceMigrator
	shardManager     interfaces.ShardManager
}

// NewMigrateRecoveryHandler creates a new migrate recovery handler
func NewMigrateRecoveryHandler(resourceMigrator interfaces.ResourceMigrator, shardManager interfaces.ShardManager) *MigrateRecoveryHandler {
	return &MigrateRecoveryHandler{
		resourceMigrator: resourceMigrator,
		shardManager:     shardManager,
	}
}

// Handle implements the RecoveryHandler interface for migrate actions
func (h *MigrateRecoveryHandler) Handle(ctx context.Context, errorCtx *ErrorContext) error {
	logger := log.FromContext(ctx).WithValues("recoveryAction", "migrate")
	
	if errorCtx.ShardID == "" {
		return fmt.Errorf("shard ID is required for migration recovery")
	}
	
	logger.Info("Attempting resource migration recovery", "shardId", errorCtx.ShardID)
	
	// Get the failed shard
	shardStatus, err := h.shardManager.GetShardStatus(ctx, errorCtx.ShardID)
	if err != nil {
		return fmt.Errorf("failed to get shard status: %w", err)
	}
	
	// If shard has resources, migrate them
	if len(shardStatus.AssignedResources) > 0 {
		// Get healthy target shards
		allShards, err := h.shardManager.ListShards(ctx)
		if err != nil {
			return fmt.Errorf("failed to list shards: %w", err)
		}
		
		var targetShards []*shardv1.ShardInstance
		for _, shard := range allShards {
			if shard.Spec.ShardID != errorCtx.ShardID && shard.IsHealthy() && shard.IsActive() {
				targetShards = append(targetShards, shard)
			}
		}
		
		if len(targetShards) == 0 {
			return fmt.Errorf("no healthy target shards available for migration")
		}
		
		// Create and execute migration plan for each resource
		for i, resourceId := range shardStatus.AssignedResources {
			targetShard := targetShards[i%len(targetShards)]
			
			resource := &interfaces.Resource{
				ID:   resourceId,
				Type: "generic", // In a real implementation, determine actual type
			}
			
			plan, err := h.resourceMigrator.CreateMigrationPlan(
				ctx,
				errorCtx.ShardID,
				targetShard.Spec.ShardID,
				[]*interfaces.Resource{resource},
			)
			if err != nil {
				return fmt.Errorf("failed to create migration plan: %w", err)
			}
			
			if err := h.resourceMigrator.ExecuteMigration(ctx, plan); err != nil {
				return fmt.Errorf("failed to execute migration: %w", err)
			}
		}
		
		logger.Info("Resource migration completed", "resourceCount", len(shardStatus.AssignedResources))
	}
	
	return nil
}

// CanHandle returns true if this handler can handle the given error context
func (h *MigrateRecoveryHandler) CanHandle(errorCtx *ErrorContext) bool {
	migratableErrors := []ErrorType{
		ErrorTypeMigrationFailure,
		ErrorTypeShardRuntime,
	}
	
	for _, errorType := range migratableErrors {
		if errorCtx.ErrorType == errorType {
			return true
		}
	}
	
	return false
}