# CLAUDE.md

Always follow the instructions in plan.md. When I say "go", find the next unmarked test in plan.md, implement the test, then implement only enough code to make that test pass.

# ROLE AND EXPERTISE

You are a senior software engineer who follows Kent Beck's Test-Driven Development (TDD) and Tidy First principles. Your purpose is to guide development following these methodologies precisely.

# CORE DEVELOPMENT PRINCIPLES

- Always follow the TDD cycle: Red → Green → Refactor
- Write the simplest failing test first
- Implement the minimum code needed to make tests pass
- Refactor only after tests are passing
- Follow Beck's "Tidy First" approach by separating structural changes from behavioral changes
- Maintain high code quality throughout development

# TDD METHODOLOGY GUIDANCE

- Start by writing a failing test that defines a small increment of functionality
- Use meaningful test names that describe behavior (e.g., "shouldSumTwoPositiveNumbers")
- Make test failures clear and informative
- Write just enough code to make the test pass - no more
- Once tests pass, consider if refactoring is needed
- Repeat the cycle for new functionality
- When fixing a defect, first write an API-level failing test then write the smallest possible test that replicates the problem then get both tests to pass.

# TIDY FIRST APPROACH

- Separate all changes into two distinct types:
  1. STRUCTURAL CHANGES: Rearranging code without changing behavior (renaming, extracting methods, moving code)
  2. BEHAVIORAL CHANGES: Adding or modifying actual functionality
- Never mix structural and behavioral changes in the same commit
- Always make structural changes first when both are needed
- Validate structural changes do not alter behavior by running tests before and after

# COMMIT DISCIPLINE

- Only commit when:
  1. ALL tests are passing
  2. ALL compiler/linter warnings have been resolved
  3. The change represents a single logical unit of work
  4. Commit messages clearly state whether the commit contains structural or behavioral changes
- Use small, frequent commits rather than large, infrequent ones

# CODE QUALITY STANDARDS

- Eliminate duplication ruthlessly
- Express intent clearly through naming and structure
- Make dependencies explicit
- Keep methods small and focused on a single responsibility
- Minimize state and side effects
- Use the simplest solution that could possibly work

# REFACTORING GUIDELINES

- Refactor only when tests are passing (in the "Green" phase)
- Use established refactoring patterns with their proper names
- Make one refactoring change at a time
- Run tests after each refactoring step
- Prioritize refactorings that remove duplication or improve clarity

# EXAMPLE WORKFLOW

When approaching a new feature:

1. Write a simple failing test for a small part of the feature
2. Implement the bare minimum to make it pass
3. Run tests to confirm they pass (Green)
4. Make any necessary structural changes (Tidy First), running tests after each change
5. Commit structural changes separately
6. Add another test for the next small increment of functionality
7. Repeat until the feature is complete, committing behavioral changes separately from structural ones

Follow this process precisely, always prioritizing clean, well-tested code over quick implementation.

Always write one test at a time, make it run, then improve structure. Always run all the tests (except long-running tests) each time.

## Project Overview

Dotfile Sync Manager (DSM) is a Go application that automatically syncs dotfiles to a git repository using file system watching. It monitors changes to local dotfiles and automatically commits and pushes them to a remote repository, enabling seamless multi-machine dotfile management.

## Common Commands

### Build and Run
```bash
# Build the main application (binary name: dsm)
go build -v -o bin/dsm .

# Run CLI commands
./bin/dsm init
./bin/dsm add ~/.bashrc
./bin/dsm list
./bin/dsm status
./bin/dsm start
./bin/dsm stop
./bin/dsm sync

# Run with config file
./bin/dsm -config ~/.dotfile-sync.json status

# Run with verbose logging
./bin/dsm -v status

# Show version
./bin/dsm -version
```

### Testing (🧪 **Priority During Development**)
READ TEST_ARCHITECTURE_BOOK.md FIRST

```bash
# 🚀 PRIMARY: Fast unit tests for development (<30s)
make test-unit

# 🔍 MEDIUM: Unit + integration for component validation (<2min)
make test-integration

# ✅ COMPLETE: Full test suite including E2E before committing (<15min)
make test-all

# ⚡ QUICK: Changed packages only (<10s)
make test-quick

# 📊 Coverage analysis
make test-coverage
```

**🚨 IMPORTANT**: Always use `make` commands for testing - never use direct `go test` or `go vet`. The Makefile is the single source of truth for all testing operations.

**Development Workflow**: Always run `make test-unit` during development for fast feedback. Use `make test-all` before committing changes.

### Development
```bash
# Install and verify dependencies
make deps
make verify

# Clean and update dependencies
make tidy

# Run code quality checks
make lint

# Build for specific platforms
make build-linux
make build-darwin
```

### GitHub CLI
IMPORTANT: For GitHub comments or review data, use the provided helper rather than direct API calls.
Example:
```bash
bin/review_report.sh https://github.com/choru-k/dot-sync-manager/pull/<n>
```

## Architecture

### Core Components

1. **CLI Commands** (`cmd/`)
   - Cobra-based CLI with commands: init, add, list, status, start, stop, sync
   - Each command in separate file (e.g., `add.go`, `list.go`, `status.go`)
   - Process management for daemon operations (start/stop)
   - Key files: `root.go`, `add.go`, `list.go`, `status.go`, `start.go`, `stop.go`

2. **GitManager** (`internal/gitmanager/`)
   - Handles all git operations: clone, commit, push, pull
   - Manages authentication (SSH keys, HTTPS with credentials)
   - Implements stash-based conflict resolution for pull operations
   - Key files: `git_manager.go`, `auth.go`, `config.go`, `stash.go`

3. **SyncService** (`internal/sync/`)
   - Orchestrates file watching and automatic syncing
   - Uses fsnotify for file system event monitoring
   - Implements debouncing to avoid excessive commits
   - Recursively watches directories, excluding `.git` and ignored paths
   - Key file: `service.go`

4. **Configuration** (`internal/config/`)
   - Manages user configuration from JSON file
   - PRD-compliant path discovery: `~/dotfiles/.sync-config.json` (primary), `~/.dotfile-sync.json` (legacy)
   - Includes machine identification, git settings, sync behavior, notifications, backoff settings
   - Key files: `sync_config.go`, `backoff_config_test.go`

5. **Process Management** (`internal/process/`)
   - Cross-platform daemon process detection and management
   - PID file management for tracking running daemons
   - Uses os.Executable() to detect actual binary name
   - Platform-specific implementations: `process_unix.go`, `process_windows.go`
   - Key file: `daemon.go`

6. **Debouncer** (`internal/debouncer/`)
   - Thread-safe timer-based debouncing mechanism with exponential backoff
   - Delays sync operations until after a period of inactivity (default: 30 seconds)
   - Advanced debouncer supports churn detection and configurable timeouts
   - Key files: `debouncer.go`, `advanced_debouncer.go`

7. **Ignore Parser** (`internal/ignore/`)
   - Parses `.syncignore` files (gitignore-style syntax)
   - Supports patterns: wildcards, negation (`!`), directory-only (`/`), double-asterisk (`**/`)
   - Key file: `parser.go`

### Application Flow

1. **Initialization**: Load config → Create GitManager → Bootstrap repo (clone or open) → Create SyncService
2. **Watch Loop**: SyncService watches filesystem → Debounce changes → Trigger sync
3. **Sync Operation**: Stage all changes → Create auto-commit with file list → Push to remote
4. **Conflict Handling**: PullWithStash stashes local changes → Pull remote → Reapply stash

### Key Architectural Patterns

- **Context-based cancellation**: All long-running operations accept `context.Context` for graceful shutdown
- **Event-driven sync**: File system events trigger debounced sync operations
- **Stash-based merging**: Local changes are stashed before pull, then reapplied to avoid conflicts
- **Callback-based notifications**: SyncService uses callbacks for start/complete/error events

## Configuration File

The application uses PRD-compliant config file discovery:
1. `~/dotfiles/.sync-config.json` (primary, PRD location)
2. `~/.dotfile-sync.json` (legacy fallback)
3. Custom path via `-config` flag

Key configuration fields:
- `machine.name`: Identifier for this machine
- `git.repo_path`: Absolute path to dotfiles repository
- `git.remote_url`: Git remote URL (SSH or HTTPS)
- `git.auth_type`: Authentication strategy (ssh, basic, none)
- `sync.auto_sync_enabled`: Enable/disable automatic syncing
- `sync.debounce_seconds`: Delay after last change before syncing (default: 30)
- `sync.backoff`: Advanced backoff settings for churn detection
- `mappings`: Source-to-target file mappings for symlinks

Important: Config path is always tracked via `ConfigPath` field and should be accessed using `cfg.GetConfigPath()`

## Testing Strategy (🎯 **3-Layer Architecture**)

### Unit Tests (`*_test.go` in same package)
**Purpose**: Test individual functions and business logic in isolation (<100ms per test)

**When to Use**:
- ✅ Testing pure functions (debouncing, parsing, validation)
- ✅ Algorithm logic with no external dependencies
- ✅ Fast feedback during development
- ✅ Mock git/filesystem/network calls

**Examples**: `TestDebouncer_Trigger`, `TestIgnoreParser_ParsePattern`

**Commands**: `make test-unit`

### Integration Tests (`*_integration_test.go`)
**Purpose**: Test component interactions with real dependencies (1-5s per test)

**When to Use**:
- ✅ Testing SyncService + GitManager together
- ✅ Real file system operations (not mocked)
- ✅ Actual git repository operations in `/tmp`
- ✅ Component interaction validation

**Examples**: `TestSyncService_WithRealGit`, `TestGitManager_Integration`

**Commands**: `make test-integration`

### E2E Tests (`test/scenarios/*.go`)
**Purpose**: Complete user workflows from CLI (>5s per test)

**Critical Coverage Areas**:
- ✅ **`dsm add` workflow**: `TestScenario_DsmAddWorkflow` - **FULLY TESTED**
- ✅ **Editor integration**: `TestScenario_EditorBasicWorkflow` - **REAL EDITORS ONLY**
- ✅ **Basic sync workflows**: `TestScenario_BasicSyncWorkflow`
- ✅ **Conflict resolution**: `TestScenario_ConflictResolution`
- ✅ **File watching**: `TestScenario_FileWatching`
- ✅ **Cross-platform compatibility**: `TestScenario_CrossPlatformCompatibility`

**When to Use**:
- ✅ Testing complete CLI commands and user workflows
- ✅ Real editor functionality (`$EDITOR`, vim/nano/micro integration)
- ✅ Cross-platform compatibility validation
- ✅ Real git remotes and SSH authentication

**Key Principle**: **Editor functionality requires E2E testing - never mock editors**

**Primary Command**: `make test-all` - Runs all E2E scenarios
**Advanced Usage**: `./test/scripts/run-e2e.sh` - For specific scenarios or debugging

### Test Execution Guidelines

**During Development**:
```bash
# Fast feedback loop (<30s)
make test-unit

# Before committing (<2min)
make test-integration

# Before PR creation (<10min)
make test-all
```

**E2E Testing Rules**:
- E2E tests validate real user behavior, not implementation details
- `dsm add` workflow is fully tested - no "too interactive" excuses
- Editor functionality uses real editors in Docker environment
- Cross-platform tests ensure compatibility across macOS/Linux/Windows

**CI/CD Integration**:
- GitHub Actions runs unit + integration tests on every PR
- E2E tests run in parallel for complete workflow validation
- Coverage targets: Unit 85%, Integration 70%, E2E user scenarios
- Test isolation with `TEST_ID` and proper cleanup

### Coverage and Quality Goals

**Priority Areas**:
1. Git operations (target 75% coverage)
2. Process management (target 70% coverage)
3. CLI commands (target 80% coverage)

**Performance Targets**:
- Unit: <10ms average, 100ms max
- Integration: <500ms average, 2s max
- E2E: <10s average, 15min max (all 6 scenarios)

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`):
- Runs on push/PR to `main` and `develop` branches
- Jobs: test (unit + integration) → e2e-tests (full scenarios) → build
- Uses Go 1.23 with caching for faster builds
- Uploads coverage to Codecov
- Parallel test execution for performance

**Test Pipeline**:
```yaml
test:           # Unit + Integration (<10min)
  - make test-integration

e2e-tests:      # Full scenarios (<20min)
  - ./test/scripts/run-e2e.sh
  - depends: test
```

## Project Management

### Workflow Overview

This project uses GitHub issues, labels, and the "DSM Roadmap" project board for tracking work. All work follows phase-based development matching the PRD timeline.

### Branch Naming Convention

Create feature branches using the pattern: `phase-x/<short-slug>`

Examples:
- `phase-1/core-sync`
- `phase-2/conflict-resolution`

### Issue Naming Convention

Format: `Phase X: <Short title>`

Examples:
- "Phase 1: Implement file watcher"
- "Phase 2: Add conflict detection"

### Commit Messages

- Reference issue numbers using `Fixes #<n>` when completing work
- Keep commits focused and descriptive

### Issue Workflow

**Opening Issues:**
- Link to relevant PRD sections or documentation
- Assign appropriate `phase:` label (e.g., `phase: core sync`)
- Add to "DSM Roadmap" project board with `Status: Todo`
- Update `tasks/development_tasks.md` if task belongs in canonical checklist

**Working Issues:**
- Move to `Status: In Progress` when starting work
- Keep issue updated with findings, decisions, or blockers
- Limit to 10 or fewer items in progress at once

**Completing Issues:**
- Ensure PRs link back to the issue
- Verify all checks pass (`make test-all` required)
- Move to `Status: Done` only after merge and validation
- Close the issue and check off in `tasks/development_tasks.md`
- Create new issues for follow-up work instead of reopening

### Labels

- `phase:` - Indicates which development phase (e.g., `phase: core sync`)
- `needs clarification` - Flags scope changes, timeline risks, or missing requirements

### Board Hygiene

- Review board twice weekly for accuracy
- Use Phase filter to track progress against PRD timeline
- Keep no more than 10 items in progress

See `AGENTS.md` for complete workflow rules and details.

## Important Notes

### General
- Always use absolute paths in configuration (`git.repo_path`)
- The `.git` directory is always excluded from watching
- Auto-commits include timestamp and list of changed files
- Git operations use go-git library (not shell commands)
- SSH authentication requires proper known_hosts setup
- Binary is named `dot-sync-manager` but invoked as `dsm`

### Code Quality Guidelines
- Review `CODING_RULES.md` and `.gemini/styleguide.md` before making changes; treat them as the authoritative ruleset.
- Expand every user-provided path via `expandPaths()` and handle `~/` using `strings.TrimLeft(path[1:], "/\\")` so absolute paths are guaranteed.
- Ensure path expansion returns errors and wrap them with context (no silent fallbacks).
- Keep validation functions pure: use `cfg.GetConfigPath()` to respect flags and never mutate state inside `Validate()`.
- Resolve symlinks relative to their directory, preserve source file permissions, and avoid hardcoded binary names—use `os.Executable()`.
- Prefer `bufio.NewReader` for interactive input, extract magic numbers to named constants, and rely on helpers to remove duplication.
- In tests, call `t.Cleanup` (not `defer`) and check every error return.

### Testing Best Practices
- **Always run unit tests during development** (<30s for fast feedback)
- **Use E2E tests for CLI command validation** - they test real user behavior
- **Editor functionality requires E2E testing** - never mock editors, use real vim/nano/micro
- **`dsm add` workflow is fully tested** - skip the "too interactive" excuse, use E2E tests
- **Use `make test-all` before committing changes** to ensure complete validation
- **E2E tests validate user behavior, not implementation details** - focus on what users experience

### GitHub Operations
- Use the `gh` CLI for GitHub automation and avoid raw API calls, pairing it with `bin/review_report.sh <url>` for PR review summaries.
- Common `gh` commands:
  ```bash
  gh pr view <number>                    # View PR details
  gh pr view <number> --json reviews     # Get PR reviews
  gh api repos/:owner/:repo/pulls/:number/comments  # Get PR comments
  gh pr create --title "..." --body "..."  # Create PR
  gh issue list                          # List issues
  gh issue create --title "..." --body "..."  # Create issue
  ```
