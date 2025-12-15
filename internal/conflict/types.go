// Package conflict provides conflict detection and resolution services.
package conflict

import (
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

// Service manages conflict detection and resolution.
type Service struct {
	gitMgr      *gitmanager.GitManager
	cfg         *config.SyncConfig
	conflictDir string
	notifier    ConflictNotifier
}

// ConflictInfo represents metadata about a conflict.
type ConflictInfo struct {
	File       string    `json:"file"`
	DetectedAt time.Time `json:"detectedAt"`
	LocalMod   time.Time `json:"localMod"`
	RemoteMod  time.Time `json:"remoteMod"`
	HasBase    bool      `json:"hasBase"`
	IsDeleted  bool      `json:"isDeleted"`
}

// ConflictDetails contains the full content of conflicting versions.
type ConflictDetails struct {
	File          string
	LocalPath     string
	RemotePath    string
	BasePath      string
	LocalContent  []byte
	RemoteContent []byte
	BaseContent   []byte
}

// ConflictNotifier is the interface for conflict event notifications.
type ConflictNotifier interface {
	OnConflictDetected(conflicts []ConflictInfo)
	OnConflictResolved(file string)
	OnAllConflictsResolved()
}
