#!/bin/bash

# Docker image build script for Shard Controller
# This script builds Docker images for both manager and worker components

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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
REGISTRY=${REGISTRY:-"shard-controller"}
TAG=${TAG:-"latest"}
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
COMMIT=${COMMIT:-$(git rev-parse HEAD 2>/dev/null || echo "unknown")}
BUILD_DATE=${BUILD_DATE:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}

# Build options
BUILD_MANAGER=true
BUILD_WORKER=true
PUSH_IMAGES=false
USE_MULTI_STAGE=false
PLATFORM="linux/amd64"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --registry)
            REGISTRY="$2"
            shift 2
            ;;
        --tag)
            TAG="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --manager-only)
            BUILD_MANAGER=true
            BUILD_WORKER=false
            shift
            ;;
        --worker-only)
            BUILD_MANAGER=false
            BUILD_WORKER=true
            shift
            ;;
        --push)
            PUSH_IMAGES=true
            shift
            ;;
        --multi-stage)
            USE_MULTI_STAGE=true
            shift
            ;;
        --platform)
            PLATFORM="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --registry REGISTRY    Docker registry (default: shard-controller)"
            echo "  --tag TAG             Docker tag (default: latest)"
            echo "  --version VERSION     Version string (default: git describe)"
            echo "  --manager-only        Build only manager image"
            echo "  --worker-only         Build only worker image"
            echo "  --push                Push images after building"
            echo "  --multi-stage         Use multi-stage Dockerfile"
            echo "  --platform PLATFORM   Target platform (default: linux/amd64)"
            echo "  --help, -h            Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "🐳 Building Shard Controller Docker Images"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  Registry: $REGISTRY"
echo "  Tag: $TAG"
echo "  Version: $VERSION"
echo "  Commit: ${COMMIT:0:8}"
echo "  Build Date: $BUILD_DATE"
echo "  Platform: $PLATFORM"
echo "  Multi-stage: $USE_MULTI_STAGE"
echo ""

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

# Check if we're in the project root
if [[ ! -f "go.mod" ]]; then
    log_error "Please run this script from the project root directory"
    exit 1
fi

# Build function for individual components
build_component() {
    local component=$1
    local dockerfile=$2
    
    log_info "Building $component image..."
    
    local build_args=(
        "--build-arg" "VERSION=$VERSION"
        "--build-arg" "COMMIT=$COMMIT"
        "--build-arg" "BUILD_DATE=$BUILD_DATE"
        "--platform" "$PLATFORM"
        "-f" "$dockerfile"
        "-t" "$REGISTRY/$component:$TAG"
    )
    
    # Add version tag if different from latest
    if [[ "$TAG" != "$VERSION" && "$VERSION" != "dev" ]]; then
        build_args+=("-t" "$REGISTRY/$component:$VERSION")
    fi
    
    build_args+=(".")
    
    if docker build "${build_args[@]}"; then
        log_success "$component image built successfully"
        
        # Show image info
        docker images "$REGISTRY/$component:$TAG" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
    else
        log_error "Failed to build $component image"
        exit 1
    fi
}

# Build function for multi-stage
build_multi_stage() {
    local component=$1
    
    log_info "Building $component image using multi-stage Dockerfile..."
    
    local build_args=(
        "--build-arg" "COMPONENT=$component"
        "--build-arg" "VERSION=$VERSION"
        "--build-arg" "COMMIT=$COMMIT"
        "--build-arg" "BUILD_DATE=$BUILD_DATE"
        "--platform" "$PLATFORM"
        "-t" "$REGISTRY/$component:$TAG"
    )
    
    # Add version tag if different from latest
    if [[ "$TAG" != "$VERSION" && "$VERSION" != "dev" ]]; then
        build_args+=("-t" "$REGISTRY/$component:$VERSION")
    fi
    
    build_args+=(".")
    
    if docker build "${build_args[@]}"; then
        log_success "$component image built successfully"
        
        # Show image info
        docker images "$REGISTRY/$component:$TAG" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
    else
        log_error "Failed to build $component image"
        exit 1
    fi
}

# Push function
push_image() {
    local component=$1
    
    log_info "Pushing $component image..."
    
    if docker push "$REGISTRY/$component:$TAG"; then
        log_success "$component:$TAG pushed successfully"
    else
        log_error "Failed to push $component:$TAG"
        exit 1
    fi
    
    # Push version tag if different
    if [[ "$TAG" != "$VERSION" && "$VERSION" != "dev" ]]; then
        if docker push "$REGISTRY/$component:$VERSION"; then
            log_success "$component:$VERSION pushed successfully"
        else
            log_error "Failed to push $component:$VERSION"
            exit 1
        fi
    fi
}

# Build images
if [[ "$USE_MULTI_STAGE" == "true" ]]; then
    if [[ "$BUILD_MANAGER" == "true" ]]; then
        build_multi_stage "manager"
    fi
    
    if [[ "$BUILD_WORKER" == "true" ]]; then
        build_multi_stage "worker"
    fi
else
    if [[ "$BUILD_MANAGER" == "true" ]]; then
        build_component "manager" "Dockerfile.manager"
    fi
    
    if [[ "$BUILD_WORKER" == "true" ]]; then
        build_component "worker" "Dockerfile.worker"
    fi
fi

# Push images if requested
if [[ "$PUSH_IMAGES" == "true" ]]; then
    log_info "Pushing images to registry..."
    
    if [[ "$BUILD_MANAGER" == "true" ]]; then
        push_image "manager"
    fi
    
    if [[ "$BUILD_WORKER" == "true" ]]; then
        push_image "worker"
    fi
fi

echo ""
log_success "✅ Build completed successfully!"
echo ""

# Show final summary
echo "Built images:"
if [[ "$BUILD_MANAGER" == "true" ]]; then
    echo "  - $REGISTRY/manager:$TAG"
    if [[ "$TAG" != "$VERSION" && "$VERSION" != "dev" ]]; then
        echo "  - $REGISTRY/manager:$VERSION"
    fi
fi

if [[ "$BUILD_WORKER" == "true" ]]; then
    echo "  - $REGISTRY/worker:$TAG"
    if [[ "$TAG" != "$VERSION" && "$VERSION" != "dev" ]]; then
        echo "  - $REGISTRY/worker:$VERSION"
    fi
fi

echo ""
echo "Next steps:"
echo "1. Test the images:"
echo "   docker run --rm $REGISTRY/manager:$TAG --help"
echo "   docker run --rm $REGISTRY/worker:$TAG --help"
echo ""
echo "2. Run with Docker Compose:"
echo "   docker-compose up -d"
echo ""
echo "3. Load into kind cluster:"
echo "   make kind-load"
echo ""
echo "4. Deploy to Kubernetes:"
echo "   kubectl apply -f manifests/"