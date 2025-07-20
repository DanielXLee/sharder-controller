# API Reference

## Overview

This document provides a complete reference for the Kubernetes Shard Controller Custom Resource Definitions (CRDs) and their APIs.

## Custom Resource Definitions

### ShardConfig

The `ShardConfig` resource defines the desired sharding configuration for a workload.

#### API Version
- **Group**: `shard.io`
- **Version**: `v1`
- **Kind**: `ShardConfig`

#### Specification

```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: string                    # Required: Name of the ShardConfig
  namespace: string               # Required: Namespace
  labels: map[string]string       # Optional: Labels
  annotations: map[string]string  # Optional: Annotations
spec:
  # Scaling Configuration (Required)
  minShards: integer              # Minimum number of shards (>= 1)
  maxShards: integer              # Maximum number of shards (> minShards)
  scaleUpThreshold: float         # Load threshold to scale up (0.0-1.0)
  scaleDownThreshold: float       # Load threshold to scale down (0.0-1.0)
  cooldownPeriod: duration        # Wait time between scaling operations
  
  # Load Balancing Configuration
  loadBalanceStrategy: string     # Strategy: consistent-hash, round-robin, least-loaded
  rebalanceThreshold: float       # Load difference threshold for rebalancing
  rebalanceInterval: duration     # How often to check for rebalancing
  
  # Health Check Configuration
  healthCheckInterval: duration   # How often to check shard health
  healthCheckTimeout: duration    # Timeout for health checks
  failureThreshold: integer       # Failures before marking unhealthy
  recoveryThreshold: integer      # Successes before marking healthy
  
  # Resource Configuration
  resources: ResourceRequirements # Kubernetes resource requirements
  nodeSelector: map[string]string # Node selection constraints
  tolerations: []Toleration       # Pod tolerations
  affinity: Affinity             # Pod affinity/anti-affinity
  
  # Migration Configuration
  migration: MigrationConfig      # Migration settings
  
  # Custom Metrics Configuration
  customMetrics: CustomMetricsConfig # Custom metrics for scaling
  
  # Security Configuration
  security: SecurityConfig        # Security settings
  
  # Additional Configuration
  gracefulShutdownTimeout: duration # Graceful shutdown timeout
status:
  # Status fields (read-only)
  phase: string                   # Current phase: Pending, Running, Scaling, Failed
  currentShards: integer          # Current number of shards
  desiredShards: integer          # Desired number of shards
  readyShards: integer           # Number of ready shards
  conditions: []Condition         # Status conditions
  lastScaleTime: timestamp        # Last scaling operation time
  observedGeneration: integer     # Last observed generation
```

#### Field Descriptions

##### Scaling Configuration

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `minShards` | integer | Yes | Minimum number of shards | - |
| `maxShards` | integer | Yes | Maximum number of shards | - |
| `scaleUpThreshold` | float | No | Load threshold to trigger scale up (0.0-1.0) | 0.8 |
| `scaleDownThreshold` | float | No | Load threshold to trigger scale down (0.0-1.0) | 0.3 |
| `cooldownPeriod` | duration | No | Wait time between scaling operations | 300s |

##### Load Balancing Configuration

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `loadBalanceStrategy` | string | No | Load balancing strategy | consistent-hash |
| `rebalanceThreshold` | float | No | Load difference threshold for rebalancing | 0.2 |
| `rebalanceInterval` | duration | No | How often to check for rebalancing | 60s |

**Load Balance Strategies:**
- `consistent-hash`: Uses consistent hashing for stable resource assignment
- `round-robin`: Distributes resources evenly in round-robin fashion
- `least-loaded`: Assigns resources to the least loaded shard

##### Health Check Configuration

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `healthCheckInterval` | duration | No | How often to check shard health | 30s |
| `healthCheckTimeout` | duration | No | Timeout for health checks | 10s |
| `failureThreshold` | integer | No | Failures before marking unhealthy | 3 |
| `recoveryThreshold` | integer | No | Successes before marking healthy | 2 |

##### Migration Configuration

```yaml
migration:
  timeout: duration           # Maximum time for migration (default: 600s)
  retryAttempts: integer     # Number of retry attempts (default: 3)
  retryDelay: duration       # Delay between retries (default: 30s)
  batchSize: integer         # Resources to migrate per batch (default: 100)
  strategy: string           # Migration strategy: graceful, immediate (default: graceful)
```

##### Custom Metrics Configuration

```yaml
customMetrics:
  enabled: boolean           # Enable custom metrics (default: false)
  metrics:
  - name: string            # Metric name
    weight: float           # Weight in scaling decision (0.0-1.0)
    threshold: float        # Threshold value
    aggregation: string     # Aggregation method: avg, max, min (default: avg)
```

##### Security Configuration

```yaml
security:
  podSecurityContext: PodSecurityContext     # Pod security context
  securityContext: SecurityContext          # Container security context
  networkPolicy: NetworkPolicySpec          # Network policy specification
```

#### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase: Pending, Running, Scaling, Failed |
| `currentShards` | integer | Current number of active shards |
| `desiredShards` | integer | Desired number of shards |
| `readyShards` | integer | Number of ready shards |
| `conditions` | []Condition | Status conditions |
| `lastScaleTime` | timestamp | Time of last scaling operation |
| `observedGeneration` | integer | Last observed generation |

#### Conditions

| Type | Status | Reason | Description |
|------|--------|--------|-------------|
| `Ready` | True/False | Various | ShardConfig is ready and operational |
| `Scaling` | True/False | ScaleUp/ScaleDown | Scaling operation in progress |
| `Healthy` | True/False | HealthCheck | All shards are healthy |
| `LoadBalanced` | True/False | Rebalancing | Load is balanced across shards |

### ShardInstance

The `ShardInstance` resource represents an individual shard instance.

#### API Version
- **Group**: `shard.io`
- **Version**: `v1`
- **Kind**: `ShardInstance`

#### Specification

```yaml
apiVersion: shard.io/v1
kind: ShardInstance
metadata:
  name: string                    # Required: Name of the ShardInstance
  namespace: string               # Required: Namespace
  labels: map[string]string       # Optional: Labels
  annotations: map[string]string  # Optional: Annotations
  ownerReferences: []OwnerReference # Owner references (typically ShardConfig)
spec:
  shardId: string                 # Unique shard identifier
  shardConfigRef: ObjectReference # Reference to parent ShardConfig
  hashRange: HashRange           # Hash range assigned to this shard
  resources: ResourceRequirements # Resource requirements for this shard
  nodeSelector: map[string]string # Node selection constraints
  tolerations: []Toleration      # Pod tolerations
  affinity: Affinity            # Pod affinity/anti-affinity
status:
  phase: string                  # Current phase: Pending, Running, Draining, Failed
  load: float                   # Current load (0.0-1.0)
  capacity: float               # Current capacity (0.0-1.0)
  lastHeartbeat: timestamp      # Last heartbeat time
  assignedResources: []string   # List of assigned resource IDs
  conditions: []Condition       # Status conditions
  podRef: ObjectReference       # Reference to the underlying pod
  startTime: timestamp          # Shard start time
  readyTime: timestamp          # Time when shard became ready
```

#### Field Descriptions

##### Specification Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `shardId` | string | Yes | Unique identifier for the shard |
| `shardConfigRef` | ObjectReference | Yes | Reference to parent ShardConfig |
| `hashRange` | HashRange | Yes | Hash range assigned to this shard |
| `resources` | ResourceRequirements | No | Resource requirements |
| `nodeSelector` | map[string]string | No | Node selection constraints |
| `tolerations` | []Toleration | No | Pod tolerations |
| `affinity` | Affinity | No | Pod affinity/anti-affinity |

##### Hash Range

```yaml
hashRange:
  start: integer    # Start of hash range (inclusive)
  end: integer      # End of hash range (inclusive)
```

##### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current phase: Pending, Running, Draining, Failed |
| `load` | float | Current load as percentage (0.0-1.0) |
| `capacity` | float | Current capacity as percentage (0.0-1.0) |
| `lastHeartbeat` | timestamp | Time of last heartbeat from worker |
| `assignedResources` | []string | List of resource IDs assigned to this shard |
| `conditions` | []Condition | Status conditions |
| `podRef` | ObjectReference | Reference to the underlying pod |
| `startTime` | timestamp | Time when shard was started |
| `readyTime` | timestamp | Time when shard became ready |

#### Phases

| Phase | Description |
|-------|-------------|
| `Pending` | Shard is being created |
| `Running` | Shard is active and processing resources |
| `Draining` | Shard is being drained for shutdown/migration |
| `Failed` | Shard has failed and needs intervention |

#### Conditions

| Type | Status | Reason | Description |
|------|--------|--------|-------------|
| `Ready` | True/False | Various | Shard is ready to process resources |
| `Healthy` | True/False | HealthCheck | Shard is healthy |
| `LoadBalanced` | True/False | Migration | Shard load is within acceptable range |

## Common Types

### ResourceRequirements

Standard Kubernetes resource requirements:

```yaml
resources:
  requests:
    cpu: string      # CPU request (e.g., "200m")
    memory: string   # Memory request (e.g., "256Mi")
  limits:
    cpu: string      # CPU limit (e.g., "500m")
    memory: string   # Memory limit (e.g., "512Mi")
```

### Condition

Standard Kubernetes condition:

```yaml
conditions:
- type: string              # Condition type
  status: string            # True, False, or Unknown
  reason: string            # Machine-readable reason
  message: string           # Human-readable message
  lastTransitionTime: timestamp # Last transition time
  lastUpdateTime: timestamp     # Last update time
```

### ObjectReference

Standard Kubernetes object reference:

```yaml
objectReference:
  apiVersion: string    # API version
  kind: string         # Resource kind
  name: string         # Resource name
  namespace: string    # Resource namespace
  uid: string          # Resource UID
```

## API Operations

### Supported Operations

#### ShardConfig
- `GET` - Retrieve ShardConfig(s)
- `POST` - Create new ShardConfig
- `PUT` - Update existing ShardConfig
- `PATCH` - Partially update ShardConfig
- `DELETE` - Delete ShardConfig
- `WATCH` - Watch for changes

#### ShardInstance
- `GET` - Retrieve ShardInstance(s)
- `PATCH` - Update status (controller only)
- `DELETE` - Delete ShardInstance (controller only)
- `WATCH` - Watch for changes

### Examples

#### Create ShardConfig

```bash
kubectl apply -f - <<EOF
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: example-shards
  namespace: default
spec:
  minShards: 2
  maxShards: 10
  scaleUpThreshold: 0.8
  scaleDownThreshold: 0.3
  loadBalanceStrategy: "consistent-hash"
EOF
```

#### Get ShardConfig

```bash
# Get all ShardConfigs
kubectl get shardconfigs

# Get specific ShardConfig
kubectl get shardconfig example-shards -o yaml

# Get ShardConfig status
kubectl get shardconfig example-shards -o jsonpath='{.status.phase}'
```

#### Update ShardConfig

```bash
# Update scaling thresholds
kubectl patch shardconfig example-shards --type='merge' -p='
{
  "spec": {
    "scaleUpThreshold": 0.7,
    "scaleDownThreshold": 0.4
  }
}'
```

#### Watch Resources

```bash
# Watch ShardConfigs
kubectl get shardconfigs -w

# Watch ShardInstances
kubectl get shardinstances -w
```

## Validation Rules

### ShardConfig Validation

1. `minShards` must be >= 1
2. `maxShards` must be > `minShards`
3. `scaleUpThreshold` must be > `scaleDownThreshold`
4. Thresholds must be between 0.0 and 1.0
5. Duration fields must be valid Go duration strings
6. `loadBalanceStrategy` must be one of: consistent-hash, round-robin, least-loaded

### ShardInstance Validation

1. `shardId` must be unique within namespace
2. `hashRange.start` must be <= `hashRange.end`
3. Hash ranges must not overlap within same ShardConfig
4. `shardConfigRef` must reference existing ShardConfig

## Error Handling

### Common Error Codes

| Code | Reason | Description |
|------|--------|-------------|
| 400 | BadRequest | Invalid resource specification |
| 409 | Conflict | Resource already exists or conflicts |
| 422 | Invalid | Validation failed |
| 500 | InternalError | Controller internal error |

### Error Examples

```yaml
# Validation error response
apiVersion: v1
kind: Status
status: Failure
code: 422
reason: Invalid
message: "ShardConfig.shard.io \"example\" is invalid: spec.minShards: Invalid value: 0: must be greater than 0"
```

## Metrics and Monitoring

### Custom Resource Metrics

The controller exposes metrics for custom resources:

```
# ShardConfig metrics
shard_controller_shardconfigs_total{namespace="default"} 5
shard_controller_shardconfig_shards{name="example",namespace="default"} 3

# ShardInstance metrics
shard_controller_shardinstances_total{namespace="default"} 15
shard_controller_shardinstance_load{name="shard-001",namespace="default"} 0.65
```

### Events

The controller generates Kubernetes events for important operations:

```bash
# View events for ShardConfig
kubectl describe shardconfig example-shards

# View all shard-related events
kubectl get events --field-selector involvedObject.apiVersion=shard.io/v1
```

## Best Practices

### Resource Naming
- Use descriptive names for ShardConfigs
- Include environment or application context
- Follow Kubernetes naming conventions

### Configuration
- Start with conservative scaling thresholds
- Monitor resource usage before adjusting limits
- Use appropriate load balancing strategy for your workload

### Monitoring
- Set up alerts for failed conditions
- Monitor scaling frequency
- Track resource utilization trends

### Security
- Use appropriate RBAC permissions
- Implement network policies
- Follow security best practices for resource specifications