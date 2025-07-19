package controllers

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
)

func TestConfigManager_NewConfigManager(t *testing.T) {
	cfg := config.DefaultConfig()
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	tests := []struct {
		name        string
		client      client.Client
		config      *config.Config
		expectError bool
	}{
		{
			name:        "valid parameters",
			client:      cl,
			config:      cfg,
			expectError: false,
		},
		{
			name:        "nil client",
			client:      nil,
			config:      cfg,
			expectError: true,
		},
		{
			name:        "nil config",
			client:      cl,
			config:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := NewConfigManager(tt.client, tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cm)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cm)
				assert.Equal(t, "shard-controller-config", cm.configMapName)
				assert.Equal(t, "default-shard-config", cm.shardConfigName)
				assert.True(t, cm.enableConfigMap)
				assert.True(t, cm.enableHotReload)
			}
		})
	}
}

func TestConfigManager_NewConfigManagerWithOptions(t *testing.T) {
	cfg := config.DefaultConfig()
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	opts := &ConfigManagerOptions{
		ConfigMapName:   "custom-config",
		ShardConfigName: "custom-shard-config",
		EnableConfigMap: false,
		EnableHotReload: false,
	}

	cm, err := NewConfigManagerWithOptions(cl, cfg, opts)
	require.NoError(t, err)
	assert.Equal(t, "custom-config", cm.configMapName)
	assert.Equal(t, "custom-shard-config", cm.shardConfigName)
	assert.False(t, cm.enableConfigMap)
	assert.False(t, cm.enableHotReload)
}

func TestConfigManager_LoadConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	cfg := config.DefaultConfig()

	t.Run("load from existing CRD", func(t *testing.T) {
		shardConfig := &shardv1.ShardConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "default-shard-config", Namespace: cfg.Namespace},
			Spec: shardv1.ShardConfigSpec{
				MinShards:               2,
				MaxShards:               8,
				ScaleUpThreshold:        0.9,
				ScaleDownThreshold:      0.2,
				HealthCheckInterval:     metav1.Duration{Duration: 45 * time.Second},
				LoadBalanceStrategy:     shardv1.RoundRobinStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 45 * time.Second},
			},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(shardConfig).Build()
		cm, err := NewConfigManager(cl, cfg)
		require.NoError(t, err)

		loadedCfg, err := cm.LoadConfig(context.Background())
		require.NoError(t, err)
		assert.Equal(t, shardConfig.Spec, loadedCfg.Spec)
	})

	t.Run("create default CRD when not exists", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).Build()
		cm, err := NewConfigManager(cl, cfg)
		require.NoError(t, err)

		loadedCfg, err := cm.LoadConfig(context.Background())
		require.NoError(t, err)
		
		// Should match default config
		assert.Equal(t, cfg.DefaultShardConfig.MinShards, loadedCfg.Spec.MinShards)
		assert.Equal(t, cfg.DefaultShardConfig.MaxShards, loadedCfg.Spec.MaxShards)
		assert.Equal(t, cfg.DefaultShardConfig.ScaleUpThreshold, loadedCfg.Spec.ScaleUpThreshold)
	})

	t.Run("load from ConfigMap and CRD", func(t *testing.T) {
		// Create ConfigMap with some values
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shard-controller-config", Namespace: cfg.Namespace},
			Data: map[string]string{
				"minShards":        "3",
				"maxShards":        "12",
				"scaleUpThreshold": "0.85",
			},
		}

		// Create CRD that should override ConfigMap
		shardConfig := &shardv1.ShardConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "default-shard-config", Namespace: cfg.Namespace},
			Spec: shardv1.ShardConfigSpec{
				MinShards:               5, // This should override ConfigMap value
				MaxShards:               15,
				ScaleUpThreshold:        0.95,
				ScaleDownThreshold:      0.25,
				HealthCheckInterval:     metav1.Duration{Duration: 60 * time.Second},
				LoadBalanceStrategy:     shardv1.LeastLoadedStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 60 * time.Second},
			},
		}

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap, shardConfig).Build()
		cm, err := NewConfigManager(cl, cfg)
		require.NoError(t, err)

		loadedCfg, err := cm.LoadConfig(context.Background())
		require.NoError(t, err)
		
		// CRD should take precedence over ConfigMap
		assert.Equal(t, 5, loadedCfg.Spec.MinShards)
		assert.Equal(t, 15, loadedCfg.Spec.MaxShards)
		assert.Equal(t, 0.95, loadedCfg.Spec.ScaleUpThreshold)
	})
}

func TestConfigManager_LoadFromConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	cfg := config.DefaultConfig()

	t.Run("parse individual fields", func(t *testing.T) {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shard-controller-config", Namespace: cfg.Namespace},
			Data: map[string]string{
				"minShards":               "2",
				"maxShards":               "8",
				"scaleUpThreshold":        "0.85",
				"scaleDownThreshold":      "0.25",
				"healthCheckInterval":     "45s",
				"loadBalanceStrategy":     "round-robin",
				"gracefulShutdownTimeout": "45s",
			},
		}

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
		cm, err := NewConfigManager(cl, cfg)
		require.NoError(t, err)

		shardConfig := cm.createDefaultShardConfig()
		err = cm.loadFromConfigMap(context.Background(), shardConfig)
		require.NoError(t, err)

		assert.Equal(t, 2, shardConfig.Spec.MinShards)
		assert.Equal(t, 8, shardConfig.Spec.MaxShards)
		assert.Equal(t, 0.85, shardConfig.Spec.ScaleUpThreshold)
		assert.Equal(t, 0.25, shardConfig.Spec.ScaleDownThreshold)
		assert.Equal(t, 45*time.Second, shardConfig.Spec.HealthCheckInterval.Duration)
		assert.Equal(t, shardv1.RoundRobinStrategy, shardConfig.Spec.LoadBalanceStrategy)
		assert.Equal(t, 45*time.Second, shardConfig.Spec.GracefulShutdownTimeout.Duration)
	})

	t.Run("parse JSON configuration", func(t *testing.T) {
		jsonConfig := shardv1.ShardConfigSpec{
			MinShards:               4,
			MaxShards:               16,
			ScaleUpThreshold:        0.9,
			ScaleDownThreshold:      0.3,
			HealthCheckInterval:     metav1.Duration{Duration: 60 * time.Second},
			LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
			GracefulShutdownTimeout: metav1.Duration{Duration: 60 * time.Second},
		}
		jsonData, err := json.Marshal(jsonConfig)
		require.NoError(t, err)

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shard-controller-config", Namespace: cfg.Namespace},
			Data: map[string]string{
				"minShards":   "2", // This should be overridden by JSON
				"config.json": string(jsonData),
			},
		}

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
		cm, err := NewConfigManager(cl, cfg)
		require.NoError(t, err)

		shardConfig := cm.createDefaultShardConfig()
		err = cm.loadFromConfigMap(context.Background(), shardConfig)
		require.NoError(t, err)

		// JSON should override individual fields
		assert.Equal(t, jsonConfig, shardConfig.Spec)
	})
}

func TestConfigManager_ValidateConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	cm, err := NewConfigManager(cl, cfg)
	require.NoError(t, err)

	tests := []struct {
		name        string
		config      *shardv1.ShardConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &shardv1.ShardConfig{
				Spec: shardv1.ShardConfigSpec{
					MinShards:               1,
					MaxShards:               10,
					ScaleUpThreshold:        0.8,
					ScaleDownThreshold:      0.3,
					HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
					LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
					GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
				},
			},
			expectError: false,
		},
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "invalid min shards",
			config: &shardv1.ShardConfig{
				Spec: shardv1.ShardConfigSpec{
					MinShards:               0,
					MaxShards:               10,
					ScaleUpThreshold:        0.8,
					ScaleDownThreshold:      0.3,
					HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
					LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
					GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
				},
			},
			expectError: true,
		},
		{
			name: "invalid thresholds",
			config: &shardv1.ShardConfig{
				Spec: shardv1.ShardConfigSpec{
					MinShards:               1,
					MaxShards:               10,
					ScaleUpThreshold:        0.3, // Should be > ScaleDownThreshold
					ScaleDownThreshold:      0.8,
					HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
					LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
					GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.ValidateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigManager_ApplyDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	cm, err := NewConfigManager(cl, cfg)
	require.NoError(t, err)

	// Create config with some zero values
	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:               0, // Should be set to default
			MaxShards:               5, // Should remain unchanged
			ScaleUpThreshold:        0, // Should be set to default
			ScaleDownThreshold:      0.2, // Should remain unchanged
			HealthCheckInterval:     metav1.Duration{Duration: 0}, // Should be set to default
			LoadBalanceStrategy:     "", // Should be set to default
			GracefulShutdownTimeout: metav1.Duration{Duration: 45 * time.Second}, // Should remain unchanged
		},
	}

	cm.applyDefaults(shardConfig)

	assert.Equal(t, cfg.DefaultShardConfig.MinShards, shardConfig.Spec.MinShards)
	assert.Equal(t, 5, shardConfig.Spec.MaxShards) // Should remain unchanged
	assert.Equal(t, cfg.DefaultShardConfig.ScaleUpThreshold, shardConfig.Spec.ScaleUpThreshold)
	assert.Equal(t, 0.2, shardConfig.Spec.ScaleDownThreshold) // Should remain unchanged
	assert.Equal(t, cfg.DefaultShardConfig.HealthCheckInterval, shardConfig.Spec.HealthCheckInterval.Duration)
	assert.Equal(t, cfg.DefaultShardConfig.LoadBalanceStrategy, shardConfig.Spec.LoadBalanceStrategy)
	assert.Equal(t, 45*time.Second, shardConfig.Spec.GracefulShutdownTimeout.Duration) // Should remain unchanged
}

func TestConfigManager_ConfigChangeCallbacks(t *testing.T) {
	cfg := config.DefaultConfig()
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	cm, err := NewConfigManager(cl, cfg)
	require.NoError(t, err)

	// Test adding callbacks
	var callbackCount int
	var mu sync.Mutex
	var receivedConfig *shardv1.ShardConfig

	callback := func(config *shardv1.ShardConfig) {
		mu.Lock()
		defer mu.Unlock()
		callbackCount++
		receivedConfig = config
	}

	cm.AddConfigChangeCallback(callback)
	assert.Len(t, cm.callbacks, 1)

	// Test callback execution
	testConfig := &shardv1.ShardConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: cfg.Namespace},
		Spec: shardv1.ShardConfigSpec{
			MinShards:               2,
			MaxShards:               8,
			ScaleUpThreshold:        0.8,
			ScaleDownThreshold:      0.3,
			HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
			GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
		},
	}

	cm.handleConfigChange(testConfig)

	// Wait for callback to be executed
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 1, callbackCount)
	assert.NotNil(t, receivedConfig)
	assert.Equal(t, testConfig.Spec, receivedConfig.Spec)
	mu.Unlock()

	// Test removing callbacks
	cm.RemoveAllCallbacks()
	assert.Len(t, cm.callbacks, 0)
}

func TestConfigManager_UpdateConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	cfg := config.DefaultConfig()

	t.Run("create new ConfigMap", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).Build()
		cm, err := NewConfigManager(cl, cfg)
		require.NoError(t, err)

		data := map[string]string{
			"minShards": "3",
			"maxShards": "9",
		}

		err = cm.UpdateConfigMap(context.Background(), data)
		require.NoError(t, err)

		// Verify ConfigMap was created
		configMap := &corev1.ConfigMap{}
		err = cl.Get(context.Background(), types.NamespacedName{
			Namespace: cfg.Namespace,
			Name:      "shard-controller-config",
		}, configMap)
		require.NoError(t, err)
		assert.Equal(t, data, configMap.Data)
	})

	t.Run("update existing ConfigMap", func(t *testing.T) {
		existingConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shard-controller-config", Namespace: cfg.Namespace},
			Data: map[string]string{
				"minShards": "1",
				"maxShards": "5",
			},
		}

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingConfigMap).Build()
		cm, err := NewConfigManager(cl, cfg)
		require.NoError(t, err)

		newData := map[string]string{
			"minShards": "2",
			"maxShards": "8",
			"scaleUpThreshold": "0.9",
		}

		err = cm.UpdateConfigMap(context.Background(), newData)
		require.NoError(t, err)

		// Verify ConfigMap was updated
		configMap := &corev1.ConfigMap{}
		err = cl.Get(context.Background(), types.NamespacedName{
			Namespace: cfg.Namespace,
			Name:      "shard-controller-config",
		}, configMap)
		require.NoError(t, err)
		assert.Equal(t, newData, configMap.Data)
	})
}

func TestConfigManager_GetCurrentConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	cm, err := NewConfigManager(cl, cfg)
	require.NoError(t, err)

	// Initially should return nil
	assert.Nil(t, cm.GetCurrentConfig())

	// Load config
	_, err = cm.LoadConfig(context.Background())
	require.NoError(t, err)

	// Should return a copy of the current config
	currentConfig := cm.GetCurrentConfig()
	assert.NotNil(t, currentConfig)
	assert.Equal(t, cfg.DefaultShardConfig.MinShards, currentConfig.Spec.MinShards)

	// Modifying returned config should not affect internal state
	currentConfig.Spec.MinShards = 999
	internalConfig := cm.GetCurrentConfig()
	assert.NotEqual(t, 999, internalConfig.Spec.MinShards)
}

func TestConfigManager_GetConfigMapData(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	cfg := config.DefaultConfig()
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "shard-controller-config", Namespace: cfg.Namespace},
		Data: map[string]string{
			"minShards": "2",
			"maxShards": "8",
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
	cm, err := NewConfigManager(cl, cfg)
	require.NoError(t, err)

	// Load config to populate ConfigMap data
	_, err = cm.LoadConfig(context.Background())
	require.NoError(t, err)

	// Get ConfigMap data
	data := cm.GetConfigMapData()
	assert.Equal(t, "2", data["minShards"])
	assert.Equal(t, "8", data["maxShards"])

	// Modifying returned data should not affect internal state
	data["minShards"] = "999"
	internalData := cm.GetConfigMapData()
	assert.Equal(t, "2", internalData["minShards"])
}
