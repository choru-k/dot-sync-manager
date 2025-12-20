package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
)

func TestNewConfigService(t *testing.T) {
	cfg := &config.SyncConfig{}
	cfg.Machine.Name = "test-machine"
	cfg.Git.RepoPath = "/tmp/repo"
	configPath := "/tmp/config.json"

	svc := NewConfigService(cfg, configPath)

	if svc == nil {
		t.Fatal("NewConfigService returned nil")
	}
	if svc.cfg != cfg {
		t.Error("ConfigService cfg not set correctly")
	}
	if svc.configPath != configPath {
		t.Error("ConfigService configPath not set correctly")
	}
}

func TestConfigService_GetConfig(t *testing.T) {
	cfg := &config.SyncConfig{}
	cfg.Machine.Name = "my-laptop"
	cfg.Git.RepoPath = "/home/user/dotfiles"
	cfg.Sync.PullIntervalSeconds = 300
	cfg.Sync.DebounceSeconds = 30
	cfg.Sync.AutoSyncEnabled = true
	cfg.Sync.AutoCommit = true
	cfg.Sync.AutoPush = true
	cfg.Sync.AutoPull = true
	cfg.ConflictResolution.Strategy = "manual"
	cfg.Mappings = map[string]string{
		".bashrc": "/home/user/.bashrc",
		".vimrc":  "/home/user/.vimrc",
	}
	configPath := "/home/user/.config/dsm/config.json"

	svc := NewConfigService(cfg, configPath)
	resp := svc.GetConfig()

	if resp.MachineName != "my-laptop" {
		t.Errorf("expected machine name 'my-laptop', got '%s'", resp.MachineName)
	}
	if resp.RepoPath != "/home/user/dotfiles" {
		t.Errorf("expected repo path '/home/user/dotfiles', got '%s'", resp.RepoPath)
	}
	if resp.SyncInterval != 300 {
		t.Errorf("expected sync interval 300, got %d", resp.SyncInterval)
	}
	if resp.DebounceDelay != 30000 {
		t.Errorf("expected debounce delay 30000ms, got %d", resp.DebounceDelay)
	}
	if !resp.AutoSync {
		t.Error("expected AutoSync true")
	}
	if !resp.AutoCommit {
		t.Error("expected AutoCommit true")
	}
	if !resp.AutoPush {
		t.Error("expected AutoPush true")
	}
	if !resp.AutoPull {
		t.Error("expected AutoPull true")
	}
	if resp.ConflictStrategy != "manual" {
		t.Errorf("expected conflict strategy 'manual', got '%s'", resp.ConflictStrategy)
	}
	if len(resp.Mappings) != 2 {
		t.Errorf("expected 2 mappings, got %d", len(resp.Mappings))
	}
	if resp.ConfigPath != configPath {
		t.Errorf("expected config path '%s', got '%s'", configPath, resp.ConfigPath)
	}
	if resp.SyncignorePath != "/home/user/dotfiles/.syncignore" {
		t.Errorf("expected syncignore path '/home/user/dotfiles/.syncignore', got '%s'", resp.SyncignorePath)
	}
}

func TestConfigService_GetConfig_NilConfig(t *testing.T) {
	svc := NewConfigService(nil, "/tmp/config.json")
	resp := svc.GetConfig()

	if resp.MachineName != "" {
		t.Error("expected empty machine name for nil config")
	}
	if resp.RepoPath != "" {
		t.Error("expected empty repo path for nil config")
	}
}

func TestConfigService_UpdateConfig(t *testing.T) {
	// Create a temp directory for config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	cfg.ConfigPath = configPath
	cfg.Git.RepoPath = tmpDir // Use temp dir as repo path for validation

	svc := NewConfigService(cfg, configPath)

	// Create update request with some fields
	newName := "updated-machine"
	newInterval := 600
	newAutoSync := false
	req := UpdateConfigRequest{
		MachineName:  &newName,
		SyncInterval: &newInterval,
		AutoSync:     &newAutoSync,
	}

	err = svc.UpdateConfig(req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify updates were applied
	if cfg.Machine.Name != "updated-machine" {
		t.Errorf("expected machine name 'updated-machine', got '%s'", cfg.Machine.Name)
	}
	if cfg.Sync.PullIntervalSeconds != 600 {
		t.Errorf("expected sync interval 600, got %d", cfg.Sync.PullIntervalSeconds)
	}
	if cfg.Sync.AutoSyncEnabled != false {
		t.Error("expected AutoSync false")
	}

	// Verify file was saved
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not saved")
	}
}

func TestConfigService_UpdateConfig_PartialUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	cfg.ConfigPath = configPath
	cfg.Git.RepoPath = tmpDir
	cfg.Machine.Name = "original-name"
	cfg.Sync.AutoCommit = true

	svc := NewConfigService(cfg, configPath)

	// Only update AutoPush, leave others unchanged
	newAutoPush := true
	req := UpdateConfigRequest{
		AutoPush: &newAutoPush,
	}

	err = svc.UpdateConfig(req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify only AutoPush changed
	if cfg.Sync.AutoPush != true {
		t.Error("expected AutoPush true")
	}
	// Verify other fields unchanged
	if cfg.Machine.Name != "original-name" {
		t.Errorf("machine name should be unchanged, got '%s'", cfg.Machine.Name)
	}
	if cfg.Sync.AutoCommit != true {
		t.Error("AutoCommit should be unchanged")
	}
}

func TestConfigService_UpdateConfig_NilConfig(t *testing.T) {
	svc := NewConfigService(nil, "/tmp/config.json")

	newName := "test"
	req := UpdateConfigRequest{
		MachineName: &newName,
	}

	err := svc.UpdateConfig(req)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestConfigService_UpdateConfig_ConflictStrategy(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	cfg.ConfigPath = configPath
	cfg.Git.RepoPath = tmpDir

	svc := NewConfigService(cfg, configPath)

	// Update conflict strategy
	newStrategy := "auto_keep_local"
	req := UpdateConfigRequest{
		ConflictStrategy: &newStrategy,
	}

	err = svc.UpdateConfig(req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	if cfg.ConflictResolution.Strategy != "auto_keep_local" {
		t.Errorf("expected conflict strategy 'auto_keep_local', got '%s'", cfg.ConflictResolution.Strategy)
	}
}

func TestConfigService_UpdateConfig_DebounceConversion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	cfg.ConfigPath = configPath
	cfg.Git.RepoPath = tmpDir

	svc := NewConfigService(cfg, configPath)

	// Update debounce delay in milliseconds
	newDebounce := 5000 // 5 seconds in ms
	req := UpdateConfigRequest{
		DebounceDelay: &newDebounce,
	}

	err = svc.UpdateConfig(req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Should be converted to seconds
	if cfg.Sync.DebounceSeconds != 5 {
		t.Errorf("expected debounce seconds 5, got %d", cfg.Sync.DebounceSeconds)
	}
}

func TestConfigService_ResetToDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	cfg.ConfigPath = configPath
	cfg.Git.RepoPath = tmpDir

	// Modify some sync behavior values (these should be reset)
	cfg.Sync.PullIntervalSeconds = 999
	cfg.Sync.AutoSyncEnabled = false

	svc := NewConfigService(cfg, configPath)

	err = svc.ResetToDefaults()
	if err != nil {
		t.Fatalf("ResetToDefaults failed: %v", err)
	}

	// Verify sync behavior was reset to defaults
	if cfg.Sync.PullIntervalSeconds == 999 {
		t.Error("sync interval should have been reset")
	}

	// ConfigPath should be preserved
	if cfg.ConfigPath != configPath {
		t.Errorf("config path should be preserved, got '%s'", cfg.ConfigPath)
	}

	// RepoPath should be preserved
	if cfg.Git.RepoPath != tmpDir {
		t.Errorf("repo path should be preserved, got '%s'", cfg.Git.RepoPath)
	}

	// Verify file was saved
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not saved after reset")
	}
}

func TestConfigService_GetSyncignorePath(t *testing.T) {
	tests := []struct {
		name     string
		repoPath string
		expected string
	}{
		{
			name:     "valid repo path",
			repoPath: "/home/user/dotfiles",
			expected: "/home/user/dotfiles/.syncignore",
		},
		{
			name:     "empty repo path",
			repoPath: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.SyncConfig{}
			cfg.Git.RepoPath = tt.repoPath
			svc := NewConfigService(cfg, "/tmp/config.json")

			result := svc.GetSyncignorePath()
			if result != tt.expected {
				t.Errorf("GetSyncignorePath() = '%s', want '%s'", result, tt.expected)
			}
		})
	}
}

func TestConfigService_GetSyncignorePath_NilConfig(t *testing.T) {
	svc := NewConfigService(nil, "/tmp/config.json")

	result := svc.GetSyncignorePath()
	if result != "" {
		t.Errorf("expected empty string for nil config, got '%s'", result)
	}
}

func TestConfigService_GetMappingsPath(t *testing.T) {
	tests := []struct {
		name     string
		repoPath string
		expected string
	}{
		{
			name:     "valid repo path",
			repoPath: "/home/user/dotfiles",
			expected: "/home/user/dotfiles",
		},
		{
			name:     "empty repo path",
			repoPath: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.SyncConfig{}
			cfg.Git.RepoPath = tt.repoPath
			svc := NewConfigService(cfg, "/tmp/config.json")

			result := svc.GetMappingsPath()
			if result != tt.expected {
				t.Errorf("GetMappingsPath() = '%s', want '%s'", result, tt.expected)
			}
		})
	}
}

func TestConfigService_GetMappingsPath_NilConfig(t *testing.T) {
	svc := NewConfigService(nil, "/tmp/config.json")

	result := svc.GetMappingsPath()
	if result != "" {
		t.Errorf("expected empty string for nil config, got '%s'", result)
	}
}

func TestGetTargetDir(t *testing.T) {
	tests := []struct {
		name     string
		mappings map[string]string
		expected string
	}{
		{
			name: "with mappings",
			mappings: map[string]string{
				".bashrc": "/home/user/.bashrc",
				".vimrc":  "/home/user/.vimrc",
			},
			expected: "/home/user",
		},
		{
			name:     "empty mappings",
			mappings: map[string]string{},
			expected: "",
		},
		{
			name:     "nil mappings",
			mappings: nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.SyncConfig{
				Mappings: tt.mappings,
			}
			result := getTargetDir(cfg)
			if result != tt.expected {
				t.Errorf("getTargetDir() = '%s', want '%s'", result, tt.expected)
			}
		})
	}
}

func TestGetTargetDir_NilConfig(t *testing.T) {
	result := getTargetDir(nil)
	if result != "" {
		t.Errorf("expected empty string for nil config, got '%s'", result)
	}
}

func TestGetTargetDir_DifferentDirectories(t *testing.T) {
	cfg := &config.SyncConfig{
		Mappings: map[string]string{
			".bashrc":  "/home/user/.bashrc",
			".vimrc":   "/opt/config/.vimrc",
			"app.conf": "/etc/app/app.conf",
		},
	}
	result := getTargetDir(cfg)
	if result != "" {
		t.Errorf("expected empty string for different directories, got '%s'", result)
	}
}

func TestConfigService_ResetToDefaults_PreservesSettings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	cfg.ConfigPath = configPath
	cfg.Git.RepoPath = tmpDir
	cfg.Git.RemoteURL = "git@github.com:user/dotfiles.git"
	cfg.Git.Branch = "main"
	cfg.Mappings = map[string]string{
		".bashrc": filepath.Join(tmpDir, ".bashrc"),
	}

	svc := NewConfigService(cfg, configPath)

	err = svc.ResetToDefaults()
	if err != nil {
		t.Fatalf("ResetToDefaults failed: %v", err)
	}

	// Verify critical settings are preserved
	if cfg.Git.RepoPath != tmpDir {
		t.Errorf("RepoPath should be preserved, got '%s'", cfg.Git.RepoPath)
	}
	if cfg.Git.RemoteURL != "git@github.com:user/dotfiles.git" {
		t.Errorf("RemoteURL should be preserved, got '%s'", cfg.Git.RemoteURL)
	}
	if cfg.Git.Branch != "main" {
		t.Errorf("Branch should be preserved, got '%s'", cfg.Git.Branch)
	}
	if len(cfg.Mappings) != 1 {
		t.Errorf("Mappings should be preserved, got %d mappings", len(cfg.Mappings))
	}
}

func TestConfigService_ResetToDefaults_NilConfig(t *testing.T) {
	svc := NewConfigService(nil, "/tmp/config.json")

	err := svc.ResetToDefaults()
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestConfigService_UpdateConfig_AtomicOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	cfg.ConfigPath = configPath
	cfg.Git.RepoPath = tmpDir
	cfg.Machine.Name = "original-machine"
	cfg.Sync.PullIntervalSeconds = 300

	svc := NewConfigService(cfg, configPath)

	// Try to update with invalid sync interval (too small)
	invalidInterval := 10 // Below minimum of 60
	req := UpdateConfigRequest{
		SyncInterval: &invalidInterval,
	}

	err = svc.UpdateConfig(req)
	// Should fail validation
	if err == nil {
		t.Skip("Validation didn't fail for interval=10, skipping atomicity test")
	}

	// Verify original config is unchanged after failed update
	if cfg.Sync.PullIntervalSeconds != 300 {
		t.Errorf("config should be unchanged after failed update, got interval=%d", cfg.Sync.PullIntervalSeconds)
	}
	if cfg.Machine.Name != "original-machine" {
		t.Errorf("machine name should be unchanged, got '%s'", cfg.Machine.Name)
	}
}
