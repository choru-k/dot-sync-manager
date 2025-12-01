// Package status provides real-time daemon status reporting through Unix domain sockets.
//
// This package enables rich status monitoring for the Dotfile Sync Manager daemon,
// exposing comprehensive metrics and state information through a secure Unix socket
// interface. The status server runs alongside the sync service and provides
// real-time updates on daemon health, sync operations, and watched paths.
//
// # Key Features
//
//   - Real-time status updates via Unix domain socket communication
//   - Thread-safe status tracking with atomic operations and mutex protection
//   - State-based daemon lifecycle management (starting → running → syncing → idle/error)
//   - Comprehensive metrics: uptime, sync counts, error tracking, file statistics
//   - Secure socket permissions (user-only access, 0600)
//   - Graceful fallback when daemon is not running
//
// # Architecture
//
// The package consists of two main components:
//
//   - StatusManager: Manages daemon status and provides Unix socket server
//   - DaemonStatus: Immutable status snapshot with all daemon metrics
//
// # Usage Example
//
//	// Server side (daemon):
//	statusMgr := status.NewStatusManager("1.0.0", "/etc/dsm/config.json")
//	if err := statusMgr.Start(); err != nil {
//	    log.Fatal(err)
//	}
//	defer statusMgr.Stop(context.Background())
//
//	// Update status throughout daemon lifecycle
//	statusMgr.SetState(status.StateRunning)
//	statusMgr.UpdateSync(changedFiles, nil)
//
//	// Client side (status command):
//	daemonStatus, err := status.GetStatusFromSocket()
//	if err != nil {
//	    // Daemon not running or socket unavailable
//	    return err
//	}
//	fmt.Printf("Daemon PID: %d, Uptime: %s\n", daemonStatus.PID, daemonStatus.Uptime)
//
// # Thread Safety
//
// All StatusManager methods are thread-safe and can be called concurrently.
// Status updates use atomic operations for counters and mutex protection for
// complex state transitions, ensuring consistency under high concurrency.
//
// # Socket Security
//
// The Unix socket is created with 0600 permissions (user-only access),
// preventing unauthorized access to daemon status information. Socket paths
// are expanded to support home directory notation (~/).
package status

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// SocketTimeout defines the timeout for socket connections
	SocketTimeout = 5 * time.Second
	// DefaultSocketPath defines the default path for the Unix socket
	DefaultSocketPath = "~/.dotfile-sync-manager.sock"
)

// DaemonState represents the current state of the daemon
type DaemonState string

const (
	StateStarting DaemonState = "starting"
	StateRunning  DaemonState = "running"
	StateSyncing  DaemonState = "syncing"
	StateIdle     DaemonState = "idle"
	StateStopping DaemonState = "stopping"
	StateError    DaemonState = "error"
)

// DaemonStatus represents the current status of the daemon
type DaemonStatus struct {
	PID            int           `json:"pid"`
	Uptime         time.Duration `json:"uptime"`
	LastSync       time.Time     `json:"last_sync"`
	LastSyncResult string        `json:"last_sync_result"`
	FilesSynced    int           `json:"files_synced"`
	CurrentState   DaemonState   `json:"current_state"`
	Version        string        `json:"version"`
	ConfigPath     string        `json:"config_path"`
	StartTime      time.Time     `json:"start_time"`
	SyncCount      int64         `json:"sync_count"`
	ErrorCount     int64         `json:"error_count"`
	LastError      string        `json:"last_error,omitempty"`
	WatchedPaths   []string      `json:"watched_paths"`
}

// StatusManager manages the daemon status and provides Unix socket server
type StatusManager struct {
	// Status data
	status  DaemonStatus
	mutex   sync.RWMutex
	socket  net.Listener
	ctx     context.Context
	cancel  context.CancelFunc
	running int32 // atomic.Bool

	// Shutdown coordination
	socketMutex  sync.Mutex     // Protects socket field access
	acceptWG     sync.WaitGroup // Tracks acceptConnections goroutine
	shutdownOnce sync.Once      // Ensures single shutdown execution
}

// NewStatusManager creates a new status manager
func NewStatusManager(version, configPath string) *StatusManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &StatusManager{
		status: DaemonStatus{
			Version:      version,
			ConfigPath:   configPath,
			CurrentState: StateStarting,
			StartTime:    time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
		socket: nil, // Will be created when Start() is called
	}
}

// Start starts the Unix socket server
func (sm *StatusManager) Start() error {
	if !atomic.CompareAndSwapInt32(&sm.running, 0, 1) {
		return fmt.Errorf("status manager already running")
	}

	// Expand socket path
	socketPath := expandPath(DefaultSocketPath)

	// Remove existing socket file if it exists
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create socket directory if it doesn't exist
	socketDir := filepath.Dir(socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Create Unix domain socket
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create unix socket: %w", err)
	}

	// Set socket permissions to user-only
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	sm.socket = listener
	sm.setState(StateRunning)

	// Start accepting connections with goroutine lifecycle tracking
	sm.acceptWG.Add(1)
	go func() {
		defer sm.acceptWG.Done()
		sm.acceptConnections()
	}()

	return nil
}

// Stop stops the Unix socket server with race-free ordered shutdown protocol
//
// Design: Four ordered phases with independent timeout contexts
// 1. Signal shutdown (cancel context)
// 2. Wait for acceptConnections goroutine exit (2s timeout)
// 3. Close socket synchronously (guaranteed safe after goroutine exit)
// 4. Remove socket file (best-effort, 500ms timeout)
//
// This fixes three race conditions:
// - Race #1: Socket closure now synchronous after goroutine wait
// - Race #2: Independent timeout contexts immune to parent cancellation
// - Race #3: No concurrent access to sm.socket (goroutine exits before write)
func (sm *StatusManager) Stop(ctx context.Context) error {
	// Idempotent guard
	if !atomic.CompareAndSwapInt32(&sm.running, 1, 0) {
		return nil // Already stopped
	}

	var shutdownErrors []error

	sm.shutdownOnce.Do(func() {
		// Phase 1: Signal shutdown
		sm.setState(StateStopping)
		sm.cancel() // Signals acceptConnections to exit

		// Phase 2: Wait for acceptConnections goroutine with timeout (2s)
		// This ensures no concurrent access to sm.socket before we close it
		done := make(chan struct{})
		go func() {
			sm.acceptWG.Wait()
			close(done)
		}()

		waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer waitCancel()

		select {
		case <-done:
			// Goroutine exited cleanly
		case <-waitCtx.Done():
			shutdownErrors = append(shutdownErrors,
				fmt.Errorf("timeout waiting for acceptConnections goroutine to exit"))
		}

		// Phase 3: Close socket synchronously (guaranteed safe - goroutine exited)
		if sm.socket != nil {
			if err := sm.socket.Close(); err != nil {
				shutdownErrors = append(shutdownErrors,
					fmt.Errorf("failed to close socket: %w", err))
			}
			sm.socket = nil
		}

		// Phase 4: Remove socket file (best-effort with timeout)
		// Socket file is auto-cleaned on next Start(), so filesystem issues
		// shouldn't prevent daemon shutdown
		socketPath := expandPath(DefaultSocketPath)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cleanupCancel()

		removeDone := make(chan error, 1)
		go func() {
			removeDone <- os.Remove(socketPath)
		}()

		select {
		case err := <-removeDone:
			if err != nil && !os.IsNotExist(err) {
				// Log but don't fail - socket cleaned on next Start()
				log.Printf("status: warning - failed to remove socket file: %v", err)
			}
		case <-cleanupCtx.Done():
			log.Printf("status: warning - timeout removing socket file")
		}
	})

	// Return first error if any
	if len(shutdownErrors) > 0 {
		return shutdownErrors[0]
	}
	return nil
}

// UpdatePID updates the daemon PID
func (sm *StatusManager) UpdatePID(pid int) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.status.PID = pid
}

// SetState updates the daemon state
func (sm *StatusManager) SetState(state DaemonState) {
	sm.setState(state)
}

// SetError sets an error state and message
func (sm *StatusManager) SetError(err error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.status.CurrentState = StateError
	sm.status.LastError = err.Error()
	atomic.AddInt64(&sm.status.ErrorCount, 1)
}

// UpdateSync updates sync-related information
func (sm *StatusManager) UpdateSync(files []string, err error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	now := time.Now()
	sm.status.LastSync = now
	sm.status.FilesSynced = len(files)
	atomic.AddInt64(&sm.status.SyncCount, 1)

	if err != nil {
		sm.status.LastSyncResult = "error: " + err.Error()
		atomic.AddInt64(&sm.status.ErrorCount, 1)
	} else if len(files) > 0 {
		sm.status.LastSyncResult = fmt.Sprintf("synced %d files", len(files))
	} else {
		sm.status.LastSyncResult = "no changes to sync"
	}
}

// SetWatchedPaths sets the list of watched paths
func (sm *StatusManager) SetWatchedPaths(paths []string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.status.WatchedPaths = make([]string, len(paths))
	copy(sm.status.WatchedPaths, paths)
}

// GetStatus returns a copy of the current status
func (sm *StatusManager) GetStatus() DaemonStatus {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// Calculate uptime
	uptime := time.Since(sm.status.StartTime)

	// Copy status to avoid race conditions
	status := sm.status
	status.Uptime = uptime

	return status
}

// acceptConnections accepts and handles incoming socket connections
// This method runs in a goroutine tracked by acceptWG in Start()
// It exits cleanly when sm.ctx is cancelled during Stop()
func (sm *StatusManager) acceptConnections() {
	for {
		// Check context before each accept
		select {
		case <-sm.ctx.Done():
			return // Clean exit on context cancellation
		default:
		}

		// Set accept timeout to avoid indefinite blocking
		// This allows periodic context checks even when no connections arrive
		if tc, ok := sm.socket.(*net.UnixListener); ok {
			_ = tc.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := sm.socket.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Timeout is normal, check context on next iteration
			}
			if sm.ctx.Err() != nil {
				return // Context cancelled during Accept()
			}
			// Log error but continue accepting connections
			continue
		}

		// Handle connection in separate goroutine
		go sm.handleConnection(conn)
	}
}

// handleConnection handles a single socket connection
func (sm *StatusManager) handleConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	// Set connection timeout
	_ = conn.SetDeadline(time.Now().Add(SocketTimeout))

	// Send current status
	status := sm.GetStatus()
	encoder := json.NewEncoder(conn)

	if err := encoder.Encode(status); err != nil {
		// Connection error, just return
		return
	}
}

// setState updates the daemon state (internal method)
func (sm *StatusManager) setState(state DaemonState) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.status.CurrentState = state
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			// Handle the case where path is exactly "~"
			suffix := ""
			if len(path) > 1 {
				suffix = path[1:]
			}
			return filepath.Join(home, suffix)
		}
	}
	return path
}

// GetStatusFromSocket connects to a running daemon's socket and returns its status
func GetStatusFromSocket() (*DaemonStatus, error) {
	socketPath := expandPath(DefaultSocketPath)

	// Create connection with timeout
	conn, err := net.DialTimeout("unix", socketPath, SocketTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon socket: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Set read timeout
	_ = conn.SetDeadline(time.Now().Add(SocketTimeout))

	// Decode status
	var status DaemonStatus
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status: %w", err)
	}

	return &status, nil
}

// IsDaemonRunning checks if a daemon is running by attempting to connect to its socket
func IsDaemonRunning() bool {
	socketPath := expandPath(DefaultSocketPath)

	conn, err := net.DialTimeout("unix", socketPath, 1*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
