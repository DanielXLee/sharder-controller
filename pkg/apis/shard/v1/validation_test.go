package v1

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateShardConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *ShardConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ShardConfig{
				Spec: ShardConfigSpec{
					MinShards:               1,
					MaxShards:               10,
					ScaleUpThreshold:        0.8,
					ScaleDownThreshold:      0.3,
					HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
					LoadBalanceStrategy:     ConsistentHashStrategy,
					GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid min shards",
			config: &ShardConfig{
				Spec: ShardConfigSpec{
					MinShards:               0,
					MaxShards:               10,
					ScaleUpThreshold:        0.8,
					ScaleDownThreshold:      0.3,
					HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
					LoadBalanceStrategy:     ConsistentHashStrategy,
					GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
				},
			},
			wantErr: true,
		},
		{
			name: "min shards greater than max shards",
			config: &ShardConfig{
				Spec: ShardConfigSpec{
					MinShards:               10,
					MaxShards:               5,
					ScaleUpThreshold:        0.8,
					ScaleDownThreshold:      0.3,
					HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
					LoadBalanceStrategy:     ConsistentHashStrategy,
					GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid scale thresholds",
			config: &ShardConfig{
				Spec: ShardConfigSpec{
					MinShards:               1,
					MaxShards:               10,
					ScaleUpThreshold:        0.3,
					ScaleDownThreshold:      0.8,
					HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
					LoadBalanceStrategy:     ConsistentHashStrategy,
					GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid load balance strategy",
			config: &ShardConfig{
				Spec: ShardConfigSpec{
					MinShards:               1,
					MaxShards:               10,
					ScaleUpThreshold:        0.8,
					ScaleDownThreshold:      0.3,
					HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
					LoadBalanceStrategy:     "invalid-strategy",
					GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateShardConfig(tt.config)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("ValidateShardConfig() error = %v, wantErr %v, errors: %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateShardInstance(t *testing.T) {
	tests := []struct {
		name     string
		instance *ShardInstance
		wantErr  bool
	}{
		{
			name: "valid instance",
			instance: &ShardInstance{
				Spec: ShardInstanceSpec{
					ShardID: "shard-1",
					HashRange: &HashRange{
						Start: 0,
						End:   1000,
					},
				},
				Status: ShardInstanceStatus{
					Phase: ShardPhasePending,
				},
			},
			wantErr: false,
		},
		{
			name: "empty shard ID",
			instance: &ShardInstance{
				Spec: ShardInstanceSpec{
					ShardID: "",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid hash range",
			instance: &ShardInstance{
				Spec: ShardInstanceSpec{
					ShardID: "shard-1",
					HashRange: &HashRange{
						Start: 1000,
						End:   500,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateShardInstance(tt.instance)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("ValidateShardInstance() error = %v, wantErr %v, errors: %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestSetShardConfigDefaults(t *testing.T) {
	config := &ShardConfig{}
	SetShardConfigDefaults(config)

	if config.Spec.MinShards != 1 {
		t.Errorf("Expected MinShards to be 1, got %d", config.Spec.MinShards)
	}
	if config.Spec.MaxShards != 10 {
		t.Errorf("Expected MaxShards to be 10, got %d", config.Spec.MaxShards)
	}
	if config.Spec.ScaleUpThreshold != 0.8 {
		t.Errorf("Expected ScaleUpThreshold to be 0.8, got %f", config.Spec.ScaleUpThreshold)
	}
	if config.Spec.ScaleDownThreshold != 0.3 {
		t.Errorf("Expected ScaleDownThreshold to be 0.3, got %f", config.Spec.ScaleDownThreshold)
	}
	if config.Spec.LoadBalanceStrategy != ConsistentHashStrategy {
		t.Errorf("Expected LoadBalanceStrategy to be %s, got %s", ConsistentHashStrategy, config.Spec.LoadBalanceStrategy)
	}
}

func TestSetShardInstanceDefaults(t *testing.T) {
	instance := &ShardInstance{}
	SetShardInstanceDefaults(instance)

	if instance.Status.Phase != ShardPhasePending {
		t.Errorf("Expected Phase to be %s, got %s", ShardPhasePending, instance.Status.Phase)
	}
	if instance.Status.HealthStatus == nil {
		t.Error("Expected HealthStatus to be initialized")
	}
	if !instance.Status.HealthStatus.Healthy {
		t.Error("Expected HealthStatus.Healthy to be true")
	}
}

func TestValidateShardPhaseTransition(t *testing.T) {
	tests := []struct {
		name     string
		oldPhase ShardPhase
		newPhase ShardPhase
		wantErr  bool
	}{
		{
			name:     "pending to running",
			oldPhase: ShardPhasePending,
			newPhase: ShardPhaseRunning,
			wantErr:  false,
		},
		{
			name:     "running to draining",
			oldPhase: ShardPhaseRunning,
			newPhase: ShardPhaseDraining,
			wantErr:  false,
		},
		{
			name:     "draining to terminated",
			oldPhase: ShardPhaseDraining,
			newPhase: ShardPhaseTerminated,
			wantErr:  false,
		},
		{
			name:     "invalid transition",
			oldPhase: ShardPhasePending,
			newPhase: ShardPhaseTerminated,
			wantErr:  true,
		},
		{
			name:     "from terminated",
			oldPhase: ShardPhaseTerminated,
			newPhase: ShardPhaseRunning,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateShardPhaseTransition(tt.oldPhase, tt.newPhase)
			hasErr := err != nil
			if hasErr != tt.wantErr {
				t.Errorf("ValidateShardPhaseTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test serialization methods
func TestShardConfigSerialization(t *testing.T) {
	config := &ShardConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: ShardConfigSpec{
			MinShards:               2,
			MaxShards:               10,
			ScaleUpThreshold:        0.8,
			ScaleDownThreshold:      0.3,
			HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy:     ConsistentHashStrategy,
			GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
		},
	}

	// Test ToJSON
	data, err := config.ToJSON()
	if err != nil {
		t.Errorf("ToJSON() error = %v", err)
	}

	// Test FromJSON
	newConfig := &ShardConfig{}
	err = newConfig.FromJSON(data)
	if err != nil {
		t.Errorf("FromJSON() error = %v", err)
	}

	if newConfig.Name != config.Name {
		t.Errorf("Expected Name %s, got %s", config.Name, newConfig.Name)
	}
	if newConfig.Spec.MinShards != config.Spec.MinShards {
		t.Errorf("Expected MinShards %d, got %d", config.Spec.MinShards, newConfig.Spec.MinShards)
	}

	// Test String method
	str := config.String()
	if str == "" {
		t.Error("String() returned empty string")
	}
}

func TestShardInstanceSerialization(t *testing.T) {
	instance := &ShardInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instance",
			Namespace: "default",
		},
		Spec: ShardInstanceSpec{
			ShardID: "shard-1",
			HashRange: &HashRange{
				Start: 0,
				End:   1000,
			},
		},
		Status: ShardInstanceStatus{
			Phase: ShardPhaseRunning,
			Load:  0.5,
		},
	}

	// Test ToJSON
	data, err := instance.ToJSON()
	if err != nil {
		t.Errorf("ToJSON() error = %v", err)
	}

	// Test FromJSON
	newInstance := &ShardInstance{}
	err = newInstance.FromJSON(data)
	if err != nil {
		t.Errorf("FromJSON() error = %v", err)
	}

	if newInstance.Spec.ShardID != instance.Spec.ShardID {
		t.Errorf("Expected ShardID %s, got %s", instance.Spec.ShardID, newInstance.Spec.ShardID)
	}

	// Test String method
	str := instance.String()
	if str == "" {
		t.Error("String() returned empty string")
	}
}

// Test state machine methods
func TestShardInstanceStateMachine(t *testing.T) {
	instance := &ShardInstance{
		Status: ShardInstanceStatus{
			Phase: ShardPhasePending,
		},
	}

	// Test CanTransitionTo
	if !instance.CanTransitionTo(ShardPhaseRunning) {
		t.Error("Should be able to transition from Pending to Running")
	}
	if instance.CanTransitionTo(ShardPhaseTerminated) {
		t.Error("Should not be able to transition from Pending to Terminated")
	}

	// Test TransitionTo
	err := instance.TransitionTo(ShardPhaseRunning)
	if err != nil {
		t.Errorf("TransitionTo() error = %v", err)
	}
	if instance.Status.Phase != ShardPhaseRunning {
		t.Errorf("Expected phase %s, got %s", ShardPhaseRunning, instance.Status.Phase)
	}

	// Test invalid transition
	err = instance.TransitionTo(ShardPhaseTerminated)
	if err == nil {
		t.Error("Expected error for invalid transition")
	}

	// Test IsHealthy, IsActive, IsTerminal
	instance.Status.HealthStatus = &HealthStatus{Healthy: true}
	if !instance.IsHealthy() {
		t.Error("Expected instance to be healthy")
	}
	if !instance.IsActive() {
		t.Error("Expected instance to be active")
	}
	if instance.IsTerminal() {
		t.Error("Expected instance not to be terminal")
	}
}

func TestShardInstanceResourceManagement(t *testing.T) {
	instance := &ShardInstance{}

	// Test AddResource
	instance.AddResource("resource-1")
	instance.AddResource("resource-2")
	instance.AddResource("resource-1") // Duplicate should be ignored

	if instance.GetResourceCount() != 2 {
		t.Errorf("Expected 2 resources, got %d", instance.GetResourceCount())
	}

	// Test RemoveResource
	instance.RemoveResource("resource-1")
	if instance.GetResourceCount() != 1 {
		t.Errorf("Expected 1 resource, got %d", instance.GetResourceCount())
	}

	// Test UpdateLoad
	instance.UpdateLoad(0.75)
	if instance.Status.Load != 0.75 {
		t.Errorf("Expected load 0.75, got %f", instance.Status.Load)
	}

	// Test UpdateHeartbeat
	oldHeartbeat := instance.Status.LastHeartbeat
	time.Sleep(time.Millisecond) // Ensure time difference
	instance.UpdateHeartbeat()
	if !instance.Status.LastHeartbeat.After(oldHeartbeat.Time) {
		t.Error("Expected heartbeat to be updated")
	}
}

// Test HashRange methods
func TestHashRange(t *testing.T) {
	hashRange := &HashRange{
		Start: 100,
		End:   200,
	}

	// Test Contains
	if !hashRange.Contains(150) {
		t.Error("Expected hash 150 to be contained in range")
	}
	if hashRange.Contains(50) {
		t.Error("Expected hash 50 not to be contained in range")
	}
	if hashRange.Contains(250) {
		t.Error("Expected hash 250 not to be contained in range")
	}

	// Test Size
	expectedSize := uint32(101) // 200 - 100 + 1
	if hashRange.Size() != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, hashRange.Size())
	}

	// Test String
	str := hashRange.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Test nil HashRange
	var nilRange *HashRange
	if nilRange.Contains(100) {
		t.Error("Nil range should not contain any hash")
	}
	if nilRange.Size() != 0 {
		t.Error("Nil range should have size 0")
	}
}

// Test HealthStatus methods
func TestHealthStatus(t *testing.T) {
	healthStatus := &HealthStatus{}

	// Test MarkHealthy
	healthStatus.MarkHealthy("All systems operational")
	if !healthStatus.Healthy {
		t.Error("Expected health status to be healthy")
	}
	if healthStatus.ErrorCount != 0 {
		t.Errorf("Expected error count 0, got %d", healthStatus.ErrorCount)
	}
	if healthStatus.Message != "All systems operational" {
		t.Errorf("Expected message 'All systems operational', got '%s'", healthStatus.Message)
	}

	// Test MarkUnhealthy
	healthStatus.MarkUnhealthy("Connection failed")
	if healthStatus.Healthy {
		t.Error("Expected health status to be unhealthy")
	}
	if healthStatus.ErrorCount != 1 {
		t.Errorf("Expected error count 1, got %d", healthStatus.ErrorCount)
	}

	// Test IsStale
	healthStatus.LastCheck = metav1.Now()
	time.Sleep(time.Millisecond)
	if healthStatus.IsStale(time.Hour) {
		t.Error("Expected health status not to be stale with very short duration")
	}
	
	// Set an old timestamp to test staleness
	healthStatus.LastCheck = metav1.Time{Time: time.Now().Add(-2 * time.Hour)}
	if !healthStatus.IsStale(time.Hour) {
		t.Error("Expected health status to be stale with long duration")
	}
}

// Test LoadMetrics methods
func TestLoadMetrics(t *testing.T) {
	loadMetrics := &LoadMetrics{
		ResourceCount:  500,
		CPUUsage:       0.7,
		MemoryUsage:    0.6,
		ProcessingRate: 100.5,
		QueueLength:    10,
	}

	// Test CalculateOverallLoad
	overallLoad := loadMetrics.CalculateOverallLoad()
	if overallLoad < 0 || overallLoad > 1 {
		t.Errorf("Expected overall load between 0 and 1, got %f", overallLoad)
	}

	// Test IsOverloaded
	if !loadMetrics.IsOverloaded(0.5, 0.5, 5) {
		t.Error("Expected metrics to be overloaded with low thresholds")
	}
	if loadMetrics.IsOverloaded(0.9, 0.9, 50) {
		t.Error("Expected metrics not to be overloaded with high thresholds")
	}
}

// Test enhanced validation functions
func TestValidateHashRange(t *testing.T) {
	tests := []struct {
		name      string
		hashRange *HashRange
		wantErr   bool
	}{
		{
			name:      "nil hash range",
			hashRange: nil,
			wantErr:   false,
		},
		{
			name: "valid hash range",
			hashRange: &HashRange{
				Start: 0,
				End:   1000,
			},
			wantErr: false,
		},
		{
			name: "invalid hash range - start > end",
			hashRange: &HashRange{
				Start: 1000,
				End:   500,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateHashRange(tt.hashRange)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("ValidateHashRange() error = %v, wantErr %v, errors: %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateHealthStatus(t *testing.T) {
	tests := []struct {
		name         string
		healthStatus *HealthStatus
		wantErr      bool
	}{
		{
			name:         "nil health status",
			healthStatus: nil,
			wantErr:      false,
		},
		{
			name: "valid health status",
			healthStatus: &HealthStatus{
				Healthy:    true,
				LastCheck:  metav1.Now(),
				ErrorCount: 0,
				Message:    "OK",
			},
			wantErr: false,
		},
		{
			name: "invalid error count",
			healthStatus: &HealthStatus{
				Healthy:    false,
				LastCheck:  metav1.Now(),
				ErrorCount: -1,
				Message:    "Error",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateHealthStatus(tt.healthStatus)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("ValidateHealthStatus() error = %v, wantErr %v, errors: %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateLoadMetrics(t *testing.T) {
	tests := []struct {
		name        string
		loadMetrics *LoadMetrics
		wantErr     bool
	}{
		{
			name:        "nil load metrics",
			loadMetrics: nil,
			wantErr:     false,
		},
		{
			name: "valid load metrics",
			loadMetrics: &LoadMetrics{
				ResourceCount:   100,
				CPUUsage:        0.5,
				MemoryUsage:     0.6,
				ProcessingRate:  50.0,
				QueueLength:     10,
			},
			wantErr: false,
		},
		{
			name: "invalid resource count",
			loadMetrics: &LoadMetrics{
				ResourceCount:   -1,
				CPUUsage:        0.5,
				MemoryUsage:     0.6,
				ProcessingRate:  50.0,
				QueueLength:     10,
			},
			wantErr: true,
		},
		{
			name: "invalid CPU usage",
			loadMetrics: &LoadMetrics{
				ResourceCount:   100,
				CPUUsage:        1.5,
				MemoryUsage:     0.6,
				ProcessingRate:  50.0,
				QueueLength:     10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateLoadMetrics(tt.loadMetrics)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("ValidateLoadMetrics() error = %v, wantErr %v, errors: %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateMigrationPlan(t *testing.T) {
	tests := []struct {
		name           string
		migrationPlan  *MigrationPlan
		wantErr        bool
	}{
		{
			name:          "nil migration plan",
			migrationPlan: nil,
			wantErr:       false,
		},
		{
			name: "valid migration plan",
			migrationPlan: &MigrationPlan{
				SourceShard:   "shard-1",
				TargetShard:   "shard-2",
				Resources:     []string{"resource-1", "resource-2"},
				EstimatedTime: metav1.Duration{Duration: 5 * time.Minute},
				Priority:      MigrationPriorityHigh,
			},
			wantErr: false,
		},
		{
			name: "missing source shard",
			migrationPlan: &MigrationPlan{
				SourceShard:   "",
				TargetShard:   "shard-2",
				Resources:     []string{"resource-1"},
				EstimatedTime: metav1.Duration{Duration: 5 * time.Minute},
				Priority:      MigrationPriorityHigh,
			},
			wantErr: true,
		},
		{
			name: "same source and target",
			migrationPlan: &MigrationPlan{
				SourceShard:   "shard-1",
				TargetShard:   "shard-1",
				Resources:     []string{"resource-1"},
				EstimatedTime: metav1.Duration{Duration: 5 * time.Minute},
				Priority:      MigrationPriorityHigh,
			},
			wantErr: true,
		},
		{
			name: "no resources",
			migrationPlan: &MigrationPlan{
				SourceShard:   "shard-1",
				TargetShard:   "shard-2",
				Resources:     []string{},
				EstimatedTime: metav1.Duration{Duration: 5 * time.Minute},
				Priority:      MigrationPriorityHigh,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateMigrationPlan(tt.migrationPlan)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("ValidateMigrationPlan() error = %v, wantErr %v, errors: %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateShardConfigSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ShardConfigSpec
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: &ShardConfigSpec{
				MinShards:               2,
				MaxShards:               20,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
			},
			wantErr: false,
		},
		{
			name: "thresholds too close",
			spec: &ShardConfigSpec{
				MinShards:               1,
				MaxShards:               10,
				ScaleUpThreshold:        0.5,
				ScaleDownThreshold:      0.45, // Only 0.05 difference
				HealthCheckInterval:     metav1.Duration{Duration: 30 * time.Second},
				LoadBalanceStrategy:     ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
			},
			wantErr: true,
		},
		{
			name: "health check interval too long",
			spec: &ShardConfigSpec{
				MinShards:               1,
				MaxShards:               10,
				ScaleUpThreshold:        0.8,
				ScaleDownThreshold:      0.3,
				HealthCheckInterval:     metav1.Duration{Duration: 15 * time.Minute}, // Too long
				LoadBalanceStrategy:     ConsistentHashStrategy,
				GracefulShutdownTimeout: metav1.Duration{Duration: 300 * time.Second},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateShardConfigSpec(tt.spec)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("ValidateShardConfigSpec() error = %v, wantErr %v, errors: %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

func TestValidateShardInstanceSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ShardInstanceSpec
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: &ShardInstanceSpec{
				ShardID: "shard-1",
				HashRange: &HashRange{
					Start: 0,
					End:   1000,
				},
				Resources: []string{"resource-1", "resource-2"},
			},
			wantErr: false,
		},
		{
			name: "invalid shard ID format",
			spec: &ShardInstanceSpec{
				ShardID: "Shard_1", // Invalid characters
			},
			wantErr: true,
		},
		{
			name: "shard ID too long",
			spec: &ShardInstanceSpec{
				ShardID: "this-is-a-very-long-shard-id-that-exceeds-the-maximum-allowed-length-of-63-characters",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateShardInstanceSpec(tt.spec)
			hasErr := len(errs) > 0
			if hasErr != tt.wantErr {
				t.Errorf("ValidateShardInstanceSpec() error = %v, wantErr %v, errors: %v", hasErr, tt.wantErr, errs)
			}
		})
	}
}

// Test state machine utility functions
func TestIsValidPhaseTransition(t *testing.T) {
	tests := []struct {
		name     string
		oldPhase ShardPhase
		newPhase ShardPhase
		want     bool
	}{
		{
			name:     "pending to running",
			oldPhase: ShardPhasePending,
			newPhase: ShardPhaseRunning,
			want:     true,
		},
		{
			name:     "pending to failed",
			oldPhase: ShardPhasePending,
			newPhase: ShardPhaseFailed,
			want:     true,
		},
		{
			name:     "running to draining",
			oldPhase: ShardPhaseRunning,
			newPhase: ShardPhaseDraining,
			want:     true,
		},
		{
			name:     "draining to running (cancel drain)",
			oldPhase: ShardPhaseDraining,
			newPhase: ShardPhaseRunning,
			want:     true,
		},
		{
			name:     "failed to running (recovery)",
			oldPhase: ShardPhaseFailed,
			newPhase: ShardPhaseRunning,
			want:     true,
		},
		{
			name:     "terminated to running (invalid)",
			oldPhase: ShardPhaseTerminated,
			newPhase: ShardPhaseRunning,
			want:     false,
		},
		{
			name:     "pending to terminated (invalid)",
			oldPhase: ShardPhasePending,
			newPhase: ShardPhaseTerminated,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidPhaseTransition(tt.oldPhase, tt.newPhase)
			if got != tt.want {
				t.Errorf("IsValidPhaseTransition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAllValidTransitions(t *testing.T) {
	tests := []struct {
		name     string
		phase    ShardPhase
		expected int // Number of valid transitions
	}{
		{
			name:     "pending phase",
			phase:    ShardPhasePending,
			expected: 2, // Running, Failed
		},
		{
			name:     "running phase",
			phase:    ShardPhaseRunning,
			expected: 2, // Draining, Failed
		},
		{
			name:     "draining phase",
			phase:    ShardPhaseDraining,
			expected: 2, // Terminated, Running
		},
		{
			name:     "failed phase",
			phase:    ShardPhaseFailed,
			expected: 2, // Running, Terminated
		},
		{
			name:     "terminated phase",
			phase:    ShardPhaseTerminated,
			expected: 0, // No valid transitions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transitions := GetAllValidTransitions(tt.phase)
			if len(transitions) != tt.expected {
				t.Errorf("GetAllValidTransitions() returned %d transitions, expected %d", len(transitions), tt.expected)
			}
		})
	}
}

// Test DNS validation helper
func TestIsValidDNS1123Subdomain(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "valid subdomain",
			value: "shard-1",
			want:  true,
		},
		{
			name:  "valid with dots",
			value: "shard-1.example.com",
			want:  true,
		},
		{
			name:  "empty string",
			value: "",
			want:  false,
		},
		{
			name:  "starts with dash",
			value: "-shard",
			want:  false,
		},
		{
			name:  "ends with dash",
			value: "shard-",
			want:  false,
		},
		{
			name:  "uppercase letters",
			value: "Shard-1",
			want:  false,
		},
		{
			name:  "underscore",
			value: "shard_1",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidDNS1123Subdomain(tt.value)
			if got != tt.want {
				t.Errorf("isValidDNS1123Subdomain() = %v, want %v", got, tt.want)
			}
		})
	}
}