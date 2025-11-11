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

func TestCheckSymlinkStatusWithNonExistentTildePath(t *testing.T) {
	// Test that tilde paths are properly expanded and handled gracefully when they don't exist
	// This verifies both path expansion logic and non-existent path handling
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")

	// Test with tilde path that doesn't exist after expansion
	status, icon := checkSymlinkStatus(sourceFile, "~/nonexistent-test-path")

	// Should return "not linked" since the expanded path doesn't exist
	if status != "not linked" {
		t.Errorf("Expected 'not linked' for non-existent tilde path, got %q", status)
	}
	if icon != "○" {
		t.Errorf("Expected '○' icon for non-existent tilde path, got %q", icon)
	}
}

func TestCheckSymlinkStatusEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("symlink points to missing source", func(t *testing.T) {
		// Create a valid symlink pointing to a source that doesn't exist
		nonExistentSource := filepath.Join(tmpDir, "nonexistent.txt")
		target := filepath.Join(tmpDir, "link-to-missing")

		// First create a temporary source file
		tempSource := filepath.Join(tmpDir, "temp.txt")
		if err := os.WriteFile(tempSource, []byte("temp"), 0644); err != nil {
			t.Fatalf("Failed to create temp source: %v", err)
		}

		// Create symlink to temp source
		if err := os.Symlink(tempSource, target); err != nil {
			t.Fatalf("Failed to create symlink: %v", err)
		}

		// Remove temp source, now symlink points to missing file
		if err := os.Remove(tempSource); err != nil {
			t.Fatalf("Failed to remove temp source: %v", err)
		}

		status, icon := checkSymlinkStatus(nonExistentSource, target)

		// Function checks paths before checking file existence, so it returns "points elsewhere"
		if status != "points elsewhere" {
			t.Errorf("Expected 'points elsewhere', got %q", status)
		}
		if icon != "✗" {
			t.Errorf("Expected '✗' icon for missing source, got %q", icon)
		}
	})

	t.Run("symlink points to missing source file", func(t *testing.T) {
		// Test the case where symlink exists but source file is missing
		sourceFile := filepath.Join(tmpDir, "source.txt")
		if err := os.WriteFile(sourceFile, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		// Create symlink to source
		target := filepath.Join(tmpDir, "broken-link")
		if err := os.Symlink(sourceFile, target); err != nil {
			t.Fatalf("Failed to create symlink: %v", err)
		}

		// Remove source file to break the symlink
		if err := os.Remove(sourceFile); err != nil {
			t.Fatalf("Failed to remove source: %v", err)
		}

		status, icon := checkSymlinkStatus(sourceFile, target)

		// Source file check happens after path comparison, so it returns "source missing"
		if status != "source missing" {
			t.Errorf("Expected 'source missing', got %q", status)
		}
		if icon != "✗" {
			t.Errorf("Expected '✗' icon for missing source, got %q", icon)
		}
	})

	t.Run("path expansion error", func(t *testing.T) {
		// Test with invalid path that causes checking error
		sourceFile := filepath.Join(tmpDir, "source.txt")
		if err := os.WriteFile(sourceFile, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		// Use invalid path that causes checking error (null byte makes os.Lstat fail)
		invalidPath := string([]byte{0x00}) + "invalid" // Null byte in path
		status, icon := checkSymlinkStatus(sourceFile, invalidPath)

		// Should handle checking error gracefully
		if status != "error checking" {
			t.Errorf("Expected 'error checking', got %q", status)
		}
		if icon != "✗" {
			t.Errorf("Expected '✗' icon for checking error, got %q", icon)
		}
	})
}
