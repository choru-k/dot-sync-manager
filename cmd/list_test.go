package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSymlinkStatus(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a source file
	sourceFile := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(sourceFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	tests := []struct {
		name           string
		setup          func() (sourcePath, targetPath string)
		expectedStatus string
		expectedIcon   string
	}{
		{
			name: "valid symlink",
			setup: func() (string, string) {
				target := filepath.Join(tmpDir, "valid-link")
				if err := os.Symlink(sourceFile, target); err != nil {
					t.Fatalf("Failed to create symlink: %v", err)
				}
				return sourceFile, target
			},
			expectedStatus: "linked",
			expectedIcon:   "✓",
		},
		{
			name: "target does not exist",
			setup: func() (string, string) {
				return sourceFile, filepath.Join(tmpDir, "nonexistent")
			},
			expectedStatus: "not linked",
			expectedIcon:   "○",
		},
		{
			name: "target is not a symlink",
			setup: func() (string, string) {
				target := filepath.Join(tmpDir, "regular-file")
				if err := os.WriteFile(target, []byte("not a link"), 0644); err != nil {
					t.Fatalf("Failed to create regular file: %v", err)
				}
				return sourceFile, target
			},
			expectedStatus: "not a symlink",
			expectedIcon:   "○",
		},
		{
			name: "broken symlink pointing to source location",
			setup: func() (string, string) {
				// Create symlink pointing to sourceFile, then delete source
				differentSource := filepath.Join(tmpDir, "different-source.txt")
				if err := os.WriteFile(differentSource, []byte("temp"), 0644); err != nil {
					t.Fatalf("Failed to create temp source: %v", err)
				}
				target := filepath.Join(tmpDir, "broken-link")
				if err := os.Symlink(differentSource, target); err != nil {
					t.Fatalf("Failed to create symlink: %v", err)
				}
				// Delete the source file
				if err := os.Remove(differentSource); err != nil {
					t.Fatalf("Failed to remove source: %v", err)
				}
				// Now check against a different source location
				return sourceFile, target
			},
			expectedStatus: "points elsewhere",
			expectedIcon:   "✗",
		},
		{
			name: "symlink points elsewhere",
			setup: func() (string, string) {
				otherFile := filepath.Join(tmpDir, "other.txt")
				if err := os.WriteFile(otherFile, []byte("other"), 0644); err != nil {
					t.Fatalf("Failed to create other file: %v", err)
				}
				target := filepath.Join(tmpDir, "wrong-link")
				if err := os.Symlink(otherFile, target); err != nil {
					t.Fatalf("Failed to create symlink: %v", err)
				}
				return sourceFile, target
			},
			expectedStatus: "points elsewhere",
			expectedIcon:   "✗",
		},
		{
			name: "relative symlink",
			setup: func() (string, string) {
				target := filepath.Join(tmpDir, "relative-link")
				// Create relative symlink
				if err := os.Symlink("source.txt", target); err != nil {
					t.Fatalf("Failed to create relative symlink: %v", err)
				}
				return sourceFile, target
			},
			expectedStatus: "linked",
			expectedIcon:   "✓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourcePath, targetPath := tt.setup()
			status, icon := checkSymlinkStatus(sourcePath, targetPath)

			if status != tt.expectedStatus {
				t.Errorf("Expected status %q, got %q", tt.expectedStatus, status)
			}
			if icon != tt.expectedIcon {
				t.Errorf("Expected icon %q, got %q", tt.expectedIcon, icon)
			}
		})
	}
}

func TestCheckSymlinkStatusWithTildePath(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(sourceFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create symlink in temp dir
	target := filepath.Join(tmpDir, "link")
	if err := os.Symlink(sourceFile, target); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Test with a path that would need expansion (but won't match our test setup)
	// This tests the path expansion logic
	status, icon := checkSymlinkStatus(sourceFile, "~/nonexistent-path-for-testing")

	// Should return "not linked" since the expanded path won't exist
	if status != "not linked" {
		t.Errorf("Expected 'not linked' for non-existent tilde path, got %q", status)
	}
	if icon != "○" {
		t.Errorf("Expected '○' icon for non-existent path, got %q", icon)
	}
}

func TestCheckSymlinkStatusPathExpansionError(t *testing.T) {
	// Test with invalid path that would fail expansion
	// This is difficult to trigger since ExpandPath is quite robust
	// But we can test the error handling path exists
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	
	status, icon := checkSymlinkStatus(sourceFile, "~/valid-path")
	
	// Should handle the path (may not exist, but expansion should work)
	// This verifies the function doesn't panic on tilde paths
	if status == "" {
		t.Error("Expected non-empty status")
	}
	if icon == "" {
		t.Error("Expected non-empty icon")
	}
}
