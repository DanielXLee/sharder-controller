# Shard Controller Configuration Guide

## Overview

This guide covers detailed configuration options for the Kubernetes Shard Controller, including manager settings, worker configuration, and custom resource definitions.

## Manager Configuration

### Leader Election Settings

```yaml
manager:
  leaderElection:
    enabled: true              # Enable leader election for HA
    leaseDuration: 15s         # How long the leader holds the lease
    renewDeadline: 10s         # Deadline for renewing the lease
    retryPeriod: 2s           # Retry interval for acquiring lease
```

### Health Check Configuration

```yaml
manager:
  healthCheck:
    interval: 30s             # How often to check shard health
    timeout: 10s              # Timeout for health check requests
    failureThreshold: 3       # Consecutive failures before marking unhealthy
```

### Scaling Configuration

```yaml
manager:
  scaling:
    minShards: 1              # Minimum number of shards
    maxShards: 10             # Maximum number of shards
    scaleUpThreshold: 0.8     # Load threshold to trigger scale up (0.0-1.0)
    scaleDownThreshold: 0.3   # Load threshold to trigger scale down (0.0-1.0)
    cooldownPeriod: 300s      # Wait time between scaling operations
```

### Load Balancing Configuration

```yaml
manager:
  loadBalancing:
    strategy: "consistent-hash"  # Options: consistent-hash, round-robin, least-loaded
    rebalanceThreshold: 0.2     # Load difference threshold to trigger rebalancing
```

#### Load Balancing Strategies

1. **consistent-hash**: Uses consistent hashing for stable resource assignment
2. **round-robin**: Distributes resources evenly across shards
3. **least-loaded**: Assigns resources to the shard with lowest current load

## Worker Configuration

### Heartbeat Settings

```yaml
worker:
  heartbeat:
    interval: 10s             # How often to send heartbeat to manager
    timeout: 5s               # Timeout for heartbeat requests
```

### Processing Configuration

```yaml
worker:
  processing:
    batchSize: 100            # Number of resources to process in one batch
    maxConcurrency: 10        # Maximum concurrent processing goroutines
    timeout: 30s              # Timeout for processing individual resources
```

### Migration Settings

```yaml
worker:
  migration:
    timeout: 300s             # Timeout for resource migration operations
    retryAttempts: 3          # Number of retry attempts for failed migrations
    retryDelay: 10s           # Delay between retry attempts
```

## Custom Resource Configuration

### ShardConfig Specification

```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: example-config
  namespace: default
spec:
  # Scaling parameters
  minShards: 2
  maxShards: 8
  scaleUpThreshold: 0.75
  scaleDownThreshold: 0.25
  
  # Health check settings
  healthCheckInterval: "30s"
  
  # Load balancing
  loadBalanceStrategy: "consistent-hash"
  
  # Resource constraints
  resourceLimits:
    cpu: "1000m"
    memory: "1Gi"
  resourceRequests:
    cpu: "200m"
    memory: "256Mi"
```

### ShardInstance Status

```yaml
apiVersion: shard.io/v1
kind: ShardInstance
metadata:
  name: shard-001
  namespace: default
spec:
  shardId: "shard-001"
  hashRange:
    start: 0
    end: 1073741823
status:
  phase: "Running"           # Pending, Running, Draining, Failed
  load: 0.65                # Current load (0.0-1.0)
  lastHeartbeat: "2024-01-15T10:30:00Z"
  assignedResources:
    - "resource-001"
    - "resource-002"
  conditions:
    - type: "Ready"
      status: "True"
      lastTransitionTime: "2024-01-15T10:00:00Z"
```

## Environment Variables

### Manager Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `POD_NAMESPACE` | Current pod namespace | - |
| `POD_NAME` | Current pod name | - |
| `LEADER_ELECTION_NAMESPACE` | Namespace for leader election | Same as pod |
| `METRICS_ADDR` | Metrics server address | `0.0.0.0:8080` |
| `HEALTH_ADDR` | Health server address | `0.0.0.0:8081` |
| `LOG_LEVEL` | Logging level | `info` |

### Worker Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `POD_NAMESPACE` | Current pod namespace | - |
| `POD_NAME` | Current pod name | - |
| `POD_IP` | Current pod IP | - |
| `SHARD_ID` | Unique shard identifier | Pod name |
| `MANAGER_ENDPOINT` | Manager service endpoint | Auto-discovered |

## Resource Requirements

### Recommended Resource Limits

#### Manager Pod
```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

#### Worker Pod
```yaml
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

### Scaling Considerations

- **CPU**: Primarily used for coordination and decision-making
- **Memory**: Scales with number of managed resources and shards
- **Network**: Moderate usage for inter-shard communication

## Security Configuration

### Pod Security Context

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  fsGroup: 65534
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true
```

### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: shard-controller-netpol
  namespace: shard-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: shard-controller
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 443
    - protocol: TCP
      port: 6443
```

## Monitoring Configuration

### Prometheus Metrics

The controller exposes the following metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `shard_controller_shards_total` | Gauge | Total number of shards |
| `shard_controller_healthy_shards` | Gauge | Number of healthy shards |
| `shard_controller_load_average` | Gauge | Average load across all shards |
| `shard_controller_scaling_operations_total` | Counter | Total scaling operations |
| `shard_controller_migration_operations_total` | Counter | Total migration operations |
| `shard_controller_errors_total` | Counter | Total errors by type |

### Log Configuration

```yaml
monitoring:
  logLevel: "info"           # debug, info, warn, error
  logFormat: "json"          # json, console
  logOutput: "stdout"        # stdout, file
```

## Advanced Configuration

### Custom Load Calculation

```yaml
manager:
  loadBalancing:
    customMetrics:
      enabled: true
      metrics:
        - name: "cpu_usage"
          weight: 0.4
        - name: "memory_usage"
          weight: 0.3
        - name: "queue_length"
          weight: 0.3
```

### Failure Recovery

```yaml
manager:
  recovery:
    maxFailedShards: 2        # Maximum failed shards before emergency scaling
    emergencyScaleUp: true    # Enable emergency scaling
    backoffMultiplier: 2.0    # Exponential backoff multiplier
    maxBackoffDelay: 300s     # Maximum backoff delay
```

## Configuration Validation

### Validation Rules

1. `minShards` must be >= 1
2. `maxShards` must be > `minShards`
3. `scaleUpThreshold` must be > `scaleDownThreshold`
4. Thresholds must be between 0.0 and 1.0
5. Timeout values must be positive durations

### Configuration Testing

```bash
# Validate configuration
kubectl apply --dry-run=client -f config.yaml

# Test configuration changes
kubectl patch shardconfig example-config --type='merge' -p='{"spec":{"minShards":3}}'

# Monitor configuration changes
kubectl get events --field-selector reason=ConfigurationChanged
```

## Best Practices

1. **Start Small**: Begin with conservative scaling thresholds
2. **Monitor First**: Enable monitoring before production deployment
3. **Test Scaling**: Verify scaling behavior in staging environment
4. **Resource Planning**: Plan for peak load scenarios
5. **Security**: Use least-privilege RBAC permissions
6. **Backup**: Backup custom resource configurations
7. **Documentation**: Document custom configurations for your environment