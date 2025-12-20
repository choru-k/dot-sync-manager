// Package services provides GUI service bindings for Wails v3.
// These services wrap internal packages and expose them to the frontend.
package services

// StatusResponse represents the daemon status for GUI display.
// This is a frontend-friendly format with pre-formatted strings.
type StatusResponse struct {
	MachineName   string         `json:"machineName"`
	State         string         `json:"state"`     // synced, syncing, error, stopped, idle
	StateIcon     string         `json:"stateIcon"` // emoji for UI display
	RepoPath      string         `json:"repoPath"`
	LastSync      string         `json:"lastSync"` // "5 minutes ago" or "Never"
	FilesSynced   int            `json:"filesSynced"`
	RecentChanges []RecentChange `json:"recentChanges"`
	TrackedFiles  int            `json:"trackedFiles"`
	SyncCount     int64          `json:"syncCount"`
	ErrorCount    int64          `json:"errorCount"`
	Uptime        string         `json:"uptime"` // "2h 30m" or "Not running"
	IsRunning     bool           `json:"isRunning"`
	LastError     string         `json:"lastError,omitempty"`
	Version       string         `json:"version"`
	ConfigPath    string         `json:"configPath"`
}

// RecentChange represents a single file change for display.
type RecentChange struct {
	File      string `json:"file"`
	Action    string `json:"action"`    // added, modified, deleted
	Timestamp string `json:"timestamp"` // relative time string
}
