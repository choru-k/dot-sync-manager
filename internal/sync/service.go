package sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/ignore"
	"github.com/fsnotify/fsnotify"
)

// Constants for sync service configuration
const (
	DefaultDebounceDelay = 30 * time.Second
	DefaultIgnoreFile    = ".syncignore"
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
// Lock Ordering Rules (to prevent deadlocks):
// 1. Atomic state operations (CompareAndSwapInt32, LoadInt32) - no locks required
// 2. sync.RWMutex (mu) - protects watcher recreation and critical sections
// 3. Atomic shutdownInitiated flag - ensures cleanup happens exactly once per lifecycle
//
// IMPORTANT: Never acquire locks after performing atomic state changes.
// Always use atomic operations for state transitions, and only acquire
// mu for protecting watcher recreation and other resources.
//
// State Transitions (atomic only):
//   StateStopped -> StateRunning (Start())
//   StateRunning -> StateStopping (Stop())
//   StateStopping -> StateStopped (Stop() completion)
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

	// Atomic state management
	state   int32 // atomic ServiceState
	ctx     context.Context
	cancel  context.CancelFunc

	// Goroutine coordination for graceful shutdown
	shutdownWG sync.WaitGroup

	// Thread safety - protects watcher access during state transitions
	// Lock ordering: mu protects watcher recreation, atomic state protects service lifecycle
	mu       sync.RWMutex

	// Flag to ensure shutdown operations happen exactly once per stop cycle
	shutdownInitiated int32 // atomic: 0 = not initiated, 1 = initiated

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

	service := &SyncService{
		gitManager:        gitManager,
		watcher:           watcher,
		debouncer:         debouncerImpl,
		advancedDebouncer: advancedDebouncer,
		ignoreParser:      ignoreParser,
		config:            config,
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
// - Uses atomic shutdownInitiated flag in Stop() to ensure cleanup happens exactly once per lifecycle
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

	// Rollback function to reset state if initialization fails
	rollbackState := func() {
		atomic.StoreInt32(&s.state, int32(StateStopped))
	}

	// Check if this is a restart scenario (service was stopped before)
	// Services support restart after Stop() by recreating all necessary resources
	// Use read lock first for fast path, then write lock for recreation
	s.mu.RLock()
	watcher := s.watcher
	s.mu.RUnlock()

	if watcher == nil {
		s.mu.Lock()
		// Double-check after acquiring write lock
		if s.watcher == nil {
			log.Printf("sync: restarting service - recreating resources for repository: %s", s.config.RepoPath)
			s.ctx, s.cancel = context.WithCancel(context.Background())
			atomic.StoreInt32(&s.shutdownInitiated, 0) // reset shutdown flag for new lifecycle

			newWatcher, err := fsnotify.NewWatcher()
			if err != nil {
				s.mu.Unlock()
				rollbackState()
				return fmt.Errorf("sync: failed to create watcher during service restart [%s]: %w", ErrIDWatcherCreateFailed, err)
			}
			s.watcher = newWatcher

			// Restart advanced debouncer if it was previously stopped
			if s.advancedDebouncer != nil {
				s.advancedDebouncer.Start()
				log.Printf("sync: restarted advanced debouncer for repository: %s", s.config.RepoPath)
			}
			// Update the local watcher reference after successful recreation
			watcher = s.watcher
		}
		s.mu.Unlock()
	}

	// Ensure we have a valid watcher (this should never be nil after the above logic)
	if watcher == nil {
		// This shouldn't happen, but handle it gracefully
		rollbackState()
		return fmt.Errorf("sync: watcher is not available [%s]", ErrIDWatcherCloseFailed)
	}

	// Add the repository path to the watcher
	if err := watcher.Add(s.config.RepoPath); err != nil {
		rollbackState()
		return fmt.Errorf("sync: failed to watch repo path [%s]: %w", ErrIDWatcherAddPathFailed, err)
	}

	// Watch subdirectories recursively
	if err := s.watchRecursiveWithWatcher(watcher, s.config.RepoPath); err != nil {
		rollbackState()
		return fmt.Errorf("sync: failed to watch recursively [%s]: %w", ErrIDRepoWatchRecursive, err)
	}

	
	// Start the event loop
	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()
		s.eventLoop()
	}()

	log.Printf("sync: started watching %s", s.config.RepoPath)
	return nil
}

// Stop stops the file watching service and returns any error encountered during shutdown
func (s *SyncService) Stop() error {
	// Early return if not running - this prevents unnecessary work
	if !atomic.CompareAndSwapInt32(&s.state, int32(StateRunning), int32(StateStopping)) {
		// Either not running or already stopping/stopped
		return nil
	}

	var shutdownErrors []error

	// Ensure shutdown operations happen exactly once using atomic flag
	if atomic.CompareAndSwapInt32(&s.shutdownInitiated, 0, 1) {
		// Cancel context to signal shutdown
		s.cancel()

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
				log.Printf("sync: warning - watcher close error may indicate filesystem issues [%s]: %v", ErrIDWatcherCloseFailed, err)
			}
		} else {
			s.mu.Unlock() // Release lock if no watcher to close
		}
	}

	// Safely transition to stopped state
	atomic.StoreInt32(&s.state, int32(StateStopped))

	// Wait for event loop to finish
	s.shutdownWG.Wait()

	log.Println("sync: stopped")

	// Return any shutdown errors using modern Go error aggregation
	if len(shutdownErrors) > 0 {
		if len(shutdownErrors) == 1 {
			return shutdownErrors[0]
		}
		// Wrap the joined error with an error ID for monitoring
		return fmt.Errorf("sync: multiple errors during shutdown [%s]: %w", ErrIDMultipleShutdownErrors, errors.Join(shutdownErrors...))
	}

	return nil
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
				return fmt.Errorf("sync: failed to watch %s: %w", path, err)
			}
		}

		return nil
	})
}

// eventLoop handles file system events
func (s *SyncService) eventLoop() {
	for {
		// Protect watcher access in event loop - get reference under lock
		s.mu.RLock()
		watcher := s.watcher
		s.mu.RUnlock()

		// If watcher is nil, exit the event loop (service is being stopped)
		if watcher == nil {
			return
		}

		// Check if context is cancelled before accessing watcher channels
		select {
		case <-s.ctx.Done():
			return
		default:
			// Continue to watcher event handling
		}

		// Access watcher channels with the reference we obtained under lock
		// This prevents race conditions where watcher could be set to nil
		select {
		case <-s.ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			s.handleEvent(event)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}

			if s.onError != nil {
				s.onError(fmt.Errorf("sync: watcher error: %w", err))
			}
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

	// Add panic recovery to prevent service crashes
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in manual sync for repository %s [%s]: %v", s.config.RepoPath, ErrIDManualSyncFailed, r)
			log.Printf("sync: %v", err)
			s.notifyError(err)
			s.notifySyncComplete(nil, err)
		}
	}()

	s.notifySyncStart()

	// Use the git manager to stage, commit, and push changes
	changedFiles, err := s.gitManager.StageCommitAndPush(s.ctx, time.Now())

	if err != nil {
		// Check if this is a context cancellation (expected during shutdown)
		if errors.Is(err, context.Canceled) {
			log.Printf("sync: operation cancelled for repository %s: %v", s.config.RepoPath, err)
			// For context cancellation, treat as successful completion with nil error
			s.notifySyncComplete(changedFiles, nil)
			return
		}

		err = fmt.Errorf("manual sync failed on repository %s [%s]: %w", s.config.RepoPath, ErrIDManualSyncFailed, err)
		log.Printf("sync: %v", err)
		s.notifyError(err)
		s.notifySyncComplete(changedFiles, err)
		return
	}

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
