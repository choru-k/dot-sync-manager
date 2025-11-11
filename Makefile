# Simple Makefile for dot-sync-manager testing
# Based on TEST_ARCHITECTURE_BOOK guidelines

.PHONY: test test-unit test-integration test-all clean build help

# Default target
test: test-unit
	@echo "✅ Unit tests completed. Use 'make test-all' for full suite."

# Unit tests only (<30 seconds)
test-unit:
	@echo "🧪 Running unit tests..."
	go test -v ./...

# Unit + Integration tests (<2 minutes)
test-integration:
	@echo "🧪 Running unit + integration tests..."
	go test -v -tags=integration ./...

# All tests including E2E (<15 minutes)
test-all: test-integration
	@echo "🧪 Running all E2E tests..."
	./test/scripts/run-e2e.sh --scenarios=all

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

# Run tests with coverage
test-coverage:
	@echo "📊 Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod verify

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

# Show help
help:
	@echo "Available targets:"
	@echo "  test          - Run unit tests only (default)"
	@echo "  test-unit     - Run unit tests only"
	@echo "  test-integration - Run unit + integration tests"
	@echo "  test-all      - Run all tests including E2E"
	@echo "  test-quick    - Quick test for development"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  build         - Build the DSM binary"
	@echo "  clean         - Clean build artifacts and test data"
	@echo "  deps          - Install/update dependencies"
	@echo "  lint          - Run code linter"
	@echo "  setup-dev     - Set up development environment"
	@echo "  help          - Show this help message"