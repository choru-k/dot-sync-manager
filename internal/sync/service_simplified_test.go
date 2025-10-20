package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

func TestSimplifiedServiceStateTransitions(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create git manager
	gitMgr, err := gitmanager.New(gitmanager.Config{
		RepoPath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	// Create simplified service
	service, err := NewSimplified(gitMgr, &Config{
		RepoPath:      tmpDir,
		DebounceDelay: 100 * time.Millisecond,
		AutoSyncEnabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Test initial state
	if service.GetState() != stateStopped {
		t.Errorf("Expected stopped state, got %s", service.GetState())
	}
	if service.IsRunning() {
		t.Error("Expected IsRunning() to be false")
	}

	// Test start
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	if service.GetState() != stateRunning {
		t.Errorf("Expected running state, got %s", service.GetState())
	}
	if !service.IsRunning() {
		t.Error("Expected IsRunning() to be true")
	}

	// Test duplicate start
	err = service.Start()
	if err == nil {
		t.Error("Expected error when starting already running service")
	}

	// Test stop
	if err := service.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // Give it time to stop

	if service.GetState() != stateStopped {
		t.Errorf("Expected stopped state, got %s", service.GetState())
	}
	if service.IsRunning() {
		t.Error("Expected IsRunning() to be false")
	}

	// Test duplicate stop (should be safe)
	if err := service.Stop(); err != nil {
		t.Logf("Duplicate Stop() error: %v", err)
	} // Should not panic or block
}

func TestSimplifiedServiceConcurrentAccess(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create git manager
	gitMgr, err := gitmanager.New(gitmanager.Config{
		RepoPath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	// Create simplified service
	service, err := NewSimplified(gitMgr, &Config{
		RepoPath:      tmpDir,
		DebounceDelay: 100 * time.Millisecond,
		AutoSyncEnabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Start service
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer func() {
		if err := service.Stop(); err != nil {
			t.Errorf("Failed to stop service: %v", err)
		}
	}()

	// Test concurrent state checks
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			_ = service.IsRunning()
			_ = service.GetState()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for goroutines")
		}
	}

	// Test concurrent manual sync
	syncDone := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			syncDone <- service.ManualSync()
		}()
	}

	// Check results
	for i := 0; i < 10; i++ {
		select {
		case err := <-syncDone:
			if err != nil {
				t.Errorf("Manual sync failed: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for manual sync")
		}
	}
}

func TestSimplifiedServiceStats(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create git manager
	gitMgr, err := gitmanager.New(gitmanager.Config{
		RepoPath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	// Create simplified service
	service, err := NewSimplified(gitMgr, &Config{
		RepoPath:      tmpDir,
		DebounceDelay: 100 * time.Millisecond,
		AutoSyncEnabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Get stats
	stats := service.GetStats()

	// Verify required fields
	requiredFields := []string{
		"state", "running", "repo_path", "debounce_delay",
		"auto_sync", "pending_debounces", "ignore_patterns",
		"advanced_debouncer",
	}

	for _, field := range requiredFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Missing required field in stats: %s", field)
		}
	}

	// Verify state is correct in stats
	if stats["state"] != "stopped" {
		t.Errorf("Expected state 'stopped', got %v", stats["state"])
	}

	// Start service and check stats
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	stats = service.GetStats()
	if stats["state"] != "running" {
		t.Errorf("Expected state 'running', got %v", stats["state"])
	}
	if stats["running"] != true {
		t.Error("Expected running to be true")
	}

	if err := service.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestSimplifiedServiceFileWatching(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create git manager
	gitMgr, err := gitmanager.New(gitmanager.Config{
		RepoPath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create git manager: %v", err)
	}

	// Create a file to watch
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Track sync events
	syncStarted := make(chan bool)
	syncCompleted := make(chan bool)

	// Create simplified service
	service, err := NewSimplified(gitMgr, &Config{
		RepoPath:      tmpDir,
		DebounceDelay: 50 * time.Millisecond,
		AutoSyncEnabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Set callbacks
	service.SetEventCallbacks(
		func() { syncStarted <- true },
		func(files []string, err error) { syncCompleted <- true },
		func(error) {},
	)

	// Start service
	err = service.Start()
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer func() {
		if err := service.Stop(); err != nil {
			t.Errorf("Failed to stop service: %v", err)
		}
	}()

	// Modify file
	err = os.WriteFile(testFile, []byte("modified"), 0644)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Wait for sync
	select {
	case <-syncStarted:
	case <-time.After(200 * time.Millisecond):
		t.Error("Sync did not start within timeout")
	}

	select {
	case <-syncCompleted:
	case <-time.After(500 * time.Millisecond):
		t.Error("Sync did not complete within timeout")
	}
}

// Benchmark comparing original vs simplified implementation
func BenchmarkStateCheck(b *testing.B) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dsm-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create git manager
	gitMgr, err := gitmanager.New(gitmanager.Config{
		RepoPath: tmpDir,
	})
	if err != nil {
		b.Fatalf("Failed to create git manager: %v", err)
	}

	// Create simplified service
	service, err := NewSimplified(gitMgr, &Config{
		RepoPath:      tmpDir,
		DebounceDelay: 100 * time.Millisecond,
		AutoSyncEnabled: true,
	})
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	err = service.Start()
	if err != nil {
		b.Fatalf("Failed to start service: %v", err)
	}
	defer func() {
		if err := service.Stop(); err != nil {
			t.Errorf("Failed to stop service: %v", err)
		}
	}()

	// Benchmark state checks
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = service.IsRunning()
			_ = service.GetState()
		}
	})
}