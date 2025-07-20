package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	shardclient "github.com/k8s-shard-controller/pkg/client"
	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// ControllerManager manages all controllers and components
type ControllerManager struct {
	config     *config.Config
	client     client.Client
	kubeClient kubernetes.Interface
	manager    manager.Manager

	// Core components
	shardManager     interfaces.ShardManager
	loadBalancer     interfaces.LoadBalancer
	healthChecker    interfaces.HealthChecker
	resourceMigrator interfaces.ResourceMigrator
	configManager    interfaces.ConfigManager
	metricsCollector interfaces.MetricsCollector
	alertManager     interfaces.AlertManager
	structuredLogger interfaces.StructuredLogger

	// State management
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewControllerManager creates a new controller manager
func NewControllerManager(cfg *config.Config) (*ControllerManager, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Create client set
	clientSet, err := shardclient.NewClientSet(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client set: %w", err)
	}

	// Create controller-runtime manager
	mgr, err := shardclient.NewManagerWithClientSet(clientSet, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create controller manager: %w", err)
	}

	cm := &ControllerManager{
		config:     cfg,
		client:     mgr.GetClient(),
		kubeClient: clientSet.KubeClient,
		manager:    mgr,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	return cm, nil
}

// Start starts the controller manager
func (cm *ControllerManager) Start(ctx context.Context) error {
	cm.mu.Lock()
	if cm.running {
		cm.mu.Unlock()
		return fmt.Errorf("controller manager is already running")
	}
	cm.running = true
	cm.mu.Unlock()

	// Initialize components
	if err := cm.initializeComponents(); err != nil {
		return fmt.Errorf("failed to initialize components: %w", err)
	}

	// Start the manager
	go func() {
		defer close(cm.doneCh)
		if err := cm.manager.Start(ctx); err != nil {
			// Log error but don't panic
			fmt.Printf("Controller manager stopped with error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the controller manager
func (cm *ControllerManager) Stop() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.running {
		return nil
	}

	close(cm.stopCh)
	cm.running = false

	// Wait for manager to stop
	select {
	case <-cm.doneCh:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for controller manager to stop")
	}
}

// GetClient returns the Kubernetes client
func (cm *ControllerManager) GetClient() client.Client {
	return cm.client
}

// GetKubeClient returns the Kubernetes clientset
func (cm *ControllerManager) GetKubeClient() kubernetes.Interface {
	return cm.kubeClient
}

// GetShardManager returns the shard manager
func (cm *ControllerManager) GetShardManager() interfaces.ShardManager {
	return cm.shardManager
}

// GetLoadBalancer returns the load balancer
func (cm *ControllerManager) GetLoadBalancer() interfaces.LoadBalancer {
	return cm.loadBalancer
}

// GetHealthChecker returns the health checker
func (cm *ControllerManager) GetHealthChecker() interfaces.HealthChecker {
	return cm.healthChecker
}

// GetResourceMigrator returns the resource migrator
func (cm *ControllerManager) GetResourceMigrator() interfaces.ResourceMigrator {
	return cm.resourceMigrator
}

// GetConfigManager returns the config manager
func (cm *ControllerManager) GetConfigManager() interfaces.ConfigManager {
	return cm.configManager
}

// GetMetricsCollector returns the metrics collector
func (cm *ControllerManager) GetMetricsCollector() interfaces.MetricsCollector {
	return cm.metricsCollector
}

// GetAlertManager returns the alert manager
func (cm *ControllerManager) GetAlertManager() interfaces.AlertManager {
	return cm.alertManager
}

// GetStructuredLogger returns the structured logger
func (cm *ControllerManager) GetStructuredLogger() interfaces.StructuredLogger {
	return cm.structuredLogger
}

// initializeComponents initializes all components
func (cm *ControllerManager) initializeComponents() error {
	// Validate configuration first
	if err := cm.validateConfiguration(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Initialize config manager
	configManager, err := NewConfigManager(cm.client, cm.config)
	if err != nil {
		return fmt.Errorf("failed to initialize config manager: %w", err)
	}

	// Set the manager reference for watching resources
	if cm.manager != nil {
		configManager.SetManager(cm.manager)
	}

	cm.configManager = configManager

	// Initialize load balancer
	loadBalancer, err := NewLoadBalancer(cm.config.DefaultShardConfig.LoadBalanceStrategy, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize load balancer: %w", err)
	}
	cm.loadBalancer = loadBalancer

	// Initialize structured logger
	structuredLogger, err := NewStructuredLogger(*cm.config)
	if err != nil {
		return fmt.Errorf("failed to initialize structured logger: %w", err)
	}
	cm.structuredLogger = structuredLogger

	// Initialize health checker
	healthChecker, err := NewHealthChecker(cm.client, cm.config.HealthCheck)
	if err != nil {
		return fmt.Errorf("failed to initialize health checker: %w", err)
	}
	cm.healthChecker = healthChecker

	// Initialize alert manager
	alertManager := NewAlertManager(AlertingConfig{
		Enabled:    cm.config.Alerting.Enabled,
		WebhookURL: cm.config.Alerting.WebhookURL,
		Timeout:    cm.config.Alerting.Timeout,
		RetryCount: cm.config.Alerting.RetryCount,
		RetryDelay: cm.config.Alerting.RetryDelay,
	}, structuredLogger.GetLogger())
	cm.alertManager = alertManager

	// Initialize metrics collector
	metricsCollector, err := NewMetricsCollector(cm.config.Metrics, structuredLogger.GetLogger())
	if err != nil {
		return fmt.Errorf("failed to initialize metrics collector: %w", err)
	}
	cm.metricsCollector = metricsCollector

	// Initialize shard manager first (without resource migrator)
	shardManagerImpl, err := NewShardManager(
		cm.client,
		cm.kubeClient,
		cm.loadBalancer,
		cm.healthChecker,
		nil, // resource migrator will be set later
		cm.configManager,
		cm.metricsCollector,
		cm.alertManager,
		cm.structuredLogger,
		cm.config,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize shard manager: %w", err)
	}
	cm.shardManager = shardManagerImpl

	// Initialize resource migrator (depends on shard manager)
	resourceMigrator := NewResourceMigrator(cm.shardManager)
	cm.resourceMigrator = resourceMigrator

	// Set the resource migrator in the shard manager
	shardManagerImpl.SetResourceMigrator(resourceMigrator)

	// Start hot reload for configuration changes
	ctx := context.Background()
	if err := cm.configManager.(*ConfigManager).StartHotReload(ctx); err != nil {
		// Log warning but don't fail initialization
		fmt.Printf("Warning: failed to start config hot reload: %v\n", err)
	}

	// Start metrics server
	go func() {
		if err := cm.metricsCollector.StartMetricsServer(context.Background()); err != nil {
			fmt.Printf("Warning: failed to start metrics server: %v\n", err)
		}
	}()

	return nil
}

// validateConfiguration validates the controller configuration
func (cm *ControllerManager) validateConfiguration() error {
	if cm.config.DefaultShardConfig.MinShards <= 0 {
		return fmt.Errorf("minShards must be greater than 0")
	}

	if cm.config.DefaultShardConfig.MaxShards < cm.config.DefaultShardConfig.MinShards {
		return fmt.Errorf("maxShards must be greater than or equal to minShards")
	}

	if cm.config.DefaultShardConfig.ScaleUpThreshold <= 0 || cm.config.DefaultShardConfig.ScaleUpThreshold > 1 {
		return fmt.Errorf("scaleUpThreshold must be between 0 and 1")
	}

	if cm.config.DefaultShardConfig.ScaleDownThreshold <= 0 || cm.config.DefaultShardConfig.ScaleDownThreshold > 1 {
		return fmt.Errorf("scaleDownThreshold must be between 0 and 1")
	}

	if cm.config.DefaultShardConfig.ScaleDownThreshold >= cm.config.DefaultShardConfig.ScaleUpThreshold {
		return fmt.Errorf("scaleDownThreshold must be less than scaleUpThreshold")
	}

	return nil
}

// IsRunning returns whether the controller manager is running
func (cm *ControllerManager) IsRunning() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.running
}
