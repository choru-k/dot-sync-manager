#!/bin/bash
# Local development test runner for DSM E2E testing
# Provides convenient commands for local development workflow

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/e2e/docker-compose.test.yml"
LOCAL_DEV_DIR="$HOME/.dsm-dev"

# Default configuration
TEST_SCENARIOS="${TEST_SCENARIOS:-basic}"
TEST_VERBOSE="${TEST_VERBOSE:-true}"
TEST_KEEP_CONTAINERS="${TEST_KEEP_CONTAINERS:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_dev() {
    echo -e "${PURPLE}[DEV]${NC} $1"
}

log_test() {
    echo -e "${CYAN}[TEST]${NC} $1"
}

# Check development environment
check_dev_environment() {
    log_info "🔍 Checking development environment..."

    # Check if we're in the right directory
    if [ ! -f "$PROJECT_ROOT/go.mod" ]; then
        log_error "Not in DSM project directory. Please run from ~/dot-sync-manager"
        exit 1
    fi

    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running"
        exit 1
    fi

    # Check if development directory exists
    if [ ! -d "$LOCAL_DEV_DIR" ]; then
        log_warning "Development directory not found. Running setup..."
        "$SCRIPT_DIR/setup-dev.sh" setup
    fi

    # Check if test image exists
    if ! docker images | grep -q "dot-sync-manager.*test"; then
        log_warning "Test image not found. Building..."
        docker-compose -f "$COMPOSE_FILE" build
    fi

    log_success "✅ Development environment ready"
}

# Quick test - run basic scenarios only
run_quick_test() {
    log_test "🚀 Running quick E2E tests..."

    local test_id="dev-quick-$(date +%s)"
    local env_file="/tmp/dsm-dev-quick-$test_id.env"

    # Create environment file
    cat > "$env_file" << EOF
TEST_ID=$test_id
TEST_SCENARIOS=basic
TEST_VERBOSE=$TEST_VERBOSE
TEST_KEEP_CONTAINERS=$TEST_KEEP_CONTAINERS
EOF

    # Run tests
    log_dev "Starting quick test with ID: $test_id"

    if "$PROJECT_ROOT/test/scripts/run-e2e.sh" \
        --test-id "$test_id" \
        --scenarios "basic" \
        --verbose; then
        log_success "✅ Quick tests passed"
    else
        log_error "❌ Quick tests failed"
        return 1
    fi

    # Clean up
    rm -f "$env_file"

    if [ "$TEST_KEEP_CONTAINERS" != "true" ]; then
        "$PROJECT_ROOT/test/scripts/cleanup.sh" cleanup
    fi
}

# Full test - run all scenarios
run_full_test() {
    log_test "🧪 Running full E2E test suite..."

    local test_id="dev-full-$(date +%s)"
    local env_file="/tmp/dsm-dev-full-$test_id.env"

    # Create environment file
    cat > "$env_file" << EOF
TEST_ID=$test_id
TEST_SCENARIOS=all
TEST_VERBOSE=$TEST_VERBOSE
TEST_KEEP_CONTAINERS=$TEST_KEEP_CONTAINERS
EOF

    # Run tests
    log_dev "Starting full test with ID: $test_id"

    if "$PROJECT_ROOT/test/scripts/run-e2e.sh" \
        --test-id "$test_id" \
        --scenarios "all" \
        --verbose; then
        log_success "✅ Full test suite passed"
    else
        log_error "❌ Full test suite failed"
        return 1
    fi

    # Clean up
    rm -f "$env_file"

    if [ "$TEST_KEEP_CONTAINERS" != "true" ]; then
        "$PROJECT_ROOT/test/scripts/cleanup.sh" cleanup
    fi
}

# Watch mode - run tests on file changes
run_watch_mode() {
    log_dev "👀 Starting development watch mode..."

    # Check if fswatch is available
    if ! command -v fswatch >/dev/null 2>&1; then
        log_error "fswatch is required for watch mode"
        log_info "Install with: brew install fswatch (macOS) or apt-get install fswatch (Ubuntu)"
        exit 1
    fi

    log_info "Watching for changes in cmd/, internal/, test/, go.mod, go.sum"
    log_info "Press Ctrl+C to stop watching"

    # Initial test run
    run_quick_test

    # Watch for changes
    fswatch -o -r -1 \
        cmd/ \
        internal/ \
        test/ \
        go.mod \
        go.sum | while read event; do
        echo ""
        log_dev "🔄 Changes detected at $(date '+%H:%M:%S')"

        # Short delay to allow file system to settle
        sleep 2

        # Run quick tests
        if run_quick_test; then
            log_success "✅ Watch mode test completed successfully"
        else
            log_error "❌ Watch mode test failed"
        fi

        echo ""
        log_info "👀 Watching for changes..."
    done
}

# Show status
show_status() {
    log_info "📊 DSM Development Environment Status"

    echo ""
    echo "🐳 Docker Status:"
    if docker info >/dev/null 2>&1; then
        local docker_version=$(docker --version)
        echo "  ✅ Docker: $docker_version"
    else
        echo "  ❌ Docker: Not running"
    fi

    echo ""
    echo "🏗️ Build Status:"
    if docker images | grep -q "dot-sync-manager.*test"; then
        echo "  ✅ Test image: Available"
    else
        echo "  ❌ Test image: Not built"
    fi

    echo ""
    echo "📁 Environment:"
    if [ -d "$LOCAL_DEV_DIR" ]; then
        echo "  ✅ Dev directory: $LOCAL_DEV_DIR"
        echo "  📊 Config: $LOCAL_DEV_DIR/dev-config.json"
        echo "  📝 Logs: $LOCAL_DEV_DIR/logs/"
    else
        echo "  ❌ Dev directory: Not found"
    fi

    echo ""
    echo "🔧 Test Environment:"
    local containers=$(docker-compose -f "$COMPOSE_FILE" ps 2>/dev/null | grep -c "Up\|running" || echo "0")
    local volumes=$(docker volume ls 2>/dev/null | grep -c "dot-sync-manager" || echo "0")

    echo "  📦 Running containers: $containers"
    echo "  💾 Test volumes: $volumes"

    echo ""
    log_info "For detailed container status, run: docker-compose -f $COMPOSE_FILE ps"
}

# Show logs
show_logs() {
    log_info "📋 Showing test environment logs..."

    if docker-compose -f "$COMPOSE_FILE" ps | grep -q "Up"; then
        log_info "Container logs (press Ctrl+C to exit):"
        docker-compose -f "$COMPOSE_FILE" logs -f
    else
        log_warning "No containers are currently running"
        log_info "Start containers with: ./test/scripts/run-dev.sh test"
    fi
}

# Clean environment
clean_environment() {
    log_info "🧹 Cleaning development environment..."

    # Stop and remove containers
    if docker-compose -f "$COMPOSE_FILE" ps | grep -q "Up"; then
        log_info "Stopping containers..."
        docker-compose -f "$COMPOSE_FILE" down
    fi

    # Clean volumes
    log_info "Cleaning volumes..."
    "$PROJECT_ROOT/test/scripts/cleanup.sh" cleanup

    # Clean local temp files
    log_info "Cleaning local temp files..."
    rm -rf /tmp/dsm-dev-*.env 2>/dev/null || true

    log_success "✅ Development environment cleaned"
}

# Interactive mode
interactive_mode() {
    log_info "🎮 DSM Development Interactive Mode"
    echo ""

    while true; do
        echo "Choose an option:"
        echo "1) Quick test"
        echo "2) Full test"
        echo "3) Watch mode"
        echo "4) Show status"
        echo "5) Show logs"
        echo "6) Clean environment"
        echo "7) Exit"
        echo ""
        read -p "Enter choice [1-7]: " choice

        case $choice in
            1)
                run_quick_test
                ;;
            2)
                run_full_test
                ;;
            3)
                run_watch_mode
                ;;
            4)
                show_status
                ;;
            5)
                show_logs
                ;;
            6)
                clean_environment
                ;;
            7)
                log_info "👋 Goodbye!"
                exit 0
                ;;
            *)
                log_error "Invalid choice. Please enter 1-7."
                ;;
        esac

        echo ""
    done
}

# Main function
main() {
    # Ensure we're in the right directory
    cd "$PROJECT_ROOT"

    case "${1:-interactive}" in
        "test"|"quick")
            check_dev_environment
            run_quick_test
            ;;
        "full")
            check_dev_environment
            run_full_test
            ;;
        "watch")
            check_dev_environment
            run_watch_mode
            ;;
        "status")
            show_status
            ;;
        "logs")
            show_logs
            ;;
        "clean")
            clean_environment
            ;;
        "interactive")
            check_dev_environment
            interactive_mode
            ;;
        "help"|"-h"|"--help")
            echo "DSM Development Test Runner"
            echo ""
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  test, quick    - Run quick E2E tests"
            echo "  full          - Run full E2E test suite"
            echo "  watch         - Start watch mode for development"
            echo "  status        - Show development environment status"
            echo "  logs          - Show container logs"
            echo "  clean         - Clean development environment"
            echo "  interactive   - Interactive mode (default)"
            echo "  help          - Show this help message"
            echo ""
            echo "Environment Variables:"
            echo "  TEST_SCENARIOS      - Test scenarios to run (default: basic)"
            echo "  TEST_VERBOSE        - Enable verbose output (default: true)"
            echo "  TEST_KEEP_CONTAINERS - Keep containers after tests (default: false)"
            echo ""
            echo "Examples:"
            echo "  $0 quick                    # Run quick tests"
            echo "  $0 full                     # Run full test suite"
            echo "  TEST_SCENARIOS=basic $0     # Run specific scenarios"
            echo "  TEST_KEEP_CONTAINERS=true $0 # Keep containers for debugging"
            ;;
        *)
            log_error "Unknown command: $1"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Execute main function
main "$@"