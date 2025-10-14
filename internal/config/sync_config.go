package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
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

// DefaultConfig returns a default configuration
func DefaultConfig() *SyncConfig {
	homeDir, _ := os.UserHomeDir()

	return &SyncConfig{
		Version: "1.0",
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
			PullIntervalSeconds: 300, // 5 minutes
			DebounceSeconds:     30,  // 30 seconds
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
			KeepBackupsDays: 7,
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
			MaxLogSizeMB: 10,
		},
	}
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(filename string) (*SyncConfig, error) {
	config := DefaultConfig()

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

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config: invalid configuration: %w", err)
	}

	return config, nil
}

// SaveToFile saves configuration to a JSON file
func (c *SyncConfig) SaveToFile(filename string) error {
	// Validate before saving
	if err := c.Validate(); err != nil {
		return fmt.Errorf("config: invalid configuration: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("config: failed to create config directory: %w", err)
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

// Validate checks if the configuration is valid
func (c *SyncConfig) Validate() error {
	if c.Machine.Name == "" {
		return fmt.Errorf("machine name is required")
	}

	if c.Git.RepoPath == "" {
		return fmt.Errorf("git repo path is required")
	}

	if !filepath.IsAbs(c.Git.RepoPath) {
		return fmt.Errorf("git repo path must be absolute")
	}

	if c.Git.AuthorName == "" || c.Git.AuthorEmail == "" {
		return fmt.Errorf("git author name and email are required")
	}

	if c.Sync.PullIntervalSeconds <= 0 {
		return fmt.Errorf("pull interval must be positive")
	}

	if c.Sync.DebounceSeconds <= 0 {
		return fmt.Errorf("debounce delay must be positive")
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

		// Always validate manual sync timeout if specified
		if c.Sync.Backoff.ManualSyncTimeoutSeconds < 0 {
			return fmt.Errorf("backoff manual sync timeout must be non-negative")
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
