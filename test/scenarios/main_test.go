package scenarios

import (
	"fmt"
	"os"
	"testing"
)

// TestMain runs setup and teardown for all scenario tests
func TestMain(m *testing.M) {
	// Initialize path resolution early
	initPathResolution()

	// Ensure all required config templates and fixtures exist
	if err := ensureTestConfigTemplates(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ensure test config templates: %v\n", err)
		os.Exit(1)
	}

	// Run the tests
	code := m.Run()

	// Cleanup any remaining test artifacts
	testID := os.Getenv("TEST_ID")
	if testID != "" {
		verification := VerifyCleanup(testID)
		if !verification.Success {
			fmt.Fprintf(os.Stderr, "Cleanup verification failed for TEST_ID=%s: %v\n", testID, verification.Issues)
			if err := ForceCleanupIfNeeded(testID); err != nil {
				fmt.Fprintf(os.Stderr, "Force cleanup failed: %v\n", err)
			}
		}
	}

	// Exit with the test result code
	os.Exit(code)
}
