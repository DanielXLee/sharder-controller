#!/bin/bash

# Demo System Verification Script
# Validates that all demo scripts are properly configured

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log_info() { echo -e "${BLUE}[VERIFY]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }

verify_script_exists() {
    local script="$1"
    local description="$2"
    
    if [[ -f "$SCRIPT_DIR/$script" ]]; then
        log_success "$description ($script)"
        return 0
    else
        log_error "$description ($script) - File not found"
        return 1
    fi
}

verify_script_executable() {
    local script="$1"
    
    if [[ -x "$SCRIPT_DIR/$script" ]]; then
        log_success "$script is executable"
        return 0
    else
        log_warning "$script is not executable"
        chmod +x "$SCRIPT_DIR/$script" 2>/dev/null && log_success "Fixed permissions for $script" || log_error "Failed to fix permissions for $script"
        return 1
    fi
}

verify_script_syntax() {
    local script="$1"
    
    if bash -n "$SCRIPT_DIR/$script" 2>/dev/null; then
        log_success "$script syntax is valid"
        return 0
    else
        log_error "$script has syntax errors"
        return 1
    fi
}

verify_makefile_targets() {
    local makefile="../Makefile"
    
    if [[ -f "$makefile" ]]; then
        log_info "Checking Makefile targets..."
        
        local targets=("demo" "demo-comprehensive" "demo-advanced" "demo-performance" "demo-launcher" "demo-cleanup")
        
        for target in "${targets[@]}"; do
            if grep -q "^$target:" "$makefile"; then
                log_success "Makefile target: $target"
            else
                log_error "Missing Makefile target: $target"
            fi
        done
    else
        log_error "Makefile not found"
    fi
}

verify_demo_config() {
    local config_file="demo-config.env"
    
    if [[ -f "$SCRIPT_DIR/$config_file" ]]; then
        log_success "Demo configuration file exists"
        
        # Check for required variables
        local required_vars=("DEMO_CLUSTER_NAME" "DEMO_NAMESPACE" "CONTROLLER_NAMESPACE")
        
        for var in "${required_vars[@]}"; do
            if grep -q "^$var=" "$SCRIPT_DIR/$config_file"; then
                log_success "Configuration variable: $var"
            else
                log_warning "Missing configuration variable: $var"
            fi
        done
    else
        log_error "Demo configuration file not found"
    fi
}

verify_prerequisites() {
    log_info "Checking prerequisites..."
    
    local tools=("kubectl" "kind" "go" "docker" "curl")
    local missing_tools=()
    
    for tool in "${tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            log_success "$tool is available"
        else
            log_warning "$tool is not available"
            missing_tools+=("$tool")
        fi
    done
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_warning "Missing tools: ${missing_tools[*]}"
        echo "These tools are required for full demo functionality"
    fi
}

verify_project_structure() {
    log_info "Checking project structure..."
    
    local required_dirs=("pkg" "cmd" "manifests" "examples")
    
    for dir in "${required_dirs[@]}"; do
        if [[ -d "../$dir" ]]; then
            log_success "Directory exists: $dir"
        else
            log_error "Missing directory: $dir"
        fi
    done
    
    local required_files=("go.mod" "Makefile" "README.md")
    
    for file in "${required_files[@]}"; do
        if [[ -f "../$file" ]]; then
            log_success "File exists: $file"
        else
            log_error "Missing file: $file"
        fi
    done
}

generate_verification_report() {
    echo ""
    echo "=== Verification Report ==="
    echo "Demo system verification completed at $(date)"
    echo ""
    echo "Available demo scripts:"
    ls -la "$SCRIPT_DIR"/*.sh | grep -E "(demo|launcher)" || echo "No demo scripts found"
    echo ""
    echo "To run the demos:"
    echo "  make demo                 # Interactive launcher"
    echo "  make demo-comprehensive   # Full demo"
    echo "  make demo-advanced       # Advanced features"
    echo "  make demo-performance    # Performance testing"
    echo ""
    echo "For help:"
    echo "  ./scripts/demo-launcher.sh help"
    echo "  cat scripts/README-DEMOS.md"
}

main() {
    echo "🔍 Demo System Verification"
    echo "=========================="
    echo ""
    
    # Verify we're in the right location
    if [[ ! -f "../go.mod" ]]; then
        log_error "Please run this script from the project root/scripts directory"
        exit 1
    fi
    
    log_info "Verifying demo scripts..."
    
    # Check demo scripts
    verify_script_exists "demo-launcher.sh" "Interactive Demo Launcher"
    verify_script_exists "comprehensive-demo.sh" "Comprehensive Demo"
    verify_script_exists "advanced-demo.sh" "Advanced Features Demo"
    verify_script_exists "performance-demo.sh" "Performance Testing Demo"
    
    echo ""
    log_info "Checking script permissions..."
    
    for script in demo-launcher.sh comprehensive-demo.sh advanced-demo.sh performance-demo.sh; do
        if [[ -f "$SCRIPT_DIR/$script" ]]; then
            verify_script_executable "$script"
        fi
    done
    
    echo ""
    log_info "Validating script syntax..."
    
    for script in demo-launcher.sh comprehensive-demo.sh advanced-demo.sh performance-demo.sh; do
        if [[ -f "$SCRIPT_DIR/$script" ]]; then
            verify_script_syntax "$script"
        fi
    done
    
    echo ""
    verify_demo_config
    
    echo ""
    verify_makefile_targets
    
    echo ""
    verify_project_structure
    
    echo ""
    verify_prerequisites
    
    echo ""
    generate_verification_report
    
    log_success "Demo system verification completed!"
}

main "$@"