package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
)

type MetricsCollector struct {
	config config.MetricsConfig
	logger *logrus.Logger

	// Shard metrics
	shardCountGauge        prometheus.Gauge
	healthyShardCountGauge prometheus.Gauge
	shardLoadVec           *prometheus.GaugeVec
	shardResourceVec       *prometheus.GaugeVec
	shardStatusVec         *prometheus.GaugeVec
	shardHealthVec         *prometheus.GaugeVec

	// System metrics
	totalResourcesGauge     prometheus.Gauge
	migrationCounterVec     *prometheus.CounterVec
	migrationDurationVec    *prometheus.HistogramVec
	scaleOperationCounterVec *prometheus.CounterVec
	errorCounterVec         *prometheus.CounterVec

	// Performance metrics
	operationDurationVec *prometheus.HistogramVec
	queueLengthVec       *prometheus.GaugeVec

	registry *prometheus.Registry
}

// NewMetricsCollector creates a new MetricsCollector
func NewMetricsCollector(config config.MetricsConfig, logger *logrus.Logger) (*MetricsCollector, error) {
	reg := prometheus.NewRegistry()

	mc := &MetricsCollector{
		config:   config,
		logger:   logger,
		registry: reg,

		// Shard metrics
		shardCountGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "shard_controller_total_shards",
				Help: "Total number of shards in the system",
			},
		),
		healthyShardCountGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "shard_controller_healthy_shards",
				Help: "Number of healthy shards in the system",
			},
		),
		shardLoadVec: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "shard_controller_shard_load",
				Help: "Current load of each shard (0.0 to 1.0)",
			},
			[]string{"shard_id", "shard_status"},
		),
		shardResourceVec: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "shard_controller_shard_resources",
				Help: "Number of resources assigned to each shard",
			},
			[]string{"shard_id", "shard_status"},
		),
		shardStatusVec: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "shard_controller_shard_status",
				Help: "Status of each shard (1=Running, 0=Failed, 0.5=Draining)",
			},
			[]string{"shard_id", "status"},
		),
		shardHealthVec: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "shard_controller_shard_health",
				Help: "Health status of each shard (1=Healthy, 0=Unhealthy)",
			},
			[]string{"shard_id"},
		),

		// System metrics
		totalResourcesGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "shard_controller_total_resources",
				Help: "Total number of resources managed by the system",
			},
		),
		migrationCounterVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shard_controller_migrations_total",
				Help: "Total number of resource migrations",
			},
			[]string{"source_shard", "target_shard", "status"},
		),
		migrationDurationVec: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "shard_controller_migration_duration_seconds",
				Help:    "Duration of resource migrations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"source_shard", "target_shard"},
		),
		scaleOperationCounterVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shard_controller_scale_operations_total",
				Help: "Total number of scale operations",
			},
			[]string{"operation", "status"},
		),
		errorCounterVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "shard_controller_errors_total",
				Help: "Total number of errors by component and type",
			},
			[]string{"component", "error_type"},
		),

		// Performance metrics
		operationDurationVec: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "shard_controller_operation_duration_seconds",
				Help:    "Duration of various operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "component"},
		),
		queueLengthVec: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "shard_controller_queue_length",
				Help: "Length of various processing queues",
			},
			[]string{"queue_type", "shard_id"},
		),
	}

	// Register all metrics
	mc.registry.MustRegister(mc.shardCountGauge)
	mc.registry.MustRegister(mc.healthyShardCountGauge)
	mc.registry.MustRegister(mc.shardLoadVec)
	mc.registry.MustRegister(mc.shardResourceVec)
	mc.registry.MustRegister(mc.shardStatusVec)
	mc.registry.MustRegister(mc.shardHealthVec)
	mc.registry.MustRegister(mc.totalResourcesGauge)
	mc.registry.MustRegister(mc.migrationCounterVec)
	mc.registry.MustRegister(mc.migrationDurationVec)
	mc.registry.MustRegister(mc.scaleOperationCounterVec)
	mc.registry.MustRegister(mc.errorCounterVec)
	mc.registry.MustRegister(mc.operationDurationVec)
	mc.registry.MustRegister(mc.queueLengthVec)

	return mc, nil
}

// CollectShardMetrics collects metrics from a shard
func (mc *MetricsCollector) CollectShardMetrics(shard *shardv1.ShardInstance) error {
	if shard == nil {
		return fmt.Errorf("shard cannot be nil")
	}

	shardID := shard.Spec.ShardID
	status := string(shard.Status.Phase)

	// Update shard load and resource metrics
	mc.shardLoadVec.WithLabelValues(shardID, status).Set(shard.Status.Load)
	mc.shardResourceVec.WithLabelValues(shardID, status).Set(float64(len(shard.Status.AssignedResources)))

	// Update shard status metric
	var statusValue float64
	switch shard.Status.Phase {
	case shardv1.ShardPhaseRunning:
		statusValue = 1.0
	case shardv1.ShardPhaseDraining:
		statusValue = 0.5
	case shardv1.ShardPhaseFailed:
		statusValue = 0.0
	default:
		statusValue = 0.0
	}
	mc.shardStatusVec.WithLabelValues(shardID, status).Set(statusValue)

	// Update health metric based on last heartbeat
	healthValue := 0.0
	if time.Since(shard.Status.LastHeartbeat.Time) < 2*time.Minute {
		healthValue = 1.0
	}
	mc.shardHealthVec.WithLabelValues(shardID).Set(healthValue)

	mc.logger.WithFields(logrus.Fields{
		"shard_id": shardID,
		"load":     shard.Status.Load,
		"resources": len(shard.Status.AssignedResources),
		"status":   status,
		"healthy":  healthValue == 1.0,
	}).Debug("Collected shard metrics")

	return nil
}

// CollectSystemMetrics collects system-wide metrics
func (mc *MetricsCollector) CollectSystemMetrics(shards []*shardv1.ShardInstance) error {
	totalShards := len(shards)
	healthyShards := 0
	totalResources := 0

	for _, shard := range shards {
		if shard.Status.Phase == shardv1.ShardPhaseRunning && 
		   time.Since(shard.Status.LastHeartbeat.Time) < 2*time.Minute {
			healthyShards++
		}
		totalResources += len(shard.Status.AssignedResources)
	}

	mc.shardCountGauge.Set(float64(totalShards))
	mc.healthyShardCountGauge.Set(float64(healthyShards))
	mc.totalResourcesGauge.Set(float64(totalResources))

	mc.logger.WithFields(logrus.Fields{
		"total_shards":   totalShards,
		"healthy_shards": healthyShards,
		"total_resources": totalResources,
	}).Info("Collected system metrics")

	return nil
}

// RecordMigration records a resource migration event
func (mc *MetricsCollector) RecordMigration(sourceShard, targetShard, status string, duration time.Duration) {
	mc.migrationCounterVec.WithLabelValues(sourceShard, targetShard, status).Inc()
	if status == "success" {
		mc.migrationDurationVec.WithLabelValues(sourceShard, targetShard).Observe(duration.Seconds())
	}

	mc.logger.WithFields(logrus.Fields{
		"source_shard": sourceShard,
		"target_shard": targetShard,
		"status":       status,
		"duration":     duration.Seconds(),
	}).Info("Recorded migration event")
}

// RecordScaleOperation records a scale operation (up/down)
func (mc *MetricsCollector) RecordScaleOperation(operation, status string) {
	mc.scaleOperationCounterVec.WithLabelValues(operation, status).Inc()

	mc.logger.WithFields(logrus.Fields{
		"operation": operation,
		"status":    status,
	}).Info("Recorded scale operation")
}

// RecordError records an error event
func (mc *MetricsCollector) RecordError(component, errorType string) {
	mc.errorCounterVec.WithLabelValues(component, errorType).Inc()

	mc.logger.WithFields(logrus.Fields{
		"component":  component,
		"error_type": errorType,
	}).Error("Recorded error event")
}

// RecordOperationDuration records the duration of an operation
func (mc *MetricsCollector) RecordOperationDuration(operation, component string, duration time.Duration) {
	mc.operationDurationVec.WithLabelValues(operation, component).Observe(duration.Seconds())

	mc.logger.WithFields(logrus.Fields{
		"operation": operation,
		"component": component,
		"duration":  duration.Seconds(),
	}).Debug("Recorded operation duration")
}

// UpdateQueueLength updates the length of a processing queue
func (mc *MetricsCollector) UpdateQueueLength(queueType, shardID string, length int) {
	mc.queueLengthVec.WithLabelValues(queueType, shardID).Set(float64(length))

	mc.logger.WithFields(logrus.Fields{
		"queue_type": queueType,
		"shard_id":   shardID,
		"length":     length,
	}).Debug("Updated queue length")
}

// ExposeMetrics exposes metrics for scraping
func (mc *MetricsCollector) ExposeMetrics() error {
	if !mc.config.Enabled {
		mc.logger.Info("Metrics collection is disabled")
		return nil
	}

	handler := promhttp.HandlerFor(mc.registry, promhttp.HandlerOpts{})
	http.Handle(mc.config.Path, handler)
	
	mc.logger.WithFields(logrus.Fields{
		"port": mc.config.Port,
		"path": mc.config.Path,
	}).Info("Starting metrics server")

	return http.ListenAndServe(fmt.Sprintf(":%d", mc.config.Port), nil)
}

// StartMetricsServer starts the metrics server in a goroutine
func (mc *MetricsCollector) StartMetricsServer(ctx context.Context) error {
	if !mc.config.Enabled {
		mc.logger.Info("Metrics collection is disabled")
		return nil
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", mc.config.Port),
	}

	handler := promhttp.HandlerFor(mc.registry, promhttp.HandlerOpts{})
	http.Handle(mc.config.Path, handler)

	go func() {
		<-ctx.Done()
		mc.logger.Info("Shutting down metrics server")
		server.Shutdown(context.Background())
	}()

	mc.logger.WithFields(logrus.Fields{
		"port": mc.config.Port,
		"path": mc.config.Path,
	}).Info("Starting metrics server")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("metrics server failed: %w", err)
	}

	return nil
}

// RecordCustomMetric records a custom metric
func (mc *MetricsCollector) RecordCustomMetric(name string, value float64, labels map[string]string) error {
	// Create a gauge for custom metrics if it doesn't exist
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: fmt.Sprintf("shard_controller_%s", name),
			Help: fmt.Sprintf("Custom metric: %s", name),
		},
		getLabelKeys(labels),
	)

	// Try to register the gauge (ignore error if already registered)
	mc.registry.Register(gauge)

	// Set the value
	gauge.WithLabelValues(getLabelValues(labels)...).Set(value)

	mc.logger.WithFields(logrus.Fields{
		"metric_name": name,
		"value":       value,
		"labels":      labels,
	}).Debug("Recorded custom metric")

	return nil
}

// getLabelKeys extracts label keys from a map
func getLabelKeys(labels map[string]string) []string {
	if labels == nil {
		return []string{}
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	return keys
}

// getLabelValues extracts label values from a map in the same order as keys
func getLabelValues(labels map[string]string) []string {
	if labels == nil {
		return []string{}
	}
	keys := getLabelKeys(labels)
	values := make([]string, len(keys))
	for i, k := range keys {
		values[i] = labels[k]
	}
	return values
}

// GetRegistry returns the prometheus registry for testing
func (mc *MetricsCollector) GetRegistry() *prometheus.Registry {
	return mc.registry
}