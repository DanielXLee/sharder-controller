# Shard Controller Troubleshooting Guide

## Common Issues and Solutions

### 1. Installation Issues

#### CRD Installation Fails
**Symptoms:**
```
error validating data: ValidationError(CustomResourceDefinition.spec)
```

**Solution:**
```bash
# Check Kubernetes version
kubectl version

# Ensure you have cluster admin permissions
kubectl auth can-i create customresourcedefinitions

# Manually install CRDs
kubectl apply -f manifests/crds/ --validate=false
```

#### RBAC Permission Denied
**Symptoms:**
```
forbidden: User "system:serviceaccount:shard-system:shard-manager" cannot create resource
```

**Solution:**
```bash
# Check service account exists
kubectl get sa -n shard-system

# Verify RBAC bindings
kubectl get clusterrolebinding | grep shard

# Reapply RBAC configuration
kubectl apply -f manifests/rbac.yaml
```

### 2. Manager Pod Issues

#### Manager Pod Not Starting
**Symptoms:**
- Pod stuck in `Pending` or `CrashLoopBackOff` state

**Diagnosis:**
```bash
# Check pod status
kubectl get pods -n shard-system

# Check pod events
kubectl describe pod -n shard-system <manager-pod-name>

# Check logs
kubectl logs -n shard-system <manager-pod-name>
```

**Common Solutions:**
```bash
# Resource constraints
kubectl describe node <node-name>

# Image pull issues
kubectl get events -n shard-system --sort-by='.lastTimestamp'

# Configuration issues
kubectl get configmap -n shard-system shard-manager-config -o yaml
```

#### Leader Election Issues
**Symptoms:**
```
failed to acquire lease shard-system/shard-manager
```

**Solution:**
```bash
# Check existing leases
kubectl get leases -n shard-system

# Delete stale lease if needed
kubectl delete lease -n shard-system shard-manager

# Check RBAC permissions for coordination.k8s.io
kubectl auth can-i create leases --as=system:serviceaccount:shard-system:shard-manager
```

### 3. Worker Pod Issues

#### Workers Not Registering
**Symptoms:**
- Workers start but don't appear in shard instances

**Diagnosis:**
```bash
# Check worker logs
kubectl logs -n shard-system deployment/shard-worker

# Check custom resources
kubectl get shardinstances --all-namespaces

# Check network connectivity
kubectl exec -n shard-system <worker-pod> -- nslookup kubernetes.default
```

**Solution:**
```bash
# Verify worker configuration
kubectl get configmap -n shard-system shard-worker-config -o yaml

# Check service discovery
kubectl get svc -n shard-system

# Restart workers
kubectl rollout restart deployment/shard-worker -n shard-system
```

#### High Worker Memory Usage
**Symptoms:**
- Workers getting OOMKilled
- High memory consumption

**Solution:**
```bash
# Check resource limits
kubectl describe pod -n shard-system <worker-pod>

# Increase memory limits
kubectl patch deployment shard-worker -n shard-system -p '{"spec":{"template":{"spec":{"containers":[{"name":"worker","resources":{"limits":{"memory":"2Gi"}}}]}}}}'

# Check for memory leaks in logs
kubectl logs -n shard-system <worker-pod> | grep -i memory
```

### 4. Scaling Issues

#### Auto-scaling Not Working
**Symptoms:**
- Load increases but no new shards created

**Diagnosis:**
```bash
# Check manager logs for scaling decisions
kubectl logs -n shard-system deployment/shard-manager | grep -i scale

# Check shard config
kubectl get shardconfig --all-namespaces -o yaml

# Check metrics
kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080
curl http://localhost:8080/metrics | grep shard_load
```

**Solution:**
```bash
# Verify scaling thresholds
kubectl get shardconfig <config-name> -o yaml

# Check cooldown periods
kubectl logs -n shard-system deployment/shard-manager | grep cooldown

# Manual scaling test
kubectl patch shardconfig <config-name> --type='merge' -p='{"spec":{"minShards":5}}'
```

### 5. Load Balancing Issues

#### Uneven Load Distribution
**Symptoms:**
- Some shards heavily loaded while others idle

**Diagnosis:**
```bash
# Check load balancing strategy
kubectl get shardconfig <config-name> -o jsonpath='{.spec.loadBalanceStrategy}'

# Check shard loads
kubectl get shardinstances --all-namespaces -o custom-columns=NAME:.metadata.name,LOAD:.status.load

# Check hash distribution
kubectl logs -n shard-system deployment/shard-manager | grep hash
```

**Solution:**
```bash
# Try different load balancing strategy
kubectl patch shardconfig <config-name> --type='merge' -p='{"spec":{"loadBalanceStrategy":"least-loaded"}}'

# Force rebalance
kubectl annotate shardconfig <config-name> shard.io/force-rebalance="$(date)"

# Check rebalance threshold
kubectl patch shardconfig <config-name> --type='merge' -p='{"spec":{"rebalanceThreshold":0.1}}'
```

### 6. Resource Migration Issues

#### Migration Stuck
**Symptoms:**
- Resources not moving between shards
- Migration timeout errors

**Diagnosis:**
```bash
# Check migration logs
kubectl logs -n shard-system deployment/shard-manager | grep migration

# Check source and target shard status
kubectl get shardinstances --all-namespaces

# Check network connectivity between shards
kubectl exec -n shard-system <source-pod> -- nc -zv <target-pod-ip> 8080
```

**Solution:**
```bash
# Increase migration timeout
kubectl patch configmap shard-worker-config -n shard-system --type='merge' -p='{"data":{"config.yaml":"worker:\n  migration:\n    timeout: 600s"}}'

# Restart affected pods
kubectl delete pod -n shard-system <stuck-pod>

# Check resource locks
kubectl get shardinstances <instance-name> -o yaml | grep -A 10 resources
```

### 7. Monitoring and Observability Issues

#### Metrics Not Available
**Symptoms:**
- Prometheus not scraping metrics
- Health endpoints not responding

**Diagnosis:**
```bash
# Check service endpoints
kubectl get endpoints -n shard-system

# Test metrics endpoint
kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080
curl http://localhost:8080/metrics

# Check ServiceMonitor (if using Prometheus Operator)
kubectl get servicemonitor -n monitoring
```

**Solution:**
```bash
# Verify service configuration
kubectl get svc -n shard-system -o yaml

# Check pod labels match service selector
kubectl get pods -n shard-system --show-labels

# Restart monitoring components
kubectl rollout restart deployment/shard-manager -n shard-system
```

## Diagnostic Commands

### Quick Health Check
```bash
#!/bin/bash
echo "=== Shard Controller Health Check ==="

echo "1. Checking namespace..."
kubectl get ns shard-system

echo "2. Checking pods..."
kubectl get pods -n shard-system

echo "3. Checking services..."
kubectl get svc -n shard-system

echo "4. Checking custom resources..."
kubectl get shardconfigs,shardinstances --all-namespaces

echo "5. Checking recent events..."
kubectl get events -n shard-system --sort-by='.lastTimestamp' | tail -10

echo "6. Checking manager logs (last 20 lines)..."
kubectl logs -n shard-system deployment/shard-manager --tail=20

echo "7. Checking worker logs (last 20 lines)..."
kubectl logs -n shard-system deployment/shard-worker --tail=20
```

### Performance Analysis
```bash
#!/bin/bash
echo "=== Performance Analysis ==="

echo "1. Resource usage..."
kubectl top pods -n shard-system

echo "2. Shard load distribution..."
kubectl get shardinstances --all-namespaces -o custom-columns=NAME:.metadata.name,LOAD:.status.load,RESOURCES:.status.assignedResources

echo "3. Recent scaling events..."
kubectl logs -n shard-system deployment/shard-manager | grep -E "(scale|rebalance)" | tail -10
```

## Getting Help

### Log Collection
```bash
# Collect all relevant logs
kubectl logs -n shard-system deployment/shard-manager > manager.log
kubectl logs -n shard-system deployment/shard-worker > worker.log
kubectl get events -n shard-system > events.log
kubectl describe pods -n shard-system > pod-descriptions.log
```

### Support Information
When reporting issues, please include:

1. Kubernetes version: `kubectl version`
2. Shard controller version: `kubectl get deployment -n shard-system shard-manager -o jsonpath='{.spec.template.spec.containers[0].image}'`
3. Configuration: `kubectl get configmap -n shard-system -o yaml`
4. Logs from the diagnostic commands above
5. Description of the issue and steps to reproduce

### Community Resources
- GitHub Issues: https://github.com/example/shard-controller/issues
- Documentation: https://docs.example.com/shard-controller
- Slack Channel: #shard-controller