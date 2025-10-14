package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const pidFileName = ".dotfile-sync-manager.pid"

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
func IsDaemonRunning() bool {
	pid, err := readPID()
	if err == nil {
		if pid != os.Getpid() && processExists(pid) {
			return true
		}
		_ = RemovePID()
	}

	if pid, err = findProcessByName(defaultProcessName()); err == nil {
		if pid == os.Getpid() {
			return false
		}
		if processExists(pid) {
			return true
		}
	}
	return false
}

// GetDaemonPID retrieves the PID from the pidfile or falls back to process discovery.
func GetDaemonPID() (int, error) {
	if pid, err := readPID(); err == nil {
		if processExists(pid) {
			return pid, nil
		}
		_ = RemovePID()
	}

	pid, err := findProcessByName(defaultProcessName())
	if err != nil {
		return 0, err
	}
	if processExists(pid) {
		return pid, nil
	}
	return 0, fmt.Errorf("process: pid %d not running", pid)
}

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
