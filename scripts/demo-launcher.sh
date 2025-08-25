#!/bin/bash

# Kubernetes Shard Controller - Master Demo Launcher
# Interactive menu to demonstrate all core capabilities

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

show_main_banner() {
    clear
    echo -e "${CYAN}"
    cat << 'EOF'
╔══════════════════════════════════════════════════════════════════════╗
║                Kubernetes Shard Controller                          ║
║                     🚀 Demo Launcher 🚀                             ║
║                                                                      ║
║           Comprehensive Showcase of All Core Capabilities           ║
╚══════════════════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
}

show_menu() {
    echo -e "${BLUE}Choose a demo to run:${NC}"
    echo ""
    echo -e "${GREEN}1.${NC} 🔧 Comprehensive Demo (All capabilities)"
    echo -e "${GREEN}2.${NC} ⚡ Advanced Features (Migration, Recovery)"  
    echo -e "${GREEN}3.${NC} 📊 Performance Testing"
    echo -e "${GREEN}4.${NC} 🏗️  Step-by-step Setup & Basic Demo"
    echo -e "${GREEN}5.${NC} 🔍 Interactive Exploration Mode"
    echo -e "${GREEN}6.${NC} 📋 Show System Status"
    echo -e "${GREEN}7.${NC} 🧹 Cleanup Resources"
    echo -e "${GREEN}8.${NC} ❓ Help & Documentation"
    echo ""
    echo -e "${YELLOW}q.${NC} Quit"
    echo ""
    echo -n "Enter your choice: "
}

check_environment() {
    echo -e "${BLUE}Checking environment...${NC}"
    
    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]] || [[ ! -d "pkg" ]]; then
        echo -e "${RED}Error: Please run this script from the project root directory${NC}"
        exit 1
    fi
    
    # Check required tools
    missing_tools=()
    for tool in kubectl kind go docker; do
        if ! command -v $tool &> /dev/null; then
            missing_tools+=($tool)
        fi
    done
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        echo -e "${RED}Missing required tools: ${missing_tools[*]}${NC}"
        echo "Please install the missing tools and try again."
        exit 1
    fi
    
    echo -e "${GREEN}Environment check passed!${NC}"
}

run_comprehensive_demo() {
    echo -e "${PURPLE}Running Comprehensive Demo...${NC}"
    if [[ -f "$SCRIPT_DIR/comprehensive-demo.sh" ]]; then
        "$SCRIPT_DIR/comprehensive-demo.sh"
    else
        echo -e "${RED}Comprehensive demo script not found!${NC}"
    fi
}

run_advanced_demo() {
    echo -e "${PURPLE}Running Advanced Features Demo...${NC}"
    if [[ -f "$SCRIPT_DIR/advanced-demo.sh" ]]; then
        "$SCRIPT_DIR/advanced-demo.sh"
    else
        echo -e "${RED}Advanced demo script not found!${NC}"
    fi
}

run_performance_demo() {
    echo -e "${PURPLE}Running Performance Testing Demo...${NC}"
    if [[ -f "$SCRIPT_DIR/performance-demo.sh" ]]; then
        "$SCRIPT_DIR/performance-demo.sh"
    else
        echo -e "${RED}Performance demo script not found!${NC}"
    fi
}

run_step_by_step_demo() {
    echo -e "${PURPLE}Step-by-step Setup & Demo${NC}"
    echo ""
    echo "This will guide you through:"
    echo "1. Environment setup"
    echo "2. Controller deployment"
    echo "3. Basic functionality test"
    echo ""
    read -p "Continue? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        if [[ -f "$SCRIPT_DIR/local-demo.sh" ]]; then
            "$SCRIPT_DIR/local-demo.sh"
        else
            echo -e "${RED}Local demo script not found!${NC}"
        fi
    fi
}

interactive_exploration() {
    echo -e "${PURPLE}Interactive Exploration Mode${NC}"
    echo ""
    
    while true; do
        echo "Available commands:"
        echo "1. Show all shard resources"
        echo "2. Create test ShardConfig"
        echo "3. Check controller logs"
        echo "4. Port-forward to metrics"
        echo "5. Show controller status"
        echo "6. Back to main menu"
        echo ""
        read -p "Choose command (1-6): " choice
        
        case $choice in
            1)
                echo "All shard resources:"
                kubectl get shardconfigs,shardinstances --all-namespaces -o wide 2>/dev/null || echo "No resources found"
                ;;
            2)
                echo "Creating test ShardConfig..."
                kubectl create namespace demo --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null
                cat <<EOF | kubectl apply -f -
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: interactive-test
  namespace: demo
spec:
  minShards: 2
  maxShards: 4
  loadBalanceStrategy: "consistent-hash"
EOF
                echo "Test ShardConfig created!"
                ;;
            3)
                echo "Recent controller logs:"
                kubectl logs -n shard-system deployment/shard-manager --tail=10 2>/dev/null || echo "No logs available"
                ;;
            4)
                echo "Starting port-forward to metrics (Ctrl+C to stop)..."
                kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080 2>/dev/null || echo "Port-forward failed"
                ;;
            5)
                echo "Controller status:"
                kubectl get deployment -n shard-system -o wide 2>/dev/null || echo "Controller not deployed"
                ;;
            6)
                break
                ;;
            *)
                echo "Invalid choice"
                ;;
        esac
        echo ""
        read -p "Press Enter to continue..." -r
    done
}

show_system_status() {
    echo -e "${PURPLE}System Status${NC}"
    echo ""
    
    echo -e "${BLUE}Cluster Info:${NC}"
    kubectl cluster-info 2>/dev/null || echo "Cluster not accessible"
    
    echo ""
    echo -e "${BLUE}Controller Status:${NC}"
    kubectl get deployment -n shard-system 2>/dev/null || echo "Controller not deployed"
    
    echo ""
    echo -e "${BLUE}Custom Resources:${NC}"
    kubectl get crd | grep shard.io || echo "CRDs not installed"
    
    echo ""
    echo -e "${BLUE}Shard Resources:${NC}"
    kubectl get shardconfigs,shardinstances --all-namespaces 2>/dev/null || echo "No shard resources found"
    
    echo ""
    read -p "Press Enter to continue..." -r
}

cleanup_resources() {
    echo -e "${PURPLE}Cleanup Resources${NC}"
    echo ""
    
    echo "This will clean up:"
    echo "- Demo namespaces and resources"
    echo "- Test configurations"
    echo "- Kind clusters (optional)"
    echo ""
    read -p "Continue with cleanup? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Cleaning up demo resources..."
        kubectl delete namespace demo --ignore-not-found=true
        
        echo "Cleaning up test resources..."
        kubectl delete shardconfigs --all --all-namespaces --ignore-not-found=true
        kubectl delete shardinstances --all --all-namespaces --ignore-not-found=true
        
        read -p "Also delete controller deployment? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            kubectl delete namespace shard-system --ignore-not-found=true
            kubectl delete crd shardconfigs.shard.io shardinstances.shard.io --ignore-not-found=true
        fi
        
        read -p "Delete kind clusters? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            for cluster in $(kind get clusters 2>/dev/null); do
                if [[ $cluster == *"shard"* ]] || [[ $cluster == *"demo"* ]]; then
                    echo "Deleting cluster: $cluster"
                    kind delete cluster --name "$cluster"
                fi
            done
        fi
        
        echo -e "${GREEN}Cleanup completed!${NC}"
    fi
    
    read -p "Press Enter to continue..." -r
}

show_help() {
    echo -e "${PURPLE}Help & Documentation${NC}"
    echo ""
    echo -e "${BLUE}Available Demos:${NC}"
    echo "• Comprehensive Demo: Full showcase of all capabilities"
    echo "• Advanced Features: Migration, failure recovery, load balancing"
    echo "• Performance Testing: Scalability and performance metrics"
    echo "• Step-by-step: Guided setup and basic functionality"
    echo ""
    echo -e "${BLUE}Documentation:${NC}"
    echo "• README.md - Project overview and quick start"
    echo "• docs/ - Detailed documentation"
    echo "• examples/ - Configuration examples"
    echo "• kubernetes-分片控制器-架构文档.md - Architecture analysis (Chinese)"
    echo ""
    echo -e "${BLUE}Key Capabilities Demonstrated:${NC}"
    echo "✅ Dynamic Sharding & Auto-scaling"
    echo "✅ Load Balancing (Consistent Hash, Round Robin, Least Loaded)"
    echo "✅ Health Monitoring & Failure Recovery"
    echo "✅ Resource Migration"
    echo "✅ Observability (Metrics, Logging, Monitoring)"
    echo "✅ High Availability with Leader Election"
    echo ""
    read -p "Press Enter to continue..." -r
}

main() {
    while true; do
        show_main_banner
        show_menu
        read -r choice
        
        case $choice in
            1)
                check_environment
                run_comprehensive_demo
                ;;
            2)
                check_environment
                run_advanced_demo
                ;;
            3)
                check_environment
                run_performance_demo
                ;;
            4)
                check_environment
                run_step_by_step_demo
                ;;
            5)
                interactive_exploration
                ;;
            6)
                show_system_status
                ;;
            7)
                cleanup_resources
                ;;
            8)
                show_help
                ;;
            q|Q)
                echo -e "${GREEN}Thanks for exploring Kubernetes Shard Controller! 🎉${NC}"
                exit 0
                ;;
            *)
                echo -e "${RED}Invalid choice. Please try again.${NC}"
                sleep 2
                ;;
        esac
    done
}

main "$@"