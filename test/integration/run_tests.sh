#!/bin/bash

# Integration Test Runner Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
USE_KIND=${USE_KIND:-false}
CLEANUP=${CLEANUP:-true}
VERBOSE=${VERBOSE:-false}
TEST_PATTERN=${TEST_PATTERN:-""}
TIMEOUT=${TIMEOUT:-30m}

# Print usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -k, --kind           Use Kind cluster for testing"
    echo "  -c, --no-cleanup     Don't cleanup test environment after tests"
    echo "  -v, --verbose        Enable verbose output"
    echo "  -p, --pattern PATTERN Run only tests matching pattern"
    echo "  -t, --timeout TIMEOUT Test timeout (default: 30m)"
    echo "  -h, --help           Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 --kind                    # Run all tests with Kind"
    echo "  $0 --pattern LoadBalancing   # Run only load balancing tests"
    echo "  $0 --verbose --no-cleanup    # Run with verbose output, no cleanup"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -k|--kind)
            USE_KIND=true
            shift
            ;;
        -c|--no-cleanup)
            CLEANUP=false
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -p|--pattern)
            TEST_PATTERN="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Logging functions
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

# Cleanup function
cleanup() {
    if [ "$CLEANUP" = true ]; then
        log_info "Cleaning up test environment..."
        
        if [ "$USE_KIND" = true ]; then
            if kind get clusters | grep -q "shard-controller-test"; then
                log_info "Deleting Kind cluster..."
                kind delete cluster --name shard-controller-test || true
            fi
        fi
        
        # Clean up any test artifacts
        rm -f "${PROJECT_ROOT}/coverage.out" || true
        rm -f "${PROJECT_ROOT}/test-results.xml" || true
    fi
}

# Set up trap for cleanup
trap cleanup EXIT

# Main execution
main() {
    log_info "Starting integration tests..."
    log_info "Configuration:"
    log_info "  - Use Kind: $USE_KIND"
    log_info "  - Cleanup: $CLEANUP"
    log_info "  - Verbose: $VERBOSE"
    log_info "  - Test Pattern: ${TEST_PATTERN:-'all'}"
    log_info "  - Timeout: $TIMEOUT"
    
    # Change to project root
    cd "$PROJECT_ROOT"
    
    # Setup test environment
    log_info "Setting up test environment..."
    if [ "$USE_KIND" = true ]; then
        export USE_KIND=true
        ./test/integration/setup.sh
    else
        ./test/integration/setup.sh
    fi
    
    # Build test flags
    TEST_FLAGS="-v -timeout $TIMEOUT"
    
    if [ "$VERBOSE" = true ]; then
        TEST_FLAGS="$TEST_FLAGS -race"
    fi
    
    if [ -n "$TEST_PATTERN" ]; then
        TEST_FLAGS="$TEST_FLAGS -run $TEST_PATTERN"
    fi
    
    # Add coverage if not running specific pattern
    if [ -z "$TEST_PATTERN" ]; then
        TEST_FLAGS="$TEST_FLAGS -coverprofile=coverage.out"
    fi
    
    # Run tests
    log_info "Running integration tests with flags: $TEST_FLAGS"
    
    if go test $TEST_FLAGS ./test/integration/...; then
        log_success "All integration tests passed!"
        
        # Generate coverage report if coverage file exists
        if [ -f "coverage.out" ]; then
            log_info "Generating coverage report..."
            go tool cover -html=coverage.out -o coverage.html
            log_success "Coverage report generated: coverage.html"
            
            # Show coverage summary
            COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
            log_info "Total coverage: $COVERAGE"
        fi
        
        return 0
    else
        log_error "Integration tests failed!"
        return 1
    fi
}

# Run main function
main "$@"