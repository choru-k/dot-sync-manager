package process

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/choru-k/dot-sync-manager/internal/util"
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

// WritePIDExclusive atomically creates the PID file with O_EXCL flag.
// This prevents TOCTOU race conditions between checking if daemon is running
// and writing the PID file. Returns an error if PID file already exists.
func WritePIDExclusive(pid int) (err error) {
	if pid <= 0 {
		return fmt.Errorf("process: write pid exclusive: invalid pid %d", pid)
	}
	
	// Get the current executable name for reliable daemon detection
	exeName := defaultProcessName()
	
	path, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("process: write pid exclusive: failed to get path: %w", err)
	}
	
	// Store both PID and executable name in format: "PID:exe_name"
	content := fmt.Sprintf("%d:%s", pid, exeName)
	
	// Create file atomically with O_EXCL flag
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	var file *os.File
	file, err = os.OpenFile(path, flags, pidFilePerms)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("process: daemon already running (PID file exists)")
		}
		return fmt.Errorf("process: write pid exclusive: %w", err)
	}
	defer util.CloseAndCaptureErr(file, &err)
	
	if _, err := file.WriteString(content); err != nil {
		// Clean up on write failure
		if removeErr := os.Remove(path); removeErr != nil {
			log.Printf("process: warning - failed to remove PID file after write error: %v", removeErr)
		}
		return fmt.Errorf("process: write pid exclusive: %w", err)
	}
	
	return nil
}

// WritePID persists the daemon PID and executable name for later lookup.
// This ensures daemon detection works reliably even if the binary is renamed.
func WritePID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("process: write pid: invalid pid %d", pid)
	}
	
	// Get the current executable name for reliable daemon detection
	exeName := defaultProcessName()
	
	path, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("process: write pid: failed to get path: %w", err)
	}
	
	// Store both PID and executable name in format: "PID:exe_name"
	content := fmt.Sprintf("%d:%s", pid, exeName)
	return os.WriteFile(path, []byte(content), pidFilePerms)
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

// pidInfo stores both the PID and executable name from the PID file.
type pidInfo struct {
	pid      int
	exeName string
}

// readPID reads and parses the PID and executable name from the PID file.
// Returns an error if the file doesn't exist, can't be read, or contains invalid data.
func readPID() (*pidInfo, error) {
	path, err := pidFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("process: read pid file: %w", err)
	}
	content := strings.TrimSpace(string(data))
	
	// Parse content into parts (legacy: "PID", new: "PID:exe_name")
	parts := strings.SplitN(content, ":", 2)
	pidStr := parts[0]
	
	// Validate PID first (common to both formats)
	pid, convErr := strconv.Atoi(pidStr)
	if convErr != nil {
		return nil, fmt.Errorf("process: invalid pid value %q: %w", pidStr, convErr)
	}
	if pid <= 0 {
		return nil, fmt.Errorf("process: invalid pid value %q: must be positive", pidStr)
	}
	
	// Handle format-specific logic
	if len(parts) == 1 {
		// Legacy format: just the PID, no executable name
		return &pidInfo{pid: pid, exeName: ""}, nil
	}
	
	// New format: PID:exe_name
	exeName := parts[1]
	if exeName == "" {
		return nil, fmt.Errorf("process: missing executable name in pid file")
	}
	
	return &pidInfo{pid: pid, exeName: exeName}, nil
}

// IsDaemonRunning checks if the daemon associated with the stored PID is running.
//
// This function implements a two-phase approach to daemon detection:
// 1. Check PID file for stored daemon PID
// 2. Fall back to process discovery by name if PID file is missing/invalid
//
// Note: This function contains benign race conditions where a process could
// terminate between checking existence and verifying name. The worst case is
// a false negative (reporting daemon not running when it just started), which
// is acceptable for daemon management as subsequent operations will fail safely.
// When falling back to process enumeration the lookup may be slow on systems
// with many processes; callers should treat this as an infrequent operation.
func IsDaemonRunning() bool {
	var expectedName string
	
	if pidInfo, err := readPID(); err == nil {
		// Use stored executable name from PID file for reliable detection
		expectedName = pidInfo.exeName
		if expectedName == "" {
			// Legacy PID file format - use current executable name
			expectedName = defaultProcessName()
		}

		switch {
		case pidInfo.pid == os.Getpid():
			log.Printf("process: removing self-PID file - daemon process is current process")
			if err := RemovePID(); err != nil {
				log.Printf("process: warning - failed to remove self PID file: %v", err)
			}
		case processExists(pidInfo.pid) && verifyProcessName(pidInfo.pid, expectedName):
			return true
		default:
			if err := RemovePID(); err != nil {
				log.Printf("process: warning - failed to remove stale PID file: %v", err)
			}
		}
	}

	// Fallback to process discovery using current executable name if no PID file
	if expectedName == "" {
		expectedName = defaultProcessName()
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
	if pidInfo, err := readPID(); err == nil {
		// Use stored executable name from PID file for reliable detection
		expectedName := pidInfo.exeName
		if expectedName == "" {
			// Legacy PID file format - use current executable name
			expectedName = defaultProcessName()
		}

		if pidInfo.pid == os.Getpid() {
			log.Printf("process: removing self-PID file - found current process in PID file")
			if err := RemovePID(); err != nil {
				log.Printf("process: warning - failed to remove self PID file: %v", err)
			}
		} else if processExists(pidInfo.pid) && verifyProcessName(pidInfo.pid, expectedName) {
			return pidInfo.pid, nil
		} else if err := RemovePID(); err != nil {
			log.Printf("process: warning - failed to remove PID file during cleanup: %v", err)
		}
	}

	// Fallback to process discovery using current executable name
	expectedName := defaultProcessName()

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

// FindProcessByName searches for a running process by name across all processes.
// Returns the PID of the first matching process (excluding the current process).
// This operation may be slow as it involves process enumeration.
// Returns an error if no matching process is found or if the name is invalid.
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
	// On Linux, when the executable file is deleted while the process is still running,
	// the kernel adds " (deleted)" to the process name in /proc/<pid>/exe and ps output.
	// We strip this suffix to ensure reliable process name matching.
	base = strings.TrimSuffix(base, " (deleted)")
	if ext := filepath.Ext(base); ext != "" && strings.EqualFold(ext, exeExtension) {
		base = base[:len(base)-len(ext)]
	}
	return base
}
