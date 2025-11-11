#!/bin/bash
set -e

echo "=== CI Environment Simulation (using Makefile) ==="
echo "Go version:"
go version
echo ""

echo "=== Step 1: Install dependencies ==="
make deps
echo ""

echo "=== Step 2: Verify dependencies ==="
make verify
echo ""

echo "=== Step 3: Run code quality checks ==="
make lint
echo ""

echo "=== Step 4: Run tests with coverage ==="
make test-coverage
echo ""

echo "=== Step 5: Build ==="
make build
echo ""

echo "=== Step 6: Verify binary ==="
ls -la bin/dsm
echo ""

echo "=== CI Test Results Summary ==="
echo "Checking test status via make..."
if make test-quick > /dev/null 2>&1; then
    echo "✅ All tests passed!"
    echo ""
    echo "Coverage report:"
    go tool cover -func=coverage.out
else
    echo "❌ Some tests failed!"
    exit 1
fi
echo ""
echo "✅ CI Simulation Completed Successfully!"