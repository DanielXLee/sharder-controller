package controllers

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/k8s-shard-controller/pkg/config"
)

// StructuredLogger provides structured logging capabilities
type StructuredLogger struct {
	logger *logrus.Logger
	config config.Config
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(config config.Config) (*StructuredLogger, error) {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(config.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set log format
	switch config.LogFormat {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	default:
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	}

	logger.SetOutput(os.Stdout)

	return &StructuredLogger{
		logger: logger,
		config: config,
	}, nil
}

// GetLogger returns the underlying logrus logger
func (sl *StructuredLogger) GetLogger() *logrus.Logger {
	return sl.logger
}

// LogShardEvent logs shard-related events with structured fields
func (sl *StructuredLogger) LogShardEvent(ctx context.Context, event string, shardID string, fields map[string]interface{}) {
	entry := sl.logger.WithFields(logrus.Fields{
		"component": "shard_manager",
		"event":     event,
		"shard_id":  shardID,
		"timestamp": time.Now().UTC(),
	})

	for k, v := range fields {
		entry = entry.WithField(k, v)
	}

	entry.Info("Shard event occurred")
}

// LogMigrationEvent logs resource migration events
func (sl *StructuredLogger) LogMigrationEvent(ctx context.Context, sourceShard, targetShard string, resourceCount int, status string, duration time.Duration) {
	sl.logger.WithFields(logrus.Fields{
		"component":      "resource_migrator",
		"event":          "migration",
		"source_shard":   sourceShard,
		"target_shard":   targetShard,
		"resource_count": resourceCount,
		"status":         status,
		"duration_ms":    duration.Milliseconds(),
		"timestamp":      time.Now().UTC(),
	}).Info("Resource migration event")
}

// LogScaleEvent logs scaling events
func (sl *StructuredLogger) LogScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string, status string) {
	sl.logger.WithFields(logrus.Fields{
		"component":   "shard_manager",
		"event":       "scale",
		"operation":   operation,
		"from_count":  fromCount,
		"to_count":    toCount,
		"reason":      reason,
		"status":      status,
		"timestamp":   time.Now().UTC(),
	}).Info("Scale operation event")
}

// LogHealthEvent logs health check events
func (sl *StructuredLogger) LogHealthEvent(ctx context.Context, shardID string, healthy bool, errorCount int, message string) {
	level := logrus.InfoLevel
	if !healthy {
		level = logrus.WarnLevel
	}

	sl.logger.WithFields(logrus.Fields{
		"component":   "health_checker",
		"event":       "health_check",
		"shard_id":    shardID,
		"healthy":     healthy,
		"error_count": errorCount,
		"message":     message,
		"timestamp":   time.Now().UTC(),
	}).Log(level, "Health check event")
}

// LogErrorEvent logs error events with context
func (sl *StructuredLogger) LogErrorEvent(ctx context.Context, component, operation string, err error, fields map[string]interface{}) {
	entry := sl.logger.WithFields(logrus.Fields{
		"component": component,
		"event":     "error",
		"operation": operation,
		"error":     err.Error(),
		"timestamp": time.Now().UTC(),
	})

	for k, v := range fields {
		entry = entry.WithField(k, v)
	}

	entry.Error("Error event occurred")
}

// LogPerformanceEvent logs performance-related events
func (sl *StructuredLogger) LogPerformanceEvent(ctx context.Context, operation, component string, duration time.Duration, success bool) {
	sl.logger.WithFields(logrus.Fields{
		"component":   component,
		"event":       "performance",
		"operation":   operation,
		"duration_ms": duration.Milliseconds(),
		"success":     success,
		"timestamp":   time.Now().UTC(),
	}).Info("Performance event")
}

// LogConfigEvent logs configuration change events
func (sl *StructuredLogger) LogConfigEvent(ctx context.Context, configType string, changes map[string]interface{}) {
	sl.logger.WithFields(logrus.Fields{
		"component":   "config_manager",
		"event":       "config_change",
		"config_type": configType,
		"changes":     changes,
		"timestamp":   time.Now().UTC(),
	}).Info("Configuration change event")
}

// LogSystemEvent logs system-wide events
func (sl *StructuredLogger) LogSystemEvent(ctx context.Context, event string, severity string, fields map[string]interface{}) {
	entry := sl.logger.WithFields(logrus.Fields{
		"component": "system",
		"event":     event,
		"severity":  severity,
		"timestamp": time.Now().UTC(),
	})

	for k, v := range fields {
		entry = entry.WithField(k, v)
	}

	switch severity {
	case "critical":
		entry.Error("Critical system event")
	case "warning":
		entry.Warn("System warning event")
	case "info":
		entry.Info("System info event")
	default:
		entry.Info("System event")
	}
}