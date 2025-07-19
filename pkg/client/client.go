package client

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/k8s-shard-controller/pkg/config"
)

// ClientSet holds all the Kubernetes clients
type ClientSet struct {
	KubeClient kubernetes.Interface
	Client     client.Client
	Config     *rest.Config
}

// NewClientSet creates a new ClientSet from the given configuration
func NewClientSet(cfg *config.Config) (*ClientSet, error) {
	// Create REST config
	restConfig, err := createRestConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	// Create Kubernetes clientset
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create controller-runtime client
	runtimeClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create controller-runtime client: %w", err)
	}

	return &ClientSet{
		KubeClient: kubeClient,
		Client:     runtimeClient,
		Config:     restConfig,
	}, nil
}

// NewManagerWithClientSet creates a controller-runtime manager with the given ClientSet
func NewManagerWithClientSet(cs *ClientSet, cfg *config.Config) (manager.Manager, error) {
	mgr, err := manager.New(cs.Config, manager.Options{
		LeaderElection:     cfg.LeaderElection.Enabled,
		LeaderElectionID:   cfg.LeaderElection.ResourceName,
		LeaseDuration:      &cfg.LeaderElection.LeaseDuration,
		RenewDeadline:      &cfg.LeaderElection.RenewDeadline,
		RetryPeriod:        &cfg.LeaderElection.RetryPeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create controller manager: %w", err)
	}

	return mgr, nil
}

// createRestConfig creates a REST config from the given configuration
func createRestConfig(cfg *config.Config) (*rest.Config, error) {
	var restConfig *rest.Config
	var err error

	if cfg.KubeConfig != "" {
		// Use kubeconfig file
		restConfig, err = clientcmd.BuildConfigFromFlags(cfg.MasterURL, cfg.KubeConfig)
	} else {
		// Use in-cluster config
		restConfig, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	return restConfig, nil
}