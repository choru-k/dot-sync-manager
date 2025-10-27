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
	running int32 // atomic: 1 = running, 0 = not running
	ctx     context.Context
	cancel  context.CancelFunc

	// Goroutine coordination for graceful shutdown
	shutdownWG sync.WaitGroup

	// Thread safety
	stopOnce sync.Once

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
func (s *SyncService) Start() error {
	// Check if service is already running
	if atomic.LoadInt32(&s.running) == 1 {
		return fmt.Errorf("sync: service already running")
	}

	// Reinitialize resources if this is a restart
	if s.watcher == nil {
		s.ctx, s.cancel = context.WithCancel(context.Background())
		s.stopOnce = sync.Once{}
		s.shutdownWG = sync.WaitGroup{}

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return fmt.Errorf("sync: failed to create watcher: %w", err)
		}
		s.watcher = watcher
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
		s.eventLoop()
	}()

	log.Printf("sync: started watching %s", s.config.RepoPath)
	return nil
}

// Stop stops the file watching service and returns any error encountered during shutdown
func (s *SyncService) Stop() error {
	// Early return if not running - this prevents burning sync.Once
	if atomic.LoadInt32(&s.running) == 0 {
		return nil
	}

	var shutdownErrors []error
	var wasRunning bool

	// Perform shutdown operations exactly once
	s.stopOnce.Do(func() {
		wasRunning = atomic.SwapInt32(&s.running, 0) == 1
		if !wasRunning {
			return
		}

		// Cancel context to signal shutdown
		s.cancel()

		// Cancel pending debounces
		s.debouncer.CancelAll()

		// Stop advanced debouncer if used
		if s.advancedDebouncer != nil {
			if err := s.advancedDebouncer.Stop(); err != nil {
				shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to stop advanced debouncer: %w", err))
			}
		}

		// Close watcher
		if s.watcher != nil {
			if err := s.watcher.Close(); err != nil {
				shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to close watcher: %w", err))
			}
		}
	})

	// If service wasn't running when stopOnce was executed, nothing to do
	if !wasRunning {
		return nil
	}

	// Wait for event loop to finish
	s.shutdownWG.Wait()

	// Clean up for potential restart
	s.watcher = nil

	log.Println("sync: stopped")

	// Return any shutdown errors
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
			err := fmt.Errorf("panic in manual sync for repository %s: %v", s.config.RepoPath, r)
			log.Printf("sync: %v", err)
			s.notifyError(err)
			s.notifySyncComplete(nil, err)
		}
	}()

	s.notifySyncStart()

	// Use the git manager to stage, commit, and push changes
	changedFiles, err := s.gitManager.StageCommitAndPush(s.ctx, time.Now())

	s.notifySyncComplete(changedFiles, err)

	if err != nil {
		err = fmt.Errorf("manual sync failed on repository %s: %w", s.config.RepoPath, err)
		log.Printf("sync: %v", err)
		s.notifyError(err)
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
