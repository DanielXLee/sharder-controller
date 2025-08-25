#!/bin/bash

# Kubernetes Shard Controller Comprehensive Demo Tool
# Interactive demonstration of all core capabilities

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
CLUSTER_NAME="shard-demo"
NAMESPACE="shard-system"
DEMO_NAMESPACE="demo"
LOG_FILE="/tmp/shard-demo.log"

# Helper functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1" | tee -a $LOG_FILE; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a $LOG_FILE; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a $LOG_FILE; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" | tee -a $LOG_FILE; }
log_demo() { echo -e "${PURPLE}[DEMO]${NC} $1" | tee -a $LOG_FILE; }

press_enter() {
    echo -e "\n${CYAN}Press Enter to continue...${NC}"
    read -r
}

show_banner() {
    echo -e "${CYAN}"
    cat << 'EOF'
  ____  _                   _    ____            _             _ _           
 / ___|| |__   __ _ _ __ __| |  / ___|___  _ __ | |_ _ __ ___ | | | ___ _ __ 
 \___ \| '_ \ / _` | '__/ _` | | |   / _ \| '_ \| __| '__/ _ \| | |/ _ \ '__|
  ___) | | | | (_| | | | (_| | | |__| (_) | | | | |_| | | (_) | | |  __/ |   
 |____/|_| |_|\__,_|_|  \__,_|  \____\___/|_| |_|\__|_|  \___/|_|_|\___|_|   
                                                                            
     Comprehensive Demo - All Core Capabilities Showcase
EOF
    echo -e "${NC}"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check tools
    for tool in kubectl kind go docker curl; do
        if ! command -v $tool &> /dev/null; then
            log_error "$tool is not installed. Please install it first."
            exit 1
        fi
    done
    
    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    if [[ "$(printf '%s\n' "1.20" "$GO_VERSION" | sort -V | head -n1)" != "1.20" ]]; then
        log_error "Go version 1.20+ is required. Current: $GO_VERSION"
        exit 1
    fi
    
    log_success "All prerequisites met!"
}

setup_environment() {
    log_info "Setting up demo environment..."
    
    # Create kind cluster if needed
    if ! kind get clusters | grep -q "^$CLUSTER_NAME$"; then
        log_info "Creating kind cluster..."
        cat <<EOF | kind create cluster --name $CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
EOF
    fi
    
    # Build and load controller
    log_info "Building controller..."
    make build
    make docker-build
    kind load docker-image shard-controller/manager:latest --name $CLUSTER_NAME
    kind load docker-image shard-controller/worker:latest --name $CLUSTER_NAME
    
    # Install CRDs and controller
    kubectl apply -f manifests/crds/
    kubectl apply -f manifests/namespace.yaml
    kubectl apply -f manifests/rbac.yaml
    kubectl apply -f manifests/configmap.yaml
    kubectl apply -f manifests/services.yaml
    kubectl apply -f manifests/deployment/
    
    # Wait for deployment
    kubectl wait --for=condition=available --timeout=300s deployment/shard-manager -n $NAMESPACE
    
    log_success "Environment setup complete!"
}

demo_dynamic_sharding() {
    log_demo "=== Demo 1: Dynamic Sharding & Auto-scaling ==="
    
    echo "This demo shows how the controller dynamically creates and manages shards"
    press_enter
    
    # Create initial ShardConfig
    log_info "Creating initial ShardConfig with 2-6 shards..."
    cat <<EOF | kubectl apply -f -
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: scaling-demo
  namespace: $DEMO_NAMESPACE
spec:
  minShards: 2
  maxShards: 6
  scaleUpThreshold: 0.6
  scaleDownThreshold: 0.2
  healthCheckInterval: "15s"
  loadBalanceStrategy: "consistent-hash"
EOF
    
    sleep 5
    kubectl get shardconfigs,shardinstances -n $DEMO_NAMESPACE -o wide
    
    log_info "Triggering scale-up by lowering threshold..."
    kubectl patch shardconfig scaling-demo -n $DEMO_NAMESPACE --type='merge' -p='{"spec":{"scaleUpThreshold":0.1}}'
    
    sleep 10
    kubectl get shardinstances -n $DEMO_NAMESPACE -o wide
    
    log_success "Dynamic sharding demonstrated!"
    press_enter
}

demo_load_balancing() {
    log_demo "=== Demo 2: Load Balancing Strategies ==="
    
    echo "Demonstrating different load balancing strategies:"
    echo "1. Consistent Hash - for cache scenarios"
    echo "2. Round Robin - for even distribution"  
    echo "3. Least Loaded - for dynamic optimization"
    press_enter
    
    for strategy in "consistent-hash" "round-robin" "least-loaded"; do
        log_info "Testing $strategy strategy..."
        cat <<EOF | kubectl apply -f -
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: lb-demo-$strategy
  namespace: $DEMO_NAMESPACE
spec:
  minShards: 3
  maxShards: 5
  scaleUpThreshold: 0.7
  scaleDownThreshold: 0.3
  loadBalanceStrategy: "$strategy"
  healthCheckInterval: "30s"
EOF
        
        sleep 5
        kubectl get shardconfigs lb-demo-$strategy -n $DEMO_NAMESPACE -o yaml | grep loadBalanceStrategy
    done
    
    log_success "Load balancing strategies demonstrated!"
    press_enter
}

demo_health_monitoring() {
    log_demo "=== Demo 3: Health Monitoring & Failure Recovery ==="
    
    echo "Monitoring shard health and demonstrating failure recovery"
    press_enter
    
    # Check health endpoints
    log_info "Checking health endpoints..."
    kubectl port-forward -n $NAMESPACE svc/shard-manager-metrics 8080:8080 &
    PF_PID=$!
    sleep 3
    
    if curl -s http://localhost:8080/healthz | grep -q "ok"; then
        log_success "Manager health check: OK"
    fi
    
    if curl -s http://localhost:8080/metrics | grep -q shard; then
        log_success "Metrics endpoint: OK"
    fi
    
    kill $PF_PID 2>/dev/null || true
    
    # Show controller logs
    log_info "Recent controller activity:"
    kubectl logs -n $NAMESPACE deployment/shard-manager --tail=5
    
    log_success "Health monitoring demonstrated!"
    press_enter
}

demo_observability() {
    log_demo "=== Demo 4: Observability & Metrics ==="
    
    echo "Showing metrics, logging, and monitoring capabilities"
    press_enter
    
    # Port forward for metrics
    kubectl port-forward -n $NAMESPACE svc/shard-manager-metrics 8080:8080 &
    PF_PID=$!
    sleep 3
    
    log_info "Available metrics:"
    curl -s http://localhost:8080/metrics | grep "^shard_" | head -10
    
    kill $PF_PID 2>/dev/null || true
    
    log_info "Current system status:"
    kubectl get shardconfigs,shardinstances --all-namespaces -o wide
    
    log_success "Observability features demonstrated!"
    press_enter
}

show_summary() {
    log_demo "=== Demo Summary ==="
    
    echo "Core capabilities demonstrated:"
    echo "✅ Dynamic Sharding & Auto-scaling"
    echo "✅ Multiple Load Balancing Strategies"
    echo "✅ Health Monitoring & Recovery" 
    echo "✅ Rich Observability & Metrics"
    echo ""
    echo "Next steps:"
    echo "- Explore examples/ directory"
    echo "- Read docs/ for detailed guides"
    echo "- Try production deployment with Helm"
    
    log_success "Comprehensive demo completed! 🎉"
}

cleanup() {
    log_info "Cleaning up demo resources..."
    kubectl delete namespace $DEMO_NAMESPACE --ignore-not-found=true
    
    read -p "Delete kind cluster? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        kind delete cluster --name $CLUSTER_NAME
        log_success "Cleanup complete!"
    fi
}

main() {
    show_banner
    
    case "${1:-}" in
        "cleanup")
            cleanup
            ;;
        "help"|"-h"|"--help")
            echo "Usage: $0 [cleanup|help]"
            echo "Commands:"
            echo "  (no args)  Run comprehensive demo"
            echo "  cleanup    Clean up resources"
            echo "  help       Show help"
            ;;
        "")
            check_prerequisites
            setup_environment
            kubectl create namespace $DEMO_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
            
            demo_dynamic_sharding
            demo_load_balancing  
            demo_health_monitoring
            demo_observability
            show_summary
            
            echo -e "\n${CYAN}Demo log saved to: $LOG_FILE${NC}"
            ;;
        *)
            log_error "Unknown command: $1"
            exit 1
            ;;
    esac
}

main "$@"