# Simple Makefile for dot-sync-manager testing
# Based on TEST_ARCHITECTURE_BOOK guidelines

.PHONY: test test-unit test-integration test-all clean build help

# Default target
test: test-unit
	@echo "✅ Unit tests completed. Use 'make test-all' for full suite."

# Unit tests only (<30 seconds)
test-unit:
	@echo "🧪 Running unit tests..."
	go test -v $(go list ./... | grep -v test/scenarios)

# Unit + Integration tests (<2 minutes)
test-integration:
	@echo "🧪 Running unit + integration tests..."
	go test -v -tags=integration $(go list ./... | grep -v test/scenarios)

# All tests including E2E (<15 minutes)
test-all: test-integration
	@echo "🧪 Running all E2E tests..."
	./test/scripts/run-e2e.sh -s all

# Build the application
build:
	@echo "🔨 Building DSM..."
	go build -v -o bin/dsm .

# Clean build artifacts and test data
clean:
	@echo "🧹 Cleaning up..."
	rm -rf bin/
	rm -rf coverage.out
	./test/scripts/cleanup.sh cleanup 2>/dev/null || true

# Quick test for development (<10 seconds)
test-quick:
	@echo "⚡ Quick test (changed packages only)..."
	go test -v ./internal/...

# Run tests with coverage (unit + integration only, excludes E2E)
test-coverage:
	@echo "📊 Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./internal/... ./cmd/...
	go tool cover -func=coverage.out

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod verify
	@echo "🔧 Installing golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	else \
		echo "golangci-lint already installed"; \
	fi

# Verify code quality (combines all checks)
verify:
	@echo "🔍 Running all quality checks..."
	$(MAKE) lint
	$(MAKE) test-unit
	$(MAKE) test-integration
	$(MAKE) build
	@echo "✅ All quality checks passed"

# Lint code
lint:
	@echo "🔍 Running linter..."
	golangci-lint run ./...

# Development setup
setup-dev:
	@echo "🛠️ Setting up development environment..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	chmod +x test/scripts/*.sh
	chmod +x test/fixtures/ssh_keys/*.sh

# E2E tests alias (for backward compatibility)
test-e2e: test-all

# Show help
help:
	@echo "Available targets:"
	@echo "  test          - Run unit tests only (default)"
	@echo "  test-unit     - Run unit tests only"
	@echo "  test-integration - Run unit + integration tests"
	@echo "  test-all      - Run all tests including E2E"
	@echo "  test-e2e      - Run all tests including E2E (alias for test-all)"
	@echo "  test-quick    - Quick test for development"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  build         - Build the DSM binary"
	@echo "  clean         - Clean build artifacts and test data"
	@echo "  deps          - Install/update dependencies"
	@echo "  lint          - Run code linter"
	@echo "  setup-dev     - Set up development environment"
	@echo "  verify        - Run all quality checks (lint + tests + build)"
	@echo "  help          - Show this help message"
