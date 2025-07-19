package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// MockClientSet is a mock implementation of the client set
type MockClientSet struct {
	Client     client.Client
	KubeClient *MockKubeClient
}

// MockKubeClient is a mock implementation of kubernetes.Interface
type MockKubeClient struct {
	mock.Mock
}

func TestNewWorkerShard(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.Config
		shardId   string
		expectErr bool
	}{
		{
			name:      "nil config",
			config:    nil,
			shardId:   "shard-1",
			expectErr: true,
		},
		{
			name:      "empty shard ID",
			config:    &config.Config{Namespace: "test"},
			shardId:   "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws, err := NewWorkerShard(tt.config, tt.shardId)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, ws)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ws)
				assert.Equal(t, tt.shardId, ws.shardId)
				assert.Equal(t, tt.config, ws.config)
			}
		})
	}
}

func TestWorkerShard_ProcessResource(t *testing.T) {
	ws := createTestWorkerShard(t)
	ctx := context.Background()

	// Start the worker shard
	ws.running = true

	tests := []struct {
		name      string
		resource  *interfaces.Resource
		running   bool
		draining  bool
		expectErr bool
	}{
		{
			name: "valid resource",
			resource: &interfaces.Resource{
				ID:   "resource-1",
				Type: "test",
				Data: map[string]string{"key": "value"},
			},
			running:   true,
			draining:  false,
			expectErr: false,
		},
		{
			name:      "nil resource",
			resource:  nil,
			running:   true,
			draining:  false,
			expectErr: true,
		},
		{
			name: "shard not running",
			resource: &interfaces.Resource{
				ID:   "resource-2",
				Type: "test",
			},
			running:   false,
			draining:  false,
			expectErr: true,
		},
		{
			name: "shard draining",
			resource: &interfaces.Resource{
				ID:   "resource-3",
				Type: "test",
			},
			running:   true,
			draining:  true,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws.mu.Lock()
			ws.running = tt.running
			ws.draining = tt.draining
			ws.mu.Unlock()

			err := ws.ProcessResource(ctx, tt.resource)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkerShard_GetAssignedResources(t *testing.T) {
	ws := createTestWorkerShard(t)
	ctx := context.Background()

	// Add some test resources
	testResources := []*interfaces.Resource{
		{ID: "resource-1", Type: "test1"},
		{ID: "resource-2", Type: "test2"},
		{ID: "resource-3", Type: "test3"},
	}

	ws.mu.Lock()
	for _, resource := range testResources {
		ws.assignedResources[resource.ID] = resource
	}
	ws.mu.Unlock()

	resources, err := ws.GetAssignedResources(ctx)

	assert.NoError(t, err)
	assert.Len(t, resources, len(testResources))

	// Check that all resources are present
	resourceMap := make(map[string]*interfaces.Resource)
	for _, resource := range resources {
		resourceMap[resource.ID] = resource
	}

	for _, expected := range testResources {
		actual, exists := resourceMap[expected.ID]
		assert.True(t, exists)
		assert.Equal(t, expected.ID, actual.ID)
		assert.Equal(t, expected.Type, actual.Type)
	}
}

func TestWorkerShard_ReportHealth(t *testing.T) {
	ws := createTestWorkerShard(t)
	ctx := context.Background()

	tests := []struct {
		name            string
		running         bool
		draining        bool
		resourceCount   int
		expectedHealthy bool
		expectedMessage string
	}{
		{
			name:            "healthy running shard",
			running:         true,
			draining:        false,
			resourceCount:   5,
			expectedHealthy: true,
			expectedMessage: "Shard is healthy (resources: 5)",
		},
		{
			name:            "draining shard",
			running:         true,
			draining:        true,
			resourceCount:   3,
			expectedHealthy: false,
			expectedMessage: "Shard is draining (resources: 3)",
		},
		{
			name:            "not running shard",
			running:         false,
			draining:        false,
			resourceCount:   0,
			expectedHealthy: false,
			expectedMessage: "Shard is not running (resources: 0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws.mu.Lock()
			ws.running = tt.running
			ws.draining = tt.draining
			// Clear and add test resources
			ws.assignedResources = make(map[string]*interfaces.Resource)
			for i := 0; i < tt.resourceCount; i++ {
				ws.assignedResources[fmt.Sprintf("resource-%d", i)] = &interfaces.Resource{
					ID: fmt.Sprintf("resource-%d", i),
				}
			}
			ws.mu.Unlock()

			health, err := ws.ReportHealth(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, health)
			assert.Equal(t, tt.expectedHealthy, health.Healthy)
			assert.Equal(t, tt.expectedMessage, health.Message)
			assert.Equal(t, 0, health.ErrorCount)
		})
	}
}

func TestWorkerShard_GetLoadMetrics(t *testing.T) {
	ws := createTestWorkerShard(t)
	ctx := context.Background()

	// Set up test metrics
	expectedMetrics := &shardv1.LoadMetrics{
		ResourceCount:  10,
		CPUUsage:       0.5,
		MemoryUsage:    0.3,
		ProcessingRate: 2.5,
		QueueLength:    5,
	}

	ws.mu.Lock()
	ws.loadMetrics = expectedMetrics
	ws.mu.Unlock()

	metrics, err := ws.GetLoadMetrics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Equal(t, expectedMetrics.ResourceCount, metrics.ResourceCount)
	assert.Equal(t, expectedMetrics.CPUUsage, metrics.CPUUsage)
	assert.Equal(t, expectedMetrics.MemoryUsage, metrics.MemoryUsage)
	assert.Equal(t, expectedMetrics.ProcessingRate, metrics.ProcessingRate)
	assert.Equal(t, expectedMetrics.QueueLength, metrics.QueueLength)
}

func TestWorkerShard_AcceptMigratedResources(t *testing.T) {
	ws := createTestWorkerShard(t)
	ctx := context.Background()

	// Set up shard instance in fake client
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.shardId,
			Namespace: ws.config.Namespace,
		},
		Status: shardv1.ShardInstanceStatus{
			Phase: shardv1.ShardPhaseRunning,
		},
	}
	err := ws.client.Create(ctx, shard)
	require.NoError(t, err)

	tests := []struct {
		name      string
		resources []*interfaces.Resource
		running   bool
		draining  bool
		expectErr bool
	}{
		{
			name: "accept valid resources",
			resources: []*interfaces.Resource{
				{ID: "migrated-1", Type: "test"},
				{ID: "migrated-2", Type: "test"},
			},
			running:   true,
			draining:  false,
			expectErr: false,
		},
		{
			name:      "empty resources list",
			resources: []*interfaces.Resource{},
			running:   true,
			draining:  false,
			expectErr: false,
		},
		{
			name: "shard not running",
			resources: []*interfaces.Resource{
				{ID: "migrated-3", Type: "test"},
			},
			running:   false,
			draining:  false,
			expectErr: true,
		},
		{
			name: "shard draining",
			resources: []*interfaces.Resource{
				{ID: "migrated-4", Type: "test"},
			},
			running:   true,
			draining:  true,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws.mu.Lock()
			ws.running = tt.running
			ws.draining = tt.draining
			initialCount := len(ws.assignedResources)
			ws.mu.Unlock()

			err := ws.AcceptMigratedResources(ctx, tt.resources)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check that resources were added
				ws.mu.RLock()
				finalCount := len(ws.assignedResources)
				ws.mu.RUnlock()

				expectedCount := initialCount + len(tt.resources)
				assert.Equal(t, expectedCount, finalCount)

				// Verify specific resources were added
				for _, resource := range tt.resources {
					ws.mu.RLock()
					_, exists := ws.assignedResources[resource.ID]
					ws.mu.RUnlock()
					assert.True(t, exists, "Resource %s should be assigned", resource.ID)
				}
			}
		})
	}
}

func TestWorkerShard_Drain(t *testing.T) {
	ws := createTestWorkerShard(t)
	ctx := context.Background()

	// Set up shard instance in fake client
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.shardId,
			Namespace: ws.config.Namespace,
		},
		Status: shardv1.ShardInstanceStatus{
			Phase: shardv1.ShardPhaseRunning,
		},
	}
	err := ws.client.Create(ctx, shard)
	require.NoError(t, err)

	// Test first drain
	err = ws.Drain(ctx)
	assert.NoError(t, err)

	ws.mu.RLock()
	draining := ws.draining
	ws.mu.RUnlock()
	assert.True(t, draining)

	// Test second drain (should fail)
	err = ws.Drain(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already draining")
}

func TestWorkerShard_GetShardID(t *testing.T) {
	expectedID := "test-shard-123"
	ws := createTestWorkerShardWithID(t, expectedID)

	actualID := ws.GetShardID()
	assert.Equal(t, expectedID, actualID)
}

func TestWorkerShard_MigrateResourcesTo(t *testing.T) {
	ws := createTestWorkerShard(t)
	ctx := context.Background()

	// Set up shard instance in fake client
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.shardId,
			Namespace: ws.config.Namespace,
		},
		Status: shardv1.ShardInstanceStatus{
			Phase: shardv1.ShardPhaseRunning,
		},
	}
	err := ws.client.Create(ctx, shard)
	require.NoError(t, err)

	// Start the migration handling loop
	ws.running = true
	go ws.migrationHandlingLoop(ctx)

	// Add some resources to migrate
	testResources := []*interfaces.Resource{
		{ID: "resource-1", Type: "test"},
		{ID: "resource-2", Type: "test"},
	}

	ws.mu.Lock()
	for _, resource := range testResources {
		ws.assignedResources[resource.ID] = resource
	}
	initialCount := len(ws.assignedResources)
	ws.mu.Unlock()

	tests := []struct {
		name        string
		targetShard string
		resources   []*interfaces.Resource
		expectErr   bool
	}{
		{
			name:        "valid migration",
			targetShard: "target-shard-1",
			resources:   testResources,
			expectErr:   false,
		},
		{
			name:        "empty target shard",
			targetShard: "",
			resources:   testResources,
			expectErr:   true,
		},
		{
			name:        "empty resources",
			targetShard: "target-shard-2",
			resources:   []*interfaces.Resource{},
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ws.MigrateResourcesTo(ctx, tt.targetShard, tt.resources)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if len(tt.resources) > 0 {
					// Check that resources were removed
					ws.mu.RLock()
					finalCount := len(ws.assignedResources)
					ws.mu.RUnlock()

					expectedCount := initialCount - len(tt.resources)
					assert.Equal(t, expectedCount, finalCount)
				}
			}
		})
	}
}

func TestWorkerShard_CollectMetrics(t *testing.T) {
	ws := createTestWorkerShard(t)

	// Add some test resources
	testResources := []*interfaces.Resource{
		{ID: "resource-1", Type: "test"},
		{ID: "resource-2", Type: "test"},
		{ID: "resource-3", Type: "test"},
	}

	ws.mu.Lock()
	for _, resource := range testResources {
		ws.assignedResources[resource.ID] = resource
	}
	// Add some items to the queue
	for i := 0; i < 5; i++ {
		ws.resourceQueue <- &interfaces.Resource{ID: fmt.Sprintf("queued-%d", i)}
	}
	ws.mu.Unlock()

	// Collect metrics
	ws.collectMetrics()

	ws.mu.RLock()
	metrics := ws.loadMetrics
	ws.mu.RUnlock()

	assert.Equal(t, len(testResources), metrics.ResourceCount)
	assert.Equal(t, 5, metrics.QueueLength)
	assert.True(t, metrics.CPUUsage >= 0 && metrics.CPUUsage <= 1.0)
	assert.True(t, metrics.MemoryUsage >= 0 && metrics.MemoryUsage <= 1.0)
	assert.True(t, metrics.ProcessingRate >= 0)
}

// Helper functions

// createTestWorkerShard creates a worker shard for testing
func createTestWorkerShard(t *testing.T) *WorkerShard {
	return createTestWorkerShardWithID(t, "test-shard")
}

// createTestWorkerShardWithID creates a worker shard with specific ID for testing
func createTestWorkerShardWithID(t *testing.T, shardId string) *WorkerShard {
	cfg := &config.Config{
		Namespace: "test-namespace",
		HealthCheck: config.HealthCheckConfig{
			Interval: 30 * time.Second,
		},
		DefaultShardConfig: config.ShardConfig{
			GracefulShutdownTimeout: 60 * time.Second,
		},
	}

	// Create fake Kubernetes client
	scheme := runtime.NewScheme()
	err := shardv1.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ws := &WorkerShard{
		config:            cfg,
		shardId:           shardId,
		client:            fakeClient,
		assignedResources: make(map[string]*interfaces.Resource),
		resourceQueue:     make(chan *interfaces.Resource, 1000),
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		migrationCh:       make(chan *migrationRequest, 100),
		loadMetrics: &shardv1.LoadMetrics{
			ResourceCount:  0,
			CPUUsage:       0.0,
			MemoryUsage:    0.0,
			ProcessingRate: 0.0,
			QueueLength:    0,
		},
	}

	return ws
}

// Mock function variable for dependency injection in tests
var newClientSetFunc = func(cfg *config.Config) (*MockClientSet, error) {
	return nil, fmt.Errorf("mock not implemented")
}