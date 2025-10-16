package config

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/util"
)

// Configuration version
const CurrentVersion = "1.0"

// Default configuration values
const (
	DefaultPullIntervalSeconds = 300 // 5 minutes
	DefaultDebounceSeconds     = 30  // 30 seconds
	DefaultMaxLogSizeMB        = 10
	DefaultKeepBackupsDays     = 7
)

// Validation constants
const (
	minPullIntervalSeconds = 60
	maxBackupRetentionDays = 365
	maxLogSizeMB           = 1000
)

// SyncConfig represents the complete configuration for the dotfile sync manager
type SyncConfig struct {
	// Version of the configuration file format
	Version string `json:"version"`

	// Machine identification
	Machine MachineConfig `json:"machine"`

	// Git configuration
	Git GitConfig `json:"git"`

	// Sync settings
	Sync SyncSettings `json:"sync"`

	// Notification settings
	Notifications NotificationConfig `json:"notifications"`

	// Conflict resolution settings
	ConflictResolution ConflictConfig `json:"conflict_resolution"`

	// File mappings
	Mappings map[string]string `json:"mappings"`

	// UI settings
	UI UIConfig `json:"ui"`

	// Advanced settings
	Advanced AdvancedConfig `json:"advanced"`

	// ConfigPath stores the path from which this config was loaded (not persisted to JSON)
	ConfigPath string `json:"-"`
}

// MachineConfig holds machine-specific settings
type MachineConfig struct {
	// Name of this machine (e.g., "work-laptop", "personal-mac")
	Name string `json:"name"`
}

// GitConfig extends the gitmanager.Config with additional fields
type GitConfig struct {
	// Repository path (absolute path to dotfiles directory)
	RepoPath string `json:"repo_path"`

	// Remote repository URL
	RemoteURL string `json:"remote_url"`

	// Remote name (usually "origin")
	RemoteName string `json:"remote_name"`

	// Branch name (usually "main" or "master")
	Branch string `json:"branch"`

	// Author information for commits
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`

	// Authentication settings
	AuthType         gitmanager.AuthStrategy `json:"auth_type"`
	Username         string                  `json:"username,omitempty"`
	Password         string                  `json:"password,omitempty"`
	SSHKeyPath       string                  `json:"ssh_key_path,omitempty"`
	SSHKeyPassphrase string                  `json:"ssh_key_passphrase,omitempty"`
	KnownHostsPath   string                  `json:"known_hosts_path,omitempty"`
}

// BackoffSettings controls advanced debouncer behavior
type BackoffSettings struct {
	// Enable exponential backoff during rapid file changes
	Enabled bool `json:"enabled"`

	// Maximum debounce delay in seconds
	MaxDelaySeconds int `json:"max_delay_seconds"`

	// Backoff multiplier (multiplies delay for each successive change)
	Multiplier float64 `json:"multiplier"`

	// Number of changes in time window to trigger churn mode
	ChurnThreshold int `json:"churn_threshold"`

	// Time window in seconds to detect churn
	ChurnWindowSeconds int `json:"churn_window_seconds"`

	// Time in seconds of inactivity to reset backoff to normal
	DecayResetSeconds int `json:"decay_reset_seconds"`

	// Manual sync timeout in seconds
	ManualSyncTimeoutSeconds int `json:"manual_sync_timeout_seconds,omitempty"`
}

// SyncSettings controls synchronization behavior
type SyncSettings struct {
	// Enable automatic synchronization
	AutoSyncEnabled bool `json:"auto_sync_enabled"`

	// Pull interval in seconds
	PullIntervalSeconds int `json:"pull_interval_seconds"`

	// Debounce delay in seconds (wait after last change before syncing)
	DebounceSeconds int `json:"debounce_seconds"`

	// Enable automatic commits
	AutoCommit bool `json:"auto_commit"`

	// Enable automatic pushes
	AutoPush bool `json:"auto_push"`

	// Enable automatic pulls
	AutoPull bool `json:"auto_pull"`

	// Advanced debouncer settings
	Backoff *BackoffSettings `json:"backoff,omitempty"`
}

// NotificationConfig controls desktop notifications
type NotificationConfig struct {
	// Enable notifications
	Enabled bool `json:"enabled"`

	// Show success notifications
	ShowSuccess bool `json:"show_success"`

	// Show pull notifications
	ShowPulls bool `json:"show_pulls"`

	// Play sound on conflicts
	PlaySoundOnConflict bool `json:"play_sound_on_conflict"`
}

// ConflictConfig controls conflict resolution behavior
type ConflictConfig struct {
	// Strategy for resolving conflicts ("manual", "auto_keep_local", "auto_keep_remote")
	Strategy string `json:"strategy"`

	// Backup directory for conflict files
	BackupDir string `json:"backup_dir"`

	// How many days to keep backup files
	KeepBackupsDays int `json:"keep_backups_days"`
}

// UIConfig controls user interface settings
type UIConfig struct {
	// Start application at system boot
	StartAtBoot bool `json:"start_at_boot"`

	// Minimize to system tray
	MinimizeToTray bool `json:"minimize_to_tray"`

	// Theme ("auto", "light", "dark")
	Theme string `json:"theme"`
}

// AdvancedConfig holds advanced technical settings
type AdvancedConfig struct {
	// Enable debug logging
	DebugLogging bool `json:"debug_logging"`

	// Log file path
	LogFile string `json:"log_file"`

	// Maximum log file size in MB
	MaxLogSizeMB int `json:"max_log_size_mb"`
}

// DefaultConfig returns a default configuration.
// Returns an error if the user's home directory cannot be determined.
//
// SECURITY WARNING: This configuration can store sensitive data including passwords,
// SSH key passphrases, and authentication credentials in plain text. Never commit
// .sync-config.json files to version control repositories. Consider using a
// system keychain/credential manager for sensitive authentication data.
func DefaultConfig() (*SyncConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	return &SyncConfig{
		Version: CurrentVersion,
		Machine: MachineConfig{
			Name: getDefaultMachineName(),
		},
		Git: GitConfig{
			RepoPath:    filepath.Join(homeDir, "dotfiles"),
			RemoteURL:   "",
			RemoteName:  "origin",
			Branch:      "main",
			AuthorName:  getGitConfig("user.name"),
			AuthorEmail: getGitConfig("user.email"),
			AuthType:    gitmanager.AuthStrategySSH,
		},
		Sync: SyncSettings{
			AutoSyncEnabled:     true,
			PullIntervalSeconds: DefaultPullIntervalSeconds,
			DebounceSeconds:     DefaultDebounceSeconds,
			AutoCommit:          true,
			AutoPush:            true,
			AutoPull:            true,
			Backoff: &BackoffSettings{
				Enabled:                  true,
				MaxDelaySeconds:          300, // 5 minutes
				Multiplier:               2.0,
				ChurnThreshold:           10,
				ChurnWindowSeconds:       60,  // 1 minute
				DecayResetSeconds:        300, // 5 minutes
				ManualSyncTimeoutSeconds: 10,  // 10 seconds
			},
		},
		Notifications: NotificationConfig{
			Enabled:             true,
			ShowSuccess:         false,
			ShowPulls:           true,
			PlaySoundOnConflict: false,
		},
		ConflictResolution: ConflictConfig{
			Strategy:        "manual",
			BackupDir:       filepath.Join(homeDir, "dotfiles", ".backup"),
			KeepBackupsDays: DefaultKeepBackupsDays,
		},
		Mappings: make(map[string]string),
		UI: UIConfig{
			StartAtBoot:    false,
			MinimizeToTray: true,
			Theme:          "auto",
		},
		Advanced: AdvancedConfig{
			DebugLogging: false,
			LogFile:      filepath.Join(homeDir, ".dotfile-sync.log"),
			MaxLogSizeMB: DefaultMaxLogSizeMB,
		},
	}, nil
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(filename string) (*SyncConfig, error) {
	config, err := DefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create default config: %w", err)
	}

	// Store the config path (resolve to absolute path)
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("config: failed to resolve absolute path for %s: %w", filename, err)
	}
	config.ConfigPath = absPath

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return config, nil
	}

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("config: failed to read config file: %w", err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("config: failed to parse config file: %w", err)
	}

	// Expand all user-provided paths for consistency
	if err := config.expandPaths(); err != nil {
		return nil, fmt.Errorf("config: failed to expand paths: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config: invalid configuration: %w", err)
	}

	return config, nil
}

// FindConfigFile finds the configuration file using the following priority:
// 1. Path provided explicitly (if exists)
// 2. ~/dotfiles/.sync-config.json (PRD location)
// 3. ~/.dotfile-sync.json (legacy location)
// Returns the path to the config file and whether it exists
func FindConfigFile(explicitPath string) (string, bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", false, fmt.Errorf("config: failed to get home directory: %w", err)
	}

	// If explicit path provided, use it
	if explicitPath != "" {
		path, err := util.ExpandPath(explicitPath)
		if err != nil {
			return "", false, fmt.Errorf("config: failed to expand explicit path: %w", err)
		}
		if _, err := os.Stat(path); err == nil {
			return path, true, nil
		} else if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("config: error accessing config file %s: %w", path, err)
		}
		// Explicit path doesn't exist, return it anyway (caller can decide what to do)
		return path, false, nil
	}

	// Check standard locations in order of priority
	prdPath := filepath.Join(homeDir, "dotfiles", ".sync-config.json")
	searchPaths := []string{
		prdPath, // PRD location
		filepath.Join(homeDir, ".dotfile-sync.json"), // Legacy location
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, true, nil
		} else if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("config: error accessing config location %s: %w", path, err)
		}
	}

	// No config file found, return the PRD location as the default
	return prdPath, false, nil
}

// LoadFromDefaultLocation loads configuration from the default location
// It searches for config files in the standard locations and returns a default config if none found
func LoadFromDefaultLocation() (*SyncConfig, error) {
	configPath, exists, err := FindConfigFile("")
	if err != nil {
		return nil, err
	}

	if exists {
		return LoadFromFile(configPath)
	}

	// Return default config if no file found, store the expected path
	cfg, err := DefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create default config: %w", err)
	}
	cfg.ConfigPath = configPath
	return cfg, nil
}

// expandPaths normalizes all user-provided paths to absolute paths
func (c *SyncConfig) expandPaths() error {
	// Helper to expand a single path field
	expand := func(path *string, fieldName string) error {
		if *path == "" {
			return nil
		}
		expanded, err := util.ExpandPath(*path)
		if err != nil {
			return fmt.Errorf("failed to expand %s: %w", fieldName, err)
		}
		*path = expanded
		return nil
	}

	// Expand Git-related paths
	if err := expand(&c.Git.RepoPath, "git.repo_path"); err != nil {
		return err
	}
	if err := expand(&c.Git.SSHKeyPath, "git.ssh_key_path"); err != nil {
		return err
	}
	if err := expand(&c.Git.KnownHostsPath, "git.known_hosts_path"); err != nil {
		return err
	}

	// Expand conflict resolution paths
	if err := expand(&c.ConflictResolution.BackupDir, "conflict_resolution.backup_dir"); err != nil {
		return err
	}

	// Expand advanced paths
	if err := expand(&c.Advanced.LogFile, "advanced.log_file"); err != nil {
		return err
	}

	// Expand mapping targets
	for key, target := range c.Mappings {
		if target == "" {
			continue
		}
		expanded, err := util.ExpandPath(target)
		if err != nil {
			return fmt.Errorf("failed to expand mapping target for '%s': %w", key, err)
		}
		c.Mappings[key] = expanded
	}

	return nil
}

// SaveToFile saves configuration to a JSON file
func (c *SyncConfig) SaveToFile(filename string) error {
	// Expand paths before validation and saving
	if err := c.expandPaths(); err != nil {
		return fmt.Errorf("config: failed to expand paths: %w", err)
	}

	// Validate before saving
	if err := c.Validate(); err != nil {
		return fmt.Errorf("config: invalid configuration: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("config: failed to create config directory: %w", err)
	}

	if c.Git.Password != "" || c.Git.SSHKeyPassphrase != "" {
		fmt.Fprintln(os.Stderr, "⚠️  Warning: configuration contains plaintext credentials; ensure this file remains private")
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: failed to marshal config: %w", err)
	}

	// Write file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("config: failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the path from which this config was loaded.
// This path is always set during loading by LoadFromFile or LoadFromDefaultLocation.
func (c *SyncConfig) GetConfigPath() string {
	return c.ConfigPath
}

// validThemes contains allowed UI theme values for O(1) validation
var validThemes = map[string]struct{}{
	"auto":  {},
	"light": {},
	"dark":  {},
}

// validConflictStrategies contains allowed conflict resolution strategy values for O(1) validation
var validConflictStrategies = map[string]struct{}{
	"manual":           {},
	"auto_keep_local":  {},
	"auto_keep_remote": {},
}

// validateTheme checks if a UI theme value is valid using O(1) map lookup
func validateTheme(value string) error {
	if _, ok := validThemes[value]; !ok {
		return fmt.Errorf("invalid UI theme: %s (must be one of: %s)", value, strings.Join(getValidThemeKeys(), ", "))
	}
	return nil
}

// getValidThemeKeys returns the valid theme keys as a slice for error messages
func getValidThemeKeys() []string {
	keys := make([]string, 0, len(validThemes))
	for theme := range validThemes {
		keys = append(keys, theme)
	}
	sort.Strings(keys)
	return keys
}

// getValidConflictStrategyKeys returns valid conflict strategies sorted for deterministic output
func getValidConflictStrategyKeys() []string {
	keys := make([]string, 0, len(validConflictStrategies))
	for strategy := range validConflictStrategies {
		keys = append(keys, strategy)
	}
	sort.Strings(keys)
	return keys
}

// Validate checks if the configuration is valid
func (c *SyncConfig) Validate() error {
	// Validate version
	if c.Version == "" {
		return fmt.Errorf("configuration version is required")
	}

	// Validate machine configuration
	if c.Machine.Name == "" {
		return fmt.Errorf("machine name is required")
	}

	// Validate git configuration
	if c.Git.RepoPath == "" {
		return fmt.Errorf("git repo path is required")
	}

	if !filepath.IsAbs(c.Git.RepoPath) {
		return fmt.Errorf("git repo path must be absolute")
	}

	if c.Git.AuthorName == "" || c.Git.AuthorEmail == "" {
		return fmt.Errorf("git author name and email are required")
	}

	// Validate git email format using RFC 5322 compliant parsing
	if _, err := mail.ParseAddress(c.Git.AuthorEmail); err != nil {
		return fmt.Errorf("git author email must be a valid email address: %w", err)
	}

	// Validate sync settings
	if c.Sync.PullIntervalSeconds <= 0 {
		return fmt.Errorf("pull interval must be positive")
	}

	if c.Sync.PullIntervalSeconds < minPullIntervalSeconds {
		return fmt.Errorf("pull interval must be at least %d seconds", minPullIntervalSeconds)
	}

	if c.Sync.DebounceSeconds <= 0 {
		return fmt.Errorf("debounce delay must be positive")
	}

	if c.Sync.DebounceSeconds > c.Sync.PullIntervalSeconds {
		return fmt.Errorf("debounce delay must not exceed pull interval")
	}

	// Notification settings validated elsewhere if needed

	// Validate conflict resolution settings
	if c.ConflictResolution.Strategy == "" {
		return fmt.Errorf("conflict resolution strategy is required")
	}

	// Use package-level map for O(1) strategy validation
	if _, ok := validConflictStrategies[c.ConflictResolution.Strategy]; !ok {
		return fmt.Errorf("invalid conflict resolution strategy: %s (must be one of: %s)", c.ConflictResolution.Strategy, strings.Join(getValidConflictStrategyKeys(), ", "))
	}

	if c.ConflictResolution.KeepBackupsDays < 0 {
		return fmt.Errorf("backup retention days must be non-negative")
	}

	if c.ConflictResolution.KeepBackupsDays > maxBackupRetentionDays {
		return fmt.Errorf("backup retention days must not exceed %d", maxBackupRetentionDays)
	}

	// Validate UI settings
	if c.UI.Theme == "" {
		return fmt.Errorf("UI theme is required")
	}

	if err := validateTheme(c.UI.Theme); err != nil {
		return err
	}

	// Validate advanced settings
	if c.Advanced.MaxLogSizeMB <= 0 {
		return fmt.Errorf("maximum log size must be positive")
	}

	if c.Advanced.MaxLogSizeMB > maxLogSizeMB {
		return fmt.Errorf("maximum log size must not exceed %d MB", maxLogSizeMB)
	}

	// Validate file mappings (paths are already expanded by expandPaths)
	if c.Mappings != nil {
		for source, target := range c.Mappings {
			if source == "" {
				return fmt.Errorf("mapping source cannot be empty")
			}
			if target == "" {
				return fmt.Errorf("mapping target for '%s' cannot be empty", source)
			}
		expandedTarget, err := util.ExpandPath(target)
		if err != nil {
			return fmt.Errorf("failed to expand mapping target for '%s': %w", source, err)
		}
		if !filepath.IsAbs(expandedTarget) {
			return fmt.Errorf("mapping target for '%s' must expand to an absolute path, but got '%s'", source, expandedTarget)
		}
		}
	}

	// Validate backoff settings if provided
	if c.Sync.Backoff != nil {
		// Only validate detailed backoff parameters if backoff is enabled
		if c.Sync.Backoff.Enabled {
			if c.Sync.Backoff.MaxDelaySeconds <= 0 {
				return fmt.Errorf("backoff max delay must be positive")
			}
			if c.Sync.Backoff.MaxDelaySeconds < c.Sync.DebounceSeconds {
				return fmt.Errorf("backoff max delay must be >= debounce delay")
			}
			if c.Sync.Backoff.Multiplier <= 1.0 {
				return fmt.Errorf("backoff multiplier must be > 1.0")
			}
			if c.Sync.Backoff.ChurnThreshold <= 0 {
				return fmt.Errorf("backoff churn threshold must be positive")
			}
			if c.Sync.Backoff.ChurnWindowSeconds <= 0 {
				return fmt.Errorf("backoff churn window must be positive")
			}
			if c.Sync.Backoff.DecayResetSeconds <= 0 {
				return fmt.Errorf("backoff decay reset must be positive")
			}
		}

		// If manual sync timeout is not set (or is 0), a default of 10 seconds is used.
		// A negative value is invalid.
		if c.Sync.Backoff.ManualSyncTimeoutSeconds < 0 {
			return fmt.Errorf("backoff manual sync timeout cannot be negative")
		}
	}

	return nil
}

// ToGitManagerConfig converts to gitmanager.Config
func (c *SyncConfig) ToGitManagerConfig() gitmanager.Config {
	return gitmanager.Config{
		RepoPath:         c.Git.RepoPath,
		RemoteURL:        c.Git.RemoteURL,
		RemoteName:       c.Git.RemoteName,
		AuthorName:       c.Git.AuthorName,
		AuthorEmail:      c.Git.AuthorEmail,
		AuthType:         c.Git.AuthType,
		Username:         c.Git.Username,
		Password:         c.Git.Password,
		SSHKeyPath:       c.Git.SSHKeyPath,
		SSHKeyPassphrase: c.Git.SSHKeyPassphrase,
		KnownHostsPath:   c.Git.KnownHostsPath,
	}
}

// SyncServiceConfig represents the configuration for the sync service
type SyncServiceConfig struct {
	RepoPath        string
	DebounceDelay   time.Duration
	AutoSyncEnabled bool
	IgnoreFile      string
	Backoff         *debouncer.AdvancedDebouncerConfig
}

// ToSyncServiceConfig converts to sync.Config
func (c *SyncConfig) ToSyncServiceConfig() SyncServiceConfig {
	config := SyncServiceConfig{
		RepoPath:        c.Git.RepoPath,
		DebounceDelay:   time.Duration(c.Sync.DebounceSeconds) * time.Second,
		AutoSyncEnabled: c.Sync.AutoSyncEnabled,
		IgnoreFile:      ".syncignore",
	}

	// Add backoff configuration if provided
	if c.Sync.Backoff != nil {
		manualSyncTimeout := 10 * time.Second // default
		if c.Sync.Backoff.ManualSyncTimeoutSeconds > 0 {
			manualSyncTimeout = time.Duration(c.Sync.Backoff.ManualSyncTimeoutSeconds) * time.Second
		}

		config.Backoff = &debouncer.AdvancedDebouncerConfig{
			BaseDelay:          config.DebounceDelay,
			MaxDelay:           time.Duration(c.Sync.Backoff.MaxDelaySeconds) * time.Second,
			BackoffEnabled:     c.Sync.Backoff.Enabled,
			BackoffMultiplier:  c.Sync.Backoff.Multiplier,
			ChurnThreshold:     c.Sync.Backoff.ChurnThreshold,
			ChurnWindow:        time.Duration(c.Sync.Backoff.ChurnWindowSeconds) * time.Second,
			DecayResetDuration: time.Duration(c.Sync.Backoff.DecayResetSeconds) * time.Second,
			ManualSyncTimeout:  manualSyncTimeout,
		}
	}

	return config
}

// Helper functions

func getDefaultMachineName() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}
	return "unknown-machine"
}

func getGitConfig(key string) string {
	// Try to get git config from system
	cmd := exec.Command("git", "config", "--global", key)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to environment variables or defaults
		if key == "user.name" {
			if username := os.Getenv("USER"); username != "" {
				return username
			}
			return "Dotfile Sync User"
		}
		if key == "user.email" {
			if hostname, _ := os.Hostname(); hostname != "" {
				return fmt.Sprintf("user@%s", strings.ToLower(hostname))
			}
			return "user@localhost"
		}
		return ""
	}

	return strings.TrimSpace(string(output))
}