// +k8s:deepcopy-gen=package

package v1

import (
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShardConfig defines the configuration for shard management
type ShardConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShardConfigSpec   `json:"spec,omitempty"`
	Status ShardConfigStatus `json:"status,omitempty"`
}

// ShardConfigSpec defines the desired state of ShardConfig
type ShardConfigSpec struct {
	MinShards               int                   `json:"minShards"`
	MaxShards               int                   `json:"maxShards"`
	ScaleUpThreshold        float64              `json:"scaleUpThreshold"`
	ScaleDownThreshold      float64              `json:"scaleDownThreshold"`
	HealthCheckInterval     metav1.Duration      `json:"healthCheckInterval"`
	LoadBalanceStrategy     LoadBalanceStrategy  `json:"loadBalanceStrategy"`
	GracefulShutdownTimeout metav1.Duration      `json:"gracefulShutdownTimeout"`
}

// ShardConfigStatus defines the observed state of ShardConfig
type ShardConfigStatus struct {
	CurrentShards int     `json:"currentShards"`
	HealthyShards int     `json:"healthyShards"`
	TotalLoad     float64 `json:"totalLoad"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShardInstance represents a single shard instance
type ShardInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShardInstanceSpec   `json:"spec,omitempty"`
	Status ShardInstanceStatus `json:"status,omitempty"`
}

// ShardInstanceSpec defines the desired state of ShardInstance
type ShardInstanceSpec struct {
	ShardID   string     `json:"shardId"`
	HashRange *HashRange `json:"hashRange,omitempty"`
	Resources []string   `json:"resources,omitempty"`
}

// ShardInstanceStatus defines the observed state of ShardInstance
type ShardInstanceStatus struct {
	Phase             ShardPhase        `json:"phase"`
	Load              float64           `json:"load"`
	LastHeartbeat     metav1.Time       `json:"lastHeartbeat"`
	AssignedResources []string          `json:"assignedResources,omitempty"`
	HealthStatus      *HealthStatus     `json:"healthStatus,omitempty"`
}

// HashRange defines the hash range for consistent hashing
type HashRange struct {
	Start uint32 `json:"start"`
	End   uint32 `json:"end"`
}

// HealthStatus represents the health status of a shard
type HealthStatus struct {
	Healthy    bool        `json:"healthy"`
	LastCheck  metav1.Time `json:"lastCheck"`
	ErrorCount int         `json:"errorCount"`
	Message    string      `json:"message,omitempty"`
}

// LoadMetrics represents load metrics for a shard
type LoadMetrics struct {
	ResourceCount   int     `json:"resourceCount"`
	CPUUsage        float64 `json:"cpuUsage"`
	MemoryUsage     float64 `json:"memoryUsage"`
	ProcessingRate  float64 `json:"processingRate"`
	QueueLength     int     `json:"queueLength"`
}

// MigrationPlan represents a resource migration plan
type MigrationPlan struct {
	SourceShard   string            `json:"sourceShard"`
	TargetShard   string            `json:"targetShard"`
	Resources     []string          `json:"resources"`
	EstimatedTime metav1.Duration   `json:"estimatedTime"`
	Priority      MigrationPriority `json:"priority"`
}

// LoadBalanceStrategy defines the load balancing strategy
type LoadBalanceStrategy string

const (
	ConsistentHashStrategy LoadBalanceStrategy = "consistent-hash"
	RoundRobinStrategy     LoadBalanceStrategy = "round-robin"
	LeastLoadedStrategy    LoadBalanceStrategy = "least-loaded"
)

// ShardPhase defines the phase of a shard
type ShardPhase string

const (
	ShardPhasePending    ShardPhase = "Pending"
	ShardPhaseRunning    ShardPhase = "Running"
	ShardPhaseDraining   ShardPhase = "Draining"
	ShardPhaseFailed     ShardPhase = "Failed"
	ShardPhaseTerminated ShardPhase = "Terminated"
)

// MigrationPriority defines the priority of migration
type MigrationPriority string

const (
	MigrationPriorityHigh   MigrationPriority = "High"
	MigrationPriorityMedium MigrationPriority = "Medium"
	MigrationPriorityLow    MigrationPriority = "Low"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShardConfigList contains a list of ShardConfig
type ShardConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShardConfig `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShardInstanceList contains a list of ShardInstance
type ShardInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShardInstance `json:"items"`
}

// Serialization methods for ShardConfig

// ToJSON serializes ShardConfig to JSON
func (sc *ShardConfig) ToJSON() ([]byte, error) {
	return json.Marshal(sc)
}

// FromJSON deserializes ShardConfig from JSON
func (sc *ShardConfig) FromJSON(data []byte) error {
	return json.Unmarshal(data, sc)
}

// String returns a string representation of ShardConfig
func (sc *ShardConfig) String() string {
	return fmt.Sprintf("ShardConfig{Name: %s, MinShards: %d, MaxShards: %d, Strategy: %s}",
		sc.Name, sc.Spec.MinShards, sc.Spec.MaxShards, sc.Spec.LoadBalanceStrategy)
}

// Serialization methods for ShardInstance

// ToJSON serializes ShardInstance to JSON
func (si *ShardInstance) ToJSON() ([]byte, error) {
	return json.Marshal(si)
}

// FromJSON deserializes ShardInstance from JSON
func (si *ShardInstance) FromJSON(data []byte) error {
	return json.Unmarshal(data, si)
}

// String returns a string representation of ShardInstance
func (si *ShardInstance) String() string {
	return fmt.Sprintf("ShardInstance{Name: %s, ShardID: %s, Phase: %s, Load: %.2f}",
		si.Name, si.Spec.ShardID, si.Status.Phase, si.Status.Load)
}

// State machine methods for ShardInstance

// CanTransitionTo checks if the shard can transition to the given phase
func (si *ShardInstance) CanTransitionTo(newPhase ShardPhase) bool {
	return IsValidPhaseTransition(si.Status.Phase, newPhase)
}

// TransitionTo transitions the shard to a new phase if valid
func (si *ShardInstance) TransitionTo(newPhase ShardPhase) error {
	if !si.CanTransitionTo(newPhase) {
		return fmt.Errorf("invalid phase transition from %s to %s", si.Status.Phase, newPhase)
	}
	
	si.Status.Phase = newPhase
	
	// Update timestamps and status based on phase
	now := metav1.Now()
	switch newPhase {
	case ShardPhaseRunning:
		si.Status.LastHeartbeat = now
		if si.Status.HealthStatus == nil {
			si.Status.HealthStatus = &HealthStatus{
				Healthy:   true,
				LastCheck: now,
			}
		}
	case ShardPhaseFailed:
		if si.Status.HealthStatus != nil {
			si.Status.HealthStatus.Healthy = false
			si.Status.HealthStatus.LastCheck = now
			si.Status.HealthStatus.ErrorCount++
		}
	case ShardPhaseTerminated:
		// Clear resources when terminated
		si.Status.AssignedResources = nil
		si.Status.Load = 0
	}
	
	return nil
}

// IsHealthy returns true if the shard is healthy
func (si *ShardInstance) IsHealthy() bool {
	if si.Status.HealthStatus == nil {
		return false
	}
	return si.Status.HealthStatus.Healthy
}

// IsActive returns true if the shard is in an active state
func (si *ShardInstance) IsActive() bool {
	return si.Status.Phase == ShardPhaseRunning
}

// IsTerminal returns true if the shard is in a terminal state
func (si *ShardInstance) IsTerminal() bool {
	return si.Status.Phase == ShardPhaseTerminated
}

// UpdateHeartbeat updates the last heartbeat timestamp
func (si *ShardInstance) UpdateHeartbeat() {
	si.Status.LastHeartbeat = metav1.Now()
}

// UpdateLoad updates the current load of the shard
func (si *ShardInstance) UpdateLoad(load float64) {
	si.Status.Load = load
}

// AddResource adds a resource to the shard
func (si *ShardInstance) AddResource(resource string) {
	for _, r := range si.Status.AssignedResources {
		if r == resource {
			return // Already exists
		}
	}
	si.Status.AssignedResources = append(si.Status.AssignedResources, resource)
}

// RemoveResource removes a resource from the shard
func (si *ShardInstance) RemoveResource(resource string) {
	for i, r := range si.Status.AssignedResources {
		if r == resource {
			si.Status.AssignedResources = append(si.Status.AssignedResources[:i], si.Status.AssignedResources[i+1:]...)
			return
		}
	}
}

// GetResourceCount returns the number of assigned resources
func (si *ShardInstance) GetResourceCount() int {
	return len(si.Status.AssignedResources)
}

// Utility methods for HashRange

// Contains checks if a hash value is within the range
func (hr *HashRange) Contains(hash uint32) bool {
	if hr == nil {
		return false
	}
	return hash >= hr.Start && hash <= hr.End
}

// Size returns the size of the hash range
func (hr *HashRange) Size() uint32 {
	if hr == nil {
		return 0
	}
	return hr.End - hr.Start + 1
}

// String returns a string representation of HashRange
func (hr *HashRange) String() string {
	if hr == nil {
		return "HashRange{nil}"
	}
	return fmt.Sprintf("HashRange{Start: %d, End: %d}", hr.Start, hr.End)
}

// Utility methods for HealthStatus

// MarkHealthy marks the health status as healthy
func (hs *HealthStatus) MarkHealthy(message string) {
	hs.Healthy = true
	hs.LastCheck = metav1.Now()
	hs.ErrorCount = 0
	hs.Message = message
}

// MarkUnhealthy marks the health status as unhealthy
func (hs *HealthStatus) MarkUnhealthy(message string) {
	hs.Healthy = false
	hs.LastCheck = metav1.Now()
	hs.ErrorCount++
	hs.Message = message
}

// IsStale checks if the health status is stale based on the given duration
func (hs *HealthStatus) IsStale(staleDuration time.Duration) bool {
	return time.Since(hs.LastCheck.Time) > staleDuration
}

// Utility methods for LoadMetrics

// CalculateOverallLoad calculates an overall load score
func (lm *LoadMetrics) CalculateOverallLoad() float64 {
	// Weighted average of different metrics
	resourceWeight := 0.3
	cpuWeight := 0.3
	memoryWeight := 0.2
	queueWeight := 0.2
	
	// Normalize resource count (assuming max 1000 resources)
	normalizedResources := float64(lm.ResourceCount) / 1000.0
	if normalizedResources > 1.0 {
		normalizedResources = 1.0
	}
	
	// Normalize queue length (assuming max 100 items)
	normalizedQueue := float64(lm.QueueLength) / 100.0
	if normalizedQueue > 1.0 {
		normalizedQueue = 1.0
	}
	
	return resourceWeight*normalizedResources +
		cpuWeight*lm.CPUUsage +
		memoryWeight*lm.MemoryUsage +
		queueWeight*normalizedQueue
}

// IsOverloaded checks if the shard is overloaded based on thresholds
func (lm *LoadMetrics) IsOverloaded(cpuThreshold, memoryThreshold float64, queueThreshold int) bool {
	return lm.CPUUsage > cpuThreshold ||
		lm.MemoryUsage > memoryThreshold ||
		lm.QueueLength > queueThreshold
}

// State machine utility functions

// IsValidPhaseTransition checks if a phase transition is valid
func IsValidPhaseTransition(oldPhase, newPhase ShardPhase) bool {
	validTransitions := map[ShardPhase][]ShardPhase{
		ShardPhasePending:    {ShardPhaseRunning, ShardPhaseFailed},
		ShardPhaseRunning:    {ShardPhaseDraining, ShardPhaseFailed},
		ShardPhaseDraining:   {ShardPhaseTerminated, ShardPhaseRunning}, // Can go back to running if drain is cancelled
		ShardPhaseFailed:     {ShardPhaseRunning, ShardPhaseTerminated},
		ShardPhaseTerminated: {}, // Terminal state
	}
	
	validNext, exists := validTransitions[oldPhase]
	if !exists {
		return false
	}
	
	for _, valid := range validNext {
		if newPhase == valid {
			return true
		}
	}
	
	return false
}

// GetAllValidTransitions returns all valid transitions from a given phase
func GetAllValidTransitions(phase ShardPhase) []ShardPhase {
	validTransitions := map[ShardPhase][]ShardPhase{
		ShardPhasePending:    {ShardPhaseRunning, ShardPhaseFailed},
		ShardPhaseRunning:    {ShardPhaseDraining, ShardPhaseFailed},
		ShardPhaseDraining:   {ShardPhaseTerminated, ShardPhaseRunning},
		ShardPhaseFailed:     {ShardPhaseRunning, ShardPhaseTerminated},
		ShardPhaseTerminated: {},
	}
	
	if transitions, exists := validTransitions[phase]; exists {
		return transitions
	}
	return []ShardPhase{}
}