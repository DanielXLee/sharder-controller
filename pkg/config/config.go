package config

import (
	"time"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
)

// Config represents the configuration for the shard controller
type Config struct {
	// Kubernetes configuration
	KubeConfig string `json:"kubeconfig,omitempty"`
	MasterURL  string `json:"masterURL,omitempty"`
	Namespace  string `json:"namespace"`
	NodeName   string `json:"nodeName"`

	// Controller configuration
	LeaderElection LeaderElectionConfig `json:"leaderElection"`
	HealthCheck    HealthCheckConfig    `json:"healthCheck"`
	Metrics        MetricsConfig        `json:"metrics"`
	Alerting       AlertingConfig       `json:"alerting"`

	// Shard configuration defaults
	DefaultShardConfig ShardConfig `json:"defaultShardConfig"`

	// Logging configuration
	LogLevel  string `json:"logLevel"`
	LogFormat string `json:"logFormat"`
}

// ShardConfig represents shard configuration (separate from CRD)
type ShardConfig struct {
	MinShards               int                         `json:"minShards"`
	MaxShards               int                         `json:"maxShards"`
	ScaleUpThreshold        float64                     `json:"scaleUpThreshold"`
	ScaleDownThreshold      float64                     `json:"scaleDownThreshold"`
	HealthCheckInterval     time.Duration               `json:"healthCheckInterval"`
	LoadBalanceStrategy     shardv1.LoadBalanceStrategy `json:"loadBalanceStrategy"`
	GracefulShutdownTimeout time.Duration               `json:"gracefulShutdownTimeout"`
}

// LeaderElectionConfig defines leader election configuration
type LeaderElectionConfig struct {
	Enabled           bool          `json:"enabled"`
	LeaseDuration     time.Duration `json:"leaseDuration"`
	RenewDeadline     time.Duration `json:"renewDeadline"`
	RetryPeriod       time.Duration `json:"retryPeriod"`
	ResourceLock      string        `json:"resourceLock"`
	ResourceName      string        `json:"resourceName"`
	ResourceNamespace string        `json:"resourceNamespace"`
}

// HealthCheckConfig defines health check configuration
type HealthCheckConfig struct {
	Interval         time.Duration `json:"interval"`
	Timeout          time.Duration `json:"timeout"`
	FailureThreshold int           `json:"failureThreshold"`
	SuccessThreshold int           `json:"successThreshold"`
	Port             int           `json:"port"`
	Path             string        `json:"path"`
}

// MetricsConfig defines metrics configuration
type MetricsConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Path    string `json:"path"`
}

// AlertingConfig defines alerting configuration
type AlertingConfig struct {
	Enabled    bool          `json:"enabled"`
	WebhookURL string        `json:"webhookUrl"`
	Timeout    time.Duration `json:"timeout"`
	RetryCount int           `json:"retryCount"`
	RetryDelay time.Duration `json:"retryDelay"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Namespace: "default",
		NodeName:  "shard-controller-node",
		LeaderElection: LeaderElectionConfig{
			Enabled:           true,
			LeaseDuration:     15 * time.Second,
			RenewDeadline:     10 * time.Second,
			RetryPeriod:       2 * time.Second,
			ResourceLock:      "leases",
			ResourceName:      "shard-controller-leader",
			ResourceNamespace: "default",
		},
		HealthCheck: HealthCheckConfig{
			Interval:         30 * time.Second,
			Timeout:          5 * time.Second,
			FailureThreshold: 3,
			SuccessThreshold: 1,
			Port:             8080,
			Path:             "/healthz",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    8081,
			Path:    "/metrics",
		},
		Alerting: AlertingConfig{
			Enabled:    false,
			WebhookURL: "",
			Timeout:    10 * time.Second,
			RetryCount: 3,
			RetryDelay: 5 * time.Second,
		},
		DefaultShardConfig: ShardConfig{
			MinShards:               1,
			MaxShards:               10,
			ScaleUpThreshold:        0.8,
			ScaleDownThreshold:      0.3,
			HealthCheckInterval:     30 * time.Second,
			LoadBalanceStrategy:     shardv1.ConsistentHashStrategy,
			GracefulShutdownTimeout: 30 * time.Second,
		},
		LogLevel:  "info",
		LogFormat: "json",
	}
}
