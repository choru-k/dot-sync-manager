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

// ConfigResponse represents config for GUI display.
// This is a flattened, frontend-friendly structure.
type ConfigResponse struct {
	MachineName      string            `json:"machineName"`
	RepoPath         string            `json:"repoPath"`
	TargetDir        string            `json:"targetDir"`
	SyncInterval     int               `json:"syncInterval"`  // seconds
	DebounceDelay    int               `json:"debounceDelay"` // milliseconds
	AutoSync         bool              `json:"autoSync"`
	AutoCommit       bool              `json:"autoCommit"`
	AutoPush         bool              `json:"autoPush"`
	AutoPull         bool              `json:"autoPull"`
	ConflictStrategy string            `json:"conflictStrategy"` // local, remote, manual
	Mappings         map[string]string `json:"mappings"`
	ConfigPath       string            `json:"configPath"`
	SyncignorePath   string            `json:"syncignorePath"`
}

// UpdateConfigRequest for partial config updates from GUI.
// Pointer fields allow distinguishing between "not provided" and "set to zero/empty".
// Note: TargetDir is not included as it's a derived value from mappings.
type UpdateConfigRequest struct {
	MachineName      *string `json:"machineName,omitempty"`
	RepoPath         *string `json:"repoPath,omitempty"`
	SyncInterval     *int    `json:"syncInterval,omitempty"`
	DebounceDelay    *int    `json:"debounceDelay,omitempty"`
	AutoSync         *bool   `json:"autoSync,omitempty"`
	AutoCommit       *bool   `json:"autoCommit,omitempty"`
	AutoPush         *bool   `json:"autoPush,omitempty"`
	AutoPull         *bool   `json:"autoPull,omitempty"`
	ConflictStrategy *string `json:"conflictStrategy,omitempty"`
}
