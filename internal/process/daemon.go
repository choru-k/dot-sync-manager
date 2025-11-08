package process

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const (
	pidFileName        = ".dotfile-sync-manager.pid"
	pidFilePerms       = 0o600 // owner read/write only to protect daemon PID information
	exeExtension       = ".exe" // Windows executable extension
	lockTimeout        = 5 * time.Second
	lockRetryInterval  = 100 * time.Millisecond
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

// LockManager manages the lifecycle of a PID file lock for daemon exclusivity.
type LockManager struct {
	lock    *flock.Flock
	pidPath string
	lockPath string
}

// WritePIDExclusive atomically creates the PID file with exclusive file locking.
// This prevents TOCTOU race conditions between checking if daemon is running
// and writing the PID file. Uses flock for cross-platform file locking.
// Returns a LockManager that must be held for the daemon's entire lifetime.
func WritePIDExclusive(pid int) (*LockManager, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("process: write pid exclusive: invalid pid %d", pid)
	}

	// Get the current executable name for reliable daemon detection
	exeName := DefaultProcessName()

	path, err := pidFilePath()
	if err != nil {
		return nil, fmt.Errorf("process: write pid exclusive: failed to get path: %w", err)
	}

	// Create lock file path - use .lock extension for the lock file
	lockPath := path + ".lock"
	fileLock := flock.New(lockPath)

	// Try to acquire exclusive lock with timeout
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()

	locked, err := fileLock.TryLockContext(ctx, lockRetryInterval)
	if err != nil {
		return nil, fmt.Errorf("process: write pid exclusive: failed to acquire lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("process: daemon already running (cannot acquire lock)")
	}

	// Check for stale PID file and clean up if necessary
	if err := cleanupStalePIDFile(path, exeName); err != nil {
		// Release lock on cleanup failure
		if unlockErr := fileLock.Unlock(); unlockErr != nil {
			log.Printf("process: warning - failed to unlock PID file during cleanup: %v", unlockErr)
		}
		if removeErr := os.Remove(lockPath); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Printf("process: warning - failed to remove lock file during cleanup: %v", removeErr)
		}
		return nil, fmt.Errorf("process: write pid exclusive: failed to cleanup stale PID: %w", err)
	}

	// Store both PID and executable name in format: "PID:exe_name"
	content := fmt.Sprintf("%d:%s", pid, exeName)

	// Write PID file atomically
	if err := os.WriteFile(path, []byte(content), pidFilePerms); err != nil {
		// Release lock on write failure
		if unlockErr := fileLock.Unlock(); unlockErr != nil {
			log.Printf("process: warning - failed to unlock PID file during write failure: %v", unlockErr)
		}
		if removeErr := os.Remove(lockPath); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Printf("process: warning - failed to remove lock file during write failure: %v", removeErr)
		}
		return nil, fmt.Errorf("process: write pid exclusive: %w", err)
	}

	// Create and return LockManager
	lockManager := &LockManager{
		lock:    fileLock,
		pidPath: path,
		lockPath: lockPath,
	}

	return lockManager, nil
}

// Unlock releases the file lock and cleans up the lock file.
// This should be called when the daemon is shutting down.
func (lm *LockManager) Unlock() error {
	var errs []error

	// Release the file lock
	if lm.lock != nil {
		if err := lm.lock.Unlock(); err != nil {
			log.Printf("process: warning - failed to unlock PID file: %v", err)
			errs = append(errs, fmt.Errorf("unlock failed: %w", err))
		}
	}

	// Clean up lock file
	if err := os.Remove(lm.lockPath); err != nil && !os.IsNotExist(err) {
		log.Printf("process: warning - failed to remove lock file: %v", err)
		errs = append(errs, fmt.Errorf("remove lock file failed: %w", err))
	}

	// Remove PID file
	if err := os.Remove(lm.pidPath); err != nil && !os.IsNotExist(err) {
		log.Printf("process: warning - failed to remove PID file: %v", err)
		errs = append(errs, fmt.Errorf("remove PID file failed: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("process: cleanup failed with %d errors: %v", len(errs), errs[0])
	}
	return nil
}

// cleanupStalePIDFile checks if the existing PID file contains a stale PID and
// removes it if the process is no longer running or doesn't match the expected name.
func cleanupStalePIDFile(pidPath, expectedExeName string) error {
	pidInfo, err := readPIDFromPath(pidPath)
	if err != nil {
		// PID file doesn't exist or can't be read - that's fine
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("process: cleanup stale pid: failed to read PID file: %w", err)
	}

	// Check if the stored PID is still running and matches expected name
	if pidInfo.pid == os.Getpid() {
		// PID file contains current process PID - remove it
		if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("process: cleanup stale pid: failed to remove self PID file: %w", err)
		}
		return nil
	}

	// Verify if process is actually running
	if !processExists(pidInfo.pid) {
		// Process is not running - stale PID file
		if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("process: cleanup stale pid: failed to remove stale PID file: %w", err)
		}
		log.Printf("process: removed stale PID file for dead process %d", pidInfo.pid)
		return nil
	}

	// Process is running, verify the executable name
	storedExeName := pidInfo.exeName
	if storedExeName == "" {
		// Legacy PID file format - use current executable name for comparison
		storedExeName = expectedExeName
	}

	if !verifyProcessName(pidInfo.pid, storedExeName) {
		// Process name doesn't match - PID has been recycled
		if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("process: cleanup stale pid: failed to remove recycled PID file: %w", err)
		}
		log.Printf("process: removed PID file with recycled PID %d (name mismatch)", pidInfo.pid)
		return nil
	}

	// Process is running and name matches - daemon is already running
	return fmt.Errorf("process: daemon already running (PID %d)", pidInfo.pid)
}

// readPIDFromPath reads and parses the PID and executable name from a specific PID file path.
// This is a version of readPID that works with any path for testing and cleanup purposes.
func readPIDFromPath(path string) (*pidInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err // Don't wrap error to preserve os.IsNotExist check
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

// WritePID persists the daemon PID and executable name for later lookup.
// This ensures daemon detection works reliably even if the binary is renamed.
func WritePID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("process: write pid: invalid pid %d", pid)
	}
	
	// Get the current executable name for reliable daemon detection
	exeName := DefaultProcessName()
	
	path, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("process: write pid: failed to get path: %w", err)
	}
	
	// Store both PID and executable name in format: "PID:exe_name"
	content := fmt.Sprintf("%d:%s", pid, exeName)
	return os.WriteFile(path, []byte(content), pidFilePerms)
}

// RemovePID deletes the stored PID file and associated lock file if they exist.
func RemovePID() error {
	path, err := pidFilePath()
	if err != nil {
		return fmt.Errorf("process: remove pid: failed to get path: %w", err)
	}

	var errs []error

	// Remove PID file
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("remove pid file: %w", err))
	}

	// Remove lock file if it exists
	lockPath := path + ".lock"
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Errorf("remove lock file: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("process: remove pid failed with %d errors: %v", len(errs), errs[0])
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

	pidInfo, err := readPIDFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("process: read pid file: %w", err)
	}

	return pidInfo, nil
}

// cleanupPIDFile removes the PID file and logs any cleanup errors.
// This is a separate function to ensure cleanup errors are properly handled.
func cleanupPIDFile(reason string) {
	if err := RemovePID(); err != nil {
		log.Printf("process: warning - failed to remove %s: %v", reason, err)
	}
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
			expectedName = DefaultProcessName()
		}

		switch {
		case pidInfo.pid == os.Getpid():
			cleanupPIDFile("self PID file")
		case processExists(pidInfo.pid) && verifyProcessName(pidInfo.pid, expectedName):
			return true
		default:
			cleanupPIDFile("stale PID file")
		}
	}

	// Fallback to process discovery using current executable name if no PID file
	if expectedName == "" {
		expectedName = DefaultProcessName()
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
			expectedName = DefaultProcessName()
		}

		if pidInfo.pid == os.Getpid() {
			cleanupPIDFile("self PID file")
		} else if processExists(pidInfo.pid) && verifyProcessName(pidInfo.pid, expectedName) {
			return pidInfo.pid, nil
		} else {
			cleanupPIDFile("stale PID file during PID discovery")
		}
	}

	// Fallback to process discovery using current executable name
	expectedName := DefaultProcessName()

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

// DefaultProcessName returns the expected binary name for daemon detection.
// Uses os.Executable() to get the actual running binary name, falling back
// to "dot-sync-manager" if that fails. This ensures we can find the daemon
// even if the binary was renamed or executed from a different path.
func DefaultProcessName() string {
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
	if err := stopAllDaemons(DefaultProcessName()); err != nil {
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
