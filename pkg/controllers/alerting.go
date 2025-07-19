package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/k8s-shard-controller/pkg/interfaces"
)



// AlertingConfig represents alerting configuration
type AlertingConfig struct {
	Enabled     bool          `json:"enabled"`
	WebhookURL  string        `json:"webhookUrl"`
	Timeout     time.Duration `json:"timeout"`
	RetryCount  int           `json:"retryCount"`
	RetryDelay  time.Duration `json:"retryDelay"`
}

// AlertManager handles alert notifications
type AlertManager struct {
	config AlertingConfig
	logger *logrus.Logger
	client *http.Client
}

// NewAlertManager creates a new alert manager
func NewAlertManager(config AlertingConfig, logger *logrus.Logger) *AlertManager {
	return &AlertManager{
		config: config,
		logger: logger,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// SendAlert sends an alert notification
func (am *AlertManager) SendAlert(ctx context.Context, alert interfaces.Alert) error {
	if !am.config.Enabled {
		am.logger.Debug("Alerting is disabled, skipping alert")
		return nil
	}

	alert.Timestamp = time.Now().UTC()

	// Log the alert
	am.logger.WithFields(logrus.Fields{
		"title":     alert.Title,
		"severity":  alert.Severity,
		"component": alert.Component,
	}).Info("Sending alert")

	// Send webhook notification if configured
	if am.config.WebhookURL != "" {
		return am.sendWebhookAlert(ctx, alert)
	}

	return nil
}

// sendWebhookAlert sends alert via webhook
func (am *AlertManager) sendWebhookAlert(ctx context.Context, alert interfaces.Alert) error {
	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	var lastErr error
	for i := 0; i <= am.config.RetryCount; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(am.config.RetryDelay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", am.config.WebhookURL, bytes.NewBuffer(payload))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := am.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send webhook: %w", err)
			continue
		}

		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			am.logger.WithFields(logrus.Fields{
				"status_code": resp.StatusCode,
				"attempt":     i + 1,
			}).Debug("Alert sent successfully")
			return nil
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("failed to send alert after %d attempts: %w", am.config.RetryCount+1, lastErr)
}

// AlertShardFailure sends an alert for shard failure
func (am *AlertManager) AlertShardFailure(ctx context.Context, shardID string, reason string) error {
	alert := interfaces.Alert{
		Title:     fmt.Sprintf("Shard Failure: %s", shardID),
		Message:   fmt.Sprintf("Shard %s has failed: %s", shardID, reason),
		Severity:  interfaces.AlertCritical,
		Component: "shard_manager",
		Labels: map[string]string{
			"shard_id": shardID,
			"type":     "shard_failure",
		},
		Annotations: map[string]interface{}{
			"reason": reason,
		},
	}

	return am.SendAlert(ctx, alert)
}

// AlertHighErrorRate sends an alert for high error rate
func (am *AlertManager) AlertHighErrorRate(ctx context.Context, component string, errorRate float64, threshold float64) error {
	alert := interfaces.Alert{
		Title:     fmt.Sprintf("High Error Rate: %s", component),
		Message:   fmt.Sprintf("Component %s has error rate %.2f%% (threshold: %.2f%%)", component, errorRate*100, threshold*100),
		Severity:  interfaces.AlertWarning,
		Component: component,
		Labels: map[string]string{
			"component": component,
			"type":      "high_error_rate",
		},
		Annotations: map[string]interface{}{
			"error_rate": errorRate,
			"threshold":  threshold,
		},
	}

	return am.SendAlert(ctx, alert)
}

// AlertScaleEvent sends an alert for scaling events
func (am *AlertManager) AlertScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string) error {
	severity := interfaces.AlertInfo
	if operation == "scale_up" && toCount > fromCount*2 {
		severity = interfaces.AlertWarning
	}

	alert := interfaces.Alert{
		Title:     fmt.Sprintf("Scale Operation: %s", operation),
		Message:   fmt.Sprintf("Scaled %s from %d to %d shards. Reason: %s", operation, fromCount, toCount, reason),
		Severity:  severity,
		Component: "shard_manager",
		Labels: map[string]string{
			"operation": operation,
			"type":      "scale_event",
		},
		Annotations: map[string]interface{}{
			"from_count": fromCount,
			"to_count":   toCount,
			"reason":     reason,
		},
	}

	return am.SendAlert(ctx, alert)
}

// AlertMigrationFailure sends an alert for migration failures
func (am *AlertManager) AlertMigrationFailure(ctx context.Context, sourceShard, targetShard string, resourceCount int, reason string) error {
	alert := interfaces.Alert{
		Title:     "Resource Migration Failed",
		Message:   fmt.Sprintf("Failed to migrate %d resources from %s to %s: %s", resourceCount, sourceShard, targetShard, reason),
		Severity:  interfaces.AlertCritical,
		Component: "resource_migrator",
		Labels: map[string]string{
			"source_shard": sourceShard,
			"target_shard": targetShard,
			"type":         "migration_failure",
		},
		Annotations: map[string]interface{}{
			"resource_count": resourceCount,
			"reason":         reason,
		},
	}

	return am.SendAlert(ctx, alert)
}

// AlertSystemOverload sends an alert for system overload
func (am *AlertManager) AlertSystemOverload(ctx context.Context, totalLoad float64, threshold float64, shardCount int) error {
	alert := interfaces.Alert{
		Title:     "System Overload Detected",
		Message:   fmt.Sprintf("System load %.2f exceeds threshold %.2f with %d shards", totalLoad, threshold, shardCount),
		Severity:  interfaces.AlertCritical,
		Component: "load_balancer",
		Labels: map[string]string{
			"type": "system_overload",
		},
		Annotations: map[string]interface{}{
			"total_load":  totalLoad,
			"threshold":   threshold,
			"shard_count": shardCount,
		},
	}

	return am.SendAlert(ctx, alert)
}

// AlertConfigurationChange sends an alert for configuration changes
func (am *AlertManager) AlertConfigurationChange(ctx context.Context, configType string, changes map[string]interface{}) error {
	alert := interfaces.Alert{
		Title:     fmt.Sprintf("Configuration Changed: %s", configType),
		Message:   fmt.Sprintf("Configuration %s has been updated", configType),
		Severity:  interfaces.AlertInfo,
		Component: "config_manager",
		Labels: map[string]string{
			"config_type": configType,
			"type":        "config_change",
		},
		Annotations: map[string]interface{}{
			"changes": changes,
		},
	}

	return am.SendAlert(ctx, alert)
}