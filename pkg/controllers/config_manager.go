package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/utils"
)

// ConfigManager implements the ConfigManager interface
type ConfigManager struct {
	client  client.Client
	manager manager.Manager
	config  *config.Config

	mu                sync.RWMutex
	shardConfig       *shardv1.ShardConfig
	configMapData     map[string]string
	watchContext      context.Context
	watchCancel       context.CancelFunc
	configMapWatchCtx context.Context
	configMapCancel   context.CancelFunc
	callbacks         []func(*shardv1.ShardConfig)

	// Configuration sources
	configMapName   string
	shardConfigName string
	enableConfigMap bool
	enableHotReload bool
}

// ConfigManagerOptions defines options for creating a ConfigManager
type ConfigManagerOptions struct {
	ConfigMapName   string
	ShardConfigName string
	EnableConfigMap bool
	EnableHotReload bool
}

// DefaultConfigManagerOptions returns default options
func DefaultConfigManagerOptions() *ConfigManagerOptions {
	return &ConfigManagerOptions{
		ConfigMapName:   "shard-controller-config",
		ShardConfigName: "default-shard-config",
		EnableConfigMap: true,
		EnableHotReload: true,
	}
}

// NewConfigManager creates a new ConfigManager
func NewConfigManager(client client.Client, cfg *config.Config) (*ConfigManager, error) {
	return NewConfigManagerWithOptions(client, cfg, DefaultConfigManagerOptions())
}

// NewConfigManagerWithOptions creates a new ConfigManager with custom options
func NewConfigManagerWithOptions(client client.Client, cfg *config.Config, opts *ConfigManagerOptions) (*ConfigManager, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if opts == nil {
		opts = DefaultConfigManagerOptions()
	}

	return &ConfigManager{
		client:          client,
		config:          cfg,
		configMapData:   make(map[string]string),
		callbacks:       make([]func(*shardv1.ShardConfig), 0),
		configMapName:   opts.ConfigMapName,
		shardConfigName: opts.ShardConfigName,
		enableConfigMap: opts.EnableConfigMap,
		enableHotReload: opts.EnableHotReload,
	}, nil
}

// LoadConfig loads the ShardConfig from multiple sources (CRD and ConfigMap)
func (cm *ConfigManager) LoadConfig(ctx context.Context) (*shardv1.ShardConfig, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Start with default configuration
	shardConfig := cm.createDefaultShardConfig()

	// Load from ConfigMap if enabled
	if cm.enableConfigMap {
		if err := cm.loadFromConfigMap(ctx, shardConfig); err != nil {
			// Log warning but continue with defaults
			fmt.Printf("Warning: failed to load from ConfigMap: %v\n", err)
		}
	}

	// Load from CRD (takes precedence over ConfigMap)
	if err := cm.loadFromCRD(ctx, shardConfig); err != nil {
		// If CRD doesn't exist, create it with current config
		if errors.IsNotFound(err) {
			if err := cm.createDefaultCRD(ctx, shardConfig); err != nil {
				return nil, fmt.Errorf("failed to create default ShardConfig CRD: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load from CRD: %w", err)
		}
	}

	// Apply defaults and validate
	cm.applyDefaults(shardConfig)
	if err := cm.ValidateConfig(shardConfig); err != nil {
		return nil, fmt.Errorf("invalid configuration after loading: %w", err)
	}

	cm.shardConfig = shardConfig
	return shardConfig, nil
}

// ReloadConfig reloads the ShardConfig from Kubernetes
func (cm *ConfigManager) ReloadConfig(ctx context.Context) error {
	_, err := cm.LoadConfig(ctx)
	return err
}

// ValidateConfig validates the ShardConfig
func (cm *ConfigManager) ValidateConfig(config *shardv1.ShardConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	return utils.ValidateShardConfig(&config.Spec)
}

// WatchConfigChanges watches for changes to the ShardConfig
func (cm *ConfigManager) WatchConfigChanges(ctx context.Context, callback func(*shardv1.ShardConfig)) error {
	// Add the callback to the list of callbacks
	cm.AddConfigChangeCallback(callback)

	// Start hot reload if not already started
	return cm.StartHotReload(ctx)
}

func (cm *ConfigManager) updateConfig(shardConfig *shardv1.ShardConfig, callback func(*shardv1.ShardConfig)) {
	if err := utils.ValidateShardConfig(&shardConfig.Spec); err == nil {
		cm.mu.Lock()
		cm.shardConfig = shardConfig
		cm.mu.Unlock()
		callback(shardConfig)
	} else {
		fmt.Printf("Invalid ShardConfig: %v\n", err)
	}
}

// createDefaultShardConfig creates a default ShardConfig from the base config
func (cm *ConfigManager) createDefaultShardConfig() *shardv1.ShardConfig {
	return &shardv1.ShardConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.shardConfigName,
			Namespace: cm.config.Namespace,
		},
		Spec: shardv1.ShardConfigSpec{
			MinShards:               cm.config.DefaultShardConfig.MinShards,
			MaxShards:               cm.config.DefaultShardConfig.MaxShards,
			ScaleUpThreshold:        cm.config.DefaultShardConfig.ScaleUpThreshold,
			ScaleDownThreshold:      cm.config.DefaultShardConfig.ScaleDownThreshold,
			HealthCheckInterval:     metav1.Duration{Duration: cm.config.DefaultShardConfig.HealthCheckInterval},
			LoadBalanceStrategy:     cm.config.DefaultShardConfig.LoadBalanceStrategy,
			GracefulShutdownTimeout: metav1.Duration{Duration: cm.config.DefaultShardConfig.GracefulShutdownTimeout},
		},
	}
}

// loadFromConfigMap loads configuration from a ConfigMap
func (cm *ConfigManager) loadFromConfigMap(ctx context.Context, shardConfig *shardv1.ShardConfig) error {
	configMap := &corev1.ConfigMap{}
	err := cm.client.Get(ctx, types.NamespacedName{
		Namespace: cm.config.Namespace,
		Name:      cm.configMapName,
	}, configMap)

	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap doesn't exist, create a default one
			return cm.createDefaultConfigMap(ctx)
		}
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	// Store ConfigMap data for reference
	cm.configMapData = configMap.Data

	// Parse configuration from ConfigMap data
	return cm.parseConfigMapData(configMap.Data, shardConfig)
}

// loadFromCRD loads configuration from the ShardConfig CRD
func (cm *ConfigManager) loadFromCRD(ctx context.Context, shardConfig *shardv1.ShardConfig) error {
	crdConfig := &shardv1.ShardConfig{}
	err := cm.client.Get(ctx, types.NamespacedName{
		Namespace: cm.config.Namespace,
		Name:      cm.shardConfigName,
	}, crdConfig)

	if err != nil {
		return err
	}

	// Merge CRD config into the current config (CRD takes precedence)
	cm.mergeCRDConfig(crdConfig, shardConfig)
	return nil
}

// createDefaultCRD creates a default ShardConfig CRD
func (cm *ConfigManager) createDefaultCRD(ctx context.Context, shardConfig *shardv1.ShardConfig) error {
	return cm.client.Create(ctx, shardConfig)
}

// createDefaultConfigMap creates a default ConfigMap with current configuration
func (cm *ConfigManager) createDefaultConfigMap(ctx context.Context) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.configMapName,
			Namespace: cm.config.Namespace,
		},
		Data: map[string]string{
			"minShards":               strconv.Itoa(cm.config.DefaultShardConfig.MinShards),
			"maxShards":               strconv.Itoa(cm.config.DefaultShardConfig.MaxShards),
			"scaleUpThreshold":        fmt.Sprintf("%.2f", cm.config.DefaultShardConfig.ScaleUpThreshold),
			"scaleDownThreshold":      fmt.Sprintf("%.2f", cm.config.DefaultShardConfig.ScaleDownThreshold),
			"healthCheckInterval":     cm.config.DefaultShardConfig.HealthCheckInterval.String(),
			"loadBalanceStrategy":     string(cm.config.DefaultShardConfig.LoadBalanceStrategy),
			"gracefulShutdownTimeout": cm.config.DefaultShardConfig.GracefulShutdownTimeout.String(),
		},
	}

	return cm.client.Create(ctx, configMap)
}

// parseConfigMapData parses configuration data from ConfigMap
func (cm *ConfigManager) parseConfigMapData(data map[string]string, shardConfig *shardv1.ShardConfig) error {
	// Parse minShards
	if val, exists := data["minShards"]; exists {
		if minShards, err := strconv.Atoi(val); err == nil {
			shardConfig.Spec.MinShards = minShards
		}
	}

	// Parse maxShards
	if val, exists := data["maxShards"]; exists {
		if maxShards, err := strconv.Atoi(val); err == nil {
			shardConfig.Spec.MaxShards = maxShards
		}
	}

	// Parse scaleUpThreshold
	if val, exists := data["scaleUpThreshold"]; exists {
		if threshold, err := strconv.ParseFloat(val, 64); err == nil {
			shardConfig.Spec.ScaleUpThreshold = threshold
		}
	}

	// Parse scaleDownThreshold
	if val, exists := data["scaleDownThreshold"]; exists {
		if threshold, err := strconv.ParseFloat(val, 64); err == nil {
			shardConfig.Spec.ScaleDownThreshold = threshold
		}
	}

	// Parse healthCheckInterval
	if val, exists := data["healthCheckInterval"]; exists {
		if duration, err := time.ParseDuration(val); err == nil {
			shardConfig.Spec.HealthCheckInterval = metav1.Duration{Duration: duration}
		}
	}

	// Parse loadBalanceStrategy
	if val, exists := data["loadBalanceStrategy"]; exists {
		shardConfig.Spec.LoadBalanceStrategy = shardv1.LoadBalanceStrategy(val)
	}

	// Parse gracefulShutdownTimeout
	if val, exists := data["gracefulShutdownTimeout"]; exists {
		if duration, err := time.ParseDuration(val); err == nil {
			shardConfig.Spec.GracefulShutdownTimeout = metav1.Duration{Duration: duration}
		}
	}

	// Parse JSON configuration if present
	if val, exists := data["config.json"]; exists {
		var jsonConfig shardv1.ShardConfigSpec
		if err := json.Unmarshal([]byte(val), &jsonConfig); err == nil {
			// JSON config overrides individual fields
			shardConfig.Spec = jsonConfig
		}
	}

	return nil
}

// mergeCRDConfig merges CRD configuration into the target config
func (cm *ConfigManager) mergeCRDConfig(source, target *shardv1.ShardConfig) {
	// CRD takes precedence over ConfigMap, so we replace the entire spec
	target.Spec = source.Spec
	target.ObjectMeta = source.ObjectMeta
}

// applyDefaults applies default values to configuration fields that are not set
func (cm *ConfigManager) applyDefaults(shardConfig *shardv1.ShardConfig) {
	defaults := cm.config.DefaultShardConfig

	// Apply defaults only if values are zero/empty
	if shardConfig.Spec.MinShards <= 0 {
		shardConfig.Spec.MinShards = defaults.MinShards
	}
	if shardConfig.Spec.MaxShards <= 0 {
		shardConfig.Spec.MaxShards = defaults.MaxShards
	}
	if shardConfig.Spec.ScaleUpThreshold <= 0 {
		shardConfig.Spec.ScaleUpThreshold = defaults.ScaleUpThreshold
	}
	if shardConfig.Spec.ScaleDownThreshold <= 0 {
		shardConfig.Spec.ScaleDownThreshold = defaults.ScaleDownThreshold
	}
	if shardConfig.Spec.HealthCheckInterval.Duration <= 0 {
		shardConfig.Spec.HealthCheckInterval = metav1.Duration{Duration: defaults.HealthCheckInterval}
	}
	if shardConfig.Spec.LoadBalanceStrategy == "" {
		shardConfig.Spec.LoadBalanceStrategy = defaults.LoadBalanceStrategy
	}
	if shardConfig.Spec.GracefulShutdownTimeout.Duration <= 0 {
		shardConfig.Spec.GracefulShutdownTimeout = metav1.Duration{Duration: defaults.GracefulShutdownTimeout}
	}
}

// StartHotReload starts the hot reload mechanism for configuration changes
func (cm *ConfigManager) StartHotReload(ctx context.Context) error {
	if !cm.enableHotReload {
		return nil
	}

	// Start watching CRD changes
	if err := cm.startCRDWatch(ctx); err != nil {
		return fmt.Errorf("failed to start CRD watch: %w", err)
	}

	// Start watching ConfigMap changes if enabled
	if cm.enableConfigMap {
		if err := cm.startConfigMapWatch(ctx); err != nil {
			return fmt.Errorf("failed to start ConfigMap watch: %w", err)
		}
	}

	return nil
}

// StopHotReload stops the hot reload mechanism
func (cm *ConfigManager) StopHotReload() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.watchCancel != nil {
		cm.watchCancel()
		cm.watchCancel = nil
	}

	if cm.configMapCancel != nil {
		cm.configMapCancel()
		cm.configMapCancel = nil
	}
}

// AddConfigChangeCallback adds a callback function to be called when configuration changes
func (cm *ConfigManager) AddConfigChangeCallback(callback func(*shardv1.ShardConfig)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.callbacks = append(cm.callbacks, callback)
}

// RemoveAllCallbacks removes all registered callbacks
func (cm *ConfigManager) RemoveAllCallbacks() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.callbacks = cm.callbacks[:0]
}

// GetCurrentConfig returns the currently loaded configuration
func (cm *ConfigManager) GetCurrentConfig() *shardv1.ShardConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.shardConfig == nil {
		return nil
	}

	// Return a deep copy to prevent external modifications
	return cm.shardConfig.DeepCopy()
}

// startCRDWatch starts watching for CRD changes
func (cm *ConfigManager) startCRDWatch(ctx context.Context) error {
	if cm.manager == nil {
		return fmt.Errorf("manager is not set, cannot start CRD watch")
	}

	// Cancel any existing watch
	if cm.watchCancel != nil {
		cm.watchCancel()
	}

	// Create a new watch context
	watchCtx, cancel := context.WithCancel(ctx)
	cm.watchContext = watchCtx
	cm.watchCancel = cancel

	// Create a new informer for ShardConfig
	informer, err := cm.manager.GetCache().GetInformer(ctx, &shardv1.ShardConfig{})
	if err != nil {
		return fmt.Errorf("failed to get informer for ShardConfig: %w", err)
	}

	// Add an event handler to the informer
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			shardConfig := obj.(*shardv1.ShardConfig)
			if shardConfig.Name == cm.shardConfigName && shardConfig.Namespace == cm.config.Namespace {
				cm.handleConfigChange(shardConfig)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			shardConfig := newObj.(*shardv1.ShardConfig)
			if shardConfig.Name == cm.shardConfigName && shardConfig.Namespace == cm.config.Namespace {
				cm.handleConfigChange(shardConfig)
			}
		},
		DeleteFunc: func(obj interface{}) {
			shardConfig := obj.(*shardv1.ShardConfig)
			if shardConfig.Name == cm.shardConfigName && shardConfig.Namespace == cm.config.Namespace {
				// Config was deleted, revert to defaults
				defaultConfig := cm.createDefaultShardConfig()
				cm.handleConfigChange(defaultConfig)
			}
		},
	})

	return nil
}

// startConfigMapWatch starts watching for ConfigMap changes
func (cm *ConfigManager) startConfigMapWatch(ctx context.Context) error {
	if cm.manager == nil {
		return fmt.Errorf("manager is not set, cannot start ConfigMap watch")
	}

	// Cancel any existing ConfigMap watch
	if cm.configMapCancel != nil {
		cm.configMapCancel()
	}

	// Create a new watch context
	watchCtx, cancel := context.WithCancel(ctx)
	cm.configMapWatchCtx = watchCtx
	cm.configMapCancel = cancel

	// Create a new informer for ConfigMap
	informer, err := cm.manager.GetCache().GetInformer(ctx, &corev1.ConfigMap{})
	if err != nil {
		return fmt.Errorf("failed to get informer for ConfigMap: %w", err)
	}

	// Add an event handler to the informer
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			configMap := obj.(*corev1.ConfigMap)
			if configMap.Name == cm.configMapName && configMap.Namespace == cm.config.Namespace {
				cm.handleConfigMapChange(configMap)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			configMap := newObj.(*corev1.ConfigMap)
			if configMap.Name == cm.configMapName && configMap.Namespace == cm.config.Namespace {
				cm.handleConfigMapChange(configMap)
			}
		},
		DeleteFunc: func(obj interface{}) {
			configMap := obj.(*corev1.ConfigMap)
			if configMap.Name == cm.configMapName && configMap.Namespace == cm.config.Namespace {
				// ConfigMap was deleted, clear stored data
				cm.mu.Lock()
				cm.configMapData = make(map[string]string)
				cm.mu.Unlock()
				// Reload configuration without ConfigMap data
				cm.triggerConfigReload()
			}
		},
	})

	return nil
}

// handleConfigChange handles changes to the ShardConfig CRD
func (cm *ConfigManager) handleConfigChange(shardConfig *shardv1.ShardConfig) {
	// Validate the new configuration
	if err := cm.ValidateConfig(shardConfig); err != nil {
		fmt.Printf("Invalid ShardConfig received, ignoring: %v\n", err)
		return
	}

	// Update the current configuration
	cm.mu.Lock()
	cm.shardConfig = shardConfig.DeepCopy()
	callbacks := make([]func(*shardv1.ShardConfig), len(cm.callbacks))
	copy(callbacks, cm.callbacks)
	cm.mu.Unlock()

	// Notify all registered callbacks
	for _, callback := range callbacks {
		go func(cb func(*shardv1.ShardConfig)) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Config change callback panicked: %v\n", r)
				}
			}()
			cb(shardConfig.DeepCopy())
		}(callback)
	}
}

// handleConfigMapChange handles changes to the ConfigMap
func (cm *ConfigManager) handleConfigMapChange(configMap *corev1.ConfigMap) {
	// Store the new ConfigMap data
	cm.mu.Lock()
	cm.configMapData = make(map[string]string)
	for k, v := range configMap.Data {
		cm.configMapData[k] = v
	}
	cm.mu.Unlock()

	// Trigger a configuration reload
	cm.triggerConfigReload()
}

// triggerConfigReload triggers a configuration reload from all sources
func (cm *ConfigManager) triggerConfigReload() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := cm.ReloadConfig(ctx); err != nil {
			fmt.Printf("Failed to reload configuration: %v\n", err)
		}
	}()
}

// SetManager sets the controller manager for watching resources
func (cm *ConfigManager) SetManager(mgr manager.Manager) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.manager = mgr
}

// GetConfigMapData returns the current ConfigMap data
func (cm *ConfigManager) GetConfigMapData() map[string]string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range cm.configMapData {
		result[k] = v
	}
	return result
}

// UpdateConfigMap updates the ConfigMap with new configuration values
func (cm *ConfigManager) UpdateConfigMap(ctx context.Context, data map[string]string) error {
	configMap := &corev1.ConfigMap{}
	err := cm.client.Get(ctx, types.NamespacedName{
		Namespace: cm.config.Namespace,
		Name:      cm.configMapName,
	}, configMap)

	if err != nil {
		if errors.IsNotFound(err) {
			// Create new ConfigMap
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cm.configMapName,
					Namespace: cm.config.Namespace,
				},
				Data: data,
			}
			return cm.client.Create(ctx, configMap)
		}
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	// Update existing ConfigMap
	configMap.Data = data
	return cm.client.Update(ctx, configMap)
}

// LoadFromConfigMap loads configuration from a specific ConfigMap
func (cm *ConfigManager) LoadFromConfigMap(ctx context.Context, configMapName, namespace string) (*shardv1.ShardConfig, error) {
	configMap := &corev1.ConfigMap{}
	err := cm.client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      configMapName,
	}, configMap)

	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, configMapName, err)
	}

	// Create a new ShardConfig with defaults
	shardConfig := cm.createDefaultShardConfig()
	shardConfig.Namespace = namespace

	// Parse configuration from ConfigMap data
	if err := cm.parseConfigMapData(configMap.Data, shardConfig); err != nil {
		return nil, fmt.Errorf("failed to parse ConfigMap data: %w", err)
	}

	// Apply defaults and validate
	cm.applyDefaults(shardConfig)
	if err := cm.ValidateConfig(shardConfig); err != nil {
		return nil, fmt.Errorf("invalid configuration from ConfigMap: %w", err)
	}

	return shardConfig, nil
}

// StartWatching starts watching for configuration changes
func (cm *ConfigManager) StartWatching(ctx context.Context) error {
	return cm.StartHotReload(ctx)
}

// StopWatching stops watching for configuration changes
func (cm *ConfigManager) StopWatching() {
	cm.StopHotReload()
}

// ValidateAndApplyConfig validates and applies a new configuration
func (cm *ConfigManager) ValidateAndApplyConfig(ctx context.Context, newConfig *shardv1.ShardConfig) error {
	// Validate the new configuration
	if err := cm.ValidateConfig(newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Apply defaults
	cm.applyDefaults(newConfig)

	// Update the CRD
	existingConfig := &shardv1.ShardConfig{}
	err := cm.client.Get(ctx, types.NamespacedName{
		Namespace: cm.config.Namespace,
		Name:      cm.shardConfigName,
	}, existingConfig)

	if err != nil {
		if errors.IsNotFound(err) {
			// Create new CRD
			return cm.client.Create(ctx, newConfig)
		}
		return fmt.Errorf("failed to get existing ShardConfig: %w", err)
	}

	// Update existing CRD
	existingConfig.Spec = newConfig.Spec
	return cm.client.Update(ctx, existingConfig)
}
