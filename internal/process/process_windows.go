//go:build windows

package process

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// getProcessInfo queries tasklist for process information by PID.
// Uses CSV format for robust parsing and includes bounds checking to prevent panics.
// Returns the image name and whether the process exists.
func getProcessInfo(pid int) (imageName string, exists bool) {
	if pid <= 0 {
		return "", false
	}

	output, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV").Output()
	if err != nil {
		return "", false
	}

	// Use csv.Reader for robust parsing
	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 || len(records[1]) < 1 {
		return "", false
	}

	// First column is Image Name
	return records[1][0], true
}

func processExists(pid int) bool {
	_, exists := getProcessInfo(pid)
	return exists
}

// verifyProcessName checks if the given PID belongs to a process with the expected name.
// Uses exact matching (case-insensitive) to avoid false positives from partial name matches.
// For example, it won't match "chrome-sync-service.exe" when looking for "sync".
// Returns false if the PID doesn't exist or if the process name doesn't match.
func verifyProcessName(pid int, expectedName string) bool {
	imageName, exists := getProcessInfo(pid)
	if !exists {
		return false
	}

	// Remove .exe extension from imageName if present for comparison
	cleanImageName := strings.TrimSuffix(imageName, ".exe")

	// Use exact matching (case-insensitive) to avoid false positives
	return strings.EqualFold(cleanImageName, expectedName)
}

func terminateProcess(proc *os.Process) error {
	return proc.Kill()
}

func stopAllDaemons(name string) error {
	cmd := exec.Command("taskkill", "/F", "/IM", name+".exe")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("taskkill", "/F", "/IM", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("process: stop daemons: %w", err)
		}
	}
	return nil
}

// findProcessByName searches for a process by image name using tasklist command.
// Tries both with and without .exe extension. This may be slow as it lists all processes.
// Returns the first matching PID or an error if not found.
func findProcessByName(name string) (int, error) {
	// Use /NH (No Header) to simplify parsing
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to name without .exe
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq "+name, "/FO", "CSV", "/NH")
		output, err = cmd.Output()
		if err != nil {
			// This error usually means the process was not found.
			return 0, fmt.Errorf("process: not found: %s", name)
		}
	}

	// Use csv.Reader for robust parsing
	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 {
		return 0, fmt.Errorf("process: not found: %s", name)
	}

	// We expect at least one record, and it should have at least 2 columns (Image Name, PID)
	record := records[0]
	if len(record) < 2 {
		return 0, fmt.Errorf("process: unexpected tasklist output format")
	}

	pid, convErr := strconv.Atoi(record[1])
	if convErr != nil {
		return 0, fmt.Errorf("process: could not parse PID '%s': %w", record[1], convErr)
	}

	return pid, nil
}
