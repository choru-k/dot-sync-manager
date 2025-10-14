# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Dotfile Sync Manager (DSM) is a Go application that automatically syncs dotfiles to a git repository using file system watching. It monitors changes to local dotfiles and automatically commits and pushes them to a remote repository, enabling seamless multi-machine dotfile management.

## Common Commands

### Build and Run
```bash
# Build the main application
go build -v -o bin/dotfile-sync-manager .

# Run the application
./bin/dotfile-sync-manager -config ~/.dotfile-sync.json

# Run with verbose logging
./bin/dotfile-sync-manager -config ~/.dotfile-sync.json -verbose

# Show version
./bin/dotfile-sync-manager -version
```

### Testing
```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...

# Run tests for a specific package
go test -v ./internal/gitmanager/
go test -v ./internal/sync/

# Run a single test
go test -v -run TestGitManager_StageCommitAndPush ./internal/gitmanager/
```

### Development
```bash
# Verify dependencies
go mod verify

# Download dependencies
go mod download

# Tidy dependencies
go mod tidy

# Run go vet
go vet ./...

# Build for specific platforms
GOOS=linux GOARCH=amd64 go build -v -o bin/dotfile-sync-manager-linux .
GOOS=darwin GOARCH=arm64 go build -v -o bin/dotfile-sync-manager-darwin .
```

## Architecture

### Core Components

1. **GitManager** (`internal/gitmanager/`)
   - Handles all git operations: clone, commit, push, pull
   - Manages authentication (SSH keys, HTTPS with credentials)
   - Implements stash-based conflict resolution for pull operations
   - Key files: `git_manager.go`, `auth.go`, `config.go`, `stash.go`

2. **SyncService** (`internal/sync/`)
   - Orchestrates file watching and automatic syncing
   - Uses fsnotify for file system event monitoring
   - Implements debouncing to avoid excessive commits
   - Recursively watches directories, excluding `.git` and ignored paths
   - Key file: `service.go`

3. **Configuration** (`internal/config/`)
   - Manages user configuration from JSON file (default: `~/.dotfile-sync.json`)
   - Includes machine identification, git settings, sync behavior, notifications
   - Key file: `sync_config.go`

4. **Debouncer** (`internal/debouncer/`)
   - Thread-safe timer-based debouncing mechanism
   - Delays sync operations until after a period of inactivity (default: 30 seconds)
   - Key file: `debouncer.go`

5. **Ignore Parser** (`internal/ignore/`)
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

The application expects a JSON configuration file at `~/.dotfile-sync.json` (or specified via `-config` flag). Key fields:

- `machine.name`: Identifier for this machine
- `git.repo_path`: Absolute path to dotfiles repository
- `git.remote_url`: Git remote URL (SSH or HTTPS)
- `git.auth_type`: "ssh" or "basic" authentication
- `sync.auto_sync_enabled`: Enable/disable automatic syncing
- `sync.debounce_seconds`: Delay after last change before syncing (default: 30)

## Testing Strategy

- **Unit tests**: Test individual components in isolation (e.g., debouncer, ignore parser)
- **Integration tests**: Test GitManager operations against real git repos in `/tmp`
- **Service tests**: Test SyncService with mock watchers and temporary directories
- Use table-driven tests where appropriate for multiple test cases
- Coverage is tracked and uploaded to Codecov via CI

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`):
- Runs on push/PR to `main` and `develop` branches
- Jobs: test (go vet, tests with coverage) → build (compile binary, upload artifact)
- Uses Go 1.23 with caching for faster builds
- Uploads coverage to Codecov

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
- Verify all checks pass
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

- Always use absolute paths in configuration (`git.repo_path`)
- The `.git` directory is always excluded from watching
- Auto-commits include timestamp and list of changed files
- Git operations use go-git library (not shell commands)
- SSH authentication requires proper known_hosts setup
- Main entry point is `main.go`, but there's also `cmd/dotfile-sync-manager/main.go` (example/alternative)
