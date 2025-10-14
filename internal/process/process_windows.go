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
	if err != nil || len(records) < 2 {
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
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/FO", "CSV")
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq "+name, "/FO", "CSV")
		output, err = cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("process: tasklist failed: %w", err)
		}
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\",\"")
		if len(fields) < 2 {
			continue
		}
		pidStr := strings.Trim(fields[1], "\"")
		pid, convErr := strconv.Atoi(pidStr)
		if convErr == nil {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("process: not found: %s", name)
}
