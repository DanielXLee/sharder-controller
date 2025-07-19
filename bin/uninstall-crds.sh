#!/bin/bash

# Script to uninstall Custom Resource Definitions for K8s Shard Controller
# Usage: ./bin/uninstall-crds.sh [--force]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CRD_DIR="$PROJECT_ROOT/manifests/crds"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is not installed or not in PATH"
    exit 1
fi

# Check if we can connect to the cluster
if ! kubectl cluster-info &> /dev/null; then
    print_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
    exit 1
fi

# Parse command line arguments
FORCE=""
if [[ "${1:-}" == "--force" ]]; then
    FORCE="--force"
    print_warn "Force mode enabled - will not prompt for confirmation"
fi

# Check if CRDs exist and have instances
print_info "Checking for existing CRD instances..."

SHARDCONFIG_COUNT=0
SHARDINSTANCE_COUNT=0

if kubectl get crd shardconfigs.shard.io &> /dev/null; then
    SHARDCONFIG_COUNT=$(kubectl get shardconfigs --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
fi

if kubectl get crd shardinstances.shard.io &> /dev/null; then
    SHARDINSTANCE_COUNT=$(kubectl get shardinstances --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
fi

TOTAL_INSTANCES=$((SHARDCONFIG_COUNT + SHARDINSTANCE_COUNT))

if [[ $TOTAL_INSTANCES -gt 0 ]] && [[ -z "$FORCE" ]]; then
    print_warn "Found $SHARDCONFIG_COUNT ShardConfig(s) and $SHARDINSTANCE_COUNT ShardInstance(s)"
    print_warn "Uninstalling CRDs will delete ALL instances of these resources!"
    echo -n "Are you sure you want to continue? (y/N): "
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        print_info "Uninstallation cancelled"
        exit 0
    fi
fi

print_info "Uninstalling Custom Resource Definitions..."

# Uninstall ShardInstance CRD first (to avoid dependency issues)
print_info "Uninstalling ShardInstance CRD..."
if kubectl delete -f "$CRD_DIR/shardinstance-crd.yaml" --ignore-not-found=true; then
    print_info "✓ ShardInstance CRD uninstalled successfully"
else
    print_error "✗ Failed to uninstall ShardInstance CRD"
    exit 1
fi

# Uninstall ShardConfig CRD
print_info "Uninstalling ShardConfig CRD..."
if kubectl delete -f "$CRD_DIR/shardconfig-crd.yaml" --ignore-not-found=true; then
    print_info "✓ ShardConfig CRD uninstalled successfully"
else
    print_error "✗ Failed to uninstall ShardConfig CRD"
    exit 1
fi

# Wait for CRDs to be fully removed
print_info "Waiting for CRDs to be fully removed..."
timeout=60
while [[ $timeout -gt 0 ]]; do
    if ! kubectl get crd shardconfigs.shard.io &> /dev/null && \
       ! kubectl get crd shardinstances.shard.io &> /dev/null; then
        break
    fi
    sleep 1
    ((timeout--))
done

if [[ $timeout -eq 0 ]]; then
    print_warn "CRDs may still be in the process of being removed"
else
    print_info "All CRDs removed successfully!"
fi

# Verify removal
print_info "Verifying CRD removal..."
REMAINING_CRDS=$(kubectl get crd | grep shard.io | wc -l || echo "0")
if [[ $REMAINING_CRDS -eq 0 ]]; then
    print_info "✓ All shard.io CRDs have been removed"
else
    print_warn "Some shard.io CRDs may still be present:"
    kubectl get crd | grep shard.io || true
fi