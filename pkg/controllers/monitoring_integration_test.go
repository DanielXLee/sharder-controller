package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
)

func TestMonitoringIntegration(t *testing.T) {
	// Initialize monitoring components
	cfg := config.DefaultConfig()
	logger := logrus.New()
	
	// Create structured logger
	structuredLogger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	// Create metrics collector
	metricsCollector, err := NewMetricsCollector(cfg.Metrics, logger)
	require.NoError(t, err)
	
	// Create alert manager
	alertManager := NewAlertManager(AlertingConfig{
		Enabled:    false, // Disable for testing
		WebhookURL: "",
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}, logger)
	
	// Test metrics collection
	t.Run("MetricsCollection", func(t *testing.T) {
		now := metav1.Now()
		shard := &shardv1.ShardInstance{
			Spec: shardv1.ShardInstanceSpec{
				ShardID: "test-shard-metrics",
			},
			Status: shardv1.ShardInstanceStatus{
				Phase:             shardv1.ShardPhaseRunning,
				Load:              0.7,
				AssignedResources: []string{"res1", "res2", "res3"},
				LastHeartbeat:     now,
			},
		}
		
		err := metricsCollector.CollectShardMetrics(shard)
		assert.NoError(t, err)
		
		// Test system metrics
		shards := []*shardv1.ShardInstance{shard}
		err = metricsCollector.CollectSystemMetrics(shards)
		assert.NoError(t, err)
		
		// Test custom metrics
		err = metricsCollector.RecordCustomMetric("test_metric", 42.0, map[string]string{
			"test_label": "test_value",
		})
		assert.NoError(t, err)
		
		// Test event recording
		metricsCollector.RecordMigration("shard-1", "shard-2", "success", 2*time.Second)
		metricsCollector.RecordScaleOperation("scale_up", "success")
		metricsCollector.RecordError("test_component", "test_error")
		metricsCollector.RecordOperationDuration("test_operation", "test_component", 1*time.Second)
		metricsCollector.UpdateQueueLength("test_queue", "shard-1", 5)
	})
	
	// Test structured logging
	t.Run("StructuredLogging", func(t *testing.T) {
		ctx := context.Background()
		
		// Test shard event logging
		structuredLogger.LogShardEvent(ctx, "test_event", "test-shard", map[string]interface{}{
			"test_field": "test_value",
		})
		
		// Test migration event logging
		structuredLogger.LogMigrationEvent(ctx, "shard-1", "shard-2", 5, "success", 2*time.Second)
		
		// Test scale event logging
		structuredLogger.LogScaleEvent(ctx, "scale_up", 2, 3, "high_load", "success")
		
		// Test health event logging
		structuredLogger.LogHealthEvent(ctx, "test-shard", true, 0, "healthy")
		
		// Test error event logging
		testErr := assert.AnError
		structuredLogger.LogErrorEvent(ctx, "test_component", "test_operation", testErr, map[string]interface{}{
			"context": "test",
		})
		
		// Test performance event logging
		structuredLogger.LogPerformanceEvent(ctx, "test_operation", "test_component", 1*time.Second, true)
		
		// Test config event logging
		changes := map[string]interface{}{
			"test_config": "test_value",
		}
		structuredLogger.LogConfigEvent(ctx, "test_config", changes)
		
		// Test system event logging
		structuredLogger.LogSystemEvent(ctx, "test_system_event", "info", map[string]interface{}{
			"system_field": "system_value",
		})
	})
	
	// Test alerting
	t.Run("Alerting", func(t *testing.T) {
		ctx := context.Background()
		
		// Test shard failure alert
		err := alertManager.AlertShardFailure(ctx, "test-shard", "connection timeout")
		assert.NoError(t, err) // Should not error even when disabled
		
		// Test high error rate alert
		err = alertManager.AlertHighErrorRate(ctx, "test_component", 0.15, 0.10)
		assert.NoError(t, err)
		
		// Test scale event alert
		err = alertManager.AlertScaleEvent(ctx, "scale_up", 2, 4, "high_load")
		assert.NoError(t, err)
		
		// Test migration failure alert
		err = alertManager.AlertMigrationFailure(ctx, "shard-1", "shard-2", 10, "target unavailable")
		assert.NoError(t, err)
		
		// Test system overload alert
		err = alertManager.AlertSystemOverload(ctx, 0.95, 0.80, 5)
		assert.NoError(t, err)
		
		// Test configuration change alert
		changes := map[string]interface{}{
			"scale_threshold": 0.8,
		}
		err = alertManager.AlertConfigurationChange(ctx, "shard_config", changes)
		assert.NoError(t, err)
	})
	
	// Test monitoring integration
	t.Run("MonitoringIntegration", func(t *testing.T) {
		ctx := context.Background()
		
		// Simulate a complete monitoring scenario
		// 1. Create a shard
		now := metav1.Now()
		shard := &shardv1.ShardInstance{
			Spec: shardv1.ShardInstanceSpec{
				ShardID: "integration-test-shard",
			},
			Status: shardv1.ShardInstanceStatus{
				Phase:             shardv1.ShardPhaseRunning,
				Load:              0.8,
				AssignedResources: []string{"res1", "res2"},
				LastHeartbeat:     now,
			},
		}
		
		// 2. Log shard creation
		structuredLogger.LogShardEvent(ctx, "shard_created", shard.Spec.ShardID, map[string]interface{}{
			"initial_load": shard.Status.Load,
			"resources":    len(shard.Status.AssignedResources),
		})
		
		// 3. Collect metrics
		err := metricsCollector.CollectShardMetrics(shard)
		assert.NoError(t, err)
		
		// 4. Simulate high load triggering scale up
		if shard.Status.Load > 0.7 {
			// Log scale decision
			structuredLogger.LogScaleEvent(ctx, "scale_up", 1, 2, "high_load", "initiated")
			
			// Record scale operation
			metricsCollector.RecordScaleOperation("scale_up", "success")
			
			// Send scale alert
			err := alertManager.AlertScaleEvent(ctx, "scale_up", 1, 2, "high_load")
			assert.NoError(t, err)
		}
		
		// 5. Simulate resource migration
		startTime := time.Now()
		
		// Log migration start
		structuredLogger.LogMigrationEvent(ctx, "old-shard", "new-shard", 2, "started", 0)
		
		// Simulate migration completion
		time.Sleep(10 * time.Millisecond) // Simulate migration time
		duration := time.Since(startTime)
		
		// Log migration completion
		structuredLogger.LogMigrationEvent(ctx, "old-shard", "new-shard", 2, "success", duration)
		
		// Record migration metrics
		metricsCollector.RecordMigration("old-shard", "new-shard", "success", duration)
		
		// 6. Test system overload detection
		totalLoad := 0.95
		if totalLoad > 0.9 {
			// Log system overload
			structuredLogger.LogSystemEvent(ctx, "system_overload", "critical", map[string]interface{}{
				"total_load":  totalLoad,
				"threshold":   0.9,
				"shard_count": 2,
			})
			
			// Send overload alert
			err := alertManager.AlertSystemOverload(ctx, totalLoad, 0.9, 2)
			assert.NoError(t, err)
		}
		
		// 7. Test error handling
		testErr := assert.AnError
		structuredLogger.LogErrorEvent(ctx, "integration_test", "test_operation", testErr, map[string]interface{}{
			"shard_id": shard.Spec.ShardID,
		})
		
		metricsCollector.RecordError("integration_test", "test_error")
	})
}

func TestMonitoringComponentsInitialization(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := logrus.New()
	
	// Test that all monitoring components can be initialized
	t.Run("StructuredLoggerInit", func(t *testing.T) {
		structuredLogger, err := NewStructuredLogger(*cfg)
		require.NoError(t, err)
		assert.NotNil(t, structuredLogger)
		assert.NotNil(t, structuredLogger.GetLogger())
	})
	
	t.Run("MetricsCollectorInit", func(t *testing.T) {
		metricsCollector, err := NewMetricsCollector(cfg.Metrics, logger)
		require.NoError(t, err)
		assert.NotNil(t, metricsCollector)
		assert.NotNil(t, metricsCollector.GetRegistry())
	})
	
	t.Run("AlertManagerInit", func(t *testing.T) {
		alertConfig := AlertingConfig{
			Enabled:    true,
			WebhookURL: "http://example.com/webhook",
			Timeout:    10 * time.Second,
			RetryCount: 3,
			RetryDelay: 1 * time.Second,
		}
		
		alertManager := NewAlertManager(alertConfig, logger)
		assert.NotNil(t, alertManager)
	})
}

func TestMonitoringMetricsExposure(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Metrics.Port = 0 // Use random port for testing
	logger := logrus.New()
	
	metricsCollector, err := NewMetricsCollector(cfg.Metrics, logger)
	require.NoError(t, err)
	
	// Test metrics server startup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start metrics server in background
	go func() {
		err := metricsCollector.StartMetricsServer(ctx)
		assert.NoError(t, err)
	}()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Cancel context to shut down server
	cancel()
	
	// Give server time to shut down
	time.Sleep(100 * time.Millisecond)
}