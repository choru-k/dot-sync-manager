package services

import (
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/status"
)

func TestNewStatusService(t *testing.T) {
	cfg := &config.SyncConfig{}
	cfg.Machine.Name = "test-machine"
	cfg.Git.RepoPath = "/tmp/repo"

	svc := NewStatusService(cfg)

	if svc == nil {
		t.Fatal("NewStatusService returned nil")
	}
	if svc.cfg != cfg {
		t.Error("StatusService cfg not set correctly")
	}
}

func TestStatusService_GetStatus_DaemonNotRunning(t *testing.T) {
	cfg := &config.SyncConfig{}
	cfg.Machine.Name = "test-machine"
	cfg.Git.RepoPath = "/tmp/dotfiles"
	cfg.ConfigPath = "/tmp/config.json"

	svc := NewStatusService(cfg)
	resp := svc.GetStatus()

	// When daemon is not running, should return fallback response
	if resp.IsRunning {
		t.Skip("Daemon is actually running - skipping fallback test")
	}

	if resp.State != "stopped" {
		t.Errorf("expected state 'stopped', got '%s'", resp.State)
	}
	if resp.MachineName != "test-machine" {
		t.Errorf("expected machine name 'test-machine', got '%s'", resp.MachineName)
	}
	if resp.RepoPath != "/tmp/dotfiles" {
		t.Errorf("expected repo path '/tmp/dotfiles', got '%s'", resp.RepoPath)
	}
	if resp.Uptime != "Not running" {
		t.Errorf("expected uptime 'Not running', got '%s'", resp.Uptime)
	}
	if resp.LastSync != "Never" {
		t.Errorf("expected last sync 'Never', got '%s'", resp.LastSync)
	}
}

func TestStatusService_IsDaemonRunning(t *testing.T) {
	cfg := &config.SyncConfig{}
	svc := NewStatusService(cfg)

	// Just verify the function doesn't panic
	// Result depends on whether daemon is actually running
	_ = svc.IsDaemonRunning()
}

func TestStatusService_ManualSync_DaemonNotRunning(t *testing.T) {
	cfg := &config.SyncConfig{}
	svc := NewStatusService(cfg)

	err := svc.ManualSync()

	// Should return an error when daemon is not running
	if err == nil {
		t.Skip("Daemon is running - cannot test error case")
	}
}

func TestStateToString(t *testing.T) {
	tests := []struct {
		state    status.DaemonState
		expected string
	}{
		{status.StateStarting, "starting"},
		{status.StateRunning, "synced"},
		{status.StateSyncing, "syncing"},
		{status.StateIdle, "idle"},
		{status.StateStopping, "stopping"},
		{status.StateError, "error"},
		{status.DaemonState("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			result := stateToString(tt.state)
			if result != tt.expected {
				t.Errorf("stateToString(%s) = %s, want %s", tt.state, result, tt.expected)
			}
		})
	}
}

func TestStateToIcon(t *testing.T) {
	tests := []struct {
		state    status.DaemonState
		expected string
	}{
		{status.StateStarting, "🔄"},
		{status.StateRunning, "✅"},
		{status.StateSyncing, "🔄"},
		{status.StateIdle, "💤"},
		{status.StateStopping, "⏹️"},
		{status.StateError, "❌"},
		{status.DaemonState("unknown"), "❓"},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			result := stateToIcon(tt.state)
			if result != tt.expected {
				t.Errorf("stateToIcon(%s) = %s, want %s", tt.state, result, tt.expected)
			}
		})
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{"zero time", time.Time{}, "Never"},
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"1 minute ago", now.Add(-1 * time.Minute), "1 minute ago"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1 hour ago"},
		{"3 hours ago", now.Add(-3 * time.Hour), "3 hours ago"},
		{"yesterday", now.Add(-30 * time.Hour), "Yesterday"},
		{"3 days ago", now.Add(-72 * time.Hour), "3 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.input)
			if result != tt.expected {
				t.Errorf("formatRelativeTime() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{"zero", 0, "0m"},
		{"negative", -5 * time.Minute, "0m"},
		{"5 minutes", 5 * time.Minute, "5m"},
		{"1 hour", 1 * time.Hour, "1h"},
		{"1 hour 30 minutes", 90 * time.Minute, "1h 30m"},
		{"2 hours", 2 * time.Hour, "2h"},
		{"1 day", 24 * time.Hour, "1d"},
		{"1 day 5 hours", 29 * time.Hour, "1d 5h"},
		{"2 days", 48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.input)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildFallbackResponse(t *testing.T) {
	cfg := &config.SyncConfig{}
	cfg.Machine.Name = "my-laptop"
	cfg.Git.RepoPath = "/home/user/dotfiles"
	cfg.ConfigPath = "/home/user/.config/dsm/config.json"

	svc := NewStatusService(cfg)
	resp := svc.buildFallbackResponse()

	if resp.IsRunning {
		t.Error("fallback response should have IsRunning=false")
	}
	if resp.State != "stopped" {
		t.Errorf("expected state 'stopped', got '%s'", resp.State)
	}
	if resp.MachineName != "my-laptop" {
		t.Errorf("expected machine name 'my-laptop', got '%s'", resp.MachineName)
	}
	if resp.RepoPath != "/home/user/dotfiles" {
		t.Errorf("expected repo path '/home/user/dotfiles', got '%s'", resp.RepoPath)
	}
	if resp.ConfigPath != "/home/user/.config/dsm/config.json" {
		t.Errorf("expected config path, got '%s'", resp.ConfigPath)
	}
	if resp.Uptime != "Not running" {
		t.Errorf("expected uptime 'Not running', got '%s'", resp.Uptime)
	}
	if resp.LastSync != "Never" {
		t.Errorf("expected last sync 'Never', got '%s'", resp.LastSync)
	}
}

func TestMapDaemonStatus(t *testing.T) {
	cfg := &config.SyncConfig{}
	cfg.Machine.Name = "test-machine"
	cfg.Git.RepoPath = "/tmp/repo"

	svc := NewStatusService(cfg)

	ds := &status.DaemonStatus{
		PID:            12345,
		Uptime:         2*time.Hour + 30*time.Minute,
		LastSync:       time.Now().Add(-5 * time.Minute),
		LastSyncResult: "synced 3 files",
		FilesSynced:    3,
		CurrentState:   status.StateRunning,
		Version:        "1.0.0",
		ConfigPath:     "/etc/dsm/config.json",
		SyncCount:      42,
		ErrorCount:     2,
		LastError:      "",
		WatchedPaths:   []string{"/home/user/.bashrc", "/home/user/.vimrc"},
	}

	resp := svc.mapDaemonStatus(ds)

	if !resp.IsRunning {
		t.Error("expected IsRunning=true")
	}
	if resp.State != "synced" {
		t.Errorf("expected state 'synced', got '%s'", resp.State)
	}
	if resp.StateIcon != "✅" {
		t.Errorf("expected icon '✅', got '%s'", resp.StateIcon)
	}
	if resp.MachineName != "test-machine" {
		t.Errorf("expected machine name 'test-machine', got '%s'", resp.MachineName)
	}
	if resp.FilesSynced != 3 {
		t.Errorf("expected 3 files synced, got %d", resp.FilesSynced)
	}
	if resp.TrackedFiles != 2 {
		t.Errorf("expected 2 tracked files, got %d", resp.TrackedFiles)
	}
	if resp.SyncCount != 42 {
		t.Errorf("expected sync count 42, got %d", resp.SyncCount)
	}
	if resp.ErrorCount != 2 {
		t.Errorf("expected error count 2, got %d", resp.ErrorCount)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", resp.Version)
	}
	if resp.Uptime != "2h 30m" {
		t.Errorf("expected uptime '2h 30m', got '%s'", resp.Uptime)
	}
	if resp.LastSync != "5 minutes ago" {
		t.Errorf("expected last sync '5 minutes ago', got '%s'", resp.LastSync)
	}
}
