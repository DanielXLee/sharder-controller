# Go parameters
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_CLEAN=$(GO_CMD) clean
GO_TEST=$(GO_CMD) test
GO_FMT=$(GO_CMD) fmt
GO_VET=$(GO_CMD) vet
GO_RUN=$(GO_CMD) run

# Project variables
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
	@$(GO_BUILD) -o $(BINARY_DIR)/$(MANAGER_BINARY) $(MANAGER_MAIN)

worker:
	@echo "Building worker..."
	@$(GO_BUILD) -o $(BINARY_DIR)/$(WORKER_BINARY) $(WORKER_MAIN)

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

# Test targets
test-unit:
	@echo "Running unit tests..."
	@$(GO_TEST) -v -race -coverprofile=coverage.out ./pkg/...

test-integration: test-integration-setup
	@echo "Running integration tests..."
	@$(GO_TEST) -v -timeout $(INTEGRATION_TEST_TIMEOUT) ./$(INTEGRATION_TEST_DIR)/...

test-integration-setup:
	@echo "Setting up integration test environment..."
	@./$(INTEGRATION_TEST_DIR)/setup.sh

test-integration-kind:
	@echo "Running integration tests with Kind..."
	@USE_KIND=true ./$(INTEGRATION_TEST_DIR)/setup.sh
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

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@if [ -f $(BINARY_DIR)/$(MANAGER_BINARY) ]; then rm $(BINARY_DIR)/$(MANAGER_BINARY); fi
	@if [ -f $(BINARY_DIR)/$(WORKER_BINARY) ]; then rm $(BINARY_DIR)/$(WORKER_BINARY); fi
	@if [ -f coverage.out ]; then rm coverage.out; fi
	@if [ -f coverage.html ]; then rm coverage.html; fi

