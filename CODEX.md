# CODEX Guide for Dotfile Sync Manager

## Overview

This guide provides concise workflow instructions for Codex agents working on the Dotfile Sync Manager (DSM) project. DSM automatically syncs dotfiles to a git repository using file system watching.

## Daily Triage & Board Hygiene

- **Check notifications** and add new issues to `DSM Roadmap` board
- **Assign Phase field** matching PRD timeline, set `Status: Todo` unless work started
- **Apply `phase:` labels** (e.g., `phase: core sync`) to all issues
- **Update `tasks/development_tasks.md`** for canonical checklist items
- **Review board twice weekly** for status accuracy and assignments
- **Keep ≤10 items in `In Progress`** - negotiate hand-offs or park items
- **Use Phase filter** to validate progress against PRD timeline

## Task Planning & Execution

**IMPORTANT: Activate the Serena project context before any other action.** Use the provided Serena integration to ensure all work is properly tracked and coordinated with project management systems.

### Planning Requirements
- **Read issue thoroughly** and link to relevant PRD sections
- **Create feature branch**: `phase-x/<short-slug>` referencing issue number
- **Break work into droid exec commands** - never edit files directly
- **Update issue** with findings, decisions, or blockers

### **IMPORTANT** Droid Exec Pattern
- Split work into atomic commands that can be executed independently
- Each command should have clear purpose and verification steps
- Use descriptive command names matching the task
- Include `droid.md` alongside your prompt whenever running `droid exec`
- Never manually edit files - always use droid commands

#### Running `droid exec`
- Base syntax: `droid exec [options] [prompt]` or pipe stdin (e.g., ``echo "summarize repo" | droid exec``)
- Always start prompts from `droid.md` plus task-specific context; favor `droid exec -f prompt.md` for longer briefs
- Default to `--auto high` for DSM work so end-to-end flows (tests → commit → push/deploy) succeed without extra reruns; consciously drop to lower levels when the task truly stays local
- Know the autonomy envelope before launching:
  - default (no flags) stays read-only
  - `--auto low` covers safe read/write tasks with minimal side-effects
  - `--auto medium` unlocks routine dev flows (installs, builds, local git operations)
  - `--auto high` enables production-impacting actions (e.g., running untrusted scripts, opening ports, `git push`, migrations, handling secrets) and still blocks destructive commands like `sudo rm -rf /`
- `--skip-permissions-unsafe` removes all safeguards and must only run inside disposable sandboxes; it cannot be combined with any `--auto` flag
- Prefer `--session-id` to continue an existing run only when explicitly coordinating with teammates; otherwise each exec should stay isolated
- Capture outputs (logs, artifacts) immediately after the command finishes—`droid exec` exits once the task is complete
- Note: This command can be long-running. Allow it to complete without interruption.

## PR Review Handling

### Using review_report.sh
```bash
# Generate review report from GitHub URL
bin/review_report.sh https://github.com/choru-k/dot-sync-manager/pull/<n>

# Accepts multiple URLs or stdin
cat urls.txt | bin/review_report.sh
```

### Review Process
- **Address each review issue** in separate commits with descriptive messages
- **Group related fixes** (e.g., all "magic numbers" in one commit)
- **Reference review IDs** in commit messages
- **Re-run full test suite** after all fixes
- **Request Gemini review**: `/gemini review` in PR comments

## Critical Coding Standards

Comprehensive coding standards are defined in CODING_RULES.md and .gemini/styleguide.md - this section highlights the most critical patterns for daily development.

### Path Handling
- **Expand all user paths early** via dedicated `expandPaths()` method
- **Path expansion must return errors** - never silent fallback
- **Use `strings.TrimLeft(path[1:], "/\\")` for `~/` prefix** (not `path[2:]`)
- **Propagate path resolution errors** immediately (fail fast)

### Validation Patterns
- **Separate normalization → validation → use** stages
- **Validation never mutates** state (only checks)
- **Use "must" not "should"** in error messages
- **Extract magic numbers** to named constants
- **Use maps for validation lookups** instead of loops (O(1) performance)

### Configuration Management
- **Use `DefaultConfig()`** instead of manual struct creation
- **Check file existence before overwriting** (preserve user data)
- **Require confirmation for destructive operations**
- **Preserve existing config values** when updating (don't clobber auth/settings)
- **Extract file permissions to constants** (0600, 0644, etc.)

### Code Quality
- **Wrap errors with context**: `fmt.Errorf("operation failed: %w", err)`
- **Use raw string literals** for multi-line messages
- **Add godoc to all exported functions**
- **Mark stubs with TODO(PRx) comments**
- **Use `t.Cleanup` not `defer` in tests**
- **Check all error returns** in tests and error handling

## Development Workflow

### Required Commands Before Finishing
```bash
# Run all tests
go test ./...

# Run linter (if available)
golangci-lint run

# Self-review against coding rules checklist
# Request Gemini review in PR: /gemini review
```

### Pre-Commit Verification
- [ ] All paths expanded via `expandPaths()` method
- [ ] Path expansion returns errors properly
- [ ] `~/path` uses `strings.TrimLeft(path[1:], "/\\")` not `path[2:]`
- [ ] Validation only checks, doesn't modify
- [ ] Error messages use "must" language
- [ ] Magic numbers extracted to constants
- [ ] Used `DefaultConfig()` instead of duplication
- [ ] Tests use `t.Cleanup` for setup/teardown
- [ ] All error returns checked
- [ ] Godoc on exported functions
- [ ] Config updates preserve existing values

## Finalization Steps

### Issue Management
- **Ensure PRs link back to issue** and have passing checks
- **Move project item to `Status: Done`** only after merge and validation
- **Close issue and check off** in `tasks/development_tasks.md`
- **Create follow-up issues** instead of reopening completed work

### Post-Merge Validation
- **Verify tests pass**: `go test ./...`
- **Confirm build succeeds**: `go build -v -o bin/dsm .`
- **Check binary functionality**: `./bin/dsm -version`
- **Update documentation** if behavior changed

### Escalations
- **Flag scope changes, timeline risks, or missing requirements**
- **Create issue labeled `needs clarification`** tagging product owner
- **Document unresolved questions** with PRD section links

## Architecture Reference

### Core Components
- **GitManager**: Git operations (clone, commit, push, pull, auth)
- **SyncService**: File watching and orchestration
- **Configuration**: JSON config management (~/.dotfile-sync.json)
- **Debouncer**: Timer-based debouncing for sync operations
- **Ignore Parser**: .syncignore file parsing (gitignore-style)

### Key Patterns
- **Context-based cancellation** for graceful shutdown
- **Event-driven sync** with debouncing
- **Stash-based conflict resolution**
- **Callback-based notifications**

## Quick Commands Reference

```bash
# Build
go build -v -o bin/dsm .

# Run
./bin/dsm -config ~/.dotfile-sync.json -verbose

# Test
go test -v ./...
go test -v -coverprofile=coverage.out ./...

# Dependencies
go mod tidy
go mod verify
go vet ./...
```
