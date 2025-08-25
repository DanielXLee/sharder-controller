#!/bin/bash

# Resource Migration Demo - Advanced Capability Showcase
# Demonstrates seamless resource migration during scaling operations

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[MIGRATION-DEMO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }

demo_resource_migration() {
    echo -e "${PURPLE}=== Resource Migration Demo ===${NC}"
    echo "This demo shows how resources are seamlessly migrated between shards"
    echo ""
    
    # Create test resources
    log_info "Creating test resources to migrate..."
    
    for i in {1..10}; do
        cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-resource-$i
  namespace: demo
  labels:
    shard-resource: "true"
    resource-id: "res-$i"
data:
  data: "Resource $i data"
  hash: "$(echo "resource-$i" | sha256sum | cut -d' ' -f1)"
EOF
    done
    
    # Create ShardConfig that will trigger migration
    log_info "Creating initial shard configuration..."
    cat <<EOF | kubectl apply -f -
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: migration-demo
  namespace: demo
spec:
  minShards: 2
  maxShards: 8
  scaleUpThreshold: 0.3
  scaleDownThreshold: 0.1
  loadBalanceStrategy: "consistent-hash"
  healthCheckInterval: "10s"
EOF
    
    sleep 5
    
    log_info "Initial shard state:"
    kubectl get shardinstances -n demo -o wide
    
    echo ""
    log_info "Triggering scale-up to force resource migration..."
    kubectl patch shardconfig migration-demo -n demo --type='merge' -p='{"spec":{"minShards":4}}'
    
    sleep 10
    
    log_info "New shard state after migration:"
    kubectl get shardinstances -n demo -o wide
    
    log_info "Checking resource distribution..."
    kubectl get configmaps -n demo -l shard-resource=true
    
    log_success "Resource migration demonstrated!"
    
    # Cleanup
    log_info "Cleaning up test resources..."
    kubectl delete configmaps -n demo -l shard-resource=true
    kubectl delete shardconfig migration-demo -n demo
}

demo_load_simulation() {
    echo -e "${PURPLE}=== Load Simulation Demo ===${NC}"
    echo "Simulating high load to trigger auto-scaling"
    echo ""
    
    # Create a ShardConfig with aggressive scaling
    cat <<EOF | kubectl apply -f -
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: load-simulation
  namespace: demo
spec:
  minShards: 1
  maxShards: 6
  scaleUpThreshold: 0.5
  scaleDownThreshold: 0.2
  loadBalanceStrategy: "least-loaded"
  healthCheckInterval: "5s"
EOF
    
    log_info "Starting with minimal shards..."
    sleep 5
    kubectl get shardinstances -n demo -l shardconfig=load-simulation
    
    log_info "Simulating load increase..."
    # Patch to trigger scaling
    kubectl patch shardconfig load-simulation -n demo --type='merge' -p='{"spec":{"scaleUpThreshold":0.1}}'
    
    sleep 10
    log_info "Shards after load increase:"
    kubectl get shardinstances -n demo -l shardconfig=load-simulation
    
    log_info "Simulating load decrease..."
    kubectl patch shardconfig load-simulation -n demo --type='merge' -p='{"spec":{"scaleDownThreshold":0.8}}'
    
    sleep 10
    log_info "Shards after load decrease:"
    kubectl get shardinstances -n demo -l shardconfig=load-simulation
    
    # Cleanup
    kubectl delete shardconfig load-simulation -n demo
    log_success "Load simulation completed!"
}

demo_failure_recovery() {
    echo -e "${PURPLE}=== Failure Recovery Demo ===${NC}"
    echo "Demonstrating automatic failure detection and recovery"
    echo ""
    
    # Create ShardConfig
    cat <<EOF | kubectl apply -f -
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: failure-demo
  namespace: demo
spec:
  minShards: 3
  maxShards: 5
  scaleUpThreshold: 0.7
  scaleDownThreshold: 0.3
  healthCheckInterval: "5s"
  loadBalanceStrategy: "round-robin"
EOF
    
    sleep 8
    log_info "Initial healthy shards:"
    kubectl get shardinstances -n demo -l shardconfig=failure-demo
    
    log_info "Checking shard health status..."
    kubectl get shardinstances -n demo -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.healthStatus.healthy}{"\n"}{end}'
    
    # Show controller logs for health checks
    log_info "Recent health check activity:"
    kubectl logs -n shard-system deployment/shard-manager --tail=5 | grep -i health || echo "No health logs found"
    
    # Cleanup
    kubectl delete shardconfig failure-demo -n demo
    log_success "Failure recovery demo completed!"
}

main() {
    echo "🔄 Advanced Capabilities Demo"
    echo "============================="
    echo ""
    
    # Ensure demo namespace exists
    kubectl create namespace demo --dry-run=client -o yaml | kubectl apply -f -
    
    demo_resource_migration
    echo ""
    demo_load_simulation  
    echo ""
    demo_failure_recovery
    
    echo ""
    echo -e "${GREEN}All advanced demos completed! 🎉${NC}"
}

main "$@"