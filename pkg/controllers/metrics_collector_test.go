package controllers

import (
	"context"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
)

func TestNewMetricsCollector(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	logger := logrus.New()

	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, mc)
	assert.NotNil(t, mc.GetRegistry())
}

func TestMetricsCollector_CollectShardMetrics(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	logger := logrus.New()
	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)

	now := metav1.Now()
	shard := &shardv1.ShardInstance{
		Spec: shardv1.ShardInstanceSpec{
			ShardID: "test-shard",
		},
		Status: shardv1.ShardInstanceStatus{
			Phase:             shardv1.ShardPhaseRunning,
			Load:              0.5,
			AssignedResources: []string{"res1", "res2"},
			LastHeartbeat:     now,
		},
	}

	err = mc.CollectShardMetrics(shard)
	assert.NoError(t, err)

	// Verify metrics were recorded
	metricFamilies, err := mc.GetRegistry().Gather()
	require.NoError(t, err)

	// Check that we have the expected metrics
	metricNames := make(map[string]bool)
	for _, mf := range metricFamilies {
		metricNames[*mf.Name] = true
	}

	assert.True(t, metricNames["shard_controller_shard_load"])
	assert.True(t, metricNames["shard_controller_shard_resources"])
	assert.True(t, metricNames["shard_controller_shard_status"])
	assert.True(t, metricNames["shard_controller_shard_health"])
}

func TestMetricsCollector_CollectSystemMetrics(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	logger := logrus.New()
	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)

	now := metav1.Now()
	shards := []*shardv1.ShardInstance{
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard-1"},
			Status: shardv1.ShardInstanceStatus{
				Phase:             shardv1.ShardPhaseRunning,
				Load:              0.3,
				AssignedResources: []string{"res1", "res2"},
				LastHeartbeat:     now,
			},
		},
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard-2"},
			Status: shardv1.ShardInstanceStatus{
				Phase:             shardv1.ShardPhaseRunning,
				Load:              0.7,
				AssignedResources: []string{"res3"},
				LastHeartbeat:     now,
			},
		},
	}

	err = mc.CollectSystemMetrics(shards)
	assert.NoError(t, err)

	// Verify system metrics were recorded
	metricFamilies, err := mc.GetRegistry().Gather()
	require.NoError(t, err)

	// Find the total shards metric
	var totalShardsMetric *dto.MetricFamily
	for _, mf := range metricFamilies {
		if *mf.Name == "shard_controller_total_shards" {
			totalShardsMetric = mf
			break
		}
	}

	require.NotNil(t, totalShardsMetric)
	assert.Equal(t, float64(2), *totalShardsMetric.Metric[0].Gauge.Value)
}

func TestMetricsCollector_RecordMigration(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	logger := logrus.New()
	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)

	mc.RecordMigration("shard-1", "shard-2", "success", 5*time.Second)

	// Verify migration metrics were recorded
	metricFamilies, err := mc.GetRegistry().Gather()
	require.NoError(t, err)

	// Check migration counter
	var migrationCounter *dto.MetricFamily
	for _, mf := range metricFamilies {
		if *mf.Name == "shard_controller_migrations_total" {
			migrationCounter = mf
			break
		}
	}

	require.NotNil(t, migrationCounter)
	assert.Equal(t, float64(1), *migrationCounter.Metric[0].Counter.Value)
}

func TestMetricsCollector_RecordScaleOperation(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	logger := logrus.New()
	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)

	mc.RecordScaleOperation("scale_up", "success")

	// Verify scale operation metrics were recorded
	metricFamilies, err := mc.GetRegistry().Gather()
	require.NoError(t, err)

	// Check scale operation counter
	var scaleCounter *dto.MetricFamily
	for _, mf := range metricFamilies {
		if *mf.Name == "shard_controller_scale_operations_total" {
			scaleCounter = mf
			break
		}
	}

	require.NotNil(t, scaleCounter)
	assert.Equal(t, float64(1), *scaleCounter.Metric[0].Counter.Value)
}

func TestMetricsCollector_RecordError(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	logger := logrus.New()
	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)

	mc.RecordError("shard_manager", "startup_failure")

	// Verify error metrics were recorded
	metricFamilies, err := mc.GetRegistry().Gather()
	require.NoError(t, err)

	// Check error counter
	var errorCounter *dto.MetricFamily
	for _, mf := range metricFamilies {
		if *mf.Name == "shard_controller_errors_total" {
			errorCounter = mf
			break
		}
	}

	require.NotNil(t, errorCounter)
	assert.Equal(t, float64(1), *errorCounter.Metric[0].Counter.Value)
}

func TestMetricsCollector_RecordOperationDuration(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	logger := logrus.New()
	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)

	mc.RecordOperationDuration("create_shard", "shard_manager", 2*time.Second)

	// Verify operation duration metrics were recorded
	metricFamilies, err := mc.GetRegistry().Gather()
	require.NoError(t, err)

	// Check operation duration histogram
	var durationHistogram *dto.MetricFamily
	for _, mf := range metricFamilies {
		if *mf.Name == "shard_controller_operation_duration_seconds" {
			durationHistogram = mf
			break
		}
	}

	require.NotNil(t, durationHistogram)
	assert.Equal(t, uint64(1), *durationHistogram.Metric[0].Histogram.SampleCount)
}

func TestMetricsCollector_UpdateQueueLength(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	logger := logrus.New()
	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)

	mc.UpdateQueueLength("migration", "shard-1", 5)

	// Verify queue length metrics were recorded
	metricFamilies, err := mc.GetRegistry().Gather()
	require.NoError(t, err)

	// Check queue length gauge
	var queueGauge *dto.MetricFamily
	for _, mf := range metricFamilies {
		if *mf.Name == "shard_controller_queue_length" {
			queueGauge = mf
			break
		}
	}

	require.NotNil(t, queueGauge)
	assert.Equal(t, float64(5), *queueGauge.Metric[0].Gauge.Value)
}

func TestMetricsCollector_StartMetricsServer(t *testing.T) {
	cfg := config.DefaultConfig().Metrics
	cfg.Port = 0 // Use random port for testing
	logger := logrus.New()
	mc, err := NewMetricsCollector(cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in goroutine
	go func() {
		err := mc.StartMetricsServer(ctx)
		// Server should shut down gracefully when context is cancelled
		assert.NoError(t, err)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to shut down server
	cancel()

	// Give server time to shut down
	time.Sleep(100 * time.Millisecond)
}
