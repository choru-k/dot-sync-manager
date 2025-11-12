#!/bin/bash
# Main E2E test execution script
# Provides stateless test environment with complete isolation

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/test/docker-compose.test.yml"
PROJECT_NAME="dot-sync-manager"

# Test configuration
TEST_ID="${TEST_ID:-e2e-$(date +%s)-$$}"
TEST_SCENARIOS="${TEST_SCENARIOS:-all}"
TEST_VERBOSE="${TEST_VERBOSE:-false}"
TEST_KEEP_CONTAINERS="${TEST_KEEP_CONTAINERS:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
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

log_test() {
    echo -e "${PURPLE}[TEST]${NC} $1"
}

format_duration() {
    local seconds="$1"
    printf "%02d:%02d" $((seconds / 60)) $((seconds % 60))
}

run_with_timing() {
    local label="$1"
    shift
    local start_ts
    start_ts=$(date +%s)
    log_info "⏱️  $label - start"
    if "$@"; then
        local end_ts elapsed
        end_ts=$(date +%s)
        elapsed=$((end_ts - start_ts))
        log_success "⏱️  $label - completed in $(format_duration "$elapsed")"
        return 0
    else
        local exit_code=$?
        local end_ts elapsed
        end_ts=$(date +%s)
        elapsed=$((end_ts - start_ts))
        log_error "⏱️  $label - failed after $(format_duration "$elapsed")"
        return $exit_code
    fi
}

# Pre-flight checks
preflight_checks() {
    log_info "🔍 Running pre-flight checks..."

    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running or accessible"
        exit 1
    fi

    # Check if docker compose is available
    if ! docker compose version >/dev/null 2>&1; then
        log_error "docker compose is not available"
        exit 1
    fi

    # Check if compose file exists
    if [ ! -f "$COMPOSE_FILE" ]; then
        log_error "Docker Compose file not found: $COMPOSE_FILE"
        exit 1
    fi

    # Clean up any existing test environment
    if [ "${TEST_KEEP_CONTAINERS}" != "true" ]; then
        log_info "Cleaning up any existing test environment..."
        "$SCRIPT_DIR/cleanup.sh" cleanup
    fi

    log_success "✅ Pre-flight checks completed"
}

# Setup test environment
setup_test_environment() {
    log_info "🔧 Setting up test environment..."
    log_info "Test ID: $TEST_ID"

    # Create test-specific environment file
    local env_file="/tmp/dsm-test-env-$TEST_ID.env"
    cat > "$env_file" << EOF
TEST_ID=$TEST_ID
TEST_MODE=e2e
TEST_VERBOSE=$TEST_VERBOSE
EOF

    # Build Docker images
    log_info "Building Docker images..."
    run_with_timing "Docker build" docker compose -f "$COMPOSE_FILE" --env-file "$env_file" build

    # Start containers
    log_info "Starting test containers..."
    run_with_timing "Docker up" docker compose -f "$COMPOSE_FILE" --env-file "$env_file" up -d

    # Wait for services to be ready
    log_info "Waiting for services to be ready..."
    run_with_timing "Service readiness" wait_for_services

    # Setup SSH keys
    log_info "Setting up SSH keys..."
    run_with_timing "SSH key setup" docker compose -f "$COMPOSE_FILE" --env-file "$env_file" exec dsm-test \
        bash /app/test/fixtures/ssh_keys/test_ssh_keygen.sh

    # Clean up environment file
    rm -f "$env_file"

    log_success "✅ Test environment setup completed"
}

# Wait for services to be healthy
wait_for_services() {
    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if docker compose -f "$COMPOSE_FILE" ps --services --filter "status=running" | grep -q .; then
            # Check if dsm-test is healthy
            if docker compose -f "$COMPOSE_FILE" exec -T dsm-test dsm version >/dev/null 2>&1; then
                log_success "✅ All services are ready"
                return 0
            fi
        fi

        log_info "Waiting for services... (attempt $attempt/$max_attempts)"
        sleep 2
        ((attempt++))
    done

    log_error "Services failed to become ready within timeout"
    return 1
}

# Run test scenarios
run_test_scenarios() {
    log_info "🧪 Running E2E test scenarios..."

    local scenarios=()
    case "$TEST_SCENARIOS" in
        "all")
            scenarios=("basic_sync" "conflict_resolution" "file_watching" "cross_platform" "editor" "dsm_add")
            ;;
        "basic")
            scenarios=("basic_sync")
            ;;
        "advanced")
            scenarios=("conflict_resolution" "file_watching" "editor" "dsm_add")
            ;;
        *)
            IFS=',' read -ra scenarios <<< "$TEST_SCENARIOS"
            ;;
    esac

    local total_scenarios=${#scenarios[@]}
    local passed_scenarios=0
    local failed_scenarios=0

    log_info "Running $total_scenarios test scenarios..."

    for scenario in "${scenarios[@]}"; do
        log_test "Running scenario: $scenario"

        if run_single_scenario "$scenario"; then
            log_success "✅ Scenario '$scenario' passed"
            ((passed_scenarios++))
        else
            log_error "❌ Scenario '$scenario' failed"
            ((failed_scenarios++))
        fi

        # Brief pause between scenarios
        sleep 1
    done

    # Report results
    log_info "📊 Test Results Summary:"
    log_info "  Total scenarios: $total_scenarios"
    log_success "  Passed: $passed_scenarios"
    [ "$failed_scenarios" -gt 0 ] && log_error "  Failed: $failed_scenarios"

    return $([ "$failed_scenarios" -eq 0 ] && echo 0 || echo 1)
}

# Run a single test scenario
run_single_scenario() {
    local scenario="$1"
    local test_file="/app/test/scenarios/${scenario}_test.go"

    # Check if test file exists
    if ! docker compose -f "$COMPOSE_FILE" exec -T dsm-test test -f "$test_file"; then
        log_warning "Test file not found: $test_file, skipping scenario"
        return 0
    fi

    # Run the test with environment variables
    local test_env="TEST_ID=$TEST_ID TEST_SCENARIO=$scenario TEST_VERBOSE=$TEST_VERBOSE"

    if docker compose -f "$COMPOSE_FILE" exec -T -e "$test_env" dsm-test \
        bash /usr/local/bin/test-runner "$scenario"; then
        return 0
    else
        # Show test logs for debugging
        if [ "$TEST_VERBOSE" = "true" ]; then
            log_info "Container logs for scenario '$scenario':"
            docker compose -f "$COMPOSE_FILE" logs --tail=20 dsm-test
        fi
        return 1
    fi
}

# Generate test report
generate_test_report() {
    log_info "📋 Generating test report..."

    local report_file="/tmp/dsm-e2e-report-$TEST_ID.txt"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')

    cat > "$report_file" << EOF
DSM E2E Test Report
===================

Test ID: $TEST_ID
Timestamp: $timestamp
Test Scenarios: $TEST_SCENARIOS

Container Status:
$(docker compose -f "$COMPOSE_FILE" ps)

Volume Status:
$(docker volume ls --filter "name=$PROJECT_NAME")

Network Status:
$(docker network ls --filter "name=$PROJECT_NAME")

EOF

    log_success "📋 Test report saved to: $report_file"
}

# Main execution
main() {
    local exit_code=0

    log_info "🚀 Starting DSM E2E Test Suite"
    log_info "Project Root: $PROJECT_ROOT"
    log_info "Test ID: $TEST_ID"

    # Set up cleanup trap
    if [ "${TEST_KEEP_CONTAINERS}" != "true" ]; then
        trap 'cleanup $?' EXIT INT TERM
    fi

    # Execute test phases
    run_with_timing "Pre-flight checks" preflight_checks || exit_code=1
    if [ $exit_code -eq 0 ]; then
        run_with_timing "Environment setup" setup_test_environment || exit_code=2
    fi
    if [ $exit_code -eq 0 ]; then
        run_with_timing "Scenario execution" run_test_scenarios || exit_code=3
    fi
    if [ $exit_code -eq 0 ]; then
        run_with_timing "Report generation" generate_test_report
    fi

    if [ $exit_code -eq 0 ]; then
        log_success "🎉 E2E Test Suite completed successfully!"
    else
        log_error "💥 E2E Test Suite failed with exit code: $exit_code"
    fi

    exit $exit_code
}

# Cleanup function
cleanup() {
    local exit_code=${1:-0}

    if [ "${TEST_KEEP_CONTAINERS}" != "true" ]; then
        log_info "🧹 Cleaning up test environment..."
        "$SCRIPT_DIR/cleanup.sh" cleanup
    else
        log_info "🏃 Leaving containers running for debugging (TEST_KEEP_CONTAINERS=true)"
    fi
}

# Handle command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --scenarios|-s)
            TEST_SCENARIOS="$2"
            shift 2
            ;;
        --verbose|-v)
            TEST_VERBOSE="true"
            shift
            ;;
        --keep-containers|-k)
            TEST_KEEP_CONTAINERS="true"
            shift
            ;;
        --test-id|-i)
            TEST_ID="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  -s, --scenarios SCENARIOS    Comma-separated list of scenarios (default: all)"
            echo "  -v, --verbose               Enable verbose output"
            echo "  -k, --keep-containers      Keep containers running after tests"
            echo "  -i, --test-id ID           Custom test ID"
            echo "  -h, --help                 Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use '$0 --help' for usage information"
            exit 1
            ;;
    esac
done

# Execute main function
main
