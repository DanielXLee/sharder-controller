# Kubernetes Shard Controller

[![Go Report Card](https://goreportcard.com/badge/github.com/k8s-shard-controller)](https://goreportcard.com/report/github.com/k8s-shard-controller)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.19+-blue.svg)](https://kubernetes.io/)
[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org/)

A production-ready Kubernetes controller that provides dynamic sharding capabilities for distributed workloads, enabling automatic scaling, load balancing, and resource management across multiple shards.

> **Project Status**: This project is actively developed and includes comprehensive testing, documentation, and examples. The controller has been validated through extensive integration tests and performance benchmarks.

## 🚀 Features

- **Dynamic Sharding**: Automatically create, scale, and manage shards based on workload demands
- **Intelligent Load Balancing**: Multiple strategies including consistent hashing, round-robin, and least-loaded
- **Auto-scaling**: Configurable scaling policies with customizable thresholds and cooldown periods
- **Health Monitoring**: Comprehensive health checks with automatic failure detection and recovery
- **Resource Migration**: Seamless resource migration between shards during scaling operations
- **High Availability**: Leader election support for multi-replica deployments
- **Observability**: Rich metrics, logging, and monitoring integration with Prometheus and Grafana
- **Production Ready**: Comprehensive testing, error handling, and recovery mechanisms

## ⚡ Quick Start

### Prerequisites

- Kubernetes cluster (v1.19+) or [kind](https://kind.sigs.k8s.io/) for local development
- kubectl configured to access your cluster
- Go 1.24+ (for building from source)

### Option 1: Quick Demo Script

Run our automated demo script to see the controller in action:

```bash
# Clone the repository
git clone https://github.com/k8s-shard-controller/shard-controller.git
cd shard-controller

# For local cluster demo (uses existing cluster)
./scripts/local-demo.sh

# For complete demo with kind cluster (creates new cluster)
./scripts/quick-demo.sh
```

### Option 2: Manual Setup

#### Step 1: Build the Controller

```bash
# Clone the repository
git clone https://github.com/k8s-shard-controller/shard-controller.git
cd shard-controller

# Build the binaries
make build
```

#### Step 2: Install CRDs

```bash
# Install Custom Resource Definitions
kubectl apply -f manifests/crds/
```

#### Step 3: Create Namespace and RBAC

```bash
# Create namespace
kubectl apply -f manifests/namespace.yaml

# Apply RBAC permissions
kubectl apply -f manifests/rbac.yaml
```

#### Step 4: Deploy the Controller

```bash
# Deploy configuration
kubectl apply -f manifests/configmap.yaml

# Deploy services
kubectl apply -f manifests/services.yaml

# Deploy the controller components
kubectl apply -f manifests/deployment/
```

#### Step 5: Verify Installation

```bash
# Check if pods are running
kubectl get pods -n shard-system

# Verify CRDs are installed
kubectl get crd | grep shard.io

# Check controller logs
kubectl logs -n shard-system deployment/shard-manager
```

#### Step 6: Create Your First ShardConfig

```bash
# Apply the basic example
kubectl apply -f examples/basic-shardconfig.yaml

# Check the created resources
kubectl get shardconfigs
kubectl get shardinstances
```

### Verification

```bash
# Quick verification script
./scripts/verify-installation.sh

# Or check manually
kubectl get shardconfigs,shardinstances -o wide
kubectl get pods -n shard-system
kubectl logs -n shard-system deployment/shard-manager
```

## 🏗️ Architecture

The Shard Controller consists of two main components:

- **Manager**: Orchestrates shard lifecycle, scaling decisions, and load balancing
- **Worker**: Handles resource processing within each shard

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Manager       │    │   Worker 1      │    │   Worker 2      │
│   - Scaling     │◄──►│   - Processing  │    │   - Processing  │
│   - Balancing   │    │   - Heartbeat   │    │   - Heartbeat   │
│   - Monitoring  │    │   - Migration   │    │   - Migration   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

For detailed architecture information, see [Architecture Guide](docs/architecture.md).

## 📚 Documentation

| Guide | Description |
|-------|-------------|
| [Installation Guide](docs/installation.md) | Detailed installation instructions |
| [User Guide](docs/user-guide.md) | How to use the controller |
| [Configuration Guide](docs/configuration.md) | Complete configuration reference |
| [API Reference](docs/api-reference.md) | Custom Resource API documentation |
| [Architecture Guide](docs/architecture.md) | System architecture and design |
| [Developer Guide](docs/developer-guide.md) | Development and contribution guide |
| [Troubleshooting](docs/troubleshooting.md) | Common issues and solutions |
| [Contributing](docs/contributing.md) | How to contribute to the project |
| [Examples](examples/) | Configuration examples for different use cases |
| [Docker Guide](docs/docker.md) | Docker build, run, and deployment guide |

## 🛠️ Development

```bash
# Build the project
make build

# Build Docker images
make docker-build

# Run tests
make test

# Run locally (requires kubeconfig)
make run

# Validate quick start guide
./scripts/validate-quickstart.sh
```

See the [Developer Guide](docs/developer-guide.md) for detailed development instructions.

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](docs/contributing.md) for details.

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Documentation**: [docs/](docs/)
- **Issues**: [GitHub Issues](https://github.com/k8s-shard-controller/shard-controller/issues)
- **Discussions**: [GitHub Discussions](https://github.com/k8s-shard-controller/shard-controller/discussions)

## 📈 Project Status

- ✅ **Core Features**: Complete implementation with comprehensive testing
- ✅ **Documentation**: Full documentation suite with examples
- ✅ **Testing**: Unit, integration, and performance tests
- ✅ **Security**: Security scanning and compliance validation
- ✅ **Production Ready**: Validated through extensive testing framework

## 🗺️ Roadmap

- [ ] Multi-cluster sharding support
- [ ] Advanced scheduling policies
- [ ] Integration with service mesh
- [ ] Enhanced security features
- [ ] Performance optimizations
- [ ] Additional load balancing strategies

See [CHANGELOG.md](CHANGELOG.md) for detailed release information.

---

**Made with ❤️ by the Shard Controller community**