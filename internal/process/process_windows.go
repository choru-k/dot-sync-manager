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

// getProcessInfo queries tasklist for process information by PID
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

// verifyProcessName checks if the given PID belongs to a process with the expected name
func verifyProcessName(pid int, expectedName string) bool {
	imageName, exists := getProcessInfo(pid)
	if !exists {
		return false
	}

	return strings.Contains(imageName, expectedName) ||
		strings.Contains(imageName, expectedName+".exe")
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
