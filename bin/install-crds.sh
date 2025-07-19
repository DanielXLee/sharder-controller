#!/bin/bash

# Script to install Custom Resource Definitions for K8s Shard Controller
# Usage: ./bin/install-crds.sh [--dry-run]

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
DRY_RUN=""
if [[ "${1:-}" == "--dry-run" ]]; then
    DRY_RUN="--dry-run=client"
    print_info "Running in dry-run mode"
fi

print_info "Installing Custom Resource Definitions..."

# Install ShardConfig CRD
print_info "Installing ShardConfig CRD..."
if kubectl apply -f "$CRD_DIR/shardconfig-crd.yaml" $DRY_RUN; then
    print_info "✓ ShardConfig CRD installed successfully"
else
    print_error "✗ Failed to install ShardConfig CRD"
    exit 1
fi

# Install ShardInstance CRD
print_info "Installing ShardInstance CRD..."
if kubectl apply -f "$CRD_DIR/shardinstance-crd.yaml" $DRY_RUN; then
    print_info "✓ ShardInstance CRD installed successfully"
else
    print_error "✗ Failed to install ShardInstance CRD"
    exit 1
fi

if [[ -z "$DRY_RUN" ]]; then
    # Wait for CRDs to be established
    print_info "Waiting for CRDs to be established..."
    
    if kubectl wait --for condition=established --timeout=60s crd/shardconfigs.shard.io; then
        print_info "✓ ShardConfig CRD is established"
    else
        print_error "✗ ShardConfig CRD failed to establish"
        exit 1
    fi
    
    if kubectl wait --for condition=established --timeout=60s crd/shardinstances.shard.io; then
        print_info "✓ ShardInstance CRD is established"
    else
        print_error "✗ ShardInstance CRD failed to establish"
        exit 1
    fi
    
    print_info "All CRDs installed and established successfully!"
    
    # Show installed CRDs
    print_info "Installed CRDs:"
    kubectl get crd | grep shard.io || true
else
    print_info "Dry-run completed successfully!"
fi