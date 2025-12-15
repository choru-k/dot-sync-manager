package conflict

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

// timestampFormat is the format used by gitmanager for conflict directories.
const timestampFormat = "20060102T150405Z0700"

// NewService creates a new conflict service.
// The gitMgr parameter may be nil if git operations are not needed.
// The cfg.Git.RepoPath must be non-empty and absolute.
func NewService(gitMgr *gitmanager.GitManager, cfg *config.SyncConfig) *Service {
	repoPath := cfg.Git.RepoPath
	conflictDir := filepath.Join(repoPath, ".dsm", "conflicts")

	return &Service{
		gitMgr:      gitMgr,
		cfg:         cfg,
		conflictDir: conflictDir,
	}
}

// GetConflicts returns all active conflicts by scanning gitmanager's conflict directory.
// Conflicts are stored in timestamp-named subdirectories (.dsm/conflicts/<timestamp>/)
// with files named [path].local, [path].remote, and optionally [path].base.
// When multiple timestamps exist for the same file, only the most recent is returned.
// Returns empty slice (not error) if no conflicts exist or directory doesn't exist.
func (s *Service) GetConflicts() ([]ConflictInfo, error) {
	var conflicts []ConflictInfo

	// Read conflict directory for timestamp subdirectories
	entries, err := os.ReadDir(s.conflictDir)
	if os.IsNotExist(err) {
		return conflicts, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read conflict directory: %w", err)
	}

	// Track unique files across all timestamp directories
	fileSet := make(map[string]ConflictInfo)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse timestamp from directory name
		detectedAt, err := time.Parse(timestampFormat, entry.Name())
		if err != nil {
			// Not a valid timestamp directory, skip
			continue
		}

		timestampDir := filepath.Join(s.conflictDir, entry.Name())
		files, err := os.ReadDir(timestampDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read timestamp directory %s: %w", entry.Name(), err)
		}

		// Find unique files by looking for .remote suffix
		for _, f := range files {
			if f.IsDir() {
				continue
			}

			name := f.Name()
			if !strings.HasSuffix(name, ".remote") {
				continue
			}

			// Extract the original filename
			originalFile := strings.TrimSuffix(name, ".remote")

			// Check if base file exists
			basePath := filepath.Join(timestampDir, originalFile+".base")
			_, baseErr := os.Stat(basePath)
			hasBase := baseErr == nil

			// Check if local is deleted marker
			localPath := filepath.Join(timestampDir, originalFile+".local")
			localContent, localErr := os.ReadFile(localPath)
			isDeleted := localErr == nil && strings.HasPrefix(string(localContent), "<<local deleted")

			// Get modification times from files
			var localMod, remoteMod time.Time
			if localInfo, err := os.Stat(localPath); err == nil {
				localMod = localInfo.ModTime()
			}
			remotePath := filepath.Join(timestampDir, originalFile+".remote")
			if remoteInfo, err := os.Stat(remotePath); err == nil {
				remoteMod = remoteInfo.ModTime()
			}

			info := ConflictInfo{
				File:       originalFile,
				DetectedAt: detectedAt,
				LocalMod:   localMod,
				RemoteMod:  remoteMod,
				HasBase:    hasBase,
				IsDeleted:  isDeleted,
			}

			// Keep the most recent conflict for each file
			if existing, ok := fileSet[originalFile]; ok {
				if detectedAt.After(existing.DetectedAt) {
					fileSet[originalFile] = info
				}
			} else {
				fileSet[originalFile] = info
			}
		}
	}

	// Convert map to slice
	for _, info := range fileSet {
		conflicts = append(conflicts, info)
	}

	// Sort by file name for consistent output
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].File < conflicts[j].File
	})

	return conflicts, nil
}

// HasConflicts returns true if there are any active conflicts.
// Returns false on error (fails closed to avoid blocking operations).
func (s *Service) HasConflicts() bool {
	conflicts, err := s.GetConflicts()
	if err != nil {
		return false
	}
	return len(conflicts) > 0
}

// GetConflictDir returns the path to the conflict directory (.dsm/conflicts).
// This is where gitmanager stores conflict artifacts during sync operations.
func (s *Service) GetConflictDir() string {
	return s.conflictDir
}

// GetConflictDetails returns the full content of conflicting versions for a file.
// The file parameter is the original filename (e.g., ".bashrc").
// It finds the latest timestamp directory containing the requested file.
// Returns error if the conflict doesn't exist or files cannot be read.
func (s *Service) GetConflictDetails(file string) (*ConflictDetails, error) {
	// Find the latest timestamp directory containing this file
	entries, err := os.ReadDir(s.conflictDir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("conflict not found: %s", file)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read conflict directory: %w", err)
	}

	var latestDir string
	var latestTime time.Time

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		detectedAt, err := time.Parse(timestampFormat, entry.Name())
		if err != nil {
			continue
		}

		// Check if this directory contains the file
		remotePath := filepath.Join(s.conflictDir, entry.Name(), file+".remote")
		if _, err := os.Stat(remotePath); os.IsNotExist(err) {
			continue
		}

		if latestDir == "" || detectedAt.After(latestTime) {
			latestDir = filepath.Join(s.conflictDir, entry.Name())
			latestTime = detectedAt
		}
	}

	if latestDir == "" {
		return nil, fmt.Errorf("conflict not found: %s", file)
	}

	details := &ConflictDetails{
		File:       file,
		LocalPath:  filepath.Join(latestDir, file+".local"),
		RemotePath: filepath.Join(latestDir, file+".remote"),
		BasePath:   filepath.Join(latestDir, file+".base"),
	}

	// Read local version (required)
	localData, err := os.ReadFile(details.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read local conflict content: %w", err)
	}
	details.LocalContent = localData

	// Read remote version (required)
	remoteData, err := os.ReadFile(details.RemotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote conflict content: %w", err)
	}
	details.RemoteContent = remoteData

	// Read base version (optional)
	baseData, err := os.ReadFile(details.BasePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read base conflict content: %w", err)
		}
		// Base file doesn't exist, that's OK
	} else {
		details.BaseContent = baseData
	}

	return details, nil
}

// CheckForConflicts scans for conflicts and fires events if a notifier is set.
// Calls OnConflictDetected with the full list when conflicts exist.
// Returns the list of detected conflicts.
func (s *Service) CheckForConflicts() ([]ConflictInfo, error) {
	conflicts, err := s.GetConflicts()
	if err != nil {
		return nil, err
	}

	if len(conflicts) > 0 {
		s.notifyConflictDetected(conflicts)
	}

	return conflicts, nil
}
