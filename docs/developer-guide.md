# Developer Guide

## Overview

This guide covers how to develop, build, test, and contribute to the Kubernetes Shard Controller project.

## Development Environment Setup

### Prerequisites

- **Go**: 1.24 or later
- **Docker**: For building container images
- **kubectl**: For interacting with Kubernetes
- **kind**: For local Kubernetes clusters (recommended for development)
- **Git**: For version control

### Clone and Setup

```bash
# Clone the repository
git clone https://github.com/k8s-shard-controller/shard-controller.git
cd shard-controller

# Install dependencies
go mod download

# Verify setup
make test
```

### Development Tools

Install recommended development tools:

```bash
# Install golangci-lint for linting
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2

# Install goimports for import formatting
go install golang.org/x/tools/cmd/goimports@latest

# Install controller-gen for code generation
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
```

## Project Structure

```
shard-controller/
├── cmd/                    # Main applications
│   ├── manager/           # Manager component
│   ├── worker/            # Worker component
│   └── performance-test/  # Performance testing tool
├── pkg/                   # Library code
│   ├── apis/             # API definitions
│   │   └── shard/v1/     # Shard API v1
│   ├── config/           # Configuration management
│   ├── controllers/      # Controller implementations
│   ├── worker/           # Worker implementations
│   └── utils/            # Utility functions
├── manifests/            # Kubernetes manifests
│   ├── crds/            # Custom Resource Definitions
│   ├── deployment/      # Deployment manifests
│   └── monitoring/      # Monitoring configurations
├── docs/                # Documentation
├── examples/            # Example configurations
├── test/               # Test files
│   ├── integration/    # Integration tests
│   └── performance/    # Performance tests
├── scripts/            # Build and utility scripts
├── helm/              # Helm charts
└── hack/              # Development scripts
```

## Building the Project

### Build Binaries

```bash
# Build all binaries
make build

# Build specific components
make manager
make worker

# Build with race detection (for development)
go build -race -o bin/manager ./cmd/manager
go build -race -o bin/worker ./cmd/worker
```

### Build Docker Images

```bash
# Build all images
make docker-build

# Build specific images
docker build -t shard-controller/manager:dev -f Dockerfile.manager .
docker build -t shard-controller/worker:dev -f Dockerfile.worker .

# Build for different architectures
docker buildx build --platform linux/amd64,linux/arm64 -t shard-controller/manager:dev .
```

## Running Locally

### Using Kind

```bash
# Create kind cluster
kind create cluster --name shard-dev

# Load images into kind
kind load docker-image shard-controller/manager:dev --name shard-dev
kind load docker-image shard-controller/worker:dev --name shard-dev

# Deploy to kind cluster
kubectl apply -f manifests/crds/
kubectl apply -f manifests/namespace.yaml
kubectl apply -f manifests/rbac.yaml
kubectl apply -f manifests/configmap.yaml
kubectl apply -f manifests/services.yaml
kubectl apply -f manifests/deployment/
```

### Running Components Directly

```bash
# Run manager locally (requires kubeconfig)
make run

# Run with custom configuration
./bin/manager --kubeconfig ~/.kube/config --log-level debug

# Run worker locally
./bin/worker --kubeconfig ~/.kube/config --shard-id local-worker-1
```

## Testing

### Unit Tests

```bash
# Run all unit tests
make test

# Run tests with coverage
make coverage

# Run specific package tests
go test -v ./pkg/controllers/...

# Run tests with race detection
go test -race ./...
```

### Integration Tests

```bash
# Run integration tests (requires cluster)
make test-integration

# Run with kind cluster
make test-integration-kind

# Run specific integration tests
go test -v ./test/integration/ -run TestShardConfigController
```

### Performance Tests

```bash
# Run performance tests
make test-performance

# Run specific performance tests
go test -v ./test/performance/ -run TestScalingPerformance

# Run with custom parameters
go test -v ./test/performance/ -args -shards=100 -duration=5m
```

### End-to-End Tests

```bash
# Run complete e2e test suite
./test/integration/run_final_tests.sh

# Run specific e2e scenarios
go test -v ./test/integration/ -run TestTask15
```

## Code Generation

The project uses code generation for various components:

```bash
# Generate all code
make generate

# Generate CRD manifests
controller-gen crd:trivialVersions=true paths="./pkg/apis/..." output:crd:artifacts:config=manifests/crds

# Generate RBAC manifests
controller-gen rbac:roleName=shard-manager paths="./pkg/controllers/..." output:rbac:artifacts:config=manifests

# Generate deepcopy methods
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/..."
```

## Development Workflow

### Making Changes

1. **Create Feature Branch**:
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Make Changes**: Implement your feature or fix

3. **Add Tests**: Ensure your changes are covered by tests

4. **Run Tests**:
   ```bash
   make test
   make test-integration
   ```

5. **Format Code**:
   ```bash
   make fmt
   make lint
   ```

6. **Commit Changes**:
   ```bash
   git add .
   git commit -m "Add new feature: description"
   ```

### Code Style

Follow these coding standards:

- Use `gofmt` for formatting
- Follow Go naming conventions
- Add comments for exported functions and types
- Use meaningful variable and function names
- Keep functions small and focused

### Commit Messages

Use conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Examples:
```
feat(controller): add support for custom metrics
fix(worker): resolve memory leak in heartbeat manager
docs(api): update ShardConfig specification
```

## Debugging

### Local Debugging

```bash
# Run with debug logging
./bin/manager --log-level debug

# Use delve debugger
dlv exec ./bin/manager -- --kubeconfig ~/.kube/config

# Debug tests
dlv test ./pkg/controllers/
```

### Cluster Debugging

```bash
# Check pod logs
kubectl logs -n shard-system deployment/shard-manager -f

# Port forward for debugging
kubectl port-forward -n shard-system svc/shard-manager-metrics 8080:8080

# Access debug endpoints
curl http://localhost:8080/debug/pprof/
```

### Common Issues

#### Build Issues
- Ensure Go version is 1.24+
- Run `go mod tidy` to clean dependencies
- Check for missing build tools

#### Test Failures
- Ensure cluster is accessible
- Check resource permissions
- Verify CRDs are installed

#### Runtime Issues
- Check RBAC permissions
- Verify network connectivity
- Review resource limits

## Contributing

### Before Contributing

1. Read the [Code of Conduct](CODE_OF_CONDUCT.md)
2. Check existing issues and PRs
3. Discuss major changes in issues first

### Pull Request Process

1. **Fork the Repository**
2. **Create Feature Branch**
3. **Make Changes with Tests**
4. **Update Documentation**
5. **Submit Pull Request**

### PR Requirements

- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated
- [ ] Commit messages follow convention
- [ ] PR description explains changes

### Review Process

1. Automated checks must pass
2. At least one maintainer review required
3. Address review feedback
4. Maintainer will merge when approved

## Architecture Deep Dive

### Controller Pattern

The project follows the Kubernetes controller pattern:

```go
// Controller reconciliation loop
func (r *ShardConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the resource
    var shardConfig shardv1.ShardConfig
    if err := r.Get(ctx, req.NamespacedName, &shardConfig); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 2. Reconcile desired state
    return r.reconcileShardConfig(ctx, &shardConfig)
}
```

### Manager Component

Key responsibilities:
- **Shard Lifecycle Management**: Create/update/delete shards
- **Load Balancing**: Distribute resources across shards
- **Health Monitoring**: Monitor shard health and trigger recovery
- **Scaling Decisions**: Make scaling decisions based on metrics

### Worker Component

Key responsibilities:
- **Resource Processing**: Handle assigned resources
- **Heartbeat Management**: Report health to manager
- **Migration Support**: Support resource migration
- **Metrics Reporting**: Report load and performance metrics

### Custom Resources

#### ShardConfig Controller

```go
type ShardConfigReconciler struct {
    client.Client
    Scheme *runtime.Scheme
    
    // Dependencies
    LoadBalancer  *controllers.LoadBalancer
    HealthChecker *controllers.HealthChecker
    MetricsCollector *controllers.MetricsCollector
}
```

#### ShardInstance Controller

```go
type ShardInstanceReconciler struct {
    client.Client
    Scheme *runtime.Scheme
    
    // Worker management
    WorkerManager *worker.Manager
}
```

## Performance Considerations

### Optimization Guidelines

1. **Efficient Resource Usage**:
   - Use resource pools where possible
   - Implement proper cleanup
   - Monitor memory usage

2. **Concurrent Processing**:
   - Use worker pools for parallel processing
   - Implement proper synchronization
   - Avoid blocking operations in hot paths

3. **Network Optimization**:
   - Batch API calls when possible
   - Use informers for watching resources
   - Implement proper retry logic

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.

# Memory profiling
go test -memprofile=mem.prof -bench=.

# Analyze profiles
go tool pprof cpu.prof
go tool pprof mem.prof
```

## Security Considerations

### RBAC

Implement least-privilege RBAC:

```yaml
# Manager needs broader permissions
rules:
- apiGroups: ["shard.io"]
  resources: ["shardconfigs", "shardinstances"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Worker needs limited permissions
rules:
- apiGroups: ["shard.io"]
  resources: ["shardinstances"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["shard.io"]
  resources: ["shardinstances/status"]
  verbs: ["update", "patch"]
```

### Security Context

Use appropriate security contexts:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
```

### Input Validation

Always validate inputs:

```go
func validateShardConfig(config *shardv1.ShardConfig) error {
    if config.Spec.MinShards < 1 {
        return fmt.Errorf("minShards must be >= 1")
    }
    if config.Spec.MaxShards <= config.Spec.MinShards {
        return fmt.Errorf("maxShards must be > minShards")
    }
    // Additional validation...
    return nil
}
```

## Release Process

### Versioning

Follow semantic versioning (SemVer):
- **Major**: Breaking changes
- **Minor**: New features (backward compatible)
- **Patch**: Bug fixes (backward compatible)

### Release Steps

1. **Update Version**: Update version in relevant files
2. **Update Changelog**: Document changes
3. **Create Release Branch**: `release/v1.2.3`
4. **Run Tests**: Ensure all tests pass
5. **Build Artifacts**: Build binaries and images
6. **Create Tag**: `git tag v1.2.3`
7. **Push Release**: Push tag and create GitHub release

### Automated Release

The project uses GitHub Actions for automated releases:

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags:
      - 'v*'
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - name: Build and Release
      run: make release
```

## Monitoring and Observability

### Metrics

Implement comprehensive metrics:

```go
var (
    shardsTotal = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "shard_controller_shards_total",
            Help: "Total number of shards",
        },
        []string{"namespace", "shardconfig"},
    )
)
```

### Logging

Use structured logging:

```go
logger := log.FromContext(ctx).WithValues(
    "shardconfig", shardConfig.Name,
    "namespace", shardConfig.Namespace,
)
logger.Info("Reconciling ShardConfig")
```

### Tracing

Implement distributed tracing for complex operations:

```go
func (r *ShardConfigReconciler) reconcileShardConfig(ctx context.Context, config *shardv1.ShardConfig) error {
    span, ctx := opentracing.StartSpanFromContext(ctx, "reconcile-shardconfig")
    defer span.Finish()
    
    // Implementation...
}
```

## Documentation

### Code Documentation

- Document all exported functions and types
- Include examples in documentation
- Keep documentation up to date with code changes

### API Documentation

- Update API reference for any API changes
- Include examples for new features
- Document breaking changes clearly

### User Documentation

- Update user guide for new features
- Include troubleshooting information
- Provide migration guides for breaking changes

## Community

### Getting Help

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and general discussion
- **Slack**: Real-time chat and support

### Contributing to Community

- Answer questions in discussions
- Review pull requests
- Improve documentation
- Share usage examples and best practices

---

This developer guide should help you get started with contributing to the Kubernetes Shard Controller project. For specific questions, please reach out through our community channels.