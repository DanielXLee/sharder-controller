# Troubleshooting Guide

## Overview

This guide helps you diagnose and resolve common issues with the Kubernetes Shard Controller.

## Quick Diagnostics

### Health Check Commands

```bash
# Check controller pod status
kubectl get pods -n shard-system

# Check controller logs
kubectl logs -n shard-system deployment/shard-manager --tail=50
kubectl logs -n shard-system deployment/shard-worker --tail=50

# Check custom resources
kubectl get shardconfigs,shardinstances --all-namespaces

# Check events
kubectl get events --sort-by=.metadata.creationTimestamp --all-namespaces | grep -i shard
```

### Health Endpoints

```bash
# Port forward to health endpoints
kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080 &
kubectl port-forward -n shard-system svc/shard-manager-metrics 8081:8081 &

# Check health
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz

# Check metrics
curl http://localhost:8080/metrics | grep shard_controller
```

## Common Issues

### Installation Issues

#### Issue: CRDs Not Installing

**Symptoms:**
```
error validating data: ValidationError(ShardConfig): unknown field "spec"
```

**Diagnosis:**
```bash
# Check if CRDs are installed
kubectl get crd | grep shard.io

# Check CRD status
kubectl describe crd shardconfigs.shard.io
```

**Solution:**
```bash
# Reinstall CRDs
kubectl apply -f manifests/crds/

# Wait for CRDs to be established
kubectl wait --for condition=established --timeout=60s crd/shardconfigs.shard.io
kubectl wait --for condition=established --timeout=60s crd/shardinstances.shard.io
```

#### Issue: RBAC Permissions Denied

**Symptoms:**
```
forbidden: User "system:serviceaccount:shard-system:shard-manager" cannot create resource "shardinstances"
```

**Diagnosis:**
```bash
# Check service accounts
kubectl get sa -n shard-system

# Check cluster role bindings
kubectl get clusterrolebinding | grep shard

# Check permissions
kubectl auth can-i create shardinstances --as=system:serviceaccount:shard-system:shard-manager
```

**Solution:**
```bash
# Reapply RBAC configuration
kubectl apply -f manifests/rbac.yaml

# Verify permissions
kubectl describe clusterrole shard-manager
kubectl describe clusterrolebinding shard-manager
```

#### Issue: Controller Pods Not Starting

**Symptoms:**
```
Pod shard-manager-xxx is in CrashLoopBackOff state
```

**Diagnosis:**
```bash
# Check pod status
kubectl describe pod -n shard-system -l app=shard-manager

# Check logs
kubectl logs -n shard-system -l app=shard-manager --previous

# Check resource usage
kubectl top pods -n shard-system
```

**Common Causes and Solutions:**

1. **Insufficient Resources:**
   ```bash
   # Check node resources
   kubectl describe nodes
   
   # Reduce resource requests
   kubectl patch deployment shard-manager -n shard-system -p='
   {
     "spec": {
       "template": {
         "spec": {
           "containers": [{
             "name": "manager",
             "resources": {
               "requests": {"cpu": "50m", "memory": "64Mi"}
             }
           }]
         }
       }
     }
   }'
   ```

2. **Configuration Issues:**
   ```bash
   # Check ConfigMap
   kubectl describe configmap shard-manager-config -n shard-system
   
   # Validate configuration
   kubectl get configmap shard-manager-config -n shard-system -o yaml
   ```

3. **Image Pull Issues:**
   ```bash
   # Check image pull policy
   kubectl describe pod -n shard-system -l app=shard-manager | grep -A5 "Image:"
   
   # Use local images for development
   kubectl patch deployment shard-manager -n shard-system -p='
   {
     "spec": {
       "template": {
         "spec": {
           "containers": [{
             "name": "manager",
             "imagePullPolicy": "Never"
           }]
         }
       }
     }
   }'
   ```

### Runtime Issues

#### Issue: ShardConfig Not Creating Shards

**Symptoms:**
- ShardConfig exists but no ShardInstances are created
- ShardConfig status shows `phase: Pending`

**Diagnosis:**
```bash
# Check ShardConfig status
kubectl describe shardconfig <name> -n <namespace>

# Check controller logs
kubectl logs -n shard-system deployment/shard-manager | grep -i "shardconfig\|error"

# Check events
kubectl get events --field-selector involvedObject.kind=ShardConfig
```

**Common Causes and Solutions:**

1. **Invalid Configuration:**
   ```bash
   # Validate configuration
   kubectl get shardconfig <name> -o yaml
   
   # Check for validation errors
   kubectl describe shardconfig <name> | grep -A10 "Conditions:"
   ```

2. **Resource Constraints:**
   ```bash
   # Check cluster resources
   kubectl describe nodes
   kubectl get pods --all-namespaces | grep Pending
   
   # Reduce resource requirements
   kubectl patch shardconfig <name> --type='merge' -p='
   {
     "spec": {
       "resources": {
         "requests": {"cpu": "50m", "memory": "64Mi"}
       }
     }
   }'
   ```

3. **Controller Not Running:**
   ```bash
   # Check controller status
   kubectl get pods -n shard-system
   
   # Restart controller if needed
   kubectl rollout restart deployment/shard-manager -n shard-system
   ```

#### Issue: Shards Not Scaling

**Symptoms:**
- Load is high but shards don't scale up
- Load is low but shards don't scale down

**Diagnosis:**
```bash
# Check current shard status
kubectl get shardinstances -o wide

# Check scaling metrics
kubectl logs -n shard-system deployment/shard-manager | grep -i "scaling\|threshold"

# Check ShardConfig scaling settings
kubectl get shardconfig <name> -o jsonpath='{.spec.scaleUpThreshold}'
kubectl get shardconfig <name> -o jsonpath='{.spec.scaleDownThreshold}'
```

**Solutions:**

1. **Adjust Scaling Thresholds:**
   ```bash
   # Make scaling more aggressive
   kubectl patch shardconfig <name> --type='merge' -p='
   {
     "spec": {
       "scaleUpThreshold": 0.6,
       "scaleDownThreshold": 0.4
     }
   }'
   ```

2. **Check Cooldown Period:**
   ```bash
   # Reduce cooldown period
   kubectl patch shardconfig <name> --type='merge' -p='
   {
     "spec": {
       "cooldownPeriod": "60s"
     }
   }'
   ```

3. **Verify Load Metrics:**
   ```bash
   # Check if load metrics are being reported
   kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080
   curl http://localhost:8080/metrics | grep shard_controller_load
   ```

#### Issue: Shard Health Check Failures

**Symptoms:**
- ShardInstances showing as unhealthy
- Frequent shard restarts
- Health check timeouts

**Diagnosis:**
```bash
# Check shard health status
kubectl get shardinstances -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,LOAD:.status.load,LAST_HEARTBEAT:.status.lastHeartbeat

# Check worker logs
kubectl logs -n shard-system -l app=shard-worker | grep -i "health\|heartbeat"

# Check network connectivity
kubectl exec -n shard-system deployment/shard-manager -- nslookup shard-worker-headless
```

**Solutions:**

1. **Adjust Health Check Settings:**
   ```bash
   kubectl patch shardconfig <name> --type='merge' -p='
   {
     "spec": {
       "healthCheckInterval": "60s",
       "healthCheckTimeout": "30s",
       "failureThreshold": 5
     }
   }'
   ```

2. **Check Resource Limits:**
   ```bash
   # Increase worker resources
   kubectl patch deployment shard-worker -n shard-system -p='
   {
     "spec": {
       "template": {
         "spec": {
           "containers": [{
             "name": "worker",
             "resources": {
               "limits": {"cpu": "2000m", "memory": "2Gi"}
             }
           }]
         }
       }
     }
   }'
   ```

3. **Network Issues:**
   ```bash
   # Check network policies
   kubectl get networkpolicy -n shard-system
   
   # Test connectivity
   kubectl exec -n shard-system deployment/shard-manager -- wget -qO- http://shard-worker-headless:8081/healthz
   ```

#### Issue: Resource Migration Failures

**Symptoms:**
- Migration operations timing out
- Resources stuck in migration state
- Data loss during migrations

**Diagnosis:**
```bash
# Check migration logs
kubectl logs -n shard-system deployment/shard-manager | grep -i "migration"
kubectl logs -n shard-system -l app=shard-worker | grep -i "migration"

# Check shard status during migration
kubectl get shardinstances -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,ASSIGNED:.status.assignedResources

# Check migration metrics
curl http://localhost:8080/metrics | grep migration
```

**Solutions:**

1. **Increase Migration Timeout:**
   ```bash
   kubectl patch shardconfig <name> --type='merge' -p='
   {
     "spec": {
       "migration": {
         "timeout": "1200s",
         "retryAttempts": 5,
         "retryDelay": "60s"
       }
     }
   }'
   ```

2. **Reduce Migration Batch Size:**
   ```bash
   kubectl patch shardconfig <name> --type='merge' -p='
   {
     "spec": {
       "migration": {
         "batchSize": 50
       }
     }
   }'
   ```

3. **Check Resource Availability:**
   ```bash
   # Ensure target shards have capacity
   kubectl get shardinstances -o custom-columns=NAME:.metadata.name,LOAD:.status.load,CAPACITY:.status.capacity
   ```

### Performance Issues

#### Issue: High Controller CPU Usage

**Symptoms:**
- Controller pods consuming high CPU
- Slow response times
- Frequent reconciliation loops

**Diagnosis:**
```bash
# Check resource usage
kubectl top pods -n shard-system

# Check reconciliation frequency
kubectl logs -n shard-system deployment/shard-manager | grep "Reconciling" | tail -20

# Profile CPU usage
kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
```

**Solutions:**

1. **Increase Resource Limits:**
   ```bash
   kubectl patch deployment shard-manager -n shard-system -p='
   {
     "spec": {
       "template": {
         "spec": {
           "containers": [{
             "name": "manager",
             "resources": {
               "limits": {"cpu": "2000m", "memory": "2Gi"}
             }
           }]
         }
       }
     }
   }'
   ```

2. **Optimize Reconciliation:**
   ```bash
   # Increase reconciliation intervals
   kubectl patch configmap shard-manager-config -n shard-system -p='
   {
     "data": {
       "config.yaml": "manager:\n  healthCheck:\n    interval: 60s\n  scaling:\n    cooldownPeriod: 600s"
     }
   }'
   ```

3. **Reduce Watch Scope:**
   ```bash
   # Use namespace-scoped watching if possible
   kubectl set env deployment/shard-manager -n shard-system WATCH_NAMESPACE=production
   ```

#### Issue: High Memory Usage

**Symptoms:**
- Controller pods being OOMKilled
- Memory usage continuously growing
- Slow garbage collection

**Diagnosis:**
```bash
# Check memory usage
kubectl top pods -n shard-system

# Check for memory leaks
kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080
curl http://localhost:8080/debug/pprof/heap > heap.prof

# Check pod events
kubectl describe pod -n shard-system -l app=shard-manager | grep -A10 "Events:"
```

**Solutions:**

1. **Increase Memory Limits:**
   ```bash
   kubectl patch deployment shard-manager -n shard-system -p='
   {
     "spec": {
       "template": {
         "spec": {
           "containers": [{
             "name": "manager",
             "resources": {
               "limits": {"memory": "4Gi"}
             }
           }]
         }
       }
     }
   }'
   ```

2. **Optimize Caching:**
   ```bash
   # Reduce cache size
   kubectl set env deployment/shard-manager -n shard-system MAX_CACHE_SIZE=1000
   ```

3. **Enable Memory Profiling:**
   ```bash
   kubectl set env deployment/shard-manager -n shard-system ENABLE_PPROF=true
   ```

## Debugging Tools

### Log Analysis

```bash
# Structured log analysis
kubectl logs -n shard-system deployment/shard-manager | jq 'select(.level == "error")'

# Follow logs with filtering
kubectl logs -n shard-system deployment/shard-manager -f | grep -E "(ERROR|WARN|scaling)"

# Export logs for analysis
kubectl logs -n shard-system deployment/shard-manager --since=1h > manager.log
```

### Metrics Analysis

```bash
# Key metrics to monitor
curl -s http://localhost:8080/metrics | grep -E "(shard_controller_shards_total|shard_controller_load_average|shard_controller_errors_total)"

# Prometheus queries (if using Prometheus)
# Rate of scaling operations
rate(shard_controller_scaling_operations_total[5m])

# Average shard load
avg(shard_controller_shard_load)

# Error rate
rate(shard_controller_errors_total[5m])
```

### Resource Analysis

```bash
# Check resource quotas
kubectl describe quota -n shard-system

# Check limit ranges
kubectl describe limitrange -n shard-system

# Check pod resource usage
kubectl describe pod -n shard-system -l app=shard-manager | grep -A10 "Requests:"
```

## Recovery Procedures

### Controller Recovery

```bash
# Restart controller components
kubectl rollout restart deployment/shard-manager -n shard-system
kubectl rollout restart deployment/shard-worker -n shard-system

# Wait for rollout to complete
kubectl rollout status deployment/shard-manager -n shard-system
kubectl rollout status deployment/shard-worker -n shard-system
```

### Data Recovery

```bash
# Backup current state
kubectl get shardconfigs,shardinstances --all-namespaces -o yaml > backup.yaml

# Restore from backup
kubectl apply -f backup.yaml

# Force reconciliation
kubectl annotate shardconfig <name> kubectl.kubernetes.io/restartedAt="$(date -Iseconds)"
```

### Emergency Procedures

#### Complete System Reset

```bash
# WARNING: This will delete all shard resources
kubectl delete shardinstances --all --all-namespaces
kubectl delete shardconfigs --all --all-namespaces

# Restart controllers
kubectl rollout restart deployment/shard-manager -n shard-system
kubectl rollout restart deployment/shard-worker -n shard-system

# Recreate resources
kubectl apply -f your-shardconfigs.yaml
```

#### Cluster Migration

```bash
# Export resources from source cluster
kubectl get shardconfigs --all-namespaces -o yaml > shardconfigs-backup.yaml

# Apply to target cluster
kubectl apply -f shardconfigs-backup.yaml

# Verify migration
kubectl get shardconfigs,shardinstances --all-namespaces
```

## Prevention

### Monitoring Setup

```bash
# Set up basic monitoring
kubectl apply -f manifests/monitoring/

# Configure alerts
kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: shard-controller-alerts
spec:
  groups:
  - name: shard-controller
    rules:
    - alert: ShardControllerDown
      expr: up{job="shard-controller"} == 0
      for: 5m
    - alert: HighShardLoad
      expr: shard_controller_load_average > 0.9
      for: 10m
    - alert: ScalingFailures
      expr: rate(shard_controller_scaling_failures_total[5m]) > 0.1
      for: 5m
EOF
```

### Best Practices

1. **Resource Planning:**
   - Monitor resource usage trends
   - Set appropriate resource limits
   - Plan for peak load scenarios

2. **Configuration Management:**
   - Use version control for configurations
   - Test changes in staging first
   - Document configuration decisions

3. **Monitoring:**
   - Set up comprehensive monitoring
   - Configure meaningful alerts
   - Regular health checks

4. **Backup and Recovery:**
   - Regular configuration backups
   - Test recovery procedures
   - Document emergency procedures

## Getting Help

### Information to Collect

When seeking help, collect the following information:

```bash
# System information
kubectl version
kubectl get nodes -o wide

# Controller status
kubectl get pods -n shard-system -o wide
kubectl describe deployment -n shard-system

# Resource status
kubectl get shardconfigs,shardinstances --all-namespaces -o wide

# Logs (last 100 lines)
kubectl logs -n shard-system deployment/shard-manager --tail=100
kubectl logs -n shard-system deployment/shard-worker --tail=100

# Events
kubectl get events --sort-by=.metadata.creationTimestamp --all-namespaces | tail -50

# Configuration
kubectl get configmap -n shard-system -o yaml
```

### Support Channels

- **GitHub Issues**: For bug reports and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Documentation**: Check the latest documentation
- **Community Slack**: For real-time support

### Issue Template

When reporting issues, include:

1. **Environment Details:**
   - Kubernetes version
   - Controller version
   - Cluster setup (cloud provider, on-premises, etc.)

2. **Problem Description:**
   - What you expected to happen
   - What actually happened
   - Steps to reproduce

3. **Logs and Diagnostics:**
   - Controller logs
   - Resource configurations
   - Error messages

4. **Workarounds:**
   - Any temporary solutions tried
   - Current impact on operations

This troubleshooting guide should help you resolve most common issues. For complex problems, don't hesitate to reach out to the community for support.