# Shard Controller Installation Guide

## Overview

The Kubernetes Shard Controller is a custom controller that provides dynamic sharding capabilities for Kubernetes workloads. This guide covers installation, configuration, and basic usage.

## Prerequisites

- Kubernetes cluster version 1.19+
- kubectl configured to access your cluster
- Helm 3.0+ (for Helm installation method)
- Cluster admin permissions

## Installation Methods

### Method 1: Helm Installation (Recommended)

#### 1. Add Helm Repository (if available)
```bash
helm repo add shard-controller https://example.com/helm-charts
helm repo update
```

#### 2. Install with Default Values
```bash
helm install shard-controller shard-controller/shard-controller \
  --namespace shard-system \
  --create-namespace
```

#### 3. Install with Custom Values
```bash
# Create custom values file
cat > values-custom.yaml << EOF
manager:
  replicaCount: 1
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi

worker:
  replicaCount: 5
  resources:
    requests:
      cpu: 300m
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 1Gi

monitoring:
  enabled: true
  prometheus:
    enabled: true
    serviceMonitor:
      enabled: true
EOF

# Install with custom values
helm install shard-controller shard-controller/shard-controller \
  --namespace shard-system \
  --create-namespace \
  --values values-custom.yaml
```

### Method 2: Manual YAML Installation

#### 1. Clone Repository
```bash
git clone https://github.com/example/shard-controller.git
cd shard-controller
```

#### 2. Install CRDs
```bash
kubectl apply -f manifests/crds/
```

#### 3. Create Namespace and RBAC
```bash
kubectl apply -f manifests/namespace.yaml
kubectl apply -f manifests/rbac.yaml
```

#### 4. Deploy Configuration
```bash
kubectl apply -f manifests/configmap.yaml
```

#### 5. Deploy Services
```bash
kubectl apply -f manifests/services.yaml
```

#### 6. Deploy Controllers
```bash
kubectl apply -f manifests/deployment/
```

## Configuration

### Manager Configuration

The manager component can be configured through the ConfigMap or Helm values:

```yaml
manager:
  leaderElection:
    enabled: true
    leaseDuration: 15s
    renewDeadline: 10s
    retryPeriod: 2s
  healthCheck:
    interval: 30s
    timeout: 10s
    failureThreshold: 3
  scaling:
    minShards: 1
    maxShards: 10
    scaleUpThreshold: 0.8
    scaleDownThreshold: 0.3
    cooldownPeriod: 300s
  loadBalancing:
    strategy: "consistent-hash"  # Options: consistent-hash, round-robin, least-loaded
    rebalanceThreshold: 0.2
```

### Worker Configuration

```yaml
worker:
  heartbeat:
    interval: 10s
    timeout: 5s
  processing:
    batchSize: 100
    maxConcurrency: 10
    timeout: 30s
  migration:
    timeout: 300s
    retryAttempts: 3
    retryDelay: 10s
```

### Custom Resource Configuration

#### ShardConfig Example
```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: example-shard-config
  namespace: default
spec:
  minShards: 2
  maxShards: 8
  scaleUpThreshold: 0.75
  scaleDownThreshold: 0.25
  healthCheckInterval: "30s"
  loadBalanceStrategy: "consistent-hash"
```

## Verification

### 1. Check Pod Status
```bash
kubectl get pods -n shard-system
```

Expected output:
```
NAME                             READY   STATUS    RESTARTS   AGE
shard-manager-xxx                1/1     Running   0          2m
shard-worker-xxx                 1/1     Running   0          2m
shard-worker-yyy                 1/1     Running   0          2m
shard-worker-zzz                 1/1     Running   0          2m
```

### 2. Check Custom Resources
```bash
kubectl get shardconfigs,shardinstances --all-namespaces
```

### 3. Check Logs
```bash
# Manager logs
kubectl logs -n shard-system deployment/shard-manager

# Worker logs
kubectl logs -n shard-system deployment/shard-worker
```

### 4. Health Check
```bash
# Manager health
kubectl port-forward -n shard-system svc/shard-manager-metrics 8081:8081
curl http://localhost:8081/healthz

# Worker health
kubectl port-forward -n shard-system svc/shard-worker-metrics 8081:8081
curl http://localhost:8081/healthz
```

## Monitoring Setup

### Prometheus Integration

If you have Prometheus installed, enable monitoring:

```yaml
# values.yaml
monitoring:
  enabled: true
  prometheus:
    enabled: true
    serviceMonitor:
      enabled: true
      namespace: monitoring
      interval: 30s
```

### Grafana Dashboard

Import the provided Grafana dashboard:
```bash
kubectl apply -f manifests/monitoring/grafana-dashboard.json
```

## Upgrading

### Helm Upgrade
```bash
helm upgrade shard-controller shard-controller/shard-controller \
  --namespace shard-system \
  --values values-custom.yaml
```

### Manual Upgrade
```bash
# Update CRDs first
kubectl apply -f manifests/crds/

# Update deployments
kubectl apply -f manifests/deployment/
```

## Uninstallation

### Helm Uninstall
```bash
helm uninstall shard-controller --namespace shard-system
kubectl delete namespace shard-system
```

### Manual Uninstall
```bash
kubectl delete -f manifests/deployment/
kubectl delete -f manifests/services.yaml
kubectl delete -f manifests/configmap.yaml
kubectl delete -f manifests/rbac.yaml
kubectl delete -f manifests/namespace.yaml
kubectl delete -f manifests/crds/
```

## Next Steps

- Review the [Configuration Guide](configuration.md) for advanced settings
- Check the [Troubleshooting Guide](troubleshooting.md) if you encounter issues
- See [Usage Examples](examples.md) for common use cases