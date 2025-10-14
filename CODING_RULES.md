# Coding Rules & Best Practices

Quick reference guide for maintaining code quality and consistency in this project.

## Path Handling Rules

1. **Always expand user paths early** - Normalize `~` and relative paths immediately after loading config
2. **Path expansion must return errors** - Never silently fail on `os.UserHomeDir()` or `filepath.Abs()` errors  
3. **Use `path[2:]` for `~/` prefix** - Not `path[1:]` which creates absolute paths incorrectly
4. **Expand ALL path fields consistently** - Create `expandPaths()` method to handle all path fields uniformly

## Validation Rules

5. **Separate normalization from validation** - Load → Normalize → Validate (not all in validate)
6. **Validation never mutates** - Only check state, don't modify during validation
7. **Use "must" not "should"** - Validation errors are requirements: "must be X" not "should be X"
8. **Extract magic numbers** - Use named constants: `const minValue = 60`

## Code Quality Rules

10. **Reduce duplication with helpers** - Extract repeated validation patterns into helper functions
11. **Refactor to loops** - Replace repeated if/else blocks with loop over slice/map
12. **Remove unnecessary wrappers** - Don't create wrapper functions that just forward calls
13. **Wrap errors with context** - Use `fmt.Errorf("operation failed: %w", err)` pattern
14. **Document actual behavior** - Comments must reflect reality, especially with JSON tags like `omitempty`
15. **Move large literals to constants** - Multi-line strings (>10 lines) belong at package level, not in functions
16. **Honest function signatures** - Don't return error types that are never actually used or always nil

## Testing Rules

17. **Use `t.Cleanup` not `defer`** - Modern Go testing: cleanup runs after all subtests
18. **Check all error returns** - Tests must check `os.Setenv`, `os.Mkdir`, etc. return values (and in error handling too!)
19. **Test post-normalization state** - Use absolute paths in tests when testing validation

## Architecture Principles

20. **Separation of concerns** - Normalize → Validate → Save/Use (three distinct stages)
21. **Fail fast** - Return errors early, don't continue with invalid state
22. **Group related operations** - Keep related path expansions, validations together with comments
23. **Design for optionality** - Make fields/parameters optional when legitimate use cases exist without them
24. **Guard optional operations** - Check preconditions before calling functions that depend on optional config

## Quick Examples

### ✅ Good Path Handling
```go
func (c *Config) expandPaths() error {
    expand := func(path *string, name string) error {
        if *path == "" { return nil }
        expanded, err := util.ExpandPath(*path)
        if err != nil {
            return fmt.Errorf("failed to expand %s: %w", name, err)
        }
        *path = expanded
        return nil
    }
    if err := expand(&c.RepoPath, "repo_path"); err != nil { return err }
    return nil
}
```

### ✅ Good Validation
```go
func (c *Config) Validate() error {
    if !filepath.IsAbs(c.RepoPath) {
        return fmt.Errorf("repo path must be absolute, got '%s'", c.RepoPath)
    }
    if c.PullInterval < minPullIntervalSeconds {
        return fmt.Errorf("pull interval must be at least %d seconds", minPullIntervalSeconds)
    }
    return nil
}
```

### ✅ Good Testing
```go
func TestValidation(t *testing.T) {
    if err := os.Setenv("HOME", "/test"); err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    t.Cleanup(func() {
        os.Setenv("HOME", originalHome)
    })
    // test code
}
```

## Common Mistakes to Avoid

### ❌ Don't Mix Normalization and Validation
```go
// BAD
func (c *Config) Validate() error {
    c.RepoPath = util.ExpandPath(c.RepoPath)  // Mutating during validation!
    return nil
}
```

### ❌ Don't Ignore Path Expansion Errors
```go
// BAD
func ExpandPath(path string) string {
    homeDir, _ := os.UserHomeDir()  // Ignoring error!
    return filepath.Join(homeDir, path[1:])  // And using wrong index!
}
```

### ❌ Don't Fall Back on Path Resolution Failures
```go
// BAD
absPath, err := filepath.Abs(filename)
if err != nil {
    absPath = filename  // Silent fallback - could be relative!
}
config.Path = absPath  // Other code expects absolute path
```

### ❌ Don't Use Weak Validation Language
```go
// BAD
if value < 60 {
    return errors.New("value should be at least 60")  // "should" is too weak
}
```

### ❌ Don't Write Misleading Comments About omitempty
```go
// BAD
// Validate only if explicitly set (omitempty)
if c.TimeoutSeconds < 0 {  // With int, can't distinguish omitted from 0!
    return errors.New("timeout cannot be negative")
}

// GOOD
// If not set (or is 0), a default of 10 seconds is used.
// A negative value is invalid.
if c.TimeoutSeconds < 0 {
    return errors.New("timeout cannot be negative")
}
```

### ❌ Don't Embed Large String Literals in Functions
```go
// BAD - 37 lines embedded in function
func createIgnoreFile() error {
    content := `# Line 1
# Line 2
... 35 more lines ...
`
    return os.WriteFile(path, []byte(content), 0644)
}

// GOOD - Package-level constant
const defaultIgnoreContent = `# Line 1
# Line 2
... 35 more lines ...
`

func createIgnoreFile() error {
    return os.WriteFile(path, []byte(defaultIgnoreContent), 0644)
}
```

### ❌ Don't Return Errors That Are Never Used
```go
// BAD - Error return never populated
func getMachineName() (string, error) {
    if hostname, err := os.Hostname(); err == nil {
        return hostname, nil
    }
    return "unknown-machine", nil  // Always returns nil error!
}

// GOOD - Honest signature
func getMachineName() string {
    if hostname, err := os.Hostname(); err == nil {
        return hostname
    }
    return "unknown-machine"
}
```

### ❌ Don't Ignore Errors in Error Handling
```go
// BAD - Error ignored when building error message
homeDir, _ := os.UserHomeDir()  // Ignored!
return fmt.Errorf("config not found at: %s", homeDir)  // Could be empty!

// GOOD - Check error even in error paths
homeDir, homeErr := os.UserHomeDir()
if homeErr != nil {
    return fmt.Errorf("config not found (unable to determine home: %v)", homeErr)
}
return fmt.Errorf("config not found at: %s", homeDir)
```

### ❌ Don't Make Fields Required When They're Optional
```go
// BAD - Always requires remote even for local-only use
func (c *Config) validate() error {
    if c.RemoteURL == "" {
        return errors.New("remote URL is required")  // Blocks local-only!
    }
}

// GOOD - Optional field with guarded operations
func (c *Config) validate() error {
    // RemoteURL is optional - local-only repos don't need it
    if c.RemoteURL != "" && c.RemoteName == "" {
        c.RemoteName = "origin"
    }
}

// Guard dependent operations
if gm.cfg.RemoteURL != "" {
    return gm.ensureRemote()  // Only when configured
}
```

## Review Checklist

Before submitting code, verify:

- [ ] All paths expanded via dedicated `expandPaths()` method
- [ ] Path expansion returns errors properly (never silent fallback)
- [ ] `~/path` uses `path[2:]` not `path[1:]`
- [ ] Path resolution errors propagated immediately (fail fast)
- [ ] Validation only checks, doesn't modify
- [ ] Error messages use "must" language
- [ ] Magic numbers/strings extracted to constants
- [ ] Large multi-line strings (>10 lines) moved to package constants
- [ ] Function signatures don't return unused errors
- [ ] Helper functions reduce duplication
- [ ] Tests use `t.Cleanup` for setup/teardown
- [ ] All error returns checked (even in error handling code)
- [ ] Comments accurately reflect behavior (especially with `omitempty`)
- [ ] Optional fields designed for legitimate use cases
- [ ] Optional operations guarded with precondition checks
- [ ] Clear separation: normalize → validate → use

## References

- Detailed examples: `.gemini/styleguide.md`
- Go testing: `go doc testing.T.Cleanup`
- Error wrapping: `go doc fmt.Errorf`
