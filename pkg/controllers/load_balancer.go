package controllers

import (
	"fmt"
	"sort"
	"sync"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/interfaces"
	"github.com/k8s-shard-controller/pkg/utils"
)

// LoadBalancer implements the LoadBalancer interface
type LoadBalancer struct {
	strategy shardv1.LoadBalanceStrategy

	// For consistent hash strategy
	consistentHash *utils.ConsistentHash

	// For round robin strategy
	mu             sync.Mutex
	lastShardIndex int
}

// NewLoadBalancer creates a new LoadBalancer
func NewLoadBalancer(strategy shardv1.LoadBalanceStrategy, shards []*shardv1.ShardInstance) (*LoadBalancer, error) {
	lb := &LoadBalancer{
		strategy: strategy,
	}

	// Initialize strategy-specific components
	switch strategy {
	case shardv1.ConsistentHashStrategy:
		lb.consistentHash = utils.NewConsistentHash(10) // 10 virtual nodes per real node
		if shards != nil {
			for _, shard := range shards {
				lb.consistentHash.AddNode(shard.Spec.ShardID)
			}
		}
	case shardv1.RoundRobinStrategy:
		lb.lastShardIndex = -1
	case shardv1.LeastLoadedStrategy:
		// No special initialization needed
	default:
		return nil, fmt.Errorf("unsupported load balance strategy: %s", strategy)
	}

	return lb, nil
}

// CalculateShardLoad calculates the load for a shard using multiple metrics
func (lb *LoadBalancer) CalculateShardLoad(shard *shardv1.ShardInstance) float64 {
	if shard == nil {
		return 0
	}

	// If shard is not healthy or not running, return maximum load to avoid assignment
	if !utils.IsShardHealthy(&shard.Status) || shard.Status.Phase != shardv1.ShardPhaseRunning {
		return 1.0
	}

	// If we have explicit load value, use it
	if shard.Status.Load > 0 {
		return shard.Status.Load
	}

	// Calculate load based on assigned resources
	resourceCount := len(shard.Status.AssignedResources)
	resourceLoad := float64(resourceCount) / 1000.0 // Normalize assuming max 1000 resources
	if resourceLoad > 1.0 {
		resourceLoad = 1.0
	}

	return resourceLoad
}

// GetOptimalShard returns the optimal shard for a new resource
func (lb *LoadBalancer) GetOptimalShard(shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards available")
	}

	// Filter out unhealthy shards
	healthyShards := make([]*shardv1.ShardInstance, 0, len(shards))
	for _, shard := range shards {
		if utils.IsShardHealthy(&shard.Status) {
			healthyShards = append(healthyShards, shard)
		}
	}

	if len(healthyShards) == 0 {
		return nil, fmt.Errorf("no healthy shards available")
	}

	// If only one shard, return it
	if len(healthyShards) == 1 {
		return healthyShards[0], nil
	}

	// Use the appropriate strategy
	switch lb.strategy {
	case shardv1.ConsistentHashStrategy:
		return lb.getOptimalShardConsistentHash(healthyShards)
	case shardv1.RoundRobinStrategy:
		return lb.getOptimalShardRoundRobin(healthyShards)
	case shardv1.LeastLoadedStrategy:
		return lb.getOptimalShardLeastLoaded(healthyShards)
	default:
		return nil, fmt.Errorf("unsupported load balance strategy: %s", lb.strategy)
	}
}

// getOptimalShardConsistentHash returns the optimal shard using consistent hashing
func (lb *LoadBalancer) getOptimalShardConsistentHash(shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	// When no specific resource key is provided, consistent hashing is not the ideal strategy
	// for picking a generic optimal shard. In this scenario, falling back to the least loaded
	// strategy is a more sensible approach to ensure balanced distribution.
	return lb.getOptimalShardLeastLoaded(shards)
}

// getOptimalShardRoundRobin returns the optimal shard using round robin
func (lb *LoadBalancer) getOptimalShardRoundRobin(shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.lastShardIndex = (lb.lastShardIndex + 1) % len(shards)
	return shards[lb.lastShardIndex], nil
}

// getOptimalShardLeastLoaded returns the optimal shard using least loaded
func (lb *LoadBalancer) getOptimalShardLeastLoaded(shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	// Sort shards by load
	sort.Slice(shards, func(i, j int) bool {
		loadI := lb.CalculateShardLoad(shards[i])
		loadJ := lb.CalculateShardLoad(shards[j])
		return loadI < loadJ
	})

	// Return the least loaded shard
	return shards[0], nil
}

// ShouldRebalance determines if the shards should be rebalanced
func (lb *LoadBalancer) ShouldRebalance(shards []*shardv1.ShardInstance) bool {
	if len(shards) <= 1 {
		return false
	}

	// Calculate min and max load
	var minLoad, maxLoad float64
	minLoad = 1.0

	for _, shard := range shards {
		load := lb.CalculateShardLoad(shard)
		if load < minLoad {
			minLoad = load
		}
		if load > maxLoad {
			maxLoad = load
		}
	}

	// Rebalance if the difference is more than 20%
	return (maxLoad - minLoad) > 0.2
}

// GenerateRebalancePlan generates a plan to rebalance the shards
func (lb *LoadBalancer) GenerateRebalancePlan(shards []*shardv1.ShardInstance) (*shardv1.MigrationPlan, error) {
	if len(shards) <= 1 {
		return nil, fmt.Errorf("not enough shards to rebalance")
	}

	// Sort shards by load
	sort.Slice(shards, func(i, j int) bool {
		loadI := lb.CalculateShardLoad(shards[i])
		loadJ := lb.CalculateShardLoad(shards[j])
		return loadI > loadJ // Sort in descending order
	})

	// Get the most and least loaded shards
	mostLoaded := shards[0]
	leastLoaded := shards[len(shards)-1]

	// Calculate the load difference
	mostLoadedLoad := lb.CalculateShardLoad(mostLoaded)
	leastLoadedLoad := lb.CalculateShardLoad(leastLoaded)
	loadDiff := mostLoadedLoad - leastLoadedLoad

	// If the difference is less than 20%, no need to rebalance
	if loadDiff <= 0.2 {
		return nil, fmt.Errorf("load difference is within acceptable range")
	}

	// Calculate how many resources to migrate
	// For simplicity, we'll migrate 10% of the resources from the most loaded shard
	resourcesToMigrate := make([]string, 0)
	if len(mostLoaded.Status.AssignedResources) > 0 {
		resourceCount := len(mostLoaded.Status.AssignedResources)
		migrateCount := resourceCount / 10
		if migrateCount == 0 {
			migrateCount = 1
		}

		// Select resources to migrate
		for i := 0; i < migrateCount && i < resourceCount; i++ {
			resourcesToMigrate = append(resourcesToMigrate, mostLoaded.Status.AssignedResources[i])
		}
	}

	// Create the migration plan
	plan := &shardv1.MigrationPlan{
		SourceShard: mostLoaded.Spec.ShardID,
		TargetShard: leastLoaded.Spec.ShardID,
		Resources:   resourcesToMigrate,
		Priority:    shardv1.MigrationPriorityMedium,
	}

	return plan, nil
}

// AssignResourceToShard assigns a resource to a shard
func (lb *LoadBalancer) AssignResourceToShard(resource *interfaces.Resource, shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	if resource == nil {
		return nil, fmt.Errorf("resource cannot be nil")
	}

	if len(shards) == 0 {
		return nil, fmt.Errorf("no shards available")
	}

	// Filter out unhealthy shards
	healthyShards := make([]*shardv1.ShardInstance, 0, len(shards))
	for _, shard := range shards {
		if utils.IsShardHealthy(&shard.Status) && shard.Status.Phase == shardv1.ShardPhaseRunning {
			healthyShards = append(healthyShards, shard)
		}
	}

	if len(healthyShards) == 0 {
		return nil, fmt.Errorf("no healthy shards available")
	}

	// Use the appropriate strategy
	switch lb.strategy {
	case shardv1.ConsistentHashStrategy:
		// Get the shard for this resource
		shardID := lb.consistentHash.GetNode(resource.ID)

		// Find the shard with the matching ID
		for _, shard := range healthyShards {
			if shard.Spec.ShardID == shardID {
				return shard, nil
			}
		}

		// Fallback to the least loaded shard if the designated shard is not available
		return lb.getOptimalShardLeastLoaded(healthyShards)

	case shardv1.RoundRobinStrategy:
		return lb.getOptimalShardRoundRobin(healthyShards)

	case shardv1.LeastLoadedStrategy:
		return lb.getOptimalShardLeastLoaded(healthyShards)

	default:
		return nil, fmt.Errorf("unsupported load balance strategy: %s", lb.strategy)
	}
}

// UpdateShardNodes updates the consistent hash ring when shards change
func (lb *LoadBalancer) UpdateShardNodes(shards []*shardv1.ShardInstance) error {
	if lb.strategy != shardv1.ConsistentHashStrategy {
		return nil // Only relevant for consistent hash strategy
	}

	if lb.consistentHash == nil {
		lb.consistentHash = utils.NewConsistentHash(10)
	}

	// Get current nodes in the hash ring
	currentNodes := lb.consistentHash.GetAllNodes()
	currentNodeSet := make(map[string]bool)
	for _, node := range currentNodes {
		currentNodeSet[node] = true
	}

	// Get new nodes from shards
	newNodeSet := make(map[string]bool)
	for _, shard := range shards {
		if shard.Status.Phase == shardv1.ShardPhaseRunning {
			newNodeSet[shard.Spec.ShardID] = true
		}
	}

	// Remove nodes that are no longer present
	for node := range currentNodeSet {
		if !newNodeSet[node] {
			lb.consistentHash.RemoveNode(node)
		}
	}

	// Add new nodes
	for node := range newNodeSet {
		if !currentNodeSet[node] {
			lb.consistentHash.AddNode(node)
		}
	}

	return nil
}

// GetLoadDistribution returns the current load distribution across shards
func (lb *LoadBalancer) GetLoadDistribution(shards []*shardv1.ShardInstance) map[string]float64 {
	distribution := make(map[string]float64)

	for _, shard := range shards {
		load := lb.CalculateShardLoad(shard)
		distribution[shard.Spec.ShardID] = load
	}

	return distribution
}

// GetStrategy returns the current load balancing strategy
func (lb *LoadBalancer) GetStrategy() shardv1.LoadBalanceStrategy {
	return lb.strategy
}

// SetStrategy changes the load balancing strategy
func (lb *LoadBalancer) SetStrategy(strategy shardv1.LoadBalanceStrategy, shards []*shardv1.ShardInstance) {
	if lb.strategy == strategy {
		return // No change needed
	}

	// Check if strategy is valid before setting it
	switch strategy {
	case shardv1.ConsistentHashStrategy:
		lb.strategy = strategy
		lb.consistentHash = utils.NewConsistentHash(10)
		if shards != nil {
			for _, shard := range shards {
				if shard.Status.Phase == shardv1.ShardPhaseRunning {
					lb.consistentHash.AddNode(shard.Spec.ShardID)
				}
			}
		}
	case shardv1.RoundRobinStrategy:
		lb.strategy = strategy
		lb.mu.Lock()
		lb.lastShardIndex = -1
		lb.mu.Unlock()
		lb.consistentHash = nil
	case shardv1.LeastLoadedStrategy:
		lb.strategy = strategy
		lb.consistentHash = nil
	default:
		// Log error but don't change strategy since it's invalid
		fmt.Printf("Warning: unsupported load balance strategy: %s, keeping current strategy\n", strategy)
		return
	}
}

// SetStrategyWithError changes the load balancing strategy and returns error if any
func (lb *LoadBalancer) SetStrategyWithError(strategy shardv1.LoadBalanceStrategy, shards []*shardv1.ShardInstance) error {
	if lb.strategy == strategy {
		return nil // No change needed
	}

	lb.strategy = strategy

	// Initialize strategy-specific components
	switch strategy {
	case shardv1.ConsistentHashStrategy:
		lb.consistentHash = utils.NewConsistentHash(10)
		if shards != nil {
			for _, shard := range shards {
				if shard.Status.Phase == shardv1.ShardPhaseRunning {
					lb.consistentHash.AddNode(shard.Spec.ShardID)
				}
			}
		}
	case shardv1.RoundRobinStrategy:
		lb.mu.Lock()
		lb.lastShardIndex = -1
		lb.mu.Unlock()
		lb.consistentHash = nil
	case shardv1.LeastLoadedStrategy:
		lb.consistentHash = nil
	default:
		return fmt.Errorf("unsupported load balance strategy: %s", strategy)
	}

	return nil
}

// GetOptimalShardForResource returns the optimal shard for a specific resource
func (lb *LoadBalancer) GetOptimalShardForResource(resource *interfaces.Resource, shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error) {
	return lb.AssignResourceToShard(resource, shards)
}

// CalculateRebalanceScore calculates a score indicating how much rebalancing is needed
func (lb *LoadBalancer) CalculateRebalanceScore(shards []*shardv1.ShardInstance) float64 {
	if len(shards) <= 1 {
		return 0.0
	}

	loads := make([]float64, 0, len(shards))
	var totalLoad float64

	for _, shard := range shards {
		if utils.IsShardHealthy(&shard.Status) && shard.Status.Phase == shardv1.ShardPhaseRunning {
			load := lb.CalculateShardLoad(shard)
			loads = append(loads, load)
			totalLoad += load
		}
	}

	if len(loads) <= 1 {
		return 0.0
	}

	// Calculate standard deviation as a measure of imbalance
	avgLoad := totalLoad / float64(len(loads))
	var variance float64

	for _, load := range loads {
		diff := load - avgLoad
		variance += diff * diff
	}

	variance /= float64(len(loads))
	stdDev := variance // Using variance as approximation for simplicity

	// Normalize the score (higher score means more imbalance)
	return stdDev
}
