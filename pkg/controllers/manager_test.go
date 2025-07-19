package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/k8s-shard-controller/pkg/config"
)

// newControllerManagerWithClient is a test helper to create a new controller manager with a fake client
func newControllerManagerWithClient(cfg *config.Config, cl client.Client, kubeClient kubernetes.Interface) (*ControllerManager, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	cm := &ControllerManager{
		config:     cfg,
		client:     cl,
		kubeClient: kubeClient,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	return cm, nil
}

func TestNewControllerManager(t *testing.T) {
	cfg := config.DefaultConfig()

	// Create a fake client
	cl := fake.NewClientBuilder().Build()
	kubeClient := kubefake.NewSimpleClientset()

	// Test with default config
	cm, err := newControllerManagerWithClient(cfg, cl, kubeClient)
	require.NoError(t, err)
	assert.NotNil(t, cm)
	assert.False(t, cm.IsRunning())

	// Test with nil config (should use default)
	cm2, err := newControllerManagerWithClient(nil, cl, kubeClient)
	require.NoError(t, err)
	assert.NotNil(t, cm2)
}

func TestControllerManagerValidation(t *testing.T) {
	cfg := config.DefaultConfig()

	// Create a fake client
	cl := fake.NewClientBuilder().Build()
	kubeClient := kubefake.NewSimpleClientset()

	cm, err := newControllerManagerWithClient(cfg, cl, kubeClient)
	require.NoError(t, err)

	// Test configuration validation
	err = cm.validateConfiguration()
	assert.NoError(t, err)

	// Test with invalid config
	cfg.DefaultShardConfig.MinShards = 0
	cm2, err := newControllerManagerWithClient(cfg, cl, kubeClient)
	require.NoError(t, err)

	err = cm2.validateConfiguration()
	assert.Error(t, err)
}

func TestControllerManagerGetters(t *testing.T) {
	cfg := config.DefaultConfig()

	// Create a fake client
	cl := fake.NewClientBuilder().Build()
	kubeClient := kubefake.NewSimpleClientset()

	cm, err := newControllerManagerWithClient(cfg, cl, kubeClient)
	require.NoError(t, err)

	// Test getters return non-nil values
	assert.NotNil(t, cm.GetClient())
	assert.NotNil(t, cm.GetKubeClient())

	// These will be nil until components are initialized in later tasks
	assert.Nil(t, cm.GetShardManager())
	assert.Nil(t, cm.GetLoadBalancer())
	assert.Nil(t, cm.GetHealthChecker())
	assert.Nil(t, cm.GetResourceMigrator())
	assert.Nil(t, cm.GetConfigManager())
	assert.Nil(t, cm.GetMetricsCollector())
}
