package services

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

// millisecondsPerSecond is the conversion factor between milliseconds and seconds.
const millisecondsPerSecond = 1000

// ConfigService provides configuration management for the GUI.
// It wraps the internal/config package and exposes methods for reading and updating config.
type ConfigService struct {
	cfg        *config.SyncConfig
	configPath string
}

// NewConfigService creates a new ConfigService.
func NewConfigService(cfg *config.SyncConfig, configPath string) *ConfigService {
	return &ConfigService{
		cfg:        cfg,
		configPath: configPath,
	}
}

// GetConfig returns the current configuration formatted for GUI display.
func (s *ConfigService) GetConfig() ConfigResponse {
	if s.cfg == nil {
		return ConfigResponse{}
	}

	return ConfigResponse{
		MachineName:      s.cfg.Machine.Name,
		RepoPath:         s.cfg.Git.RepoPath,
		TargetDir:        getTargetDir(s.cfg),
		SyncInterval:     s.cfg.Sync.PullIntervalSeconds,
		DebounceDelay:    s.cfg.Sync.DebounceSeconds * millisecondsPerSecond,
		AutoSync:         s.cfg.Sync.AutoSyncEnabled,
		AutoCommit:       s.cfg.Sync.AutoCommit,
		AutoPush:         s.cfg.Sync.AutoPush,
		AutoPull:         s.cfg.Sync.AutoPull,
		ConflictStrategy: s.cfg.ConflictResolution.Strategy,
		Mappings:         s.cfg.Mappings,
		ConfigPath:       s.configPath,
		SyncignorePath:   s.GetSyncignorePath(),
	}
}

// UpdateConfig applies partial updates to the configuration.
// Only non-nil fields in the request are applied.
// Updates are atomic: the original config is only modified after successful validation and save.
func (s *ConfigService) UpdateConfig(req UpdateConfigRequest) error {
	if s.cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	// Create a copy to avoid mutating before validation
	cfgCopy := *s.cfg

	// Apply updates to copy
	if req.MachineName != nil {
		cfgCopy.Machine.Name = *req.MachineName
	}
	if req.RepoPath != nil {
		cfgCopy.Git.RepoPath = *req.RepoPath
	}
	if req.SyncInterval != nil {
		cfgCopy.Sync.PullIntervalSeconds = *req.SyncInterval
	}
	if req.DebounceDelay != nil {
		cfgCopy.Sync.DebounceSeconds = *req.DebounceDelay / millisecondsPerSecond
	}
	if req.AutoSync != nil {
		cfgCopy.Sync.AutoSyncEnabled = *req.AutoSync
	}
	if req.AutoCommit != nil {
		cfgCopy.Sync.AutoCommit = *req.AutoCommit
	}
	if req.AutoPush != nil {
		cfgCopy.Sync.AutoPush = *req.AutoPush
	}
	if req.AutoPull != nil {
		cfgCopy.Sync.AutoPull = *req.AutoPull
	}
	if req.ConflictStrategy != nil {
		cfgCopy.ConflictResolution.Strategy = *req.ConflictStrategy
	}

	// Validate the copy before applying changes
	if err := cfgCopy.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Save to file
	if err := cfgCopy.SaveToFile(s.configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Only update original after success
	*s.cfg = cfgCopy
	return nil
}

// ResetToDefaults resets the configuration to default values while preserving
// critical user settings like repository path, mappings, and authentication.
// Only sync behavior settings (intervals, auto-sync, etc.) are reset.
func (s *ConfigService) ResetToDefaults() error {
	if s.cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	defaultCfg, err := config.DefaultConfig()
	if err != nil {
		return fmt.Errorf("failed to create default config: %w", err)
	}

	// Preserve critical user settings that shouldn't be reset
	defaultCfg.ConfigPath = s.configPath
	defaultCfg.Git.RepoPath = s.cfg.Git.RepoPath
	defaultCfg.Git.RemoteURL = s.cfg.Git.RemoteURL
	defaultCfg.Git.RemoteName = s.cfg.Git.RemoteName
	defaultCfg.Git.Branch = s.cfg.Git.Branch
	defaultCfg.Git.AuthorName = s.cfg.Git.AuthorName
	defaultCfg.Git.AuthorEmail = s.cfg.Git.AuthorEmail
	defaultCfg.Git.AuthType = s.cfg.Git.AuthType
	defaultCfg.Git.Username = s.cfg.Git.Username
	defaultCfg.Git.SSHKeyPath = s.cfg.Git.SSHKeyPath
	defaultCfg.Git.SSHKeyPassphrase = s.cfg.Git.SSHKeyPassphrase
	defaultCfg.Git.KnownHostsPath = s.cfg.Git.KnownHostsPath
	defaultCfg.Mappings = s.cfg.Mappings

	// Validate (should always pass for defaults with preserved paths)
	if err := defaultCfg.Validate(); err != nil {
		return fmt.Errorf("default configuration is invalid: %w", err)
	}

	// Save to file
	if err := defaultCfg.SaveToFile(s.configPath); err != nil {
		return fmt.Errorf("failed to save default configuration: %w", err)
	}

	// Update in-memory config
	*s.cfg = *defaultCfg

	return nil
}

// GetSyncignorePath returns the path to the .syncignore file.
func (s *ConfigService) GetSyncignorePath() string {
	if s.cfg == nil || s.cfg.Git.RepoPath == "" {
		return ""
	}
	return filepath.Join(s.cfg.Git.RepoPath, ".syncignore")
}

// GetMappingsPath returns the path to the mappings directory.
func (s *ConfigService) GetMappingsPath() string {
	if s.cfg == nil || s.cfg.Git.RepoPath == "" {
		return ""
	}
	return s.cfg.Git.RepoPath
}

// GetConfigLastModified returns the last modification time of the config file.
// TODO: Implement actual file stat lookup using os.Stat(s.configPath)
func (s *ConfigService) GetConfigLastModified() string {
	if s.cfg == nil {
		return ""
	}

	// Placeholder: returns current time until os.Stat implementation is added
	return time.Now().Format(time.RFC3339)
}

// getTargetDir extracts the common target directory from mappings.
// Returns the shared parent directory if all mapping targets share the same parent.
// Returns empty string if mappings are empty or have different parent directories.
func getTargetDir(cfg *config.SyncConfig) string {
	if cfg == nil || len(cfg.Mappings) == 0 {
		return ""
	}

	// Check if all mapping targets share the same parent directory
	var commonDir string
	for _, target := range cfg.Mappings {
		dir := filepath.Dir(target)
		if commonDir == "" {
			commonDir = dir
		} else if commonDir != dir {
			// Mappings have different parent directories
			return ""
		}
	}
	return commonDir
}
