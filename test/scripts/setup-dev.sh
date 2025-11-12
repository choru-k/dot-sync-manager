#!/bin/bash
# Local development environment setup script for DSM E2E testing
# Sets up local environment to match CI testing conditions

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/e2e/docker compose.test.yml"

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

log_dev() {
    echo -e "${PURPLE}[DEV]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "🔍 Checking prerequisites..."

    # Check Docker
    if ! command -v docker >/dev/null 2>&1; then
        log_error "Docker is not installed or not in PATH"
        echo "Please install Docker: https://docs.docker.com/get-docker/"
        exit 1
    fi

    # Check Docker Compose
    if ! command -v docker compose >/dev/null 2>&1; then
        log_error "docker compose is not installed or not in PATH"
        echo "Please install Docker Compose: https://docs.docker.com/compose/install/"
        exit 1
    fi

    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running"
        echo "Please start Docker and try again"
        exit 1
    }

    # Check Go installation
    if ! command -v go >/dev/null 2>&1; then
        log_warning "Go is not installed, some features may not work"
    else
        local go_version=$(go version)
        log_info "Go version: $go_version"
    fi

    log_success "✅ Prerequisites check completed"
}

# Build development images
build_dev_images() {
    log_info "🔨 Building development images..."

    # Build test image
    log_info "Building DSM test image..."
    docker compose -f "$COMPOSE_FILE" build

    log_success "✅ Development images built successfully"
}

# Setup local directories
setup_local_directories() {
    log_info "📁 Setting up local directories..."

    # Create local test data directory
    local local_test_dir="$HOME/.dsm-dev"
    mkdir -p "$local_test_dir"/{data,reports,logs}

    # Create local SSH key directory
    mkdir -p "$local_test_dir/ssh"

    # Generate local SSH key if it doesn't exist
    if [ ! -f "$local_test_dir/ssh/dsm_dev_key" ]; then
        log_info "Generating local SSH key..."
        ssh-keygen -t ed25519 -f "$local_test_dir/ssh/dsm_dev_key" -N "" -C "dsm-dev-key"
        chmod 600 "$local_test_dir/ssh/dsm_dev_key"
        chmod 644 "$local_test_dir/ssh/dsm_dev_key.pub"
        log_success "Local SSH key generated"
    fi

    # Create local git repositories directory
    mkdir -p "$local_test_dir/repos"

    log_success "✅ Local directories setup completed"
}

# Create development configuration
create_dev_config() {
    log_info "⚙️ Creating development configuration..."

    local local_test_dir="$HOME/.dsm-dev"
    local config_file="$local_test_dir/dev-config.json"

    cat > "$config_file" << 'EOF'
{
  "machine": {
    "name": "dev-machine",
    "description": "Local Development Machine"
  },
  "git": {
    "repo_path": "'$HOME'/.dsm-dev/repos/dotfiles-dev",
    "remote_url": "http://localhost:3000/dev/dotfiles-dev.git",
    "auth_type": "none",
    "branch": "main"
  },
  "sync": {
    "auto_sync_enabled": true,
    "debounce_seconds": 5,
    "conflict_resolution": "stash",
    "max_retries": 3
  },
  "notifications": {
    "enabled": true,
    "webhook_url": "",
    "email_enabled": false
  },
  "mappings": [],
  "ignore_patterns": [
    "*.tmp",
    "*.log",
    ".DS_Store",
    "node_modules/",
    ".cache/",
    "*.swp",
    "*.swo"
  ],
  "backoff": {
    "enabled": true,
    "initial_delay": 2,
    "max_delay": 30,
    "multiplier": 2,
    "jitter": 0.1
  }
}
EOF

    log_success "✅ Development configuration created: $config_file"
}

# Setup shell aliases and functions
setup_shell_integration() {
    log_info "🐚 Setting up shell integration..."

    local local_test_dir="$HOME/.dsm-dev"
    local shell_rc_file="$local_test_dir/shell-integration.sh"

    cat > "$shell_rc_file" << 'EOF'
# DSM Development Shell Integration
# Source this file in your shell: source ~/.dsm-dev/shell-integration.sh

# Colors
export DSM_DEV_BLUE='\033[0;34m'
export DSM_DEV_GREEN='\033[0;32m'
export DSM_DEV_YELLOW='\033[1;33m'
export DSM_DEV_RED='\033[0;31m'
export DSM_DEV_NC='\033[0m'

# DSM Dev aliases
alias dsm-dev='cd ~/dot-sync-manager && ./test/scripts/run-dev.sh'
alias dsm-test='./test/scripts/run-dev.sh test'
alias dsm-quick='./test/scripts/run-dev.sh quick'
alias dsm-watch='./test/scripts/run-dev.sh watch'
alias dsm-status='./test/scripts/run-dev.sh status'
alias dsm-clean='./test/scripts/run-dev.sh clean'
alias dsm-logs='./test/scripts/run-dev.sh logs'

# DSM Dev functions
dsm-dev-info() {
    echo -e "${DSM_DEV_BLUE}DSM Development Environment${DSM_DEV_NC}"
    echo "Project: ~/dot-sync-manager"
    echo "Config: ~/.dsm-dev/dev-config.json"
    echo "Logs: ~/.dsm-dev/logs/"
    echo "Data: ~/.dsm-dev/data/"
}

dsm-dev-help() {
    echo "DSM Development Commands:"
    echo "  dsm-dev     - Go to project directory"
    echo "  dsm-test    - Run all E2E tests"
    echo "  dsm-quick   - Run quick tests"
    echo "  dsm-watch   - Start watch mode"
    echo "  dsm-status  - Show test environment status"
    echo "  dsm-clean   - Clean test environment"
    echo "  dsm-logs    - Show test logs"
    echo ""
    echo "Functions:"
    echo "  dsm-dev-info - Show development environment info"
    echo "  dsm-dev-help - Show this help"
}

# Auto-completion for DSM dev commands
if command -v complete >/dev/null 2>&1; then
    complete -W "test quick watch status clean logs" dsm-dev
fi

echo -e "${DSM_DEV_GREEN}DSM Development environment loaded!${DSM_DEV_NC}"
echo -e "${DSM_DEV_YELLOW}Type 'dsm-dev-help' for available commands${DSM_DEV_NC}"
EOF

    log_success "✅ Shell integration created: $shell_rc_file"
    log_info "To enable, run: source ~/.dsm-dev/shell-integration.sh"
}

# Create development helper scripts
create_dev_helpers() {
    log_info "🛠️ Creating development helper scripts..."

    local local_test_dir="$HOME/.dsm-dev"

    # Quick test script
    cat > "$local_test_dir/quick-test.sh" << 'EOF'
#!/bin/bash
# Quick test runner for development

cd ~/dot-sync-manager
./test/scripts/run-e2e.sh --scenarios=basic --verbose
EOF

    # Watch mode script
    cat > "$local_test_dir/watch-mode.sh" << 'EOF'
#!/bin/bash
# Watch mode for development - runs tests on file changes

cd ~/dot-sync-manager

# Install fswatch if not available
if ! command -v fswatch >/dev/null 2>&1; then
    echo "fswatch is required for watch mode"
    echo "Install with: brew install fswatch (macOS) or apt-get install fswatch (Ubuntu)"
    exit 1
fi

echo "👀 Starting DSM development watch mode..."
echo "Press Ctrl+C to stop"

# Watch for changes in key directories
fswatch -o -r -1 \
    cmd/ \
    internal/ \
    test/ \
    go.mod \
    go.sum | while read event; do
    echo "🔄 Changes detected, running quick tests..."
    ./test/scripts/run-e2e.sh --scenarios=basic
    echo "✅ Quick tests completed"
    echo "👀 Watching for changes..."
done
EOF

    # Make scripts executable
    chmod +x "$local_test_dir/quick-test.sh"
    chmod +x "$local_test_dir/watch-mode.sh"

    log_success "✅ Development helper scripts created"
}

# Verify development setup
verify_setup() {
    log_info "🔍 Verifying development setup..."

    # Check if images are built
    if docker images | grep -q "dot-sync-manager.*test"; then
        log_success "✅ Test image is available"
    else
        log_warning "⚠️ Test image not found - run build first"
    fi

    # Check if directories exist
    if [ -d "$HOME/.dsm-dev" ]; then
        log_success "✅ Development directory exists"
    else
        log_error "❌ Development directory not found"
    fi

    # Check configuration
    if [ -f "$HOME/.dsm-dev/dev-config.json" ]; then
        log_success "✅ Development configuration exists"
    else
        log_error "❌ Development configuration not found"
    fi

    log_info "🎯 Development setup verification completed"
}

# Main setup function
main() {
    log_info "🚀 Setting up DSM E2E development environment..."

    check_prerequisites
    setup_local_directories
    create_dev_config
    setup_shell_integration
    create_dev_helpers

    log_success "🎉 DSM development environment setup completed!"
    log_info ""
    log_info "Next steps:"
    log_info "1. Build development images: cd ~/dot-sync-manager && ./test/scripts/setup-dev.sh build"
    log_info "2. Load shell integration: source ~/.dsm-dev/shell-integration.sh"
    log_info "3. Run tests: dsm-test or ./test/scripts/run-e2e.sh"
    log_info ""
    log_info "For help, run: dsm-dev-help"
}

# Handle command line arguments
case "${1:-setup}" in
    "setup")
        main
        ;;
    "build")
        log_info "Building development images..."
        build_dev_images
        ;;
    "verify")
        verify_setup
        ;;
    "clean")
        log_info "Cleaning development environment..."
        if [ -d "$HOME/.dsm-dev" ]; then
            rm -rf "$HOME/.dsm-dev"
            log_success "Development environment cleaned"
        else
            log_info "No development environment to clean"
        fi
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [setup|build|verify|clean|help]"
        echo "  setup   - Set up development environment (default)"
        echo "  build   - Build development Docker images"
        echo "  verify  - Verify development setup"
        echo "  clean   - Clean development environment"
        echo "  help    - Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac