package services

import (
	"fmt"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/status"
)

// StatusService provides daemon status information for the GUI.
// It wraps the internal/status package and formats data for frontend display.
type StatusService struct {
	cfg *config.SyncConfig
}

// NewStatusService creates a new StatusService.
func NewStatusService(cfg *config.SyncConfig) *StatusService {
	return &StatusService{
		cfg: cfg,
	}
}

// GetStatus returns the current daemon status formatted for GUI display.
// If the daemon is not running, returns a fallback response with IsRunning=false.
func (s *StatusService) GetStatus() StatusResponse {
	daemonStatus, err := status.GetStatusFromSocket()
	if err != nil {
		// Daemon not running - return fallback
		return s.buildFallbackResponse()
	}

	return s.mapDaemonStatus(daemonStatus)
}

// IsDaemonRunning checks if the daemon is currently running.
func (s *StatusService) IsDaemonRunning() bool {
	return status.IsDaemonRunning()
}

// ManualSync triggers a manual sync operation.
// Returns an error if the daemon is not running.
func (s *StatusService) ManualSync() error {
	if !status.IsDaemonRunning() {
		return fmt.Errorf("daemon is not running")
	}
	// Note: ManualSync requires sending a command to the daemon.
	// For now, return an error indicating this is not yet implemented.
	// This will be implemented when we add command support to the socket.
	return fmt.Errorf("manual sync via GUI not yet implemented - use 'dsm sync' command")
}

// buildFallbackResponse creates a response when daemon is not running.
func (s *StatusService) buildFallbackResponse() StatusResponse {
	resp := StatusResponse{
		State:       "stopped",
		StateIcon:   stateToIcon(status.StateStopping),
		IsRunning:   false,
		Uptime:      "Not running",
		LastSync:    "Never",
		FilesSynced: 0,
	}

	// Add config info if available
	if s.cfg != nil {
		resp.MachineName = s.cfg.Machine.Name
		resp.RepoPath = s.cfg.Git.RepoPath
		resp.ConfigPath = s.cfg.ConfigPath
	}

	return resp
}

// mapDaemonStatus converts DaemonStatus to StatusResponse.
func (s *StatusService) mapDaemonStatus(ds *status.DaemonStatus) StatusResponse {
	resp := StatusResponse{
		State:        stateToString(ds.CurrentState),
		StateIcon:    stateToIcon(ds.CurrentState),
		RepoPath:     s.cfg.Git.RepoPath,
		LastSync:     formatRelativeTime(ds.LastSync),
		FilesSynced:  ds.FilesSynced,
		TrackedFiles: len(ds.WatchedPaths),
		SyncCount:    ds.SyncCount,
		ErrorCount:   ds.ErrorCount,
		Uptime:       formatDuration(ds.Uptime),
		IsRunning:    true,
		LastError:    ds.LastError,
		Version:      ds.Version,
		ConfigPath:   ds.ConfigPath,
	}

	// Add config info
	if s.cfg != nil {
		resp.MachineName = s.cfg.Machine.Name
	}

	return resp
}

// stateToString converts DaemonState to a user-friendly string.
func stateToString(state status.DaemonState) string {
	switch state {
	case status.StateStarting:
		return "starting"
	case status.StateRunning:
		return "synced"
	case status.StateSyncing:
		return "syncing"
	case status.StateIdle:
		return "idle"
	case status.StateStopping:
		return "stopping"
	case status.StateError:
		return "error"
	default:
		return "unknown"
	}
}

// stateToIcon returns an emoji icon for the given state.
func stateToIcon(state status.DaemonState) string {
	switch state {
	case status.StateStarting:
		return "🔄"
	case status.StateRunning:
		return "✅"
	case status.StateSyncing:
		return "🔄"
	case status.StateIdle:
		return "💤"
	case status.StateStopping:
		return "⏹️"
	case status.StateError:
		return "❌"
	default:
		return "❓"
	}
}

// formatRelativeTime formats a time as a human-readable relative string.
// Examples: "just now", "5 minutes ago", "2 hours ago", "Yesterday"
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 48*time.Hour:
		return "Yesterday"
	default:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
}

// formatDuration formats a duration as a human-readable string.
// Examples: "5m", "2h 30m", "1d 5h"
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	switch {
	case days > 0:
		if hours > 0 {
			return fmt.Sprintf("%dd %dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	case hours > 0:
		if mins > 0 {
			return fmt.Sprintf("%dh %dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}
