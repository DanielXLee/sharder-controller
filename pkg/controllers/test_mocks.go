package controllers

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// Common mock implementations for testing

type TestMockAlertManager struct {
	mock.Mock
}

func (m *TestMockAlertManager) SendAlert(ctx context.Context, alert interfaces.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

func (m *TestMockAlertManager) AlertShardFailure(ctx context.Context, shardID string, reason string) error {
	args := m.Called(ctx, shardID, reason)
	return args.Error(0)
}

func (m *TestMockAlertManager) AlertHighErrorRate(ctx context.Context, component string, errorRate float64, threshold float64) error {
	args := m.Called(ctx, component, errorRate, threshold)
	return args.Error(0)
}

func (m *TestMockAlertManager) AlertScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string) error {
	args := m.Called(ctx, operation, fromCount, toCount, reason)
	return args.Error(0)
}

func (m *TestMockAlertManager) AlertMigrationFailure(ctx context.Context, sourceShard, targetShard string, resourceCount int, reason string) error {
	args := m.Called(ctx, sourceShard, targetShard, resourceCount, reason)
	return args.Error(0)
}

func (m *TestMockAlertManager) AlertSystemOverload(ctx context.Context, totalLoad float64, threshold float64, shardCount int) error {
	args := m.Called(ctx, totalLoad, threshold, shardCount)
	return args.Error(0)
}

func (m *TestMockAlertManager) AlertConfigurationChange(ctx context.Context, configType string, changes map[string]interface{}) error {
	args := m.Called(ctx, configType, changes)
	return args.Error(0)
}

type TestMockStructuredLogger struct {
	mock.Mock
}

func (m *TestMockStructuredLogger) LogShardEvent(ctx context.Context, event string, shardID string, fields map[string]interface{}) {
	m.Called(ctx, event, shardID, fields)
}

func (m *TestMockStructuredLogger) LogMigrationEvent(ctx context.Context, sourceShard, targetShard string, resourceCount int, status string, duration time.Duration) {
	m.Called(ctx, sourceShard, targetShard, resourceCount, status, duration)
}

func (m *TestMockStructuredLogger) LogScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string, status string) {
	m.Called(ctx, operation, fromCount, toCount, reason, status)
}

func (m *TestMockStructuredLogger) LogHealthEvent(ctx context.Context, shardID string, healthy bool, errorCount int, message string) {
	m.Called(ctx, shardID, healthy, errorCount, message)
}

func (m *TestMockStructuredLogger) LogErrorEvent(ctx context.Context, component, operation string, err error, fields map[string]interface{}) {
	m.Called(ctx, component, operation, err, fields)
}

func (m *TestMockStructuredLogger) LogPerformanceEvent(ctx context.Context, operation, component string, duration time.Duration, success bool) {
	m.Called(ctx, operation, component, duration, success)
}

func (m *TestMockStructuredLogger) LogConfigEvent(ctx context.Context, configType string, changes map[string]interface{}) {
	m.Called(ctx, configType, changes)
}

func (m *TestMockStructuredLogger) LogSystemEvent(ctx context.Context, event string, severity string, fields map[string]interface{}) {
	m.Called(ctx, event, severity, fields)
}

type TestMockMetricsCollector struct {
	mock.Mock
}

func (m *TestMockMetricsCollector) CollectShardMetrics(shard *shardv1.ShardInstance) error {
	args := m.Called(shard)
	return args.Error(0)
}

func (m *TestMockMetricsCollector) CollectSystemMetrics(shards []*shardv1.ShardInstance) error {
	args := m.Called(shards)
	return args.Error(0)
}

func (m *TestMockMetricsCollector) ExposeMetrics() error {
	args := m.Called()
	return args.Error(0)
}

func (m *TestMockMetricsCollector) StartMetricsServer(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *TestMockMetricsCollector) RecordMigration(sourceShard, targetShard, status string, duration time.Duration) {
	m.Called(sourceShard, targetShard, status, duration)
}

func (m *TestMockMetricsCollector) RecordScaleOperation(operation, status string) {
	m.Called(operation, status)
}

func (m *TestMockMetricsCollector) RecordError(component, errorType string) {
	m.Called(component, errorType)
}

func (m *TestMockMetricsCollector) RecordOperationDuration(operation, component string, duration time.Duration) {
	m.Called(operation, component, duration)
}

func (m *TestMockMetricsCollector) UpdateQueueLength(queueType, shardID string, length int) {
	m.Called(queueType, shardID, length)
}

func (m *TestMockMetricsCollector) RecordCustomMetric(name string, value float64, labels map[string]string) error {
	args := m.Called(name, value, labels)
	return args.Error(0)
}

type TestMockRecoveryHandler struct {
	mock.Mock
}

func (m *TestMockRecoveryHandler) Handle(ctx context.Context, errorCtx *ErrorContext) error {
	args := m.Called(ctx, errorCtx)
	return args.Error(0)
}

func (m *TestMockRecoveryHandler) CanHandle(errorCtx *ErrorContext) bool {
	args := m.Called(errorCtx)
	return args.Bool(0)
}
