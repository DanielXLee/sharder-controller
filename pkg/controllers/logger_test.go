package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k8s-shard-controller/pkg/config"
)

func TestNewStructuredLogger(t *testing.T) {
	cfg := config.DefaultConfig()
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	assert.NotNil(t, logger)
	assert.NotNil(t, logger.GetLogger())
}

func TestStructuredLogger_JSONFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "json"
	cfg.LogLevel = "info"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	// Capture log output
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	logger.LogShardEvent(ctx, "shard_created", "test-shard", map[string]interface{}{
		"load": 0.5,
		"resources": 10,
	})
	
	// Parse JSON log output
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "shard_manager", logEntry["component"])
	assert.Equal(t, "shard_created", logEntry["event"])
	assert.Equal(t, "test-shard", logEntry["shard_id"])
	assert.Equal(t, "Shard event occurred", logEntry["message"])
	assert.Equal(t, "info", logEntry["level"])
}

func TestStructuredLogger_TextFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "text"
	cfg.LogLevel = "debug"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	// Capture log output
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	logger.LogShardEvent(ctx, "shard_updated", "test-shard-2", map[string]interface{}{
		"status": "running",
	})
	
	output := buf.String()
	assert.Contains(t, output, "shard_updated")
	assert.Contains(t, output, "test-shard-2")
	assert.Contains(t, output, "shard_manager")
}

func TestStructuredLogger_LogMigrationEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "json"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	logger.LogMigrationEvent(ctx, "shard-1", "shard-2", 5, "success", 2*time.Second)
	
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "resource_migrator", logEntry["component"])
	assert.Equal(t, "migration", logEntry["event"])
	assert.Equal(t, "shard-1", logEntry["source_shard"])
	assert.Equal(t, "shard-2", logEntry["target_shard"])
	assert.Equal(t, float64(5), logEntry["resource_count"])
	assert.Equal(t, "success", logEntry["status"])
	assert.Equal(t, float64(2000), logEntry["duration_ms"])
}

func TestStructuredLogger_LogScaleEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "json"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	logger.LogScaleEvent(ctx, "scale_up", 2, 4, "high_load", "success")
	
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "shard_manager", logEntry["component"])
	assert.Equal(t, "scale", logEntry["event"])
	assert.Equal(t, "scale_up", logEntry["operation"])
	assert.Equal(t, float64(2), logEntry["from_count"])
	assert.Equal(t, float64(4), logEntry["to_count"])
	assert.Equal(t, "high_load", logEntry["reason"])
	assert.Equal(t, "success", logEntry["status"])
}

func TestStructuredLogger_LogHealthEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "json"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	
	// Test healthy shard
	logger.LogHealthEvent(ctx, "shard-1", true, 0, "healthy")
	
	var logEntry map[string]interface{}
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	err = json.Unmarshal(lines[0], &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "health_checker", logEntry["component"])
	assert.Equal(t, "health_check", logEntry["event"])
	assert.Equal(t, "shard-1", logEntry["shard_id"])
	assert.Equal(t, true, logEntry["healthy"])
	assert.Equal(t, float64(0), logEntry["error_count"])
	assert.Equal(t, "info", logEntry["level"])
	
	// Test unhealthy shard
	buf.Reset()
	logger.LogHealthEvent(ctx, "shard-2", false, 3, "connection timeout")
	
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "shard-2", logEntry["shard_id"])
	assert.Equal(t, false, logEntry["healthy"])
	assert.Equal(t, float64(3), logEntry["error_count"])
	assert.Equal(t, "warning", logEntry["level"])
}

func TestStructuredLogger_LogErrorEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "json"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	testErr := assert.AnError
	fields := map[string]interface{}{
		"shard_id": "shard-1",
		"attempt": 3,
	}
	
	logger.LogErrorEvent(ctx, "shard_manager", "create_shard", testErr, fields)
	
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "shard_manager", logEntry["component"])
	assert.Equal(t, "error", logEntry["event"])
	assert.Equal(t, "create_shard", logEntry["operation"])
	assert.Equal(t, testErr.Error(), logEntry["error"])
	assert.Equal(t, "shard-1", logEntry["shard_id"])
	assert.Equal(t, float64(3), logEntry["attempt"])
	assert.Equal(t, "error", logEntry["level"])
}

func TestStructuredLogger_LogPerformanceEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "json"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	logger.LogPerformanceEvent(ctx, "migrate_resources", "resource_migrator", 1500*time.Millisecond, true)
	
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "resource_migrator", logEntry["component"])
	assert.Equal(t, "performance", logEntry["event"])
	assert.Equal(t, "migrate_resources", logEntry["operation"])
	assert.Equal(t, float64(1500), logEntry["duration_ms"])
	assert.Equal(t, true, logEntry["success"])
}

func TestStructuredLogger_LogConfigEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "json"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	changes := map[string]interface{}{
		"scale_up_threshold": 0.8,
		"scale_down_threshold": 0.3,
	}
	
	logger.LogConfigEvent(ctx, "shard_config", changes)
	
	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "config_manager", logEntry["component"])
	assert.Equal(t, "config_change", logEntry["event"])
	assert.Equal(t, "shard_config", logEntry["config_type"])
	
	// Check changes field
	changesField := logEntry["changes"].(map[string]interface{})
	assert.Equal(t, 0.8, changesField["scale_up_threshold"])
	assert.Equal(t, 0.3, changesField["scale_down_threshold"])
}

func TestStructuredLogger_LogSystemEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogFormat = "json"
	
	logger, err := NewStructuredLogger(*cfg)
	require.NoError(t, err)
	
	var buf bytes.Buffer
	logger.GetLogger().SetOutput(&buf)
	
	ctx := context.Background()
	
	// Test critical event
	fields := map[string]interface{}{
		"total_shards": 5,
		"failed_shards": 3,
	}
	logger.LogSystemEvent(ctx, "high_failure_rate", "critical", fields)
	
	var logEntry map[string]interface{}
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	err = json.Unmarshal(lines[0], &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "system", logEntry["component"])
	assert.Equal(t, "high_failure_rate", logEntry["event"])
	assert.Equal(t, "critical", logEntry["severity"])
	assert.Equal(t, "error", logEntry["level"])
	assert.Equal(t, float64(5), logEntry["total_shards"])
	assert.Equal(t, float64(3), logEntry["failed_shards"])
	
	// Test warning event
	buf.Reset()
	logger.LogSystemEvent(ctx, "resource_usage_high", "warning", map[string]interface{}{
		"cpu_usage": 85.5,
	})
	
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	
	assert.Equal(t, "resource_usage_high", logEntry["event"])
	assert.Equal(t, "warning", logEntry["severity"])
	assert.Equal(t, "warning", logEntry["level"])
	assert.Equal(t, 85.5, logEntry["cpu_usage"])
}

func TestStructuredLogger_LogLevels(t *testing.T) {
	testCases := []struct {
		name     string
		logLevel string
		expected logrus.Level
	}{
		{"debug", "debug", logrus.DebugLevel},
		{"info", "info", logrus.InfoLevel},
		{"warn", "warn", logrus.WarnLevel},
		{"error", "error", logrus.ErrorLevel},
		{"invalid", "invalid", logrus.InfoLevel}, // Should default to info
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.LogLevel = tc.logLevel
			
			logger, err := NewStructuredLogger(*cfg)
			require.NoError(t, err)
			
			assert.Equal(t, tc.expected, logger.GetLogger().GetLevel())
		})
	}
}