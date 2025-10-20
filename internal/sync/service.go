package sync

import (
	"context"
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
	"github.com/fsnotify/fsnotify"
)

// DebouncerInterface defines the interface for both basic and advanced debouncers
type DebouncerInterface interface {
	Add(key string, fn func())
	Cancel(key string)
	CancelAll()
	Pending() int
}

const (
	// syncServiceTimeout is the maximum time to wait for sync service operations
	syncServiceTimeout = 30 * time.Minute
)

// SyncService handles file watching and automatic syncing
//
// Thread Safety:
// - All methods are thread-safe and can be called concurrently
// - Stop() is idempotent and can be called multiple times safely
// - Uses sync.Once to ensure cleanup operations happen exactly once
// - Uses atomic operations for the running state to prevent race conditions
// - The service can be safely started and stopped from multiple goroutines
type SyncService struct {
	gitManager   *gitmanager.GitManager
	watcher      *fsnotify.Watcher
	debouncer    DebouncerInterface
	ignoreParser *ignore.Parser
	config       *Config

	// Advanced debouncer reference (if used)
	advancedDebouncer *debouncer.AdvancedDebouncer

	// State
	running int32 // atomic.Bool replacement: 1 = running, 0 = not running
	stopped int32 // atomic.Bool replacement: 1 = stopped, 0 = not stopped
	ctx     context.Context
	cancel  context.CancelFunc

	// Goroutine coordination for graceful shutdown
	shutdownWG sync.WaitGroup

	// Thread safety
	stopOnce sync.Once

	// Deadlock prevention: track if Stop() is called from eventLoop
	inEventLoop int32 // atomic.Bool: 1 if in eventLoop, 0 otherwise

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

// New creates a new sync service
func New(gitManager *gitmanager.GitManager, config *Config) (*SyncService, error) {
	if config == nil {
		return nil, fmt.Errorf("sync: config is required")
	}
	if config.RepoPath == "" {
		return nil, fmt.Errorf("sync: repo path is required")
	}
	if config.DebounceDelay == 0 {
		config.DebounceDelay = 30 * time.Second // Default from PRD
	}
	if config.IgnoreFile == "" {
		config.IgnoreFile = ".syncignore"
	}

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("sync: failed to create watcher: %w", err)
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

	ctx, cancel := context.WithTimeout(context.Background(), syncServiceTimeout)

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
func (s *SyncService) Start() error {
	// Check if service is already running
	if atomic.LoadInt32(&s.running) == 1 {
		return fmt.Errorf("sync: service already running")
	}

	// Check if service has been stopped before - prevent reuse
	if atomic.LoadInt32(&s.stopped) == 1 {
		return fmt.Errorf("sync: service cannot be restarted after Stop() - create a new service instance")
	}

	// Add the repository path to the watcher
	if err := s.watcher.Add(s.config.RepoPath); err != nil {
		return fmt.Errorf("sync: failed to watch repo path: %w", err)
	}

	// Watch subdirectories recursively
	if err := s.watchRecursive(s.config.RepoPath); err != nil {
		return fmt.Errorf("sync: failed to watch recursively: %w", err)
	}

	atomic.StoreInt32(&s.running, 1)

	// Start the event loop
	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()
		atomic.StoreInt32(&s.inEventLoop, 1)
		defer atomic.StoreInt32(&s.inEventLoop, 0)
		s.eventLoop()
	}()

	log.Printf("sync: started watching %s", s.config.RepoPath)
	return nil
}

// Stop stops the file watching service and returns any error encountered during shutdown
func (s *SyncService) Stop() error {
	var shutdownErrors []error

	// First, atomically check and claim the shutdown responsibility
	// This prevents multiple goroutines from attempting shutdown simultaneously
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		// Service was not running (running was already 0), nothing to do
		return nil
	}

	// Use sync.Once to ensure cleanup happens only once, even if called concurrently
	s.stopOnce.Do(func() {
		// State management simplified:
		// - running (atomic int32): Single source of truth for running state
		// - stopped (atomic int32): Prevents service reuse after Stop()
		// This provides both performance (atomic reads) and thread safety
		// with minimal complexity and no race conditions

		// Mark service as stopped before cleanup to prevent reuse
		atomic.StoreInt32(&s.stopped, 1)

		s.cancel()

		// Cancel pending debounces
		s.debouncer.CancelAll()

		// Stop advanced debouncer if used
		if s.advancedDebouncer != nil {
			s.advancedDebouncer.Stop()
		}

		// Close watcher
		if s.watcher != nil {
			if err := s.watcher.Close(); err != nil {
				shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to close watcher: %w", err))
			}
		}

			log.Println("sync: shutdown initiated")
	})

	// Wait for the eventLoop goroutine to finish processing
	// This is done outside the sync.Once, but we need to handle the case where
	// Stop() is called from within the event loop goroutine to prevent deadlock
	if atomic.LoadInt32(&s.inEventLoop) == 0 {
		// We're not in the eventLoop goroutine, so it's safe to wait
		s.shutdownWG.Wait()
		// Set watcher to nil after eventLoop has exited so it can be recreated in Start()
		s.watcher = nil
	}
	// If we're in the eventLoop goroutine, the Wait() would cause deadlock,
	// so we skip it. The eventLoop will exit naturally due to context cancellation.
	// In this case, we don't set watcher to nil as it could cause nil pointer dereference

	log.Println("sync: stopped")

	// Combine any shutdown errors into a single error
	if len(shutdownErrors) > 0 {
		if len(shutdownErrors) == 1 {
			return shutdownErrors[0]
		}
		return fmt.Errorf("multiple errors during shutdown: %v", shutdownErrors)
	}

	return nil
}

// watchRecursive adds all subdirectories to the watcher
func (s *SyncService) watchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the .git directory
		if strings.Contains(path, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only watch directories
		if info.IsDir() {
			// Check if directory should be ignored
			if s.ignoreParser.Match(path) {
				return filepath.SkipDir
			}

			if err := s.watcher.Add(path); err != nil {
				log.Printf("sync: warning - failed to watch %s: %v", path, err)
			}
		}

		return nil
	})
}

// eventLoop handles file system events
func (s *SyncService) eventLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return

		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			s.handleEvent(event)

		case err, ok := <-s.watcher.Errors:
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
	// Skip if the path should be ignored
	if s.ignoreParser.Match(event.Name) {
		return
	}

	// Skip .git files
	if strings.Contains(event.Name, ".git") {
		return
	}

	// Log the event for debugging
	log.Printf("sync: file event: %s %s", event.Op, event.Name)

	// If a directory was created, add it to the watcher
	if event.Op&fsnotify.Create == fsnotify.Create {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			// Add the new directory to the watcher recursively
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
			errMsg := fmt.Sprintf("panic in manual sync for repository %s: %v", s.config.RepoPath, r)
			log.Printf("sync: %s", errMsg)

			if s.onError != nil {
				s.onError(fmt.Errorf("sync: %s", errMsg))
			}

			if s.onSyncComplete != nil {
				s.onSyncComplete(nil, fmt.Errorf("sync: %s", errMsg))
			}
		}
	}()

	if s.onSyncStart != nil {
		s.onSyncStart()
	}

	// Use the git manager to stage, commit, and push changes
	changedFiles, err := s.gitManager.StageCommitAndPush(s.ctx, time.Now())

	if s.onSyncComplete != nil {
		s.onSyncComplete(changedFiles, err)
	}

	if err != nil {
		errMsg := fmt.Sprintf("manual sync failed on repository %s: %v", s.config.RepoPath, err)
		log.Printf("sync: %s", errMsg)
		if s.onError != nil {
			s.onError(fmt.Errorf("sync: %s", errMsg))
		}
		return
	}

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
		return fmt.Errorf("sync service is stopped - cannot perform manual sync on repository: %s", s.config.RepoPath)
	}

	log.Printf("sync: performing manual sync on repository: %s", s.config.RepoPath)

	// If using advanced debouncer, use its manual sync feature
	if s.advancedDebouncer != nil {
		err := s.advancedDebouncer.TriggerManualSync("manual_sync", func() {
			s.performManualSync()
		})
		if err != nil {
			return fmt.Errorf("sync: manual sync failed on repository %s: %w", s.config.RepoPath, err)
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

// IsRunning returns whether the service is currently running
func (s *SyncService) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// GetConfig returns the current configuration
func (s *SyncService) GetConfig() *Config {
	return s.config
}

// ReloadIgnorePatterns reloads the ignore patterns from file
func (s *SyncService) ReloadIgnorePatterns() error {
	ignoreFilePath := filepath.Join(s.config.RepoPath, s.config.IgnoreFile)
	s.ignoreParser.Clear()

	if err := s.ignoreParser.LoadFromFile(ignoreFilePath); err != nil {
		return fmt.Errorf("sync: failed to reload ignore patterns: %w", err)
	}

	log.Println("sync: reloaded ignore patterns")
	return nil
}

// GetStats returns statistics about the sync service
func (s *SyncService) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"running":            atomic.LoadInt32(&s.running) == 1,
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
