.PHONY: run build test clean help

# Default target
.DEFAULT_GOAL := help

# Variables
API_PORT ?= 5001
CONFIG_FILE ?= ../configs/anomaly_detection.yaml
BINARY_NAME := api-server
BINARY_PATH := bin/$(BINARY_NAME)

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

## help: Show this help message
help:
	@echo "$(GREEN)Available targets:$(NC)"
	@echo "  $(YELLOW)make run$(NC)        - Run API server (default port 5001)"
	@echo "  $(YELLOW)make build$(NC)      - Build API server binary"
	@echo "  $(YELLOW)make test$(NC)       - Run tests"
	@echo "  $(YELLOW)make clean$(NC)      - Clean build artifacts"
	@echo "  $(YELLOW)make deps$(NC)       - Install dependencies"
	@echo ""
	@echo "$(GREEN)Variables:$(NC)"
	@echo "  $(YELLOW)API_PORT$(NC)        - API server port (default: 5001)"
	@echo "  $(YELLOW)CONFIG_FILE$(NC)     - Config file path (default: configs/anomaly_detection.yaml)"
	@echo ""
	@echo "$(GREEN)Examples:$(NC)"
	@echo "  make run                    # Run on port 5001"
	@echo "  make run API_PORT=5002      # Run on port 5002"
	@echo "  make build                  # Build binary"

## run: Run API server
run:
	@echo "$(GREEN)Starting API server on port $(API_PORT)...$(NC)"
	@echo "$(YELLOW)Config file: $(CONFIG_FILE)$(NC)"
	@if [ -d "api" ]; then \
		echo "$(YELLOW)Running from root directory...$(NC)"; \
		go run api/main.go -port=$(API_PORT) -config=$(CONFIG_FILE); \
	else \
		echo "$(YELLOW)Running from api directory, changing to root...$(NC)"; \
		cd .. && go run api/main.go -port=$(API_PORT) -config=$(CONFIG_FILE); \
	fi

## build: Build API server binary
build:
	@echo "$(GREEN)Building API server...$(NC)"
	@if [ -d "api" ]; then \
		mkdir -p bin; \
		go build -o $(BINARY_PATH) api/main.go; \
	else \
		mkdir -p ../bin; \
		cd .. && go build -o $(BINARY_PATH) api/main.go; \
	fi
	@echo "$(GREEN)Binary built: $(BINARY_PATH)$(NC)"

## test: Run tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	@cd .. && go test ./api/...

## clean: Clean build artifacts
clean:
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	@rm -f bin/$(BINARY_NAME)
	@rm -f bin/$(BINARY_NAME).exe
	@echo "$(GREEN)Clean complete$(NC)"

## deps: Install dependencies
deps:
	@echo "$(GREEN)Installing dependencies...$(NC)"
	@cd .. && go mod tidy
	@cd .. && go mod download
	@echo "$(GREEN)Dependencies installed$(NC)"

## dev: Run in development mode with auto-reload (requires air)
dev:
	@echo "$(GREEN)Running in development mode...$(NC)"
	@which air > /dev/null || (echo "$(YELLOW)Installing air...$(NC)" && go install github.com/cosmtrek/air@latest)
	@cd .. && air -c api/.air.toml

