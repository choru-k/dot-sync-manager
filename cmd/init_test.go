package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestGetMachineName(t *testing.T) {
	name := getMachineName()

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
			r.Close()
		})

		go func() {
			defer w.Close()
			w.Write([]byte("valid-input\n"))
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
			r.Close()
		})

		go func() {
			defer w.Close()
			w.Write([]byte("  valid-input  \n"))
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
				r.Close()
			})

			// Write test input
			go func() {
				defer w.Close()
				w.Write([]byte(tt.input))
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
	r.Close()
	w.Close()

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
