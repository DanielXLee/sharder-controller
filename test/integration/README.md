# Integration Test Suite

This directory contains comprehensive integration tests for the K8s Shard Controller system.

## Overview

The integration test suite validates the complete functionality of the shard controller system, including:

- Shard lifecycle management (creation, scaling, deletion)
- Health checking and failure detection
- Load balancing across different strategies
- Resource migration between shards
- Configuration management and dynamic updates
- Performance and stress testing
- End-to-end scenarios

## Test Structure

### Core Test Files

- `integration_test.go` - Main integration test suite with core functionality tests
- `load_balancing_test.go` - Load balancing strategy and rebalancing tests
- `performance_test.go` - Performance and stress tests
- `config_test.go` - Configuration management and CRD tests

### Test Infrastructure

- `setup.sh` - Environment setup script (Kind/envtest)
- `run_tests.sh` - Test runner with various options
- `README.md` - This documentation

## Running Tests

### Prerequisites

- Go 1.19+
- kubectl
- Kind (optional, for local cluster testing)
- Docker (if using Kind)

### Quick Start

```bash
# Run all integration tests with envtest
make test-integration

# Run with Kind cluster
make test-integration-kind

# Run specific test patterns
make test-load-balancing
make test-performance

# Run with custom options
./test/integration/run_tests.sh --kind --verbose --pattern "LoadBalancing"
```

### Test Environment Options

#### Using envtest (Default)
- Fastest option
- Uses controller-runtime's envtest
- Automatically downloads required binaries
- No external dependencies

```bash
make test-integration
```

#### Using Kind
- More realistic Kubernetes environment
- Tests actual CRD installation
- Slower but more comprehensive

```bash
make test-integration-kind
# or
USE_KIND=true ./test/integration/run_tests.sh
```

#### Using Existing Cluster
- Use your current kubectl context
- Requires CRDs to be pre-installed

```bash
USE_EXISTING_CLUSTER=true make test-integration
```

## Test Categories

### 1. Core Functionality Tests

#### Shard Lifecycle Management
- `TestShardCreationAndDeletion` - Basic shard CRUD operations
- `TestShardScaling` - Scale up and scale down operations
- `TestEndToEndScenario` - Complete workflow from creation to cleanup

#### Health Checking
- `TestHealthCheckingAndFailureDetection` - Health monitoring
- `TestFailureRecovery` - Failure detection and recovery
- `TestChaosScenarios` - Multiple failure scenarios

### 2. Load Balancing Tests

#### Strategy Testing
- `TestLoadBalancingStrategies` - All load balancing strategies
- `TestLoadRebalancing` - Load rebalancing scenarios
- `TestDynamicLoadBalancing` - Dynamic load changes
- `TestLoadBalancingWithFailures` - Load balancing during failures

#### Performance
- `TestLoadDistribution` - Load distribution metrics
- Resource assignment consistency
- Strategy switching

### 3. Resource Migration Tests

#### Migration Scenarios
- `TestResourceMigration` - Basic migration functionality
- Failure handling during migration
- Migration rollback scenarios
- Concurrent migrations

### 4. Configuration Tests

#### Configuration Management
- `TestConfigurationDynamicUpdate` - Hot reload of configuration
- `TestConfigurationManagement` - ConfigMap-based configuration
- `TestShardConfigCRD` - CRD-based configuration
- `TestConfigurationIntegrationWithComponents` - Config impact on components

### 5. Performance Tests

#### Scalability
- `TestShardCreationPerformance` - Shard creation speed
- `TestLoadBalancingPerformance` - Load balancing performance
- `TestHealthCheckingPerformance` - Health check performance
- `TestResourceMigrationPerformance` - Migration speed
- `TestScalingPerformance` - Scaling operation speed

#### Stress Testing
- `TestConcurrentOperations` - Concurrent operation handling
- High load scenarios
- Resource exhaustion testing

## Test Configuration

### Environment Variables

- `USE_KIND` - Use Kind cluster instead of envtest
- `USE_EXISTING_CLUSTER` - Use current kubectl context
- `KUBECONFIG` - Path to kubeconfig file
- `KUBEBUILDER_ASSETS` - Path to envtest binaries

### Test Timeouts

- Default test timeout: 30 minutes
- Individual test timeouts vary by complexity
- Performance tests may take longer

### Resource Requirements

- Memory: 2GB+ recommended
- CPU: 2+ cores recommended
- Disk: 1GB+ for test artifacts

## Test Data and Fixtures

### Test Shards
- Created with predictable names (`test-shard-1`, `perf-shard-1`, etc.)
- Various phases and health states
- Configurable load levels

### Test Resources
- Mock resources for migration testing
- Configurable resource counts
- Consistent naming patterns

### Test Configurations
- Multiple ConfigMap scenarios
- Valid and invalid configurations
- Default value testing

## Debugging Tests

### Verbose Output
```bash
./test/integration/run_tests.sh --verbose
```

### Running Specific Tests
```bash
# Run only load balancing tests
go test -v -run "LoadBalancing" ./test/integration/...

# Run specific test method
go test -v -run "TestShardCreationAndDeletion" ./test/integration/...
```

### Keeping Test Environment
```bash
./test/integration/run_tests.sh --no-cleanup
```

### Logs and Debugging
- Test logs are written to stdout
- Use `-v` flag for verbose test output
- Kind cluster logs: `kubectl logs -n kube-system`

## Coverage Reports

Tests generate coverage reports when run without specific patterns:

```bash
make test-integration
# Generates coverage.html

make coverage
# Generates detailed coverage report
```

## Continuous Integration

### GitHub Actions Integration
```yaml
- name: Run Integration Tests
  run: make test-integration

- name: Run Integration Tests with Kind
  run: make test-integration-kind
```

### Test Artifacts
- Coverage reports
- Test results (JUnit XML format available)
- Performance metrics

## Troubleshooting

### Common Issues

#### Kind Cluster Issues
```bash
# Reset Kind cluster
kind delete cluster --name shard-controller-test
make test-integration-kind
```

#### Permission Issues
```bash
# Ensure scripts are executable
chmod +x test/integration/*.sh
```

#### Resource Conflicts
```bash
# Clean up test resources
kubectl delete shardconfigs --all
kubectl delete shardinstances --all
```

#### Timeout Issues
```bash
# Increase timeout
TIMEOUT=60m ./test/integration/run_tests.sh
```

### Getting Help

1. Check test logs for specific error messages
2. Verify prerequisites are installed
3. Ensure sufficient system resources
4. Check Kubernetes cluster connectivity

## Contributing

### Adding New Tests

1. Follow existing test patterns
2. Use the test suite structure
3. Add appropriate cleanup
4. Document test purpose and scenarios
5. Update this README if needed

### Test Guidelines

- Tests should be deterministic
- Clean up resources after each test
- Use meaningful test names
- Add appropriate assertions
- Handle timeouts gracefully
- Test both success and failure scenarios

### Performance Considerations

- Avoid unnecessary resource creation
- Use appropriate timeouts
- Clean up promptly
- Consider test execution time
- Parallelize where possible