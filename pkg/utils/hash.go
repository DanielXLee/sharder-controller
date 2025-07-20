package utils

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
)

// ConsistentHash implements consistent hashing for load balancing
type ConsistentHash struct {
	hashRing     map[uint32]string
	sortedHashes []uint32
	replicas     int
	mu           sync.RWMutex // Add thread safety
}

// NewConsistentHash creates a new consistent hash ring
func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		hashRing: make(map[uint32]string),
		replicas: replicas,
	}
}

// AddNode adds a node to the hash ring
func (ch *ConsistentHash) AddNode(node string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		hash := ch.hashKey(fmt.Sprintf("%s:%d", node, i))
		ch.hashRing[hash] = node
		ch.sortedHashes = append(ch.sortedHashes, hash)
	}
	sort.Slice(ch.sortedHashes, func(i, j int) bool {
		return ch.sortedHashes[i] < ch.sortedHashes[j]
	})
}

// RemoveNode removes a node from the hash ring
func (ch *ConsistentHash) RemoveNode(node string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		hash := ch.hashKey(fmt.Sprintf("%s:%d", node, i))
		delete(ch.hashRing, hash)

		// Remove from sorted hashes
		for j, h := range ch.sortedHashes {
			if h == hash {
				ch.sortedHashes = append(ch.sortedHashes[:j], ch.sortedHashes[j+1:]...)
				break
			}
		}
	}
}

// GetNode returns the node responsible for the given key
func (ch *ConsistentHash) GetNode(key string) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.sortedHashes) == 0 {
		return ""
	}

	hash := ch.hashKey(key)

	// Find the first node with hash >= key hash
	idx := sort.Search(len(ch.sortedHashes), func(i int) bool {
		return ch.sortedHashes[i] >= hash
	})

	// If no node found, wrap around to the first node
	if idx == len(ch.sortedHashes) {
		idx = 0
	}

	return ch.hashRing[ch.sortedHashes[idx]]
}

// GetNodes returns multiple nodes for the given key (for replication)
func (ch *ConsistentHash) GetNodes(key string, count int) []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.sortedHashes) == 0 || count <= 0 {
		return nil
	}

	hash := ch.hashKey(key)
	nodes := make([]string, 0, count)
	seen := make(map[string]bool)

	// Find the starting position
	idx := sort.Search(len(ch.sortedHashes), func(i int) bool {
		return ch.sortedHashes[i] >= hash
	})

	// Collect unique nodes
	for len(nodes) < count && len(seen) < len(ch.getUniqueNodes()) {
		if idx >= len(ch.sortedHashes) {
			idx = 0
		}

		node := ch.hashRing[ch.sortedHashes[idx]]
		if !seen[node] {
			nodes = append(nodes, node)
			seen[node] = true
		}
		idx++
	}

	return nodes
}

// GetAllNodes returns all nodes in the hash ring
func (ch *ConsistentHash) GetAllNodes() []string {
	return ch.getUniqueNodes()
}

// hashKey generates a hash for the given key
func (ch *ConsistentHash) hashKey(key string) uint32 {
	hash := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint32(hash[:4])
}

// getUniqueNodes returns all unique nodes in the hash ring
func (ch *ConsistentHash) getUniqueNodes() []string {
	nodeSet := make(map[string]bool)
	for _, node := range ch.hashRing {
		nodeSet[node] = true
	}

	nodes := make([]string, 0, len(nodeSet))
	for node := range nodeSet {
		nodes = append(nodes, node)
	}

	return nodes
}

// IsEmpty returns true if the hash ring is empty
func (ch *ConsistentHash) IsEmpty() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.sortedHashes) == 0
}

// GetNodeCount returns the number of unique nodes in the hash ring
func (ch *ConsistentHash) GetNodeCount() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.getUniqueNodes())
}

// GetHashForKey returns the hash value for a given key
func (ch *ConsistentHash) GetHashForKey(key string) uint32 {
	return ch.hashKey(key)
}

// GetNodeWithHash returns the node responsible for a specific hash value
func (ch *ConsistentHash) GetNodeWithHash(hash uint32) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.sortedHashes) == 0 {
		return ""
	}

	// Find the first node with hash >= given hash
	idx := sort.Search(len(ch.sortedHashes), func(i int) bool {
		return ch.sortedHashes[i] >= hash
	})

	// If no node found, wrap around to the first node
	if idx == len(ch.sortedHashes) {
		idx = 0
	}

	return ch.hashRing[ch.sortedHashes[idx]]
}

// GetLoadDistribution returns the load distribution across nodes
func (ch *ConsistentHash) GetLoadDistribution() map[string]float64 {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.sortedHashes) == 0 {
		return nil
	}

	distribution := make(map[string]float64)
	uniqueNodes := ch.getUniqueNodes()

	// Initialize all nodes with 0
	for _, node := range uniqueNodes {
		distribution[node] = 0
	}

	// Calculate the hash space each node is responsible for
	totalSpace := uint64(^uint32(0)) + 1 // 2^32

	for i, hash := range ch.sortedHashes {
		var spaceSize uint64
		if i == 0 {
			// First hash: from 0 to this hash + from last hash to max
			lastHash := ch.sortedHashes[len(ch.sortedHashes)-1]
			spaceSize = uint64(hash) + (totalSpace - uint64(lastHash))
		} else {
			// Space from previous hash to this hash
			prevHash := ch.sortedHashes[i-1]
			spaceSize = uint64(hash) - uint64(prevHash)
		}

		node := ch.hashRing[hash]
		distribution[node] += float64(spaceSize) / float64(totalSpace)
	}

	return distribution
}
