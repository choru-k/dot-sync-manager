package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/choru-k/dot-sync-manager/internal/ignore"
	"github.com/fsnotify/fsnotify"
)

// serviceState represents the current state of the SyncService
type serviceState int32

const (
	stateStopped serviceState = iota
	stateStarting
	stateRunning
	stateStopping
)

// String returns a human-readable representation of the state
func (s serviceState) String() string {
	switch s {
	case stateStopped:
		return "stopped"
	case stateStarting:
		return "starting"
	case stateRunning:
		return "running"
	case stateStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// SyncServiceSimplified is a simplified version of SyncService with unified state management
// and reduced synchronization complexity.
//
// Thread Safety:
// - All methods are thread-safe and can be called concurrently
// - Uses a single atomic state value for all state transitions
// - Context-based cancellation for clean shutdown
// - No dual-state management or complex coordination primitives
type SyncServiceSimplified struct {
	gitManager   *gitmanager.GitManager
	watcher      *fsnotify.Watcher
	debouncer    DebouncerInterface
	ignoreParser *ignore.Parser
	config       *Config

	// Advanced debouncer reference (if used)
	advancedDebouncer *debouncer.AdvancedDebouncer

	// Simplified state management - single atomic value
	state  atomic.Int32 // serviceState
	ctx    context.Context
	cancel context.CancelFunc

	// Event callbacks
	onSyncStart    func()
	onSyncComplete func(files []string, err error)
	onError        func(error)
}

// NewSimplified creates a new simplified sync service
func NewSimplified(gitManager *gitmanager.GitManager, config *Config) (*SyncServiceSimplified, error) {
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

	service := &SyncServiceSimplified{
		gitManager:        gitManager,
		watcher:           watcher,
		debouncer:         debouncerImpl,
		advancedDebouncer: advancedDebouncer,
		ignoreParser:      ignoreParser,
		config:            config,
	}

	// Initialize state as stopped
	service.state.Store(int32(stateStopped))

	// Load ignore patterns
	ignoreFilePath := filepath.Join(config.RepoPath, config.IgnoreFile)
	if err := service.ignoreParser.LoadFromFile(ignoreFilePath); err != nil {
		log.Printf("sync: warning - failed to load ignore file: %v", err)
	}

	return service, nil
}

// Start begins watching for file changes with simplified state management
func (s *SyncServiceSimplified) Start() error {
	// Atomic state transition: stopped -> starting
	if !s.state.CompareAndSwap(int32(stateStopped), int32(stateStarting)) {
		currentState := serviceState(s.state.Load())
		return fmt.Errorf("sync: service already %s", currentState)
	}

	// Add the repository path to the watcher
	if err := s.watcher.Add(s.config.RepoPath); err != nil {
		s.state.Store(int32(stateStopped))
		return fmt.Errorf("sync: failed to watch repo path: %w", err)
	}

	// Watch subdirectories recursively
	if err := s.watchRecursive(s.config.RepoPath); err != nil {
		s.state.Store(int32(stateStopped))
		return fmt.Errorf("sync: failed to watch recursively: %w", err)
	}

	// Create context for cancellation
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Transition to running state
	s.state.Store(int32(stateRunning))

	// Start the event loop
	go s.eventLoop()

	log.Printf("sync: started watching %s", s.config.RepoPath)
	return nil
}

// Stop stops the file watching service with simplified shutdown
func (s *SyncServiceSimplified) Stop() {
	// Atomic state transition: running -> stopping
	if !s.state.CompareAndSwap(int32(stateRunning), int32(stateStopping)) {
		// Already stopped or stopping
		return
	}

	log.Println("sync: stopping service...")

	// Cancel all operations
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
			log.Printf("sync: error closing watcher: %v", err)
		}
	}

	// Transition to stopped state
	s.state.Store(int32(stateStopped))

	log.Println("sync: stopped")
}

// IsRunning returns whether the service is currently running
func (s *SyncServiceSimplified) IsRunning() bool {
	return serviceState(s.state.Load()) == stateRunning
}

// GetState returns the current state of the service
func (s *SyncServiceSimplified) GetState() serviceState {
	return serviceState(s.state.Load())
}

// ManualSync triggers an immediate sync without debouncing
func (s *SyncServiceSimplified) ManualSync() error {
	// Check if service is running
	if !s.IsRunning() {
		return fmt.Errorf("sync service is %s", s.GetState())
	}

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

// eventLoop handles file system events with context-aware shutdown
func (s *SyncServiceSimplified) eventLoop() {
	for {
		select {
		case <-s.ctx.Done():
			// Context cancelled, exit gracefully
			return

		case event, ok := <-s.watcher.Events:
			if !ok {
				// Watcher closed, exit
				return
			}

			// Process event
			s.handleEvent(event)

		case err, ok := <-s.watcher.Errors:
			if !ok {
				// Error channel closed, exit
				return
			}

			// Handle error
			if s.onError != nil {
				s.onError(fmt.Errorf("sync: watcher error: %w", err))
			}
		}
	}
}

// watchRecursive adds all subdirectories to the watcher
func (s *SyncServiceSimplified) watchRecursive(root string) error {
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

// handleEvent processes a single file system event
func (s *SyncServiceSimplified) handleEvent(event fsnotify.Event) {
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
func (s *SyncServiceSimplified) performSync() {
	if !s.config.AutoSyncEnabled {
		return
	}

	s.performManualSync()
}

// performManualSync executes sync operation regardless of AutoSyncEnabled setting
func (s *SyncServiceSimplified) performManualSync() {
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

// SetEventCallbacks sets the event callback functions
func (s *SyncServiceSimplified) SetEventCallbacks(onSyncStart func(), onSyncComplete func([]string, error), onError func(error)) {
	s.onSyncStart = onSyncStart
	s.onSyncComplete = onSyncComplete
	s.onError = onError
}

// GetConfig returns the current configuration
func (s *SyncServiceSimplified) GetConfig() *Config {
	return s.config
}

// ReloadIgnorePatterns reloads the ignore patterns from file
func (s *SyncServiceSimplified) ReloadIgnorePatterns() error {
	ignoreFilePath := filepath.Join(s.config.RepoPath, s.config.IgnoreFile)
	s.ignoreParser.Clear()

	if err := s.ignoreParser.LoadFromFile(ignoreFilePath); err != nil {
		return fmt.Errorf("sync: failed to reload ignore patterns: %w", err)
	}

	log.Println("sync: reloaded ignore patterns")
	return nil
}

// GetStats returns statistics about the sync service
func (s *SyncServiceSimplified) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"state":              s.GetState().String(),
		"running":            s.IsRunning(),
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