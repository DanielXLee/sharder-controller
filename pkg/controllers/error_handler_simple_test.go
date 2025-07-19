package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestErrorHandler_Basic(t *testing.T) {
	// Create mock dependencies
	mockAlertManager := &TestMockAlertManager{}
	mockLogger := &TestMockStructuredLogger{}
	mockMetrics := &TestMockMetricsCollector{}

	// Create error handler
	eh := NewErrorHandler(mockAlertManager, mockLogger, mockMetrics)

	// Test basic initialization
	assert.NotNil(t, eh)
	assert.NotNil(t, eh.strategies)
	assert.NotNil(t, eh.activeErrors)
	assert.NotNil(t, eh.errorHistory)
	assert.NotNil(t, eh.recoveryHandlers)
	assert.NotNil(t, eh.circuitBreakers)
	assert.Equal(t, 1000, eh.maxHistorySize)

	// Check that default strategies are initialized
	assert.Contains(t, eh.strategies, ErrorTypeShardStartup)
	assert.Contains(t, eh.strategies, ErrorTypeShardRuntime)
	assert.Contains(t, eh.strategies, ErrorTypeMigrationFailure)
	assert.Contains(t, eh.strategies, ErrorTypeNetworkPartition)
	assert.Contains(t, eh.strategies, ErrorTypeConfigurationError)
	assert.Contains(t, eh.strategies, ErrorTypeResourceExhaustion)
	assert.Contains(t, eh.strategies, ErrorTypeSystemOverload)
	assert.Contains(t, eh.strategies, ErrorTypeLeaderElection)
}

func TestErrorHandler_NilErrorContext(t *testing.T) {
	mockAlertManager := &TestMockAlertManager{}
	mockLogger := &TestMockStructuredLogger{}
	mockMetrics := &TestMockMetricsCollector{}

	eh := NewErrorHandler(mockAlertManager, mockLogger, mockMetrics)

	err := eh.HandleError(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error context cannot be nil")
}

func TestErrorHandler_RegisterRecoveryHandler(t *testing.T) {
	mockAlertManager := &TestMockAlertManager{}
	mockLogger := &TestMockStructuredLogger{}
	mockMetrics := &TestMockMetricsCollector{}
	mockHandler := &TestMockRecoveryHandler{}

	eh := NewErrorHandler(mockAlertManager, mockLogger, mockMetrics)

	eh.RegisterRecoveryHandler(RecoveryActionRetry, mockHandler)

	handler := eh.getRecoveryHandler(RecoveryActionRetry)
	assert.Equal(t, mockHandler, handler)
}

func TestErrorHandler_SetRecoveryStrategy(t *testing.T) {
	mockAlertManager := &TestMockAlertManager{}
	mockLogger := &TestMockStructuredLogger{}
	mockMetrics := &TestMockMetricsCollector{}

	eh := NewErrorHandler(mockAlertManager, mockLogger, mockMetrics)

	customStrategy := &RecoveryStrategy{
		ErrorType:      ErrorTypeShardStartup,
		MaxRetries:     5,
		RetryDelay:     1 * time.Minute,
		BackoffFactor:  3.0,
		MaxRetryDelay:  10 * time.Minute,
		RecoveryAction: RecoveryActionRestart,
		Timeout:        20 * time.Minute,
		RequiresManualIntervention: false,
	}

	eh.SetRecoveryStrategy(ErrorTypeShardStartup, customStrategy)

	strategy := eh.getRecoveryStrategy(ErrorTypeShardStartup)
	assert.Equal(t, customStrategy, strategy)
}

func TestErrorHandler_GetActiveErrors(t *testing.T) {
	mockAlertManager := &TestMockAlertManager{}
	mockLogger := &TestMockStructuredLogger{}
	mockMetrics := &TestMockMetricsCollector{}

	eh := NewErrorHandler(mockAlertManager, mockLogger, mockMetrics)

	// Add some active errors
	errorCtx1 := &ErrorContext{
		ErrorType: ErrorTypeShardStartup,
		Component: "test1",
		Error:     errors.New("error1"),
	}
	errorCtx2 := &ErrorContext{
		ErrorType: ErrorTypeShardRuntime,
		Component: "test2",
		Error:     errors.New("error2"),
	}

	eh.storeActiveError("error1", errorCtx1)
	eh.storeActiveError("error2", errorCtx2)

	activeErrors := eh.GetActiveErrors()
	assert.Len(t, activeErrors, 2)
	assert.Contains(t, activeErrors, "error1")
	assert.Contains(t, activeErrors, "error2")
}

func TestErrorHandler_GetErrorHistory(t *testing.T) {
	mockAlertManager := &TestMockAlertManager{}
	mockLogger := &TestMockStructuredLogger{}
	mockMetrics := &TestMockMetricsCollector{}

	eh := NewErrorHandler(mockAlertManager, mockLogger, mockMetrics)

	// Add some history
	errorCtx1 := ErrorContext{
		ErrorType: ErrorTypeShardStartup,
		Component: "test1",
		Error:     errors.New("error1"),
	}
	errorCtx2 := ErrorContext{
		ErrorType: ErrorTypeShardRuntime,
		Component: "test2",
		Error:     errors.New("error2"),
	}

	eh.addToHistory(errorCtx1)
	eh.addToHistory(errorCtx2)

	history := eh.GetErrorHistory()
	assert.Len(t, history, 2)
	assert.Equal(t, ErrorTypeShardStartup, history[0].ErrorType)
	assert.Equal(t, ErrorTypeShardRuntime, history[1].ErrorType)
}

func TestCircuitBreaker_Basic(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Minute)

	// Initially closed
	assert.False(t, cb.IsOpen())
	assert.Equal(t, CircuitBreakerClosed, cb.GetState())
	assert.Equal(t, 0, cb.GetFailureCount())

	// Record failures
	cb.RecordFailure()
	assert.False(t, cb.IsOpen())
	assert.Equal(t, 1, cb.GetFailureCount())

	cb.RecordFailure()
	assert.False(t, cb.IsOpen())
	assert.Equal(t, 2, cb.GetFailureCount())

	// Third failure should open the circuit breaker
	cb.RecordFailure()
	assert.True(t, cb.IsOpen())
	assert.Equal(t, CircuitBreakerOpen, cb.GetState())
	assert.Equal(t, 3, cb.GetFailureCount())

	// Record success should not close it while open
	cb.RecordSuccess()
	assert.True(t, cb.IsOpen())
	assert.Equal(t, 0, cb.GetFailureCount()) // Success resets failure count
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond) // Short timeout for testing

	// Open the circuit breaker
	cb.RecordFailure()
	cb.RecordFailure()
	assert.True(t, cb.IsOpen())

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should transition to half-open
	assert.False(t, cb.IsOpen())
	assert.Equal(t, CircuitBreakerHalfOpen, cb.GetState())

	// Success in half-open should close it
	cb.RecordSuccess()
	assert.False(t, cb.IsOpen())
	assert.Equal(t, CircuitBreakerClosed, cb.GetState())
}

func TestRetryRecoveryHandler_CanHandle(t *testing.T) {
	handler := NewRetryRecoveryHandler(nil)

	testCases := []struct {
		errorType ErrorType
		expected  bool
	}{
		{ErrorTypeShardStartup, true},
		{ErrorTypeMigrationFailure, true},
		{ErrorTypeNetworkPartition, true},
		{ErrorTypeLeaderElection, true},
		{ErrorTypeShardRuntime, false},
		{ErrorTypeConfigurationError, false},
	}

	for _, tc := range testCases {
		errorCtx := &ErrorContext{ErrorType: tc.errorType}
		result := handler.CanHandle(errorCtx)
		assert.Equal(t, tc.expected, result, "ErrorType: %s", tc.errorType)
	}
}

func TestRestartRecoveryHandler_CanHandle(t *testing.T) {
	handler := NewRestartRecoveryHandler(nil)

	testCases := []struct {
		errorType ErrorType
		expected  bool
	}{
		{ErrorTypeShardRuntime, true},
		{ErrorTypeShardStartup, false},
		{ErrorTypeMigrationFailure, false},
	}

	for _, tc := range testCases {
		errorCtx := &ErrorContext{ErrorType: tc.errorType}
		result := handler.CanHandle(errorCtx)
		assert.Equal(t, tc.expected, result, "ErrorType: %s", tc.errorType)
	}
}

func TestScaleRecoveryHandler_CanHandle(t *testing.T) {
	handler := NewScaleRecoveryHandler(nil)

	testCases := []struct {
		errorType ErrorType
		expected  bool
	}{
		{ErrorTypeResourceExhaustion, true},
		{ErrorTypeSystemOverload, true},
		{ErrorTypeShardStartup, false},
		{ErrorTypeShardRuntime, false},
	}

	for _, tc := range testCases {
		errorCtx := &ErrorContext{ErrorType: tc.errorType}
		result := handler.CanHandle(errorCtx)
		assert.Equal(t, tc.expected, result, "ErrorType: %s", tc.errorType)
	}
}

func TestMigrateRecoveryHandler_CanHandle(t *testing.T) {
	handler := NewMigrateRecoveryHandler(nil, nil)

	testCases := []struct {
		errorType ErrorType
		expected  bool
	}{
		{ErrorTypeMigrationFailure, true},
		{ErrorTypeShardRuntime, true},
		{ErrorTypeShardStartup, false},
		{ErrorTypeConfigurationError, false},
	}

	for _, tc := range testCases {
		errorCtx := &ErrorContext{ErrorType: tc.errorType}
		result := handler.CanHandle(errorCtx)
		assert.Equal(t, tc.expected, result, "ErrorType: %s", tc.errorType)
	}
}

func TestErrorHandler_CalculateRetryDelay(t *testing.T) {
	mockAlertManager := &TestMockAlertManager{}
	mockLogger := &TestMockStructuredLogger{}
	mockMetrics := &TestMockMetricsCollector{}

	eh := NewErrorHandler(mockAlertManager, mockLogger, mockMetrics)

	strategy := &RecoveryStrategy{
		RetryDelay:    1 * time.Second,
		BackoffFactor: 2.0,
		MaxRetryDelay: 10 * time.Second,
	}

	testCases := []struct {
		retryCount int
		expected   time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second}, // Should be capped at MaxRetryDelay
		{5, 10 * time.Second}, // Should remain at MaxRetryDelay
	}

	for _, tc := range testCases {
		result := eh.calculateRetryDelay(strategy, tc.retryCount)
		assert.Equal(t, tc.expected, result, "RetryCount: %d", tc.retryCount)
	}
}

func TestErrorHandler_GenerateErrorID(t *testing.T) {
	mockAlertManager := &TestMockAlertManager{}
	mockLogger := &TestMockStructuredLogger{}
	mockMetrics := &TestMockMetricsCollector{}

	eh := NewErrorHandler(mockAlertManager, mockLogger, mockMetrics)

	timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	errorCtx := &ErrorContext{
		ErrorType: ErrorTypeShardStartup,
		Component: "test_component",
		Operation: "test_operation",
		Timestamp: timestamp,
	}

	expectedID := "shard_startup-test_component-test_operation-1672574400"
	actualID := eh.generateErrorID(errorCtx)
	assert.Equal(t, expectedID, actualID)
}