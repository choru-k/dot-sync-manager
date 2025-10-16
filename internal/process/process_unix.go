//go:build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// processExists checks if a process with the given PID exists using syscall.Kill(pid, 0).
// This is a lightweight check that doesn't actually send a signal.
// Returns false for invalid PIDs (<= 0) or non-existent processes.
func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// verifyProcessName checks if the given PID belongs to a process with the expected name.
// Uses multiple methods for reliability:
// 1. ps command with full command line args
// 2. Linux /proc/<pid>/exe symlink (faster when available)
// Returns true if any method finds a match containing the expected name.
// verifyProcessName checks if the process with the given PID matches the expected name.
// Uses both ps command output and /proc/<pid>/exe (on Linux) to verify the process identity.
// Returns false if the PID doesn't exist or if the process name doesn't match.
func verifyProcessName(pid int, expectedName string) bool {
	if pid <= 0 {
		return false
	}

	// Try ps command first (most portable across Unix systems)
	// Use "args=" to get full command line instead of truncated comm
	output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if err == nil {
		args := strings.TrimSpace(string(output))
		// Extract the first word (command) from args for exact matching
		parts := strings.Fields(args)
		if len(parts) > 0 {
			cmdName := filepath.Base(parts[0])
			cmdName = strings.TrimSuffix(cmdName, " (deleted)")
			if strings.EqualFold(cmdName, expectedName) {
				return true
			}
		}
	}

	// On Linux, also try /proc/<pid>/exe (faster and more reliable when available)
	if runtime.GOOS == "linux" {
		exePath := fmt.Sprintf("/proc/%d/exe", pid)
		if link, err := os.Readlink(exePath); err == nil {
			baseName := filepath.Base(link)
			// Strip " (deleted)" suffix that appears when binary is replaced
			baseName = strings.TrimSuffix(baseName, " (deleted)")
			if strings.EqualFold(baseName, expectedName) {
				return true
			}
		}
	}

	return false
}

func terminateProcess(proc *os.Process) error {
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	return nil
}

func stopAllDaemons(name string) error {
	if err := exec.Command("pkill", "-f", name).Run(); err != nil {
		if err := exec.Command("killall", name).Run(); err != nil {
			return fmt.Errorf("process: stop daemons: %w", err)
		}
	}
	return nil
}

// findProcessByName searches for a process by name using multiple methods.
// Tries pgrep first (fastest), falls back to parsing ps output.
// This may be slow as it involves process enumeration when pgrep fails.
// Returns the first matching PID or an error if not found.
func findProcessByName(name string) (int, error) {
	if output, err := exec.Command("pgrep", "-f", name).Output(); err == nil {
		pids := strings.Fields(string(output))
		for _, pidStr := range pids {
			pid, convErr := strconv.Atoi(strings.TrimSpace(pidStr))
			if convErr == nil {
				return pid, nil
			}
		}
	}

	// Fallback: parse ps output more carefully to handle command names with spaces
	// Using ps with specific format to avoid parsing issues
	psOutput, err := exec.Command("ps", "-eo", "pid=,command=").Output()
	if err != nil {
		return 0, fmt.Errorf("process: list processes: %w", err)
	}
	lines := strings.Split(string(psOutput), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Find first space to separate PID from command
		spaceIdx := strings.Index(line, " ")
		if spaceIdx == -1 {
			continue
		}
		pidStr := line[:spaceIdx]
		command := line[spaceIdx+1:]

		pid, convErr := strconv.Atoi(strings.TrimSpace(pidStr))
		if convErr != nil {
			continue
		}
		// Check if the command contains our expected name
		if strings.Contains(command, name) {
			return pid, nil
		}
	}

	return 0, fmt.Errorf("process: not found: %s", name)
}
