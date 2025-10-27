#!/bin/bash
set -e

echo "=== CI Environment Simulation ==="
echo "Go version:"
go version
echo ""

echo "=== Step 1: Download dependencies ==="
go mod download
echo ""

echo "=== Step 2: Verify dependencies ==="
go mod verify
echo ""

echo "=== Step 3: Run go vet ==="
go vet ./...
echo ""

echo "=== Step 4: Run tests with coverage ==="
go test -v -coverprofile=coverage.out ./...
echo ""

echo "=== Step 5: Build ==="
go build -v -o bin/dsm .
echo ""

echo "=== Step 6: Verify binary ==="
ls -la bin/dsm
echo ""

echo "=== CI Test Results Summary ==="
echo "Checking for any failing tests..."
if go test ./... 2>&1 | grep -q "FAIL"; then
    echo "❌ Some tests failed!"
    go test ./... 2>&1 | grep "FAIL" -A 3 -B 1
    exit 1
else
    echo "✅ All tests passed!"
    echo ""
    echo "Coverage report:"
    go tool cover -func=coverage.out
fi
echo ""
echo "✅ CI Simulation Completed Successfully!"