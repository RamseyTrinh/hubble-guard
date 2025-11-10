# Hubble Anomaly Detector Makefile

.PHONY: build run clean test deps help

# Variables
BINARY_NAME=hubble-anomaly-detector
BUILD_DIR=build
VERSION=1.0.0
DOCKER_REGISTRY=docker.io
DOCKER_USERNAME=ramseytrinh3338


# Default target
all: build

run-help:
	@echo "Usage: make <target>"
	@./$(BUILD_DIR)/$(BINARY_NAME) --help

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/hubble-detector
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

# Run with custom parameters
run-dev: build
	@echo "Running $(BINARY_NAME) in development mode..."
	@./$(BUILD_DIR)/$(BINARY_NAME) --log-level=debug --hubble-server=localhost:4245

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f alerts.log
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
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/hubble-detector
	@GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/hubble-detector
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/hubble-detector
	@echo "Multi-platform build complete"

# Create release package
release: clean build-all
	@echo "Creating release package..."
	@mkdir -p $(BUILD_DIR)/release
	@cp $(BUILD_DIR)/$(BINARY_NAME)-* $(BUILD_DIR)/release/
	@cp README.md config.json $(BUILD_DIR)/release/
	@cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-$(VERSION).tar.gz release/
	@echo "Release package created: $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION).tar.gz"

# Docker build
docker-build:
	@echo "Building Docker image..."
	@docker build -t hubble-anomaly-detector:$(VERSION) .
	@echo "Docker image built: hubble-anomaly-detector:$(VERSION)"

# Docker push (set DOCKER_REGISTRY env var)
docker-push: docker-build
	@if [ -z "$(DOCKER_REGISTRY)" ]; then \
		echo "Error: DOCKER_REGISTRY not set"; \
		exit 1; \
	fi
	@echo "Tagging and pushing image..."
	@docker tag hubble-anomaly-detector:$(VERSION) $(DOCKER_REGISTRY)/hubble-anomaly-detector:$(VERSION)
	@docker tag hubble-anomaly-detector:$(VERSION) $(DOCKER_REGISTRY)/hubble-anomaly-detector:latest
	@docker push $(DOCKER_REGISTRY)/hubble-anomaly-detector:$(VERSION)
	@docker push $(DOCKER_REGISTRY)/hubble-anomaly-detector:latest
	@echo "Image pushed to $(DOCKER_REGISTRY)/hubble-anomaly-detector:$(VERSION)"

# Helm lint
helm-lint:
	@echo "Linting Helm chart..."
	@helm lint ./helm/hubble-anomaly-detector
	@echo "Helm lint complete"

# Helm package
helm-package:
	@echo "Packaging Helm chart..."
	@helm package ./helm/hubble-anomaly-detector
	@echo "Helm chart packaged"

# Helm install (set NAMESPACE env var, default: hubble)
helm-install: helm-lint
	@NAMESPACE=$${NAMESPACE:-hubble}; \
	echo "Installing Helm chart to namespace $$NAMESPACE..."; \
	helm install hubble-detector ./helm/hubble-anomaly-detector \
		--namespace $$NAMESPACE \
		--create-namespace

# Helm upgrade
helm-upgrade: helm-lint
	@NAMESPACE=$${NAMESPACE:-hubble}; \
	echo "Upgrading Helm chart in namespace $$NAMESPACE..."; \
	helm upgrade hubble-detector ./helm/hubble-anomaly-detector \
		--namespace $$NAMESPACE

# Helm uninstall
helm-uninstall:
	@NAMESPACE=$${NAMESPACE:-hubble}; \
	echo "Uninstalling Helm chart from namespace $$NAMESPACE..."; \
	helm uninstall hubble-detector --namespace $$NAMESPACE

# Helm template (dry-run)
helm-template:
	@echo "Rendering Helm templates..."
	@helm template hubble-detector ./helm/hubble-anomaly-detector

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  run            - Build and run the application"
	@echo "  run-dev        - Run in development mode with debug logging"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Install dependencies"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  release        - Create release package"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-push    - Build and push Docker image (requires DOCKER_REGISTRY)"
	@echo "  helm-lint      - Lint Helm chart"
	@echo "  helm-package   - Package Helm chart"
	@echo "  helm-install   - Install Helm chart (set NAMESPACE env var, default: hubble)"
	@echo "  helm-upgrade   - Upgrade Helm chart"
	@echo "  helm-uninstall - Uninstall Helm chart"
	@echo "  helm-template  - Render Helm templates (dry-run)"
	@echo "  help           - Show this help message"
