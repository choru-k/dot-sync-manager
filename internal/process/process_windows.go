//go:build windows

package process

import (
	"encoding/csv"
	"errors"
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

	return strings.EqualFold(normalizeProcessName(imageName), normalizeProcessName(expectedName))
}

func terminateProcess(proc *os.Process) error {
	return proc.Kill()
}

func stopAllDaemons(name string) error {
	normalized := normalizeProcessName(name)
	if normalized == "" {
		return fmt.Errorf("process: invalid process name: %q", name)
	}

	cmd := exec.Command("taskkill", "/F", "/IM", normalized+".exe")
	if err := cmd.Run(); err != nil {
		if isTaskkillNotFound(err) {
			cmd = exec.Command("taskkill", "/F", "/IM", normalized)
			if err := cmd.Run(); err != nil {
				if isTaskkillNotFound(err) {
					return nil
				}
				return fmt.Errorf("process: stop daemons: %w", err)
			}
			return nil
		}
		cmd = exec.Command("taskkill", "/F", "/IM", normalized)
		if retryErr := cmd.Run(); retryErr != nil {
			if isTaskkillNotFound(retryErr) {
				return nil
			}
			return fmt.Errorf("process: stop daemons: %w", retryErr)
		}
		return nil
	}

	return nil
}

func isTaskkillNotFound(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 128 {
			return true
		}
	}
	return false
}

// findProcessByName searches for a process by image name using tasklist command.
// Tries both with and without .exe extension. This may be slow as it lists all processes.
// Returns the first matching PID or an error if not found.
func findProcessByName(name string) (int, error) {
	normalized := normalizeProcessName(name)
	if normalized == "" {
		return 0, fmt.Errorf("process: not found: %s", name)
	}

	currentPID := os.Getpid()

	// Use /NH (No Header) to simplify parsing
	commands := []*exec.Cmd{
		exec.Command("tasklist", "/FI", "IMAGENAME eq "+normalized+".exe", "/FO", "CSV", "/NH"),
		exec.Command("tasklist", "/FI", "IMAGENAME eq "+normalized, "/FO", "CSV", "/NH"),
	}

	for _, cmd := range commands {
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		reader := csv.NewReader(strings.NewReader(string(output)))
		records, err := reader.ReadAll()
		if err != nil {
			continue
		}

		for _, record := range records {
			if len(record) < 2 {
				continue
			}

			if !strings.EqualFold(normalizeProcessName(record[0]), normalized) {
				continue
			}

			pidStr := strings.TrimSpace(record[1])
			if pidStr == "" {
				continue
			}

			pid, convErr := strconv.Atoi(pidStr)
			if convErr != nil || pid == currentPID {
				continue
			}
			return pid, nil
		}
	}

	return 0, fmt.Errorf("process: not found: %s", name)
}

// findProcessByName searches for a process by image name using tasklist command.
// Tries both with and without .exe extension. This may be slow as it lists all processes.
// Returns the first matching PID or an error if not found.
func findProcessByName(name string) (int, error) {
	normalized := normalizeProcessName(name)
	if normalized == "" {
		return 0, fmt.Errorf("process: not found: %s", name)
	}

	currentPID := os.Getpid()

	// Use /NH (No Header) to simplify parsing
	commands := []*exec.Cmd{
		exec.Command("tasklist", "/FI", "IMAGENAME eq "+normalized+".exe", "/FO", "CSV", "/NH"),
		exec.Command("tasklist", "/FI", "IMAGENAME eq "+normalized, "/FO", "CSV", "/NH"),
	}

	for _, cmd := range commands {
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		reader := csv.NewReader(strings.NewReader(string(output)))
		records, err := reader.ReadAll()
		if err != nil {
			continue
		}

		for _, record := range records {
			if len(record) < 2 {
				continue
			}

			if !strings.EqualFold(normalizeProcessName(record[0]), normalized) {
				continue
			}

			pidStr := strings.TrimSpace(record[1])
			if pidStr == "" {
				continue
			}

			pid, convErr := strconv.Atoi(pidStr)
			if convErr != nil || pid == currentPID {
				continue
			}
			return pid, nil
		}
	}

	return 0, fmt.Errorf("process: not found: %s", name)
}
