# Kubernetes Shard Controller - Demo System

This directory contains a comprehensive demo system that showcases all core capabilities of the Kubernetes Shard Controller project.

## 🚀 Quick Start

### Interactive Demo Launcher (Recommended)
```bash
# From project root directory
make demo
# or
./scripts/demo-launcher.sh
```

### Individual Demos
```bash
# Comprehensive demo (all capabilities)
make demo-comprehensive

# Advanced features (migration, recovery)
make demo-advanced

# Performance testing
make demo-performance

# Clean up resources
make demo-cleanup
```

## 📋 Available Demos

### 1. Comprehensive Demo (`comprehensive-demo.sh`)
**Complete showcase of all core capabilities**

**Features Demonstrated:**
- ✅ Dynamic Sharding & Auto-scaling
- ✅ Load Balancing Strategies (Consistent Hash, Round Robin, Least Loaded)
- ✅ Health Monitoring & Failure Detection
- ✅ Observability (Metrics, Logging, Monitoring)
- ✅ Configuration Management

**Duration:** ~10-15 minutes  
**Requirements:** Kind cluster or existing Kubernetes cluster

### 2. Advanced Features Demo (`advanced-demo.sh`)
**Deep dive into advanced capabilities**

**Features Demonstrated:**
- 🔄 Resource Migration during scaling
- 📈 Load Simulation and auto-scaling triggers
- 🛡️ Failure Recovery mechanisms
- ⚡ Real-time configuration changes

**Duration:** ~8-12 minutes  
**Requirements:** Kubernetes cluster with controller deployed

### 3. Performance Testing Demo (`performance-demo.sh`)
**Scalability and performance validation**

**Features Demonstrated:**
- 📊 Scale performance testing (5, 10, 20+ shards)
- ⚖️ Load balancing strategy performance comparison
- 📈 Controller metrics collection and analysis
- 🔄 Reconciliation loop performance testing

**Duration:** ~15-20 minutes  
**Requirements:** Kubernetes cluster, metrics collection enabled

### 4. Interactive Demo Launcher (`demo-launcher.sh`)
**User-friendly menu system**

**Features:**
- 🎯 Interactive menu for all demo options
- 🔍 Live system exploration mode
- 📊 Real-time status monitoring
- 🧹 Comprehensive cleanup options
- ❓ Built-in help and documentation

## 🛠️ Prerequisites

### Required Tools
- **kubectl** - Kubernetes command-line tool
- **kind** - Kubernetes in Docker (for local clusters)
- **Go 1.20+** - For building the controller
- **Docker** - For containerization
- **curl** - For HTTP endpoint testing

### Environment Setup
```bash
# Check prerequisites
./scripts/demo-launcher.sh

# Or manually verify
kubectl version --client
kind version
go version
docker version
```

## 🎯 Demo Scenarios

### Scenario 1: First-time User
```bash
# Start with the interactive launcher
make demo
# Choose option 1: Comprehensive Demo
```

### Scenario 2: Developer/Contributor
```bash
# Run all demos sequentially
make demo-comprehensive
make demo-advanced
make demo-performance
```

### Scenario 3: Performance Evaluation
```bash
# Focus on performance testing
make demo-performance
```

### Scenario 4: Feature Exploration
```bash
# Use interactive mode
./scripts/demo-launcher.sh
# Choose option 5: Interactive Exploration Mode
```

## 📊 What Each Demo Shows

### Dynamic Sharding Capabilities
- Automatic shard creation based on load
- Scale-up/scale-down operations
- Configuration-driven shard management
- Real-time shard status monitoring

### Load Balancing Strategies
- **Consistent Hash**: Optimal for caching scenarios
- **Round Robin**: Even distribution across shards
- **Least Loaded**: Dynamic load-based assignment

### Health Monitoring
- Automatic health checks
- Failure detection and reporting
- Recovery mechanisms
- Health endpoint validation

### Observability Features
- Prometheus metrics collection
- Structured logging
- Real-time monitoring
- Performance metrics

### Resource Migration
- Seamless resource movement between shards
- Zero-downtime migrations
- Load balancing during migration
- Migration status tracking

## 🔧 Configuration

### Demo Configuration (`demo-config.env`)
```bash
# Cluster settings
DEMO_CLUSTER_NAME="shard-controller-demo"
DEMO_NAMESPACE="demo" 
CONTROLLER_NAMESPACE="shard-system"

# Performance settings
PERF_TEST_SCALES=(5 10 20)
PERF_TEST_TIMEOUT=300

# Demo parameters
DEFAULT_MIN_SHARDS=2
DEFAULT_MAX_SHARDS=8
```

### Customization
Edit `scripts/demo-config.env` to customize:
- Cluster names and namespaces
- Scaling thresholds and limits
- Performance test parameters
- Timeout values

## 🔍 Troubleshooting

### Common Issues

**1. "kubectl not found"**
```bash
# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && sudo mv kubectl /usr/local/bin/
```

**2. "kind not found"**
```bash
# Install kind
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind
```

**3. "Demo fails with permission errors"**
```bash
# Ensure scripts are executable
chmod +x scripts/*.sh
```

**4. "Controller not responding"**
```bash
# Check controller status
kubectl get deployment -n shard-system
kubectl logs -n shard-system deployment/shard-manager
```

**5. "Port-forward fails"**
```bash
# Check if port is already in use
lsof -i :8080
# Kill conflicting processes
kill $(lsof -t -i:8080)
```

### Debug Mode
Enable debug output in any demo:
```bash
# Set debug environment variable
export DEMO_DEBUG=true
./scripts/comprehensive-demo.sh
```

### Log Files
Demo logs are saved to `/tmp/shard-controller-demo.log`

## 🧹 Cleanup

### Manual Cleanup
```bash
# Clean demo resources only
kubectl delete namespace demo

# Clean everything including controller
kubectl delete namespace shard-system
kubectl delete crd shardconfigs.shard.io shardinstances.shard.io

# Clean kind clusters
kind delete cluster --name shard-controller-demo
```

### Automated Cleanup
```bash
# Use the cleanup target
make demo-cleanup

# Or use the interactive cleaner
./scripts/demo-launcher.sh
# Choose option 7: Cleanup Resources
```

## 📚 Learning Path

### Beginner → Intermediate → Advanced

1. **Start Here**: Interactive Demo Launcher
2. **Core Understanding**: Comprehensive Demo
3. **Deep Dive**: Advanced Features Demo
4. **Performance**: Performance Testing Demo
5. **Exploration**: Interactive Exploration Mode

### Documentation References
- `README.md` - Project overview
- `kubernetes-分片控制器-架构文档.md` - Architecture analysis
- `docs/` - Detailed documentation
- `examples/` - Configuration examples

## 🤝 Contributing

### Adding New Demos
1. Create new script in `scripts/`
2. Follow naming convention: `*-demo.sh`
3. Add to `demo-launcher.sh` menu
4. Update this README
5. Add Makefile target

### Demo Script Template
```bash
#!/bin/bash
set -e

# Source demo configuration
source "$(dirname "$0")/demo-config.env"

# Your demo logic here

main() {
    echo "Demo Name"
    echo "========="
    # Demo implementation
}

main "$@"
```

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/k8s-shard-controller/shard-controller/issues)
- **Discussions**: [GitHub Discussions](https://github.com/k8s-shard-controller/shard-controller/discussions)
- **Documentation**: [Project Docs](../docs/)

---

**Happy Demoing! 🎉**