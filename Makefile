# Go parameters
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_CLEAN=$(GO_CMD) clean
GO_TEST=$(GO_CMD) test
GO_FMT=$(GO_CMD) fmt
GO_VET=$(GO_CMD) vet
GO_RUN=$(GO_CMD) run

# Project variables
DIST_DIR=dist
BINARY_DIR=bin
MANAGER_BINARY=manager
WORKER_BINARY=worker
MANAGER_MAIN=./cmd/manager/main.go
WORKER_MAIN=./cmd/worker/main.go

.PHONY: all build manager worker run test fmt vet generate install-crds uninstall-crds clean

all: build

# Build binaries
build: manager worker

manager:
	@echo "Building manager..."
	@$(GO_BUILD) -o $(DIST_DIR)/$(MANAGER_BINARY) $(MANAGER_MAIN)

worker:
	@echo "Building worker..."
	@$(GO_BUILD) -o $(DIST_DIR)/$(WORKER_BINARY) $(WORKER_MAIN)

# Run the manager
run:
	@echo "Running manager..."
	@$(GO_RUN) $(MANAGER_MAIN)

# Run tests
test:
	@echo "Running tests..."
	@$(GO_TEST) -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	@$(GO_FMT) ./...

# Vet code
vet:
	@echo "Vetting code..."
	@$(GO_VET) ./...

# Generate code
generate:
	@echo "Generating code..."
	@./$(BINARY_DIR)/generate-code.sh

# CRD management
install-crds:
	@echo "Installing CRDs..."
	@./$(BINARY_DIR)/install-crds.sh

uninstall-crds:
	@echo "Uninstalling CRDs..."
	@./$(BINARY_DIR)/uninstall-crds.sh

# Integration test variables
INTEGRATION_TEST_DIR=test/integration
INTEGRATION_TEST_TIMEOUT=30m
E2E_TEST_TIMEOUT=45m

# Test targets
test-unit:
	@echo "Running unit tests..."
	@$(GO_TEST) -v -race -coverprofile=coverage.out ./pkg/...


test-integration-kind:
	@echo "Running integration tests with Kind..."
	@$(GO_TEST) -v -timeout $(INTEGRATION_TEST_TIMEOUT) ./$(INTEGRATION_TEST_DIR)/...

test-integration-cleanup:
	@echo "Cleaning up integration test environment..."
	@kind delete cluster --name shard-controller-test || true

test-performance:
	@echo "Running performance tests..."
	@$(GO_TEST) -v -timeout $(INTEGRATION_TEST_TIMEOUT) -run "Performance" ./$(INTEGRATION_TEST_DIR)/...

test-load-balancing:
	@echo "Running load balancing integration tests..."
	@$(GO_TEST) -v -timeout $(INTEGRATION_TEST_TIMEOUT) -run "LoadBalancing" ./$(INTEGRATION_TEST_DIR)/...

test-all: test-unit test-integration

# Coverage report
coverage:
	@echo "Generating coverage report..."
	@$(GO_TEST) -v -race -coverprofile=coverage.out ./pkg/...
	@$(GO_CMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Docker variables
REGISTRY ?= shard-controller
TAG ?= latest
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse HEAD)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Docker build targets
.PHONY: docker-build docker-build-manager docker-build-worker docker-push docker-push-manager docker-push-worker

# Build all Docker images
docker-build: docker-build-manager docker-build-worker

# Build manager Docker image
docker-build-manager:
	@echo "Building manager Docker image..."
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-f Dockerfile.manager \
		-t $(REGISTRY)/manager:$(TAG) \
		-t $(REGISTRY)/manager:$(VERSION) \
		.

# Build worker Docker image
docker-build-worker:
	@echo "Building worker Docker image..."
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-f Dockerfile.worker \
		-t $(REGISTRY)/worker:$(TAG) \
		-t $(REGISTRY)/worker:$(VERSION) \
		.

# Build using multi-stage Dockerfile
docker-build-multi:
	@echo "Building manager image using multi-stage Dockerfile..."
	@docker build \
		--build-arg COMPONENT=manager \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(REGISTRY)/manager:$(TAG) \
		.
	@echo "Building worker image using multi-stage Dockerfile..."
	@docker build \
		--build-arg COMPONENT=worker \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(REGISTRY)/worker:$(TAG) \
		.

# Push all Docker images
docker-push: docker-push-manager docker-push-worker

# Push manager Docker image
docker-push-manager:
	@echo "Pushing manager Docker image..."
	@docker push $(REGISTRY)/manager:$(TAG)
	@docker push $(REGISTRY)/manager:$(VERSION)

# Push worker Docker image
docker-push-worker:
	@echo "Pushing worker Docker image..."
	@docker push $(REGISTRY)/worker:$(TAG)
	@docker push $(REGISTRY)/worker:$(VERSION)

# Build and push all images
docker-release: docker-build docker-push

# Load images into kind cluster
kind-load:
	@echo "Loading images into kind cluster..."
	@kind load docker-image $(REGISTRY)/manager:$(TAG) --name shard-controller-demo || true
	@kind load docker-image $(REGISTRY)/worker:$(TAG) --name shard-controller-demo || true

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@if [ -f $(BINARY_DIR)/$(MANAGER_BINARY) ]; then rm $(BINARY_DIR)/$(MANAGER_BINARY); fi
	@if [ -f $(BINARY_DIR)/$(WORKER_BINARY) ]; then rm $(BINARY_DIR)/$(WORKER_BINARY); fi
	@if [ -f coverage.out ]; then rm coverage.out; fi
	@if [ -f coverage.html ]; then rm coverage.html; fi

# Clean Docker images
docker-clean:
	@echo "Cleaning Docker images..."
	@docker rmi $(REGISTRY)/manager:$(TAG) $(REGISTRY)/manager:$(VERSION) || true
	@docker rmi $(REGISTRY)/worker:$(TAG) $(REGISTRY)/worker:$(VERSION) || true

## E2E tests (Kind-based)
.PHONY: test-e2e test-e2e-keep
test-e2e:
	@echo "Running E2E tests on Kind..."
	@E2E_USE_KIND=true E2E_KEEP_CLUSTER=false $(GO_TEST) -v -timeout $(E2E_TEST_TIMEOUT) ./test/e2e/...

test-e2e-keep:
	@echo "Running E2E tests on Kind (keep cluster on success/failure)..."
	@E2E_USE_KIND=true E2E_KEEP_CLUSTER=true $(GO_TEST) -v -timeout $(E2E_TEST_TIMEOUT) ./test/e2e/...

# Demo targets
.PHONY: demo demo-comprehensive demo-advanced demo-performance demo-launcher demo-cleanup

# Run the interactive demo launcher
demo: demo-launcher

# Run comprehensive demo (all capabilities)
demo-comprehensive:
	@echo "Running comprehensive demo..."
	@./scripts/comprehensive-demo.sh

# Run advanced features demo
demo-advanced:
	@echo "Running advanced features demo..."
	@./scripts/advanced-demo.sh

# Run performance testing demo
demo-performance:
	@echo "Running performance testing demo..."
	@./scripts/performance-demo.sh

# Launch interactive demo menu
demo-launcher:
	@echo "Launching interactive demo menu..."
	@./scripts/demo-launcher.sh

# Clean up all demo resources
demo-cleanup:
	@echo "Cleaning up demo resources..."
	@./scripts/demo-launcher.sh cleanup
