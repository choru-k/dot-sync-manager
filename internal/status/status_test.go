package status

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStatusManager_NewStatusManager(t *testing.T) {
	version := "1.0.0"
	configPath := "/test/config.json"

	sm := NewStatusManager(version, configPath)

	if sm == nil {
		t.Fatal("NewStatusManager returned nil")
	}

	status := sm.GetStatus()
	if status.Version != version {
		t.Errorf("Expected version %s, got %s", version, status.Version)
	}
	if status.ConfigPath != configPath {
		t.Errorf("Expected config path %s, got %s", configPath, status.ConfigPath)
	}
	if status.CurrentState != StateStarting {
		t.Errorf("Expected state %s, got %s", StateStarting, status.CurrentState)
	}
}

func TestStatusManager_StartStop(t *testing.T) {
	tempDir := t.TempDir()
	_ = filepath.Join(tempDir, "test.sock") // Socket path for context

	// Note: DefaultSocketPath is const and can't be overridden in tests
	// In production, we'd make this configurable

	sm := NewStatusManager("1.0.0", "/test/config.json")

	// Test start
	if err := sm.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if sm.GetStatus().CurrentState != StateRunning {
		t.Errorf("Expected state %s, got %s", StateRunning, sm.GetStatus().CurrentState)
	}

	// Test double start
	if err := sm.Start(); err == nil {
		t.Error("Expected error when starting already running status manager")
	}

	// Test stop
	if err := sm.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Test double stop
	if err := sm.Stop(context.Background()); err != nil {
		t.Errorf("Expected no error when stopping already stopped status manager: %v", err)
	}
}

func TestStatusManager_UpdateStatus(t *testing.T) {
	sm := NewStatusManager("1.0.0", "/test/config.json")

	// Test PID update
	sm.UpdatePID(1234)
	if sm.GetStatus().PID != 1234 {
		t.Errorf("Expected PID 1234, got %d", sm.GetStatus().PID)
	}

	// Test state update
	sm.SetState(StateSyncing)
	if sm.GetStatus().CurrentState != StateSyncing {
		t.Errorf("Expected state %s, got %s", StateSyncing, sm.GetStatus().CurrentState)
	}

	// Test error update
	testErr := fmt.Errorf("test error")
	sm.SetError(testErr)
	if sm.GetStatus().CurrentState != StateError {
		t.Errorf("Expected state %s, got %s", StateError, sm.GetStatus().CurrentState)
	}
	if sm.GetStatus().LastError != testErr.Error() {
		t.Errorf("Expected error message %s, got %s", testErr.Error(), sm.GetStatus().LastError)
	}

	// Test sync update
	files := []string{"file1", "file2"}
	sm.UpdateSync(files, nil)
	if sm.GetStatus().FilesSynced != len(files) {
		t.Errorf("Expected %d files synced, got %d", len(files), sm.GetStatus().FilesSynced)
	}
	if sm.GetStatus().LastSyncResult != "synced 2 files" {
		t.Errorf("Expected sync result 'synced 2 files', got %s", sm.GetStatus().LastSyncResult)
	}

	// Test sync update with error
	syncErr := fmt.Errorf("sync error")
	sm.UpdateSync(nil, syncErr)
	if !strings.Contains(sm.GetStatus().LastSyncResult, "sync error") {
		t.Errorf("Expected sync result to contain 'sync error', got %s", sm.GetStatus().LastSyncResult)
	}

	// Test watched paths
	paths := []string{"/path1", "/path2"}
	sm.SetWatchedPaths(paths)
	status := sm.GetStatus()
	if len(status.WatchedPaths) != len(paths) {
		t.Errorf("Expected %d watched paths, got %d", len(paths), len(status.WatchedPaths))
	}
	for i, path := range paths {
		if status.WatchedPaths[i] != path {
			t.Errorf("Expected path %s at index %d, got %s", path, i, status.WatchedPaths[i])
		}
	}
}

func TestStatusManager_ConcurrentAccess(t *testing.T) {
	sm := NewStatusManager("1.0.0", "/test/config.json")
	_ = sm.Start()
	defer func() { _ = sm.Stop(context.Background()) }()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Test concurrent status reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = sm.GetStatus()
			}
		}()
	}

	// Test concurrent status updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				sm.UpdatePID(id)
				sm.SetState(DaemonState(fmt.Sprintf("state_%d", id%5)))
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is reasonable
	status := sm.GetStatus()
	if status.CurrentState == "" {
		t.Error("Expected non-empty state after concurrent updates")
	}
}

func TestStatusManager_SocketCommunication(t *testing.T) {
	sm := NewStatusManager("1.0.0", "/test/config.json")
	sm.UpdatePID(1234)
	sm.SetState(StateRunning)
	sm.UpdateSync([]string{"file1", "file2"}, nil)

	// Start the status manager
	if err := sm.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = sm.Stop(context.Background()) }()

	// Wait a bit for the socket to be ready
	time.Sleep(100 * time.Millisecond)

	// Test getting status from socket
	status, err := GetStatusFromSocket()
	if err != nil {
		t.Fatalf("GetStatusFromSocket failed: %v", err)
	}

	if status.PID != 1234 {
		t.Errorf("Expected PID 1234, got %d", status.PID)
	}
	if status.CurrentState != StateRunning {
		t.Errorf("Expected state %s, got %s", StateRunning, status.CurrentState)
	}
	if status.FilesSynced != 2 {
		t.Errorf("Expected 2 files synced, got %d", status.FilesSynced)
	}
}

func TestStatusManager_SocketPermissions(t *testing.T) {
	sm := NewStatusManager("1.0.0", "/test/config.json")

	if err := sm.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = sm.Stop(context.Background()) }()

	// Wait a bit for socket to be created
	time.Sleep(100 * time.Millisecond)

	// Check socket permissions using the actual socket path
	socketPath := expandPath(DefaultSocketPath)
	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Failed to stat socket at %s: %v", socketPath, err)
	}

	// On Unix systems, check that socket has user-only permissions
	// 0600 = rw-------
	if info.Mode().Perm()&077 != 0 {
		t.Errorf("Socket has too permissive permissions: %v", info.Mode().Perm())
	}

	// Clean up the socket file
	_ = os.Remove(socketPath)
}

func TestStatusManager_UptimeCalculation(t *testing.T) {
	sm := NewStatusManager("1.0.0", "/test/config.json")

	startTime := time.Now()
	status1 := sm.GetStatus()
	uptime1 := time.Since(startTime)

	// Uptime should be very small for a new status manager
	if uptime1 > 1*time.Second {
		t.Errorf("Expected uptime < 1s, got %v", uptime1)
	}

	// Wait a bit and check again
	time.Sleep(100 * time.Millisecond)
	status2 := sm.GetStatus()
	_ = time.Since(startTime) // For reference

	if status2.Uptime <= status1.Uptime {
		t.Error("Expected uptime to increase over time")
	}
}

func TestGetStatusFromSocket_NoDaemon(t *testing.T) {
	// Test with non-existent socket
	_, err := GetStatusFromSocket()
	if err == nil {
		t.Error("Expected error when no daemon is running")
	}
}

func TestIsDaemonRunning(t *testing.T) {
	// Test with no daemon running
	if IsDaemonRunning() {
		t.Error("Expected IsDaemonRunning() to return false when no daemon is running")
	}

	// Start a daemon and test again
	sm := NewStatusManager("1.0.0", "/test/config.json")
	if err := sm.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait a bit for the socket to be ready
	time.Sleep(100 * time.Millisecond)

	if !IsDaemonRunning() {
		t.Error("Expected IsDaemonRunning() to return true when daemon is running")
	}

	_ = sm.Stop(context.Background())

	// Should be false again after stopping
	time.Sleep(100 * time.Millisecond)
	if IsDaemonRunning() {
		t.Error("Expected IsDaemonRunning() to return false after daemon is stopped")
	}
}

func TestStatusJSONSerialization(t *testing.T) {
	status := DaemonStatus{
		PID:            1234,
		Uptime:         5 * time.Minute,
		LastSync:       time.Now(),
		LastSyncResult: "synced 3 files",
		FilesSynced:    3,
		CurrentState:   StateRunning,
		Version:        "1.0.0",
		ConfigPath:     "/test/config.json",
		StartTime:      time.Now().Add(-5 * time.Minute),
		SyncCount:      10,
		ErrorCount:     1,
		LastError:      "test error",
		WatchedPaths:   []string{"/path1", "/path2"},
	}

	// Test JSON serialization
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Test JSON deserialization
	var unmarshaled DaemonStatus
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify key fields
	if unmarshaled.PID != status.PID {
		t.Errorf("Expected PID %d, got %d", status.PID, unmarshaled.PID)
	}
	if unmarshaled.CurrentState != status.CurrentState {
		t.Errorf("Expected state %s, got %s", status.CurrentState, unmarshaled.CurrentState)
	}
	if unmarshaled.Version != status.Version {
		t.Errorf("Expected version %s, got %s", status.Version, unmarshaled.Version)
	}
}

func TestExpandPath(t *testing.T) {
	// Test path expansion
	expanded := expandPath("~/test")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	expected := filepath.Join(home, "test")
	if expanded != expected {
		t.Errorf("Expected expanded path %s, got %s", expected, expanded)
	}

	// Test path without ~
	noTilde := "/absolute/path"
	if expandPath(noTilde) != noTilde {
		t.Errorf("Expected unchanged path %s, got %s", noTilde, expandPath(noTilde))
	}
}
