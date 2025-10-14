package config

import (
	"testing"
	"time"
)

func TestBackoffSettings_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *SyncConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid backoff settings",
			config: &SyncConfig{
				Version: "1.0",
				Machine: MachineConfig{Name: "test"},
				Git: GitConfig{
					RepoPath:    "/tmp/test",
					AuthorName:  "Test",
					AuthorEmail: "test@example.com",
				},
				Sync: SyncSettings{
					AutoSyncEnabled:     true,
					PullIntervalSeconds: 300,
					DebounceSeconds:     30,
					Backoff: &BackoffSettings{
						Enabled:            true,
						MaxDelaySeconds:    300,
						Multiplier:         2.0,
						ChurnThreshold:     10,
						ChurnWindowSeconds: 60,
						DecayResetSeconds:  300,
					},
				},
				ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
				UI:                 UIConfig{Theme: "auto"},
				Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
			},
			wantErr: false,
		},
		{
			name: "no backoff settings",
			config: &SyncConfig{
				Version: "1.0",
				Machine: MachineConfig{Name: "test"},
				Git: GitConfig{
					RepoPath:    "/tmp/test",
					AuthorName:  "Test",
					AuthorEmail: "test@example.com",
				},
				Sync: SyncSettings{
					AutoSyncEnabled:     true,
					PullIntervalSeconds: 300,
					DebounceSeconds:     30,
					Backoff:             nil,
				},
				ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
				UI:                 UIConfig{Theme: "auto"},
				Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
			},
			wantErr: false,
		},
		{
			name: "backoff max delay too small",
			config: &SyncConfig{
				Version: "1.0",
				Machine: MachineConfig{Name: "test"},
				Git: GitConfig{
					RepoPath:    "/tmp/test",
					AuthorName:  "Test",
					AuthorEmail: "test@example.com",
				},
				Sync: SyncSettings{
					AutoSyncEnabled:     true,
					PullIntervalSeconds: 300,
					DebounceSeconds:     30,
					Backoff: &BackoffSettings{
						Enabled:            true,
						MaxDelaySeconds:    15, // Less than debounce
						Multiplier:         2.0,
						ChurnThreshold:     10,
						ChurnWindowSeconds: 60,
						DecayResetSeconds:  300,
					},
				},
				ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
				UI:                 UIConfig{Theme: "auto"},
				Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
			},
			wantErr: true,
			errMsg:  "backoff max delay must be >= debounce delay",
		},
		{
			name: "backoff multiplier too low",
			config: &SyncConfig{
				Version: "1.0",
				Machine: MachineConfig{Name: "test"},
				Git: GitConfig{
					RepoPath:    "/tmp/test",
					AuthorName:  "Test",
					AuthorEmail: "test@example.com",
				},
				Sync: SyncSettings{
					AutoSyncEnabled:     true,
					PullIntervalSeconds: 300,
					DebounceSeconds:     30,
					Backoff: &BackoffSettings{
						Enabled:            true,
						MaxDelaySeconds:    300,
						Multiplier:         0.5, // Too low
						ChurnThreshold:     10,
						ChurnWindowSeconds: 60,
						DecayResetSeconds:  300,
					},
				},
				ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
				UI:                 UIConfig{Theme: "auto"},
				Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
			},
			wantErr: true,
			errMsg:  "backoff multiplier must be > 1.0",
		},
		{
			name: "backoff churn threshold zero",
			config: &SyncConfig{
				Version: "1.0",
				Machine: MachineConfig{Name: "test"},
				Git: GitConfig{
					RepoPath:    "/tmp/test",
					AuthorName:  "Test",
					AuthorEmail: "test@example.com",
				},
				Sync: SyncSettings{
					AutoSyncEnabled:     true,
					PullIntervalSeconds: 300,
					DebounceSeconds:     30,
					Backoff: &BackoffSettings{
						Enabled:            true,
						MaxDelaySeconds:    300,
						Multiplier:         2.0,
						ChurnThreshold:     0, // Invalid
						ChurnWindowSeconds: 60,
						DecayResetSeconds:  300,
					},
				},
				ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
				UI:                 UIConfig{Theme: "auto"},
				Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
			},
			wantErr: true,
			errMsg:  "backoff churn threshold must be positive",
		},
		{
			name: "backoff window negative",
			config: &SyncConfig{
				Version: "1.0",
				Machine: MachineConfig{Name: "test"},
				Git: GitConfig{
					RepoPath:    "/tmp/test",
					AuthorName:  "Test",
					AuthorEmail: "test@example.com",
				},
				Sync: SyncSettings{
					AutoSyncEnabled:     true,
					PullIntervalSeconds: 300,
					DebounceSeconds:     30,
					Backoff: &BackoffSettings{
						Enabled:            true,
						MaxDelaySeconds:    300,
						Multiplier:         2.0,
						ChurnThreshold:     10,
						ChurnWindowSeconds: -10, // Invalid
						DecayResetSeconds:  300,
					},
				},
				ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
				UI:                 UIConfig{Theme: "auto"},
				Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
			},
			wantErr: true,
			errMsg:  "backoff churn window must be positive",
		},
		{
			name: "backoff decay reset zero",
			config: &SyncConfig{
				Version: "1.0",
				Machine: MachineConfig{Name: "test"},
				Git: GitConfig{
					RepoPath:    "/tmp/test",
					AuthorName:  "Test",
					AuthorEmail: "test@example.com",
				},
				Sync: SyncSettings{
					AutoSyncEnabled:     true,
					PullIntervalSeconds: 300,
					DebounceSeconds:     30,
					Backoff: &BackoffSettings{
						Enabled:            true,
						MaxDelaySeconds:    300,
						Multiplier:         2.0,
						ChurnThreshold:     10,
						ChurnWindowSeconds: 60,
						DecayResetSeconds:  0, // Invalid
					},
				},
				ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
				UI:                 UIConfig{Theme: "auto"},
				Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
			},
			wantErr: true,
			errMsg:  "backoff decay reset must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestToSyncServiceConfig_WithBackoff(t *testing.T) {
	config := &SyncConfig{
		Version: "1.0",
		Machine: MachineConfig{Name: "test"},
		Git: GitConfig{
			RepoPath:    "/tmp/test",
			AuthorName:  "Test",
			AuthorEmail: "test@example.com",
		},
		Sync: SyncSettings{
			AutoSyncEnabled:     true,
			PullIntervalSeconds: 300,
			DebounceSeconds:     30,
			Backoff: &BackoffSettings{
				Enabled:            true,
				MaxDelaySeconds:    300,
				Multiplier:         2.5,
				ChurnThreshold:     15,
				ChurnWindowSeconds: 90,
				DecayResetSeconds:  600,
			},
		},
		ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
		UI:                 UIConfig{Theme: "auto"},
		Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
	}

	syncConfig := config.ToSyncServiceConfig()

	// Check basic fields
	if syncConfig.RepoPath != "/tmp/test" {
		t.Errorf("Expected repo path '/tmp/test', got '%s'", syncConfig.RepoPath)
	}

	if syncConfig.DebounceDelay != 30*time.Second {
		t.Errorf("Expected debounce delay 30s, got %v", syncConfig.DebounceDelay)
	}

	if !syncConfig.AutoSyncEnabled {
		t.Errorf("Expected auto sync enabled, got false")
	}

	if syncConfig.IgnoreFile != ".syncignore" {
		t.Errorf("Expected ignore file '.syncignore', got '%s'", syncConfig.IgnoreFile)
	}

	// Check backoff configuration
	if syncConfig.Backoff == nil {
		t.Fatal("Expected backoff configuration, got nil")
	}

	if syncConfig.Backoff.BaseDelay != 30*time.Second {
		t.Errorf("Expected base delay 30s, got %v", syncConfig.Backoff.BaseDelay)
	}

	if syncConfig.Backoff.MaxDelay != 300*time.Second {
		t.Errorf("Expected max delay 300s, got %v", syncConfig.Backoff.MaxDelay)
	}

	if !syncConfig.Backoff.BackoffEnabled {
		t.Errorf("Expected backoff enabled, got false")
	}

	if syncConfig.Backoff.BackoffMultiplier != 2.5 {
		t.Errorf("Expected backoff multiplier 2.5, got %f", syncConfig.Backoff.BackoffMultiplier)
	}

	if syncConfig.Backoff.ChurnThreshold != 15 {
		t.Errorf("Expected churn threshold 15, got %d", syncConfig.Backoff.ChurnThreshold)
	}

	if syncConfig.Backoff.ChurnWindow != 90*time.Second {
		t.Errorf("Expected churn window 90s, got %v", syncConfig.Backoff.ChurnWindow)
	}

	if syncConfig.Backoff.DecayResetDuration != 600*time.Second {
		t.Errorf("Expected decay reset duration 600s, got %v", syncConfig.Backoff.DecayResetDuration)
	}
}

func TestToSyncServiceConfig_WithoutBackoff(t *testing.T) {
	config := &SyncConfig{
		Version: "1.0",
		Machine: MachineConfig{Name: "test"},
		Git: GitConfig{
			RepoPath:    "/tmp/test",
			AuthorName:  "Test",
			AuthorEmail: "test@example.com",
		},
		Sync: SyncSettings{
			AutoSyncEnabled:     true,
			PullIntervalSeconds: 300,
			DebounceSeconds:     30,
			Backoff:             nil, // No backoff settings
		},
		ConflictResolution: ConflictConfig{Strategy: "manual", BackupDir: "/tmp", KeepBackupsDays: 7},
		UI:                 UIConfig{Theme: "auto"},
		Advanced:           AdvancedConfig{MaxLogSizeMB: 10},
	}

	syncConfig := config.ToSyncServiceConfig()

	// Check basic fields
	if syncConfig.RepoPath != "/tmp/test" {
		t.Errorf("Expected repo path '/tmp/test', got '%s'", syncConfig.RepoPath)
	}

	// Check backoff configuration is nil
	if syncConfig.Backoff != nil {
		t.Errorf("Expected no backoff configuration, got %v", syncConfig.Backoff)
	}
}

func TestDefaultConfig_HasBackoff(t *testing.T) {
	config, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() failed: %v", err)
	}

	// Check that default config has backoff settings
	if config.Sync.Backoff == nil {
		t.Fatal("Expected default config to have backoff settings")
	}

	// Check default values
	if !config.Sync.Backoff.Enabled {
		t.Errorf("Expected backoff enabled by default, got false")
	}

	if config.Sync.Backoff.MaxDelaySeconds != 300 {
		t.Errorf("Expected default max delay 300s, got %d", config.Sync.Backoff.MaxDelaySeconds)
	}

	if config.Sync.Backoff.Multiplier != 2.0 {
		t.Errorf("Expected default multiplier 2.0, got %f", config.Sync.Backoff.Multiplier)
	}

	if config.Sync.Backoff.ChurnThreshold != 10 {
		t.Errorf("Expected default churn threshold 10, got %d", config.Sync.Backoff.ChurnThreshold)
	}

	if config.Sync.Backoff.ChurnWindowSeconds != 60 {
		t.Errorf("Expected default churn window 60s, got %d", config.Sync.Backoff.ChurnWindowSeconds)
	}

	if config.Sync.Backoff.DecayResetSeconds != 300 {
		t.Errorf("Expected default decay reset 300s, got %d", config.Sync.Backoff.DecayResetSeconds)
	}
}
