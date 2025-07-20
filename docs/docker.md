# Docker Guide

## Overview

This guide covers how to build, run, and deploy the Kubernetes Shard Controller using Docker containers.

## Docker Images

The project provides two main Docker images:

- **Manager Image**: `shard-controller/manager` - The control plane component
- **Worker Image**: `shard-controller/worker` - The data plane component

## Building Images

### Using Make

```bash
# Build all images
make docker-build

# Build specific components
make docker-build-manager
make docker-build-worker

# Build with custom registry and tag
REGISTRY=myregistry.com/shard-controller TAG=v1.0.0 make docker-build
```

### Using Build Script

```bash
# Build all images
./scripts/build-images.sh

# Build with custom options
./scripts/build-images.sh --registry myregistry.com --tag v1.0.0 --push

# Build only manager
./scripts/build-images.sh --manager-only

# Build for multiple platforms
./scripts/build-images.sh --platform linux/amd64,linux/arm64
```

### Manual Docker Build

```bash
# Build manager image
docker build -f Dockerfile.manager -t shard-controller/manager:latest .

# Build worker image
docker build -f Dockerfile.worker -t shard-controller/worker:latest .

# Build with build arguments
docker build \
  --build-arg VERSION=v1.0.0 \
  --build-arg COMMIT=$(git rev-parse HEAD) \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  -f Dockerfile.manager \
  -t shard-controller/manager:v1.0.0 \
  .
```

## Running with Docker

### Individual Containers

```bash
# Run manager (requires kubeconfig)
docker run -d \
  --name shard-manager \
  -p 8080:8080 -p 8081:8081 \
  -v ~/.kube:/home/nonroot/.kube:ro \
  shard-controller/manager:latest

# Run worker (requires kubeconfig)
docker run -d \
  --name shard-worker \
  -p 8082:8080 -p 8083:8081 \
  -v ~/.kube:/home/nonroot/.kube:ro \
  -e SHARD_ID=worker-1 \
  shard-controller/worker:latest
```

### Using Docker Compose

```bash
# Start all services
docker-compose up -d

# Start with monitoring stack
docker-compose --profile monitoring up -d

# View logs
docker-compose logs -f shard-manager
docker-compose logs -f shard-worker

# Stop services
docker-compose down
```

## Configuration

### Environment Variables

#### Manager Container

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | info |
| `METRICS_ADDR` | Metrics server address | 0.0.0.0:8080 |
| `HEALTH_ADDR` | Health server address | 0.0.0.0:8081 |
| `KUBECONFIG` | Path to kubeconfig file | /home/nonroot/.kube/config |

#### Worker Container

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | info |
| `METRICS_ADDR` | Metrics server address | 0.0.0.0:8080 |
| `HEALTH_ADDR` | Health server address | 0.0.0.0:8081 |
| `SHARD_ID` | Unique shard identifier | Pod name |
| `KUBECONFIG` | Path to kubeconfig file | /home/nonroot/.kube/config |

### Volume Mounts

```bash
# Mount kubeconfig
-v ~/.kube:/home/nonroot/.kube:ro

# Mount custom config
-v /path/to/config.yaml:/etc/config/config.yaml:ro

# Mount logs directory
-v /var/log/shard-controller:/var/log:rw
```

## Health Checks

### Built-in Health Checks

Both images include health check endpoints:

```bash
# Check manager health
curl http://localhost:8081/healthz

# Check worker health
curl http://localhost:8083/healthz

# Check readiness
curl http://localhost:8081/readyz
```

### Docker Health Checks

```bash
# Run with health check
docker run -d \
  --name shard-manager \
  --health-cmd="wget --quiet --tries=1 --spider http://localhost:8081/healthz || exit 1" \
  --health-interval=30s \
  --health-timeout=10s \
  --health-retries=3 \
  shard-controller/manager:latest
```

## Monitoring

### Metrics Collection

```bash
# Access metrics
curl http://localhost:8080/metrics

# Prometheus scraping
docker run -d \
  --name prometheus \
  -p 9090:9090 \
  -v ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro \
  prom/prometheus:latest
```

### Log Collection

```bash
# View container logs
docker logs shard-manager
docker logs shard-worker

# Follow logs
docker logs -f shard-manager

# Export logs
docker logs shard-manager > manager.log 2>&1
```

## Security

### Image Security

The Docker images follow security best practices:

- **Distroless base image**: Minimal attack surface
- **Non-root user**: Runs as user 65534
- **Read-only filesystem**: Where possible
- **No shell**: Distroless images don't include shell
- **Minimal dependencies**: Only necessary components

### Security Scanning

```bash
# Scan with Trivy
trivy image shard-controller/manager:latest
trivy image shard-controller/worker:latest

# Scan with Docker Scout
docker scout cves shard-controller/manager:latest
docker scout cves shard-controller/worker:latest
```

### Runtime Security

```bash
# Run with security options
docker run -d \
  --name shard-manager \
  --read-only \
  --tmpfs /tmp \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  shard-controller/manager:latest
```

## Deployment to Kubernetes

### Using Pre-built Images

```bash
# Update deployment to use your images
kubectl set image deployment/shard-manager \
  manager=myregistry.com/shard-controller/manager:v1.0.0 \
  -n shard-system

kubectl set image deployment/shard-worker \
  worker=myregistry.com/shard-controller/worker:v1.0.0 \
  -n shard-system
```

### Using Kind

```bash
# Load images into kind cluster
make kind-load

# Or manually
kind load docker-image shard-controller/manager:latest --name my-cluster
kind load docker-image shard-controller/worker:latest --name my-cluster
```

### Using Helm

```bash
# Deploy with custom images
helm install shard-controller ./helm/shard-controller \
  --set manager.image.repository=myregistry.com/shard-controller/manager \
  --set manager.image.tag=v1.0.0 \
  --set worker.image.repository=myregistry.com/shard-controller/worker \
  --set worker.image.tag=v1.0.0
```

## Multi-Architecture Builds

### Building for Multiple Platforms

```bash
# Create buildx builder
docker buildx create --name multiarch --use

# Build for multiple platforms
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile.manager \
  -t shard-controller/manager:latest \
  --push \
  .
```

### Using Build Script

```bash
# Build for multiple architectures
./scripts/build-images.sh --platform linux/amd64,linux/arm64 --push
```

## Troubleshooting

### Common Issues

#### Image Build Failures

```bash
# Check Docker daemon
docker info

# Clean build cache
docker builder prune

# Build with verbose output
docker build --progress=plain -f Dockerfile.manager .
```

#### Container Runtime Issues

```bash
# Check container logs
docker logs shard-manager

# Inspect container
docker inspect shard-manager

# Execute into container (if shell available)
docker exec -it shard-manager /bin/sh
```

#### Permission Issues

```bash
# Check file permissions
ls -la ~/.kube/config

# Fix kubeconfig permissions
chmod 600 ~/.kube/config

# Run with correct user
docker run --user $(id -u):$(id -g) ...
```

### Debugging

```bash
# Run in debug mode
docker run -it --rm \
  -e LOG_LEVEL=debug \
  shard-controller/manager:latest

# Override entrypoint for debugging
docker run -it --rm \
  --entrypoint /bin/sh \
  shard-controller/manager:latest
```

## Best Practices

### Image Management

1. **Use specific tags**: Avoid `latest` in production
2. **Multi-stage builds**: Keep images small
3. **Security scanning**: Regularly scan for vulnerabilities
4. **Layer caching**: Optimize Dockerfile for better caching

### Container Configuration

1. **Resource limits**: Set appropriate CPU/memory limits
2. **Health checks**: Always configure health checks
3. **Logging**: Use structured logging
4. **Secrets**: Use proper secret management

### Production Deployment

1. **Image registry**: Use private registry for production
2. **Image signing**: Sign images for security
3. **Monitoring**: Set up comprehensive monitoring
4. **Backup**: Backup container configurations

## CI/CD Integration

### GitHub Actions

The project includes GitHub Actions workflow for automated builds:

```yaml
# .github/workflows/docker.yml
- Builds images on push/PR
- Pushes to GitHub Container Registry
- Scans for vulnerabilities
- Supports multi-architecture builds
```

### Custom CI/CD

```bash
# Example CI/CD pipeline
./scripts/build-images.sh --registry $REGISTRY --tag $VERSION --push
docker run --rm $REGISTRY/manager:$VERSION --version
kubectl set image deployment/shard-manager manager=$REGISTRY/manager:$VERSION
```

## Performance Optimization

### Build Optimization

```bash
# Use build cache
export DOCKER_BUILDKIT=1

# Parallel builds
make -j$(nproc) docker-build

# Multi-stage caching
docker build --cache-from shard-controller/manager:cache ...
```

### Runtime Optimization

```bash
# Set resource limits
docker run -m 512m --cpus="0.5" shard-controller/manager:latest

# Use tmpfs for temporary files
docker run --tmpfs /tmp:rw,noexec,nosuid,size=100m ...
```

This guide should help you effectively use Docker with the Shard Controller project. For more information, see the [Developer Guide](developer-guide.md) and [Deployment Guide](installation.md).