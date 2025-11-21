package cmd

import (
	"bytes"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGetMachineName(t *testing.T) {
	name := getMachineNameFromOS()

	if name == "" {
		t.Error("Expected non-empty machine name")
	}

	// Should return hostname or fallback
	if name != "unknown-machine" {
		// If not the fallback, verify it's a valid hostname
		if len(name) == 0 {
			t.Error("Expected valid hostname")
		}
	}
}

func TestPromptForNonEmpty(t *testing.T) {
	t.Run("accepts non-empty input on first try", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("Failed to create pipe: %v", err)
		}
		os.Stdin = r
		t.Cleanup(func() {
			os.Stdin = oldStdin
			_ = r.Close() // Ignore error in cleanup
		})

		go func() {
			defer func() {
				if err := w.Close(); err != nil {
					t.Errorf("failed to close pipe: %v", err)
				}
			}()
			if _, err := w.Write([]byte("valid-input\n")); err != nil {
				t.Errorf("failed to write to pipe: %v", err)
			}
		}()

		result, err := promptForNonEmpty("Test prompt: ", "test field")
		if err != nil {
			t.Fatalf("promptForNonEmpty() unexpected error: %v", err)
		}
		if result != "valid-input" {
			t.Errorf("Expected 'valid-input', got %q", result)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("Failed to create pipe: %v", err)
		}
		os.Stdin = r
		t.Cleanup(func() {
			os.Stdin = oldStdin
			_ = r.Close() // Ignore error in cleanup
		})

		go func() {
			defer func() {
				if err := w.Close(); err != nil {
					t.Errorf("failed to close pipe: %v", err)
				}
			}()
			if _, err := w.Write([]byte("  valid-input  \n")); err != nil {
				t.Errorf("failed to write to pipe: %v", err)
			}
		}()

		result, err := promptForNonEmpty("Test prompt: ", "test field")
		if err != nil {
			t.Fatalf("promptForNonEmpty() unexpected error: %v", err)
		}
		if result != "valid-input" {
			t.Errorf("Expected 'valid-input', got %q", result)
		}
	})
}

func TestPromptForInput(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue string
		expected     string
	}{
		{
			name:         "uses provided input",
			input:        "test-input\n",
			defaultValue: "",
			expected:     "test-input",
		},
		{
			name:         "uses default when input is empty",
			input:        "\n",
			defaultValue: "default-value",
			expected:     "default-value",
		},
		{
			name:         "trims whitespace",
			input:        "  test-value  \n",
			defaultValue: "",
			expected:     "test-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock stdin
			oldStdin := os.Stdin
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}
			os.Stdin = r
			t.Cleanup(func() {
				os.Stdin = oldStdin
				if err := r.Close(); err != nil {
					t.Logf("failed to close pipe: %v", err)
				}
			})

			// Write test input
			go func() {
				defer func() {
					if err := w.Close(); err != nil {
						t.Errorf("failed to close pipe: %v", err)
					}
				}()
				if _, err := w.Write([]byte(tt.input)); err != nil {
					t.Errorf("failed to write to pipe: %v", err)
				}
			}()

			// Call function
			result, err := promptForInput("Test prompt: ", tt.defaultValue)
			if err != nil {
				t.Fatalf("promptForInput() returned error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPromptForInputError(t *testing.T) {
	// Create a closed pipe to simulate read error
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	_ = r.Close() // Ignore error - intentionally closed for test
	_ = w.Close() // Ignore error - intentionally closed for test

	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
	})

	_, err = promptForInput("Test: ", "")
	if err == nil {
		t.Error("Expected error when stdin is closed, got nil")
	}

	if !strings.Contains(err.Error(), "reading input") {
		t.Errorf("Expected error message to contain 'reading input', got: %v", err)
	}
}

func TestEmailValidation(t *testing.T) {
	t.Run("valid email addresses", func(t *testing.T) {
		validEmails := []string{
			"user@example.com",
			"test.email+tag@domain.co.uk",
			"user_name@sub.domain.com",
			"123@example.com",
			"user@localhost",
		}

		for _, email := range validEmails {
			if _, err := mail.ParseAddress(email); err != nil {
				t.Errorf("Expected valid email '%s' to pass validation, got error: %v", email, err)
			}
		}
	})

	t.Run("invalid email addresses", func(t *testing.T) {
		invalidEmails := []string{
			"",
			"invalid-email",
			"user@",
			"@domain.com",
			"user..name@domain.com",
			"user@.com",
			"user space@domain.com",
			"user@domain..com",
		}

		for _, email := range invalidEmails {
			if _, err := mail.ParseAddress(email); err == nil {
				t.Errorf("Expected invalid email '%s' to fail validation", email)
			}
		}
	})
}

// TestInitCmd_DryRunVariableExists verifies that the dryRun variable exists for the init command
// This is a structural test to ensure the variable is declared before we attempt to use it
func TestInitCmd_DryRunVariableExists(t *testing.T) {
	// This test ensures the dryRun variable is declared in the init command package
	// We verify this by attempting to reference the variable (which will fail compilation if not declared)

	// The variable should be accessible at package level
	// If the dryRun variable doesn't exist, this will cause a compilation error
	_ = dryRun // This will fail to compile if dryRun variable doesn't exist
}

// TestInitCmd_DryRunFlagRegistration verifies that the --dry-run flag is registered on the init command
// This test ensures the flag exists and can be looked up in the command's flag set
func TestInitCmd_DryRunFlagRegistration(t *testing.T) {
	// Look up the --dry-run flag in the init command's flag set
	flag := initCmd.Flags().Lookup("dry-run")

	// The flag should exist
	if flag == nil {
		t.Error("Expected --dry-run flag to be registered on init command, but it was not found")
	}
}

// TestInitCmd_DryRunFlagParsing verifies that the --dry-run flag can be parsed correctly
// This test ensures the flag defaults to false and can be set to true
func TestInitCmd_DryRunFlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "dry-run flag defaults to false when not specified",
			args:     []string{},
			expected: false,
		},
		{
			name:     "dry-run flag can be set to true with --dry-run",
			args:     []string{"--dry-run"},
			expected: true,
		},
		{
			name:     "dry-run flag can be set to false with --dry-run=false",
			args:     []string{"--dry-run=false"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset dryRun to initial state
			dryRun = false

			// Parse flags
			initCmd.SetArgs(tt.args)
			err := initCmd.ParseFlags(tt.args)
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			// Check if dryRun matches expected value
			if dryRun != tt.expected {
				t.Errorf("Expected dryRun to be %v, got %v", tt.expected, dryRun)
			}
		})
	}
}

// TestInitCmd_DryRunShowsCloneLocation verifies that dry-run mode displays clone location with "Would..." prefix
// This test ensures the dry-run shows where files would be cloned without actually doing it
func TestInitCmd_DryRunShowsCloneLocation(t *testing.T) {
	tests := []struct {
		name          string
		gitURL        string
		expectedSubstr string
	}{
		{
			name:          "dry-run shows clone location with Would... prefix for HTTPS URL",
			gitURL:        "https://github.com/user/dotfiles.git",
			expectedSubstr: "Would clone repository from: https://github.com/user/dotfiles.git",
		},
		{
			name:          "dry-run shows clone location with Would... prefix for SSH URL",
			gitURL:        "git@github.com:user/dotfiles.git",
			expectedSubstr: "Would clone repository from: git@github.com:user/dotfiles.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment
			dryRun = true
			repoPath = t.TempDir()

			// Capture stdout using os.Pipe
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}
			os.Stdout = w
			t.Cleanup(func() {
				os.Stdout = oldStdout
				_ = r.Close()
				_ = w.Close()
			})

			// Read output in a goroutine
			outputChan := make(chan string, 1)
			go func() {
				defer close(outputChan)
				var buf bytes.Buffer
				_, _ = buf.ReadFrom(r)
				outputChan <- buf.String()
			}()

			// Call runInit directly with our dry-run setup
			cmd := &cobra.Command{}
			args := []string{tt.gitURL}

			// Set gitURL before calling runInit
			gitURL = tt.gitURL
			err = runInit(cmd, args)

			// Close the writer to signal the reader
			_ = w.Close()

			// In dry-run mode, the function should succeed without errors
			if err != nil {
				t.Fatalf("Expected dry-run to succeed, got error: %v", err)
			}

			// Get captured output
			output := <-outputChan

			// Check that output contains expected substring
			if !strings.Contains(output, tt.expectedSubstr) {
				t.Errorf("Expected output to contain %q, but output was:\n%s", tt.expectedSubstr, output)
			}

			// Verify "Dry run mode" message is present
			if !strings.Contains(output, "Dry run mode") {
				t.Errorf("Expected output to contain 'Dry run mode', but output was:\n%s", output)
			}
		})
	}
}

// TestInitCmd_DryRunShowsConfigPath verifies that dry-run mode displays config file path with "Would..." prefix
// This test ensures the dry-run shows where the configuration file would be created
func TestInitCmd_DryRunShowsConfigPath(t *testing.T) {
	tests := []struct {
		name          string
		repoPath      string
		expectedSubstr string
	}{
		{
			name:          "dry-run shows config file path with Would... prefix for custom path",
			repoPath:      "/custom/dotfiles",
			expectedSubstr: "Would create configuration file: /custom/dotfiles/.sync-config.json",
		},
		{
			name:          "dry-run shows config file path with Would... prefix for default path",
			repoPath:      "~/dotfiles",
			expectedSubstr: "Would create configuration file:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment
			dryRun = true
			repoPath = tt.repoPath

			// Capture stdout using os.Pipe
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}
			os.Stdout = w
			t.Cleanup(func() {
				os.Stdout = oldStdout
				_ = r.Close()
				_ = w.Close()
			})

			// Read output in a goroutine
			outputChan := make(chan string, 1)
			go func() {
				defer close(outputChan)
				var buf bytes.Buffer
				_, _ = buf.ReadFrom(r)
				outputChan <- buf.String()
			}()

			// Call runInit directly with our dry-run setup
			cmd := &cobra.Command{}
			args := []string{""} // Empty gitURL for new repo
			gitURL = "" // Clear gitURL to test new repo scenario
			err = runInit(cmd, args)

			// Close the writer to signal the reader
			_ = w.Close()

			// In dry-run mode, the function should succeed without errors
			if err != nil {
				t.Fatalf("Expected dry-run to succeed, got error: %v", err)
			}

			// Get captured output
			output := <-outputChan

			// Check that output contains expected substring
			if !strings.Contains(output, tt.expectedSubstr) {
				t.Errorf("Expected output to contain %q, but output was:\n%s", tt.expectedSubstr, output)
			}

			// Verify "Dry run mode" message is present
			if !strings.Contains(output, "Dry run mode") {
				t.Errorf("Expected output to contain 'Dry run mode', but output was:\n%s", output)
			}
		})
	}
}

// TestInitCmd_DryRunShowsDefaultValues verifies that dry-run mode displays default configuration values
// This test ensures the dry-run shows what default settings would be applied
func TestInitCmd_DryRunShowsDefaultValues(t *testing.T) {
	tests := []struct {
		name           string
		repoPath       string
		expectedSubstr string
	}{
		{
			name:           "dry-run shows default configuration values with Would... prefix",
			repoPath:       "~/dotfiles",
			expectedSubstr: "Default configuration settings:",
		},
		{
			name:           "dry-run shows pull interval default",
			repoPath:       "~/dotfiles",
			expectedSubstr: "Pull interval: 300 seconds",
		},
		{
			name:           "dry-run shows backup retention default",
			repoPath:       "~/dotfiles",
			expectedSubstr: "Backup retention: 7 days",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment
			dryRun = true
			repoPath = tt.repoPath
			gitURL = "" // Use empty gitURL for new repo scenario

			// Capture stdout using os.Pipe
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("Failed to create pipe: %v", err)
			}
			os.Stdout = w
			t.Cleanup(func() {
				os.Stdout = oldStdout
				_ = r.Close()
				_ = w.Close()
			})

			// Read output in a goroutine
			outputChan := make(chan string, 1)
			go func() {
				defer close(outputChan)
				var buf bytes.Buffer
				_, _ = buf.ReadFrom(r)
				outputChan <- buf.String()
			}()

			// Call runInit directly with our dry-run setup
			cmd := &cobra.Command{}
			args := []string{""}
			err = runInit(cmd, args)

			// Close the writer to signal the reader
			_ = w.Close()

			// In dry-run mode, the function should succeed without errors
			if err != nil {
				t.Fatalf("Expected dry-run to succeed, got error: %v", err)
			}

			// Get captured output
			output := <-outputChan

			// Check that output contains expected substring
			if !strings.Contains(output, tt.expectedSubstr) {
				t.Errorf("Expected output to contain %q, but output was:\n%s", tt.expectedSubstr, output)
			}

			// Verify "Dry run mode" message is present
			if !strings.Contains(output, "Dry run mode") {
				t.Errorf("Expected output to contain 'Dry run mode', but output was:\n%s", output)
			}
		})
	}
}

// TestInitCmd_DryRunPreventsFileOperations verifies that dry-run mode doesn't create any files or directories
// This test ensures the dry-run has no side effects on the filesystem
func TestInitCmd_DryRunPreventsFileOperations(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up test environment with temporary directory
	testRepoPath := filepath.Join(tmpDir, "test-dotfiles")
	dryRun = true
	repoPath = testRepoPath
	gitURL = "" // New repo scenario

	// Capture initial filesystem state
	before, err := os.Stat(testRepoPath)
	if err == nil {
		t.Fatalf("Test directory should not exist initially, but found: %v", before)
	}

	// Capture stdout to verify dry-run output
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = r.Close()
		_ = w.Close()
	})

	// Read output in a goroutine
	outputChan := make(chan string, 1)
	go func() {
		defer close(outputChan)
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		outputChan <- buf.String()
	}()

	// Execute dry-run
	cmd := &cobra.Command{}
	args := []string{""}
	err = runInit(cmd, args)

	// Close the writer to signal the reader
	_ = w.Close()

	// Dry-run should succeed
	if err != nil {
		t.Fatalf("Expected dry-run to succeed, got error: %v", err)
	}

	// Get captured output
	output := <-outputChan

	// Verify dry-run output is present
	if !strings.Contains(output, "Dry run mode") {
		t.Errorf("Expected output to contain 'Dry run mode', but output was:\n%s", output)
	}

	// Verify no filesystem changes occurred
	after, err := os.Stat(testRepoPath)
	if err == nil {
		t.Errorf("Dry-run should not create directories, but found: %v", after)
	}

	// Check that no config file was created
	configPath := filepath.Join(testRepoPath, ".sync-config.json")
	if _, err := os.Stat(configPath); err == nil {
		t.Errorf("Dry-run should not create config file, but found: %s", configPath)
	}

	// Check that no .syncignore file was created
	ignorePath := filepath.Join(testRepoPath, ".syncignore")
	if _, err := os.Stat(ignorePath); err == nil {
		t.Errorf("Dry-run should not create ignore file, but found: %s", ignorePath)
	}

	// Verify the temporary directory itself still exists but is empty
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temporary directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Dry-run should not create any files, but found entries: %v", entries)
	}
}
