package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/ignore"
	"github.com/choru-k/dot-sync-manager/internal/status"
	"github.com/fsnotify/fsnotify"
)

// Constants for sync service configuration
const (
	DefaultDebounceDelay = 30 * time.Second
	DefaultIgnoreFile    = ".syncignore"

	// EventLoopTimeout determines how frequently the event loop checks for
	// context cancellation and service state changes. This prevents indefinite
	// blocking on watcher channels while maintaining responsiveness.
	EventLoopTimeout = 100 * time.Millisecond
)

// DebouncerInterface defines the interface for both basic and advanced debouncers
type DebouncerInterface interface {
	Add(key string, fn func())
	Cancel(key string)
	CancelAll()
	Pending() int
}

// ServiceState represents the operational state of the sync service
type ServiceState int32

const (
	StateStopped ServiceState = iota
	StateRunning
	StateStopping
)

// stateToString converts ServiceState to human-readable string
func stateToString(state ServiceState) string {
	switch state {
	case StateStopped:
		return "stopped"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// SyncService handles file watching and automatic syncing
//
// Thread Safety:
// - All methods are thread-safe and can be called concurrently
// - Stop() is idempotent and can be called multiple times safely
// - Uses sync.Once to ensure cleanup operations happen exactly once
// - Uses atomic state management to prevent race conditions
// - The service can be safely started and stopped from multiple goroutines
//
// Lifecycle Management:
// - Create a new SyncService instance for each service lifecycle
// - Start() can only be called once per instance
// - Stop() can be called multiple times safely (idempotent)
// - ⚠️  IMPORTANT: After Stop(), the service cannot be restarted. Create a new SyncService instance instead.
// - This design follows Go idioms and avoids complex restart logic and resource leaks
//
// Lock Ordering Rules (to prevent deadlocks):
// 1. Atomic state operations (CompareAndSwapInt32, LoadInt32) - no locks required
// 2. sync.RWMutex (mu) - protects watcher recreation and critical sections
// 3. sync.Once - ensures cleanup happens exactly once per lifecycle
//
// IMPORTANT: Never acquire locks after performing atomic state changes.
// Always use atomic operations for state transitions, and only acquire
// mu for protecting watcher recreation and other resources.
//
// State Transitions (atomic only):
//
//	StateStopped -> StateRunning (Start())
//	StateRunning -> StateStopping (Stop())
//	StateStopping -> StateStopped (Stop() completion)
//
// Resource Protection:
// - mu.RLock()/RUnlock(): Protects watcher reference access in eventLoop
// - mu.Lock()/Unlock(): Protects watcher recreation during restart scenarios
// - No lock should be held during blocking I/O operations
type SyncService struct {
	gitManager   *gitmanager.GitManager
	watcher      *fsnotify.Watcher
	debouncer    DebouncerInterface
	ignoreParser *ignore.Parser
	config       *Config

	// Advanced debouncer reference (if used)
	advancedDebouncer *debouncer.AdvancedDebouncer

	// Status management
	statusManager *status.StatusManager

	// Atomic state management
	state  int32 // atomic ServiceState
	ctx    context.Context
	cancel context.CancelFunc

	// Goroutine coordination for graceful shutdown
	shutdownWG sync.WaitGroup

	// Thread safety - protects watcher access during state transitions
	// Lock ordering: mu protects watcher recreation, atomic state protects service lifecycle
	mu sync.RWMutex

	// Once to ensure shutdown operations happen exactly once per stop cycle
	shutdownOnce sync.Once

	// Error tracking for concurrent Stop() calls
	lastShutdownError error
	shutdownErrorMu   sync.Mutex

	// Shutdown monitoring
	_ int32 // TODO: implement forced shutdowns tracking for monitoring

	// Event callbacks
	onSyncStart    func()
	onSyncComplete func(files []string, err error)
	onError        func(error)
}

// Config holds configuration for the sync service
type Config struct {
	// Repository path (absolute path to dotfiles directory)
	RepoPath string

	// Debounce delay - how long to wait after last change before syncing
	DebounceDelay time.Duration

	// Auto-sync enabled
	AutoSyncEnabled bool

	// Ignore file path (relative to repo)
	IgnoreFile string

	// Advanced debouncer configuration
	Backoff *debouncer.AdvancedDebouncerConfig

	// Application version for status reporting
	Version string

	// Configuration file path for status reporting
	ConfigPath string
}

// Validate validates the configuration according to style guide requirements
func (c *Config) Validate() error {
	if c.RepoPath == "" {
		return fmt.Errorf("repo path must be provided [%s]", ErrIDConfigRepoPathRequired)
	}
	if !filepath.IsAbs(c.RepoPath) {
		return fmt.Errorf("repo path must be absolute [%s]", ErrIDConfigRepoPathRequired)
	}
	return nil
}

// SetDefaults sets reasonable defaults for the configuration
func (c *Config) SetDefaults() {
	if c.DebounceDelay == 0 {
		c.DebounceDelay = DefaultDebounceDelay
	}
	if c.IgnoreFile == "" {
		c.IgnoreFile = DefaultIgnoreFile
	}
}

// New creates a new sync service
func New(gitManager *gitmanager.GitManager, config *Config) (*SyncService, error) {
	if config == nil {
		return nil, fmt.Errorf("sync: config is required [%s]", ErrIDConfigRequired)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Set defaults for any missing values
	config.SetDefaults()

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("sync: failed to create watcher [%s]: %w", ErrIDWatcherCreateFailed, err)
	}

	// Create debouncer (advanced or basic based on configuration)
	var debouncerImpl DebouncerInterface
	var advancedDebouncer *debouncer.AdvancedDebouncer

	if config.Backoff != nil {
		// Use advanced debouncer
		advancedDebouncer = debouncer.NewAdvanced(*config.Backoff)
		advancedDebouncer.Start()
		debouncerImpl = advancedDebouncer
	} else {
		// Use basic debouncer for backward compatibility
		debouncerImpl = debouncer.New(config.DebounceDelay)
	}

	// Create ignore parser
	ignoreParser := ignore.New(config.RepoPath)

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize status manager
	statusManager := status.NewStatusManager(config.Version, config.ConfigPath)

	service := &SyncService{
		gitManager:        gitManager,
		watcher:           watcher,
		debouncer:         debouncerImpl,
		advancedDebouncer: advancedDebouncer,
		ignoreParser:      ignoreParser,
		config:            config,
		statusManager:     statusManager,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Load ignore patterns
	ignoreFilePath := filepath.Join(config.RepoPath, config.IgnoreFile)
	if err := service.ignoreParser.LoadFromFile(ignoreFilePath); err != nil {
		log.Printf("sync: warning - failed to load ignore file: %v", err)
	}

	return service, nil
}

// Start begins watching for file changes
//
// Thread Safety:
// - This method is thread-safe and can be called concurrently
// - Uses atomic CompareAndSwapInt32 for lock-free state transitions
// - Supports service restart after Stop() by recreating resources
// - Uses sync.Once in Stop() to ensure cleanup happens exactly once per lifecycle
// - Restart functionality allows service reuse without creating new instances
func (s *SyncService) Start() error {
	// Fast atomic check for running state
	if !atomic.CompareAndSwapInt32(&s.state, int32(StateStopped), int32(StateRunning)) {
		currentState := atomic.LoadInt32(&s.state)
		if currentState == int32(StateRunning) {
			return fmt.Errorf("sync: service already running [%s]", ErrIDServiceAlreadyRunning)
		}
		return fmt.Errorf("sync: service is stopping, please wait (current state: %s) [%s]", stateToString(ServiceState(currentState)), ErrIDServiceStopRaceCondition)
	}

	// Track whether event loop was started for proper cleanup
	eventLoopStarted := false

	// Rollback function to reset state and clean up resources if initialization fails
	rollbackState := func() {
		atomic.StoreInt32(&s.state, int32(StateStopped))

		// Cancel context to stop event loop if it was started
		if eventLoopStarted {
			s.cancel()
		}

		// Clean up any resources that were created during Start()
		s.mu.Lock()
		if s.watcher != nil {
			// Close the watcher to prevent resource leaks
			if err := s.watcher.Close(); err != nil {
				log.Printf("sync: warning - failed to close watcher during rollback [%s]: %v", ErrIDWatcherCloseFailed, err)
			}
			s.watcher = nil
		}
		s.mu.Unlock()

		// Cancel debouncer to prevent background goroutines
		if s.debouncer != nil {
			s.debouncer.CancelAll()

			// Additional cleanup for advanced debouncer to prevent goroutine leaks
			if s.advancedDebouncer != nil {
				// Stop the advanced debouncer to clean up its goroutines and channels
				if err := s.advancedDebouncer.Stop(); err != nil {
					log.Printf("sync: warning - failed to stop advanced debouncer during rollback [%s]: %v", ErrIDAdvancedDebouncerStopFailed, err)
				}
			}
		}

		// Reset shutdownOnce to allow clean shutdown if needed later
		s.shutdownOnce = sync.Once{}
	}

	// Check if resources already exist - if not, this is a restart scenario
	// For simplicity and reliability, we don't support restart after Stop()
	// Users should create a new SyncService instance instead
	s.mu.Lock()
	watcher := s.watcher
	s.mu.Unlock()

	if watcher == nil {
		rollbackState()
		return fmt.Errorf("sync: service has been stopped and cannot be restarted [%s]: %s", ErrIDServiceRestartNotSupported, ErrIDServiceAlreadyStopped)
	}

	// Ensure we have a valid watcher (this should never be nil after the above logic)
	if watcher == nil {
		// This shouldn't happen, but handle it gracefully
		rollbackState()
		return fmt.Errorf("sync: watcher is not available [%s]", ErrIDWatcherCloseFailed)
	}

	// Start status manager
	if err := s.statusManager.Start(); err != nil {
		return fmt.Errorf("sync: failed to start status manager: %w", err)
	}

	// Update PID in status manager
	if pid := os.Getpid(); pid > 0 {
		s.statusManager.UpdatePID(pid)
	}

	// Add the repository path to the watcher
	if err := watcher.Add(s.config.RepoPath); err != nil {
		s.statusManager.SetError(fmt.Errorf("failed to watch repo path: %w", err))
		_ = s.statusManager.Stop() // Explicitly ignore error in cleanup context
		rollbackState()
		return fmt.Errorf("sync: failed to watch repo path [%s]: %w", ErrIDWatcherAddPathFailed, err)
	}

	// Watch subdirectories recursively
	if err := s.watchRecursiveWithWatcher(watcher, s.config.RepoPath); err != nil {
		s.statusManager.SetError(fmt.Errorf("failed to watch recursively: %w", err))
		_ = s.statusManager.Stop() // Explicitly ignore error in cleanup context
		rollbackState()
		return fmt.Errorf("sync: failed to watch recursively [%s]: %w", ErrIDRepoWatchRecursive, err)
	}

	// Update status with watched paths
	watchedPaths := s.getWatchedPaths()
	s.statusManager.SetWatchedPaths(watchedPaths)
	s.statusManager.SetState(status.StateRunning)

	// Start the event loop
	eventLoopStarted = true
	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()
		s.eventLoop()
	}()

	log.Printf("sync: started watching %s", s.config.RepoPath)
	return nil
}

// Stop stops the file watching service and returns any error encountered during shutdown
//
// ⚠️  IMPORTANT: After calling Stop(), this SyncService instance cannot be restarted.
// Create a new SyncService instance instead. This limitation exists because:
//   - File system watchers cannot be reliably recreated after Close()
//   - Context cancellation permanently disables the event loop
//   - Goroutine coordination structures are designed for one-time use
//
// Error Handling Contract:
// - Errors indicate resource leaks but service is still stopped
// - Returns error only for monitoring/alerting purposes
// - Service state will be StateStopped even if errors occur during cleanup
// - Errors are aggregated and returned as a single error using errors.Join()
// - Non-critical errors (like watcher close failures) are logged but don't prevent shutdown
//
// Thread Safety:
// - This method is thread-safe and can be called concurrently
// - Multiple concurrent calls to Stop() are safe due to sync.Once and atomic state management
// - Will wait for any in-progress Stop() operations to complete before returning
// - Uses sync.Once to ensure cleanup operations happen exactly once per lifecycle
func (s *SyncService) Stop() error {
	// Early return if not running - this prevents unnecessary work
	if !atomic.CompareAndSwapInt32(&s.state, int32(StateRunning), int32(StateStopping)) {
		// Check the previous state after CAS fails to handle premature Stop() correctly
		current := atomic.LoadInt32(&s.state)
		if current == int32(StateStopping) {
			// Already stopping - wait for completion to prevent race condition
			// Use a timeout to prevent potential deadlocks
			done := make(chan struct{})
			go func() {
				s.shutdownWG.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Shutdown completed successfully
			case <-time.After(5 * time.Second):
				// Timeout - don't block forever, log warning and continue
				log.Printf("sync: warning - shutdown wait timed out, this may indicate a deadlock [%s]", ErrIDShutdownTimeout)
			}

			// Return any error that occurred during the concurrent shutdown
			s.shutdownErrorMu.Lock()
			lastError := s.lastShutdownError
			s.shutdownErrorMu.Unlock()
			if lastError != nil {
				return lastError
			}
		}
		return nil
	}

	var shutdownErrors []error

	// Ensure shutdown operations happen exactly once using sync.Once
	s.shutdownOnce.Do(func() {
		// Cancel context to signal shutdown
		s.cancel()

		// Stop status manager
		if s.statusManager != nil {
			if err := s.statusManager.Stop(); err != nil {
				shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to stop status manager: %w", err))
			}
		}

		// Cancel pending debounces
		s.debouncer.CancelAll()

		// Stop advanced debouncer if used
		if s.advancedDebouncer != nil {
			if err := s.advancedDebouncer.Stop(); err != nil {
				shutdownErrors = append(shutdownErrors, fmt.Errorf("sync: failed to stop advanced debouncer for repository %s [%s]: %w", s.config.RepoPath, ErrIDAdvancedDebouncerStopFailed, err))
			}
		}

		// Close watcher with enhanced error context and thread safety
		s.mu.Lock()
		if s.watcher != nil {
			// Get reference for closing (will be nil-ed below)
			watcherToClose := s.watcher
			s.watcher = nil // Prevent further access

			s.mu.Unlock() // Release lock before blocking Close() call

			if err := watcherToClose.Close(); err != nil {
				shutdownErrors = append(shutdownErrors, fmt.Errorf("sync: failed to close file system watcher for repository %s [%s]: %w", s.config.RepoPath, ErrIDWatcherCloseFailed, err))
				log.Printf("sync: ERROR - watcher close failed, potential resource leak detected [%s]: %v", ErrIDWatcherCloseFailed, err)
			}
		} else {
			s.mu.Unlock() // Release lock if no watcher to close
		}
	})

	// Wait for event loop to finish BEFORE transitioning to stopped state
	// This prevents a race condition where Start() could increment the WaitGroup
	// after Stop() has already begun waiting, which would cause a deadlock
	// Use a timeout to prevent potential deadlocks
	done := make(chan struct{})
	go func() {
		s.shutdownWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Event loop completed successfully
	case <-time.After(5 * time.Second):
		// Timeout - don't block forever, log warning and continue
		log.Printf("sync: warning - event loop shutdown wait timed out [%s]", ErrIDShutdownTimeout)
	}

	// Safely transition to stopped state only after all goroutines have finished
	atomic.StoreInt32(&s.state, int32(StateStopped))

	log.Println("sync: stopped")

	// Store the final error for concurrent callers
	var finalError error
	if len(shutdownErrors) > 0 {
		if len(shutdownErrors) == 1 {
			finalError = fmt.Errorf("sync: shutdown error [%s]: %w", ErrIDShutdownError, shutdownErrors[0])
		} else {
			// Wrap the joined error with an error ID for monitoring
			finalError = fmt.Errorf("sync: multiple errors during shutdown [%s]: %w", ErrIDMultipleShutdownErrors, errors.Join(shutdownErrors...))
		}

		// Store error for concurrent callers
		s.shutdownErrorMu.Lock()
		s.lastShutdownError = finalError
		s.shutdownErrorMu.Unlock()
	}

	return finalError
}

// watchRecursive adds all subdirectories to the watcher
func (s *SyncService) watchRecursive(root string) error {
	// Get a reference to the current watcher for this operation
	s.mu.RLock()
	watcher := s.watcher
	s.mu.RUnlock()

	return s.watchRecursiveWithWatcher(watcher, root)
}

// watchRecursiveWithWatcher adds all subdirectories to the given watcher
func (s *SyncService) watchRecursiveWithWatcher(watcher *fsnotify.Watcher, root string) error {
	// If watcher is nil, return early (service is being stopped)
	if watcher == nil {
		return fmt.Errorf("sync: watcher is not available [%s]", ErrIDWatcherCloseFailed)
	}

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directories entirely
		if filepath.Base(path) == ".git" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Watch directories that aren't ignored
		if info.IsDir() {
			if s.ignoreParser.Match(path) {
				return filepath.SkipDir
			}

			if err := watcher.Add(path); err != nil {
				// Log warning but continue - single directory failure shouldn't stop the service
				log.Printf("sync: warning - failed to watch %s: %v", path, err)
			}
		}

		return nil
	})
}

// eventLoop handles file system events
func (s *SyncService) eventLoop() {
	for {
		// Check if context is cancelled first - this ensures prompt exit
		select {
		case <-s.ctx.Done():
			return
		default:
			// Continue to watcher access
		}

		// Protect watcher access in event loop - get reference under lock
		s.mu.RLock()
		watcher := s.watcher
		s.mu.RUnlock()

		// If watcher is nil, exit the event loop (service is being stopped)
		if watcher == nil {
			return
		}

		// Use a timeout select to prevent blocking indefinitely on watcher channels
		// This allows the event loop to check context cancellation and watcher nil status periodically
		timeout := time.NewTimer(EventLoopTimeout)
		select {
		case <-s.ctx.Done():
			timeout.Stop()
			return

		case event, ok := <-watcher.Events:
			timeout.Stop()
			if !ok {
				return
			}
			s.handleEvent(event)

		case err, ok := <-watcher.Errors:
			timeout.Stop()
			if !ok {
				return
			}
			if s.onError != nil {
				s.onError(fmt.Errorf("sync: watcher error: %w", err))
			}

		case <-timeout.C:
			// Timeout occurred - loop again to check context and watcher status
			// This prevents indefinite blocking and allows graceful shutdown
			timeout.Stop()
			continue
		}
	}
}

// handleEvent processes a single file system event
func (s *SyncService) handleEvent(event fsnotify.Event) {
	// Skip ignored paths and .git files
	if s.ignoreParser.Match(event.Name) || filepath.Base(event.Name) == ".git" {
		return
	}

	log.Printf("sync: file event: %s %s", event.Op, event.Name)

	// If a directory was created, watch it recursively
	if event.Op&fsnotify.Create == fsnotify.Create {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := s.watchRecursive(event.Name); err != nil {
				log.Printf("sync: warning - failed to watch new directory %s: %v", event.Name, err)
			} else {
				log.Printf("sync: added new directory to watcher: %s", event.Name)
			}
		}
	}

	// Debounce the sync operation
	s.debouncer.Add("sync", func() {
		s.performSync()
	})
}

// performSync executes the actual sync operation (for auto-sync)
func (s *SyncService) performSync() {
	if !s.config.AutoSyncEnabled {
		return
	}

	s.performManualSync()
}

// performManualSync executes sync operation regardless of AutoSyncEnabled setting
func (s *SyncService) performManualSync() {
	log.Printf("sync: performing manual sync on repository: %s", s.config.RepoPath)

	// Update status to syncing
	s.statusManager.SetState(status.StateSyncing)

	// Add panic recovery to prevent service crashes
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic in manual sync for repository %s [%s]: %v", s.config.RepoPath, ErrIDManualSyncFailed, r)
			log.Printf("sync: %s", errMsg)

			// Update status with error
			s.statusManager.SetError(fmt.Errorf("sync: %s", errMsg))
			s.statusManager.SetState(status.StateRunning)

			s.notifyError(fmt.Errorf("sync: %s", errMsg))
			s.notifySyncComplete(nil, fmt.Errorf("sync: %s", errMsg))
		}
	}()

	s.notifySyncStart()

	// Use the git manager to stage, commit, and push changes
	changedFiles, err := s.gitManager.StageCommitAndPush(s.ctx, time.Now())

	// Update status with sync results
	s.statusManager.UpdateSync(changedFiles, err)

	if err != nil {
		// Check if this is a context cancellation (expected during shutdown)
		if errors.Is(err, context.Canceled) {
			log.Printf("sync: operation cancelled for repository %s: %v", s.config.RepoPath, err)
			// For context cancellation, treat as successful completion with nil error
			s.notifySyncComplete(changedFiles, nil)
			return
		}

		errMsg := fmt.Sprintf("manual sync failed on repository %s [%s]: %v", s.config.RepoPath, ErrIDManualSyncFailed, err)
		log.Printf("sync: %s", errMsg)

		// Update status with error
		s.statusManager.SetError(fmt.Errorf("sync: %s", errMsg))
		s.statusManager.SetState(status.StateRunning)

		s.notifyError(fmt.Errorf("sync: %s", errMsg))
		s.notifySyncComplete(changedFiles, err)
		return
	}

	// Update status back to running
	s.statusManager.SetState(status.StateRunning)

	// Only notify success if we reached here without errors
	s.notifySyncComplete(changedFiles, nil)

	if len(changedFiles) > 0 {
		log.Printf("sync: manually synced %d files from repository %s: %v", len(changedFiles), s.config.RepoPath, changedFiles)
	} else {
		log.Printf("sync: no changes to sync in repository: %s", s.config.RepoPath)
	}
}

// ManualSync triggers an immediate sync without debouncing
func (s *SyncService) ManualSync() error {
	// Check if service is running using atomic operation to avoid race conditions
	if !s.IsRunning() {
		currentState := atomic.LoadInt32(&s.state)
		return fmt.Errorf("sync service is %s - cannot perform manual sync on repository: %s [%s]", stateToString(ServiceState(currentState)), s.config.RepoPath, ErrIDServiceStopRaceCondition)
	}

	log.Printf("sync: performing manual sync on repository: %s", s.config.RepoPath)

	// If using advanced debouncer, use its manual sync feature
	if s.advancedDebouncer != nil {
		err := s.advancedDebouncer.TriggerManualSync("manual_sync", func() {
			s.performManualSync()
		})
		if err != nil {
			return fmt.Errorf("sync: manual sync failed on repository %s [%s]: %w", s.config.RepoPath, ErrIDManualSyncFailed, err)
		}
		return nil
	}

	// Otherwise perform sync directly
	s.performManualSync()
	return nil
}

// SetEventCallbacks sets the event callback functions
func (s *SyncService) SetEventCallbacks(onSyncStart func(), onSyncComplete func([]string, error), onError func(error)) {
	s.onSyncStart = onSyncStart
	s.onSyncComplete = onSyncComplete
	s.onError = onError
}

// notifySyncStart triggers the sync start callback if set
func (s *SyncService) notifySyncStart() {
	if s.onSyncStart != nil {
		s.onSyncStart()
	}
}

// notifySyncComplete triggers the sync complete callback if set
func (s *SyncService) notifySyncComplete(files []string, err error) {
	if s.onSyncComplete != nil {
		s.onSyncComplete(files, err)
	}
}

// notifyError triggers the error callback if set
func (s *SyncService) notifyError(err error) {
	if s.onError != nil {
		s.onError(err)
	}
}

// IsRunning returns whether the service is currently running
func (s *SyncService) IsRunning() bool {
	return atomic.LoadInt32(&s.state) == int32(StateRunning)
}

// GetConfig returns the current configuration for the sync service.
// The returned configuration should not be modified.
func (s *SyncService) GetConfig() *Config {
	return s.config
}

// ReloadIgnorePatterns reloads the ignore patterns from file
func (s *SyncService) ReloadIgnorePatterns() error {
	ignoreFilePath := filepath.Join(s.config.RepoPath, s.config.IgnoreFile)
	s.ignoreParser.Clear()

	if err := s.ignoreParser.LoadFromFile(ignoreFilePath); err != nil {
		return fmt.Errorf("sync: failed to reload ignore patterns [%s]: %w", ErrIDReloadIgnorePatterns, err)
	}

	log.Println("sync: reloaded ignore patterns")
	return nil
}

// GetStats returns statistics about the sync service including running state,
// configuration, and advanced debouncer statistics if available.
func (s *SyncService) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"running":            atomic.LoadInt32(&s.state) == int32(StateRunning),
		"state":              stateToString(ServiceState(atomic.LoadInt32(&s.state))),
		"repo_path":          s.config.RepoPath,
		"debounce_delay":     s.config.DebounceDelay.String(),
		"auto_sync":          s.config.AutoSyncEnabled,
		"pending_debounces":  s.debouncer.Pending(),
		"ignore_patterns":    len(s.ignoreParser.GetPatterns()),
		"advanced_debouncer": s.advancedDebouncer != nil,
	}

	// Add advanced debouncer stats if available
	if s.advancedDebouncer != nil {
		advancedStats := s.advancedDebouncer.GetStats()
		stats["backoff_stats"] = advancedStats
	}

	return stats
}

// GetStatusManager returns the status manager
func (s *SyncService) GetStatusManager() *status.StatusManager {
	return s.statusManager
}

// getWatchedPaths returns a list of currently watched paths
func (s *SyncService) getWatchedPaths() []string {
	// Protect watcher access with read lock to prevent race conditions
	s.mu.RLock()
	watcher := s.watcher
	s.mu.RUnlock()

	if watcher == nil {
		return nil
	}

	// Get the watched paths from the watcher
	// Note: fsnotify doesn't provide a direct way to get watched paths,
	// so we'll return the repo path and recursively watched directories
	paths := []string{s.config.RepoPath}

	// Walk the repository and add all directories that would be watched
	_ = filepath.Walk(s.config.RepoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignore errors and continue
		}

		// Skip .git directory
		if strings.Contains(path, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include directories
		if info.IsDir() {
			// Check if directory should be ignored
			if s.ignoreParser.Match(path) {
				return filepath.SkipDir
			}
			paths = append(paths, path)
		}

		return nil
	})

	return paths
}
