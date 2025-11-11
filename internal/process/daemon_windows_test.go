//go:build windows

package process

import (
	"os"
	"testing"
)

func TestProcessDetectionWindows(t *testing.T) {
	// Test processExists for non-existent PID
	nonExistentPID := findNonExistentPID()
	if processExists(nonExistentPID) {
		t.Errorf("processExists returned true for non-existent PID %d", nonExistentPID)
	}

	// Test processExists for current PID
	currentPID := os.Getpid()
	if !processExists(currentPID) {
		t.Errorf("processExists returned false for current PID %d", currentPID)
	}

	// Test invalid PID handling
	if processExists(0) {
		t.Error("processExists returned true for PID 0")
	}

	if processExists(-1) {
		t.Error("processExists returned true for negative PID")
	}
}

func findNonExistentPID() int {
	// Start from a high PID and work down to find one that doesn't exist
	for pid := 99999; pid > 50000; pid-- {
		if _, exists := getProcessInfo(pid); !exists {
			return pid
		}
	}
	// Fallback (unlikely to reach here)
	return 99999
}

func TestGetProcessInfoWindows(t *testing.T) {
	// Test getProcessInfo for current process
	currentPID := os.Getpid()
	imageName, exists := getProcessInfo(currentPID)
	if !exists {
		t.Errorf("getProcessInfo failed to find current process (PID %d)", currentPID)
	}
	if imageName == "" {
		t.Error("getProcessInfo returned empty image name for current process")
	}

	// Test getProcessInfo for non-existent PID
	nonExistentPID := findNonExistentPID()
	_, exists = getProcessInfo(nonExistentPID)
	if exists {
		t.Errorf("getProcessInfo returned true for non-existent PID %d", nonExistentPID)
	}

	// Test invalid PIDs
	_, exists = getProcessInfo(0)
	if exists {
		t.Error("getProcessInfo returned true for PID 0")
	}

	_, exists = getProcessInfo(-1)
	if exists {
		t.Error("getProcessInfo returned true for negative PID")
	}
}

func TestVerifyProcessNameWindows(t *testing.T) {
	// We can't reliably test this without knowing what the current process is named
	// But we can test the invalid cases
	if verifyProcessName(99999, "nonexistent") {
		t.Error("verifyProcessName returned true for non-existent PID")
	}

	if verifyProcessName(0, "test") {
		t.Error("verifyProcessName returned true for PID 0")
	}

	if verifyProcessName(-1, "test") {
		t.Error("verifyProcessName returned true for negative PID")
	}
}

func TestFindProcessByNameWindows(t *testing.T) {
	// Test finding a process that definitely exists on Windows
	// We'll try to find "svchost" or "explorer" which are system processes
	testProcesses := []string{"svchost", "explorer"}

	foundAtLeastOne := false
	for _, procName := range testProcesses {
		pid, err := findProcessByName(procName)
		if err == nil && pid > 0 {
			foundAtLeastOne = true

			// Verify the process exists using processExists
			if !processExists(pid) {
				t.Errorf("findProcessByName returned PID %d for %s, but processExists returned false", pid, procName)
			}
			break
		}
	}

	if !foundAtLeastOne {
		t.Logf("Warning: Could not find any test system processes. This test may be unreliable.")
	}

	// Test finding a process that definitely doesn't exist
	_, err := findProcessByName("nonexistent-process-name-12345")
	if err == nil {
		t.Error("findProcessByName should return error for non-existent process")
	}
}
