package cmd

import (
	"net/mail"
	"os"
	"strings"
	"testing"
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
