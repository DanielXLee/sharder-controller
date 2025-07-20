package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k8s-shard-controller/pkg/interfaces"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAlertManager(t *testing.T) {
	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: "http://example.com/webhook",
		Timeout:    10 * time.Second,
		RetryCount: 3,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()

	am := NewAlertManager(config, logger)
	assert.NotNil(t, am)
	assert.Equal(t, config, am.config)
	assert.Equal(t, logger, am.logger)
	assert.NotNil(t, am.client)
}

func TestAlertManager_SendAlert_Disabled(t *testing.T) {
	config := AlertingConfig{
		Enabled: false,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	alert := interfaces.Alert{
		Title:     "Test Alert",
		Message:   "This is a test",
		Severity:  interfaces.AlertInfo,
		Component: "test",
	}

	ctx := context.Background()
	err := am.SendAlert(ctx, alert)
	assert.NoError(t, err)
}

func TestAlertManager_SendWebhookAlert_Success(t *testing.T) {
	// Create test server
	var receivedAlert interfaces.Alert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		err := json.NewDecoder(r.Body).Decode(&receivedAlert)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	alert := interfaces.Alert{
		Title:     "Test Alert",
		Message:   "This is a test alert",
		Severity:  interfaces.AlertWarning,
		Component: "test_component",
		Labels: map[string]string{
			"type": "test",
		},
		Annotations: map[string]interface{}{
			"value": 42,
		},
	}

	ctx := context.Background()
	err := am.SendAlert(ctx, alert)
	assert.NoError(t, err)

	// Verify received alert
	assert.Equal(t, "Test Alert", receivedAlert.Title)
	assert.Equal(t, "This is a test alert", receivedAlert.Message)
	assert.Equal(t, interfaces.AlertWarning, receivedAlert.Severity)
	assert.Equal(t, "test_component", receivedAlert.Component)
	assert.Equal(t, "test", receivedAlert.Labels["type"])
	assert.Equal(t, float64(42), receivedAlert.Annotations["value"])
	assert.False(t, receivedAlert.Timestamp.IsZero())
}

func TestAlertManager_SendWebhookAlert_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 3,
		RetryDelay: 100 * time.Millisecond,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	alert := interfaces.Alert{
		Title:     "Test Alert",
		Message:   "This is a test alert",
		Severity:  interfaces.AlertCritical,
		Component: "test_component",
	}

	ctx := context.Background()
	err := am.SendAlert(ctx, alert)
	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestAlertManager_SendWebhookAlert_MaxRetriesExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 2,
		RetryDelay: 100 * time.Millisecond,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	alert := interfaces.Alert{
		Title:     "Test Alert",
		Message:   "This is a test alert",
		Severity:  interfaces.AlertCritical,
		Component: "test_component",
	}

	ctx := context.Background()
	err := am.SendAlert(ctx, alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send alert after 3 attempts")
}

func TestAlertManager_AlertShardFailure(t *testing.T) {
	var receivedAlert interfaces.Alert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedAlert)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	ctx := context.Background()
	err := am.AlertShardFailure(ctx, "shard-1", "connection timeout")
	assert.NoError(t, err)

	assert.Equal(t, "Shard Failure: shard-1", receivedAlert.Title)
	assert.Contains(t, receivedAlert.Message, "shard-1")
	assert.Contains(t, receivedAlert.Message, "connection timeout")
	assert.Equal(t, interfaces.AlertCritical, receivedAlert.Severity)
	assert.Equal(t, "shard_manager", receivedAlert.Component)
	assert.Equal(t, "shard-1", receivedAlert.Labels["shard_id"])
	assert.Equal(t, "shard_failure", receivedAlert.Labels["type"])
	assert.Equal(t, "connection timeout", receivedAlert.Annotations["reason"])
}

func TestAlertManager_AlertHighErrorRate(t *testing.T) {
	var receivedAlert interfaces.Alert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedAlert)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	ctx := context.Background()
	err := am.AlertHighErrorRate(ctx, "load_balancer", 0.15, 0.10)
	assert.NoError(t, err)

	assert.Equal(t, "High Error Rate: load_balancer", receivedAlert.Title)
	assert.Contains(t, receivedAlert.Message, "15.00%")
	assert.Contains(t, receivedAlert.Message, "10.00%")
	assert.Equal(t, interfaces.AlertWarning, receivedAlert.Severity)
	assert.Equal(t, "load_balancer", receivedAlert.Component)
	assert.Equal(t, "load_balancer", receivedAlert.Labels["component"])
	assert.Equal(t, "high_error_rate", receivedAlert.Labels["type"])
	assert.Equal(t, 0.15, receivedAlert.Annotations["error_rate"])
	assert.Equal(t, 0.10, receivedAlert.Annotations["threshold"])
}

func TestAlertManager_AlertScaleEvent(t *testing.T) {
	var receivedAlert interfaces.Alert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedAlert)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	ctx := context.Background()

	// Test normal scale up (should be info)
	err := am.AlertScaleEvent(ctx, "scale_up", 2, 3, "high_load")
	assert.NoError(t, err)
	assert.Equal(t, interfaces.AlertInfo, receivedAlert.Severity)

	// Test aggressive scale up (should be warning)
	err = am.AlertScaleEvent(ctx, "scale_up", 2, 5, "very_high_load")
	assert.NoError(t, err)

	assert.Equal(t, "Scale Operation: scale_up", receivedAlert.Title)
	assert.Contains(t, receivedAlert.Message, "scale_up from 2 to 5")
	assert.Contains(t, receivedAlert.Message, "very_high_load")
	assert.Equal(t, interfaces.AlertWarning, receivedAlert.Severity)
	assert.Equal(t, "shard_manager", receivedAlert.Component)
	assert.Equal(t, "scale_up", receivedAlert.Labels["operation"])
	assert.Equal(t, "scale_event", receivedAlert.Labels["type"])
	assert.Equal(t, float64(2), receivedAlert.Annotations["from_count"])
	assert.Equal(t, float64(5), receivedAlert.Annotations["to_count"])
	assert.Equal(t, "very_high_load", receivedAlert.Annotations["reason"])
}

func TestAlertManager_AlertMigrationFailure(t *testing.T) {
	var receivedAlert interfaces.Alert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedAlert)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	ctx := context.Background()
	err := am.AlertMigrationFailure(ctx, "shard-1", "shard-2", 10, "target shard unavailable")
	assert.NoError(t, err)

	assert.Equal(t, "Resource Migration Failed", receivedAlert.Title)
	assert.Contains(t, receivedAlert.Message, "10 resources")
	assert.Contains(t, receivedAlert.Message, "shard-1")
	assert.Contains(t, receivedAlert.Message, "shard-2")
	assert.Contains(t, receivedAlert.Message, "target shard unavailable")
	assert.Equal(t, interfaces.AlertCritical, receivedAlert.Severity)
	assert.Equal(t, "resource_migrator", receivedAlert.Component)
	assert.Equal(t, "shard-1", receivedAlert.Labels["source_shard"])
	assert.Equal(t, "shard-2", receivedAlert.Labels["target_shard"])
	assert.Equal(t, "migration_failure", receivedAlert.Labels["type"])
	assert.Equal(t, float64(10), receivedAlert.Annotations["resource_count"])
	assert.Equal(t, "target shard unavailable", receivedAlert.Annotations["reason"])
}

func TestAlertManager_AlertSystemOverload(t *testing.T) {
	var receivedAlert interfaces.Alert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedAlert)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	ctx := context.Background()
	err := am.AlertSystemOverload(ctx, 0.95, 0.80, 5)
	assert.NoError(t, err)

	assert.Equal(t, "System Overload Detected", receivedAlert.Title)
	assert.Contains(t, receivedAlert.Message, "0.95")
	assert.Contains(t, receivedAlert.Message, "0.80")
	assert.Contains(t, receivedAlert.Message, "5 shards")
	assert.Equal(t, interfaces.AlertCritical, receivedAlert.Severity)
	assert.Equal(t, "load_balancer", receivedAlert.Component)
	assert.Equal(t, "system_overload", receivedAlert.Labels["type"])
	assert.Equal(t, 0.95, receivedAlert.Annotations["total_load"])
	assert.Equal(t, 0.80, receivedAlert.Annotations["threshold"])
	assert.Equal(t, float64(5), receivedAlert.Annotations["shard_count"])
}

func TestAlertManager_AlertConfigurationChange(t *testing.T) {
	var receivedAlert interfaces.Alert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedAlert)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	ctx := context.Background()
	changes := map[string]interface{}{
		"scale_up_threshold":   0.8,
		"scale_down_threshold": 0.3,
	}
	err := am.AlertConfigurationChange(ctx, "shard_config", changes)
	assert.NoError(t, err)

	assert.Equal(t, "Configuration Changed: shard_config", receivedAlert.Title)
	assert.Contains(t, receivedAlert.Message, "shard_config")
	assert.Equal(t, interfaces.AlertInfo, receivedAlert.Severity)
	assert.Equal(t, "config_manager", receivedAlert.Component)
	assert.Equal(t, "shard_config", receivedAlert.Labels["config_type"])
	assert.Equal(t, "config_change", receivedAlert.Labels["type"])

	// Check changes in annotations
	alertChanges := receivedAlert.Annotations["changes"].(map[string]interface{})
	assert.Equal(t, 0.8, alertChanges["scale_up_threshold"])
	assert.Equal(t, 0.3, alertChanges["scale_down_threshold"])
}

func TestAlertManager_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := AlertingConfig{
		Enabled:    true,
		WebhookURL: server.URL,
		Timeout:    5 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}
	logger := logrus.New()
	am := NewAlertManager(config, logger)

	alert := interfaces.Alert{
		Title:     "Test Alert",
		Message:   "This is a test alert",
		Severity:  interfaces.AlertInfo,
		Component: "test_component",
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := am.SendAlert(ctx, alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
