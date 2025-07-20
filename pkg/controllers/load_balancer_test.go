package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
)

func TestNewLoadBalancer(t *testing.T) {
	tests := []struct {
		name     string
		strategy shardv1.LoadBalanceStrategy
		shards   []*shardv1.ShardInstance
		wantErr  bool
	}{
		{
			name:     "consistent hash strategy",
			strategy: shardv1.ConsistentHashStrategy,
			shards:   nil,
			wantErr:  false,
		},
		{
			name:     "round robin strategy",
			strategy: shardv1.RoundRobinStrategy,
			shards:   nil,
			wantErr:  false,
		},
		{
			name:     "least loaded strategy",
			strategy: shardv1.LeastLoadedStrategy,
			shards:   nil,
			wantErr:  false,
		},
		{
			name:     "unsupported strategy",
			strategy: "unsupported-strategy",
			shards:   nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb, err := NewLoadBalancer(tt.strategy, tt.shards)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, lb)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, lb)
				assert.Equal(t, tt.strategy, lb.GetStrategy())
			}
		})
	}
}

func TestCalculateShardLoad(t *testing.T) {
	lb, _ := NewLoadBalancer(shardv1.LeastLoadedStrategy, nil)

	tests := []struct {
		name     string
		shard    *shardv1.ShardInstance
		expected float64
	}{
		{
			name:     "nil shard",
			shard:    nil,
			expected: 0,
		},
		{
			name: "unhealthy shard",
			shard: &shardv1.ShardInstance{
				Status: shardv1.ShardInstanceStatus{
					Phase:        shardv1.ShardPhaseRunning,
					HealthStatus: &shardv1.HealthStatus{Healthy: false},
				},
			},
			expected: 1.0,
		},
		{
			name: "non-running shard",
			shard: &shardv1.ShardInstance{
				Status: shardv1.ShardInstanceStatus{
					Phase:        shardv1.ShardPhasePending,
					HealthStatus: &shardv1.HealthStatus{Healthy: true},
				},
			},
			expected: 1.0,
		},
		{
			name: "shard with explicit load",
			shard: &shardv1.ShardInstance{
				Status: shardv1.ShardInstanceStatus{
					Phase:        shardv1.ShardPhaseRunning,
					HealthStatus: &shardv1.HealthStatus{Healthy: true},
					Load:         0.5,
				},
			},
			expected: 0.5,
		},
		{
			name: "shard with resources",
			shard: &shardv1.ShardInstance{
				Status: shardv1.ShardInstanceStatus{
					Phase:             shardv1.ShardPhaseRunning,
					HealthStatus:      &shardv1.HealthStatus{Healthy: true},
					AssignedResources: []string{"res1", "res2", "res3"},
				},
			},
			expected: 0.003,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			load := lb.CalculateShardLoad(tt.shard)
			assert.Equal(t, tt.expected, load)
		})
	}
}

func TestGetOptimalShard(t *testing.T) {
	shards := []*shardv1.ShardInstance{
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard1"},
			Status: shardv1.ShardInstanceStatus{
				Phase:        shardv1.ShardPhaseRunning,
				HealthStatus: &shardv1.HealthStatus{Healthy: true},
				Load:         0.1,
			},
		},
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard2"},
			Status: shardv1.ShardInstanceStatus{
				Phase:        shardv1.ShardPhaseRunning,
				HealthStatus: &shardv1.HealthStatus{Healthy: true},
				Load:         0.5,
			},
		},
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard3"},
			Status: shardv1.ShardInstanceStatus{
				Phase:        shardv1.ShardPhaseFailed,
				HealthStatus: &shardv1.HealthStatus{Healthy: false},
				Load:         0.2,
			},
		},
	}

	tests := []struct {
		name     string
		strategy shardv1.LoadBalanceStrategy
		shards   []*shardv1.ShardInstance
		wantErr  bool
	}{
		{
			name:     "least loaded strategy",
			strategy: shardv1.LeastLoadedStrategy,
			shards:   shards,
			wantErr:  false,
		},
		{
			name:     "round robin strategy",
			strategy: shardv1.RoundRobinStrategy,
			shards:   shards,
			wantErr:  false,
		},
		{
			name:     "consistent hash strategy",
			strategy: shardv1.ConsistentHashStrategy,
			shards:   shards,
			wantErr:  false,
		},
		{
			name:     "no shards",
			strategy: shardv1.LeastLoadedStrategy,
			shards:   []*shardv1.ShardInstance{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb, err := NewLoadBalancer(tt.strategy, tt.shards)
			require.NoError(t, err)

			optimalShard, err := lb.GetOptimalShard(tt.shards)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, optimalShard)
				// For least loaded, should return shard1 (lowest load)
				if tt.strategy == shardv1.LeastLoadedStrategy {
					assert.Equal(t, "shard1", optimalShard.Spec.ShardID)
				}
			}
		})
	}
}

func TestAssignResourceToShard(t *testing.T) {
	shards := []*shardv1.ShardInstance{
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard1"},
			Status: shardv1.ShardInstanceStatus{
				Phase:        shardv1.ShardPhaseRunning,
				HealthStatus: &shardv1.HealthStatus{Healthy: true},
			},
		},
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard2"},
			Status: shardv1.ShardInstanceStatus{
				Phase:        shardv1.ShardPhaseRunning,
				HealthStatus: &shardv1.HealthStatus{Healthy: true},
			},
		},
	}

	resource := &interfaces.Resource{ID: "test-resource"}

	t.Run("consistent hash strategy", func(t *testing.T) {
		lb, err := NewLoadBalancer(shardv1.ConsistentHashStrategy, shards)
		require.NoError(t, err)

		shard, err := lb.AssignResourceToShard(resource, shards)
		assert.NoError(t, err)
		assert.NotEmpty(t, shard.Spec.ShardID)

		// Same resource should always go to the same shard
		shard2, err := lb.AssignResourceToShard(resource, shards)
		assert.NoError(t, err)
		assert.Equal(t, shard.Spec.ShardID, shard2.Spec.ShardID)
	})

	t.Run("round robin strategy", func(t *testing.T) {
		lb, err := NewLoadBalancer(shardv1.RoundRobinStrategy, shards)
		require.NoError(t, err)

		shard1, err := lb.AssignResourceToShard(resource, shards)
		assert.NoError(t, err)
		assert.Equal(t, "shard1", shard1.Spec.ShardID)

		shard2, err := lb.AssignResourceToShard(resource, shards)
		assert.NoError(t, err)
		assert.Equal(t, "shard2", shard2.Spec.ShardID)

		// Should wrap around
		shard3, err := lb.AssignResourceToShard(resource, shards)
		assert.NoError(t, err)
		assert.Equal(t, "shard1", shard3.Spec.ShardID)
	})

	t.Run("least loaded strategy", func(t *testing.T) {
		// Set different loads
		shards[0].Status.Load = 0.8
		shards[1].Status.Load = 0.2

		lb, err := NewLoadBalancer(shardv1.LeastLoadedStrategy, shards)
		require.NoError(t, err)

		shard, err := lb.AssignResourceToShard(resource, shards)
		assert.NoError(t, err)
		assert.Equal(t, "shard2", shard.Spec.ShardID) // Should pick the less loaded one
	})

	t.Run("nil resource", func(t *testing.T) {
		lb, err := NewLoadBalancer(shardv1.LeastLoadedStrategy, shards)
		require.NoError(t, err)

		_, err = lb.AssignResourceToShard(nil, shards)
		assert.Error(t, err)
	})

	t.Run("no healthy shards", func(t *testing.T) {
		unhealthyShards := []*shardv1.ShardInstance{
			{
				Spec: shardv1.ShardInstanceSpec{ShardID: "shard1"},
				Status: shardv1.ShardInstanceStatus{
					Phase:        shardv1.ShardPhaseFailed,
					HealthStatus: &shardv1.HealthStatus{Healthy: false},
				},
			},
		}

		lb, err := NewLoadBalancer(shardv1.LeastLoadedStrategy, unhealthyShards)
		require.NoError(t, err)

		_, err = lb.AssignResourceToShard(resource, unhealthyShards)
		assert.Error(t, err)
	})
}

func TestShouldRebalance(t *testing.T) {
	lb, _ := NewLoadBalancer(shardv1.LeastLoadedStrategy, nil)

	tests := []struct {
		name     string
		shards   []*shardv1.ShardInstance
		expected bool
	}{
		{
			name:     "single shard",
			shards:   []*shardv1.ShardInstance{{Status: shardv1.ShardInstanceStatus{Load: 0.5}}},
			expected: false,
		},
		{
			name: "balanced shards",
			shards: []*shardv1.ShardInstance{
				{Status: shardv1.ShardInstanceStatus{Load: 0.4}},
				{Status: shardv1.ShardInstanceStatus{Load: 0.5}},
			},
			expected: false,
		},
		{
			name: "imbalanced shards",
			shards: []*shardv1.ShardInstance{
				{
					Status: shardv1.ShardInstanceStatus{
						Phase:        shardv1.ShardPhaseRunning,
						HealthStatus: &shardv1.HealthStatus{Healthy: true},
						Load:         0.1,
					},
				},
				{
					Status: shardv1.ShardInstanceStatus{
						Phase:        shardv1.ShardPhaseRunning,
						HealthStatus: &shardv1.HealthStatus{Healthy: true},
						Load:         0.8,
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lb.ShouldRebalance(tt.shards)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateRebalancePlan(t *testing.T) {
	lb, _ := NewLoadBalancer(shardv1.LeastLoadedStrategy, nil)

	t.Run("successful rebalance plan", func(t *testing.T) {
		shards := []*shardv1.ShardInstance{
			{
				Spec: shardv1.ShardInstanceSpec{ShardID: "shard1"},
				Status: shardv1.ShardInstanceStatus{
					Phase:             shardv1.ShardPhaseRunning,
					HealthStatus:      &shardv1.HealthStatus{Healthy: true},
					Load:              0.8,
					AssignedResources: []string{"res1", "res2", "res3", "res4", "res5"},
				},
			},
			{
				Spec: shardv1.ShardInstanceSpec{ShardID: "shard2"},
				Status: shardv1.ShardInstanceStatus{
					Phase:        shardv1.ShardPhaseRunning,
					HealthStatus: &shardv1.HealthStatus{Healthy: true},
					Load:         0.1,
				},
			},
		}

		plan, err := lb.GenerateRebalancePlan(shards)
		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, "shard1", plan.SourceShard)
		assert.Equal(t, "shard2", plan.TargetShard)
		assert.NotEmpty(t, plan.Resources)
	})

	t.Run("no rebalance needed", func(t *testing.T) {
		shards := []*shardv1.ShardInstance{
			{
				Spec:   shardv1.ShardInstanceSpec{ShardID: "shard1"},
				Status: shardv1.ShardInstanceStatus{Load: 0.4},
			},
			{
				Spec:   shardv1.ShardInstanceSpec{ShardID: "shard2"},
				Status: shardv1.ShardInstanceStatus{Load: 0.5},
			},
		}

		_, err := lb.GenerateRebalancePlan(shards)
		assert.Error(t, err)
	})
}

func TestUpdateShardNodes(t *testing.T) {
	t.Run("consistent hash strategy", func(t *testing.T) {
		lb, err := NewLoadBalancer(shardv1.ConsistentHashStrategy, nil)
		require.NoError(t, err)

		shards := []*shardv1.ShardInstance{
			{
				Spec: shardv1.ShardInstanceSpec{ShardID: "shard1"},
				Status: shardv1.ShardInstanceStatus{
					Phase: shardv1.ShardPhaseRunning,
				},
			},
			{
				Spec: shardv1.ShardInstanceSpec{ShardID: "shard2"},
				Status: shardv1.ShardInstanceStatus{
					Phase: shardv1.ShardPhaseRunning,
				},
			},
		}

		err = lb.UpdateShardNodes(shards)
		assert.NoError(t, err)

		// Verify nodes were added
		nodes := lb.consistentHash.GetAllNodes()
		assert.Len(t, nodes, 2)
		assert.Contains(t, nodes, "shard1")
		assert.Contains(t, nodes, "shard2")
	})

	t.Run("non-consistent hash strategy", func(t *testing.T) {
		lb, err := NewLoadBalancer(shardv1.RoundRobinStrategy, nil)
		require.NoError(t, err)

		shards := []*shardv1.ShardInstance{
			{Spec: shardv1.ShardInstanceSpec{ShardID: "shard1"}},
		}

		err = lb.UpdateShardNodes(shards)
		assert.NoError(t, err) // Should not error, but also should not do anything
	})
}

func TestGetLoadDistribution(t *testing.T) {
	lb, _ := NewLoadBalancer(shardv1.LeastLoadedStrategy, nil)

	shards := []*shardv1.ShardInstance{
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard1"},
			Status: shardv1.ShardInstanceStatus{
				Phase:        shardv1.ShardPhaseRunning,
				HealthStatus: &shardv1.HealthStatus{Healthy: true},
				Load:         0.3,
			},
		},
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard2"},
			Status: shardv1.ShardInstanceStatus{
				Phase:        shardv1.ShardPhaseRunning,
				HealthStatus: &shardv1.HealthStatus{Healthy: true},
				Load:         0.7,
			},
		},
	}

	distribution := lb.GetLoadDistribution(shards)
	assert.Len(t, distribution, 2)
	assert.Equal(t, 0.3, distribution["shard1"])
	assert.Equal(t, 0.7, distribution["shard2"])
}

func TestSetStrategy(t *testing.T) {
	lb, err := NewLoadBalancer(shardv1.RoundRobinStrategy, nil)
	require.NoError(t, err)

	shards := []*shardv1.ShardInstance{
		{
			Spec: shardv1.ShardInstanceSpec{ShardID: "shard1"},
			Status: shardv1.ShardInstanceStatus{
				Phase: shardv1.ShardPhaseRunning,
			},
		},
	}

	// Change to consistent hash
	lb.SetStrategy(shardv1.ConsistentHashStrategy, shards)
	assert.Equal(t, shardv1.ConsistentHashStrategy, lb.GetStrategy())
	assert.NotNil(t, lb.consistentHash)

	// Change to least loaded
	lb.SetStrategy(shardv1.LeastLoadedStrategy, shards)
	assert.Equal(t, shardv1.LeastLoadedStrategy, lb.GetStrategy())
	assert.Nil(t, lb.consistentHash)

	// Invalid strategy - should keep current strategy
	lb.SetStrategy("invalid", shards)
	assert.Equal(t, shardv1.LeastLoadedStrategy, lb.GetStrategy()) // Should remain unchanged
}

func TestCalculateRebalanceScore(t *testing.T) {
	lb, _ := NewLoadBalancer(shardv1.LeastLoadedStrategy, nil)

	tests := []struct {
		name     string
		shards   []*shardv1.ShardInstance
		expected float64
	}{
		{
			name:     "single shard",
			shards:   []*shardv1.ShardInstance{{Status: shardv1.ShardInstanceStatus{Phase: shardv1.ShardPhaseRunning, HealthStatus: &shardv1.HealthStatus{Healthy: true}, Load: 0.5}}},
			expected: 0.0,
		},
		{
			name: "balanced shards",
			shards: []*shardv1.ShardInstance{
				{Status: shardv1.ShardInstanceStatus{Phase: shardv1.ShardPhaseRunning, HealthStatus: &shardv1.HealthStatus{Healthy: true}, Load: 0.5}},
				{Status: shardv1.ShardInstanceStatus{Phase: shardv1.ShardPhaseRunning, HealthStatus: &shardv1.HealthStatus{Healthy: true}, Load: 0.5}},
			},
			expected: 0.0,
		},
		{
			name: "imbalanced shards",
			shards: []*shardv1.ShardInstance{
				{Status: shardv1.ShardInstanceStatus{Phase: shardv1.ShardPhaseRunning, HealthStatus: &shardv1.HealthStatus{Healthy: true}, Load: 0.1}},
				{Status: shardv1.ShardInstanceStatus{Phase: shardv1.ShardPhaseRunning, HealthStatus: &shardv1.HealthStatus{Healthy: true}, Load: 0.9}},
			},
			expected: 0.16, // (0.4-0.5)^2 + (0.4-0.5)^2 / 2 = 0.16
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := lb.CalculateRebalanceScore(tt.shards)
			assert.InDelta(t, tt.expected, score, 0.01)
		})
	}
}
