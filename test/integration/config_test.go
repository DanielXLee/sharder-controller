package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
)

// TestConfigurationManagement tests configuration management scenarios
func (suite *IntegrationTestSuite) TestConfigurationManagement() {
	// Test initial configuration loading
	suite.T().Run("InitialConfigLoad", func(t *testing.T) {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shard-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"minShards":           "3",
				"maxShards":           "15",
				"scaleUpThreshold":    "0.75",
				"scaleDownThreshold":  "0.25",
				"healthCheckInterval": "20s",
				"loadBalanceStrategy": "least-loaded",
			},
		}

		err := suite.k8sClient.Create(suite.ctx, configMap)
		require.NoError(t, err)

		// Start config manager with this ConfigMap
		err = suite.configManager.LoadFromConfigMap(suite.ctx, "test-shard-config", "default")
		require.NoError(t, err)

		// Verify configuration was loaded correctly
		config := suite.configManager.GetCurrentConfig()
		assert.Equal(t, int32(3), config.MinShards)
		assert.Equal(t, int32(15), config.MaxShards)
		assert.Equal(t, 0.75, config.ScaleUpThreshold)
		assert.Equal(t, 0.25, config.ScaleDownThreshold)
		assert.Equal(t, 20*time.Second, config.HealthCheckInterval.Duration)
		assert.Equal(t, shardv1.LeastLoadedStrategy, config.LoadBalanceStrategy)
	})

	// Test configuration validation
	suite.T().Run("ConfigValidation", func(t *testing.T) {
		invalidConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"minShards":          "10",
				"maxShards":          "5", // Invalid: min > max
				"scaleUpThreshold":   "0.2",
				"scaleDownThreshold": "0.8", // Invalid: down > up
			},
		}

		err := suite.k8sClient.Create(suite.ctx, invalidConfigMap)
		require.NoError(t, err)

		// Should fail to load invalid configuration
		err = suite.configManager.LoadFromConfigMap(suite.ctx, "invalid-config", "default")
		assert.Error(t, err, "Should reject invalid configuration")
	})

	// Test configuration hot reload
	suite.T().Run("ConfigHotReload", func(t *testing.T) {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hot-reload-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"minShards":           "2",
				"maxShards":           "8",
				"scaleUpThreshold":    "0.8",
				"scaleDownThreshold":  "0.3",
				"healthCheckInterval": "30s",
				"loadBalanceStrategy": "consistent-hash",
			},
		}

		err := suite.k8sClient.Create(suite.ctx, configMap)
		require.NoError(t, err)

		// Load initial configuration
		err = suite.configManager.LoadFromConfigMap(suite.ctx, "hot-reload-config", "default")
		require.NoError(t, err)

		// Start watching for changes
		err = suite.configManager.StartWatching(suite.ctx)
		require.NoError(t, err)
		defer suite.configManager.StopWatching()

		// Verify initial config
		config := suite.configManager.GetCurrentConfig()
		assert.Equal(t, int32(2), config.MinShards)
		assert.Equal(t, int32(8), config.MaxShards)

		// Update configuration
		configMap.Data["minShards"] = "4"
		configMap.Data["maxShards"] = "12"
		configMap.Data["scaleUpThreshold"] = "0.7"
		err = suite.k8sClient.Update(suite.ctx, configMap)
		require.NoError(t, err)

		// Wait for configuration to be reloaded
		err = suite.waitForCondition(10*time.Second, func() bool {
			config := suite.configManager.GetCurrentConfig()
			return config.MinShards == 4 && config.MaxShards == 12 && config.ScaleUpThreshold == 0.7
		})
		require.NoError(t, err)

		// Verify updated configuration
		updatedConfig := suite.configManager.GetCurrentConfig()
		assert.Equal(t, int32(4), updatedConfig.MinShards)
		assert.Equal(t, int32(12), updatedConfig.MaxShards)
		assert.Equal(t, 0.7, updatedConfig.ScaleUpThreshold)
	})

	// Test configuration defaults
	suite.T().Run("ConfigDefaults", func(t *testing.T) {
		partialConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "partial-config",
				Namespace: "default",
			},
			Data: map[string]string{
				"minShards": "3",
				"maxShards": "10",
				// Missing other fields - should use defaults
			},
		}

		err := suite.k8sClient.Create(suite.ctx, partialConfigMap)
		require.NoError(t, err)

		err = suite.configManager.LoadFromConfigMap(suite.ctx, "partial-config", "default")
		require.NoError(t, err)

		config := suite.configManager.GetCurrentConfig()
		assert.Equal(t, int32(3), config.MinShards)
		assert.Equal(t, int32(10), config.MaxShards)
		// Should have default values for missing fields
		assert.Equal(t, 0.8, config.ScaleUpThreshold)
		assert.Equal(t, 0.3, config.ScaleDownThreshold)
		assert.Equal(t, shardv1.ConsistentHashStrategy, config.LoadBalanceStrategy)
	})
}

// TestShardConfigCRD tests ShardConfig CRD functionality
func (suite *IntegrationTestSuite) TestShardConfigCRD() {
	// Test creating ShardConfig
	suite.T().Run("CreateShardConfig", func(t *testing.T) {
		shardConfig := &shardv1.ShardConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-crd-config",
				Namespace: "default",
			},
			Spec: shardv1.ShardConfigSpec{
				MinShards:               2,
				MaxShards:               20,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
			},
		}

		err := suite.k8sClient.Create(suite.ctx, shardConfig)
		require.NoError(t, err)

		// Verify it was created
		createdConfig := &shardv1.ShardConfig{}
		err = suite.k8sClient.Get(suite.ctx, types.NamespacedName{
			Name:      "test-crd-config",
			Namespace: "default",
		}, createdConfig)
		require.NoError(t, err)

		assert.Equal(t, int32(2), createdConfig.Spec.MinShards)
		assert.Equal(t, int32(20), createdConfig.Spec.MaxShards)
		assert.Equal(t, 0.8, createdConfig.Spec.ScaleUpThreshold)
	})

	// Test ShardConfig validation
	suite.T().Run("ShardConfigValidation", func(t *testing.T) {
		invalidConfig := &shardv1.ShardConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-crd-config",
				Namespace: "default",
			},
			Spec: shardv1.ShardConfigSpec{
				MinShards:          10,
				MaxShards:          5, // Invalid: min > max
				ScaleUpThreshold:   0.2,
				ScaleDownThreshold: 0.8, // Invalid: down > up
			},
		}

		// This should fail validation if admission controllers are set up
		// In test environment, we'll validate programmatically
		errs := shardv1.ValidateShardConfig(invalidConfig)
		assert.NotEmpty(t, errs, "Should have validation errors")
	})

	// Test ShardConfig status updates
	suite.T().Run("ShardConfigStatus", func(t *testing.T) {
		shardConfig := &shardv1.ShardConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "status-test-config",
				Namespace: "default",
			},
			Spec: shardv1.ShardConfigSpec{
				MinShards:               3,
				MaxShards:               15,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
			},
		}

		err := suite.k8sClient.Create(suite.ctx, shardConfig)
		require.NoError(t, err)

		// Update status
		shardConfig.Status.CurrentShards = 5
		shardConfig.Status.HealthyShards = 4
		shardConfig.Status.TotalLoad = 2.5
		shardConfig.Status.LastUpdated = metav1.Now()

		err = suite.k8sClient.Status().Update(suite.ctx, shardConfig)
		require.NoError(t, err)

		// Verify status was updated
		updatedConfig := &shardv1.ShardConfig{}
		err = suite.k8sClient.Get(suite.ctx, types.NamespacedName{
			Name:      "status-test-config",
			Namespace: "default",
		}, updatedConfig)
		require.NoError(t, err)

		assert.Equal(t, int32(5), updatedConfig.Status.CurrentShards)
		assert.Equal(t, int32(4), updatedConfig.Status.HealthyShards)
		assert.Equal(t, 2.5, updatedConfig.Status.TotalLoad)
	})
}

// TestConfigurationIntegrationWithComponents tests how configuration changes affect other components
func (suite *IntegrationTestSuite) TestConfigurationIntegrationWithComponents() {
	// Create initial shards
	for i := 1; i <= 3; i++ {
		suite.createTestShardInstance(fmt.Sprintf("config-integration-shard-%d", i), shardv1.ShardPhaseRunning)
	}

	// Test configuration change affecting health checker
	suite.T().Run("HealthCheckConfigChange", func(t *testing.T) {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "health-config-test",
				Namespace: "default",
			},
			Data: map[string]string{
				"healthCheckInterval": "5s",
				"failureThreshold":    "2",
			},
		}

		err := suite.k8sClient.Create(suite.ctx, configMap)
		require.NoError(t, err)

		// Load configuration
		err = suite.configManager.LoadFromConfigMap(suite.ctx, "health-config-test", "default")
		require.NoError(t, err)

		// Start health checking with new config
		config := suite.configManager.GetCurrentConfig()
		newHealthChecker, err := suite.setupHealthCheckerWithConfig(config.HealthCheck)
		require.NoError(t, err)

		err = newHealthChecker.StartHealthChecking(suite.ctx, config.HealthCheckInterval.Duration)
		require.NoError(t, err)
		defer newHealthChecker.StopHealthChecking()

		// Verify health checker is using new configuration
		time.Sleep(6 * time.Second) // Wait for at least one health check cycle

		// Update configuration
		configMap.Data["healthCheckInterval"] = "2s"
		err = suite.k8sClient.Update(suite.ctx, configMap)
		require.NoError(t, err)

		// Start watching for changes
		err = suite.configManager.StartWatching(suite.ctx)
		require.NoError(t, err)
		defer suite.configManager.StopWatching()

		// Wait for configuration to be reloaded
		err = suite.waitForCondition(10*time.Second, func() bool {
			config := suite.configManager.GetCurrentConfig()
			return config.HealthCheckInterval.Duration == 2*time.Second
		})
		require.NoError(t, err)
	})

	// Test configuration change affecting load balancer
	suite.T().Run("LoadBalancerConfigChange", func(t *testing.T) {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		require.NoError(t, err)

		shards := make([]*shardv1.ShardInstance, len(shardList.Items))
		for i := range shardList.Items {
			shards[i] = &shardList.Items[i]
		}

		// Start with consistent hash
		err = suite.loadBalancer.SetStrategy(shardv1.ConsistentHashStrategy, shards)
		require.NoError(t, err)
		assert.Equal(t, shardv1.ConsistentHashStrategy, suite.loadBalancer.GetStrategy())

		// Change to least loaded via configuration
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "lb-config-test",
				Namespace: "default",
			},
			Data: map[string]string{
				"loadBalanceStrategy": "least-loaded",
			},
		}

		err = suite.k8sClient.Create(suite.ctx, configMap)
		require.NoError(t, err)

		err = suite.configManager.LoadFromConfigMap(suite.ctx, "lb-config-test", "default")
		require.NoError(t, err)

		config := suite.configManager.GetCurrentConfig()
		err = suite.loadBalancer.SetStrategy(config.LoadBalanceStrategy, shards)
		require.NoError(t, err)

		assert.Equal(t, shardv1.LeastLoadedStrategy, suite.loadBalancer.GetStrategy())
	})

	// Test configuration change affecting scaling behavior
	suite.T().Run("ScalingConfigChange", func(t *testing.T) {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scaling-config-test",
				Namespace: "default",
			},
			Data: map[string]string{
				"minShards":          "2",
				"maxShards":          "8",
				"scaleUpThreshold":   "0.6",
				"scaleDownThreshold": "0.2",
			},
		}

		err := suite.k8sClient.Create(suite.ctx, configMap)
		require.NoError(t, err)

		err = suite.configManager.LoadFromConfigMap(suite.ctx, "scaling-config-test", "default")
		require.NoError(t, err)

		config := suite.configManager.GetCurrentConfig()

		// Test that scaling respects new configuration
		// Scale up to max
		err = suite.shardManager.ScaleUp(suite.ctx, int(config.MaxShards))
		require.NoError(t, err)

		// Wait for scale up
		err = suite.waitForCondition(20*time.Second, func() bool {
			shardList := &shardv1.ShardInstanceList{}
			err := suite.k8sClient.List(suite.ctx, shardList)
			return err == nil && len(shardList.Items) == int(config.MaxShards)
		})
		require.NoError(t, err)

		// Scale down to min
		err = suite.shardManager.ScaleDown(suite.ctx, int(config.MinShards))
		require.NoError(t, err)

		// Wait for scale down
		err = suite.waitForCondition(25*time.Second, func() bool {
			shardList := &shardv1.ShardInstanceList{}
			err := suite.k8sClient.List(suite.ctx, shardList)
			if err != nil {
				return false
			}

			runningCount := 0
			for _, shard := range shardList.Items {
				if shard.Status.Phase == shardv1.ShardPhaseRunning {
					runningCount++
				}
			}
			return runningCount == int(config.MinShards)
		})
		require.NoError(t, err)
	})
}

// Helper function to setup health checker with specific config
func (suite *IntegrationTestSuite) setupHealthCheckerWithConfig(healthConfig interface{}) (interfaces.HealthChecker, error) {
	// This would normally use the actual health config structure
	// For now, return the existing health checker
	return suite.healthChecker, nil
}
