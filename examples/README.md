# Shard Controller Examples

This directory contains example configurations for the Kubernetes Shard Controller.

## Files

### basic-shardconfig.yaml
A basic ShardConfig custom resource example suitable for development and testing environments.

**Features:**
- 2-6 shard scaling range
- Conservative scaling thresholds
- Consistent hash load balancing
- Basic resource limits

**Usage:**
```bash
kubectl apply -f basic-shardconfig.yaml
```

### production-values.yaml
Production-ready Helm values configuration with optimized settings for production workloads.

**Features:**
- High availability configuration
- Production resource limits
- Node affinity and anti-affinity rules
- Enhanced monitoring and security
- Optimized scaling parameters

**Usage:**
```bash
helm install shard-controller shard-controller/shard-controller \
  --namespace shard-system \
  --create-namespace \
  --values production-values.yaml
```

## Configuration Examples

### Development Environment
```yaml
# Quick setup for development
manager:
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
worker:
  replicaCount: 2
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
```

### High-Load Production Environment
```yaml
# Configuration for high-load scenarios
manager:
  config:
    scaling:
      maxShards: 50
      scaleUpThreshold: 0.6
worker:
  replicaCount: 10
  resources:
    limits:
      cpu: 4000m
      memory: 4Gi
```

### Multi-Zone Deployment
```yaml
# Spread across availability zones
worker:
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/component
            operator: In
            values:
            - worker
        topologyKey: topology.kubernetes.io/zone
```

## Custom Resource Examples

### Advanced ShardConfig
```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: advanced-config
spec:
  minShards: 5
  maxShards: 25
  scaleUpThreshold: 0.8
  scaleDownThreshold: 0.2
  loadBalanceStrategy: "least-loaded"
  customMetrics:
    enabled: true
    metrics:
      - name: "queue_depth"
        weight: 0.5
      - name: "processing_time"
        weight: 0.3
      - name: "error_rate"
        weight: 0.2
```

### Monitoring Configuration
```yaml
# Enable comprehensive monitoring
monitoring:
  enabled: true
  prometheus:
    enabled: true
    serviceMonitor:
      enabled: true
      labels:
        prometheus: kube-prometheus
  grafana:
    enabled: true
    dashboard:
      enabled: true
```

## Best Practices

1. **Start Small**: Begin with conservative settings and scale up based on observed behavior
2. **Monitor First**: Enable monitoring before deploying to production
3. **Test Scaling**: Verify scaling behavior in a staging environment
4. **Resource Planning**: Plan for peak load scenarios
5. **Security**: Use appropriate security contexts and RBAC permissions

## Troubleshooting

If you encounter issues with these examples:

1. Check the [troubleshooting guide](../docs/troubleshooting.md)
2. Verify your Kubernetes version compatibility
3. Ensure proper RBAC permissions
4. Check resource availability in your cluster