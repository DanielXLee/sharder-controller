package controllers

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
)

// DemoConfigManager demonstrates the enhanced ConfigManager functionality
func DemoConfigManager() error {
	fmt.Println("=== ConfigManager Demo ===")

	// Create a fake client for demonstration
	scheme := runtime.NewScheme()
	if err := shardv1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add scheme: %w", err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	cfg := config.DefaultConfig()

	// Create ConfigManager with custom options
	opts := &ConfigManagerOptions{
		ConfigMapName:   "demo-config",
		ShardConfigName: "demo-shard-config",
		EnableConfigMap: true,
		EnableHotReload: true,
	}

	cm, err := NewConfigManagerWithOptions(client, cfg, opts)
	if err != nil {
		return fmt.Errorf("failed to create ConfigManager: %w", err)
	}

	ctx := context.Background()

	// 1. Load initial configuration
	fmt.Println("\n1. Loading initial configuration...")
	shardConfig, err := cm.LoadConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	fmt.Printf("   Initial config: MinShards=%d, MaxShards=%d, ScaleUpThreshold=%.2f\n",
		shardConfig.Spec.MinShards, shardConfig.Spec.MaxShards, shardConfig.Spec.ScaleUpThreshold)

	// 2. Demonstrate configuration validation
	fmt.Println("\n2. Testing configuration validation...")

	// Valid config
	validConfig := &shardv1.ShardConfig{
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

	if err := cm.ValidateConfig(validConfig); err != nil {
		fmt.Printf("   Unexpected validation error: %v\n", err)
	} else {
		fmt.Println("   ✓ Valid configuration passed validation")
	}

	// Invalid config
	invalidConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:               0, // Invalid: must be > 0
			MaxShards:               8,
			ScaleUpThreshold:        0.3, // Invalid: must be > ScaleDownThreshold
			ScaleDownThreshold:      0.8,
			HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
			GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
		},
	}

	if err := cm.ValidateConfig(invalidConfig); err != nil {
		fmt.Printf("   ✓ Invalid configuration rejected: %v\n", err)
	} else {
		fmt.Println("   ✗ Invalid configuration should have been rejected")
	}

	// 3. Demonstrate default value application
	fmt.Println("\n3. Testing default value application...")
	configWithDefaults := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:        0, // Will be set to default
			MaxShards:        5, // Will remain unchanged
			ScaleUpThreshold: 0, // Will be set to default
		},
	}

	cm.applyDefaults(configWithDefaults)
	fmt.Printf("   After applying defaults: MinShards=%d, MaxShards=%d, ScaleUpThreshold=%.2f\n",
		configWithDefaults.Spec.MinShards, configWithDefaults.Spec.MaxShards, configWithDefaults.Spec.ScaleUpThreshold)

	// 4. Demonstrate ConfigMap operations
	fmt.Println("\n4. Testing ConfigMap operations...")
	configMapData := map[string]string{
		"minShards":        "3",
		"maxShards":        "12",
		"scaleUpThreshold": "0.85",
	}

	if err := cm.UpdateConfigMap(ctx, configMapData); err != nil {
		fmt.Printf("   Failed to update ConfigMap: %v\n", err)
	} else {
		fmt.Println("   ✓ ConfigMap updated successfully")
	}

	// 5. Demonstrate callback registration
	fmt.Println("\n5. Testing configuration change callbacks...")
	callbackExecuted := false

	callback := func(config *shardv1.ShardConfig) {
		callbackExecuted = true
		fmt.Printf("   ✓ Callback executed with config: MinShards=%d\n", config.Spec.MinShards)
	}

	cm.AddConfigChangeCallback(callback)

	// Simulate a configuration change
	testConfig := &shardv1.ShardConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: cfg.Namespace},
		Spec: shardv1.ShardConfigSpec{
			MinShards:               4,
			MaxShards:               16,
			ScaleUpThreshold:        0.9,
			ScaleDownThreshold:      0.2,
			HealthCheckInterval:     metav1.Duration{Duration: 45 * time.Second},
			LoadBalanceStrategy:     shardv1.RoundRobinStrategy,
			GracefulShutdownTimeout: metav1.Duration{Duration: 45 * time.Second},
		},
	}

	cm.handleConfigChange(testConfig)

	// Wait for callback to execute
	time.Sleep(100 * time.Millisecond)

	if callbackExecuted {
		fmt.Println("   ✓ Configuration change callback executed successfully")
	} else {
		fmt.Println("   ✗ Configuration change callback was not executed")
	}

	// 6. Demonstrate current config retrieval
	fmt.Println("\n6. Testing current configuration retrieval...")
	currentConfig := cm.GetCurrentConfig()
	if currentConfig != nil {
		fmt.Printf("   Current config: MinShards=%d, MaxShards=%d\n",
			currentConfig.Spec.MinShards, currentConfig.Spec.MaxShards)
	} else {
		fmt.Println("   No current configuration available")
	}

	// 7. Demonstrate ConfigMap data retrieval
	fmt.Println("\n7. Testing ConfigMap data retrieval...")
	configMapDataRetrieved := cm.GetConfigMapData()
	if len(configMapDataRetrieved) > 0 {
		fmt.Printf("   ConfigMap data: %v\n", configMapDataRetrieved)
	} else {
		fmt.Println("   No ConfigMap data available")
	}

	fmt.Println("\n=== Demo completed successfully ===")
	return nil
}

// DemoConfigManagerFeatures demonstrates specific features of the ConfigManager
func DemoConfigManagerFeatures() {
	fmt.Println("\n=== ConfigManager Features Demo ===")

	// Feature 1: Multiple configuration sources
	fmt.Println("\n1. Multiple Configuration Sources:")
	fmt.Println("   - CRD (ShardConfig): Primary source with full validation")
	fmt.Println("   - ConfigMap: Secondary source for simple key-value configs")
	fmt.Println("   - JSON in ConfigMap: Structured configuration support")
	fmt.Println("   - Default values: Fallback when sources are unavailable")

	// Feature 2: Hot reload mechanism
	fmt.Println("\n2. Hot Reload Mechanism:")
	fmt.Println("   - Watches CRD changes using Kubernetes informers")
	fmt.Println("   - Watches ConfigMap changes for dynamic updates")
	fmt.Println("   - Callback system for notifying components of changes")
	fmt.Println("   - Graceful handling of invalid configurations")

	// Feature 3: Validation and defaults
	fmt.Println("\n3. Validation and Default Handling:")
	fmt.Println("   - Comprehensive validation of all configuration fields")
	fmt.Println("   - Automatic application of default values")
	fmt.Println("   - Validation of cross-field dependencies")
	fmt.Println("   - Support for custom validation rules")

	// Feature 4: Configuration management
	fmt.Println("\n4. Configuration Management:")
	fmt.Println("   - Thread-safe configuration access")
	fmt.Println("   - Immutable configuration copies")
	fmt.Println("   - Configuration versioning and history")
	fmt.Println("   - Rollback support for invalid changes")

	// Feature 5: Integration features
	fmt.Println("\n5. Integration Features:")
	fmt.Println("   - Controller-runtime manager integration")
	fmt.Println("   - Kubernetes client integration")
	fmt.Println("   - Metrics and monitoring support")
	fmt.Println("   - Structured logging for configuration changes")

	fmt.Println("\n=== Features demo completed ===")
}
