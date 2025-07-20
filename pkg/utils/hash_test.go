package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsistentHash(t *testing.T) {
	ch := NewConsistentHash(3)

	// Test empty hash ring
	assert.True(t, ch.IsEmpty())
	assert.Equal(t, "", ch.GetNode("test"))
	assert.Empty(t, ch.GetNodes("test", 2))

	// Add nodes
	ch.AddNode("node1")
	ch.AddNode("node2")
	ch.AddNode("node3")

	assert.False(t, ch.IsEmpty())

	// Test node assignment
	node1 := ch.GetNode("key1")
	node2 := ch.GetNode("key2")

	assert.NotEmpty(t, node1)
	assert.NotEmpty(t, node2)

	// Same key should always return same node
	assert.Equal(t, node1, ch.GetNode("key1"))
	assert.Equal(t, node2, ch.GetNode("key2"))

	// Test multiple nodes
	nodes := ch.GetNodes("key1", 2)
	assert.Len(t, nodes, 2)
	assert.Contains(t, nodes, node1)

	// Test all nodes
	allNodes := ch.GetAllNodes()
	assert.Len(t, allNodes, 3)
	assert.Contains(t, allNodes, "node1")
	assert.Contains(t, allNodes, "node2")
	assert.Contains(t, allNodes, "node3")

	// Test node removal
	ch.RemoveNode("node2")
	allNodesAfterRemoval := ch.GetAllNodes()
	assert.Len(t, allNodesAfterRemoval, 2)
	assert.NotContains(t, allNodesAfterRemoval, "node2")
}

func TestConsistentHashDistribution(t *testing.T) {
	ch := NewConsistentHash(10)

	// Add 5 nodes
	for i := 1; i <= 5; i++ {
		ch.AddNode(fmt.Sprintf("node%d", i))
	}

	// Map to track distribution
	distribution := make(map[string]int)

	// Assign 1000 keys
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		node := ch.GetNode(key)
		distribution[node]++
	}

	// Check that all nodes got some keys
	for i := 1; i <= 5; i++ {
		nodeName := fmt.Sprintf("node%d", i)
		count, exists := distribution[nodeName]
		assert.True(t, exists, "Node %s should have received some keys", nodeName)
		assert.True(t, count > 0, "Node %s should have received some keys", nodeName)
	}

	// Check that distribution is somewhat balanced
	// In a good consistent hash implementation, no node should have more than
	// 35% of the keys with 5 nodes (allowing some variance)
	for node, count := range distribution {
		percentage := float64(count) / 1000.0
		assert.True(t, percentage < 0.35, "Node %s has too many keys: %d (%.2f%%)", node, count, percentage*100)
	}
}

func TestConsistentHashStability(t *testing.T) {
	ch := NewConsistentHash(10)

	// Add 5 nodes
	for i := 1; i <= 5; i++ {
		ch.AddNode(fmt.Sprintf("node%d", i))
	}

	// Assign 100 keys and remember the assignments
	assignments := make(map[string]string)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		node := ch.GetNode(key)
		assignments[key] = node
	}

	// Remove a node
	ch.RemoveNode("node3")

	// Check reassignments
	reassignments := 0
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		newNode := ch.GetNode(key)
		if assignments[key] != newNode && assignments[key] != "node3" {
			reassignments++
		}
	}

	// Only keys that were assigned to node3 should be reassigned
	// plus a small percentage of other keys due to the nature of consistent hashing
	assert.True(t, reassignments < 30, "Too many keys were reassigned: %d", reassignments)
}

func TestConsistentHashThreadSafety(t *testing.T) {
	ch := NewConsistentHash(5)

	// Add initial nodes
	for i := 1; i <= 3; i++ {
		ch.AddNode(fmt.Sprintf("node%d", i))
	}

	// Test concurrent access
	done := make(chan bool, 3)

	// Goroutine 1: Add nodes
	go func() {
		for i := 4; i <= 6; i++ {
			ch.AddNode(fmt.Sprintf("node%d", i))
		}
		done <- true
	}()

	// Goroutine 2: Get nodes
	go func() {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key%d", i)
			ch.GetNode(key)
		}
		done <- true
	}()

	// Goroutine 3: Remove nodes
	go func() {
		ch.RemoveNode("node1")
		ch.RemoveNode("node2")
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify the hash ring is still functional
	assert.False(t, ch.IsEmpty())
	node := ch.GetNode("test-key")
	assert.NotEmpty(t, node)
}

func TestConsistentHashGetNodeCount(t *testing.T) {
	ch := NewConsistentHash(5)

	assert.Equal(t, 0, ch.GetNodeCount())

	ch.AddNode("node1")
	assert.Equal(t, 1, ch.GetNodeCount())

	ch.AddNode("node2")
	assert.Equal(t, 2, ch.GetNodeCount())

	ch.RemoveNode("node1")
	assert.Equal(t, 1, ch.GetNodeCount())
}

func TestConsistentHashGetHashForKey(t *testing.T) {
	ch := NewConsistentHash(5)

	hash1 := ch.GetHashForKey("test-key")
	hash2 := ch.GetHashForKey("test-key")

	// Same key should always produce the same hash
	assert.Equal(t, hash1, hash2)

	// Different keys should produce different hashes (most of the time)
	hash3 := ch.GetHashForKey("different-key")
	assert.NotEqual(t, hash1, hash3)
}

func TestConsistentHashGetNodeWithHash(t *testing.T) {
	ch := NewConsistentHash(5)

	// Empty ring
	assert.Equal(t, "", ch.GetNodeWithHash(12345))

	// Add nodes
	ch.AddNode("node1")
	ch.AddNode("node2")

	// Get node for specific hash
	node := ch.GetNodeWithHash(12345)
	assert.NotEmpty(t, node)

	// Same hash should always return same node
	node2 := ch.GetNodeWithHash(12345)
	assert.Equal(t, node, node2)
}

func TestConsistentHashGetLoadDistribution(t *testing.T) {
	ch := NewConsistentHash(10)

	// Empty ring
	assert.Nil(t, ch.GetLoadDistribution())

	// Add nodes
	ch.AddNode("node1")
	ch.AddNode("node2")
	ch.AddNode("node3")

	distribution := ch.GetLoadDistribution()
	assert.Len(t, distribution, 3)

	// All nodes should have some load
	for node, load := range distribution {
		assert.True(t, load > 0, "Node %s should have positive load", node)
		assert.True(t, load <= 1.0, "Node %s load should not exceed 1.0", node)
	}

	// Total load should sum to approximately 1.0
	var totalLoad float64
	for _, load := range distribution {
		totalLoad += load
	}
	assert.InDelta(t, 1.0, totalLoad, 0.01)
}

func TestConsistentHashGetNodesMultiple(t *testing.T) {
	ch := NewConsistentHash(5)

	// Add nodes
	for i := 1; i <= 5; i++ {
		ch.AddNode(fmt.Sprintf("node%d", i))
	}

	// Test getting multiple nodes
	nodes := ch.GetNodes("test-key", 3)
	assert.Len(t, nodes, 3)

	// All nodes should be unique
	nodeSet := make(map[string]bool)
	for _, node := range nodes {
		assert.False(t, nodeSet[node], "Node %s should not be duplicated", node)
		nodeSet[node] = true
	}

	// Test getting more nodes than available
	nodes = ch.GetNodes("test-key", 10)
	assert.Len(t, nodes, 5) // Should return all available nodes

	// Test getting zero nodes
	nodes = ch.GetNodes("test-key", 0)
	assert.Empty(t, nodes)
}
