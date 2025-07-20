package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockShardManager implements interfaces.ShardManager for testing
type MockShardManager struct {
	client       client.Client
	shards       map[string]*shardv1.ShardInstance
	shardCounter int
	mu           sync.RWMutex
}

func NewMockShardManager(client client.Client) *MockShardManager {
	return &MockShardManager{
		client: client,
		shards: make(map[string]*shardv1.ShardInstance),
	}
}

func (m *MockShardManager) CreateShard(ctx context.Context, config *shardv1.ShardConfigSpec) (*shardv1.ShardInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.shardCounter++
	shardID := fmt.Sprintf("shard-%d", m.shardCounter)
	
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shardID,
			Namespace: "default",
		},
		Spec: shardv1.ShardInstanceSpec{
			ShardID: shardID,
			HashRange: &shardv1.HashRange{
				Start: uint32(m.shardCounter * 1000),
				End:   uint32((m.shardCounter + 1) * 1000),
			},
		},
		Status: shardv1.ShardInstanceStatus{
			Phase:         shardv1.ShardPhasePending,
			LastHeartbeat: metav1.Now(),
			HealthStatus:  &shardv1.HealthStatus{Healthy: true},
			Load:          0.0,
		},
	}
	
	err := m.client.Create(ctx, shard)
	if err != nil {
		return nil, err
	}
	
	m.shards[shardID] = shard
	return shard, nil
}

func (m *MockShardManager) DeleteShard(ctx context.Context, shardId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	shard, exists := m.shards[shardId]
	if !exists {
		return fmt.Errorf("shard %s not found", shardId)
	}
	
	err := m.client.Delete(ctx, shard)
	if err != nil {
		return err
	}
	
	delete(m.shards, shardId)
	return nil
}

func (m *MockShardManager) ScaleUp(ctx context.Context, targetCount int) error {
	m.mu.RLock()
	currentCount := len(m.shards)
	m.mu.RUnlock()
	
	for i := currentCount; i < targetCount; i++ {
		_, err := m.CreateShard(ctx, &shardv1.ShardConfigSpec{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *MockShardManager) ScaleDown(ctx context.Context, targetCount int) error {
	m.mu.RLock()
	currentShards := make([]*shardv1.ShardInstance, 0, len(m.shards))
	for _, shard := range m.shards {
		currentShards = append(currentShards, shard)
	}
	m.mu.RUnlock()
	
	if len(currentShards) <= targetCount {
		return nil
	}
	
	// Mark excess shards for draining
	for i := targetCount; i < len(currentShards); i++ {
		shard := currentShards[i]
		shard.Status.Phase = shardv1.ShardPhaseDraining
		err := m.client.Status().Update(ctx, shard)
		if err != nil {
			return err
		}
	}
	
	// Simulate gradual shutdown
	go func() {
		time.Sleep(2 * time.Second)
		for i := targetCount; i < len(currentShards); i++ {
			shard := currentShards[i]
			m.DeleteShard(context.Background(), shard.Spec.ShardID)
		}
	}()
	
	return nil
}

func (m *MockShardManager) RebalanceLoad(ctx context.Context) error {
	// Mock implementation
	return nil
}

func (m *MockShardManager) AssignResource(ctx context.Context, resource *interfaces.Resource) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for shardID := range m.shards {
		return shardID, nil
	}
	return "", fmt.Errorf("no shards available")
}

func (m *MockShardManager) CheckShardHealth(ctx context.Context, shardId string) (*shardv1.HealthStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	shard, exists := m.shards[shardId]
	if !exists {
		return nil, fmt.Errorf("shard %s not found", shardId)
	}
	
	return shard.Status.HealthStatus, nil
}

func (m *MockShardManager) HandleFailedShard(ctx context.Context, shardId string) error {
	// Mock implementation - just mark as failed
	m.mu.Lock()
	defer m.mu.Unlock()
	
	shard, exists := m.shards[shardId]
	if !exists {
		return fmt.Errorf("shard %s not found", shardId)
	}
	
	shard.Status.Phase = shardv1.ShardPhaseFailed
	return m.client.Status().Update(ctx, shard)
}

func (m *MockShardManager) GetShardStatus(ctx context.Context, shardId string) (*shardv1.ShardInstanceStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	shard, exists := m.shards[shardId]
	if !exists {
		return nil, fmt.Errorf("shard %s not found", shardId)
	}
	
	return &shard.Status, nil
}

func (m *MockShardManager) ListShards(ctx context.Context) ([]*shardv1.ShardInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	shards := make([]*shardv1.ShardInstance, 0, len(m.shards))
	for _, shard := range m.shards {
		shards = append(shards, shard)
	}
	return shards, nil
}

// MockLoadBalancer implements interfaces.LoadBalancer for testing
type MockLoadBalancer struct {
	strategy shardv1.LoadBalanceStrategy
	shards   []*shardv1.ShardInstance
}

func NewMockLoadBalancer() *MockLoadBalancer {
	return &MockLoadBalancer{
		strategy: shardv1.LeastLoadedStrategy,
	}
}

func (m *MockLoadBalancer) CalculateShardLoad(shard *shardv1.ShardInstance) float64 {
	return shard.Status.Load
}

func (m *MockLoadBalancer) GetOptimalShard(shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards available")
	}
	
	var optimal *shardv1.ShardInstance
	minLoad := float64(1.0)
	
	for _, shard := range shards {
		if shard.Status.Phase == shardv1.ShardPhaseRunning && shard.Status.Load < minLoad {
			optimal = shard
			minLoad = shard.Status.Load
		}
	}
	
	if optimal == nil {
		return shards[0], nil // Fallback to first shard
	}
	
	return optimal, nil
}

func (m *MockLoadBalancer) SetStrategy(strategy shardv1.LoadBalanceStrategy, shards []*shardv1.ShardInstance) {
	m.strategy = strategy
	m.shards = shards
}

func (m *MockLoadBalancer) ShouldRebalance(shards []*shardv1.ShardInstance) bool {
	if len(shards) < 2 {
		return false
	}
	
	var maxLoad, minLoad float64
	for i, shard := range shards {
		if i == 0 {
			maxLoad = shard.Status.Load
			minLoad = shard.Status.Load
		} else {
			if shard.Status.Load > maxLoad {
				maxLoad = shard.Status.Load
			}
			if shard.Status.Load < minLoad {
				minLoad = shard.Status.Load
			}
		}
	}
	
	return (maxLoad - minLoad) > 0.2 // 20% threshold
}

func (m *MockLoadBalancer) GenerateRebalancePlan(shards []*shardv1.ShardInstance) (*shardv1.MigrationPlan, error) {
	if len(shards) < 2 {
		return nil, fmt.Errorf("not enough shards for rebalancing")
	}
	
	var sourceShard, targetShard *shardv1.ShardInstance
	maxLoad := float64(0)
	minLoad := float64(1)
	
	for _, shard := range shards {
		if shard.Status.Load > maxLoad {
			maxLoad = shard.Status.Load
			sourceShard = shard
		}
		if shard.Status.Load < minLoad {
			minLoad = shard.Status.Load
			targetShard = shard
		}
	}
	
	return &shardv1.MigrationPlan{
		SourceShard:   sourceShard.Spec.ShardID,
		TargetShard:   targetShard.Spec.ShardID,
		Resources:     []string{"resource-1"}, // Mock resource
		EstimatedTime: metav1.Duration{Duration: 30 * time.Second},
		Priority:      shardv1.MigrationPriorityMedium,
	}, nil
}

func (m *MockLoadBalancer) AssignResourceToShard(resource *interfaces.Resource, shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	return m.GetOptimalShard(shards)
}

// MockHealthChecker implements interfaces.HealthChecker for testing
type MockHealthChecker struct {
	client         client.Client
	healthStatus   map[string]*shardv1.HealthStatus
	unhealthyShards map[string]bool
	mu             sync.RWMutex
	stopCh         chan struct{}
}

func NewMockHealthChecker(client client.Client) *MockHealthChecker {
	return &MockHealthChecker{
		client:          client,
		healthStatus:    make(map[string]*shardv1.HealthStatus),
		unhealthyShards: make(map[string]bool),
	}
}

func (m *MockHealthChecker) CheckHealth(ctx context.Context, shard *shardv1.ShardInstance) (*shardv1.HealthStatus, error) {
	return m.CheckShardHealth(ctx, shard.Spec.ShardID)
}

func (m *MockHealthChecker) CheckShardHealth(ctx context.Context, shardId string) (*shardv1.HealthStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if status, exists := m.healthStatus[shardId]; exists {
		return status, nil
	}
	
	return &shardv1.HealthStatus{
		Healthy:   true,
		LastCheck: metav1.Now(),
	}, nil
}

func (m *MockHealthChecker) StartHealthChecking(ctx context.Context, interval time.Duration) error {
	m.stopCh = make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				m.performHealthCheck(ctx)
			case <-m.stopCh:
				return
			}
		}
	}()
	
	return nil
}

func (m *MockHealthChecker) StopHealthChecking() error {
	if m.stopCh != nil {
		close(m.stopCh)
	}
	return nil
}

func (m *MockHealthChecker) IsShardHealthy(shardId string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return !m.unhealthyShards[shardId]
}

func (m *MockHealthChecker) GetUnhealthyShards() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var unhealthy []string
	for shardId := range m.unhealthyShards {
		if m.unhealthyShards[shardId] {
			unhealthy = append(unhealthy, shardId)
		}
	}
	return unhealthy
}

func (m *MockHealthChecker) GetHealthSummary() map[string]*shardv1.HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	summary := make(map[string]*shardv1.HealthStatus)
	for shardId, status := range m.healthStatus {
		summary[shardId] = status
	}
	return summary
}

func (m *MockHealthChecker) OnShardFailed(ctx context.Context, shardId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.unhealthyShards[shardId] = true
	m.healthStatus[shardId] = &shardv1.HealthStatus{
		Healthy:    false,
		LastCheck:  metav1.Now(),
		ErrorCount: 1,
		Message:    "Shard failed",
	}
	return nil
}

func (m *MockHealthChecker) OnShardRecovered(ctx context.Context, shardId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.unhealthyShards[shardId] = false
	m.healthStatus[shardId] = &shardv1.HealthStatus{
		Healthy:   true,
		LastCheck: metav1.Now(),
		Message:   "Shard recovered",
	}
	return nil
}

func (m *MockHealthChecker) performHealthCheck(ctx context.Context) {
	// Get all shards from cluster
	shardList := &shardv1.ShardInstanceList{}
	err := m.client.List(ctx, shardList)
	if err != nil {
		return
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, shard := range shardList.Items {
		// Check if shard is healthy based on heartbeat
		lastHeartbeat := shard.Status.LastHeartbeat.Time
		if time.Since(lastHeartbeat) > 5*time.Minute {
			m.unhealthyShards[shard.Spec.ShardID] = true
			m.healthStatus[shard.Spec.ShardID] = &shardv1.HealthStatus{
				Healthy:    false,
				LastCheck:  metav1.Now(),
				ErrorCount: 1,
				Message:    "Heartbeat timeout",
			}
		} else {
			m.unhealthyShards[shard.Spec.ShardID] = false
			m.healthStatus[shard.Spec.ShardID] = &shardv1.HealthStatus{
				Healthy:   true,
				LastCheck: metav1.Now(),
			}
		}
	}
}

// MockResourceMigrator implements interfaces.ResourceMigrator for testing
type MockResourceMigrator struct {
	client client.Client
}

func NewMockResourceMigrator(client client.Client) *MockResourceMigrator {
	return &MockResourceMigrator{client: client}
}

func (m *MockResourceMigrator) CreateMigrationPlan(ctx context.Context, sourceShard, targetShard string, resources []*interfaces.Resource) (*shardv1.MigrationPlan, error) {
	resourceIds := make([]string, len(resources))
	for i, resource := range resources {
		resourceIds[i] = resource.ID
	}
	
	return &shardv1.MigrationPlan{
		SourceShard:   sourceShard,
		TargetShard:   targetShard,
		Resources:     resourceIds,
		EstimatedTime: metav1.Duration{Duration: 30 * time.Second},
		Priority:      shardv1.MigrationPriorityMedium,
	}, nil
}

func (m *MockResourceMigrator) ExecuteMigration(ctx context.Context, plan *shardv1.MigrationPlan) error {
	// Mock migration by updating shard resource assignments
	sourceShard := &shardv1.ShardInstance{}
	err := m.client.Get(ctx, client.ObjectKey{
		Name:      plan.SourceShard,
		Namespace: "default",
	}, sourceShard)
	if err != nil {
		return err
	}
	
	targetShard := &shardv1.ShardInstance{}
	err = m.client.Get(ctx, client.ObjectKey{
		Name:      plan.TargetShard,
		Namespace: "default",
	}, targetShard)
	if err != nil {
		return err
	}
	
	// Remove resources from source
	newSourceResources := []string{}
	for _, resource := range sourceShard.Status.AssignedResources {
		found := false
		for _, migrated := range plan.Resources {
			if resource == migrated {
				found = true
				break
			}
		}
		if !found {
			newSourceResources = append(newSourceResources, resource)
		}
	}
	sourceShard.Status.AssignedResources = newSourceResources
	
	// Add resources to target
	targetShard.Status.AssignedResources = append(targetShard.Status.AssignedResources, plan.Resources...)
	
	// Update both shards
	err = m.client.Status().Update(ctx, sourceShard)
	if err != nil {
		return err
	}
	
	return m.client.Status().Update(ctx, targetShard)
}

func (m *MockResourceMigrator) GetMigrationStatus(ctx context.Context, planId string) (interfaces.MigrationStatus, error) {
	return interfaces.MigrationStatusCompleted, nil
}

func (m *MockResourceMigrator) RollbackMigration(ctx context.Context, planId string) error {
	return nil
}

// MockConfigManager implements interfaces.ConfigManager for testing
type MockConfigManager struct {
	currentConfig *shardv1.ShardConfig
	mu            sync.RWMutex
	stopCh        chan struct{}
}

func NewMockConfigManager() *MockConfigManager {
	return &MockConfigManager{
		currentConfig: &shardv1.ShardConfig{
			Spec: shardv1.ShardConfigSpec{
				MinShards:               2,
				MaxShards:               10,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 60 * time.Second},
			},
		},
	}
}

func (m *MockConfigManager) LoadConfig(ctx context.Context) (*shardv1.ShardConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentConfig, nil
}

func (m *MockConfigManager) LoadFromConfigMap(ctx context.Context, configMapName, namespace string) (*shardv1.ShardConfig, error) {
	return m.LoadConfig(ctx)
}

func (m *MockConfigManager) ReloadConfig(ctx context.Context) error {
	return nil
}

func (m *MockConfigManager) GetCurrentConfig() *shardv1.ShardConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentConfig
}

func (m *MockConfigManager) ValidateConfig(config *shardv1.ShardConfig) error {
	if config.Spec.MinShards <= 0 {
		return fmt.Errorf("minShards must be positive")
	}
	if config.Spec.MaxShards < config.Spec.MinShards {
		return fmt.Errorf("maxShards must be >= minShards")
	}
	return nil
}

func (m *MockConfigManager) WatchConfigChanges(ctx context.Context, callback func(*shardv1.ShardConfig)) error {
	return nil
}

func (m *MockConfigManager) StartWatching(ctx context.Context) error {
	return nil
}

func (m *MockConfigManager) StopWatching() {
	if m.stopCh != nil {
		close(m.stopCh)
	}
}

// MockMetricsCollector implements interfaces.MetricsCollector for testing
type MockMetricsCollector struct{}

func NewMockMetricsCollector() *MockMetricsCollector {
	return &MockMetricsCollector{}
}

func (m *MockMetricsCollector) CollectShardMetrics(shard *shardv1.ShardInstance) error {
	return nil
}

func (m *MockMetricsCollector) CollectSystemMetrics(shards []*shardv1.ShardInstance) error {
	return nil
}

func (m *MockMetricsCollector) ExposeMetrics() error {
	return nil
}

func (m *MockMetricsCollector) StartMetricsServer(ctx context.Context) error {
	return nil
}

func (m *MockMetricsCollector) RecordMigration(sourceShard, targetShard, status string, duration time.Duration) {
}

func (m *MockMetricsCollector) RecordScaleOperation(operation, status string) {
}

func (m *MockMetricsCollector) RecordError(component, errorType string) {
}

func (m *MockMetricsCollector) RecordOperationDuration(operation, component string, duration time.Duration) {
}

func (m *MockMetricsCollector) UpdateQueueLength(queueType, shardID string, length int) {
}

func (m *MockMetricsCollector) RecordCustomMetric(name string, value float64, labels map[string]string) error {
	return nil
}

// MockAlerting implements interfaces.Alerting for testing
type MockAlerting struct{}

func NewMockAlerting() *MockAlerting {
	return &MockAlerting{}
}

func (m *MockAlerting) SendAlert(ctx context.Context, alert interfaces.Alert) error {
	return nil
}

func (m *MockAlerting) ConfigureWebhook(webhookURL string) error {
	return nil
}

func (m *MockAlerting) AlertShardFailure(ctx context.Context, shardID string, reason string) error {
	return nil
}

func (m *MockAlerting) AlertHighErrorRate(ctx context.Context, component string, errorRate float64, threshold float64) error {
	return nil
}

func (m *MockAlerting) AlertScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string) error {
	return nil
}

func (m *MockAlerting) AlertMigrationFailure(ctx context.Context, sourceShard, targetShard string, resourceCount int, reason string) error {
	return nil
}

func (m *MockAlerting) AlertSystemOverload(ctx context.Context, totalLoad float64, threshold float64, shardCount int) error {
	return nil
}

func (m *MockAlerting) AlertConfigurationChange(ctx context.Context, configType string, changes map[string]interface{}) error {
	return nil
}

// MockLogger implements interfaces.Logger for testing
type MockLogger struct{}

func NewMockLogger() *MockLogger {
	return &MockLogger{}
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
}

func (m *MockLogger) Error(msg string, err error, fields map[string]interface{}) {
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
}

func (m *MockLogger) LogShardEvent(ctx context.Context, event string, shardID string, fields map[string]interface{}) {
}

func (m *MockLogger) LogMigrationEvent(ctx context.Context, sourceShard, targetShard string, resourceCount int, status string, duration time.Duration) {
}

func (m *MockLogger) LogScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string, status string) {
}

func (m *MockLogger) LogHealthEvent(ctx context.Context, shardID string, healthy bool, errorCount int, message string) {
}

func (m *MockLogger) LogErrorEvent(ctx context.Context, component, operation string, err error, fields map[string]interface{}) {
}

func (m *MockLogger) LogPerformanceEvent(ctx context.Context, operation, component string, duration time.Duration, success bool) {
}

func (m *MockLogger) LogConfigEvent(ctx context.Context, configType string, changes map[string]interface{}) {
}

func (m *MockLogger) LogSystemEvent(ctx context.Context, event string, severity string, fields map[string]interface{}) {
}