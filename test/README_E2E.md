# DSM E2E Testing Framework

This document describes the comprehensive End-to-End (E2E) testing framework for the Dotfile Sync Manager (DSM). The framework provides completely isolated, stateless testing environments using Docker containers.

## 🎯 Overview

The E2E testing framework ensures that DSM works correctly across different scenarios and environments. It uses Docker to create isolated test environments that guarantee no cross-test contamination.

## 🏗️ Architecture

### Components

1. **Security-Hardened Dockerfile** (`test/e2e/Dockerfile.test`)
   - Multi-stage build with Alpine Linux
   - Non-root user execution (UID 1000)
   - Minimal attack surface
   - Optimized for testing

2. **Stateless Docker Compose** (`test/e2e/docker compose.test.yml`)
   - Isolated test runner container
   - Gitea git remote service
   - Disposable volumes for each test run
   - Security hardening

3. **Test Scenarios** (`test/e2e/scenarios/`)
   - Basic sync workflow tests
   - Conflict resolution tests
   - File system watching tests
   - Cross-platform compatibility tests

4. **Automation Scripts** (`test/scripts/`)
   - Test execution and cleanup
   - Local development workflow
   - Environment setup

## 🚀 Quick Start

### Local Development

1. **Set up development environment:**
   ```bash
   ./test/scripts/setup-dev.sh
   ```

2. **Load shell integration:**
   ```bash
   source ~/.dsm-dev/shell-integration.sh
   ```

3. **Run quick tests:**
   ```bash
   dsm-test          # Run all E2E tests
   dsm-quick         # Run quick tests only
   dsm-watch         # Start watch mode
   ```

### Manual Testing

1. **Build test environment:**
   ```bash
   cd test/e2e
   docker compose build
   ```

2. **Run specific tests:**
   ```bash
   ./test/scripts/run-e2e.sh --scenarios=basic --verbose
   ```

3. **Clean up:**
   ```bash
   ./test/scripts/cleanup.sh
   ```

## 📁 Directory Structure

```
test/
├── e2e/
│   ├── Dockerfile.test              # Security-hardened test image
│   ├── docker compose.test.yml      # Stateless test environment
│   ├── scenarios/                   # E2E test scenarios
│   │   ├── basic_sync_test.go
│   │   ├── conflict_resolution_test.go
│   │   ├── file_watching_test.go
│   │   └── cross_platform_test.go
│   └── fixtures/                    # Test data and configs
│       ├── sample_dotfiles/
│       ├── test_configs/
│       └── ssh_keys/
├── scripts/
│   ├── run-e2e.sh                   # Main E2E test runner
│   ├── cleanup.sh                   # Cleanup automation
│   ├── setup-dev.sh                 # Development environment setup
│   └── run-dev.sh                   # Local development runner
└── .dockerignore                    # Docker ignore rules
```

## 🧪 Test Scenarios

### Basic Sync Workflow (`basic_sync_test.go`)

Tests the fundamental DSM workflow:
- Initialize DSM
- Add dotfiles
- List managed files
- Manual sync
- Verify synced files

### Conflict Resolution (`conflict_resolution_test.go`)

Tests DSM's conflict handling:
- Simulate remote changes
- Create local conflicts
- Verify stash/restore functionality
- Test multiple conflict scenarios

### File System Watching (`file_watching_test.go`)

Tests file system monitoring:
- File creation detection
- File modification detection
- File deletion handling
- Debouncing behavior
- Multiple file changes
- Ignored file patterns

### Cross-Platform Compatibility (`cross_platform_test.go`)

Tests path handling across platforms:
- Different path separators
- Special characters in filenames
- Symlink handling
- Long file paths
- Platform-specific paths

## 🔧 Configuration

### Environment Variables

- `TEST_ID`: Unique identifier for test runs
- `TEST_SCENARIOS`: Comma-separated list of scenarios to run
- `TEST_VERBOSE`: Enable verbose output (true/false)
- `TEST_KEEP_CONTAINERS`: Keep containers after tests (true/false)

### Test Configuration

Test configurations are stored in `test/e2e/fixtures/test_configs/`:

- `basic_config.json`: Standard DSM configuration for testing
- Machine-specific settings
- Git repository configuration
- Sync behavior settings

## 🐳 Docker Configuration

### Security Features

- **Non-root execution**: All containers run as UID 1000
- **Read-only filesystems**: Minimal writable directories
- **Capability dropping**: All Linux capabilities dropped
- **No new privileges**: Prevent privilege escalation

### Volume Management

- **Disposable volumes**: Fresh volumes for each test run
- **Automatic cleanup**: Complete volume removal after tests
- **Isolation**: No shared volumes between test runs

### Network Configuration

- **Isolated network**: Custom bridge network for test communication
- **Internal services**: Git remote service for testing
- **Port mapping**: Optional for debugging

## 🔍 Development Workflow

### Local Development

1. **Setup** (one-time):
   ```bash
   ./test/scripts/setup-dev.sh
   source ~/.dsm-dev/shell-integration.sh
   ```

2. **Daily workflow**:
   ```bash
   dsm-quick          # Run quick tests during development
   dsm-watch          # Start watch mode for continuous testing
   dsm-status         # Check environment status
   ```

3. **Before commits**:
   ```bash
   dsm-test           # Run full test suite
   dsm-clean          # Clean environment
   ```

### Debugging

1. **Keep containers for debugging**:
   ```bash
   TEST_KEEP_CONTAINERS=true ./test/scripts/run-e2e.sh
   ```

2. **View container logs**:
   ```bash
   docker compose -f test/e2e/docker compose.test.yml logs
   ```

3. **Inspect test environment**:
   ```bash
   docker compose -f test/e2e/docker compose.test.yml exec dsm-test bash
   ```

## 🔄 CI/CD Integration

### GitHub Actions

The E2E tests are integrated into the CI pipeline:

1. **Unit tests** run first
2. **E2E tests** run after unit tests pass
3. **Build** only runs if both test suites pass

### CI Configuration

```yaml
e2e-tests:
  runs-on: ubuntu-latest
  needs: test
  steps:
    - name: Run E2E tests
      run: ./test/scripts/run-e2e.sh --scenarios=basic --verbose
```

## 🧹 Cleanup and Maintenance

### Automatic Cleanup

- **Test completion**: All containers and volumes removed
- **Signal handling**: Graceful cleanup on interruption
- **Force cleanup**: Aggressive cleanup when needed

### Manual Cleanup

```bash
# Standard cleanup
./test/scripts/cleanup.sh

# Force cleanup (aggressive)
./test/scripts/cleanup.sh force

# Show cleanup status
./test/scripts/cleanup.sh status
```

### Environment Reset

```bash
# Reset entire development environment
./test/scripts/setup-dev.sh clean
./test/scripts/setup-dev.sh setup
```

## 📊 Test Reports

Test reports are generated and stored in `/tmp/dsm-e2e-report-*.txt`:

- Container status
- Volume status
- Network status
- Test execution details

### CI Artifacts

Test reports are uploaded as GitHub Actions artifacts for debugging.

## 🚨 Troubleshooting

### Common Issues

1. **Docker not running**:
   ```bash
   # Start Docker daemon
   sudo systemctl start docker  # Linux
   # or restart Docker Desktop
   ```

2. **Port conflicts**:
   ```bash
   # Check port usage
   lsof -i :3000
   # Stop conflicting services
   ```

3. **Permission issues**:
   ```bash
   # Fix Docker permissions
   sudo usermod -aG docker $USER
   # Re-login required
   ```

4. **Container startup failures**:
   ```bash
   # Check container logs
   docker compose -f test/e2e/docker compose.test.yml logs
   # Rebuild images
   docker compose -f test/e2e/docker compose.test.yml build --no-cache
   ```

### Debug Mode

Enable debug mode for detailed logging:

```bash
export TEST_VERBOSE=true
./test/scripts/run-e2e.sh --verbose
```

## 🎯 Best Practices

### Development

1. **Run quick tests frequently** during development
2. **Use watch mode** for continuous testing
3. **Clean environment** before major changes
4. **Check logs** when tests fail

### Testing

1. **Test isolation**: Each test run is completely isolated
2. **Stateless design**: No persistent state between tests
3. **Security first**: All containers run with minimal privileges
4. **Comprehensive coverage**: Test all major DSM functionality

### Maintenance

1. **Regular cleanup**: Prevent Docker resource accumulation
2. **Image updates**: Keep test images current
3. **Dependency management**: Update Go modules regularly
4. **Documentation**: Keep this README updated

## 📚 Additional Resources

- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [DSM Project Documentation](./README.md)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)

## 🤝 Contributing

When adding new E2E tests:

1. **Follow naming conventions**: `*_test.go` files
2. **Use test helpers**: Leverage existing utility functions
3. **Include cleanup**: Ensure proper resource cleanup
4. **Document scenarios**: Add clear test descriptions
5. **Update README**: Document new test scenarios

## 📄 License

This E2E testing framework is part of the DSM project and follows the same license terms.