# Go Coding Style Guide for Dotfile Sync Manager

This style guide documents the coding standards and best practices for this project, derived from code reviews and learned patterns.

## Table of Contents
1. [Path Handling](#path-handling)
2. [Error Handling](#error-handling)
3. [Configuration Management](#configuration-management)
4. [Validation Patterns](#validation-patterns)
5. [Testing Standards](#testing-standards)
6. [Code Organization](#code-organization)

## Path Handling

### Rule 1: Always Expand User-Provided Paths Early
**Context**: Users often provide paths with `~` or relative paths. These must be normalized to absolute paths.

**✅ DO:**
```go
// Expand paths immediately after loading/unmarshaling
func LoadFromFile(filename string) (*Config, error) {
    config := &Config{}
    json.Unmarshal(data, config)
    
    // Expand all paths right after loading
    if err := config.expandPaths(); err != nil {
        return nil, err
    }
    
    // Then validate
    if err := config.Validate(); err != nil {
        return nil, err
    }
    return config, nil
}
```

**❌ DON'T:**
```go
// Don't expand paths during validation
func (c *Config) Validate() error {
    // BAD: Mixing expansion with validation
    expanded, _ := util.ExpandPath(c.RepoPath)
    if !filepath.IsAbs(expanded) {
        return errors.New("not absolute")
    }
}
```

**Rationale**: Separation of concerns - normalization happens first, then validation checks the normalized data.

### Rule 2: Path Expansion Functions Must Return Errors
**Context**: Path expansion can fail (e.g., `os.UserHomeDir()` fails, invalid paths).

**✅ DO:**
```go
func ExpandPath(path string) (string, error) {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("failed to get home directory: %w", err)
    }
    return filepath.Join(homeDir, path[2:]), nil
}
```

**❌ DON'T:**
```go
// Don't silently swallow errors
func ExpandPath(path string) string {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return path // Silently returns unexpanded path!
    }
    return filepath.Join(homeDir, path[2:])
}
```

**Rationale**: Callers need to know if expansion failed to handle errors appropriately.

### Rule 3: Handle Tilde Expansion Correctly
**Context**: `~/path` requires special handling - must use `path[2:]` not `path[1:]`.

**✅ DO:**
```go
if path == "~" || strings.HasPrefix(path, "~/") {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    if path == "~" {
        return homeDir, nil
    }
    return filepath.Join(homeDir, path[2:]), nil  // path[2:] skips "~/"
}
```

**❌ DON'T:**
```go
if strings.HasPrefix(path, "~/") {
    // BAD: path[1:] gives "/path" which is treated as absolute!
    return filepath.Join(homeDir, path[1:])
}
```

**Rationale**: `path[1:]` on `~/path` yields `/path` which `filepath.Join` treats as absolute, discarding the home directory.

### Rule 4: Expand ALL Path Fields Consistently
**Context**: Configuration often has multiple path fields.

**✅ DO:**
```go
func (c *Config) expandPaths() error {
    expand := func(path *string, name string) error {
        if *path == "" {
            return nil
        }
        expanded, err := util.ExpandPath(*path)
        if err != nil {
            return fmt.Errorf("failed to expand %s: %w", name, err)
        }
        *path = expanded
        return nil
    }
    
    // Expand all path fields
    if err := expand(&c.Git.RepoPath, "git.repo_path"); err != nil { return err }
    if err := expand(&c.Git.SSHKeyPath, "git.ssh_key_path"); err != nil { return err }
    if err := expand(&c.Advanced.LogFile, "advanced.log_file"); err != nil { return err }
    
    // Expand maps
    for key, path := range c.Mappings {
        expanded, err := util.ExpandPath(path)
        if err != nil {
            return fmt.Errorf("failed to expand mapping '%s': %w", key, err)
        }
        c.Mappings[key] = expanded
    }
    return nil
}
```

**Rationale**: Consistency - all paths should be handled the same way.

## Error Handling

### Rule 5: Use Strong "Must" Language in Validation
**Context**: Validation errors are hard requirements, not suggestions.

**✅ DO:**
```go
if value < minValue {
    return fmt.Errorf("field must be at least %d", minValue)
}
```

**❌ DON'T:**
```go
if value < minValue {
    return fmt.Errorf("field should be at least %d", minValue)  // Too weak
}
```

**Rationale**: "Should" implies optional; "must" indicates requirement.

### Rule 6: Wrap Errors with Context
**Context**: Error messages should be traceable.

**✅ DO:**
```go
if err := util.ExpandPath(path); err != nil {
    return fmt.Errorf("failed to expand %s: %w", fieldName, err)
}
```

**❌ DON'T:**
```go
if err := util.ExpandPath(path); err != nil {
    return err  // Lost context
}
```

**Rationale**: Error wrapping preserves stack and adds context.

### Rule 7: Fail Fast on Path Resolution Errors
**Context**: When resolving paths to absolute, errors should not be silently ignored.

**✅ DO:**
```go
absPath, err := filepath.Abs(filename)
if err != nil {
    return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", filename, err)
}
config.ConfigPath = absPath  // Guaranteed absolute
```

**❌ DON'T:**
```go
absPath, err := filepath.Abs(filename)
if err != nil {
    absPath = filename  // Silent fallback - could be relative!
}
config.ConfigPath = absPath  // Might not be absolute!
```

**Rationale**: Other code may assume `ConfigPath` is always absolute. Failing fast prevents downstream issues.

## Configuration Management

### Rule 7: Separate Normalization from Validation
**Context**: Loading config involves multiple stages.

**✅ DO:**
```go
// Stage 1: Parse
json.Unmarshal(data, config)

// Stage 2: Normalize
config.expandPaths()

// Stage 3: Validate
config.Validate()
```

**❌ DON'T:**
```go
// Don't mix stages
json.Unmarshal(data, config)
config.Validate()  // Tries to validate and normalize at once
```

**Rationale**: Single Responsibility Principle - each stage has one job.

### Rule 8: Extract Magic Numbers to Constants
**Context**: Hardcoded values make code hard to maintain.

**✅ DO:**
```go
const (
    minPullIntervalSeconds = 60
    maxBackupRetentionDays = 365
    maxLogSizeMB           = 1000
)

if value < minPullIntervalSeconds {
    return fmt.Errorf("must be at least %d", minPullIntervalSeconds)
}
```

**❌ DON'T:**
```go
if value < 60 {  // What does 60 mean?
    return fmt.Errorf("must be at least 60")
}
```

**Rationale**: Named constants are self-documenting and easier to maintain.

### Rule 9: Use Helper Functions to Reduce Duplication
**Context**: Validation often has repeated patterns.

**✅ DO:**
```go
func validateInclusion(value string, options []string, fieldName string) error {
    for _, opt := range options {
        if value == opt {
            return nil
        }
    }
    return fmt.Errorf("invalid %s: %s (must be one of: %v)", fieldName, value, options)
}

// Usage
if err := validateInclusion(c.Strategy, []string{"manual", "auto"}, "strategy"); err != nil {
    return err
}
```

**❌ DON'T:**
```go
// Repeated validation logic
valid := false
for _, opt := range []string{"manual", "auto"} {
    if c.Strategy == opt {
        valid = true
        break
    }
}
if !valid {
    return errors.New("invalid strategy")
}
```

**Rationale**: DRY principle - helper functions reduce code duplication.

### Rule 10: Refactor Repetitive Patterns Into Loops
**Context**: Checking multiple locations, paths, etc.

**✅ DO:**
```go
searchPaths := []string{
    filepath.Join(homeDir, "dotfiles", ".sync-config.json"),
    filepath.Join(homeDir, ".dotfile-sync.json"),
}

for _, path := range searchPaths {
    if _, err := os.Stat(path); err == nil {
        return path, true, nil
    }
}
```

**❌ DON'T:**
```go
// Repeated if/else blocks
path1 := filepath.Join(homeDir, "dotfiles", ".sync-config.json")
if _, err := os.Stat(path1); err == nil {
    return path1, true, nil
}

path2 := filepath.Join(homeDir, ".dotfile-sync.json")
if _, err := os.Stat(path2); err == nil {
    return path2, true, nil
}
```

**Rationale**: Loop patterns are more maintainable when adding new items.

## Validation Patterns

### Rule 11: Validation Should Not Mutate State
**Context**: Validation checks should be read-only.

**✅ DO:**
```go
func (c *Config) Validate() error {
    // Only check, don't modify
    if !filepath.IsAbs(c.RepoPath) {
        return fmt.Errorf("repo path must be absolute")
    }
    return nil
}
```

**❌ DON'T:**
```go
func (c *Config) Validate() error {
    // BAD: Modifying during validation
    c.RepoPath = util.ExpandPath(c.RepoPath)
    if !filepath.IsAbs(c.RepoPath) {
        return errors.New("not absolute")
    }
    return nil
}
```

**Rationale**: Validation should only verify, not transform. Normalization happens separately.

## Testing Standards

### Rule 12: Use t.Cleanup Instead of defer
**Context**: Modern Go (1.14+) testing best practices.

**✅ DO:**
```go
func TestSomething(t *testing.T) {
    os.Setenv("VAR", "value")
    t.Cleanup(func() {
        os.Setenv("VAR", originalValue)
    })
    // test code
}
```

**❌ DON'T:**
```go
func TestSomething(t *testing.T) {
    os.Setenv("VAR", "value")
    defer os.Setenv("VAR", originalValue)  // Runs before subtests complete
    // test code
}
```

**Rationale**: `t.Cleanup` runs after all subtests, `defer` runs immediately after parent test.

### Rule 13: Check Error Returns in Tests
**Context**: Linters flag unchecked errors.

**✅ DO:**
```go
if err := os.Setenv("HOME", testHome); err != nil {
    t.Fatalf("Failed to set HOME: %v", err)
}
```

**❌ DON'T:**
```go
os.Setenv("HOME", testHome)  // Error ignored
```

**Rationale**: Tests should be explicit about setup failures.

### Rule 14: Test Against Post-Normalization State
**Context**: When testing validation of normalized data.

**✅ DO:**
```go
func TestValidation(t *testing.T) {
    c := DefaultConfig()
    // Use absolute paths as they would be after expandPaths()
    homeDir, _ := os.UserHomeDir()
    c.Mappings = map[string]string{
        "bashrc": filepath.Join(homeDir, ".bashrc"),
    }
    err := c.Validate()
    // assertions
}
```

**❌ DON'T:**
```go
func TestValidation(t *testing.T) {
    c := DefaultConfig()
    c.Mappings = map[string]string{
        "bashrc": "~/.bashrc",  // Pre-expansion state
    }
    err := c.Validate()  // Will fail if validation expects absolute
}
```

**Rationale**: Tests should match the actual runtime state of the data.

## Code Organization

### Rule 15: Remove Unnecessary Wrapper Functions
**Context**: Wrapper functions that just forward calls add no value.

**✅ DO:**
```go
// Call directly
path, err := util.ExpandPath(explicitPath)
```

**❌ DON'T:**
```go
// Unnecessary wrapper
func expandPath(path string) (string, error) {
    return util.ExpandPath(path)
}

path, err := expandPath(explicitPath)
```

**Rationale**: Simpler code is better. Direct calls are clearer.

### Rule 16: Group Related Operations
**Context**: Operations on the same data should be grouped.

**✅ DO:**
```go
// Expand Git-related paths
if err := expand(&c.Git.RepoPath, "git.repo_path"); err != nil { return err }
if err := expand(&c.Git.SSHKeyPath, "git.ssh_key_path"); err != nil { return err }

// Expand config paths
if err := expand(&c.Advanced.LogFile, "advanced.log_file"); err != nil { return err }
```

**❌ DON'T:**
```go
// Random ordering
if err := expand(&c.Git.RepoPath, "git.repo_path"); err != nil { return err }
if err := expand(&c.Advanced.LogFile, "advanced.log_file"); err != nil { return err }
if err := expand(&c.Git.SSHKeyPath, "git.ssh_key_path"); err != nil { return err }
```

**Rationale**: Logical grouping improves code readability.

### Rule 17: Comment Accuracy with JSON Tags
**Context**: Comments about optional fields with `omitempty` can be misleading with primitive types.

**✅ DO:**
```go
// If manual sync timeout is not set (or is 0), a default of 10 seconds is used.
// A negative value is invalid.
ManualSyncTimeoutSeconds int `json:"manual_sync_timeout_seconds,omitempty"`

// Validation
if c.ManualSyncTimeoutSeconds < 0 {
    return fmt.Errorf("timeout cannot be negative")
}

// Usage with default
timeout := 10 * time.Second  // default
if c.ManualSyncTimeoutSeconds > 0 {
    timeout = time.Duration(c.ManualSyncTimeoutSeconds) * time.Second
}
```

**❌ DON'T:**
```go
// Validate only if explicitly set (since it's optional with omitempty)
ManualSyncTimeoutSeconds int `json:"manual_sync_timeout_seconds,omitempty"`

// MISLEADING: Can't distinguish between omitted and explicit 0 with int type!
if c.ManualSyncTimeoutSeconds < 0 {
    return fmt.Errorf("timeout cannot be negative")
}
```

**Rationale**: With `int` + `omitempty`, both omitted and explicit 0 appear as 0 in the struct. Comments must reflect this reality. Use `*int` if you need to distinguish nil from 0.

## Summary Checklist

When writing configuration-related code, ensure:

- [ ] All path fields are expanded consistently in a dedicated method
- [ ] Path expansion returns errors, not silently failing
- [ ] Tilde expansion uses `path[2:]` for `~/` prefix
- [ ] Path resolution errors propagated immediately (no silent fallbacks)
- [ ] Validation only checks, never modifies state
- [ ] Validation messages use "must" not "should"
- [ ] Magic numbers are extracted to named constants
- [ ] Duplicated validation logic uses helper functions
- [ ] Tests use `t.Cleanup` instead of `defer`
- [ ] Tests check error returns from setup functions
- [ ] Error messages include context via wrapping
- [ ] Comments accurately reflect behavior (especially with `omitempty`)
- [ ] Separation of concerns: normalize → validate → use
