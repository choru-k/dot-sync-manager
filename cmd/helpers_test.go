package cmd

import (
	"os"
	"reflect"
	"runtime"
	"testing"
)

func TestCheckDirectoryExists(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantExists  bool
		wantError   bool
		description string
	}{
		{
			name:        "existing_directory",
			path:        ".",
			wantExists:  true,
			wantError:   false,
			description: "Current directory should exist",
		},
		{
			name:        "nonexistent_directory",
			path:        "/nonexistent/path/that/should/not/exist",
			wantExists:  false,
			wantError:   false,
			description: "Nonexistent directory should return false, no error",
		},
		{
			name:        "file_instead_of_directory",
			path:        "helpers.go",
			wantExists:  false,
			wantError:   false,
			description: "File should not be considered a directory",
		},
		{
			name:        "empty_path",
			path:        "",
			wantExists:  false,
			wantError:   false,
			description: "Empty path should be treated as non-existent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExists, err := checkDirectoryExists(tt.path)

			if tt.wantError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if gotExists != tt.wantExists {
				t.Errorf("checkDirectoryExists() = %v, want %v", gotExists, tt.wantExists)
			}
		})
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		expectedCmd  string
		expectedArgs []string
		description  string
	}{
		{
			name:         "simple_command",
			command:      "nano",
			expectedCmd:  "nano",
			expectedArgs: []string{},
			description:  "Simple command without arguments",
		},
		{
			name:         "command_with_single_arg",
			command:      "nano file.txt",
			expectedCmd:  "nano",
			expectedArgs: []string{"file.txt"},
			description:  "Command with single argument",
		},
		{
			name:         "command_with_multiple_args",
			command:      "code --new-window file.txt",
			expectedCmd:  "code",
			expectedArgs: []string{"--new-window", "file.txt"},
			description:  "Command with multiple arguments",
		},
		{
			name:         "command_with_quotes",
			command:      `code "file name with spaces.txt"`,
			expectedCmd:  "code",
			expectedArgs: []string{`"file`, `name`, `with`, `spaces.txt"`},
			description:  "Command with quoted arguments (simple splitting)",
		},
		{
			name:         "empty_command",
			command:      "",
			expectedCmd:  "",
			expectedArgs: nil,
			description:  "Empty command should return empty values",
		},
		{
			name:         "whitespace_only_command",
			command:      "   ",
			expectedCmd:  "",
			expectedArgs: nil,
			description:  "Whitespace-only command should return empty values",
		},
		{
			name:         "command_with_extra_whitespace",
			command:      "  nano   file.txt   ",
			expectedCmd:  "nano",
			expectedArgs: []string{"file.txt"},
			description:  "Command with extra whitespace should be trimmed",
		},
		{
			name:         "complex_editor_command",
			command:      "code --wait --goto line:col file.txt",
			expectedCmd:  "code",
			expectedArgs: []string{"--wait", "--goto", "line:col", "file.txt"},
			description:  "Complex editor command with multiple flags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotArgs := parseCommand(tt.command)

			if gotCmd != tt.expectedCmd {
				t.Errorf("parseCommand() cmd = %q, want %q", gotCmd, tt.expectedCmd)
			}

			if !reflect.DeepEqual(gotArgs, tt.expectedArgs) {
				t.Errorf("parseCommand() args = %v, want %v", gotArgs, tt.expectedArgs)
			}
		})
	}
}

func TestIsAllowedEditor(t *testing.T) {
	allowedEditors := []string{"vi", "vim", "emacs", "nano", "code", "subl"}

	tests := []struct {
		name           string
		editor         string
		allowed        []string
		expectedResult bool
		description    string
	}{
		{
			name:           "allowed_editor_vi",
			editor:         "vi",
			allowed:        allowedEditors,
			expectedResult: true,
			description:    "vi should be in allowed list",
		},
		{
			name:           "allowed_editor_code",
			editor:         "code",
			allowed:        allowedEditors,
			expectedResult: true,
			description:    "code should be in allowed list",
		},
		{
			name:           "not_allowed_editor",
			editor:         "malicious-editor",
			allowed:        allowedEditors,
			expectedResult: false,
			description:    "Unknown editor should not be allowed",
		},
		{
			name:           "case_sensitive_check",
			editor:         "VI",
			allowed:        allowedEditors,
			expectedResult: false,
			description:    "Editor check should be case sensitive",
		},
		{
			name:           "empty_editor_list",
			editor:         "nano",
			allowed:        []string{},
			expectedResult: false,
			description:    "Empty allowed list should not allow any editor",
		},
		{
			name:           "partial_match_not_allowed",
			editor:         "vim-enhanced",
			allowed:        allowedEditors,
			expectedResult: false,
			description:    "Partial matches should not be allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAllowedEditor(tt.editor, tt.allowed)
			if result != tt.expectedResult {
				t.Errorf("isAllowedEditor() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestHasCommand(t *testing.T) {
	// Test with commands that should exist on most systems
	commonCommands := []string{"sh", "echo"}

	// Test with commands that likely don't exist
	uncommonCommands := []string{"definitely-not-a-real-command-12345", "nonexistent-binary"}

	for _, cmd := range commonCommands {
		t.Run("existing_command_"+cmd, func(t *testing.T) {
			result := hasCommand(cmd)
			if !result {
				t.Errorf("hasCommand(%q) returned false for command that should exist", cmd)
			}
		})
	}

	for _, cmd := range uncommonCommands {
		t.Run("nonexistent_command_"+cmd, func(t *testing.T) {
			result := hasCommand(cmd)
			if result {
				t.Errorf("hasCommand(%q) returned true for nonexistent command", cmd)
			}
		})
	}
}

func TestGetDefaultEditor(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "returns_valid_editor",
			description: "Should return a valid editor name",
			testFunc: func(t *testing.T) {
				editor := getDefaultEditor()
				if editor == "" {
					t.Error("getDefaultEditor() returned empty string")
				}
			},
		},
		{
			name:        "platform_specific_defaults",
			description: "Should return platform-appropriate defaults",
			testFunc: func(t *testing.T) {
				editor := getDefaultEditor()

				switch runtime.GOOS {
				case "darwin":
					if editor != "open" {
						t.Errorf("Expected 'open' on macOS, got %q", editor)
					}
				case "windows":
					if editor != "notepad" {
						t.Errorf("Expected 'notepad' on Windows, got %q", editor)
					}
				default: // linux and others
					validLinuxEditors := []string{"code", "nano", "vim", "vi"}
					found := false
					for _, valid := range validLinuxEditors {
						if editor == valid {
							found = true
							break
						}
					}
					if !found && editor != "nano" { // fallback
						t.Errorf("Expected one of %v on Linux, got %q", validLinuxEditors, editor)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func TestGetDefaultEditorForFile(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test_file_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("Warning: failed to remove temp file: %v", err)
		}
	}()
	defer func() {
		if err := tmpFile.Close(); err != nil {
			t.Logf("Warning: failed to close temp file: %v", err)
		}
	}()

	tests := []struct {
		name        string
		filePath    string
		description string
	}{
		{
			name:        "existing_file",
			filePath:    tmpFile.Name(),
			description: "Should handle existing file paths",
		},
		{
			name:        "nonexistent_file",
			filePath:    "/nonexistent/path/file.txt",
			description: "Should handle nonexistent file paths",
		},
		{
			name:        "empty_path",
			filePath:    "",
			description: "Should handle empty paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := getDefaultEditorForFile(tt.filePath)
			if editor == "" {
				t.Errorf("getDefaultEditorForFile(%q) returned empty string", tt.filePath)
			}
		})
	}
}

func TestPrintHelperFunctions(t *testing.T) {
	// These tests mainly verify that the print functions don't panic
	// and produce output. We can't easily test the exact output due to
	// the global noEmoji variable and terminal escape sequences.

	tests := []struct {
		name        string
		description string
		testFunc    func()
	}{
		{
			name:        "printSuccess",
			description: "Should print success message without panicking",
			testFunc: func() {
				printSuccess("Test operation completed successfully")
				printSuccess("Test with %s formatting", "param")
			},
		},
		{
			name:        "printError",
			description: "Should print error message without panicking",
			testFunc: func() {
				printError("Test error occurred")
				printError("Test error with %s", "context")
			},
		},
		{
			name:        "printWarning",
			description: "Should print warning message without panicking",
			testFunc: func() {
				printWarning("Test warning message")
				printWarning("Test warning with %s", "details")
			},
		},
		{
			name:        "printInfo",
			description: "Should print info message without panicking",
			testFunc: func() {
				printInfo("Test info message")
				printInfo("Test info with %s", "data")
			},
		},
		{
			name:        "printStatus",
			description: "Should print status message without panicking",
			testFunc: func() {
				printStatus("✅", "TEST", "Test status message")
				printStatus("INFO", "STATUS", "Another status message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Function panicked: %v", r)
				}
			}()
			tt.testFunc()
		})
	}
}

func TestGetMachineNameHelper(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "nil_input",
			description: "Should handle nil input gracefully",
			testFunc: func(t *testing.T) {
				result := getMachineName(nil)
				if result != "" {
					t.Errorf("Expected empty string for nil input, got %q", result)
				}
			},
		},
		{
			name:        "non_config_input",
			description: "Should handle non-config struct input",
			testFunc: func(t *testing.T) {
				result := getMachineName("not a config")
				if result != "" {
					t.Errorf("Expected empty string for non-config input, got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}
