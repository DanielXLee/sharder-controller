# User Guide

## Overview

This guide covers how to use the Kubernetes Shard Controller to manage distributed workloads with automatic sharding, scaling, and load balancing.

## Getting Started

### Prerequisites

Before using the Shard Controller, ensure you have:

- Kubernetes cluster (v1.19+)
- kubectl configured to access your cluster
- Shard Controller installed (see [Installation Guide](installation.md))

### Basic Concepts

#### ShardConfig
A `ShardConfig` defines how your workload should be sharded:
- **Scaling parameters**: Min/max shards, thresholds
- **Load balancing strategy**: How resources are distributed
- **Resource specifications**: CPU/memory requirements
- **Health check settings**: Monitoring configuration

#### ShardInstance
A `ShardInstance` represents an individual shard:
- **Shard ID**: Unique identifier
- **Hash range**: Portion of hash space assigned
- **Status**: Current state and health information
- **Assigned resources**: Resources currently handled

## Creating Your First ShardConfig

### Basic Configuration

```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: my-app-shards
  namespace: production
spec:
  # Scaling configuration
  minShards: 2
  maxShards: 10
  scaleUpThreshold: 0.8
  scaleDownThreshold: 0.3
  
  # Load balancing
  loadBalanceStrategy: "consistent-hash"
  
  # Health monitoring
  healthCheckInterval: "30s"
  
  # Resource specifications
  resources:
    requests:
      cpu: "200m"
      memory: "256Mi"
    limits:
      cpu: "500m"
      memory: "512Mi"
```

Apply the configuration:

```bash
kubectl apply -f my-shardconfig.yaml
```

### Verify Creation

```bash
# Check ShardConfig status
kubectl get shardconfigs -n production

# Check created ShardInstances
kubectl get shardinstances -n production

# View detailed status
kubectl describe shardconfig my-app-shards -n production
```

## Configuration Options

### Scaling Parameters

```yaml
spec:
  minShards: 1              # Minimum number of shards (required)
  maxShards: 20             # Maximum number of shards (required)
  scaleUpThreshold: 0.75    # Load threshold to scale up (0.0-1.0)
  scaleDownThreshold: 0.25  # Load threshold to scale down (0.0-1.0)
  cooldownPeriod: "300s"    # Wait time between scaling operations
```

### Load Balancing Strategies

#### Consistent Hash (Recommended)
```yaml
spec:
  loadBalanceStrategy: "consistent-hash"
  rebalanceThreshold: 0.2   # Trigger rebalancing when load differs by 20%
```

**Best for**: Stable resource assignment, minimal data movement during scaling

#### Round Robin
```yaml
spec:
  loadBalanceStrategy: "round-robin"
```

**Best for**: Even distribution of new resources, simple workloads

#### Least Loaded
```yaml
spec:
  loadBalanceStrategy: "least-loaded"
  rebalanceInterval: "60s"  # How often to check for rebalancing
```

**Best for**: Dynamic workloads with varying resource requirements

### Health Check Configuration

```yaml
spec:
  healthCheckInterval: "30s"    # How often to check shard health
  healthCheckTimeout: "10s"     # Timeout for health checks
  failureThreshold: 3           # Failures before marking unhealthy
  recoveryThreshold: 2          # Successes before marking healthy
```

### Resource Specifications

```yaml
spec:
  resources:
    requests:
      cpu: "200m"
      memory: "256Mi"
    limits:
      cpu: "1000m"
      memory: "1Gi"
  
  # Optional: Node selection
  nodeSelector:
    workload-type: "compute-intensive"
  
  # Optional: Tolerations
  tolerations:
  - key: "dedicated"
    operator: "Equal"
    value: "shard-workload"
    effect: "NoSchedule"
```

## Advanced Configuration

### Custom Metrics for Scaling

```yaml
spec:
  customMetrics:
    enabled: true
    metrics:
      - name: "queue_depth"
        weight: 0.5
        threshold: 100
      - name: "processing_time"
        weight: 0.3
        threshold: 5000  # milliseconds
      - name: "error_rate"
        weight: 0.2
        threshold: 0.05  # 5%
```

### Migration Settings

```yaml
spec:
  migration:
    timeout: "600s"           # Maximum time for migration
    retryAttempts: 3          # Number of retry attempts
    retryDelay: "30s"         # Delay between retries
    batchSize: 100            # Resources to migrate per batch
```

### Security Configuration

```yaml
spec:
  security:
    podSecurityContext:
      runAsNonRoot: true
      runAsUser: 65534
      fsGroup: 65534
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
        - ALL
```

## Monitoring and Observability

### Checking Shard Status

```bash
# List all ShardConfigs
kubectl get shardconfigs --all-namespaces

# List all ShardInstances
kubectl get shardinstances --all-namespaces

# Get detailed status
kubectl describe shardconfig my-app-shards -n production
```

### Viewing Metrics

```bash
# Port forward to metrics endpoint
kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080

# View metrics
curl http://localhost:8080/metrics | grep shard_controller
```

### Key Metrics to Monitor

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `shard_controller_shards_total` | Total active shards | Monitor for unexpected changes |
| `shard_controller_healthy_shards` | Healthy shards count | Alert if < 80% of total |
| `shard_controller_load_average` | Average load across shards | Alert if > 0.9 sustained |
| `shard_controller_scaling_operations_total` | Scaling operations count | Monitor for excessive scaling |
| `shard_controller_migration_failures_total` | Failed migrations | Alert on any failures |

### Viewing Logs

```bash
# Manager logs
kubectl logs -n shard-system deployment/shard-manager -f

# Worker logs
kubectl logs -n shard-system deployment/shard-worker -f

# Specific shard instance logs
kubectl logs -n shard-system -l app=shard-worker --tail=100
```

## Common Use Cases

### Web Application Sharding

```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: webapp-shards
  namespace: production
spec:
  minShards: 3
  maxShards: 15
  scaleUpThreshold: 0.7
  scaleDownThreshold: 0.3
  loadBalanceStrategy: "consistent-hash"
  healthCheckInterval: "20s"
  resources:
    requests:
      cpu: "300m"
      memory: "512Mi"
    limits:
      cpu: "1000m"
      memory: "1Gi"
```

### Data Processing Pipeline

```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: data-processor-shards
  namespace: data-pipeline
spec:
  minShards: 5
  maxShards: 50
  scaleUpThreshold: 0.6
  scaleDownThreshold: 0.2
  loadBalanceStrategy: "least-loaded"
  resources:
    requests:
      cpu: "1000m"
      memory: "2Gi"
    limits:
      cpu: "4000m"
      memory: "8Gi"
  customMetrics:
    enabled: true
    metrics:
      - name: "queue_depth"
        weight: 0.6
        threshold: 1000
      - name: "processing_time"
        weight: 0.4
        threshold: 10000
```

### Development Environment

```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: dev-shards
  namespace: development
spec:
  minShards: 1
  maxShards: 3
  scaleUpThreshold: 0.9
  scaleDownThreshold: 0.1
  loadBalanceStrategy: "round-robin"
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "200m"
      memory: "256Mi"
```

## Scaling Operations

### Manual Scaling

```bash
# Scale up by increasing maxShards
kubectl patch shardconfig my-app-shards -n production --type='merge' \
  -p='{"spec":{"maxShards":15}}'

# Scale down by decreasing maxShards
kubectl patch shardconfig my-app-shards -n production --type='merge' \
  -p='{"spec":{"maxShards":5}}'
```

### Adjusting Scaling Thresholds

```bash
# Make scaling more aggressive
kubectl patch shardconfig my-app-shards -n production --type='merge' \
  -p='{"spec":{"scaleUpThreshold":0.6,"scaleDownThreshold":0.4}}'

# Make scaling more conservative
kubectl patch shardconfig my-app-shards -n production --type='merge' \
  -p='{"spec":{"scaleUpThreshold":0.9,"scaleDownThreshold":0.1}}'
```

## Troubleshooting

### Common Issues

#### Shards Not Scaling
1. Check load metrics: `kubectl get shardinstances -o wide`
2. Verify thresholds: `kubectl describe shardconfig <name>`
3. Check controller logs: `kubectl logs -n shard-system deployment/shard-manager`

#### Migration Failures
1. Check migration timeout settings
2. Verify resource availability
3. Review worker logs for errors

#### Health Check Failures
1. Verify health check endpoints
2. Check network connectivity
3. Review timeout settings

### Debugging Commands

```bash
# Check controller status
kubectl get pods -n shard-system

# View events
kubectl get events --sort-by=.metadata.creationTimestamp

# Check resource usage
kubectl top pods -n shard-system

# Describe problematic resources
kubectl describe shardinstance <instance-name>
```

## Best Practices

### Configuration
1. **Start Conservative**: Begin with higher thresholds and longer cooldown periods
2. **Monitor First**: Enable monitoring before production deployment
3. **Test Scaling**: Verify scaling behavior in staging environment
4. **Resource Planning**: Plan for peak load scenarios

### Operations
1. **Gradual Changes**: Make incremental configuration changes
2. **Monitor Metrics**: Set up alerting for key metrics
3. **Regular Reviews**: Periodically review and optimize configurations
4. **Backup Configs**: Keep backup copies of working configurations

### Security
1. **Least Privilege**: Use minimal required RBAC permissions
2. **Network Policies**: Implement network segmentation
3. **Security Contexts**: Use appropriate security contexts
4. **Regular Updates**: Keep controller updated with security patches

## Migration Guide

### Upgrading Configurations

When upgrading the controller or changing configurations:

1. **Backup Current State**:
   ```bash
   kubectl get shardconfigs -o yaml > backup-shardconfigs.yaml
   ```

2. **Test Changes**: Apply changes in staging environment first

3. **Rolling Updates**: Update configurations gradually

4. **Monitor Impact**: Watch metrics during and after changes

5. **Rollback Plan**: Have a rollback plan ready

### Version Compatibility

Check the [compatibility matrix](installation.md#version-compatibility) before upgrading.

## Next Steps

- Review [Configuration Guide](configuration.md) for advanced settings
- Check [Troubleshooting Guide](troubleshooting.md) for common issues
- Explore [Examples](../examples/) for more configuration samples
- Join the community discussions for support and best practices