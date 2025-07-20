package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// TestLoadBalancingStrategies tests different load balancing strategies
func (suite *IntegrationTestSuite) TestLoadBalancingStrategies() {
	// Create test shards
	shards := make([]*shardv1.ShardInstance, 3)
	for i := 0; i < 3; i++ {
		shardID := fmt.Sprintf("lb-test-shard-%d", i+1)
		shards[i] = suite.createTestShardInstance(shardID, shardv1.ShardPhaseRunning)

		// Set different loads
		shards[i].Status.Load = float64(i+1) * 0.2 // 0.2, 0.4, 0.6
		err := suite.k8sClient.Status().Update(suite.ctx, shards[i])
		require.NoError(suite.T(), err)
	}

	// Test Consistent Hash Strategy
	suite.T().Run("ConsistentHashStrategy", func(t *testing.T) {
		err := suite.loadBalancer.SetStrategy(shardv1.ConsistentHashStrategy, shards)
		require.NoError(t, err)

		// Test resource assignment consistency
		resource := &interfaces.Resource{ID: "test-resource-1"}

		// Assign same resource multiple times - should always go to same shard
		var assignedShardID string
		for i := 0; i < 5; i++ {
			shard, err := suite.loadBalancer.AssignResourceToShard(resource, shards)
			require.NoError(t, err)

			if i == 0 {
				assignedShardID = shard.Spec.ShardID
			} else {
				assert.Equal(t, assignedShardID, shard.Spec.ShardID,
					"Consistent hash should assign same resource to same shard")
			}
		}

		// Test different resources get distributed
		resourceAssignments := make(map[string]int)
		for i := 0; i < 100; i++ {
			resource := &interfaces.Resource{ID: fmt.Sprintf("resource-%d", i)}
			shard, err := suite.loadBalancer.AssignResourceToShard(resource, shards)
			require.NoError(t, err)
			resourceAssignments[shard.Spec.ShardID]++
		}

		// All shards should get some resources
		assert.Len(t, resourceAssignments, 3)
		for shardID, count := range resourceAssignments {
			assert.Greater(t, count, 0, "Shard %s should have at least one resource", shardID)
		}
	})

	// Test Round Robin Strategy
	suite.T().Run("RoundRobinStrategy", func(t *testing.T) {
		err := suite.loadBalancer.SetStrategy(shardv1.RoundRobinStrategy, shards)
		require.NoError(t, err)

		// Test round robin distribution
		assignments := make([]string, 6)
		for i := 0; i < 6; i++ {
			resource := &interfaces.Resource{ID: fmt.Sprintf("rr-resource-%d", i)}
			shard, err := suite.loadBalancer.AssignResourceToShard(resource, shards)
			require.NoError(t, err)
			assignments[i] = shard.Spec.ShardID
		}

		// Should cycle through shards
		expected := []string{
			"lb-test-shard-1", "lb-test-shard-2", "lb-test-shard-3",
			"lb-test-shard-1", "lb-test-shard-2", "lb-test-shard-3",
		}
		assert.Equal(t, expected, assignments)
	})

	// Test Least Loaded Strategy
	suite.T().Run("LeastLoadedStrategy", func(t *testing.T) {
		err := suite.loadBalancer.SetStrategy(shardv1.LeastLoadedStrategy, shards)
		require.NoError(t, err)

		// Should always pick the least loaded shard (shard-1 with load 0.2)
		for i := 0; i < 5; i++ {
			resource := &interfaces.Resource{ID: fmt.Sprintf("ll-resource-%d", i)}
			shard, err := suite.loadBalancer.AssignResourceToShard(resource, shards)
			require.NoError(t, err)
			assert.Equal(t, "lb-test-shard-1", shard.Spec.ShardID)
		}
	})
}

// TestLoadRebalancing tests load rebalancing scenarios
func (suite *IntegrationTestSuite) TestLoadRebalancing() {
	// Create shards with imbalanced loads
	shard1 := suite.createTestShardInstance("rebalance-shard-1", shardv1.ShardPhaseRunning)
	shard2 := suite.createTestShardInstance("rebalance-shard-2", shardv1.ShardPhaseRunning)
	shard3 := suite.createTestShardInstance("rebalance-shard-3", shardv1.ShardPhaseRunning)

	// Set highly imbalanced loads
	shard1.Status.Load = 0.9
	shard1.Status.AssignedResources = []string{"res1", "res2", "res3", "res4", "res5"}
	shard2.Status.Load = 0.1
	shard2.Status.AssignedResources = []string{}
	shard3.Status.Load = 0.2
	shard3.Status.AssignedResources = []string{"res6"}

	err := suite.k8sClient.Status().Update(suite.ctx, shard1)
	require.NoError(suite.T(), err)
	err = suite.k8sClient.Status().Update(suite.ctx, shard2)
	require.NoError(suite.T(), err)
	err = suite.k8sClient.Status().Update(suite.ctx, shard3)
	require.NoError(suite.T(), err)

	shards := []*shardv1.ShardInstance{shard1, shard2, shard3}

	// Test rebalancing decision
	shouldRebalance := suite.loadBalancer.ShouldRebalance(shards)
	assert.True(suite.T(), shouldRebalance, "Should detect need for rebalancing")

	// Generate rebalance plan
	plan, err := suite.loadBalancer.GenerateRebalancePlan(shards)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "rebalance-shard-1", plan.SourceShard, "Should move from highest loaded shard")
	assert.Contains(suite.T(), []string{"rebalance-shard-2", "rebalance-shard-3"}, plan.TargetShard)
	assert.NotEmpty(suite.T(), plan.Resources, "Should have resources to move")

	// Test rebalancing execution through shard manager
	err = suite.shardManager.RebalanceLoad(suite.ctx)
	require.NoError(suite.T(), err)

	// Wait for rebalancing to take effect
	err = suite.waitForCondition(10*time.Second, func() bool {
		// Refresh shard data
		updatedShard1 := &shardv1.ShardInstance{}
		err := suite.k8sClient.Get(suite.ctx,
			suite.k8sClient.ObjectKeyFromObject(shard1), updatedShard1)
		if err != nil {
			return false
		}

		updatedShard2 := &shardv1.ShardInstance{}
		err = suite.k8sClient.Get(suite.ctx,
			suite.k8sClient.ObjectKeyFromObject(shard2), updatedShard2)
		if err != nil {
			return false
		}

		// Check if load is more balanced
		loadDiff := updatedShard1.Status.Load - updatedShard2.Status.Load
		return loadDiff < 0.5 // Load difference should be reduced
	})
	require.NoError(suite.T(), err)
}

// TestDynamicLoadBalancing tests load balancing with changing conditions
func (suite *IntegrationTestSuite) TestDynamicLoadBalancing() {
	// Create initial shards
	shards := make([]*shardv1.ShardInstance, 2)
	for i := 0; i < 2; i++ {
		shardID := fmt.Sprintf("dynamic-shard-%d", i+1)
		shards[i] = suite.createTestShardInstance(shardID, shardv1.ShardPhaseRunning)
		shards[i].Status.Load = 0.3
		err := suite.k8sClient.Status().Update(suite.ctx, shards[i])
		require.NoError(suite.T(), err)
	}

	// Start with least loaded strategy
	err := suite.loadBalancer.SetStrategy(shardv1.LeastLoadedStrategy, shards)
	require.NoError(suite.T(), err)

	// Simulate load increase on one shard
	shards[0].Status.Load = 0.8
	err = suite.k8sClient.Status().Update(suite.ctx, shards[0])
	require.NoError(suite.T(), err)

	// New resources should go to less loaded shard
	resource := &interfaces.Resource{ID: "dynamic-resource-1"}
	selectedShard, err := suite.loadBalancer.GetOptimalShard(shards)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "dynamic-shard-2", selectedShard.Spec.ShardID)

	// Add a new shard
	newShard := suite.createTestShardInstance("dynamic-shard-3", shardv1.ShardPhaseRunning)
	newShard.Status.Load = 0.1
	err = suite.k8sClient.Status().Update(suite.ctx, newShard)
	require.NoError(suite.T(), err)

	updatedShards := []*shardv1.ShardInstance{shards[0], shards[1], newShard}
	err = suite.loadBalancer.UpdateShardNodes(updatedShards)
	require.NoError(suite.T(), err)

	// New shard should be selected for new resources
	selectedShard, err = suite.loadBalancer.GetOptimalShard(updatedShards)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "dynamic-shard-3", selectedShard.Spec.ShardID)

	// Test strategy change
	err = suite.loadBalancer.SetStrategy(shardv1.ConsistentHashStrategy, updatedShards)
	require.NoError(suite.T(), err)

	// Verify strategy changed
	assert.Equal(suite.T(), shardv1.ConsistentHashStrategy, suite.loadBalancer.GetStrategy())
}

// TestLoadBalancingWithFailures tests load balancing when shards fail
func (suite *IntegrationTestSuite) TestLoadBalancingWithFailures() {
	// Create shards
	healthyShard := suite.createTestShardInstance("healthy-lb-shard", shardv1.ShardPhaseRunning)
	failedShard := suite.createTestShardInstance("failed-lb-shard", shardv1.ShardPhaseFailed)

	// Set health status
	healthyShard.Status.HealthStatus = &shardv1.HealthStatus{Healthy: true}
	failedShard.Status.HealthStatus = &shardv1.HealthStatus{Healthy: false}

	err := suite.k8sClient.Status().Update(suite.ctx, healthyShard)
	require.NoError(suite.T(), err)
	err = suite.k8sClient.Status().Update(suite.ctx, failedShard)
	require.NoError(suite.T(), err)

	shards := []*shardv1.ShardInstance{healthyShard, failedShard}

	// Test that failed shards are not selected
	for _, strategy := range []shardv1.LoadBalanceStrategy{
		shardv1.LeastLoadedStrategy,
		shardv1.RoundRobinStrategy,
		shardv1.ConsistentHashStrategy,
	} {
		suite.T().Run(string(strategy), func(t *testing.T) {
			err := suite.loadBalancer.SetStrategy(strategy, shards)
			require.NoError(t, err)

			// All resources should go to healthy shard
			for i := 0; i < 5; i++ {
				resource := &interfaces.Resource{ID: fmt.Sprintf("failure-test-resource-%d", i)}
				selectedShard, err := suite.loadBalancer.AssignResourceToShard(resource, shards)
				require.NoError(t, err)
				assert.Equal(t, "healthy-lb-shard", selectedShard.Spec.ShardID)
			}
		})
	}

	// Test with no healthy shards
	healthyShard.Status.Phase = shardv1.ShardPhaseFailed
	healthyShard.Status.HealthStatus = &shardv1.HealthStatus{Healthy: false}
	err = suite.k8sClient.Status().Update(suite.ctx, healthyShard)
	require.NoError(suite.T(), err)

	resource := &interfaces.Resource{ID: "no-healthy-resource"}
	_, err = suite.loadBalancer.AssignResourceToShard(resource, shards)
	assert.Error(suite.T(), err, "Should error when no healthy shards available")
}

// TestLoadDistribution tests load distribution metrics
func (suite *IntegrationTestSuite) TestLoadDistribution() {
	// Create shards with known loads
	loads := []float64{0.2, 0.5, 0.8}
	shards := make([]*shardv1.ShardInstance, len(loads))

	for i, load := range loads {
		shardID := fmt.Sprintf("dist-shard-%d", i+1)
		shards[i] = suite.createTestShardInstance(shardID, shardv1.ShardPhaseRunning)
		shards[i].Status.Load = load
		err := suite.k8sClient.Status().Update(suite.ctx, shards[i])
		require.NoError(suite.T(), err)
	}

	// Get load distribution
	distribution := suite.loadBalancer.GetLoadDistribution(shards)

	// Verify distribution
	assert.Len(suite.T(), distribution, 3)
	assert.Equal(suite.T(), 0.2, distribution["dist-shard-1"])
	assert.Equal(suite.T(), 0.5, distribution["dist-shard-2"])
	assert.Equal(suite.T(), 0.8, distribution["dist-shard-3"])

	// Test rebalance score calculation
	score := suite.loadBalancer.CalculateRebalanceScore(shards)
	assert.Greater(suite.T(), score, 0.0, "Should have positive rebalance score for imbalanced shards")

	// Create balanced shards
	for i := range shards {
		shards[i].Status.Load = 0.5
		err := suite.k8sClient.Status().Update(suite.ctx, shards[i])
		require.NoError(suite.T(), err)
	}

	balancedScore := suite.loadBalancer.CalculateRebalanceScore(shards)
	assert.Less(suite.T(), balancedScore, score, "Balanced shards should have lower rebalance score")
}
