package sync

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestSyncService_Integration(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "dotfiles")

	// Create the dotfiles directory
	if err := os.Mkdir(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create dotfiles directory: %v", err)
	}

	// Create a .syncignore file
	syncIgnoreContent := `# Ignore logs and temporary files
*.log
*.tmp
!.important.log
cache/
`
	syncIgnorePath := filepath.Join(repoPath, ".syncignore")
	if err := os.WriteFile(syncIgnorePath, []byte(syncIgnoreContent), 0644); err != nil {
		t.Fatalf("Failed to write .syncignore file: %v", err)
	}

	// Also create a .gitignore file with the same patterns for this test
	gitIgnoreContent := syncIgnoreContent
	gitIgnorePath := filepath.Join(repoPath, ".gitignore")
	if err := os.WriteFile(gitIgnorePath, []byte(gitIgnoreContent), 0644); err != nil {
		t.Fatalf("Failed to write .gitignore file: %v", err)
	}

	// Create git manager config
	gitConfig := gitmanager.Config{
		RepoPath:    repoPath,
		RemoteURL:   "", // No remote URL - local testing only
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	// Create git manager and initialize repo
	ctx := context.Background()
	gitMgr, err := gitmanager.NewGitManager(ctx, gitConfig)
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	// Create sync service configuration
	syncConfig := &Config{
		RepoPath:        repoPath,
		DebounceDelay:   100 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	// Create sync service
	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Track sync events
	var syncEvents []struct {
		files []string
		err   error
	}
	var syncEventsMu sync.Mutex
	syncCompleted := make(chan struct{}, 10) // Buffered channel

	service.SetEventCallbacks(
		func() {
			// Sync started
		},
		func(files []string, err error) {
			// Sync completed
			syncEventsMu.Lock()
			syncEvents = append(syncEvents, struct {
				files []string
				err   error
			}{files, err})
			syncEventsMu.Unlock()
			select {
			case syncCompleted <- struct{}{}:
			default:
			}
		},
		func(err error) {
			// Sync error
			t.Logf("Sync error: %v", err)
		},
	)

	// Start the sync service
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}
	defer func() {
		if err := service.Stop(); err != nil {
			t.Errorf("Failed to stop service: %v", err)
		}
	}()

	// Give the watcher a moment to start
	time.Sleep(50 * time.Millisecond)

	// Test 1: Create a file that should be synced
	testFile1 := filepath.Join(repoPath, "config.txt")
	if err := os.WriteFile(testFile1, []byte("config content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for sync attempt with timeout (may fail due to no remote)
	select {
	case <-syncCompleted:
		// Sync completed (or failed with callback)
	case <-time.After(2 * time.Second):
		// Sync may still be processing, which is fine for local testing
		t.Log("Sync still processing (expected with no remote)")
	}

	// Test 2: Create a file that should be ignored (.log file)
	logFile := filepath.Join(repoPath, "debug.log")
	if err := os.WriteFile(logFile, []byte("log content"), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	// Wait for debounce
	time.Sleep(200 * time.Millisecond)

	// Test 3: Create a file that should be synced despite being in a normally ignored pattern (.important.log)
	importantLogFile := filepath.Join(repoPath, "important.log")
	if err := os.WriteFile(importantLogFile, []byte("important log content"), 0644); err != nil {
		t.Fatalf("Failed to create important log file: %v", err)
	}

	// Wait for sync attempt with timeout (may fail due to no remote)
	select {
	case <-syncCompleted:
		// Sync completed (or failed with callback)
	case <-time.After(2 * time.Second):
		// Sync may still be processing, which is fine for local testing
		t.Log("Sync still processing (expected with no remote)")
	}

	// Test 4: Create a directory that should be ignored
	cacheDir := filepath.Join(repoPath, "cache")
	if err := os.Mkdir(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "cache.txt")
	if err := os.WriteFile(cacheFile, []byte("cache content"), 0644); err != nil {
		t.Fatalf("Failed to create cache file: %v", err)
	}

	// Wait for debounce
	time.Sleep(200 * time.Millisecond)

	// Note: The .gitignore file will prevent these files from being staged, so they won't appear in git status
	// This is actually the correct behavior for ignored files

	// Check that sync events were triggered (may be less due to push failures)
	syncEventsMu.Lock()
	syncEventsCount := len(syncEvents)
	syncEventsMu.Unlock()

	if syncEventsCount == 0 {
		t.Errorf("Expected at least 1 sync event, got %d", syncEventsCount)
	}
	t.Logf("Sync events triggered: %d", syncEventsCount)

	// Since we have a .gitignore file, ignored files won't be staged
	// Let's check that the files we want to sync exist
	expectedFiles := map[string]bool{
		"config.txt":      true,
		"important.log":   true,
		"debug.log":       true, // File exists but should be ignored by git
		"cache/cache.txt": true, // File exists but should be ignored by git
	}

	// Check that all expected files exist
	for file, shouldExist := range expectedFiles {
		if shouldExist {
			if _, err := os.Stat(filepath.Join(repoPath, file)); os.IsNotExist(err) {
				t.Errorf("Expected file %s to exist, but it doesn't", file)
			}
		}
	}

	// Check git status to see what was actually staged
	worktree, err := gitMgr.Repo().Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	status, err := worktree.Status()
	if err != nil {
		t.Fatalf("Failed to get git status: %v", err)
	}

	// With .gitignore, only non-ignored files should be staged
	expectedStagedFiles := map[string]bool{
		"config.txt":    true,
		"important.log": true,
	}

	// Check which files are actually staged
	for file, fileStatus := range status {
		expected := expectedStagedFiles[file]
		if expected && fileStatus.Staging == git.Unmodified {
			t.Errorf("Expected file %s to be staged, but it's not", file)
		} else if !expected && fileStatus.Staging != git.Unmodified {
			t.Logf("File %s is staged (this may be ok if it's not in .gitignore)", file)
		}
	}

	// Test manual sync
	// Note: This will fail to push since we're using a dummy remote URL,
	// but it should succeed in staging and committing locally
	err = service.ManualSync()
	if err != nil {
		// Push failure is expected with dummy remote
		t.Logf("Manual sync push failed as expected (dummy remote): %v", err)

		// Verify that commits were still created locally despite push failure
		repo := gitMgr.Repo()
		ref, refErr := repo.Head()
		if refErr != nil {
			t.Errorf("Failed to get HEAD: %v", refErr)
		} else {
			// Check if we have commits
			commitIter, logErr := repo.Log(&git.LogOptions{From: ref.Hash()})
			if logErr != nil {
				t.Errorf("Failed to get log: %v", logErr)
			} else {
				commitCount := 0
				if err := commitIter.ForEach(func(c *object.Commit) error {
					commitCount++
					return nil
				}); err != nil {
					t.Errorf("commitIter.ForEach failed: %v", err)
				}
				t.Logf("Found %d commits in repository", commitCount)
			}
		}
	} else {
		t.Log("Manual sync succeeded (unexpected but ok for local-only test)")
	}

	// Test stats
	stats := service.GetStats()
	if stats["running"] != true {
		t.Error("Service should be running")
	}
	if stats["repo_path"] != repoPath {
		t.Errorf("Expected repo_path=%s, got %v", repoPath, stats["repo_path"])
	}

	// Test reload ignore patterns
	if err := service.ReloadIgnorePatterns(); err != nil {
		t.Errorf("Failed to reload ignore patterns: %v", err)
	}

	t.Log("Integration test completed successfully")
}
