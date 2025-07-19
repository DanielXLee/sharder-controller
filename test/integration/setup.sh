#!/bin/bash

# Integration Test Environment Setup Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "Setting up integration test environment..."

# Check if required tools are installed
check_tool() {
    if ! command -v "$1" &> /dev/null; then
        echo "Error: $1 is not installed or not in PATH"
        exit 1
    fi
}

echo "Checking required tools..."
check_tool go
check_tool kubectl

# Check if Kind is available for local testing
if command -v kind &> /dev/null; then
    echo "Kind is available for local cluster testing"
    USE_KIND=true
else
    echo "Kind not found, will use existing cluster or envtest"
    USE_KIND=false
fi

# Setup test environment based on available tools
setup_test_env() {
    if [ "$USE_KIND" = true ] && [ "${USE_EXISTING_CLUSTER:-}" != "true" ]; then
        echo "Setting up Kind cluster for integration tests..."
        
        # Create Kind cluster if it doesn't exist
        if ! kind get clusters | grep -q "shard-controller-test"; then
            echo "Creating Kind cluster..."
            cat <<EOF | kind create cluster --name shard-controller-test --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
EOF
        else
            echo "Kind cluster already exists"
        fi
        
        # Set kubectl context
        kubectl cluster-info --context kind-shard-controller-test
        
        # Install CRDs
        echo "Installing CRDs..."
        kubectl apply -f "${PROJECT_ROOT}/manifests/crds/"
        
        export USE_EXISTING_CLUSTER=true
        export KUBECONFIG="${HOME}/.kube/config"
        
    else
        echo "Using envtest for integration tests..."
        # Download envtest binaries if not present
        if [ ! -d "${PROJECT_ROOT}/bin" ]; then
            mkdir -p "${PROJECT_ROOT}/bin"
        fi
        
        # Setup envtest
        go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
        export KUBEBUILDER_ASSETS=$(setup-envtest use --bin-dir "${PROJECT_ROOT}/bin" -p path)
    fi
}

# Run the setup
setup_test_env

echo "Integration test environment setup complete!"

# Print environment info
echo ""
echo "Environment Information:"
echo "- Project Root: ${PROJECT_ROOT}"
echo "- Use Kind: ${USE_KIND}"
echo "- Use Existing Cluster: ${USE_EXISTING_CLUSTER:-false}"
if [ -n "${KUBEBUILDER_ASSETS:-}" ]; then
    echo "- Kubebuilder Assets: ${KUBEBUILDER_ASSETS}"
fi

echo ""
echo "To run integration tests:"
echo "  make test-integration"
echo ""
echo "To cleanup Kind cluster:"
echo "  kind delete cluster --name shard-controller-test"