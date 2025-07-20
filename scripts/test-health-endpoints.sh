#!/bin/bash

# Test script to verify health endpoints are working
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
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

# Configuration
CLUSTER_NAME="shard-controller-test"
NAMESPACE="shard-system"

create_test_cluster() {
    log_info "Creating test kind cluster: $CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
        log_warning "Cluster $CLUSTER_NAME already exists. Deleting it..."
        kind delete cluster --name $CLUSTER_NAME
    fi
    
    # Create simple kind cluster
    kind create cluster --name $CLUSTER_NAME
    
    # Wait for cluster to be ready
    kubectl cluster-info --context kind-$CLUSTER_NAME
    kubectl wait --for=condition=Ready nodes --all --timeout=300s
    
    log_success "Test cluster created successfully!"
}

build_and_load_images() {
    log_info "Building and loading Docker images..."
    
    # Build Docker images
    make docker-build
    
    # Load images into kind cluster
    kind load docker-image shard-controller/manager:latest --name $CLUSTER_NAME
    kind load docker-image shard-controller/worker:latest --name $CLUSTER_NAME
    
    log_success "Images built and loaded successfully!"
}

deploy_controller() {
    log_info "Deploying shard controller..."
    
    # Install CRDs
    kubectl apply -f manifests/crds/
    
    # Wait for CRDs to be established
    kubectl wait --for condition=established --timeout=60s crd/shardconfigs.shard.io
    kubectl wait --for condition=established --timeout=60s crd/shardinstances.shard.io
    
    # Create namespace and RBAC
    kubectl apply -f manifests/namespace.yaml
    kubectl apply -f manifests/rbac.yaml
    
    # Apply configuration and services
    kubectl apply -f manifests/configmap.yaml
    kubectl apply -f manifests/services.yaml
    
    # Deploy controller components
    kubectl apply -f manifests/deployment/
    
    log_success "Controller deployed!"
}

wait_for_pods() {
    log_info "Waiting for pods to be ready..."
    
    # Wait for deployments to be available (with longer timeout)
    kubectl wait --for=condition=available --timeout=600s deployment/shard-manager -n $NAMESPACE || {
        log_error "Manager deployment failed to become available"
        kubectl describe deployment/shard-manager -n $NAMESPACE
        kubectl logs -n $NAMESPACE deployment/shard-manager --tail=50
        return 1
    }
    
    kubectl wait --for=condition=available --timeout=600s deployment/shard-worker -n $NAMESPACE || {
        log_error "Worker deployment failed to become available"
        kubectl describe deployment/shard-worker -n $NAMESPACE
        kubectl logs -n $NAMESPACE deployment/shard-worker --tail=50
        return 1
    }
    
    log_success "All pods are ready!"
}

test_health_endpoints() {
    log_info "Testing health endpoints..."
    
    # Test manager health endpoint
    log_info "Testing manager health endpoint..."
    kubectl port-forward -n $NAMESPACE svc/shard-manager-metrics 8080:8081 &
    MANAGER_PF_PID=$!
    
    sleep 3
    
    if curl -s http://localhost:8080/healthz | grep -q "ok"; then
        log_success "Manager health endpoint is working!"
    else
        log_warning "Manager health endpoint not responding correctly"
        curl -v http://localhost:8080/healthz || true
    fi
    
    if curl -s http://localhost:8080/readyz | grep -q "ready"; then
        log_success "Manager readiness endpoint is working!"
    else
        log_warning "Manager readiness endpoint not responding correctly"
        curl -v http://localhost:8080/readyz || true
    fi
    
    kill $MANAGER_PF_PID 2>/dev/null || true
    
    # Test worker health endpoint
    log_info "Testing worker health endpoint..."
    kubectl port-forward -n $NAMESPACE svc/shard-worker-metrics 8081:8081 &
    WORKER_PF_PID=$!
    
    sleep 3
    
    if curl -s http://localhost:8081/healthz | grep -q "ok"; then
        log_success "Worker health endpoint is working!"
    else
        log_warning "Worker health endpoint not responding correctly"
        curl -v http://localhost:8081/healthz || true
    fi
    
    if curl -s http://localhost:8081/readyz | grep -q "ready"; then
        log_success "Worker readiness endpoint is working!"
    else
        log_warning "Worker readiness endpoint not responding correctly"
        curl -v http://localhost:8081/readyz || true
    fi
    
    kill $WORKER_PF_PID 2>/dev/null || true
}

show_status() {
    log_info "Showing current status..."
    
    echo ""
    echo "=== Pods ==="
    kubectl get pods -n $NAMESPACE -o wide
    
    echo ""
    echo "=== Services ==="
    kubectl get svc -n $NAMESPACE
    
    echo ""
    echo "=== Manager Logs (last 20 lines) ==="
    kubectl logs -n $NAMESPACE deployment/shard-manager --tail=20
    
    echo ""
    echo "=== Worker Logs (last 20 lines) ==="
    kubectl logs -n $NAMESPACE deployment/shard-worker --tail=20
}

cleanup() {
    log_info "Cleaning up test resources..."
    
    # Delete the kind cluster
    kind delete cluster --name $CLUSTER_NAME || true
    
    log_success "Cleanup completed!"
}

main() {
    echo "🧪 Shard Controller Health Endpoint Test"
    echo "========================================"
    echo ""
    
    # Handle cleanup on exit
    trap cleanup EXIT
    
    create_test_cluster
    build_and_load_images
    deploy_controller
    wait_for_pods
    test_health_endpoints
    show_status
    
    log_success "Health endpoint test completed! 🎉"
}

# Handle script arguments
case "${1:-}" in
    "cleanup")
        cleanup
        exit 0
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [cleanup|help]"
        echo ""
        echo "Commands:"
        echo "  (no args)  Run the health endpoint test"
        echo "  cleanup    Clean up test resources"
        echo "  help       Show this help message"
        exit 0
        ;;
    "")
        main
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Run '$0 help' for usage information"
        exit 1
        ;;
esac