package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateFileSecurely(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("test content")

	// Test creating a new file
	if err := CreateFileSecurely(testFile, testData, 0644); err != nil {
		t.Fatalf("CreateFileSecurely() failed: %v", err)
	}

	// Verify file exists and has correct content
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}
	if string(data) != string(testData) {
		t.Errorf("File content mismatch: expected %q, got %q", testData, data)
	}

	// Test creating same file again should fail
	if err := CreateFileSecurely(testFile, []byte("new content"), 0644); err == nil {
		t.Error("CreateFileSecurely() should have failed when file already exists")
	}

	// Test creating file in non-existent directory (should create parent dirs)
	nestedFile := filepath.Join(tmpDir, "subdir", "nested.txt")
	if err := CreateFileSecurely(nestedFile, testData, 0644); err != nil {
		t.Fatalf("CreateFileSecurely() failed for nested path: %v", err)
	}

	// Verify nested file exists
	if _, err := os.Stat(nestedFile); err != nil {
		t.Errorf("Nested file was not created: %v", err)
	}
}

func TestCreateDirectorySecurely(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "testdir")

	// Test creating a new directory
	if err := CreateDirectorySecurely(testDir, 0755); err != nil {
		t.Fatalf("CreateDirectorySecurely() failed: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("Failed to stat created directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Created path is not a directory")
	}

	// Test creating same directory again should succeed (idempotent)
	if err := CreateDirectorySecurely(testDir, 0755); err != nil {
		t.Errorf("CreateDirectorySecurely() should succeed when directory already exists: %v", err)
	}

	// Test creating directory where file exists should fail
	testFile := filepath.Join(tmpDir, "blocker.txt")
	if err := os.WriteFile(testFile, []byte("block"), 0644); err != nil {
		t.Fatalf("Failed to create blocker file: %v", err)
	}

	if err := CreateDirectorySecurely(testFile, 0755); err == nil {
		t.Error("CreateDirectorySecurely() should fail when file exists at target path")
	}
}
