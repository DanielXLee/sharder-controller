package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// Mock implementations for testing
type MockLoadBalancer struct {
	mock.Mock
}

func (m *MockLoadBalancer) CalculateShardLoad(shard *shardv1.ShardInstance) float64 {
	args := m.Called(shard)
	return args.Get(0).(float64)
}

func (m *MockLoadBalancer) GetOptimalShard(shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	args := m.Called(shards)
	return args.Get(0).(*shardv1.ShardInstance), args.Error(1)
}

func (m *MockLoadBalancer) ShouldRebalance(shards []*shardv1.ShardInstance) bool {
	args := m.Called(shards)
	return args.Bool(0)
}

func (m *MockLoadBalancer) GenerateRebalancePlan(shards []*shardv1.ShardInstance) (*shardv1.MigrationPlan, error) {
	args := m.Called(shards)
	return args.Get(0).(*shardv1.MigrationPlan), args.Error(1)
}

func (m *MockLoadBalancer) AssignResourceToShard(resource *interfaces.Resource, shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	args := m.Called(resource, shards)
	return args.Get(0).(*shardv1.ShardInstance), args.Error(1)
}

func (m *MockLoadBalancer) SetStrategy(strategy shardv1.LoadBalanceStrategy, shards []*shardv1.ShardInstance) {
	m.Called(strategy, shards)
}

type MockHealthChecker struct {
	mock.Mock
}

func (m *MockHealthChecker) CheckHealth(ctx context.Context, shard *shardv1.ShardInstance) (*shardv1.HealthStatus, error) {
	args := m.Called(ctx, shard)
	return args.Get(0).(*shardv1.HealthStatus), args.Error(1)
}

func (m *MockHealthChecker) StartHealthChecking(ctx context.Context, interval time.Duration) error {
	args := m.Called(ctx, interval)
	return args.Error(0)
}

func (m *MockHealthChecker) StopHealthChecking() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockHealthChecker) OnShardFailed(ctx context.Context, shardId string) error {
	args := m.Called(ctx, shardId)
	return args.Error(0)
}

func (m *MockHealthChecker) OnShardRecovered(ctx context.Context, shardId string) error {
	args := m.Called(ctx, shardId)
	return args.Error(0)
}

func (m *MockHealthChecker) CheckShardHealth(ctx context.Context, shardId string) (*shardv1.HealthStatus, error) {
	args := m.Called(ctx, shardId)
	return args.Get(0).(*shardv1.HealthStatus), args.Error(1)
}

func (m *MockHealthChecker) IsShardHealthy(shardId string) bool {
	args := m.Called(shardId)
	return args.Bool(0)
}

func (m *MockHealthChecker) GetUnhealthyShards() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockHealthChecker) GetHealthSummary() map[string]*shardv1.HealthStatus {
	args := m.Called()
	return args.Get(0).(map[string]*shardv1.HealthStatus)
}

type MockResourceMigrator struct {
	mock.Mock
}

func (m *MockResourceMigrator) CreateMigrationPlan(ctx context.Context, sourceShard, targetShard string, resources []*interfaces.Resource) (*shardv1.MigrationPlan, error) {
	args := m.Called(ctx, sourceShard, targetShard, resources)
	return args.Get(0).(*shardv1.MigrationPlan), args.Error(1)
}

func (m *MockResourceMigrator) ExecuteMigration(ctx context.Context, plan *shardv1.MigrationPlan) error {
	args := m.Called(ctx, plan)
	return args.Error(0)
}

func (m *MockResourceMigrator) GetMigrationStatus(ctx context.Context, planId string) (interfaces.MigrationStatus, error) {
	args := m.Called(ctx, planId)
	return args.Get(0).(interfaces.MigrationStatus), args.Error(1)
}

func (m *MockResourceMigrator) RollbackMigration(ctx context.Context, planId string) error {
	args := m.Called(ctx, planId)
	return args.Error(0)
}

type MockConfigManager struct {
	mock.Mock
}

func (m *MockConfigManager) LoadConfig(ctx context.Context) (*shardv1.ShardConfig, error) {
	args := m.Called(ctx)
	return args.Get(0).(*shardv1.ShardConfig), args.Error(1)
}

func (m *MockConfigManager) ReloadConfig(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockConfigManager) ValidateConfig(config *shardv1.ShardConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockConfigManager) WatchConfigChanges(ctx context.Context, callback func(*shardv1.ShardConfig)) error {
	args := m.Called(ctx, callback)
	return args.Error(0)
}

func (m *MockConfigManager) GetCurrentConfig() *shardv1.ShardConfig {
	args := m.Called()
	return args.Get(0).(*shardv1.ShardConfig)
}

func (m *MockConfigManager) LoadFromConfigMap(ctx context.Context, configMapName, namespace string) (*shardv1.ShardConfig, error) {
	args := m.Called(ctx, configMapName, namespace)
	return args.Get(0).(*shardv1.ShardConfig), args.Error(1)
}

func (m *MockConfigManager) StartWatching(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockConfigManager) StopWatching() {
	m.Called()
}

type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) CollectShardMetrics(shard *shardv1.ShardInstance) error {
	args := m.Called(shard)
	return args.Error(0)
}

func (m *MockMetricsCollector) CollectSystemMetrics(shards []*shardv1.ShardInstance) error {
	args := m.Called(shards)
	return args.Error(0)
}

func (m *MockMetricsCollector) ExposeMetrics() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMetricsCollector) RecordCustomMetric(name string, value float64, labels map[string]string) error {
	args := m.Called(name, value, labels)
	return args.Error(0)
}

func (m *MockMetricsCollector) StartMetricsServer(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMetricsCollector) RecordMigration(sourceShard, targetShard, status string, duration time.Duration) {
	m.Called(sourceShard, targetShard, status, duration)
}

func (m *MockMetricsCollector) RecordScaleOperation(operation, status string) {
	m.Called(operation, status)
}

func (m *MockMetricsCollector) RecordError(component, errorType string) {
	m.Called(component, errorType)
}

func (m *MockMetricsCollector) RecordOperationDuration(operation, component string, duration time.Duration) {
	m.Called(operation, component, duration)
}

func (m *MockMetricsCollector) UpdateQueueLength(queueType, shardID string, length int) {
	m.Called(queueType, shardID, length)
}

type MockAlertManager struct {
	mock.Mock
}

func (m *MockAlertManager) SendAlert(ctx context.Context, alert interfaces.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

func (m *MockAlertManager) AlertShardFailure(ctx context.Context, shardID string, reason string) error {
	args := m.Called(ctx, shardID, reason)
	return args.Error(0)
}

func (m *MockAlertManager) AlertHighErrorRate(ctx context.Context, component string, errorRate float64, threshold float64) error {
	args := m.Called(ctx, component, errorRate, threshold)
	return args.Error(0)
}

func (m *MockAlertManager) AlertScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string) error {
	args := m.Called(ctx, operation, fromCount, toCount, reason)
	return args.Error(0)
}

func (m *MockAlertManager) AlertMigrationFailure(ctx context.Context, sourceShard, targetShard string, resourceCount int, reason string) error {
	args := m.Called(ctx, sourceShard, targetShard, resourceCount, reason)
	return args.Error(0)
}

func (m *MockAlertManager) AlertSystemOverload(ctx context.Context, totalLoad float64, threshold float64, shardCount int) error {
	args := m.Called(ctx, totalLoad, threshold, shardCount)
	return args.Error(0)
}

func (m *MockAlertManager) AlertConfigurationChange(ctx context.Context, configType string, changes map[string]interface{}) error {
	args := m.Called(ctx, configType, changes)
	return args.Error(0)
}

type MockStructuredLogger struct {
	mock.Mock
}

func (m *MockStructuredLogger) LogShardEvent(ctx context.Context, event string, shardID string, fields map[string]interface{}) {
	m.Called(ctx, event, shardID, fields)
}

func (m *MockStructuredLogger) LogMigrationEvent(ctx context.Context, sourceShard, targetShard string, resourceCount int, status string, duration time.Duration) {
	m.Called(ctx, sourceShard, targetShard, resourceCount, status, duration)
}

func (m *MockStructuredLogger) LogScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string, status string) {
	m.Called(ctx, operation, fromCount, toCount, reason, status)
}

func (m *MockStructuredLogger) LogHealthEvent(ctx context.Context, shardID string, healthy bool, errorCount int, message string) {
	m.Called(ctx, shardID, healthy, errorCount, message)
}

func (m *MockStructuredLogger) LogErrorEvent(ctx context.Context, component, operation string, err error, fields map[string]interface{}) {
	m.Called(ctx, component, operation, err, fields)
}

func (m *MockStructuredLogger) LogPerformanceEvent(ctx context.Context, operation, component string, duration time.Duration, success bool) {
	m.Called(ctx, operation, component, duration, success)
}

func (m *MockStructuredLogger) LogConfigEvent(ctx context.Context, configType string, changes map[string]interface{}) {
	m.Called(ctx, configType, changes)
}

func (m *MockStructuredLogger) LogSystemEvent(ctx context.Context, event string, severity string, fields map[string]interface{}) {
	m.Called(ctx, event, severity, fields)
}

// Test helper functions
func createTestShardManager(t *testing.T) (*ShardManager, *MockLoadBalancer, *MockHealthChecker, *MockResourceMigrator, *MockConfigManager, *MockMetricsCollector, client.Client) {
	// Create fake clients
	scheme := runtime.NewScheme()
	require.NoError(t, shardv1.AddToScheme(scheme))
	
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := fake.NewSimpleClientset()

	// Create mocks
	mockLoadBalancer := &MockLoadBalancer{}
	mockHealthChecker := &MockHealthChecker{}
	mockResourceMigrator := &MockResourceMigrator{}
	mockConfigManager := &MockConfigManager{}
	mockMetricsCollector := &MockMetricsCollector{}
	mockAlertManager := &MockAlertManager{}
	mockStructuredLogger := &MockStructuredLogger{}

	// Create test config
	cfg := &config.Config{
		Namespace: "test-namespace",
		NodeName:  "test-node",
		DefaultShardConfig: config.ShardConfig{
			MinShards:          1,
			MaxShards:          10,
			ScaleUpThreshold:   0.8,
			ScaleDownThreshold: 0.2,
		},
	}

	// Create shard manager
	sm, err := NewShardManager(
		fakeClient,
		fakeKubeClient,
		mockLoadBalancer,
		mockHealthChecker,
		mockResourceMigrator,
		mockConfigManager,
		mockMetricsCollector,
		mockAlertManager,
		mockStructuredLogger,
		cfg,
	)
	require.NoError(t, err)

	return sm, mockLoadBalancer, mockHealthChecker, mockResourceMigrator, mockConfigManager, mockMetricsCollector, fakeClient
}

func createTestShardInstance(shardId string, phase shardv1.ShardPhase, load float64, healthy bool) *shardv1.ShardInstance {
	return &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shardId,
			Namespace: "test-namespace",
		},
		Spec: shardv1.ShardInstanceSpec{
			ShardID: shardId,
			HashRange: &shardv1.HashRange{
				Start: 0,
				End:   1000,
			},
		},
		Status: shardv1.ShardInstanceStatus{
			Phase:         phase,
			Load:          load,
			LastHeartbeat: metav1.Now(),
			HealthStatus: &shardv1.HealthStatus{
				Healthy:   healthy,
				LastCheck: metav1.Now(),
			},
		},
	}
}

// Test cases
func TestNewShardManager(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func() (client.Client, *MockLoadBalancer, *MockHealthChecker, *MockResourceMigrator, *MockConfigManager, *MockMetricsCollector, *config.Config)
		expectError bool
	}{
		{
			name: "successful creation",
			setupMocks: func() (client.Client, *MockLoadBalancer, *MockHealthChecker, *MockResourceMigrator, *MockConfigManager, *MockMetricsCollector, *config.Config) {
				scheme := runtime.NewScheme()
				shardv1.AddToScheme(scheme)
				fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
				
				return fakeClient, &MockLoadBalancer{}, &MockHealthChecker{}, &MockResourceMigrator{}, &MockConfigManager{}, &MockMetricsCollector{}, &config.Config{
					Namespace: "test",
					NodeName:  "test-node",
				}
			},
			expectError: false,
		},
		{
			name: "nil client",
			setupMocks: func() (client.Client, *MockLoadBalancer, *MockHealthChecker, *MockResourceMigrator, *MockConfigManager, *MockMetricsCollector, *config.Config) {
				return nil, &MockLoadBalancer{}, &MockHealthChecker{}, &MockResourceMigrator{}, &MockConfigManager{}, &MockMetricsCollector{}, &config.Config{}
			},
			expectError: true,
		},
		{
			name: "nil config",
			setupMocks: func() (client.Client, *MockLoadBalancer, *MockHealthChecker, *MockResourceMigrator, *MockConfigManager, *MockMetricsCollector, *config.Config) {
				scheme := runtime.NewScheme()
				shardv1.AddToScheme(scheme)
				fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
				
				return fakeClient, &MockLoadBalancer{}, &MockHealthChecker{}, &MockResourceMigrator{}, &MockConfigManager{}, &MockMetricsCollector{}, nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, lb, hc, rm, cm, mc, cfg := tt.setupMocks()
			
			fakeKubeClient := fake.NewSimpleClientset()
			
			mockAlertManager := &MockAlertManager{}
			mockStructuredLogger := &MockStructuredLogger{}
			sm, err := NewShardManager(client, fakeKubeClient, lb, hc, rm, cm, mc, mockAlertManager, mockStructuredLogger, cfg)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, sm)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, sm)
				assert.False(t, sm.IsLeader()) // Should not be leader initially
			}
		})
	}
}

func TestCreateShard(t *testing.T) {
	sm, _, _, _, mockConfigManager, mockMetricsCollector, fakeClient := createTestShardManager(t)
	
	ctx := context.Background()
	
	// Setup mocks
	testConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:          1,
			MaxShards:          10,
			ScaleUpThreshold:   0.8,
			ScaleDownThreshold: 0.2,
		},
	}
	
	mockConfigManager.On("LoadConfig", ctx).Return(testConfig, nil)
	mockMetricsCollector.On("RecordCustomMetric", "shard_created_total", float64(1), mock.AnythingOfType("map[string]string")).Return(nil)

	// Test shard creation
	shard, err := sm.CreateShard(ctx, testConfig)
	
	assert.NoError(t, err)
	assert.NotNil(t, shard)
	assert.Equal(t, shardv1.ShardPhasePending, shard.Status.Phase)
	assert.NotEmpty(t, shard.Spec.ShardID)
	assert.NotNil(t, shard.Spec.HashRange)
	
	// Verify shard was created in fake client
	createdShard := &shardv1.ShardInstance{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: shard.Name, Namespace: shard.Namespace}, createdShard)
	assert.NoError(t, err)
	assert.Equal(t, shard.Spec.ShardID, createdShard.Spec.ShardID)
	
	mockConfigManager.AssertExpectations(t)
	mockMetricsCollector.AssertExpectations(t)
}

func TestDeleteShard(t *testing.T) {
	sm, _, _, _, _, _, fakeClient := createTestShardManager(t)
	
	ctx := context.Background()
	
	// Create a test shard first
	testShard := createTestShardInstance("test-shard", shardv1.ShardPhaseRunning, 0.5, true)
	err := fakeClient.Create(ctx, testShard)
	require.NoError(t, err)
	
	// Test shard deletion
	err = sm.DeleteShard(ctx, testShard.Name)
	assert.NoError(t, err)
	
	// Verify shard was deleted
	deletedShard := &shardv1.ShardInstance{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: testShard.Name, Namespace: testShard.Namespace}, deletedShard)
	assert.Error(t, err) // Should not be found
}

func TestScaleUp(t *testing.T) {
	sm, _, _, _, mockConfigManager, mockMetricsCollector, fakeClient := createTestShardManager(t)
	
	ctx := context.Background()
	
	// Create initial shard
	initialShard := createTestShardInstance("initial-shard", shardv1.ShardPhaseRunning, 0.5, true)
	err := fakeClient.Create(ctx, initialShard)
	require.NoError(t, err)
	
	// Setup mocks
	testConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:          1,
			MaxShards:          10,
			ScaleUpThreshold:   0.8,
			ScaleDownThreshold: 0.2,
		},
	}
	
	mockConfigManager.On("LoadConfig", ctx).Return(testConfig, nil)
	mockMetricsCollector.On("RecordCustomMetric", "shard_created_total", float64(1), mock.AnythingOfType("map[string]string")).Return(nil)

	// Test scale up
	err = sm.ScaleUp(ctx, 2)
	assert.NoError(t, err)
	
	// Verify we now have 2 shards
	shards, err := sm.ListShards(ctx)
	assert.NoError(t, err)
	assert.Len(t, shards, 2)
	
	mockConfigManager.AssertExpectations(t)
	mockMetricsCollector.AssertExpectations(t)
}

func TestScaleDown(t *testing.T) {
	sm, _, _, _, _, _, fakeClient := createTestShardManager(t)
	
	ctx := context.Background()
	
	// Create multiple shards
	shard1 := createTestShardInstance("shard-1", shardv1.ShardPhaseRunning, 0.3, true)
	shard2 := createTestShardInstance("shard-2", shardv1.ShardPhaseRunning, 0.4, true)
	shard3 := createTestShardInstance("shard-3", shardv1.ShardPhaseRunning, 0.2, true)
	
	err := fakeClient.Create(ctx, shard1)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, shard2)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, shard3)
	require.NoError(t, err)

	// Test scale down
	err = sm.ScaleDown(ctx, 2)
	assert.NoError(t, err)
	
	// Verify we now have 2 shards
	shards, err := sm.ListShards(ctx)
	assert.NoError(t, err)
	assert.Len(t, shards, 2)
}

func TestAssignResource(t *testing.T) {
	sm, mockLoadBalancer, _, _, _, _, fakeClient := createTestShardManager(t)
	
	ctx := context.Background()
	
	// Create test shard
	testShard := createTestShardInstance("test-shard", shardv1.ShardPhaseRunning, 0.5, true)
	err := fakeClient.Create(ctx, testShard)
	require.NoError(t, err)
	
	// Setup mocks
	testResource := &interfaces.Resource{
		ID:   "test-resource",
		Type: "test-type",
	}
	
	mockLoadBalancer.On("AssignResourceToShard", testResource, mock.AnythingOfType("[]*v1.ShardInstance")).Return(testShard, nil)

	// Test resource assignment
	assignedShardId, err := sm.AssignResource(ctx, testResource)
	
	assert.NoError(t, err)
	assert.Equal(t, testShard.Spec.ShardID, assignedShardId)
	
	mockLoadBalancer.AssertExpectations(t)
}

func TestCheckShardHealth(t *testing.T) {
	sm, _, mockHealthChecker, _, _, _, fakeClient := createTestShardManager(t)
	
	ctx := context.Background()
	
	// Create test shard
	testShard := createTestShardInstance("test-shard", shardv1.ShardPhaseRunning, 0.5, true)
	err := fakeClient.Create(ctx, testShard)
	require.NoError(t, err)
	
	// Setup mocks
	expectedHealth := &shardv1.HealthStatus{
		Healthy:   true,
		LastCheck: metav1.Now(),
		Message:   "Shard is healthy",
	}
	
	mockHealthChecker.On("CheckHealth", ctx, mock.AnythingOfType("*v1.ShardInstance")).Return(expectedHealth, nil)

	// Test health check
	health, err := sm.CheckShardHealth(ctx, testShard.Name)
	
	assert.NoError(t, err)
	assert.Equal(t, expectedHealth.Healthy, health.Healthy)
	assert.Equal(t, expectedHealth.Message, health.Message)
	
	mockHealthChecker.AssertExpectations(t)
}

func TestHandleFailedShard(t *testing.T) {
	sm, _, mockHealthChecker, _, _, _, _ := createTestShardManager(t)
	
	ctx := context.Background()
	shardId := "failed-shard"
	
	// Setup mocks
	mockHealthChecker.On("OnShardFailed", ctx, shardId).Return(nil)

	// Test failed shard handling
	err := sm.HandleFailedShard(ctx, shardId)
	
	assert.NoError(t, err)
	mockHealthChecker.AssertExpectations(t)
}

func TestListShards(t *testing.T) {
	sm, _, _, _, _, _, fakeClient := createTestShardManager(t)
	
	ctx := context.Background()
	
	// Create multiple test shards
	shard1 := createTestShardInstance("shard-1", shardv1.ShardPhaseRunning, 0.3, true)
	shard2 := createTestShardInstance("shard-2", shardv1.ShardPhaseRunning, 0.4, true)
	
	err := fakeClient.Create(ctx, shard1)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, shard2)
	require.NoError(t, err)

	// Test listing shards
	shards, err := sm.ListShards(ctx)
	
	assert.NoError(t, err)
	assert.Len(t, shards, 2)
	
	// Verify shard IDs
	shardIds := make([]string, len(shards))
	for i, shard := range shards {
		shardIds[i] = shard.Spec.ShardID
	}
	assert.Contains(t, shardIds, "shard-1")
	assert.Contains(t, shardIds, "shard-2")
}

func TestCountHealthyShards(t *testing.T) {
	sm, _, _, _, _, _, _ := createTestShardManager(t)
	
	shards := []*shardv1.ShardInstance{
		createTestShardInstance("shard-1", shardv1.ShardPhaseRunning, 0.3, true),
		createTestShardInstance("shard-2", shardv1.ShardPhaseRunning, 0.4, false),
		createTestShardInstance("shard-3", shardv1.ShardPhaseRunning, 0.2, true),
	}
	
	healthyCount := sm.countHealthyShards(shards)
	assert.Equal(t, 2, healthyCount)
}

func TestCalculateTotalLoad(t *testing.T) {
	sm, _, _, _, _, _, _ := createTestShardManager(t)
	
	tests := []struct {
		name     string
		shards   []*shardv1.ShardInstance
		expected float64
	}{
		{
			name:     "empty shards",
			shards:   []*shardv1.ShardInstance{},
			expected: 0,
		},
		{
			name: "single shard",
			shards: []*shardv1.ShardInstance{
				createTestShardInstance("shard-1", shardv1.ShardPhaseRunning, 0.5, true),
			},
			expected: 0.5,
		},
		{
			name: "multiple shards",
			shards: []*shardv1.ShardInstance{
				createTestShardInstance("shard-1", shardv1.ShardPhaseRunning, 0.3, true),
				createTestShardInstance("shard-2", shardv1.ShardPhaseRunning, 0.7, true),
			},
			expected: 0.5, // Average of 0.3 and 0.7
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalLoad := sm.calculateTotalLoad(tt.shards)
			assert.Equal(t, tt.expected, totalLoad)
		})
	}
}

func TestCalculateHashRangeForNewShard(t *testing.T) {
	sm, _, _, _, _, _, _ := createTestShardManager(t)
	
	tests := []struct {
		name           string
		existingShards []*shardv1.ShardInstance
		expectedStart  uint32
		expectedEnd    uint32
	}{
		{
			name:           "first shard",
			existingShards: []*shardv1.ShardInstance{},
			expectedStart:  0,
			expectedEnd:    0xFFFFFFFF,
		},
		{
			name: "second shard",
			existingShards: []*shardv1.ShardInstance{
				createTestShardInstance("shard-1", shardv1.ShardPhaseRunning, 0.3, true),
			},
			expectedStart: 0x80000000,
			expectedEnd:   0xFFFFFFFF,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashRange := sm.calculateHashRangeForNewShard(tt.existingShards)
			assert.Equal(t, tt.expectedStart, hashRange.Start)
			assert.Equal(t, tt.expectedEnd, hashRange.End)
		})
	}
}

func TestSelectShardsForRemoval(t *testing.T) {
	sm, _, _, _, _, _, _ := createTestShardManager(t)
	
	shards := []*shardv1.ShardInstance{
		createTestShardInstance("healthy-shard-1", shardv1.ShardPhaseRunning, 0.3, true),
		createTestShardInstance("unhealthy-shard", shardv1.ShardPhaseFailed, 0.4, false),
		createTestShardInstance("healthy-shard-2", shardv1.ShardPhaseRunning, 0.2, true),
	}
	
	// Test selecting 1 shard for removal (should prefer unhealthy)
	selected := sm.selectShardsForRemoval(shards, 1)
	assert.Len(t, selected, 1)
	assert.Equal(t, "unhealthy-shard", selected[0].Spec.ShardID)
	
	// Test selecting 2 shards for removal
	selected = sm.selectShardsForRemoval(shards, 2)
	assert.Len(t, selected, 2)
	
	// Should include the unhealthy shard
	shardIds := make([]string, len(selected))
	for i, shard := range selected {
		shardIds[i] = shard.Spec.ShardID
	}
	assert.Contains(t, shardIds, "unhealthy-shard")
}

func TestIsLeader(t *testing.T) {
	sm, _, _, _, _, _, _ := createTestShardManager(t)
	
	// Initially should not be leader
	assert.False(t, sm.IsLeader())
	
	// Simulate becoming leader
	sm.leaderMu.Lock()
	sm.isLeader = true
	sm.leaderMu.Unlock()
	
	assert.True(t, sm.IsLeader())
	
	// Simulate losing leadership
	sm.leaderMu.Lock()
	sm.isLeader = false
	sm.leaderMu.Unlock()
	
	assert.False(t, sm.IsLeader())
}

// Integration test for scaling decision logic
func TestMakeScalingDecision(t *testing.T) {
	sm, _, _, _, mockConfigManager, mockMetricsCollector, fakeClient := createTestShardManager(t)
	
	ctx := context.Background()
	
	// Create test shards with high load
	shard1 := createTestShardInstance("shard-1", shardv1.ShardPhaseRunning, 0.9, true)
	shard2 := createTestShardInstance("shard-2", shardv1.ShardPhaseRunning, 0.8, true)
	
	err := fakeClient.Create(ctx, shard1)
	require.NoError(t, err)
	err = fakeClient.Create(ctx, shard2)
	require.NoError(t, err)
	
	// Setup mocks for scale up scenario
	testConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:          1,
			MaxShards:          10,
			ScaleUpThreshold:   0.7, // Lower threshold to trigger scale up
			ScaleDownThreshold: 0.2,
		},
	}
	
	mockConfigManager.On("LoadConfig", ctx).Return(testConfig, nil)
	mockMetricsCollector.On("RecordCustomMetric", "shard_scale_up_total", float64(1), mock.AnythingOfType("map[string]string")).Return(nil)
	mockMetricsCollector.On("RecordCustomMetric", "shard_created_total", float64(1), mock.AnythingOfType("map[string]string")).Return(nil)

	// Test scaling decision
	err = sm.makeScalingDecision(ctx)
	assert.NoError(t, err)
	
	// Verify scale up occurred
	shards, err := sm.ListShards(ctx)
	assert.NoError(t, err)
	assert.Len(t, shards, 3) // Should have scaled up from 2 to 3
	
	mockConfigManager.AssertExpectations(t)
	mockMetricsCollector.AssertExpectations(t)
}