//go:build !windows

package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// processExists checks if a process with the given PID exists using syscall.Kill(pid, 0).
// This is a lightweight check that doesn't actually send a signal.
// Returns false for invalid PIDs (<= 0) or non-existent processes.
// Returns true if the process exists, even if we don't have permission to signal it (EPERM).
func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists but we don't have permission to signal it
	// This should be treated as the process being alive
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	// ESRCH means the process doesn't exist
	return false
}

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
			if normalizeProcessName(parts[0]) == normalizeProcessName(expectedName) {
				return true
			}
		}
	}

	// On Linux, also try /proc/<pid>/exe (faster and more reliable when available)
	if runtime.GOOS == "linux" {
		exePath := fmt.Sprintf("/proc/%d/exe", pid)
		if link, err := os.Readlink(exePath); err == nil {
			if normalizeProcessName(link) == normalizeProcessName(expectedName) {
				return true
			}
		}
	}

	return false
}

func terminateProcess(proc *os.Process) error {
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("process: terminate: %w", err)
	}
	return nil
}

func stopAllDaemons(name string) error {
	normalized := normalizeProcessName(name)
	if !isValidProcessName(normalized) {
		return fmt.Errorf("process: invalid process name: %q", name)
	}

	if err := exec.Command("pkill", "-x", normalized).Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// pkill returns 1 when no processes matched; treat as success.
			return nil
		}
		if err := exec.Command("killall", normalized).Run(); err != nil {
			var exitErr *exec.ExitError
			// If killall returns an exit error, it's likely because no process was found.
			// We treat this as a success, similar to the pkill handling.
			if errors.As(err, &exitErr) {
				return nil
			}
			return fmt.Errorf("process: stop daemons with killall: %w", err)
		}
	}
	return nil
}

// findProcessByName searches for a process by name using multiple methods.
// Tries pgrep first (fastest), falls back to parsing ps output.
// This may be slow as it involves process enumeration when pgrep fails.
// Returns the first matching PID or an error if not found.
func findProcessByName(name string) (int, error) {
	normalized := normalizeProcessName(name)
	if normalized == "" {
		return 0, fmt.Errorf("process: not found: %s", name)
	}

	if output, err := exec.Command("pgrep", "-x", normalized).Output(); err == nil {
		pids := strings.Fields(string(output))
		for _, pidStr := range pids {
			pid, convErr := strconv.Atoi(strings.TrimSpace(pidStr))
			if convErr == nil && pid != os.Getpid() {
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
		if pid == os.Getpid() {
			continue
		}

		parts := strings.Fields(command)
		if len(parts) == 0 {
			continue
		}

		if normalizeProcessName(parts[0]) == normalized {
			return pid, nil
		}
	}

	return 0, fmt.Errorf("process: not found: %s", name)
}

// isValidProcessName validates that a process name contains only safe characters.
// These restrictions prevent command injection and ensure the process name can be safely
// used in shell commands and file operations. The allowed characters are:
// - Letters (a-z, A-Z)
// - Numbers (0-9) 
// - Hyphens (-), underscores (_), and periods (.)
// This matches typical process naming conventions and avoids special shell characters.
func isValidProcessName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}
