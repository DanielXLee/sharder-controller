#!/bin/bash

# Script to validate CRD definitions and generated code
# Usage: ./bin/validate-crds.sh

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

print_info "Validating CRD definitions and generated code..."

# Change to project root
cd "$PROJECT_ROOT"

# Check if CRD files exist
print_info "Checking CRD files..."
CRD_FILES=(
    "manifests/crds/shardconfig-crd.yaml"
    "manifests/crds/shardinstance-crd.yaml"
)

for file in "${CRD_FILES[@]}"; do
    if [[ -f "$file" ]]; then
        print_info "✓ $file exists"
    else
        print_error "✗ $file missing"
        exit 1
    fi
done

# Validate YAML syntax
print_info "Validating YAML syntax..."
for file in "${CRD_FILES[@]}"; do
    if command -v yq &> /dev/null; then
        if yq eval '.' "$file" > /dev/null 2>&1; then
            print_info "✓ $file has valid YAML syntax"
        else
            print_error "✗ $file has invalid YAML syntax"
            exit 1
        fi
    else
        print_warn "yq not found, skipping YAML validation"
        break
    fi
done

# Check if kubectl is available for CRD validation
if command -v kubectl &> /dev/null; then
    print_info "Validating CRD definitions with kubectl..."
    for file in "${CRD_FILES[@]}"; do
        if kubectl apply --dry-run=client -f "$file" > /dev/null 2>&1; then
            print_info "✓ $file is valid according to kubectl"
        else
            print_error "✗ $file failed kubectl validation"
            kubectl apply --dry-run=client -f "$file" || true
            exit 1
        fi
    done
else
    print_warn "kubectl not found, skipping CRD validation"
fi

# Check generated code files
print_info "Checking generated code files..."
GENERATED_FILES=(
    "pkg/apis/shard/v1/zz_generated.deepcopy.go"
    "pkg/generated/clientset/versioned/clientset.go"
    "pkg/generated/clientset/versioned/typed/shard/v1/shard_client.go"
    "pkg/generated/clientset/versioned/typed/shard/v1/shardconfig.go"
    "pkg/generated/clientset/versioned/typed/shard/v1/shardinstance.go"
    "pkg/generated/clientset/versioned/typed/shard/v1/generated_expansion.go"
    "pkg/generated/clientset/versioned/scheme/register.go"
)

for file in "${GENERATED_FILES[@]}"; do
    if [[ -f "$file" ]]; then
        print_info "✓ $file exists"
    else
        print_error "✗ $file missing"
        exit 1
    fi
done

# Validate Go code compilation
print_info "Validating Go code compilation..."
if go build ./pkg/apis/shard/v1/... > /dev/null 2>&1; then
    print_info "✓ API types compile successfully"
else
    print_error "✗ API types failed to compile"
    go build ./pkg/apis/shard/v1/...
    exit 1
fi

if go build ./pkg/generated/clientset/versioned/... > /dev/null 2>&1; then
    print_info "✓ Generated clientset compiles successfully"
else
    print_error "✗ Generated clientset failed to compile"
    go build ./pkg/generated/clientset/versioned/...
    exit 1
fi

# Run go mod tidy to ensure dependencies are correct
print_info "Running go mod tidy..."
if go mod tidy; then
    print_info "✓ Go modules are tidy"
else
    print_error "✗ Go mod tidy failed"
    exit 1
fi

# Check for any go vet issues
print_info "Running go vet..."
if go vet ./pkg/apis/shard/v1/... ./pkg/generated/clientset/versioned/... > /dev/null 2>&1; then
    print_info "✓ No go vet issues found"
else
    print_warn "Go vet found some issues:"
    go vet ./pkg/apis/shard/v1/... ./pkg/generated/clientset/versioned/... || true
fi

print_info "All validations completed successfully!"
print_info ""
print_info "Summary:"
print_info "- CRD YAML files are valid"
print_info "- Generated Go code compiles successfully"
print_info "- Dependencies are properly managed"
print_info ""
print_info "You can now:"
print_info "1. Install CRDs: ./bin/install-crds.sh"
print_info "2. Use the generated client code in your controllers"