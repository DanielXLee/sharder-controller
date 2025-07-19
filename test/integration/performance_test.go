package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

// TestShardCreationPerformance tests shard creation speed
func (suite *IntegrationTestSuite) TestShardCreationPerformance() {
	config := suite.createTestShardConfig()
	
	// Test single shard creation time
	start := time.Now()
	shard, err := suite.shardManager.CreateShard(suite.ctx, &config.Spec)
	require.NoError(suite.T(), err)
	singleCreationTime := time.Since(start)
	
	suite.T().Logf("Single shard creation time: %v", singleCreationTime)
	assert.Less(suite.T(), singleCreationTime, 5*time.Second, "Single shard creation should be fast")

	// Clean up
	err = suite.shardManager.DeleteShard(suite.ctx, shard.Spec.ShardID)
	require.NoError(suite.T(), err)

	// Test batch shard creation
	shardCount := 10
	start = time.Now()
	
	var wg sync.WaitGroup
	errors := make(chan error, shardCount)
	
	for i := 0; i < shardCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := suite.shardManager.CreateShard(suite.ctx, &config.Spec)
			if err != nil {
				errors <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	batchCreationTime := time.Since(start)
	suite.T().Logf("Batch creation of %d shards time: %v", shardCount, batchCreationTime)
	
	// Check for errors
	for err := range errors {
		require.NoError(suite.T(), err)
	}
	
	// Verify all shards were created
	err = suite.waitForCondition(10*time.Second, func() bool {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		return err == nil && len(shardList.Items) == shardCount
	})
	require.NoError(suite.T(), err)
	
	// Average creation time should be reasonable
	avgCreationTime := batchCreationTime / time.Duration(shardCount)
	suite.T().Logf("Average shard creation time: %v", avgCreationTime)
	assert.Less(suite.T(), avgCreationTime, 2*time.Second, "Average shard creation should be efficient")
}

// TestLoadBalancingPerformance tests load balancing performance
func (suite *IntegrationTestSuite) TestLoadBalancingPerformance() {
	// Create multiple shards
	shardCount := 20
	shards := make([]*shardv1.ShardInstance, shardCount)
	
	for i := 0; i < shardCount; i++ {
		shardID := fmt.Sprintf("perf-shard-%d", i)
		shards[i] = suite.createTestShardInstance(shardID, shardv1.ShardPhaseRunning)
		shards[i].Status.Load = float64(i%5) * 0.2 // Varying loads
		err := suite.k8sClient.Status().Update(suite.ctx, shards[i])
		require.NoError(suite.T(), err)
	}

	// Test different strategies
	strategies := []shardv1.LoadBalanceStrategy{
		shardv1.ConsistentHashStrategy,
		shardv1.RoundRobinStrategy,
		shardv1.LeastLoadedStrategy,
	}

	resourceCount := 1000

	for _, strategy := range strategies {
		suite.T().Run(string(strategy), func(t *testing.T) {
			err := suite.loadBalancer.SetStrategy(strategy, shards)
			require.NoError(t, err)

			// Test resource assignment performance
			start := time.Now()
			
			for i := 0; i < resourceCount; i++ {
				resource := &interfaces.Resource{ID: fmt.Sprintf("perf-resource-%d", i)}
				_, err := suite.loadBalancer.AssignResourceToShard(resource, shards)
				require.NoError(t, err)
			}
			
			assignmentTime := time.Since(start)
			avgAssignmentTime := assignmentTime / time.Duration(resourceCount)
			
			t.Logf("Strategy %s: %d assignments in %v (avg: %v)", 
				strategy, resourceCount, assignmentTime, avgAssignmentTime)
			
			// Should be able to assign resources quickly
			assert.Less(t, avgAssignmentTime, 1*time.Millisecond, 
				"Resource assignment should be fast")
		})
	}
}

// TestHealthCheckingPerformance tests health checking performance
func (suite *IntegrationTestSuite) TestHealthCheckingPerformance() {
	// Create many shards
	shardCount := 50
	for i := 0; i < shardCount; i++ {
		shardID := fmt.Sprintf("health-perf-shard-%d", i)
		shard := suite.createTestShardInstance(shardID, shardv1.ShardPhaseRunning)
		
		// Mix of healthy and unhealthy shards
		if i%5 == 0 {
			shard.Status.LastHeartbeat = metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
		}
		err := suite.k8sClient.Status().Update(suite.ctx, shard)
		require.NoError(suite.T(), err)
	}

	// Test health checking performance
	start := time.Now()
	
	shardList := &shardv1.ShardInstanceList{}
	err := suite.k8sClient.List(suite.ctx, shardList)
	require.NoError(suite.T(), err)

	var wg sync.WaitGroup
	for _, shard := range shardList.Items {
		wg.Add(1)
		go func(s shardv1.ShardInstance) {
			defer wg.Done()
			_, err := suite.healthChecker.CheckHealth(suite.ctx, &s)
			require.NoError(suite.T(), err)
		}(shard)
	}
	
	wg.Wait()
	healthCheckTime := time.Since(start)
	
	avgHealthCheckTime := healthCheckTime / time.Duration(shardCount)
	suite.T().Logf("Health check for %d shards: %v (avg: %v)", 
		shardCount, healthCheckTime, avgHealthCheckTime)
	
	assert.Less(suite.T(), avgHealthCheckTime, 100*time.Millisecond, 
		"Health check should be fast")

	// Test continuous health checking performance
	start = time.Now()
	err = suite.healthChecker.StartHealthChecking(suite.ctx, 500*time.Millisecond)
	require.NoError(suite.T(), err)
	
	// Let it run for a few cycles
	time.Sleep(3 * time.Second)
	
	err = suite.healthChecker.StopHealthChecking()
	require.NoError(suite.T(), err)
	
	continuousTime := time.Since(start)
	suite.T().Logf("Continuous health checking for %v", continuousTime)
	
	// Verify health status was updated
	summary := suite.healthChecker.GetHealthSummary()
	assert.Len(suite.T(), summary, shardCount, "All shards should have health status")
}

// TestResourceMigrationPerformance tests migration performance
func (suite *IntegrationTestSuite) TestResourceMigrationPerformance() {
	// Create source and target shards
	sourceShard := suite.createTestShardInstance("migration-source", shardv1.ShardPhaseRunning)
	targetShard := suite.createTestShardInstance("migration-target", shardv1.ShardPhaseRunning)

	// Create many resources on source shard
	resourceCount := 100
	resources := make([]string, resourceCount)
	for i := 0; i < resourceCount; i++ {
		resources[i] = fmt.Sprintf("migration-resource-%d", i)
	}
	
	sourceShard.Status.AssignedResources = resources
	err := suite.k8sClient.Status().Update(suite.ctx, sourceShard)
	require.NoError(suite.T(), err)

	// Test migration performance
	plan := &shardv1.MigrationPlan{
		SourceShard:   "migration-source",
		TargetShard:   "migration-target",
		Resources:     resources,
		EstimatedTime: metav1.Duration{Duration: 30 * time.Second},
		Priority:      shardv1.MigrationPriorityHigh,
	}

	start := time.Now()
	err = suite.resourceMigrator.ExecuteMigration(suite.ctx, plan)
	require.NoError(suite.T(), err)
	migrationTime := time.Since(start)

	suite.T().Logf("Migration of %d resources: %v", resourceCount, migrationTime)
	
	// Migration should complete within reasonable time
	assert.Less(suite.T(), migrationTime, 10*time.Second, 
		"Migration should complete quickly")

	// Verify migration completed
	err = suite.waitForCondition(5*time.Second, func() bool {
		updatedTarget := &shardv1.ShardInstance{}
		err := suite.k8sClient.Get(suite.ctx, 
			suite.k8sClient.ObjectKeyFromObject(targetShard), updatedTarget)
		return err == nil && len(updatedTarget.Status.AssignedResources) == resourceCount
	})
	require.NoError(suite.T(), err)
}

// TestScalingPerformance tests scaling performance under load
func (suite *IntegrationTestSuite) TestScalingPerformance() {
	config := suite.createTestShardConfig()

	// Test scale up performance
	scaleUpSizes := []int{2, 5, 10, 15}
	
	for _, targetSize := range scaleUpSizes {
		suite.T().Run(fmt.Sprintf("ScaleUp_%d", targetSize), func(t *testing.T) {
			// Clean up first
			suite.cleanupTestResources()
			
			start := time.Now()
			err := suite.shardManager.ScaleUp(suite.ctx, targetSize)
			require.NoError(t, err)
			
			// Wait for scale up to complete
			err = suite.waitForCondition(30*time.Second, func() bool {
				shardList := &shardv1.ShardInstanceList{}
				err := suite.k8sClient.List(suite.ctx, shardList)
				return err == nil && len(shardList.Items) == targetSize
			})
			require.NoError(t, err)
			
			scaleUpTime := time.Since(start)
			t.Logf("Scale up to %d shards: %v", targetSize, scaleUpTime)
			
			// Scale up should complete within reasonable time
			expectedTime := time.Duration(targetSize) * 2 * time.Second
			assert.Less(t, scaleUpTime, expectedTime, 
				"Scale up should complete within expected time")
		})
	}

	// Test scale down performance
	suite.T().Run("ScaleDown", func(t *testing.T) {
		// Start with 10 shards
		err := suite.shardManager.ScaleUp(suite.ctx, 10)
		require.NoError(t, err)
		
		err = suite.waitForCondition(20*time.Second, func() bool {
			shardList := &shardv1.ShardInstanceList{}
			err := suite.k8sClient.List(suite.ctx, shardList)
			return err == nil && len(shardList.Items) == 10
		})
		require.NoError(t, err)

		// Scale down to 3
		start := time.Now()
		err = suite.shardManager.ScaleDown(suite.ctx, 3)
		require.NoError(t, err)
		
		// Wait for scale down to complete
		err = suite.waitForCondition(30*time.Second, func() bool {
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
			return runningCount == 3
		})
		require.NoError(t, err)
		
		scaleDownTime := time.Since(start)
		t.Logf("Scale down from 10 to 3 shards: %v", scaleDownTime)
		
		// Scale down should complete within reasonable time
		assert.Less(t, scaleDownTime, 20*time.Second, 
			"Scale down should complete within expected time")
	})
}

// TestConcurrentOperations tests system performance under concurrent operations
func (suite *IntegrationTestSuite) TestConcurrentOperations() {
	config := suite.createTestShardConfig()

	// Create initial shards
	err := suite.shardManager.ScaleUp(suite.ctx, 5)
	require.NoError(suite.T(), err)
	
	err = suite.waitForCondition(15*time.Second, func() bool {
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		return err == nil && len(shardList.Items) == 5
	})
	require.NoError(suite.T(), err)

	// Start health checking
	err = suite.healthChecker.StartHealthChecking(suite.ctx, 1*time.Second)
	require.NoError(suite.T(), err)
	defer suite.healthChecker.StopHealthChecking()

	// Concurrent operations
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent resource assignments
	wg.Add(1)
	go func() {
		defer wg.Done()
		shardList := &shardv1.ShardInstanceList{}
		err := suite.k8sClient.List(suite.ctx, shardList)
		if err != nil {
			errors <- err
			return
		}
		
		shards := make([]*shardv1.ShardInstance, len(shardList.Items))
		for i := range shardList.Items {
			shards[i] = &shardList.Items[i]
		}

		for i := 0; i < 50; i++ {
			resource := &interfaces.Resource{ID: fmt.Sprintf("concurrent-resource-%d", i)}
			_, err := suite.loadBalancer.AssignResourceToShard(resource, shards)
			if err != nil {
				errors <- err
				return
			}
		}
	}()

	// Concurrent health checks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			shardList := &shardv1.ShardInstanceList{}
			err := suite.k8sClient.List(suite.ctx, shardList)
			if err != nil {
				errors <- err
				return
			}
			
			for _, shard := range shardList.Items {
				_, err := suite.healthChecker.CheckHealth(suite.ctx, &shard)
				if err != nil {
					errors <- err
					return
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Concurrent load updates
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			shardList := &shardv1.ShardInstanceList{}
			err := suite.k8sClient.List(suite.ctx, shardList)
			if err != nil {
				errors <- err
				return
			}
			
			if len(shardList.Items) > 0 {
				shard := &shardList.Items[i%len(shardList.Items)]
				shard.Status.Load = float64(i%10) * 0.1
				err = suite.k8sClient.Status().Update(suite.ctx, shard)
				if err != nil {
					errors <- err
					return
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	// Wait for all operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All operations completed
	case <-time.After(30 * time.Second):
		suite.T().Fatal("Concurrent operations timed out")
	}

	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		suite.T().Logf("Concurrent operation error: %v", err)
	}

	// Allow some errors due to concurrent access, but not too many
	assert.Less(suite.T(), errorCount, 10, "Should have minimal errors during concurrent operations")
}