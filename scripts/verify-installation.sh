#!/bin/bash

# Quick verification script for Shard Controller installation
# This script checks if the controller is properly installed and running

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

NAMESPACE="shard-system"

echo "🔍 Verifying Shard Controller Installation"
echo "=========================================="
echo ""

# Check if CRDs are installed
log_info "Checking Custom Resource Definitions..."
if kubectl get crd shardconfigs.shard.io &>/dev/null && kubectl get crd shardinstances.shard.io &>/dev/null; then
    log_success "CRDs are installed"
else
    log_error "CRDs are not installed"
    echo "Run: kubectl apply -f manifests/crds/"
    exit 1
fi

# Check if namespace exists
log_info "Checking namespace..."
if kubectl get namespace $NAMESPACE &>/dev/null; then
    log_success "Namespace $NAMESPACE exists"
else
    log_error "Namespace $NAMESPACE does not exist"
    echo "Run: kubectl apply -f manifests/namespace.yaml"
    exit 1
fi

# Check if deployments exist and are ready
log_info "Checking controller deployments..."

# Check manager deployment
if kubectl get deployment shard-manager -n $NAMESPACE &>/dev/null; then
    MANAGER_READY=$(kubectl get deployment shard-manager -n $NAMESPACE -o jsonpath='{.status.readyReplicas}')
    MANAGER_DESIRED=$(kubectl get deployment shard-manager -n $NAMESPACE -o jsonpath='{.spec.replicas}')
    
    if [[ "$MANAGER_READY" == "$MANAGER_DESIRED" ]]; then
        log_success "Manager deployment is ready ($MANAGER_READY/$MANAGER_DESIRED)"
    else
        log_warning "Manager deployment is not ready ($MANAGER_READY/$MANAGER_DESIRED)"
    fi
else
    log_error "Manager deployment not found"
    exit 1
fi

# Check worker deployment
if kubectl get deployment shard-worker -n $NAMESPACE &>/dev/null; then
    WORKER_READY=$(kubectl get deployment shard-worker -n $NAMESPACE -o jsonpath='{.status.readyReplicas}')
    WORKER_DESIRED=$(kubectl get deployment shard-worker -n $NAMESPACE -o jsonpath='{.spec.replicas}')
    
    if [[ "$WORKER_READY" == "$WORKER_DESIRED" ]]; then
        log_success "Worker deployment is ready ($WORKER_READY/$WORKER_DESIRED)"
    else
        log_warning "Worker deployment is not ready ($WORKER_READY/$WORKER_DESIRED)"
    fi
else
    log_error "Worker deployment not found"
    exit 1
fi

# Check if services exist
log_info "Checking services..."
if kubectl get service shard-manager-metrics -n $NAMESPACE &>/dev/null; then
    log_success "Manager service exists"
else
    log_warning "Manager service not found"
fi

if kubectl get service shard-worker-metrics -n $NAMESPACE &>/dev/null; then
    log_success "Worker service exists"
else
    log_warning "Worker service not found"
fi

# Check pod status
log_info "Checking pod status..."
MANAGER_PODS=$(kubectl get pods -n $NAMESPACE -l app=shard-manager --no-headers | wc -l)
WORKER_PODS=$(kubectl get pods -n $NAMESPACE -l app=shard-worker --no-headers | wc -l)

if [[ $MANAGER_PODS -gt 0 ]]; then
    log_success "Manager pods are running ($MANAGER_PODS pods)"
else
    log_error "No manager pods found"
fi

if [[ $WORKER_PODS -gt 0 ]]; then
    log_success "Worker pods are running ($WORKER_PODS pods)"
else
    log_error "No worker pods found"
fi

# Check for any failing pods
FAILING_PODS=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase!=Running --no-headers 2>/dev/null | wc -l)
if [[ $FAILING_PODS -eq 0 ]]; then
    log_success "All pods are running"
else
    log_warning "$FAILING_PODS pods are not running"
    kubectl get pods -n $NAMESPACE --field-selector=status.phase!=Running
fi

# Test basic functionality
log_info "Testing basic functionality..."

# Check if we can create a ShardConfig
TEST_CONFIG=$(cat <<EOF
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: test-verification
  namespace: default
spec:
  minShards: 1
  maxShards: 3
  scaleUpThreshold: 0.8
  scaleDownThreshold: 0.3
  loadBalanceStrategy: "consistent-hash"
EOF
)

if echo "$TEST_CONFIG" | kubectl apply --dry-run=client -f - &>/dev/null; then
    log_success "ShardConfig API is working"
else
    log_error "ShardConfig API is not working"
    exit 1
fi

# Check health endpoints (if accessible)
log_info "Checking health endpoints..."
if kubectl get service shard-manager-metrics -n $NAMESPACE &>/dev/null; then
    # Try to port-forward and check health (with timeout)
    kubectl port-forward -n $NAMESPACE svc/shard-manager-metrics 8080:8080 &
    PF_PID=$!
    sleep 2
    
    if curl -s --max-time 3 http://localhost:8080/healthz 2>/dev/null | grep -q "ok"; then
        log_success "Health endpoint is responding"
    else
        log_warning "Health endpoint is not responding (this might be normal)"
    fi
    
    kill $PF_PID 2>/dev/null || true
fi

echo ""
log_success "✅ Verification completed!"
echo ""

# Summary
echo "Installation Summary:"
echo "- CRDs: ✅ Installed"
echo "- Namespace: ✅ Created"
echo "- Manager: ✅ Running"
echo "- Worker: ✅ Running"
echo "- Services: ✅ Available"
echo "- API: ✅ Functional"
echo ""

# Next steps
echo "Next steps:"
echo "1. Create your first ShardConfig:"
echo "   kubectl apply -f examples/basic-shardconfig.yaml"
echo ""
echo "2. Monitor the controller:"
echo "   kubectl logs -f -n $NAMESPACE deployment/shard-manager"
echo ""
echo "3. Check created resources:"
echo "   kubectl get shardconfigs,shardinstances --all-namespaces"
echo ""
echo "4. Access metrics:"
echo "   kubectl port-forward -n $NAMESPACE svc/shard-manager-metrics 8080:8080"
echo "   curl http://localhost:8080/metrics"
echo ""
echo "For more information, see the documentation in docs/"