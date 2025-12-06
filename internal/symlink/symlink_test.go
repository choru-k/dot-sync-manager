package symlink_test

import (
	"strings"
	"testing"

	"github.com/choru-k/dot-sync-manager/internal/config"
	"github.com/choru-k/dot-sync-manager/internal/symlink"
)

func TestPackage(t *testing.T) {
	// Verify package compiles
}

func TestManager_TypeExists(t *testing.T) {
	var _ *symlink.Manager = nil
}

func TestNewManager_NilConfig(t *testing.T) {
	mgr, err := symlink.NewManager(nil)
	if err == nil {
		t.Fatal("NewManager(nil) should return error")
	}
	if mgr != nil {
		t.Errorf("NewManager(nil) should return nil manager, got %v", mgr)
	}
	if !strings.Contains(err.Error(), "SYMLINK_CONFIG_REQUIRED") {
		t.Errorf("error should contain error ID, got: %v", err)
	}
}

func TestNewManager_Success(t *testing.T) {
	cfg := &config.SyncConfig{}

	mgr, err := symlink.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() unexpected error: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewManager() returned nil manager")
	}
}

func TestMappingStatus_TypeExists(t *testing.T) {
	var _ = symlink.MappingStatus{}
}

func TestMappingStatus_Fields(t *testing.T) {
	status := symlink.MappingStatus{
		RepoPath:   "/repo/path",
		TargetPath: "/target/path",
		Status:     symlink.StateValid,
		Error:      "",
	}

	if status.RepoPath != "/repo/path" {
		t.Errorf("RepoPath mismatch")
	}
	if status.TargetPath != "/target/path" {
		t.Errorf("TargetPath mismatch")
	}
	if status.Status != symlink.StateValid {
		t.Errorf("Status mismatch, got %v, want %v", status.Status, symlink.StateValid)
	}
	if status.Error != "" {
		t.Errorf("Error mismatch")
	}
}
