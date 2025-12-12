# Simple Makefile for dot-sync-manager testing
# Based on TEST_ARCHITECTURE_BOOK guidelines

# Keep in sync with CI (.github/workflows/ci.yml)
GOLANGCI_LINT_VERSION ?= v2.6.2
BIN_DIR := $(shell go env GOPATH)/bin

.PHONY: test test-unit test-integration test-scenarios test-all clean build help fmt setup-hooks

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

# Scenario tests (critical user flow tests)
test-scenarios:
	@echo "🧪 Running scenario tests..."
	go test -v ./test/scenarios/...

# All tests including E2E (<15 minutes)
test-all: test-integration test-scenarios
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


# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod verify
	@echo "🔧 Installing golangci-lint..."
	@installed=""; \
	if command -v golangci-lint >/dev/null 2>&1; then installed=$$(golangci-lint --version | awk '{print $$4}'); fi; \
	desired="$(GOLANGCI_LINT_VERSION)"; \
	desired_stripped=$${desired#v}; \
	if [ "$$installed" != "$$desired_stripped" ]; then \
		echo "Installing golangci-lint $$desired to $(BIN_DIR)..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN_DIR) $$desired; \
	else \
		echo "golangci-lint already installed: $$installed"; \
	fi

# Verify code quality (combines all checks)
verify:
	@echo "🔍 Running all quality checks..."
	$(MAKE) lint
	$(MAKE) test-unit
	$(MAKE) test-integration
	$(MAKE) test-scenarios
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

# Format all Go files
fmt:
	@echo "🎨 Formatting Go files..."
	@gofmt -w .
	@echo "✅ Formatting complete"

# Install git hooks (pre-commit and pre-push)
setup-hooks:
	@echo "🪝 Installing git hooks..."
	@mkdir -p .git/hooks
	@ln -sf ../../scripts/hooks/pre-commit .git/hooks/pre-commit
	@ln -sf ../../scripts/hooks/pre-push .git/hooks/pre-push
	@chmod +x scripts/hooks/pre-commit
	@chmod +x scripts/hooks/pre-push
	@echo "✅ Git hooks installed (pre-commit + pre-push)"

# E2E tests alias (for backward compatibility)
test-e2e: test-all

# Show help
help:
	@echo "Available targets:"
	@echo "  test          - Run unit tests only (default)"
	@echo "  test-unit     - Run unit tests only"
	@echo "  test-integration - Run unit + integration tests"
	@echo "  test-scenarios   - Run scenario tests (critical user flows)"
	@echo "  test-all      - Run all tests including E2E"
	@echo "  test-e2e      - Run all tests including E2E (alias for test-all)"
	@echo "  test-quick    - Quick test for development"
	@echo "  build         - Build the DSM binary"
	@echo "  clean         - Clean build artifacts and test data"
	@echo "  deps          - Install/update dependencies"
	@echo "  lint          - Run code linter"
	@echo "  fmt           - Format all Go files with gofmt"
	@echo "  setup-dev     - Set up development environment"
	@echo "  setup-hooks   - Install git hooks (pre-commit + pre-push)"
	@echo "  verify        - Run all quality checks (lint + tests + scenarios + build)"
	@echo "  help          - Show this help message"
