package conflict

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// conflictDirPerms is the permission for conflict directories (owner rwx, group/others rx).
const conflictDirPerms = 0755

// debounceDelay is the delay before triggering conflict check after file events.
// This prevents rapid-fire events from causing excessive checks.
const defaultDebounceDelay = 500 * time.Millisecond

// Detector watches the conflict directory for new conflicts.
// It runs in the background and triggers conflict checks when changes are detected.
type Detector struct {
	service   *Service
	watcher   *fsnotify.Watcher
	running   bool
	runningMu sync.Mutex
	stopCh    chan struct{}
	onCheck   func() // For testing - called when check is triggered
}

// NewDetector creates a new conflict detector.
func NewDetector(service *Service) *Detector {
	return &Detector{
		service: service,
	}
}

// SetOnCheck sets a callback function that is called when conflict check is triggered.
// This is primarily for testing purposes. Thread-safe.
func (d *Detector) SetOnCheck(fn func()) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	d.onCheck = fn
}

// Start begins watching the conflict directory for changes.
func (d *Detector) Start() error {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()

	// Idempotent - already running
	if d.running {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	d.watcher = watcher
	d.stopCh = make(chan struct{})
	d.running = true

	// Watch the conflict directory
	conflictDir := d.service.GetConflictDir()
	if err := watcher.Add(conflictDir); err != nil {
		// Directory might not exist yet, create it
		if mkErr := os.MkdirAll(conflictDir, conflictDirPerms); mkErr != nil {
			_ = d.watcher.Close() // Best effort cleanup
			d.running = false
			return mkErr
		}
		if err := watcher.Add(conflictDir); err != nil {
			_ = d.watcher.Close() // Best effort cleanup
			d.running = false
			return err
		}
	}

	go d.watch()
	return nil
}

// Stop stops the detector.
func (d *Detector) Stop() {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()

	if !d.running {
		return
	}

	close(d.stopCh)
	if d.watcher != nil {
		_ = d.watcher.Close() // Best effort cleanup
	}
	d.running = false
}

// watch is the main watch loop with debouncing.
func (d *Detector) watch() {
	// Debounce timer to avoid rapid-fire events
	var debounceTimer *time.Timer
	debounceDelay := defaultDebounceDelay

	for {
		select {
		case <-d.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-d.watcher.Events:
			if !ok {
				return
			}

			// Only care about creates and deletes in conflict dir
			if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Write) != 0 {
				// Debounce: reset timer on each event
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDelay, func() {
					if _, err := d.service.CheckForConflicts(); err != nil {
						fmt.Fprintf(os.Stderr, "conflict: check failed: %v\n", err)
					}
					// Thread-safe callback access
					d.runningMu.Lock()
					cb := d.onCheck
					d.runningMu.Unlock()
					if cb != nil {
						cb()
					}
				})
			}

		case err, ok := <-d.watcher.Errors:
			if !ok {
				return
			}
			// Continue watching despite fsnotify errors (e.g., temporary permission issues).
			// Errors are typically recoverable and shouldn't stop conflict detection.
			fmt.Fprintf(os.Stderr, "conflict: watcher error: %v\n", err)
		}
	}
}

// IsRunning returns whether the detector is currently running.
func (d *Detector) IsRunning() bool {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	return d.running
}
