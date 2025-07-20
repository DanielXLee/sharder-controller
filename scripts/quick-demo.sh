#!/bin/bash

# Kubernetes Shard Controller Quick Demo Script
# This script demonstrates the complete setup and usage of the shard controller

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="shard-controller-demo"
NAMESPACE="shard-system"
DEMO_NAMESPACE="demo"

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

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed. Please install kubectl first."
        exit 1
    fi
    
    # Check if kind is installed
    if ! command -v kind &> /dev/null; then
        log_warning "kind is not installed. Installing kind..."
        # Install kind
        if [[ "$OSTYPE" == "darwin"* ]]; then
            if command -v brew &> /dev/null; then
                brew install kind
            else
                log_error "Please install kind manually: https://kind.sigs.k8s.io/docs/user/quick-start/"
                exit 1
            fi
        else
            curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
            chmod +x ./kind
            sudo mv ./kind /usr/local/bin/kind
        fi
    fi
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed. Please install Go 1.24+ first."
        exit 1
    fi
    
    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    if [[ "$(printf '%s\n' "1.24" "$GO_VERSION" | sort -V | head -n1)" != "1.24" ]]; then
        log_error "Go version 1.24+ is required. Current version: $GO_VERSION"
        exit 1
    fi
    
    log_success "All prerequisites are met!"
}

create_kind_cluster() {
    log_info "Creating kind cluster: $CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
        log_warning "Cluster $CLUSTER_NAME already exists. Deleting it..."
        kind delete cluster --name $CLUSTER_NAME
    fi
    
    # Create kind cluster with custom configuration
    cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
- role: worker
- role: worker
EOF
    
    # Wait for cluster to be ready
    kubectl cluster-info --context kind-$CLUSTER_NAME
    kubectl wait --for=condition=Ready nodes --all --timeout=300s
    
    log_success "Kind cluster created successfully!"
}

build_controller() {
    log_info "Building shard controller..."
    
    # Ensure we're in the project root
    if [[ ! -f "go.mod" ]]; then
        log_error "Please run this script from the project root directory"
        exit 1
    fi
    
    # Build the controller binaries
    make build
    
    log_success "Controller built successfully!"
}

install_crds() {
    log_info "Installing Custom Resource Definitions..."
    
    # Apply CRDs
    kubectl apply -f manifests/crds/
    
    # Wait for CRDs to be established
    kubectl wait --for condition=established --timeout=60s crd/shardconfigs.shard.io
    kubectl wait --for condition=established --timeout=60s crd/shardinstances.shard.io
    
    log_success "CRDs installed successfully!"
}

deploy_controller() {
    log_info "Deploying shard controller..."
    
    # Create namespace
    kubectl apply -f manifests/namespace.yaml
    
    # Apply RBAC
    kubectl apply -f manifests/rbac.yaml
    
    # Apply configuration
    kubectl apply -f manifests/configmap.yaml
    
    # Apply services
    kubectl apply -f manifests/services.yaml
    
    # Deploy controller components
    kubectl apply -f manifests/deployment/
    
    # Wait for deployments to be ready
    log_info "Waiting for controller pods to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/shard-manager -n $NAMESPACE
    kubectl wait --for=condition=available --timeout=300s deployment/shard-worker -n $NAMESPACE
    
    log_success "Controller deployed successfully!"
}

create_demo_resources() {
    log_info "Creating demo resources..."
    
    # Create demo namespace
    kubectl create namespace $DEMO_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    
    # Create a demo ShardConfig
    cat <<EOF | kubectl apply -f -
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: demo-shards
  namespace: $DEMO_NAMESPACE
  labels:
    app: demo-app
    environment: demo
spec:
  minShards: 2
  maxShards: 5
  scaleUpThreshold: 0.7
  scaleDownThreshold: 0.3
  healthCheckInterval: "30s"
  loadBalanceStrategy: "consistent-hash"
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "200m"
      memory: "256Mi"
  gracefulShutdownTimeout: "60s"
  rebalanceThreshold: 0.2
EOF
    
    # Wait a moment for the controller to process
    sleep 5
    
    log_success "Demo resources created!"
}

show_status() {
    log_info "Showing current status..."
    
    echo ""
    echo "=== Controller Pods ==="
    kubectl get pods -n $NAMESPACE -o wide
    
    echo ""
    echo "=== Custom Resources ==="
    kubectl get shardconfigs,shardinstances -n $DEMO_NAMESPACE -o wide
    
    echo ""
    echo "=== Controller Logs (last 10 lines) ==="
    kubectl logs -n $NAMESPACE deployment/shard-manager --tail=10
}

demonstrate_scaling() {
    log_info "Demonstrating scaling functionality..."
    
    # Update the ShardConfig to trigger scaling
    kubectl patch shardconfig demo-shards -n $DEMO_NAMESPACE --type='merge' -p='{"spec":{"maxShards":8}}'
    
    log_info "Updated maxShards to 8. Monitoring changes..."
    sleep 10
    
    echo ""
    echo "=== Updated Resources ==="
    kubectl get shardconfigs,shardinstances -n $DEMO_NAMESPACE -o wide
}

run_health_check() {
    log_info "Running health checks..."
    
    # Port forward to access health endpoint
    kubectl port-forward -n $NAMESPACE svc/shard-manager-metrics 8080:8080 &
    PORT_FORWARD_PID=$!
    
    # Wait for port forward to be ready
    sleep 3
    
    # Check health endpoint
    if curl -s http://localhost:8080/healthz | grep -q "ok"; then
        log_success "Health check passed!"
    else
        log_warning "Health check failed or endpoint not ready"
    fi
    
    # Check metrics endpoint
    if curl -s http://localhost:8080/metrics | grep -q "shard_controller"; then
        log_success "Metrics endpoint is working!"
    else
        log_warning "Metrics endpoint not ready"
    fi
    
    # Kill port forward
    kill $PORT_FORWARD_PID 2>/dev/null || true
}

cleanup() {
    log_info "Cleaning up demo resources..."
    
    # Delete demo resources
    kubectl delete namespace $DEMO_NAMESPACE --ignore-not-found=true
    
    # Optionally delete the kind cluster
    read -p "Do you want to delete the kind cluster? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        kind delete cluster --name $CLUSTER_NAME
        log_success "Kind cluster deleted!"
    else
        log_info "Kind cluster preserved. You can delete it later with: kind delete cluster --name $CLUSTER_NAME"
    fi
}

show_next_steps() {
    log_info "Demo completed! Here are some next steps:"
    
    echo ""
    echo "=== Useful Commands ==="
    echo "# View all shard resources:"
    echo "kubectl get shardconfigs,shardinstances --all-namespaces"
    echo ""
    echo "# Watch controller logs:"
    echo "kubectl logs -f -n $NAMESPACE deployment/shard-manager"
    echo ""
    echo "# Access metrics:"
    echo "kubectl port-forward -n $NAMESPACE svc/shard-manager-metrics 8080:8080"
    echo "curl http://localhost:8080/metrics"
    echo ""
    echo "# Create your own ShardConfig:"
    echo "kubectl apply -f examples/basic-shardconfig.yaml"
    echo ""
    echo "=== Documentation ==="
    echo "- Configuration Guide: docs/configuration.md"
    echo "- User Guide: docs/user-guide.md"
    echo "- API Reference: docs/api-reference.md"
    echo "- Examples: examples/"
}

main() {
    echo "🚀 Kubernetes Shard Controller Quick Demo"
    echo "=========================================="
    echo ""
    
    # Handle cleanup on exit
    trap cleanup EXIT
    
    check_prerequisites
    create_kind_cluster
    build_controller
    install_crds
    deploy_controller
    create_demo_resources
    show_status
    demonstrate_scaling
    run_health_check
    show_next_steps
    
    log_success "Demo completed successfully! 🎉"
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
        echo "  (no args)  Run the full demo"
        echo "  cleanup    Clean up demo resources"
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