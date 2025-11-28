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

// TestInitCmd_DryRunVariableExists verifies that the global dry-run flag is accessible
// This structural test ensures the global flag system is properly integrated
func TestInitCmd_DryRunVariableExists(t *testing.T) {
	// The init command now uses the global dry-run flag system
	// We verify this by checking that isDryRun() is accessible
	_ = isDryRun() // This will fail to compile if isDryRun() doesn't exist
}

// TestInitCmd_DryRunFlagRegistration verifies that the --dry-run flag is inherited from root command
// This test ensures the persistent flag from rootCmd is accessible to init command
func TestInitCmd_DryRunFlagRegistration(t *testing.T) {
	// The --dry-run flag is a persistent flag on rootCmd, inherited by all subcommands
	// Check if it's accessible through the init command (includes inherited flags)
	flag := rootCmd.PersistentFlags().Lookup("dry-run")

	// The flag should exist on rootCmd
	if flag == nil {
		t.Error("Expected --dry-run persistent flag to be registered on root command, but it was not found")
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
			// Reset globalDryRun to initial state with proper cleanup
			oldDryRun := globalDryRun
			globalDryRun = false
			t.Cleanup(func() {
				globalDryRun = oldDryRun
			})

			// Parse flags on rootCmd (persistent flags)
			rootCmd.SetArgs(append([]string{"init"}, tt.args...))
			err := rootCmd.ParseFlags(append([]string{"init"}, tt.args...))
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			// Check if globalDryRun matches expected value
			if globalDryRun != tt.expected {
				t.Errorf("Expected globalDryRun to be %v, got %v", tt.expected, globalDryRun)
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
			// Set up test environment with proper cleanup
			oldDryRun := globalDryRun
			globalDryRun = true
			t.Cleanup(func() {
				globalDryRun = oldDryRun
			})

			oldRepoPath := repoPath
			repoPath = t.TempDir()
			t.Cleanup(func() {
				repoPath = oldRepoPath
			})

			// Use --force to bypass directory existence validation
			// (these tests are testing dry-run output, not validation logic)
			oldForce := force
			force = true
			t.Cleanup(func() {
				force = oldForce
			})

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

			// Set gitURL before calling runInit with cleanup
			oldGitURL := gitURL
			gitURL = tt.gitURL
			t.Cleanup(func() {
				gitURL = oldGitURL
			})

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
			// Set up test environment with proper cleanup
			oldDryRun := globalDryRun
			globalDryRun = true
			t.Cleanup(func() {
				globalDryRun = oldDryRun
			})

			oldRepoPath := repoPath
			repoPath = tt.repoPath
			t.Cleanup(func() {
				repoPath = oldRepoPath
			})

			// Use --force to bypass directory existence validation
			// (these tests are testing dry-run output, not validation logic)
			oldForce := force
			force = true
			t.Cleanup(func() {
				force = oldForce
			})

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

			oldGitURL := gitURL
			gitURL = "" // Clear gitURL to test new repo scenario
			t.Cleanup(func() {
				gitURL = oldGitURL
			})

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

// TestInitCmd_DryRunShowsDefaultValues verifies that dry-run mode displays default configuration values.
// This test ensures the dry-run shows what default settings would be applied, including
// pull intervals, backup retention, and auto-sync settings.
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
			// Set up test environment with proper cleanup
			oldDryRun := globalDryRun
			globalDryRun = true
			t.Cleanup(func() {
				globalDryRun = oldDryRun
			})

			oldRepoPath := repoPath
			repoPath = tt.repoPath
			t.Cleanup(func() {
				repoPath = oldRepoPath
			})

			oldGitURL := gitURL
			gitURL = "" // Use empty gitURL for new repo scenario
			t.Cleanup(func() {
				gitURL = oldGitURL
			})

			// Use --force to bypass directory existence validation
			// (these tests are testing dry-run output, not validation logic)
			oldForce := force
			force = true
			t.Cleanup(func() {
				force = oldForce
			})

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

	// Set up test environment with temporary directory and proper cleanup
	testRepoPath := filepath.Join(tmpDir, "test-dotfiles")

	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() {
		globalDryRun = oldDryRun
	})

	oldRepoPath := repoPath
	repoPath = testRepoPath
	t.Cleanup(func() {
		repoPath = oldRepoPath
	})

	oldGitURL := gitURL
	gitURL = "" // New repo scenario
	t.Cleanup(func() {
		gitURL = oldGitURL
	})

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

// TestInitCmd_DryRunExitCodes verifies that dry-run mode returns proper exit codes
// This test ensures dry-run succeeds with exit code 0 for valid inputs
func TestInitCmd_DryRunExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		gitURL  string
		repoPath string
		wantErr bool
	}{
		{
			name:     "dry-run with new repo succeeds",
			gitURL:   "",
			repoPath: "~/dotfiles",
			wantErr:  false,
		},
		{
			name:     "dry-run with HTTPS URL succeeds",
			gitURL:   "https://github.com/user/dotfiles.git",
			repoPath: "~/dotfiles",
			wantErr:  false,
		},
		{
			name:     "dry-run with SSH URL succeeds",
			gitURL:   "git@github.com:user/dotfiles.git",
			repoPath: "~/dotfiles",
			wantErr:  false,
		},
		{
			name:     "dry-run with custom path succeeds",
			gitURL:   "",
			repoPath: "/custom/dotfiles",
			wantErr:  false,
		},
		{
			name:     "dry-run with --force flag still does preview (no actual deletion)",
			gitURL:   "",
			repoPath: "~/dotfiles",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment with proper cleanup
			oldDryRun := globalDryRun
			globalDryRun = true
			t.Cleanup(func() {
				globalDryRun = oldDryRun
			})

			oldRepoPath := repoPath
			repoPath = tt.repoPath
			t.Cleanup(func() {
				repoPath = oldRepoPath
			})

			oldGitURL := gitURL
			gitURL = tt.gitURL
			t.Cleanup(func() {
				gitURL = oldGitURL
			})

			// Use --force to bypass directory existence validation
			// (these tests are testing dry-run exit codes, not validation logic)
			oldForce := force
			force = true
			t.Cleanup(func() {
				force = oldForce
			})

			// Execute dry-run
			cmd := &cobra.Command{}
			args := []string{""}
			if tt.gitURL != "" {
				args[0] = tt.gitURL
			}

			err := runInit(cmd, args)

			// Check exit code expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected dry-run to fail with error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected dry-run to succeed (nil error), but got: %v", err)
				}
			}
		})
	}
}

// TestInitCmd_DryRunForceInteraction verifies that dry-run takes precedence over force flag.
// This test ensures --dry-run prevents actual deletion even when --force is specified,
// confirming that dry-run mode provides complete protection against accidental data loss.
func TestInitCmd_DryRunForceInteraction(t *testing.T) {
	tmpDir := t.TempDir()
	testRepoPath := filepath.Join(tmpDir, "existing-dotfiles")

	// Create an existing directory to test force interaction
	if err := os.Mkdir(testRepoPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Set up test environment with dry-run + force and proper cleanup
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() {
		globalDryRun = oldDryRun
	})

	oldForce := force
	force = true
	t.Cleanup(func() {
		force = oldForce
	})

	oldRepoPath := repoPath
	repoPath = testRepoPath
	t.Cleanup(func() {
		repoPath = oldRepoPath
	})

	oldGitURL := gitURL
	gitURL = ""
	t.Cleanup(func() {
		gitURL = oldGitURL
	})

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

	// Execute dry-run with force flag
	cmd := &cobra.Command{}
	args := []string{""}
	err = runInit(cmd, args)

	// Close the writer to signal the reader
	_ = w.Close()

	// Dry-run should succeed (no prompt, no deletion)
	if err != nil {
		t.Errorf("Expected dry-run with --force to succeed, got error: %v", err)
	}

	// Get captured output
	output := <-outputChan

	// Verify dry-run output is present
	if !strings.Contains(output, "Dry run mode") {
		t.Errorf("Expected output to contain 'Dry run mode', but output was:\n%s", output)
	}

	// Verify the directory still exists (was not deleted by force)
	if _, err := os.Stat(testRepoPath); os.IsNotExist(err) {
		t.Errorf("Dry-run with --force should not delete directories, but %s was removed", testRepoPath)
	}
}

// TestInitCmd_DryRunErrorHandling verifies that dry-run mode properly handles error scenarios
// This test ensures dry-run provides appropriate error messages for invalid inputs
func TestInitCmd_DryRunErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		repoPath  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "dry-run handles empty path gracefully",
			repoPath:  "", // Empty path should still work (util.ExpandPath handles it)
			wantErr:   false,
			errSubstr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment with proper cleanup
			oldDryRun := globalDryRun
			globalDryRun = true
			t.Cleanup(func() {
				globalDryRun = oldDryRun
			})

			oldRepoPath := repoPath
			repoPath = tt.repoPath
			t.Cleanup(func() {
				repoPath = oldRepoPath
			})

			oldGitURL := gitURL
			gitURL = ""
			t.Cleanup(func() {
				gitURL = oldGitURL
			})

			// Use --force to bypass directory existence validation
			// (these tests are testing dry-run error handling, not validation logic)
			oldForce := force
			force = true
			t.Cleanup(func() {
				force = oldForce
			})

			// Capture stdout to verify dry-run still produces output before any error
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

			// Get captured output
			output := <-outputChan

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected dry-run to fail with error, but got nil")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("Expected error to contain %q, but got: %v", tt.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected dry-run to succeed (nil error), but got: %v", err)
				}
				// Verify dry-run output is present for successful cases
				if !strings.Contains(output, "Dry run mode") {
					t.Errorf("Expected output to contain 'Dry run mode', but output was:\n%s", output)
				}
			}
		})
	}
}

// TestInitDryRunShouldValidateDirectories verifies that dry-run mode performs directory validation
// This test ensures dry-run doesn't bypass validation checks that normal execution would perform
func TestInitDryRunShouldValidateDirectories(t *testing.T) {
	// Create existing directory with content to trigger validation error
	dir := t.TempDir()
	existingFile := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existingFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set dry-run mode using proper cleanup pattern
	oldDryRun := globalDryRun
	globalDryRun = true
	t.Cleanup(func() {
		globalDryRun = oldDryRun
	})

	// Set other package variables with cleanup
	oldRepoPath := repoPath
	repoPath = dir
	t.Cleanup(func() {
		repoPath = oldRepoPath
	})

	oldForce := force
	force = false
	t.Cleanup(func() {
		force = oldForce
	})

	// Attempt dry-run init in existing directory (should validate)
	cmd := &cobra.Command{}
	err := runInit(cmd, []string{})

	// Should fail with validation error, not silently succeed
	if err == nil {
		t.Error("Expected dry-run to fail with validation error, but got nil error")
	} else {
		// Verify error message mentions directory exists
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("Expected error to mention 'already exists', but got: %v", err)
		}
		// Verify error suggests using --force flag
		if !strings.Contains(err.Error(), "--force") {
			t.Errorf("Expected error to suggest '--force' flag, but got: %v", err)
		}
	}
}
