package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/config"
	"github.com/k8s-shard-controller/pkg/controllers"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// IntegrationTestSuite provides a test suite for integration tests
type IntegrationTestSuite struct {
	suite.Suite
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc

	// Test components
	shardManager     interfaces.ShardManager
	loadBalancer     interfaces.LoadBalancer
	healthChecker    interfaces.HealthChecker
	resourceMigrator interfaces.ResourceMigrator
	configManager    interfaces.ConfigManager
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (suite *IntegrationTestSuite) SetupSuite() {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

	suite.ctx, suite.cancel = context.WithCancel(context.TODO())

	// Setup test environment
	suite.testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "manifests", "crds"),
		},
		ErrorIfCRDPathMissing: false,
	}

	var err error
	suite.cfg, err = suite.testEnv.Start()
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), suite.cfg)

	// Add custom resources to scheme
	err = shardv1.AddToScheme(scheme.Scheme)
	require.NoError(suite.T(), err)

	// Create k8s client
	suite.k8sClient, err = client.New(suite.cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(suite.T(), err)

	// Initialize test components
	suite.setupTestComponents()
}

func (suite *IntegrationTestSuite) TearDownSuite() {
	suite.cancel()
	err := suite.testEnv.Stop()
	require.NoError(suite.T(), err)
}

func (suite *IntegrationTestSuite) SetupTest() {
	// Clean up any existing resources before each test
	suite.cleanupTestResources()
}

func (suite *IntegrationTestSuite) TearDownTest() {
	// Clean up after each test
	suite.cleanupTestResources()
}

func (suite *IntegrationTestSuite) setupTestComponents() {
	cfg := config.DefaultConfig()

	var err error

	// Initialize health checker
	suite.healthChecker, err = controllers.NewHealthChecker(suite.k8sClient, cfg.HealthCheck)
	require.NoError(suite.T(), err)

	// Initialize load balancer
	suite.loadBalancer, err = controllers.NewLoadBalancer(shardv1.ConsistentHashStrategy, nil)
	require.NoError(suite.T(), err)

	// Initialize resource migrator
	suite.resourceMigrator, err = controllers.NewResourceMigrator(suite.k8sClient, cfg.Migration)
	require.NoError(suite.T(), err)

	// Initialize config manager
	suite.configManager, err = controllers.NewConfigManager(suite.k8sClient, cfg)
	require.NoError(suite.T(), err)

	// Initialize shard manager
	suite.shardManager, err = controllers.NewShardManager(
		suite.k8sClient,
		suite.loadBalancer,
		suite.healthChecker,
		suite.resourceMigrator,
		cfg,
	)
	require.NoError(suite.T(), err)
}

func (suite *IntegrationTestSuite) cleanupTestResources() {
	// Delete all ShardInstances
	shardList := &shardv1.ShardInstanceList{}
	err := suite.k8sClient.List(suite.ctx, shardList)
	if err == nil {
		for _, shard := range shardList.Items {
			_ = suite.k8sClient.Delete(suite.ctx, &shard)
		}
	}

	// Delete all ShardConfigs
	configList := &shardv1.ShardConfigList{}
	err = suite.k8sClient.List(suite.ctx, configList)
	if err == nil {
		for _, config := range configList.Items {
			_ = suite.k8sClient.Delete(suite.ctx, &config)
		}
	}

	// Delete test ConfigMaps
	cmList := &corev1.ConfigMapList{}
	err = suite.k8sClient.List(suite.ctx, cmList)
	if err == nil {
		for _, cm := range cmList.Items {
			if cm.Name == "shard-config" {
				_ = suite.k8sClient.Delete(suite.ctx, &cm)
			}
		}
	}

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)
}

func (suite *IntegrationTestSuite) createTestShardConfig() *shardv1.ShardConfig {
	config := &shardv1.ShardConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-shard-config",
			Namespace: "default",
		},
		Spec: shardv1.ShardConfigSpec{
			MinShards:               2,
			MaxShards:               10,
			ScaleUpThreshold:        0.8,
			ScaleDownThreshold:      0.3,
			HealthCheckInterval:     metav1.Duration{Duration: 10 * time.Second},
			LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
			GracefulShutdownTimeout: metav1.Duration{Duration: 30 * time.Second},
		},
	}

	err := suite.k8sClient.Create(suite.ctx, config)
	require.NoError(suite.T(), err)

	return config
}

func (suite *IntegrationTestSuite) createTestShardInstance(shardID string, phase shardv1.ShardPhase) *shardv1.ShardInstance {
	shard := &shardv1.ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shardID,
			Namespace: "default",
		},
		Spec: shardv1.ShardInstanceSpec{
			ShardID: shardID,
			HashRange: &shardv1.HashRange{
				Start: 0,
				End:   1000,
			},
		},
		Status: shardv1.ShardInstanceStatus{
			Phase:         phase,
			LastHeartbeat: metav1.Now(),
			HealthStatus:  &shardv1.HealthStatus{Healthy: true},
		},
	}

	err := suite.k8sClient.Create(suite.ctx, shard)
	require.NoError(suite.T(), err)

	return shard
}

func (suite *IntegrationTestSuite) waitForShardPhase(shardID string, expectedPhase shardv1.ShardPhase, timeout time.Duration) error {
	return suite.waitForCondition(timeout, func() bool {
		shard := &shardv1.ShardInstance{}
		err := suite.k8sClient.Get(suite.ctx, types.NamespacedName{
			Name:      shardID,
			Namespace: "default",
		}, shard)
		if err != nil {
			return false
		}
		return shard.Status.Phase == expectedPhase
	})
}

func (suite *IntegrationTestSuite) waitForCondition(timeout time.Duration, condition func() bool) error {
	ctx, cancel := context.WithTimeout(suite.ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for condition")
		case <-ticker.C:
			if condition() {
				return nil
			}
		}
	}
}

// TestShardCreationAndDeletion tests basic shard lifecycle management
func (suite *IntegrationTestSuite) TestShardCreationAndDeletion() {
	// Create shard config
	config := suite.createTestShardConfig()

	// Test shard creation
	shardID := "test-shard-1"
	shard, err := suite.shardManager.CreateShard(suite.ctx, &config.Spec)
	require.NoError(suite.T(), err)
	assert.NotNil(suite.T(), shard)
	assert.Equal(suite.T(), shardv1.ShardPhasePending, shard.Status.Phase)

	// Wait for shard to be created in cluster
	err = suite.waitForCondition(5*time.Second, func() bool {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		return err == nil && len(shardList.Items) > 0
	})
	require.NoError(suite.T(), err)

	// Test shard deletion
	err = suite.shardManager.DeleteShard(suite.ctx, shard.Spec.ShardID)
	require.NoError(suite.T(), err)

	// Wait for shard to be deleted
	err = suite.waitForCondition(5*time.Second, func() bool {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		return err == nil && len(shardList.Items) == 0
	})
	require.NoError(suite.T(), err)
}

// TestShardScaling tests automatic scaling up and down
func (suite *IntegrationTestSuite) TestShardScaling() {
	// Create shard config
	config := suite.createTestShardConfig()

	// Test scale up
	err := suite.shardManager.ScaleUp(suite.ctx, 3)
	require.NoError(suite.T(), err)

	// Wait for shards to be created
	err = suite.waitForCondition(10*time.Second, func() bool {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		return err == nil && len(shardList.Items) == 3
	})
	require.NoError(suite.T(), err)

	// Verify all shards are in expected state
	shardList := &shardv1.ShardInstanceList{}
	err = suite.k8sClient.List(suite.ctx, shardList)
	require.NoError(suite.T(), err)

	for _, shard := range shardList.Items {
		assert.Contains(suite.T(), []shardv1.ShardPhase{
			shardv1.ShardPhasePending,
			shardv1.ShardPhaseRunning,
		}, shard.Status.Phase)
	}

	// Test scale down
	err = suite.shardManager.ScaleDown(suite.ctx, 2)
	require.NoError(suite.T(), err)

	// Wait for scale down to complete
	err = suite.waitForCondition(15*time.Second, func() bool {
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
		return runningCount == 2
	})
	require.NoError(suite.T(), err)
}

// TestHealthCheckingAndFailureDetection tests health monitoring
func (suite *IntegrationTestSuite) TestHealthCheckingAndFailureDetection() {
	// Create test shards
	healthyShard := suite.createTestShardInstance("healthy-shard", shardv1.ShardPhaseRunning)
	unhealthyShard := suite.createTestShardInstance("unhealthy-shard", shardv1.ShardPhaseRunning)

	// Make one shard unhealthy by setting old heartbeat
	unhealthyShard.Status.LastHeartbeat = metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
	err := suite.k8sClient.Status().Update(suite.ctx, unhealthyShard)
	require.NoError(suite.T(), err)

	// Start health checking
	err = suite.healthChecker.StartHealthChecking(suite.ctx, 1*time.Second)
	require.NoError(suite.T(), err)
	defer suite.healthChecker.StopHealthChecking()

	// Wait for health status to be updated
	err = suite.waitForCondition(5*time.Second, func() bool {
		return !suite.healthChecker.IsShardHealthy("unhealthy-shard")
	})
	require.NoError(suite.T(), err)

	// Verify healthy shard is still healthy
	assert.True(suite.T(), suite.healthChecker.IsShardHealthy("healthy-shard"))

	// Get unhealthy shards list
	unhealthyShards := suite.healthChecker.GetUnhealthyShards()
	assert.Contains(suite.T(), unhealthyShards, "unhealthy-shard")
	assert.NotContains(suite.T(), unhealthyShards, "healthy-shard")
}

// TestFailureRecovery tests shard failure and recovery scenarios
func (suite *IntegrationTestSuite) TestFailureRecovery() {
	// Create test shards
	shard1 := suite.createTestShardInstance("shard-1", shardv1.ShardPhaseRunning)
	shard2 := suite.createTestShardInstance("shard-2", shardv1.ShardPhaseRunning)

	// Add some resources to shard-1
	shard1.Status.AssignedResources = []string{"resource-1", "resource-2", "resource-3"}
	err := suite.k8sClient.Status().Update(suite.ctx, shard1)
	require.NoError(suite.T(), err)

	// Simulate shard-1 failure
	shard1.Status.Phase = shardv1.ShardPhaseFailed
	shard1.Status.HealthStatus = &shardv1.HealthStatus{
		Healthy:    false,
		LastCheck:  metav1.Now(),
		ErrorCount: 3,
		Message:    "Shard failed",
	}
	err = suite.k8sClient.Status().Update(suite.ctx, shard1)
	require.NoError(suite.T(), err)

	// Test failure handling
	err = suite.shardManager.HandleFailedShard(suite.ctx, "shard-1")
	require.NoError(suite.T(), err)

	// Wait for resources to be migrated
	err = suite.waitForCondition(10*time.Second, func() bool {
		updatedShard2 := &shardv1.ShardInstance{}
		err := suite.k8sClient.Get(suite.ctx, types.NamespacedName{
			Name:      "shard-2",
			Namespace: "default",
		}, updatedShard2)
		if err != nil {
			return false
		}
		return len(updatedShard2.Status.AssignedResources) > 0
	})
	require.NoError(suite.T(), err)

	// Verify resources were migrated to healthy shard
	updatedShard2 := &shardv1.ShardInstance{}
	err = suite.k8sClient.Get(suite.ctx, types.NamespacedName{
		Name:      "shard-2",
		Namespace: "default",
	}, updatedShard2)
	require.NoError(suite.T(), err)
	assert.Greater(suite.T(), len(updatedShard2.Status.AssignedResources), 0)
}

// TestLoadBalancing tests load balancing across shards
func (suite *IntegrationTestSuite) TestLoadBalancing() {
	// Create test shards with different loads
	shard1 := suite.createTestShardInstance("shard-1", shardv1.ShardPhaseRunning)
	shard2 := suite.createTestShardInstance("shard-2", shardv1.ShardPhaseRunning)
	shard3 := suite.createTestShardInstance("shard-3", shardv1.ShardPhaseRunning)

	// Set different loads
	shard1.Status.Load = 0.9 // High load
	shard2.Status.Load = 0.2 // Low load
	shard3.Status.Load = 0.5 // Medium load

	err := suite.k8sClient.Status().Update(suite.ctx, shard1)
	require.NoError(suite.T(), err)
	err = suite.k8sClient.Status().Update(suite.ctx, shard2)
	require.NoError(suite.T(), err)
	err = suite.k8sClient.Status().Update(suite.ctx, shard3)
	require.NoError(suite.T(), err)

	// Get all shards
	shardList := &shardv1.ShardInstanceList{}
	err = suite.k8sClient.List(suite.ctx, shardList)
	require.NoError(suite.T(), err)

	shards := make([]*shardv1.ShardInstance, len(shardList.Items))
	for i := range shardList.Items {
		shards[i] = &shardList.Items[i]
	}

	// Test load balancing - should select shard with lowest load
	suite.loadBalancer.SetStrategy(shardv1.LeastLoadedStrategy, shards)
	optimalShard, err := suite.loadBalancer.GetOptimalShard(shards)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "shard-2", optimalShard.Spec.ShardID) // Should pick the least loaded

	// Test rebalancing decision
	shouldRebalance := suite.loadBalancer.ShouldRebalance(shards)
	assert.True(suite.T(), shouldRebalance) // Load difference is significant

	// Generate rebalance plan
	plan, err := suite.loadBalancer.GenerateRebalancePlan(shards)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "shard-1", plan.SourceShard) // Should move from highest load
	assert.Contains(suite.T(), []string{"shard-2", "shard-3"}, plan.TargetShard)
}

// TestResourceMigration tests resource migration between shards
func (suite *IntegrationTestSuite) TestResourceMigration() {
	// Create test shards
	sourceShard := suite.createTestShardInstance("source-shard", shardv1.ShardPhaseRunning)
	targetShard := suite.createTestShardInstance("target-shard", shardv1.ShardPhaseRunning)

	// Add resources to source shard
	resources := []string{"resource-1", "resource-2", "resource-3"}
	sourceShard.Status.AssignedResources = resources
	err := suite.k8sClient.Status().Update(suite.ctx, sourceShard)
	require.NoError(suite.T(), err)

	// Create migration plan
	plan := &shardv1.MigrationPlan{
		SourceShard:   "source-shard",
		TargetShard:   "target-shard",
		Resources:     resources[:2], // Migrate first 2 resources
		EstimatedTime: metav1.Duration{Duration: 30 * time.Second},
		Priority:      shardv1.MigrationPriorityHigh,
	}

	// Execute migration
	err = suite.resourceMigrator.ExecuteMigration(suite.ctx, plan)
	require.NoError(suite.T(), err)

	// Wait for migration to complete
	err = suite.waitForCondition(10*time.Second, func() bool {
		updatedTarget := &shardv1.ShardInstance{}
		err := suite.k8sClient.Get(suite.ctx, types.NamespacedName{
			Name:      "target-shard",
			Namespace: "default",
		}, updatedTarget)
		if err != nil {
			return false
		}
		return len(updatedTarget.Status.AssignedResources) >= 2
	})
	require.NoError(suite.T(), err)

	// Verify migration results
	updatedSource := &shardv1.ShardInstance{}
	err = suite.k8sClient.Get(suite.ctx, types.NamespacedName{
		Name:      "source-shard",
		Namespace: "default",
	}, updatedSource)
	require.NoError(suite.T(), err)

	updatedTarget := &shardv1.ShardInstance{}
	err = suite.k8sClient.Get(suite.ctx, types.NamespacedName{
		Name:      "target-shard",
		Namespace: "default",
	}, updatedTarget)
	require.NoError(suite.T(), err)

	// Source should have fewer resources
	assert.Equal(suite.T(), 1, len(updatedSource.Status.AssignedResources))
	// Target should have the migrated resources
	assert.Equal(suite.T(), 2, len(updatedTarget.Status.AssignedResources))
}

// TestConfigurationDynamicUpdate tests dynamic configuration updates
func (suite *IntegrationTestSuite) TestConfigurationDynamicUpdate() {
	// Create initial config
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shard-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"minShards":           "2",
			"maxShards":           "5",
			"scaleUpThreshold":    "0.8",
			"scaleDownThreshold":  "0.3",
			"healthCheckInterval": "30s",
			"loadBalanceStrategy": "consistent-hash",
		},
	}

	err := suite.k8sClient.Create(suite.ctx, configMap)
	require.NoError(suite.T(), err)

	// Start config manager
	err = suite.configManager.StartWatching(suite.ctx)
	require.NoError(suite.T(), err)
	defer suite.configManager.StopWatching()

	// Wait for initial config to be loaded
	err = suite.waitForCondition(5*time.Second, func() bool {
		config := suite.configManager.GetCurrentConfig()
		return config.MinShards == 2 && config.MaxShards == 5
	})
	require.NoError(suite.T(), err)

	// Update configuration
	configMap.Data["maxShards"] = "10"
	configMap.Data["scaleUpThreshold"] = "0.7"
	err = suite.k8sClient.Update(suite.ctx, configMap)
	require.NoError(suite.T(), err)

	// Wait for config to be updated
	err = suite.waitForCondition(5*time.Second, func() bool {
		config := suite.configManager.GetCurrentConfig()
		return config.MaxShards == 10 && config.ScaleUpThreshold == 0.7
	})
	require.NoError(suite.T(), err)

	// Verify updated configuration
	config := suite.configManager.GetCurrentConfig()
	assert.Equal(suite.T(), int32(2), config.MinShards)
	assert.Equal(suite.T(), int32(10), config.MaxShards)
	assert.Equal(suite.T(), 0.7, config.ScaleUpThreshold)
	assert.Equal(suite.T(), 0.3, config.ScaleDownThreshold)
}

// TestEndToEndScenario tests a complete end-to-end scenario
func (suite *IntegrationTestSuite) TestEndToEndScenario() {
	// 1. Create initial configuration
	config := suite.createTestShardConfig()

	// 2. Start with minimum shards
	err := suite.shardManager.ScaleUp(suite.ctx, int(config.Spec.MinShards))
	require.NoError(suite.T(), err)

	// Wait for initial shards
	err = suite.waitForCondition(10*time.Second, func() bool {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		return err == nil && len(shardList.Items) == int(config.Spec.MinShards)
	})
	require.NoError(suite.T(), err)

	// 3. Start health checking
	err = suite.healthChecker.StartHealthChecking(suite.ctx, 2*time.Second)
	require.NoError(suite.T(), err)
	defer suite.healthChecker.StopHealthChecking()

	// 4. Simulate high load requiring scale up
	shardList := &shardv1.ShardInstanceList{}
	err = suite.k8sClient.List(suite.ctx, shardList)
	require.NoError(suite.T(), err)

	// Set high load on all shards
	for i := range shardList.Items {
		shardList.Items[i].Status.Load = 0.9
		err = suite.k8sClient.Status().Update(suite.ctx, &shardList.Items[i])
		require.NoError(suite.T(), err)
	}

	// 5. Trigger scale up
	err = suite.shardManager.ScaleUp(suite.ctx, 4)
	require.NoError(suite.T(), err)

	// Wait for scale up
	err = suite.waitForCondition(15*time.Second, func() bool {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		return err == nil && len(shardList.Items) == 4
	})
	require.NoError(suite.T(), err)

	// 6. Simulate shard failure
	err = suite.k8sClient.List(suite.ctx, shardList)
	require.NoError(suite.T(), err)

	failedShard := &shardList.Items[0]
	failedShard.Status.Phase = shardv1.ShardPhaseFailed
	failedShard.Status.HealthStatus = &shardv1.HealthStatus{
		Healthy:    false,
		LastCheck:  metav1.Now(),
		ErrorCount: 5,
		Message:    "Simulated failure",
	}
	failedShard.Status.AssignedResources = []string{"res-1", "res-2"}
	err = suite.k8sClient.Status().Update(suite.ctx, failedShard)
	require.NoError(suite.T(), err)

	// 7. Handle failure and verify recovery
	err = suite.shardManager.HandleFailedShard(suite.ctx, failedShard.Spec.ShardID)
	require.NoError(suite.T(), err)

	// Wait for resources to be migrated
	err = suite.waitForCondition(10*time.Second, func() bool {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		if err != nil {
			return false
		}

		totalResources := 0
		for _, shard := range shardList.Items {
			if shard.Status.Phase == shardv1.ShardPhaseRunning {
				totalResources += len(shard.Status.AssignedResources)
			}
		}
		return totalResources >= 2 // Resources should be migrated
	})
	require.NoError(suite.T(), err)

	// 8. Simulate load decrease and scale down
	err = suite.k8sClient.List(suite.ctx, shardList)
	require.NoError(suite.T(), err)

	for i := range shardList.Items {
		if shardList.Items[i].Status.Phase == shardv1.ShardPhaseRunning {
			shardList.Items[i].Status.Load = 0.1 // Low load
			err = suite.k8sClient.Status().Update(suite.ctx, &shardList.Items[i])
			require.NoError(suite.T(), err)
		}
	}

	// 9. Scale down
	err = suite.shardManager.ScaleDown(suite.ctx, int(config.Spec.MinShards))
	require.NoError(suite.T(), err)

	// Wait for scale down
	err = suite.waitForCondition(20*time.Second, func() bool {
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
		return runningCount == int(config.Spec.MinShards)
	})
	require.NoError(suite.T(), err)

	// 10. Verify final state
	err = suite.k8sClient.List(suite.ctx, shardList)
	require.NoError(suite.T(), err)

	runningShards := 0
	for _, shard := range shardList.Items {
		if shard.Status.Phase == shardv1.ShardPhaseRunning {
			runningShards++
		}
	}
	assert.Equal(suite.T(), int(config.Spec.MinShards), runningShards)
}

// TestChaosScenarios tests various failure scenarios
func (suite *IntegrationTestSuite) TestChaosScenarios() {
	// Create multiple shards
	for i := 1; i <= 5; i++ {
		suite.createTestShardInstance(fmt.Sprintf("chaos-shard-%d", i), shardv1.ShardPhaseRunning)
	}

	// Start health checking
	err := suite.healthChecker.StartHealthChecking(suite.ctx, 1*time.Second)
	require.NoError(suite.T(), err)
	defer suite.healthChecker.StopHealthChecking()

	// Scenario 1: Multiple simultaneous failures
	shardList := &shardv1.ShardInstanceList{}
	err = suite.k8sClient.List(suite.ctx, shardList)
	require.NoError(suite.T(), err)

	// Fail 2 shards simultaneously
	for i := 0; i < 2; i++ {
		shard := &shardList.Items[i]
		shard.Status.Phase = shardv1.ShardPhaseFailed
		shard.Status.HealthStatus = &shardv1.HealthStatus{
			Healthy:    false,
			LastCheck:  metav1.Now(),
			ErrorCount: 3,
			Message:    "Chaos failure",
		}
		shard.Status.AssignedResources = []string{fmt.Sprintf("res-%d-1", i), fmt.Sprintf("res-%d-2", i)}
		err = suite.k8sClient.Status().Update(suite.ctx, shard)
		require.NoError(suite.T(), err)
	}

	// Wait for failures to be detected
	err = suite.waitForCondition(5*time.Second, func() bool {
		unhealthy := suite.healthChecker.GetUnhealthyShards()
		return len(unhealthy) >= 2
	})
	require.NoError(suite.T(), err)

	// Scenario 2: Network partition simulation (stale heartbeats)
	for i := 2; i < 4; i++ {
		shard := &shardList.Items[i]
		shard.Status.LastHeartbeat = metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
		err = suite.k8sClient.Status().Update(suite.ctx, shard)
		require.NoError(suite.T(), err)
	}

	// Wait for network partition to be detected
	err = suite.waitForCondition(5*time.Second, func() bool {
		unhealthy := suite.healthChecker.GetUnhealthyShards()
		return len(unhealthy) >= 4
	})
	require.NoError(suite.T(), err)

	// Verify system still has healthy shards
	healthySummary := suite.healthChecker.GetHealthSummary()
	healthyCount := 0
	for _, status := range healthySummary {
		if status.Healthy {
			healthyCount++
		}
	}
	assert.Greater(suite.T(), healthyCount, 0, "Should have at least one healthy shard")

	// Scenario 3: Recovery simulation
	// Restore one failed shard
	restoredShard := &shardList.Items[0]
	restoredShard.Status.Phase = shardv1.ShardPhaseRunning
	restoredShard.Status.HealthStatus = &shardv1.HealthStatus{
		Healthy:    true,
		LastCheck:  metav1.Now(),
		ErrorCount: 0,
		Message:    "Recovered",
	}
	restoredShard.Status.LastHeartbeat = metav1.Now()
	err = suite.k8sClient.Status().Update(suite.ctx, restoredShard)
	require.NoError(suite.T(), err)

	// Wait for recovery to be detected
	err = suite.waitForCondition(5*time.Second, func() bool {
		return suite.healthChecker.IsShardHealthy(restoredShard.Spec.ShardID)
	})
	require.NoError(suite.T(), err)
}
