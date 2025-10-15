package process

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const pidFileName = ".dotfile-sync-manager.pid"

// pidFilePath returns the absolute path to the PID file in the user's home directory.
// The PID file is used to track the running daemon process across sessions.
// Uses 0600 permissions to ensure only the owner can read/write the file.
func pidFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("process: determine pid file directory: %w", err)
	}
	return filepath.Join(home, pidFileName), nil
}

// WritePID persists the daemon PID for later lookup.
func WritePID(pid int) error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600)
}

// RemovePID deletes the stored PID file if it exists.
func RemovePID() error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("process: remove pid file: %w", err)
	}
	return nil
}

// readPID reads and parses the PID from the PID file.
// Returns an error if the file doesn't exist, can't be read, or contains invalid data.
func readPID() (int, error) {
	path, err := pidFilePath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("process: read pid file: %w", err)
	}
	pidStr := strings.TrimSpace(string(data))
	pid, convErr := strconv.Atoi(pidStr)
	if convErr != nil {
		return 0, fmt.Errorf("process: invalid pid value %q: %w", pidStr, convErr)
	}
	return pid, nil
}

// IsDaemonRunning checks if the daemon associated with the stored PID is running.
//
// This function implements a two-phase approach to daemon detection:
// 1. Check PID file for stored daemon PID
// 2. Fall back to process discovery by name if PID file is missing/invalid
//
// Note: This function contains benign race conditions where a process could
// terminate between checking existence and verifying name. These are acceptable
// for daemon management purposes and don't affect correctness.
func IsDaemonRunning() bool {
	expectedName := defaultProcessName()
	pid, err := readPID()
	if err == nil {
		if pid != os.Getpid() && processExists(pid) {
			// Verify this PID actually belongs to our process
			if verifyProcessName(pid, expectedName) {
				return true
			}
			// Stale PID file - clean it up
			if err := RemovePID(); err != nil {
				log.Printf("process: warning - failed to remove stale PID file: %v", err)
			}
		} else {
			if err := RemovePID(); err != nil {
				log.Printf("process: warning - failed to remove invalid PID file: %v", err)
			}
		}
	}

	if pid, err = findProcessByName(expectedName); err == nil {
		if pid == os.Getpid() {
			return false
		}
		if processExists(pid) && verifyProcessName(pid, expectedName) {
			return true
		}
	}
	return false
}

// GetDaemonPID retrieves the PID from the pidfile or falls back to process discovery.
// Returns the PID of a running daemon process that matches the expected binary name.
// This function may be slow as it involves process enumeration on failure.
func GetDaemonPID() (int, error) {
	expectedName := defaultProcessName()

	if pid, err := readPID(); err == nil {
		if processExists(pid) && verifyProcessName(pid, expectedName) {
			return pid, nil
		}
		if err := RemovePID(); err != nil {
			log.Printf("process: warning - failed to remove PID file during cleanup: %v", err)
		}
	}

	pid, err := findProcessByName(expectedName)
	if err != nil {
		return 0, err
	}
	if processExists(pid) {
		return pid, nil
	}
	return 0, fmt.Errorf("process: pid %d not running", pid)
}

// defaultProcessName returns the expected binary name for daemon detection.
// Uses os.Executable() to get the actual running binary name, falling back
// to "dot-sync-manager" if that fails. This ensures we can find the daemon
// even if the binary was renamed or executed from a different path.
func defaultProcessName() string {
	// The actual binary name is dot-sync-manager (or dsm as the command)
	// Use the executable name to ensure we match the actual running process
	if exe, err := os.Executable(); err == nil {
		return filepath.Base(exe)
	}
	// Fallback to the actual binary name if os.Executable fails
	return "dot-sync-manager"
}

// FindProcessByName exposes name lookup for callers that need it.
func FindProcessByName(name string) (int, error) {
	return findProcessByName(name)
}

// TerminateProcess delegates to the platform-specific implementation.
func TerminateProcess(proc *os.Process) error {
	return terminateProcess(proc)
}

// StopAllDaemons terminates all daemon processes and clears the pidfile.
func StopAllDaemons() error {
	if err := stopAllDaemons(defaultProcessName()); err != nil {
		return err
	}
	return RemovePID()
}
