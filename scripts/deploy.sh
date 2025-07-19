#!/bin/bash

# Shard Controller Deployment Script
set -e

# Configuration
NAMESPACE=${NAMESPACE:-"shard-system"}
DEPLOYMENT_METHOD=${DEPLOYMENT_METHOD:-"helm"}
HELM_RELEASE_NAME=${HELM_RELEASE_NAME:-"shard-controller"}
MANIFESTS_DIR=${MANIFESTS_DIR:-"manifests"}
HELM_CHART_DIR=${HELM_CHART_DIR:-"helm/shard-controller"}
VALUES_FILE=${VALUES_FILE:-""}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check cluster admin permissions
    if ! kubectl auth can-i create customresourcedefinitions &> /dev/null; then
        log_error "Insufficient permissions. Cluster admin access required."
        exit 1
    fi
    
    # Check Helm if using Helm deployment
    if [[ "$DEPLOYMENT_METHOD" == "helm" ]]; then
        if ! command -v helm &> /dev/null; then
            log_error "Helm is not installed or not in PATH"
            exit 1
        fi
    fi
    
    log_info "Prerequisites check passed"
}

# Deploy using Helm
deploy_helm() {
    log_info "Deploying using Helm..."
    
    # Check if chart directory exists
    if [[ ! -d "$HELM_CHART_DIR" ]]; then
        log_error "Helm chart directory not found: $HELM_CHART_DIR"
        exit 1
    fi
    
    # Prepare Helm command
    HELM_CMD="helm upgrade --install $HELM_RELEASE_NAME $HELM_CHART_DIR"
    HELM_CMD="$HELM_CMD --namespace $NAMESPACE --create-namespace"
    
    # Add values file if specified
    if [[ -n "$VALUES_FILE" && -f "$VALUES_FILE" ]]; then
        HELM_CMD="$HELM_CMD --values $VALUES_FILE"
        log_info "Using values file: $VALUES_FILE"
    fi
    
    # Execute Helm command
    log_info "Executing: $HELM_CMD"
    if eval "$HELM_CMD"; then
        log_info "Helm deployment successful"
    else
        log_error "Helm deployment failed"
        exit 1
    fi
}

# Deploy using kubectl
deploy_kubectl() {
    log_info "Deploying using kubectl..."
    
    # Check if manifests directory exists
    if [[ ! -d "$MANIFESTS_DIR" ]]; then
        log_error "Manifests directory not found: $MANIFESTS_DIR"
        exit 1
    fi
    
    # Deploy CRDs first
    log_info "Installing Custom Resource Definitions..."
    if [[ -d "$MANIFESTS_DIR/crds" ]]; then
        kubectl apply -f "$MANIFESTS_DIR/crds/"
    else
        log_warn "CRDs directory not found, skipping..."
    fi
    
    # Create namespace
    log_info "Creating namespace: $NAMESPACE"
    kubectl apply -f "$MANIFESTS_DIR/namespace.yaml"
    
    # Deploy RBAC
    log_info "Deploying RBAC configuration..."
    kubectl apply -f "$MANIFESTS_DIR/rbac.yaml"
    
    # Deploy ConfigMaps
    log_info "Deploying configuration..."
    kubectl apply -f "$MANIFESTS_DIR/configmap.yaml"
    
    # Deploy Services
    log_info "Deploying services..."
    kubectl apply -f "$MANIFESTS_DIR/services.yaml"
    
    # Deploy Controllers
    log_info "Deploying controllers..."
    kubectl apply -f "$MANIFESTS_DIR/deployment/"
    
    # Deploy monitoring (if exists)
    if [[ -d "$MANIFESTS_DIR/monitoring" ]]; then
        log_info "Deploying monitoring configuration..."
        kubectl apply -f "$MANIFESTS_DIR/monitoring/" || log_warn "Monitoring deployment failed, continuing..."
    fi
    
    log_info "kubectl deployment successful"
}

# Wait for deployment to be ready
wait_for_deployment() {
    log_info "Waiting for deployment to be ready..."
    
    # Wait for manager deployment
    log_info "Waiting for manager deployment..."
    kubectl rollout status deployment/shard-manager -n "$NAMESPACE" --timeout=300s
    
    # Wait for worker deployment
    log_info "Waiting for worker deployment..."
    kubectl rollout status deployment/shard-worker -n "$NAMESPACE" --timeout=300s
    
    log_info "All deployments are ready"
}

# Verify deployment
verify_deployment() {
    log_info "Verifying deployment..."
    
    # Check pods
    log_info "Checking pod status..."
    kubectl get pods -n "$NAMESPACE"
    
    # Check services
    log_info "Checking services..."
    kubectl get svc -n "$NAMESPACE"
    
    # Check custom resources
    log_info "Checking custom resources..."
    kubectl get shardconfigs,shardinstances --all-namespaces || log_warn "No custom resources found yet"
    
    # Health check
    log_info "Performing health check..."
    if kubectl get pods -n "$NAMESPACE" -l app=shard-manager -o jsonpath='{.items[0].status.phase}' | grep -q "Running"; then
        log_info "Manager pod is running"
    else
        log_warn "Manager pod is not running"
    fi
    
    if kubectl get pods -n "$NAMESPACE" -l app=shard-worker -o jsonpath='{.items[*].status.phase}' | grep -q "Running"; then
        log_info "Worker pods are running"
    else
        log_warn "Worker pods are not running"
    fi
    
    log_info "Deployment verification completed"
}

# Show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Deploy Kubernetes Shard Controller

OPTIONS:
    -m, --method METHOD     Deployment method: helm or kubectl (default: helm)
    -n, --namespace NAME    Target namespace (default: shard-system)
    -r, --release NAME      Helm release name (default: shard-controller)
    -v, --values FILE       Helm values file
    -h, --help             Show this help message

ENVIRONMENT VARIABLES:
    NAMESPACE              Target namespace
    DEPLOYMENT_METHOD      Deployment method (helm or kubectl)
    HELM_RELEASE_NAME      Helm release name
    VALUES_FILE            Helm values file path

EXAMPLES:
    # Deploy using Helm with default settings
    $0

    # Deploy using kubectl
    $0 --method kubectl

    # Deploy with custom values file
    $0 --values custom-values.yaml

    # Deploy to custom namespace
    $0 --namespace my-shard-system
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--method)
            DEPLOYMENT_METHOD="$2"
            shift 2
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -r|--release)
            HELM_RELEASE_NAME="$2"
            shift 2
            ;;
        -v|--values)
            VALUES_FILE="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Validate deployment method
if [[ "$DEPLOYMENT_METHOD" != "helm" && "$DEPLOYMENT_METHOD" != "kubectl" ]]; then
    log_error "Invalid deployment method: $DEPLOYMENT_METHOD. Must be 'helm' or 'kubectl'"
    exit 1
fi

# Main execution
main() {
    log_info "Starting Shard Controller deployment..."
    log_info "Deployment method: $DEPLOYMENT_METHOD"
    log_info "Target namespace: $NAMESPACE"
    
    check_prerequisites
    
    case "$DEPLOYMENT_METHOD" in
        "helm")
            deploy_helm
            ;;
        "kubectl")
            deploy_kubectl
            ;;
    esac
    
    wait_for_deployment
    verify_deployment
    
    log_info "Deployment completed successfully!"
    log_info "You can now create ShardConfig resources to start using the controller."
}

# Run main function
main