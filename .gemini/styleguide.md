# Go Coding Style Guide for Dotfile Sync Manager

This style guide documents the coding standards and best practices for this project, derived from code reviews and learned patterns.

## Table of Contents
1. [Path Handling](#path-handling)
2. [Error Handling](#error-handling)
3. [Configuration Management](#configuration-management)
4. [Validation Patterns](#validation-patterns)
5. [Testing Standards](#testing-standards)
6. [Code Organization](#code-organization)
7. [User Experience](#user-experience)

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

// ALSO BAD: Setting defaults during validation
func (c *Config) validate() error {
    // This is normalization, not validation!
    if c.RemoteURL != "" && c.RemoteName == "" {
        c.RemoteName = "origin"  // Mutating state!
    }
    return nil
}
```

**Rationale**: Validation should only verify, not transform. Normalization happens separately. Mixing concerns makes code harder to reason about and test.

### Rule 12: Use Maps for Validation Lookups
**Context**: Checking if a value is in a set of allowed values.

**✅ DO:**
```go
var validStrategies = map[string]struct{}{
    "manual":            {},
    "auto_keep_local":   {},
    "auto_keep_remote":  {},
}

func (c *Config) Validate() error {
    if _, ok := validStrategies[c.ConflictResolution.Strategy]; !ok {
        validKeys := []string{"manual", "auto_keep_local", "auto_keep_remote"}
        return fmt.Errorf("invalid conflict resolution strategy: %s (must be one of: %s)",
            c.ConflictResolution.Strategy, strings.Join(validKeys, ", "))
    }
    return nil
}
```

**❌ DON'T:**
```go
// BAD: O(n) validation loop
func (c *Config) Validate() error {
    validStrategies := []string{"manual", "auto_keep_local", "auto_keep_remote"}
    valid := false
    for _, strategy := range validStrategies {
        if c.ConflictResolution.Strategy == strategy {
            valid = true
            break
        }
    }
    if !valid {
        return fmt.Errorf("invalid conflict resolution strategy: %s", c.ConflictResolution.Strategy)
    }
    return nil
}
```

**Rationale**: Map lookups are O(1) vs O(n) for loops. More importantly, maps express intent better: "is this value in the valid set?" The performance benefit is minor for small sets, but the code clarity improvement is significant.

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

### Rule 17: Parse External Command Output Robustly
**Context**: External commands often return structured data (CSV, JSON) that should be parsed with appropriate libraries, not string manipulation.

**✅ DO:**
```go
// Use CSV parser for CSV-formatted command output
cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/FO", "CSV", "/NH")
output, err := cmd.Output()
if err != nil {
    return 0, fmt.Errorf("process: not found: %s", name)
}

reader := csv.NewReader(strings.NewReader(string(output)))
records, err := reader.ReadAll()
if err != nil || len(records) == 0 {
    return 0, fmt.Errorf("process: not found: %s", name)
}

record := records[0]
if len(record) < 2 {
    return 0, fmt.Errorf("process: unexpected tasklist output format")
}

pid, err := strconv.Atoi(record[1])
if err != nil {
    return 0, fmt.Errorf("process: could not parse PID '%s': %w", record[1], err)
}
```

**❌ DON'T:**
```go
// Don't parse structured output with string splitting
lines := strings.Split(string(output), "\n")
fields := strings.Split(lines[1], ",")
pidStr := strings.Trim(fields[1], "\"")  // Fragile manual parsing
```

**Rationale**: Structured parsers handle edge cases (quoted fields, escaped characters, etc.) that string splitting cannot.

### Rule 18: Extract File Permissions to Named Constants
**Context**: File permission modes (0600, 0644, etc.) are magic numbers.

**✅ DO:**
```go
const (
    // configFilePerms restricts config file access to owner only (sensitive data like credentials)
    configFilePerms = 0600  // -rw-------
    
    // defaultFilePerms allows owner write, all read (standard files like .gitignore)
    defaultFilePerms = 0644 // -rw-r--r--
    
    // executablePerms for scripts that need to be executed
    executablePerms = 0755  // -rwxr-xr-x
)

// Set restrictive permissions on config file
if err := os.Chmod(configPath, configFilePerms); err != nil {
    return fmt.Errorf("failed to set config file permissions: %w", err)
}

// Create ignore file with standard permissions
if err := os.WriteFile(ignorePath, data, defaultFilePerms); err != nil {
    return fmt.Errorf("failed to create ignore file: %w", err)
}
```

**❌ DON'T:**
```go
// BAD: Magic numbers without explanation
if err := os.Chmod(configPath, 0600); err != nil {
    return fmt.Errorf("failed to set config file permissions: %w", err)
}

if err := os.WriteFile(ignorePath, data, 0644); err != nil {
    return fmt.Errorf("failed to create ignore file: %w", err)
}
```

**Rationale**: Octal permission numbers are not self-documenting. Named constants with comments explain *why* those permissions are chosen (e.g., 0600 for sensitive data). Makes security intent explicit.

### Rule 19: Comment Accuracy with JSON Tags
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

## User Experience

### Rule 18: Use DefaultConfig() Instead of Manual Construction
**Context**: Creating config structs manually duplicates logic and risks missing defaults.

**✅ DO:**
```go
func initCommand() error {
    cfg := config.DefaultConfig()  // All defaults included
    
    // Override only what's needed
    cfg.Machine.Name = machineName
    cfg.Git.RepoPath = repoPath
    cfg.Git.AuthorName = authorName
    
    return cfg.SaveToFile(configPath)
}
```

**❌ DON'T:**
```go
func initCommand() error {
    // BAD: 50+ lines of duplication
    cfg := &config.SyncConfig{
        Version: config.CurrentVersion,
        Machine: config.MachineConfig{Name: machineName},
        Git: config.GitConfig{ /* ... */ },
        Sync: config.SyncSettings{ /* ... */ },
        // Missing Sync.Backoff and other fields!
    }
}
```

**Rationale**: Using `DefaultConfig()` ensures all defaults are set, reduces duplication, and makes maintenance easier. If defaults change, you get them automatically.

### Rule 19: Check File Existence Before Overwriting
**Context**: When creating config files, existing files may contain user data.

**✅ DO:**
```go
configPath := filepath.Join(repoPath, ".sync-config.json")
if _, err := os.Stat(configPath); err == nil {
    fmt.Printf("✅ Using existing configuration: %s\n", configPath)
    return nil
}

// Only create if doesn't exist
cfg := config.DefaultConfig()
cfg.SaveToFile(configPath)
fmt.Printf("✅ Configuration created: %s\n", configPath)
```

**❌ DON'T:**
```go
// BAD: Unconditionally overwrites existing config
configPath := filepath.Join(repoPath, ".sync-config.json")
cfg := config.DefaultConfig()
cfg.SaveToFile(configPath)  // Destroys user's mappings!
```

**Rationale**: When cloning an existing dotfiles repo, it likely already has a `.sync-config.json` with user's mappings and settings. Overwriting it destroys their configuration.

### Rule 20: Confirm Destructive Operations
**Context**: Operations that delete data should require explicit confirmation.

**✅ DO:**
```go
if force {
    fmt.Printf("⚠️  Warning: --force will delete the entire directory: %s\n", repoPath)
    confirmation, err := promptForInput("Type 'yes' to confirm deletion: ", "")
    if err != nil {
        return fmt.Errorf("failed to read confirmation: %w", err)
    }
    if confirmation != "yes" {
        return fmt.Errorf("operation cancelled")
    }
    os.RemoveAll(repoPath)
}
```

**❌ DON'T:**
```go
// BAD: Deletes without asking
if force {
    os.RemoveAll(repoPath)  // No confirmation!
}
```

**Rationale**: Users can accidentally use `--force` or misunderstand what it does. Explicit confirmation prevents data loss.

### Rule 21: Use Raw String Literals for Multi-line Messages
**Context**: Error messages with multiple lines are hard to read with explicit `\n`.

**✅ DO:**
```go
return fmt.Errorf(`directory %s already exists

Options:
  - Use --force to reinitialize
  - Use a different --path
  - Remove the existing directory first`, repoPath)
```

**❌ DON'T:**
```go
// BAD: Hard to read and maintain
return fmt.Errorf("directory %s already exists\n\nOptions:\n  - Use --force to reinitialize\n  - Use a different --path\n  - Remove the existing directory first", repoPath)
```

**Rationale**: Raw string literals (backticks) preserve formatting and are much easier to read and edit.

### Rule 22: Add Godoc to Exported Functions
**Context**: All exported functions should have documentation comments.

**✅ DO:**
```go
// getConfig loads configuration using proper discovery logic.
// If --config flag is provided, it loads from that path (with tilde expansion).
// Otherwise, it searches default locations (~/dotfiles/.sync-config.json, ~/.dotfile-sync.json).
// Returns error if explicit config file doesn't exist or has invalid JSON.
func getConfig() (*config.SyncConfig, error) {
    // ...
}
```

**❌ DON'T:**
```go
// BAD: No documentation
func getConfig() (*config.SyncConfig, error) {
    // ...
}
```

**Rationale**: Godoc comments make the API self-documenting and show up in `go doc` and IDE tooltips.

### Rule 23: Mark Stubs with TODO Comments
**Context**: Stub implementations should indicate they're incomplete and reference follow-up work.

**✅ DO:**
```go
// isDaemonRunning checks if the daemon is already running.
// TODO(PR3): Implement actual daemon detection via PID file or process lookup.
func isDaemonRunning() bool {
    return false
}
```

**❌ DON'T:**
```go
// BAD: No indication this is incomplete
func isDaemonRunning() bool {
    return false  // Always returns false - is this intentional?
}
```

**Rationale**: TODO comments with PR/issue references make it clear the code is intentionally incomplete and track follow-up work.

### Rule 24: Avoid Tight Coupling in Tests
**Context**: Tests should verify behavior, not implementation details from other packages.

**✅ DO:**
```go
func TestConfigHasVersion(t *testing.T) {
    cfg := config.DefaultConfig()
    if cfg.Version == "" {
        t.Error("Expected non-empty version")
    }
}
```

**❌ DON'T:**
```go
// BAD: Tests config package's constants from cmd package
func TestDefaultConstants(t *testing.T) {
    if config.CurrentVersion != "1.0" {  // Breaks when config changes
        t.Errorf("Expected version 1.0")
    }
}
```

**Rationale**: Testing another package's constants creates tight coupling. If config changes its version, this unrelated test breaks. Test behavior, not implementation.

### Rule 25: Descriptive Test Names
**Context**: Test names should clearly communicate what is being tested.

**✅ DO:**
```go
func TestCheckSymlinkStatusWithNonExistentTildePath(t *testing.T) {
    // Test that tilde paths are properly expanded even when they don't exist
    // Verifies graceful handling of non-existent paths
    // ...
}
```

**❌ DON'T:**
```go
// BAD: Unclear what "PathExpansionError" means
func TestCheckSymlinkStatusPathExpansionError(t *testing.T) {
    // Actually tests non-existent path, not expansion errors
    // ...
}
```

**Rationale**: Clear test names help future maintainers understand what behavior is being verified without reading implementation.

### Rule 26: Preserve Existing Configuration Values
**Context**: When updating configuration files, preserve user settings that shouldn't change.

**✅ DO:**
```go
func initCommand(gitURL, repoPath string) error {
    configPath := filepath.Join(repoPath, ".sync-config.json")
    
    var cfg *config.SyncConfig
    var isNewConfig bool
    
    // Check if config already exists (e.g., from cloned repo)
    if _, err := os.Stat(configPath); err == nil {
        // Load existing config
        cfg, err = config.LoadFromFile(configPath)
        if err != nil {
            return fmt.Errorf("failed to load existing config: %w", err)
        }
        isNewConfig = false
        fmt.Println("✅ Using existing configuration, updating for this machine")
    } else {
        // Create new config
        cfg = config.DefaultConfig()
        isNewConfig = true
        fmt.Println("✅ Creating new configuration")
    }
    
    // Update machine-specific fields
    cfg.Machine.Name = getMachineName()
    cfg.Git.RepoPath = repoPath
    cfg.Git.RemoteURL = gitURL
    cfg.Git.AuthorName = authorName
    cfg.Git.AuthorEmail = authorEmail
    
    // Only set default auth for new configs; preserve existing auth settings
    if isNewConfig {
        cfg.Git.AuthType = gitmanager.AuthStrategyNone
    }
    // If loading existing config, AuthType (SSH/HTTPS/etc.) is preserved
    
    return cfg.SaveToFile(configPath)
}
```

**❌ DON'T:**
```go
// BAD: Unconditionally overwrites all fields
func initCommand(gitURL, repoPath string) error {
    configPath := filepath.Join(repoPath, ".sync-config.json")
    
    cfg := config.DefaultConfig()
    cfg.Machine.Name = getMachineName()
    cfg.Git.RepoPath = repoPath
    cfg.Git.RemoteURL = gitURL
    cfg.Git.AuthorName = authorName
    cfg.Git.AuthorEmail = authorEmail
    cfg.Git.AuthType = gitmanager.AuthStrategyNone  // DESTROYS existing SSH config!
    
    // Overwrites existing config, losing user's mappings, auth settings, etc.
    return cfg.SaveToFile(configPath)
}
```

**Rationale**: When cloning an existing dotfiles repository, it already contains a `.sync-config.json` with the user's mappings, auth configuration, and preferences. Unconditionally overwriting these settings destroys user data. Only update fields that *must* change for the new machine (paths, machine name), and preserve everything else.

## Summary Checklist

When writing configuration-related code, ensure:

**Path Handling:**
- [ ] All path fields are expanded consistently in a dedicated method
- [ ] Path expansion returns errors, not silently failing
- [ ] Tilde expansion uses `path[2:]` for `~/` prefix
- [ ] Path resolution errors propagated immediately (no silent fallbacks)

**Validation:**
- [ ] Validation only checks, never modifies state (no setting defaults in validate())
- [ ] Validation messages use "must" not "should"
- [ ] Magic numbers are extracted to named constants (including file permissions)
- [ ] Duplicated validation logic uses helper functions
- [ ] Validation uses maps for set membership checks (not loops)

**Configuration:**
- [ ] Used `DefaultConfig()` instead of manually creating config structs
- [ ] Checked file existence before overwriting
- [ ] User confirmation required for destructive operations
- [ ] Preserved existing config values when updating (don't clobber auth, mappings, etc.)
- [ ] File permissions extracted to named constants (0600, 0644, etc.)

**Code Quality:**
- [ ] Raw string literals (backticks) for multi-line error messages
- [ ] Godoc comments on all exported functions
- [ ] TODO comments on stub implementations (TODO(PRx))
- [ ] Error messages include context via wrapping
- [ ] Comments accurately reflect behavior (especially with `omitempty`)
- [ ] Validation uses maps for lookups instead of loops (better performance and clarity)

**Testing:**
- [ ] Tests use `t.Cleanup` instead of `defer`
- [ ] Tests check error returns from setup functions
- [ ] Test names clearly describe what is being tested
- [ ] Tests don't verify other packages' constants (avoid tight coupling)
- [ ] Test behavior, not implementation details

**Architecture:**
- [ ] Separation of concerns: normalize → validate → use
- [ ] External command output parsed with appropriate libraries (CSV, JSON, etc.)
