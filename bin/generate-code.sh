#!/bin/bash

# Script to generate Kubernetes client code for Custom Resources
# This script uses k8s.io/code-generator to generate:
# - Clientset
# - Listers
# - Informers
# - Deep copy functions

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

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

# Check if we're in the right directory
if [[ ! -f "$PROJECT_ROOT/go.mod" ]]; then
    print_error "go.mod not found. Please run this script from the project root or bin directory."
    exit 1
fi

# Set up environment variables
export GO111MODULE=on
export GOPATH=$(go env GOPATH)
export GOROOT=$(go env GOROOT)

# Module and package information
MODULE_NAME="github.com/k8s-shard-controller"
API_PKG="pkg/apis"
OUTPUT_PKG="pkg/generated"
GROUP_VERSION="shard:v1"

print_info "Generating Kubernetes client code..."
print_info "Module: $MODULE_NAME"
print_info "API Package: $API_PKG"
print_info "Output Package: $OUTPUT_PKG"
print_info "Group Version: $GROUP_VERSION"

# Create output directory if it doesn't exist
mkdir -p "$PROJECT_ROOT/$OUTPUT_PKG"

# Change to project root
cd "$PROJECT_ROOT"

# Generate deep copy functions
print_info "Generating deep copy functions..."
go run k8s.io/code-generator/cmd/deepcopy-gen \
    --input-dirs="$MODULE_NAME/$API_PKG/shard/v1" \
    --output-file-base=zz_generated.deepcopy \
    --go-header-file=/dev/null

# Generate clientset
print_info "Generating clientset..."
go run k8s.io/code-generator/cmd/client-gen \
    --clientset-name=versioned \
    --input-base="" \
    --input="$MODULE_NAME/$API_PKG/shard/v1" \
    --output-package="$MODULE_NAME/$OUTPUT_PKG/clientset" \
    --go-header-file=/dev/null

# Generate listers
print_info "Generating listers..."
go run k8s.io/code-generator/cmd/lister-gen \
    --input-dirs="$MODULE_NAME/$API_PKG/shard/v1" \
    --output-package="$MODULE_NAME/$OUTPUT_PKG/listers" \
    --go-header-file=/dev/null

# Generate informers
print_info "Generating informers..."
go run k8s.io/code-generator/cmd/informer-gen \
    --input-dirs="$MODULE_NAME/$API_PKG/shard/v1" \
    --versioned-clientset-package="$MODULE_NAME/$OUTPUT_PKG/clientset/versioned" \
    --listers-package="$MODULE_NAME/$OUTPUT_PKG/listers" \
    --output-package="$MODULE_NAME/$OUTPUT_PKG/informers" \
    --go-header-file=/dev/null

print_info "Code generation completed successfully!"

# Verify generated files
print_info "Verifying generated files..."

EXPECTED_FILES=(
    "pkg/apis/shard/v1/zz_generated.deepcopy.go"
    "pkg/generated/clientset/versioned/clientset.go"
    "pkg/generated/listers/shard/v1/shardconfig.go"
    "pkg/generated/listers/shard/v1/shardinstance.go"
    "pkg/generated/informers/externalversions/factory.go"
)

ALL_FOUND=true
for file in "${EXPECTED_FILES[@]}"; do
    if [[ -f "$PROJECT_ROOT/$file" ]]; then
        print_info "✓ $file"
    else
        print_error "✗ $file (missing)"
        ALL_FOUND=false
    fi
done

if [[ "$ALL_FOUND" == true ]]; then
    print_info "All expected files generated successfully!"
    
    # Run go mod tidy to clean up dependencies
    print_info "Running go mod tidy..."
    go mod tidy
    
    print_info "Code generation process completed!"
else
    print_error "Some files were not generated. Please check the output above."
    exit 1
fi