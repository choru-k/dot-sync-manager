package process

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	pidFileName  = ".dotfile-sync-manager.pid"
	pidFilePerms = 0o600 // owner read/write only to protect daemon PID information
	exeExtension = ".exe" // Windows executable extension
)

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
	if pid <= 0 {
		return fmt.Errorf("process: write pid: invalid pid %d", pid)
	}
	path, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("process: write pid: failed to get path: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), pidFilePerms)
}

// RemovePID deletes the stored PID file if it exists.
func RemovePID() error {
	path, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("process: remove pid: failed to get path: %w", err)
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
	if pid <= 0 {
		return 0, fmt.Errorf("process: invalid pid value %q: must be positive", pidStr)
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
// When falling back to process enumeration the lookup may be slow on systems
// with many processes; callers should treat this as an infrequent operation.
func IsDaemonRunning() bool {
	expectedName := defaultProcessName()

	if pid, err := readPID(); err == nil {
		switch {
		case pid == os.Getpid():
			if err := RemovePID(); err != nil {
				log.Printf("process: warning - failed to remove self PID file: %v", err)
			}
		case processExists(pid) && verifyProcessName(pid, expectedName):
			return true
		default:
			if err := RemovePID(); err != nil {
				log.Printf("process: warning - failed to remove stale PID file: %v", err)
			}
		}
	}

	pid, err := findProcessByName(expectedName)
	if err != nil {
		return false
	}
	if pid == os.Getpid() {
		return false
	}
	if !processExists(pid) {
		return false
	}
	if verifyProcessName(pid, expectedName) {
		return true
	}
	log.Printf("process: found process %d but name verification failed for %q", pid, expectedName)
	return false
}

// GetDaemonPID retrieves the PID from the pidfile or falls back to process discovery.
// Returns the PID of a running daemon process that matches the expected binary name.
// This function may be slow as it involves process enumeration on failure.
func GetDaemonPID() (int, error) {
	expectedName := defaultProcessName()

	if pid, err := readPID(); err == nil {
		if pid == os.Getpid() {
			if err := RemovePID(); err != nil {
				log.Printf("process: warning - failed to remove self PID file: %v", err)
			}
		} else if processExists(pid) && verifyProcessName(pid, expectedName) {
			return pid, nil
		} else if err := RemovePID(); err != nil {
			log.Printf("process: warning - failed to remove PID file during cleanup: %v", err)
		}
	}

	pid, err := findProcessByName(expectedName)
	if err != nil {
		return 0, err
	}
	if pid == os.Getpid() {
		return 0, fmt.Errorf("process: found current process instead of daemon")
	}
	if !processExists(pid) {
		return 0, fmt.Errorf("process: pid %d not running", pid)
	}
	if !verifyProcessName(pid, expectedName) {
		return 0, fmt.Errorf("process: pid %d does not match expected daemon", pid)
	}
	return pid, nil
}

// defaultProcessName returns the expected binary name for daemon detection.
// Uses os.Executable() to get the actual running binary name, falling back
// to "dot-sync-manager" if that fails. This ensures we can find the daemon
// even if the binary was renamed or executed from a different path.
func defaultProcessName() string {
	if exe, err := os.Executable(); err == nil {
		if name := normalizeProcessName(exe); name != "" {
			return name
		}
	}
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

// normalizeProcessName trims, removes directory components, strips extensions, and
// normalizes platform-specific suffixes so we can compare process names safely.
func normalizeProcessName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return ""
	}
	base := filepath.Base(clean)
	base = strings.TrimSuffix(base, " (deleted)")
	if ext := filepath.Ext(base); ext != "" && strings.EqualFold(ext, exeExtension) {
		base = base[:len(base)-len(ext)]
	}
	return base
}
