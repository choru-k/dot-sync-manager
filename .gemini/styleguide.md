# Dotfile Sync Manager - Go Style Guide

## Project Context
Dotfile Sync Manager (DSM) is a Go application that automatically syncs dotfiles to a git repository using file system watching. This style guide defines coding conventions specific to this project.

## Core Principles
- **Context-based cancellation**: All long-running operations accept `context.Context` for graceful shutdown
- **Separation of concerns**: Validation functions should never mutate state, only check and return errors
- **Absolute paths**: Always use absolute paths in configuration and file operations
- **Cross-platform compatibility**: Code must work on Unix/Linux, macOS, and Windows

## Architecture Patterns

### Context Management
All services and long-running operations must accept and respect `context.Context`:
```go
func (s *SyncService) Watch(ctx context.Context) error {
    // Respect context cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    case event := <-s.watcher.Events:
        // Handle event
    }
}
```

### Configuration Handling
- **Always use `cfg.GetConfigPath()`** to respect the `--config` flag
- Never hardcode config paths
- Use PRD-compliant path discovery: `~/dotfiles/.sync-config.json` (primary), `~/.dotfile-sync.json` (legacy)

### Validation Functions
Validation functions must be pure - they should only check conditions and return errors:
```go
// GOOD: Pure validation
func validateRepoPath(path string) error {
    if !filepath.IsAbs(path) {
        return fmt.Errorf("repo_path must be absolute")
    }
    return nil
}

// BAD: Don't mutate state in validation
func validateRepoPath(path string) error {
    globalState.repoPath = path // NO!
    return nil
}
```

### Path Handling
- **Symlinks**: Relative symlinks must be resolved relative to the symlink's directory before converting to absolute paths
- **Windows compatibility**: Handle volume names and drive letters correctly
- **Process detection**: Use `os.Executable()` to get actual binary name, not hardcoded values

### File Operations
- Preserve source file permissions when copying: use `sourceInfo.Mode()`
- Handle cross-drive symlinks on Windows appropriately
- Always check for file existence before operations

### User Input
- Use `bufio.NewReader` instead of `fmt.Scanln` for robust input handling
- Validate email addresses using RFC 5322 compliant parsing
- Provide clear prompts and error messages

## Error Handling
- Return descriptive errors with context
- Use `fmt.Errorf` with `%w` for error wrapping
- Log errors at appropriate levels (Debug, Info, Warning, Error)
- Handle edge cases explicitly (empty files, missing directories, etc.)

## Git Operations
- Use go-git library, not shell commands
- Implement stash-based conflict resolution for pull operations
- Support both SSH and HTTPS authentication
- Auto-commits should include timestamp and list of changed files

## Testing Strategy
- **Unit tests**: Test individual components in isolation
- **Integration tests**: Use real git repos in `/tmp` for GitManager tests
- **Table-driven tests**: Use for multiple test cases
- Test both success and error paths
- Mock file system operations where appropriate

## File Watching
- Recursively watch directories
- Always exclude `.git` directory from watching
- Respect `.syncignore` files (gitignore-style syntax)
- Implement debouncing to avoid excessive commits (default: 30 seconds)
- Use fsnotify for cross-platform file system event monitoring

## Code Organization
- CLI commands in `cmd/` (Cobra-based)
- Core logic in `internal/` packages
- One command per file (e.g., `add.go`, `list.go`)
- Keep functions focused and single-purpose

## Security Considerations
- Never expose sensitive data in logs or error messages
- Validate all user input
- Check SSH known_hosts for authentication
- Handle credentials securely
- Avoid path traversal vulnerabilities

## Performance
- Use context-based timeouts for network operations
- Implement exponential backoff for retries
- Use buffered channels appropriately
- Avoid excessive file system polling

## Comments and Documentation
- Document exported functions and types
- Explain "why" in comments, not "what"
- Keep comments up to date with code changes
- Document non-obvious behavior or edge cases

## Common Anti-Patterns to Avoid
- Don't hardcode binary names; use `os.Executable()`
- Don't ignore errors from `defer` operations
- Don't use `panic` for normal error conditions
- Don't modify global state from validation functions
- Don't use shell commands when library functions exist

## Commit and PR Guidelines
- Reference issue numbers using `Fixes #<n>`
- Keep commits focused and descriptive
- Run tests and linters before committing
- Ensure all checks pass in CI/CD
- Follow branch naming: `phase-x/<short-slug>`
