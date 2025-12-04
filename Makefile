# Hubble Anomaly Detector Makefile

.PHONY: build run clean test deps help api-run api-build docker-build-guard docker-build-api docker-build-ui docker-push-guard docker-push-api docker-push-ui docker-build-all docker-push-all

# Variables
BINARY_NAME=hubble-guard
BUILD_DIR=build
VERSION=1.0.0
DOCKER_REGISTRY=docker.io/ramseytrinh338
DOCKER_USERNAME=ramseytrinh338

# Default target
all: build

run-help:
	@echo "Usage: make <target>"
	@./$(BUILD_DIR)/$(BINARY_NAME) --help

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/hubble-guard
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

# Run with custom parameters
run-dev: build
	@echo "Running $(BINARY_NAME) in development mode..."
	@./$(BUILD_DIR)/$(BINARY_NAME) --log-level=debug --hubble-server=localhost:4245

# API Server commands (run from root)
api-run:
	@echo "Starting API server on port 5001..."
	@go run api/main.go -port=5001 -config=configs/anomaly_detection.yaml

api-build:
	@echo "Building API server..."
	@mkdir -p bin
	@go build -o bin/api-server api/main.go
	@echo "Binary built: bin/api-server"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f alerts.log
	@rm -f bin/api-server
	@rm -f bin/api-server.exe
	@echo "Clean complete"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Code formatted"

# Lint code
lint:
	@echo "Linting code..."
	@go vet ./...
	@echo "Linting complete"

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/hubble-guard
	@GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/hubble-guard
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/hubble-guard
	@echo "Multi-platform build complete"

# Create release package
release: clean build-all
	@echo "Creating release package..."
	@mkdir -p $(BUILD_DIR)/release
	@cp $(BUILD_DIR)/$(BINARY_NAME)-* $(BUILD_DIR)/release/
	@cp README.md $(BUILD_DIR)/release/
	@cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-$(VERSION).tar.gz release/
	@echo "Release package created: $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION).tar.gz"

# Build hubble-guard Docker image
docker-build-guard:
	@echo "Building hubble-guard Docker image..."
	@docker build -f Dockerfile.hubble-guard -t $(DOCKER_USERNAME)/hubble-guard:$(VERSION) -t $(DOCKER_USERNAME)/hubble-guard:latest .
	@echo "Docker image built: $(DOCKER_USERNAME)/hubble-guard:$(VERSION)"

# Build hubble-guard-api Docker image
docker-build-api:
	@echo "Building hubble-guard-api Docker image..."
	@docker build -f Dockerfile.hubble-guard-api -t $(DOCKER_USERNAME)/hubble-guard-api:$(VERSION) -t $(DOCKER_USERNAME)/hubble-guard-api:latest .
	@echo "Docker image built: $(DOCKER_USERNAME)/hubble-guard-api:$(VERSION)"

# Build hubble-guard-ui Docker image
docker-build-ui:
	@echo "Building hubble-guard-ui Docker image..."
	@docker build -f ui/Dockerfile -t $(DOCKER_USERNAME)/hubble-guard-ui:$(VERSION) -t $(DOCKER_USERNAME)/hubble-guard-ui:latest ./ui
	@echo "Docker image built: $(DOCKER_USERNAME)/hubble-guard-ui:$(VERSION)"

# Build all Docker images
docker-build-all: docker-build-guard docker-build-api docker-build-ui
	@echo "All Docker images built successfully!"

# Push hubble-guard Docker image to Docker Hub
docker-push-guard:
	@if [ -z "$(DOCKER_USERNAME)" ]; then \
		echo "Error: DOCKER_USERNAME not set"; \
		exit 1; \
	fi
	@echo "Pushing hubble-guard to Docker Hub..."
	@docker push $(DOCKER_USERNAME)/hubble-guard:$(VERSION)
	@docker push $(DOCKER_USERNAME)/hubble-guard:latest
	@echo "Image pushed: $(DOCKER_USERNAME)/hubble-guard:$(VERSION)"

# Push hubble-guard-api Docker image to Docker Hub
docker-push-api:
	@if [ -z "$(DOCKER_USERNAME)" ]; then \
		echo "Error: DOCKER_USERNAME not set"; \
		exit 1; \
	fi
	@echo "Pushing hubble-guard-api to Docker Hub..."
	@docker push $(DOCKER_USERNAME)/hubble-guard-api:$(VERSION)
	@docker push $(DOCKER_USERNAME)/hubble-guard-api:latest
	@echo "Image pushed: $(DOCKER_USERNAME)/hubble-guard-api:$(VERSION)"

# Push hubble-guard-ui Docker image to Docker Hub
docker-push-ui:
	@if [ -z "$(DOCKER_USERNAME)" ]; then \
		echo "Error: DOCKER_USERNAME not set"; \
		exit 1; \
	fi
	@echo "Pushing hubble-guard-ui to Docker Hub..."
	@docker push $(DOCKER_USERNAME)/hubble-guard-ui:$(VERSION)
	@docker push $(DOCKER_USERNAME)/hubble-guard-ui:latest
	@echo "Image pushed: $(DOCKER_USERNAME)/hubble-guard-ui:$(VERSION)"

# Push all Docker images to Docker Hub
docker-push-all: docker-push-guard docker-push-api docker-push-ui
	@echo "All Docker images pushed successfully!"

# Helm lint
helm-lint:
	@echo "Linting Helm chart..."
	@helm lint ./helm/hubble-guard
	@echo "Helm lint complete"

# Helm package
helm-package:
	@echo "Packaging Helm chart..."
	@helm package ./helm/hubble-guard
	@echo "Helm chart packaged"

# Helm install (set NAMESPACE env var, default: hubble)
helm-install: helm-lint
	@NAMESPACE=$${NAMESPACE:-hubble}; \
	echo "Installing Helm chart to namespace $$NAMESPACE..."; \
	helm install hubble-guard ./helm/hubble-guard \
		--namespace $$NAMESPACE \
		--create-namespace

# Helm upgrade
helm-upgrade: helm-lint
	@NAMESPACE=$${NAMESPACE:-hubble}; \
	echo "Upgrading Helm chart in namespace $$NAMESPACE..."; \
	helm upgrade hubble-guard ./helm/hubble-guard \
		--namespace $$NAMESPACE

# Helm uninstall
helm-uninstall:
	@NAMESPACE=$${NAMESPACE:-hubble}; \
	echo "Uninstalling Helm chart from namespace $$NAMESPACE..."; \
	helm uninstall hubble-guard --namespace $$NAMESPACE

# Helm template (dry-run)
helm-template:
	@echo "Rendering Helm templates..."
	@helm template hubble-guard ./helm/hubble-guard

# Show help
help:
	@echo "Available targets:"
	@echo "  build              - Build the application"
	@echo "  run                - Build and run the application"
	@echo "  run-dev            - Run in development mode with debug logging"
	@echo "  api-run            - Run API server (from root directory)"
	@echo "  api-build          - Build API server binary"
	@echo "  clean              - Clean build artifacts"
	@echo "  deps               - Install dependencies"
	@echo "  test               - Run tests"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  fmt                - Format code"
	@echo "  lint               - Lint code"
	@echo "  build-all          - Build for multiple platforms"
	@echo "  release            - Create release package"
	@echo "  docker-build-guard - Build hubble-guard Docker image"
	@echo "  docker-build-api   - Build hubble-guard-api Docker image"
	@echo "  docker-build-ui    - Build hubble-guard-ui Docker image"
	@echo "  docker-build-all   - Build all 3 Docker images"
	@echo "  docker-push-guard  - Build and push hubble-guard to Docker Hub"
	@echo "  docker-push-api    - Build and push hubble-guard-api to Docker Hub"
	@echo "  docker-push-ui     - Build and push hubble-guard-ui to Docker Hub"
	@echo "  docker-push-all    - Build and push all 3 images to Docker Hub"
	@echo "  helm-lint          - Lint Helm chart"
	@echo "  helm-package       - Package Helm chart"
	@echo "  helm-install       - Install Helm chart (set NAMESPACE env var, default: hubble)"
	@echo "  helm-upgrade       - Upgrade Helm chart"
	@echo "  helm-uninstall     - Uninstall Helm chart"
	@echo "  helm-template      - Render Helm templates (dry-run)"
	@echo "  help               - Show this help message"
