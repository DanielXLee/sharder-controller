package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
)

func TestValidateShardConfig(t *testing.T) {
	tests := []struct {
		name    string
		spec    *shardv1.ShardConfigSpec
		wantErr bool
	}{
		{
			name: "valid config",
			spec: &shardv1.ShardConfigSpec{
				MinShards:               1,
				MaxShards:               10,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
			},
			wantErr: false,
		},
		{
			name: "invalid minShards",
			spec: &shardv1.ShardConfigSpec{
				MinShards:               0,
				MaxShards:               10,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
			},
			wantErr: true,
		},
		{
			name: "maxShards less than minShards",
			spec: &shardv1.ShardConfigSpec{
				MinShards:               5,
				MaxShards:               3,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
			},
			wantErr: true,
		},
		{
			name: "invalid scaleUpThreshold",
			spec: &shardv1.ShardConfigSpec{
				MinShards:               1,
				MaxShards:               10,
				ScaleUpThreshold:        1.5,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
			},
			wantErr: true,
		},
		{
			name: "scaleDownThreshold >= scaleUpThreshold",
			spec: &shardv1.ShardConfigSpec{
				MinShards:               1,
				MaxShards:               10,
				ScaleUpThreshold:        0.5,
				ScaleDownThreshold:      0.8,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
			},
			wantErr: true,
		},
		{
			name: "invalid load balance strategy",
			spec: &shardv1.ShardConfigSpec{
				MinShards:               1,
				MaxShards:               10,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     "invalid-strategy",
				GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateShardConfig(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateShardInstance(t *testing.T) {
	tests := []struct {
		name    string
		spec    *shardv1.ShardInstanceSpec
		wantErr bool
	}{
		{
			name: "valid instance",
			spec: &shardv1.ShardInstanceSpec{
				ShardID: "shard-1",
				HashRange: &shardv1.HashRange{
					Start: 0,
					End:   1000,
				},
			},
			wantErr: false,
		},
		{
			name: "empty shardId",
			spec: &shardv1.ShardInstanceSpec{
				ShardID: "",
			},
			wantErr: true,
		},
		{
			name: "invalid hash range",
			spec: &shardv1.ShardInstanceSpec{
				ShardID: "shard-1",
				HashRange: &shardv1.HashRange{
					Start: 1000,
					End:   500,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateShardInstance(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	validConfig := config.DefaultConfig()

	t.Run("valid config", func(t *testing.T) {
		err := ValidateConfig(validConfig)
		assert.NoError(t, err)
	})

	t.Run("empty namespace", func(t *testing.T) {
		cfg := *validConfig
		cfg.Namespace = ""
		err := ValidateConfig(&cfg)
		assert.Error(t, err)
	})

	t.Run("invalid log level", func(t *testing.T) {
		cfg := *validConfig
		cfg.LogLevel = "invalid"
		err := ValidateConfig(&cfg)
		assert.Error(t, err)
	})
}

func TestValidateHashRanges(t *testing.T) {
	tests := []struct {
		name    string
		ranges  []*shardv1.HashRange
		wantErr bool
	}{
		{
			name:    "empty ranges",
			ranges:  []*shardv1.HashRange{},
			wantErr: false,
		},
		{
			name: "single range",
			ranges: []*shardv1.HashRange{
				{Start: 0, End: 1000},
			},
			wantErr: false,
		},
		{
			name: "non-overlapping ranges",
			ranges: []*shardv1.HashRange{
				{Start: 0, End: 500},
				{Start: 501, End: 1000},
			},
			wantErr: false,
		},
		{
			name: "overlapping ranges",
			ranges: []*shardv1.HashRange{
				{Start: 0, End: 600},
				{Start: 500, End: 1000},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHashRanges(tt.ranges)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalculateLoadScore(t *testing.T) {
	metrics := &shardv1.LoadMetrics{
		ResourceCount:  100,
		CPUUsage:       0.5,
		MemoryUsage:    0.6,
		ProcessingRate: 10.0,
		QueueLength:    5,
	}

	score := CalculateLoadScore(metrics)
	assert.True(t, score >= 0 && score <= 1, "Load score should be between 0 and 1")
}

func TestIsShardHealthy(t *testing.T) {
	now := metav1.Now()

	tests := []struct {
		name   string
		status *shardv1.ShardInstanceStatus
		want   bool
	}{
		{
			name:   "nil status",
			status: nil,
			want:   false,
		},
		{
			name: "healthy shard",
			status: &shardv1.ShardInstanceStatus{
				Phase:         shardv1.ShardPhaseRunning,
				LastHeartbeat: now,
				HealthStatus: &shardv1.HealthStatus{
					Healthy: true,
				},
			},
			want: true,
		},
		{
			name: "failed shard",
			status: &shardv1.ShardInstanceStatus{
				Phase:         shardv1.ShardPhaseFailed,
				LastHeartbeat: now,
				HealthStatus: &shardv1.HealthStatus{
					Healthy: false,
				},
			},
			want: false,
		},
		{
			name: "stale heartbeat",
			status: &shardv1.ShardInstanceStatus{
				Phase: shardv1.ShardPhaseRunning,
				LastHeartbeat: metav1.Time{
					Time: time.Now().Add(-5 * time.Minute),
				},
				HealthStatus: &shardv1.HealthStatus{
					Healthy: true,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsShardHealthy(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}
