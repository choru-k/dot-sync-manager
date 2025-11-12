#!/bin/bash
# Comprehensive cleanup script for stateless E2E testing
# Ensures complete isolation between test runs

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/test/docker compose.test.yml"
PROJECT_NAME="dot-sync-manager"
DSM_FORCE_PRUNE="${DSM_FORCE_PRUNE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

docker_available() {
    docker info >/dev/null 2>&1
}

# Cleanup function for graceful shutdown
cleanup() {
    log_info "🧹 Starting comprehensive cleanup..."

    local exit_code=${1:-0}

    if docker_available; then
        # Stop and remove containers
        log_info "Stopping and removing containers..."
        if [ -f "$COMPOSE_FILE" ]; then
            docker compose -f "$COMPOSE_FILE" down -v --remove-orphans --timeout 30 || {
                log_warning "Some containers may not have stopped gracefully"
                # Force remove remaining containers
                docker compose -f "$COMPOSE_FILE" kill || true
                docker compose -f "$COMPOSE_FILE" rm -fv || true
            }
        fi

        # Remove project-specific containers
        log_info "Removing project containers..."
        docker container ls -a --filter "name=$PROJECT_NAME" --format "{{.Names}}" | \
            xargs -r docker container rm -fv || true

        # Remove project-specific volumes
        log_info "Removing project volumes..."
        docker volume ls --filter "name=$PROJECT_NAME" --format "{{.Name}}" | \
            xargs -r docker volume rm -f || true

        # Remove project-specific networks
        log_info "Removing project networks..."
        docker network ls --filter "name=$PROJECT_NAME" --format "{{.Name}}" | \
            xargs -r docker network rm || true

        if [ "$DSM_FORCE_PRUNE" = "true" ]; then
            log_info "Performing Docker system cleanup (DSM_FORCE_PRUNE=true)..."
            docker system prune -f --volumes || {
                log_warning "Docker system prune failed, continuing..."
            }
        else
            log_info "Skipping docker system prune (set DSM_FORCE_PRUNE=true to enable)."
        fi
    else
        log_warning "Docker daemon not reachable; skipping Docker resource cleanup."
    fi

    # Clean up test artifacts
    log_info "Cleaning up test artifacts..."
    rm -rf /tmp/dsm-test-* 2>/dev/null || true
    rm -rf /tmp/dsm-e2e-* 2>/dev/null || true
    rm -rf /tmp/dot-sync-test-* 2>/dev/null || true

    # Clean up any leftover processes
    log_info "Cleaning up leftover processes..."
    pkill -f "dsm-test" 2>/dev/null || true
    pkill -f "docker compose.*test" 2>/dev/null || true

    # Remove temporary SSH keys
    log_info "Cleaning up SSH keys..."
    find /tmp -name "*test-key*" -type f -delete 2>/dev/null || true

    # Clean up any test data in user home
    log_info "Cleaning up test data in home directory..."
    rm -rf ~/.dsm-test-* 2>/dev/null || true
    rm -rf ~/dsm-test-repos 2>/dev/null || true

    if docker_available; then
        # Final verification
        log_info "Verifying cleanup completion..."
        local remaining_containers
        local remaining_volumes
        remaining_containers=$(docker container ls -a --filter "name=$PROJECT_NAME" --format "{{.Names}}" | wc -l)
        remaining_volumes=$(docker volume ls --filter "name=$PROJECT_NAME" --format "{{.Name}}" | wc -l)

        if [ "$remaining_containers" -eq 0 ] && [ "$remaining_volumes" -eq 0 ]; then
            log_success "✅ Cleanup completed successfully"
        else
            log_warning "⚠️  Some resources may remain:"
            [ "$remaining_containers" -gt 0 ] && echo "  - $remaining_containers containers"
            [ "$remaining_volumes" -gt 0 ] && echo "  - $remaining_volumes volumes"
        fi
    fi

    log_info "🎯 Cleanup completed with exit code: $exit_code"
    exit $exit_code
}

# Force cleanup function (more aggressive)
force_cleanup() {
    log_info "🔥 Starting force cleanup (aggressive mode)..."

    if ! docker_available; then
        log_warning "Docker daemon not reachable; skipping force cleanup."
        return
    fi

    # Kill all containers forcefully
    docker container ls -a --filter "name=$PROJECT_NAME" --format "{{.Names}}" | \
        xargs -r docker container kill -s 9 || true

    # Remove all containers
    docker container ls -a --filter "name=$PROJECT_NAME" --format "{{.Names}}" | \
        xargs -r docker container rm -fv || true

    # Remove all volumes
    docker volume ls --filter "name=$PROJECT_NAME" --format "{{.Name}}" | \
        xargs -r docker volume rm -f || true

    # Clean all Docker resources
    docker system prune -af --volumes || true

    log_success "🔥 Force cleanup completed"
}

# Show cleanup status
show_status() {
    log_info "📊 Current cleanup status:"

    echo "Containers:"
    docker container ls -a --filter "name=$PROJECT_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" || echo "  None"

    echo ""
    echo "Volumes:"
    docker volume ls --filter "name=$PROJECT_NAME" --format "table {{.Name}}\t{{.Driver}}" || echo "  None"

    echo ""
    echo "Networks:"
    docker network ls --filter "name=$PROJECT_NAME" --format "table {{.Name}}\t{{.Driver}}\t{{.Scope}}" || echo "  None"
}

# Main execution
main() {
    case "${1:-cleanup}" in
        "cleanup")
            cleanup 0
            ;;
        "force")
            force_cleanup
            ;;
        "status")
            show_status
            ;;
        "help"|"-h"|"--help")
            echo "Usage: $0 [cleanup|force|status|help]"
            echo "  cleanup - Perform standard cleanup (default)"
            echo "  force    - Perform aggressive force cleanup"
            echo "  status   - Show current cleanup status"
            echo "  help     - Show this help message"
            ;;
        *)
            log_error "Unknown command: $1"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Set up signal handlers for graceful cleanup
trap cleanup EXIT INT TERM

# Execute main function
main "$@"
