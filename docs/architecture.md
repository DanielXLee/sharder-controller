# Architecture Guide

## Overview

The Kubernetes Shard Controller is designed as a distributed system that provides dynamic sharding capabilities for Kubernetes workloads. This document describes the system architecture, components, and design decisions.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                          │
│                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐                   │
│  │   Manager       │    │   Worker Pool   │                   │
│  │   Component     │    │                 │                   │
│  │                 │    │  ┌───────────┐  │                   │
│  │ ┌─────────────┐ │    │  │ Worker 1  │  │                   │
│  │ │ Shard       │ │◄──►│  │           │  │                   │
│  │ │ Orchestrator│ │    │  └───────────┘  │                   │
│  │ └─────────────┘ │    │                 │                   │
│  │                 │    │  ┌───────────┐  │                   │
│  │ ┌─────────────┐ │    │  │ Worker 2  │  │                   │
│  │ │ Load        │ │◄──►│  │           │  │                   │
│  │ │ Balancer    │ │    │  └───────────┘  │                   │
│  │ └─────────────┘ │    │                 │                   │
│  │                 │    │  ┌───────────┐  │                   │
│  │ ┌─────────────┐ │    │  │ Worker N  │  │                   │
│  │ │ Health      │ │◄──►│  │           │  │                   │
│  │ │ Monitor     │ │    │  └───────────┘  │                   │
│  │ └─────────────┘ │    │                 │                   │
│  └─────────────────┘    └─────────────────┘                   │
│           │                       │                           │
│           └───────────────────────┼───────────────────────────┘
│                                   │
│                      ┌─────────────────┐
│                      │   Kubernetes    │
│                      │   API Server    │
│                      │                 │
│                      │ ┌─────────────┐ │
│                      │ │ ShardConfig │ │
│                      │ │     CRD     │ │
│                      │ └─────────────┘ │
│                      │                 │
│                      │ ┌─────────────┐ │
│                      │ │ShardInstance│ │
│                      │ │     CRD     │ │
│                      │ └─────────────┘ │
│                      └─────────────────┘
```

## Core Components

### Manager Component

The Manager is the central control plane component responsible for:

#### Shard Orchestrator
- **Lifecycle Management**: Creates, updates, and deletes shard instances
- **Scaling Decisions**: Monitors load and makes scaling decisions
- **Configuration Management**: Processes ShardConfig resources
- **State Reconciliation**: Ensures desired state matches actual state

#### Load Balancer
- **Resource Distribution**: Distributes workload across available shards
- **Strategy Implementation**: Supports multiple load balancing strategies
- **Rebalancing**: Triggers resource migration when needed
- **Hash Ring Management**: Maintains consistent hash ring for resource assignment

#### Health Monitor
- **Shard Health Tracking**: Monitors health of all worker shards
- **Failure Detection**: Detects and responds to shard failures
- **Recovery Coordination**: Orchestrates recovery actions
- **Metrics Collection**: Collects and aggregates health metrics

### Worker Component

Workers are the data plane components that handle actual workload processing:

#### Resource Processor
- **Workload Execution**: Processes assigned resources
- **Batch Processing**: Handles resources in configurable batches
- **Concurrency Control**: Manages concurrent processing within limits
- **Error Handling**: Implements retry and error recovery logic

#### Heartbeat Manager
- **Health Reporting**: Sends periodic health updates to manager
- **Load Reporting**: Reports current load and capacity metrics
- **Status Updates**: Communicates processing status and errors
- **Connection Management**: Maintains connection with manager

#### Migration Handler
- **Resource Migration**: Handles incoming and outgoing resource migrations
- **State Transfer**: Transfers processing state during migrations
- **Graceful Handoff**: Ensures no data loss during migrations
- **Rollback Support**: Supports migration rollback on failures

## Custom Resources

### ShardConfig

Defines the desired sharding configuration:

```yaml
apiVersion: shard.io/v1
kind: ShardConfig
metadata:
  name: example-config
spec:
  minShards: 2
  maxShards: 10
  scaleUpThreshold: 0.8
  scaleDownThreshold: 0.3
  loadBalanceStrategy: "consistent-hash"
  healthCheckInterval: "30s"
  resources:
    requests:
      cpu: "200m"
      memory: "256Mi"
    limits:
      cpu: "500m"
      memory: "512Mi"
```

### ShardInstance

Represents an individual shard instance:

```yaml
apiVersion: shard.io/v1
kind: ShardInstance
metadata:
  name: shard-001
spec:
  shardId: "shard-001"
  hashRange:
    start: 0
    end: 1073741823
status:
  phase: "Running"
  load: 0.65
  lastHeartbeat: "2024-01-15T10:30:00Z"
  assignedResources: ["resource-001", "resource-002"]
```

## Design Principles

### Scalability
- **Horizontal Scaling**: Workers can be scaled independently
- **Efficient Resource Usage**: Optimized for high-throughput scenarios
- **Load Distribution**: Even distribution of work across shards

### Reliability
- **Fault Tolerance**: Handles individual shard failures gracefully
- **Data Consistency**: Ensures consistent resource assignment
- **Recovery Mechanisms**: Automatic recovery from failures

### Observability
- **Comprehensive Metrics**: Rich metrics for monitoring and alerting
- **Structured Logging**: Consistent logging across all components
- **Health Endpoints**: Standard health and readiness checks

### Security
- **RBAC Integration**: Follows Kubernetes RBAC best practices
- **Least Privilege**: Minimal required permissions
- **Secure Communication**: TLS for inter-component communication

## Load Balancing Strategies

### Consistent Hashing
- **Use Case**: Stable resource assignment with minimal reshuffling
- **Algorithm**: Uses consistent hash ring for resource distribution
- **Benefits**: Minimizes resource migration during scaling

### Round Robin
- **Use Case**: Even distribution of new resources
- **Algorithm**: Distributes resources sequentially across shards
- **Benefits**: Simple and predictable distribution

### Least Loaded
- **Use Case**: Dynamic load balancing based on current capacity
- **Algorithm**: Assigns resources to shard with lowest current load
- **Benefits**: Optimal resource utilization

## Scaling Behavior

### Scale Up Triggers
- Load exceeds `scaleUpThreshold` for sustained period
- Queue depth exceeds configured limits
- Custom metrics indicate need for more capacity

### Scale Down Triggers
- Load falls below `scaleDownThreshold` for cooldown period
- Excess capacity detected across multiple shards
- Cost optimization policies trigger consolidation

### Migration Process
1. **Planning**: Determine which resources to migrate
2. **Preparation**: Prepare target shard for new resources
3. **Transfer**: Move resources with state preservation
4. **Verification**: Verify successful migration
5. **Cleanup**: Remove resources from source shard

## Monitoring and Metrics

### Key Metrics
- `shard_controller_shards_total`: Total number of shards
- `shard_controller_load_average`: Average load across shards
- `shard_controller_scaling_operations_total`: Scaling operations count
- `shard_controller_migration_duration_seconds`: Migration timing

### Health Checks
- **Liveness**: Component is running and responsive
- **Readiness**: Component is ready to handle requests
- **Custom**: Application-specific health indicators

## Configuration Management

### Static Configuration
- Component-level settings (ports, timeouts, etc.)
- Security settings (TLS, authentication)
- Resource limits and requests

### Dynamic Configuration
- Scaling thresholds and policies
- Load balancing strategies
- Health check intervals

## Security Considerations

### Authentication and Authorization
- Service account-based authentication
- RBAC policies for fine-grained access control
- Network policies for traffic isolation

### Data Protection
- Encryption in transit for sensitive data
- Secure storage of configuration secrets
- Audit logging for security events

## Performance Characteristics

### Throughput
- Manager: Handles 100+ shard operations per second
- Worker: Processes 1000+ resources per second per shard
- Migration: Completes within 30 seconds for typical workloads

### Latency
- Health checks: < 100ms response time
- Scaling decisions: < 5 seconds from trigger to action
- Resource assignment: < 10ms for consistent hashing

### Resource Usage
- Manager: 100-500m CPU, 128-512Mi memory
- Worker: 200m-2 CPU, 256Mi-2Gi memory (scales with workload)
- Storage: Minimal persistent storage requirements

## Future Enhancements

### Planned Features
- Multi-cluster sharding support
- Advanced scheduling policies
- Integration with service mesh
- Enhanced security features

### Performance Optimizations
- Improved migration algorithms
- Better load prediction
- Optimized resource allocation
- Enhanced monitoring capabilities