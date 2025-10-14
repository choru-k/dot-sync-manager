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

9. **Reduce duplication with helpers** - Extract repeated validation patterns into helper functions
10. **Refactor to loops** - Replace repeated if/else blocks with loop over slice/map
11. **Remove unnecessary wrappers** - Don't create wrapper functions that just forward calls
12. **Wrap errors with context** - Use `fmt.Errorf("operation failed: %w", err)` pattern

## Testing Rules

13. **Use `t.Cleanup` not `defer`** - Modern Go testing: cleanup runs after all subtests
14. **Check all error returns** - Tests must check `os.Setenv`, `os.Mkdir`, etc. return values
15. **Test post-normalization state** - Use absolute paths in tests when testing validation

## Architecture Principles

16. **Separation of concerns** - Normalize → Validate → Save/Use (three distinct stages)
17. **Fail fast** - Return errors early, don't continue with invalid state
18. **Group related operations** - Keep related path expansions, validations together with comments

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

### ❌ Don't Use Weak Validation Language
```go
// BAD
if value < 60 {
    return errors.New("value should be at least 60")  // "should" is too weak
}
```

## Review Checklist

Before submitting code, verify:

- [ ] All paths expanded via dedicated `expandPaths()` method
- [ ] Path expansion returns errors properly
- [ ] `~/path` uses `path[2:]` not `path[1:]`
- [ ] Validation only checks, doesn't modify
- [ ] Error messages use "must" language
- [ ] Magic numbers extracted to constants
- [ ] Helper functions reduce duplication
- [ ] Tests use `t.Cleanup` for setup/teardown
- [ ] All error returns checked in tests
- [ ] Clear separation: normalize → validate → use

## References

- Detailed examples: `.gemini/styleguide.md`
- Go testing: `go doc testing.T.Cleanup`
- Error wrapping: `go doc fmt.Errorf`
