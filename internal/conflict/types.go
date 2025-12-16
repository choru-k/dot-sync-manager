// Package conflict provides conflict detection and resolution services for dotfile sync operations.
//
// It reads conflict metadata from gitmanager's .dsm/conflicts directory, where files are stored
// in timestamp-named subdirectories (e.g., 20060102T150405Z0700/) with .local, .remote, and
// optionally .base suffixes.
//
// The service provides event notifications for GUI integration via the ConflictNotifier interface.
// Services are designed for single-goroutine usage; SetNotifier should be called during initialization.
package conflict

import (
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

// Service manages conflict detection and resolution for dotfile sync operations.
// It scans gitmanager's conflict directory and provides notification events for GUI integration.
// Service is not safe for concurrent use; all methods should be called from a single goroutine.
type Service struct {
	gitMgr      *gitmanager.GitManager
	cfg         *config.SyncConfig
	conflictDir string
	notifier    ConflictNotifier
}

// ConflictInfo represents metadata about a conflict between local and remote versions of a file.
// File is the original filename (e.g., ".bashrc"), DetectedAt is when the conflict was created.
type ConflictInfo struct {
	File       string    `json:"file"`
	DetectedAt time.Time `json:"detected_at"`
	LocalMod   time.Time `json:"local_mod"`
	RemoteMod  time.Time `json:"remote_mod"`
	HasBase    bool      `json:"has_base"`
	IsDeleted  bool      `json:"is_deleted"`
}

// ConflictDetails contains the full content of conflicting versions for resolution.
// LocalContent and RemoteContent are always populated; BaseContent may be empty if no common ancestor exists.
type ConflictDetails struct {
	File          string
	LocalPath     string
	RemotePath    string
	BasePath      string
	LocalContent  []byte
	RemoteContent []byte
	BaseContent   []byte
}

// ConflictNotifier receives notifications about conflict lifecycle events.
// Implementations should handle events quickly to avoid blocking the conflict service.
//
// Thread Safety: When used with Detector, methods may be called from background
// goroutines. Implementations must be safe for concurrent use in this case.
type ConflictNotifier interface {
	// OnConflictDetected is called when CheckForConflicts finds active conflicts.
	// The conflicts slice contains all currently active conflicts.
	OnConflictDetected(conflicts []ConflictInfo)

	// OnConflictResolved is called when a specific conflict file is resolved.
	// The file parameter is the original filename (e.g., ".bashrc").
	OnConflictResolved(file string)

	// OnAllConflictsResolved is called when the last conflict is resolved.
	OnAllConflictsResolved()
}
