package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
type SyncService struct {
	gitManager   *gitmanager.GitManager
	watcher      *fsnotify.Watcher
	debouncer    DebouncerInterface
	ignoreParser *ignore.Parser
	config       *Config

	// Advanced debouncer reference (if used)
	advancedDebouncer *debouncer.AdvancedDebouncer

	// State
	running bool
	stopped bool
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.RWMutex

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
	if s.running {
		return fmt.Errorf("sync: service already running")
	}

	// Add the repository path to the watcher
	if err := s.watcher.Add(s.config.RepoPath); err != nil {
		return fmt.Errorf("sync: failed to watch repo path: %w", err)
	}

	// Watch subdirectories recursively
	if err := s.watchRecursive(s.config.RepoPath); err != nil {
		return fmt.Errorf("sync: failed to watch recursively: %w", err)
	}

	s.running = true

	// Start the event loop
	go s.eventLoop()

	log.Printf("sync: started watching %s", s.config.RepoPath)
	return nil
}

// Stop stops the file watching service
func (s *SyncService) Stop() {
	if !s.running {
		return
	}

	s.cancel()
	s.running = false

	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()

	// Cancel pending debounces
	s.debouncer.CancelAll()

	// Stop advanced debouncer if used
	if s.advancedDebouncer != nil {
		s.advancedDebouncer.Stop()
	}

	// Close watcher
	if s.watcher != nil {
		if err := s.watcher.Close(); err != nil {
			log.Printf("sync: error closing watcher: %v", err)
		}
	}

	log.Println("sync: stopped")
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
	log.Println("sync: performing manual sync")

	if s.onSyncStart != nil {
		s.onSyncStart()
	}

	// Use the git manager to stage, commit, and push changes
	changedFiles, err := s.gitManager.StageCommitAndPush(s.ctx, time.Now())

	if s.onSyncComplete != nil {
		s.onSyncComplete(changedFiles, err)
	}

	if err != nil {
		log.Printf("sync: manual sync failed: %v", err)
		if s.onError != nil {
			s.onError(fmt.Errorf("sync: manual sync failed: %w", err))
		}
		return
	}

	if len(changedFiles) > 0 {
		log.Printf("sync: manually synced %d files: %v", len(changedFiles), changedFiles)
	} else {
		log.Println("sync: no changes to sync")
	}
}

// ManualSync triggers an immediate sync without debouncing
func (s *SyncService) ManualSync() error {
	// Check if service is stopped
	s.mu.RLock()
	if s.stopped {
		s.mu.RUnlock()
		return fmt.Errorf("sync service is stopped")
	}
	s.mu.RUnlock()

	log.Println("sync: performing manual sync")

	// If using advanced debouncer, use its manual sync feature
	if s.advancedDebouncer != nil {
		err := s.advancedDebouncer.TriggerManualSync("manual_sync", func() {
			s.performManualSync()
		})
		if err != nil {
			return fmt.Errorf("sync: manual sync failed: %w", err)
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
	return s.running
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
		"running":            s.running,
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
