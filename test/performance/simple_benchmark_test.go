package performance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
	"github.com/k8s-shard-controller/pkg/controllers"
)

// TestBasicPerformance tests basic performance characteristics
func TestBasicPerformance(t *testing.T) {
	ctx := context.Background()
	
	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()
	
	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := &MockConfig{Namespace: "test"}

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(t, err)

	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           1,
			MaxShards:           100,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}

	// Test shard creation performance
	start := time.Now()
	_, err = shardManager.CreateShard(ctx, shardConfig)
	duration := time.Since(start)
	
	require.NoError(t, err)
	t.Logf("Shard creation took: %v", duration)
	
	// Should be reasonably fast in test environment
	require.Less(t, duration, 1*time.Second, "Shard creation should be fast")
}

// BenchmarkShardCreation benchmarks shard creation
func BenchmarkShardCreation(b *testing.B) {
	ctx := context.Background()
	
	// Setup test environment
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeKubeClient := kubefake.NewSimpleClientset()
	
	// Create mock dependencies
	loadBalancer := &MockLoadBalancer{}
	healthChecker := &MockHealthChecker{}
	resourceMigrator := &MockResourceMigrator{}
	configManager := &MockConfigManager{}
	metricsCollector := &MockMetricsCollector{}
	alertManager := &MockAlertManager{}
	logger := &MockLogger{}
	config := &MockConfig{Namespace: "test"}

	shardManager, err := controllers.NewShardManager(
		fakeClient, fakeKubeClient, loadBalancer, healthChecker,
		resourceMigrator, configManager, metricsCollector,
		alertManager, logger, config,
	)
	require.NoError(b, err)

	shardConfig := &shardv1.ShardConfig{
		Spec: shardv1.ShardConfigSpec{
			MinShards:           1,
			MaxShards:           100,
			ScaleUpThreshold:    0.8,
			ScaleDownThreshold:  0.2,
			HealthCheckInterval: metav1.Duration{Duration: 30 * time.Second},
			LoadBalanceStrategy: shardv1.ConsistentHashStrategy,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := shardManager.CreateShard(ctx, shardConfig)
		if err != nil {
			b.Fatalf("Failed to create shard: %v", err)
		}
	}
}