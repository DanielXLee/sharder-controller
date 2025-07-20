package interfaces

import (
	"context"
	"time"

	shardv1 "github.com/k8s-shard-controller/pkg/apis/shard/v1"
)

// Resource represents a generic resource that can be managed by shards
type Resource struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Data     map[string]string `json:"data"`
	Metadata map[string]string `json:"metadata"`
}

// ShardManager defines the interface for managing shards
type ShardManager interface {
	// Shard lifecycle management
	CreateShard(ctx context.Context, config *shardv1.ShardConfig) (*shardv1.ShardInstance, error)
	DeleteShard(ctx context.Context, shardId string) error
	
	// Scaling operations
	ScaleUp(ctx context.Context, targetCount int) error
	ScaleDown(ctx context.Context, targetCount int) error
	
	// Load balancing
	RebalanceLoad(ctx context.Context) error
	AssignResource(ctx context.Context, resource *Resource) (string, error)
	
	// Health checking
	CheckShardHealth(ctx context.Context, shardId string) (*shardv1.HealthStatus, error)
	HandleFailedShard(ctx context.Context, shardId string) error
	
	// Status and monitoring
	GetShardStatus(ctx context.Context, shardId string) (*shardv1.ShardInstanceStatus, error)
	ListShards(ctx context.Context) ([]*shardv1.ShardInstance, error)
}

// WorkerShard defines the interface for worker shard operations
type WorkerShard interface {
	// Resource processing
	ProcessResource(ctx context.Context, resource *Resource) error
	GetAssignedResources(ctx context.Context) ([]*Resource, error)
	
	// Health status
	ReportHealth(ctx context.Context) (*shardv1.HealthStatus, error)
	GetLoadMetrics(ctx context.Context) (*shardv1.LoadMetrics, error)
	
	// Resource migration
	MigrateResourcesTo(ctx context.Context, targetShard string, resources []*Resource) error
	AcceptMigratedResources(ctx context.Context, resources []*Resource) error
	
	// Graceful shutdown
	Drain(ctx context.Context) error
	Shutdown(ctx context.Context) error
	
	// Shard identification
	GetShardID() string
	GetHashRange() *shardv1.HashRange
}

// LoadBalancer defines the interface for load balancing operations
type LoadBalancer interface {
	// Load calculation
	CalculateShardLoad(shard *shardv1.ShardInstance) float64
	GetOptimalShard(shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error)
	
	// Strategy management
	SetStrategy(strategy shardv1.LoadBalanceStrategy, shards []*shardv1.ShardInstance)
	
	// Rebalancing
	ShouldRebalance(shards []*shardv1.ShardInstance) bool
	GenerateRebalancePlan(shards []*shardv1.ShardInstance) (*shardv1.MigrationPlan, error)
	
	// Resource assignment
	AssignResourceToShard(resource *Resource, shards []*shardv1.ShardInstance) (*shardv1.ShardInstance, error)
}

// HealthChecker defines the interface for health checking operations
type HealthChecker interface {
	// Health checking
	CheckHealth(ctx context.Context, shard *shardv1.ShardInstance) (*shardv1.HealthStatus, error)
	CheckShardHealth(ctx context.Context, shardId string) (*shardv1.HealthStatus, error)
	StartHealthChecking(ctx context.Context, interval time.Duration) error
	StopHealthChecking() error
	
	// Health status queries
	IsShardHealthy(shardId string) bool
	GetUnhealthyShards() []string
	GetHealthSummary() map[string]*shardv1.HealthStatus
	
	// Failure handling
	OnShardFailed(ctx context.Context, shardId string) error
	OnShardRecovered(ctx context.Context, shardId string) error
}

// ResourceMigrator defines the interface for resource migration operations
type ResourceMigrator interface {
	// Migration planning
	CreateMigrationPlan(ctx context.Context, sourceShard, targetShard string, resources []*Resource) (*shardv1.MigrationPlan, error)
	
	// Migration execution
	ExecuteMigration(ctx context.Context, plan *shardv1.MigrationPlan) error
	
	// Migration monitoring
	GetMigrationStatus(ctx context.Context, planId string) (MigrationStatus, error)
	
	// Migration rollback
	RollbackMigration(ctx context.Context, planId string) error
}

// ConfigManager defines the interface for configuration management
type ConfigManager interface {
	// Configuration loading
	LoadConfig(ctx context.Context) (*shardv1.ShardConfig, error)
	LoadFromConfigMap(ctx context.Context, configMapName, namespace string) (*shardv1.ShardConfig, error)
	ReloadConfig(ctx context.Context) error
	GetCurrentConfig() *shardv1.ShardConfig
	
	// Configuration validation
	ValidateConfig(config *shardv1.ShardConfig) error
	
	// Configuration watching
	WatchConfigChanges(ctx context.Context, callback func(*shardv1.ShardConfig)) error
	StartWatching(ctx context.Context) error
	StopWatching()
}

// MetricsCollector defines the interface for metrics collection
type MetricsCollector interface {
	// Metrics collection
	CollectShardMetrics(shard *shardv1.ShardInstance) error
	CollectSystemMetrics(shards []*shardv1.ShardInstance) error
	
	// Metrics exposure
	ExposeMetrics() error
	StartMetricsServer(ctx context.Context) error
	
	// Event recording
	RecordMigration(sourceShard, targetShard, status string, duration time.Duration)
	RecordScaleOperation(operation, status string)
	RecordError(component, errorType string)
	RecordOperationDuration(operation, component string, duration time.Duration)
	UpdateQueueLength(queueType, shardID string, length int)
	
	// Custom metrics
	RecordCustomMetric(name string, value float64, labels map[string]string) error
}

// AlertManager defines the interface for alert management
type AlertManager interface {
	// Alert sending
	SendAlert(ctx context.Context, alert Alert) error
	
	// Specific alert types
	AlertShardFailure(ctx context.Context, shardID string, reason string) error
	AlertHighErrorRate(ctx context.Context, component string, errorRate float64, threshold float64) error
	AlertScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string) error
	AlertMigrationFailure(ctx context.Context, sourceShard, targetShard string, resourceCount int, reason string) error
	AlertSystemOverload(ctx context.Context, totalLoad float64, threshold float64, shardCount int) error
	AlertConfigurationChange(ctx context.Context, configType string, changes map[string]interface{}) error
}

// Alert represents an alert to be sent
type Alert struct {
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Severity    AlertSeverity          `json:"severity"`
	Component   string                 `json:"component"`
	Timestamp   time.Time              `json:"timestamp"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]interface{} `json:"annotations"`
}

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	AlertCritical AlertSeverity = "critical"
	AlertWarning  AlertSeverity = "warning"
	AlertInfo     AlertSeverity = "info"
)

// StructuredLogger defines the interface for structured logging
type StructuredLogger interface {
	// Event logging
	LogShardEvent(ctx context.Context, event string, shardID string, fields map[string]interface{})
	LogMigrationEvent(ctx context.Context, sourceShard, targetShard string, resourceCount int, status string, duration time.Duration)
	LogScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string, status string)
	LogHealthEvent(ctx context.Context, shardID string, healthy bool, errorCount int, message string)
	LogErrorEvent(ctx context.Context, component, operation string, err error, fields map[string]interface{})
	LogPerformanceEvent(ctx context.Context, operation, component string, duration time.Duration, success bool)
	LogConfigEvent(ctx context.Context, configType string, changes map[string]interface{})
	LogSystemEvent(ctx context.Context, event string, severity string, fields map[string]interface{})
}

// Alerting defines the interface for alerting operations
type Alerting interface {
	// Alert management
	SendAlert(ctx context.Context, alert Alert) error
	ConfigureWebhook(webhookURL string) error
	
	// Specific alert types
	AlertShardFailure(ctx context.Context, shardID string, reason string) error
	AlertHighErrorRate(ctx context.Context, component string, errorRate float64, threshold float64) error
	AlertScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string) error
	AlertMigrationFailure(ctx context.Context, sourceShard, targetShard string, resourceCount int, reason string) error
	AlertSystemOverload(ctx context.Context, totalLoad float64, threshold float64, shardCount int) error
	AlertConfigurationChange(ctx context.Context, configType string, changes map[string]interface{}) error
}

// Logger defines the interface for logging operations
type Logger interface {
	// Basic logging
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, err error, fields map[string]interface{})
	Debug(msg string, fields map[string]interface{})
	
	// Event logging
	LogShardEvent(ctx context.Context, event string, shardID string, fields map[string]interface{})
	LogMigrationEvent(ctx context.Context, sourceShard, targetShard string, resourceCount int, status string, duration time.Duration)
	LogScaleEvent(ctx context.Context, operation string, fromCount, toCount int, reason string, status string)
	LogHealthEvent(ctx context.Context, shardID string, healthy bool, errorCount int, message string)
	LogErrorEvent(ctx context.Context, component, operation string, err error, fields map[string]interface{})
	LogPerformanceEvent(ctx context.Context, operation, component string, duration time.Duration, success bool)
	LogConfigEvent(ctx context.Context, configType string, changes map[string]interface{})
	LogSystemEvent(ctx context.Context, event string, severity string, fields map[string]interface{})
}

// MigrationStatus represents the status of a migration operation
type MigrationStatus string

const (
	MigrationStatusPending    MigrationStatus = "Pending"
	MigrationStatusInProgress MigrationStatus = "InProgress"
	MigrationStatusCompleted  MigrationStatus = "Completed"
	MigrationStatusFailed     MigrationStatus = "Failed"
	MigrationStatusRolledBack MigrationStatus = "RolledBack"
)