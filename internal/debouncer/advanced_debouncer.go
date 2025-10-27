package debouncer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultManualSyncQueueSize is the default buffer size for the manual sync queue
	DefaultManualSyncQueueSize = 100

	// DefaultMaxActivityHistory is the default maximum number of activity entries to track
	DefaultMaxActivityHistory = 1000

	// DefaultShutdownTimeout is the default timeout for graceful shutdown
	DefaultShutdownTimeout = 100 * time.Millisecond

	// DebouncerTimeout is the maximum time to wait for debouncer operations
	DebouncerTimeout = 10 * time.Minute

	// Error IDs for monitoring and debugging - local to debouncer package
	// These IDs help track specific error types in monitoring systems like Sentry
	ErrIDDebouncerCallbackPanic     = "DEBOUNCER_CALLBACK_PANIC"
	ErrIDDebouncerTimerCancelFailed = "DEBOUNCER_TIMER_CANCEL_FAILED"
)

// AdvancedDebouncer provides configurable debounce with exponential backoff
// and rapid file churn detection
//
// Thread Safety:
// - All public methods are thread-safe and can be called concurrently
// - Stop() is idempotent and can be called multiple times safely
// - Uses sync.Once to ensure cleanup operations happen exactly once
// - Manual sync operations are safely rejected after Stop()
// - The debouncer can be safely used from multiple goroutines
type AdvancedDebouncer struct {
	// Basic debounce settings
	baseDelay      time.Duration
	currentDelay   time.Duration
	maxDelay       time.Duration
	backoffMult    float64
	backoffEnabled bool

	// Churn detection settings
	churnThreshold     int           // Number of changes to trigger churn mode
	churnWindow        time.Duration // Time window to detect churn
	decayResetDuration time.Duration // Time to return to normal delay

	// State
	timers   map[string]*time.Timer
	mu       sync.RWMutex
	callback map[string]func()

	// Activity tracking for churn detection
	activityHistory    []time.Time
	activityMu         sync.RWMutex
	maxActivityHistory int

	// Backoff state
	backoffCount     int
	lastActivityTime time.Time

	// Manual sync handling
	manualSyncQueue chan manualSyncRequest
	manualSyncMu    sync.Mutex

	// Manual sync timeout
	manualSyncTimeout time.Duration

	// Shutdown handling
	done    chan struct{}
	stopped bool // Track stopped state to prevent race conditions

	// Goroutine coordination for graceful shutdown
	shutdownWG sync.WaitGroup

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Error handling
	onError func(error)

	// Start/stop state management
	started int32 // atomic.Bool replacement: 1 = started, 0 = not started
	startMu sync.Mutex
	stopOnce sync.Once // Ensures Stop() is only called once
}

type manualSyncRequest struct {
	fn        func()
	key       string
	immediate bool
	result    chan error
}

// AdvancedDebouncerConfig holds configuration for the advanced debouncer
type AdvancedDebouncerConfig struct {
	// Basic debounce settings
	BaseDelay time.Duration `json:"base_delay"`
	MaxDelay  time.Duration `json:"max_delay"`

	// Backoff settings
	BackoffEnabled    bool    `json:"backoff_enabled"`
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// Churn detection settings
	ChurnThreshold     int           `json:"churn_threshold"`
	ChurnWindow        time.Duration `json:"churn_window"`
	DecayResetDuration time.Duration `json:"decay_reset_duration"`

	// Manual sync settings
	ManualSyncTimeout time.Duration `json:"manual_sync_timeout"`
}

// DefaultAdvancedConfig returns a default configuration for the advanced debouncer
func DefaultAdvancedConfig() AdvancedDebouncerConfig {
	return AdvancedDebouncerConfig{
		BaseDelay:          30 * time.Second,
		MaxDelay:           5 * time.Minute,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     10,
		ChurnWindow:        1 * time.Minute,
		DecayResetDuration: 5 * time.Minute,
		ManualSyncTimeout:  10 * time.Second,
	}
}

// NewAdvanced creates a new advanced debouncer with the specified configuration
func NewAdvanced(config AdvancedDebouncerConfig) *AdvancedDebouncer {
	if config.BaseDelay <= 0 {
		config.BaseDelay = DefaultAdvancedConfig().BaseDelay
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = DefaultAdvancedConfig().MaxDelay
	}
	if config.BackoffMultiplier <= 1.0 {
		config.BackoffMultiplier = DefaultAdvancedConfig().BackoffMultiplier
	}
	if config.ChurnThreshold <= 0 {
		config.ChurnThreshold = DefaultAdvancedConfig().ChurnThreshold
	}
	if config.ChurnWindow <= 0 {
		config.ChurnWindow = DefaultAdvancedConfig().ChurnWindow
	}
	if config.DecayResetDuration <= 0 {
		config.DecayResetDuration = DefaultAdvancedConfig().DecayResetDuration
	}
	if config.ManualSyncTimeout <= 0 {
		config.ManualSyncTimeout = DefaultAdvancedConfig().ManualSyncTimeout
	}

	debouncer := &AdvancedDebouncer{
		baseDelay:          config.BaseDelay,
		currentDelay:       config.BaseDelay,
		maxDelay:           config.MaxDelay,
		backoffMult:        config.BackoffMultiplier,
		backoffEnabled:     config.BackoffEnabled,
		churnThreshold:     config.ChurnThreshold,
		churnWindow:        config.ChurnWindow,
		decayResetDuration: config.DecayResetDuration,
		timers:             make(map[string]*time.Timer),
		callback:           make(map[string]func()),
		manualSyncQueue:    make(chan manualSyncRequest, DefaultManualSyncQueueSize),
		activityHistory:    make([]time.Time, 0),
		lastActivityTime:   time.Now(),
		manualSyncTimeout:  config.ManualSyncTimeout,
		maxActivityHistory: DefaultMaxActivityHistory, // Cap to prevent unbounded growth
		done:               make(chan struct{}),
		started:            0, // Initialize as not started
	}

	// Initialize context for cancellation without timeout
	ctx, cancel := context.WithCancel(context.Background())
	debouncer.ctx, debouncer.cancel = ctx, cancel

	return debouncer
}

// Add adds a function to be debounced with advanced backoff logic
func (d *AdvancedDebouncer) Add(key string, fn func()) {
	// Use the debouncer's context for operations without creating a new one
	d.AddWithContext(d.ctx, key, fn)
}

// AddWithContext adds a function to be debounced with advanced backoff logic and context support
func (d *AdvancedDebouncer) AddWithContext(ctx context.Context, key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel existing timer for this key if it exists
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}

	// Record activity for churn detection
	d.recordActivity()

	// Calculate current delay with backoff
	delay := d.calculateDelay()

	// Store the callback
	d.callback[key] = fn

	// Capture the callback and context to avoid race conditions
	capturedFn := fn
	capturedCtx := ctx

	// Create new timer with calculated delay
	d.timers[key] = time.AfterFunc(delay, func() {
		// First, check if the operation context is cancelled
		if capturedCtx.Err() != nil {
			// Context cancelled, don't execute callback
			d.mu.Lock()
			delete(d.timers, key)
			delete(d.callback, key)
			d.mu.Unlock()
			return
		}

		// Check if debouncer context is cancelled
		if d.ctx.Err() != nil {
			// Debouncer context cancelled, don't execute callback
			d.mu.Lock()
			delete(d.timers, key)
			delete(d.callback, key)
			d.mu.Unlock()
			return
		}

		d.mu.Lock()
		defer d.mu.Unlock()

		// Double-check context cancellation after acquiring lock
		if capturedCtx.Err() != nil {
			// Context cancelled, clean up and return
			delete(d.timers, key)
			delete(d.callback, key)
			return
		}

		if d.ctx.Err() != nil {
			// Debouncer context cancelled, clean up and return
			delete(d.timers, key)
			delete(d.callback, key)
			return
		}

		// Execute the captured callback with panic recovery
		defer func() {
			if r := recover(); r != nil {
				// Create proper error from panic
				err := fmt.Errorf("panic in debounced callback for key %s: %v [%s]", key, r, ErrIDDebouncerCallbackPanic)
				log.Printf("ERROR: %v", err)

				// Propagate error through error callback if available
				if d.onError != nil {
					d.onError(err)
				}
			}
		}()

		// Execute callback
		capturedFn()

		// Clean up
		delete(d.timers, key)
		delete(d.callback, key)

		// Reset backoff count after successful execution
		d.backoffCount = 0
		d.currentDelay = d.baseDelay
	})
}

// AddImmediate executes a function immediately, bypassing debounce
// Useful for manual sync triggers
func (d *AdvancedDebouncer) AddImmediate(key string, fn func()) {
	d.manualSyncMu.Lock()
	defer d.manualSyncMu.Unlock()

	// Cancel any existing debounced operation for this key
	d.Cancel(key)

	// Execute immediately in a separate goroutine to avoid blocking
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Create proper error from panic
				err := fmt.Errorf("panic in AddImmediate for key %s: %v [%s]", key, r, "DEBOUNCER_IMMEDIATE_PANIC")
				log.Printf("ERROR: %v", err)

				// Propagate error through error callback if available
				if d.onError != nil {
					d.onError(err)
				}
			}
		}()
		fn()
	}()
}

// triggerManualSyncInternal implements the core logic for manual sync operations
// with simplified locking pattern and optional context support.
// Returns whether the request was queued to the channel or handled directly.
func (d *AdvancedDebouncer) triggerManualSyncInternal(ctx context.Context, key string, fn func(), result chan error) (bool, error) {
	request := manualSyncRequest{
		fn:        fn,
		key:       key,
		immediate: true,
		result:    result,
	}

	// Critical fix: Hold lock throughout entire check+send operation to eliminate TOCTOU race
	// This prevents "send on closed channel" panic if Stop() happens between check and send
	d.manualSyncMu.Lock()
	defer d.manualSyncMu.Unlock()

	if d.stopped {
		return false, fmt.Errorf("debouncer is stopped")
	}

	// Try to send to queue. The select has a default so it won't block.
	var sent bool
	select {
	case d.manualSyncQueue <- request:
		sent = true
		// Request queued successfully
	case <-ctx.Done():
		// Context cancelled during send attempt
		return false, ctx.Err()
	default:
		// Queue is full, execute directly
		sent = false
	}

	return sent, nil
}

// TriggerManualSync executes a manual sync with proper queue handling
func (d *AdvancedDebouncer) TriggerManualSync(key string, fn func()) error {
	result := make(chan error, 1)

	sent, err := d.triggerManualSyncInternal(context.Background(), key, fn, result)
	if err != nil {
		return err
	}

	if !sent {
		// Execute directly since queue was full
		go d.handleManualSync(manualSyncRequest{
			fn:        fn,
			key:       key,
			immediate: true,
			result:    result,
		})
	}

	// Wait for result or timeout
	select {
	case err := <-result:
		return err
	case <-time.After(d.manualSyncTimeout):
		return fmt.Errorf("manual sync timeout after %v", d.manualSyncTimeout)
	}
}

// processManualSyncQueue processes manual sync requests in a separate goroutine
func (d *AdvancedDebouncer) processManualSyncQueue() {
	for {
		select {
		case <-d.done:
			// Shutdown signal received, exit gracefully
			return
		case request, ok := <-d.manualSyncQueue:
			if !ok {
				// Queue closed, exit gracefully
				return
			}
			d.handleManualSync(request)
		}
	}
}

// handleManualSync processes a single manual sync request
func (d *AdvancedDebouncer) handleManualSync(request manualSyncRequest) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic during manual sync for key %s: %v [%s]", request.key, r, "DEBOUNCER_MANUAL_SYNC_PANIC")
			log.Printf("ERROR: %v", err)

			// Propagate error through result channel
			if request.result != nil {
				request.result <- err
			}

			// Propagate error through error callback if available
			if d.onError != nil {
				d.onError(err)
			}
		}
	}()

	// Cancel any existing debounced operation for this key
	d.Cancel(request.key)

	// Execute the function
	if request.immediate {
		// Execute immediately
		request.fn()
		if request.result != nil {
			request.result <- nil
		}
	} else {
		// Execute with normal debounce logic
		d.Add(request.key, request.fn)
		if request.result != nil {
			request.result <- nil
		}
	}
}

// recordActivity records a timestamp for churn detection
func (d *AdvancedDebouncer) recordActivity() {
	d.activityMu.Lock()
	defer d.activityMu.Unlock()

	now := time.Now()
	d.activityHistory = append(d.activityHistory, now)
	d.lastActivityTime = now

	// Clean old activity history outside the churn window
	cutoff := now.Add(-d.churnWindow)
	for len(d.activityHistory) > 0 && d.activityHistory[0].Before(cutoff) {
		d.activityHistory = d.activityHistory[1:]
	}

	// Prevent unbounded growth by capping the history size
	if len(d.activityHistory) > d.maxActivityHistory {
		// Remove oldest entries to maintain the cap
		excess := len(d.activityHistory) - d.maxActivityHistory
		d.activityHistory = d.activityHistory[excess:]
	}
}

// isChurnDetected checks if rapid file churn is detected
func (d *AdvancedDebouncer) isChurnDetected() bool {
	d.activityMu.RLock()
	defer d.activityMu.RUnlock()

	return len(d.activityHistory) >= d.churnThreshold
}

// calculateDelay calculates the appropriate delay based on current conditions
// NOTE: This method must be called while holding d.mu lock to ensure thread safety
func (d *AdvancedDebouncer) calculateDelay() time.Duration {
	// Check if we need to reset backoff due to inactivity
	if time.Since(d.lastActivityTime) > d.decayResetDuration {
		d.backoffCount = 0
		d.currentDelay = d.baseDelay
		return d.baseDelay
	}

	// Check for churn detection with minimal lock exposure
	// Read activity data before acquiring the main lock to avoid nested locking
	var activityCount int
	func() {
		d.activityMu.RLock()
		defer d.activityMu.RUnlock()
		activityCount = len(d.activityHistory)
	}()

	// If churn is detected and backoff is enabled, apply exponential backoff
	if activityCount >= d.churnThreshold && d.backoffEnabled && d.backoffMult > 1.0 {
		d.backoffCount++

		// Calculate exponential backoff delay
		backoffDelay := time.Duration(float64(d.baseDelay) *
			math.Pow(d.backoffMult, float64(d.backoffCount)))

		// Cap at max delay
		if backoffDelay > d.maxDelay {
			backoffDelay = d.maxDelay
		}

		d.currentDelay = backoffDelay
		return backoffDelay
	}

	// No churn detected, use base delay
	d.currentDelay = d.baseDelay
	return d.baseDelay
}

// Cancel cancels a pending debounced function for the given key
func (d *AdvancedDebouncer) Cancel(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if timer, exists := d.timers[key]; exists {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}
}

// CancelAll cancels all pending debounced functions
func (d *AdvancedDebouncer) CancelAll() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, timer := range d.timers {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}
}

// cancelAllTimers cancels all pending timers and returns any errors encountered
func (d *AdvancedDebouncer) cancelAllTimers() []error {
	var errors []error

	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel all timers and collect any errors
	for key, timer := range d.timers {
		if !timer.Stop() {
			// Timer already fired or expired, this is not an error per se
			// but we'll log it for debugging purposes
			log.Printf("Warning: timer for key %s was not stopped (may have already fired) [%s]", key, "DEBOUNCER_TIMER_NOT_STOPPED")
		}
		delete(d.timers, key)
	}

	// Clear all callbacks
	for key := range d.callback {
		delete(d.callback, key)
	}

	return errors
}

// Pending returns the number of pending debounced functions
func (d *AdvancedDebouncer) Pending() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.timers)
}

// GetDelay returns the current effective delay
func (d *AdvancedDebouncer) GetDelay() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.currentDelay
}

// GetBaseDelay returns the base delay
func (d *AdvancedDebouncer) GetBaseDelay() time.Duration {
	return d.baseDelay
}

// GetMaxDelay returns the maximum delay
func (d *AdvancedDebouncer) GetMaxDelay() time.Duration {
	return d.maxDelay
}

// GetBackoffCount returns the current backoff count
func (d *AdvancedDebouncer) GetBackoffCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.backoffCount
}

// IsChurnMode returns true if churn is currently detected
func (d *AdvancedDebouncer) IsChurnMode() bool {
	return d.isChurnDetected()
}

// IsBackoffEnabled returns true if exponential backoff is enabled
func (d *AdvancedDebouncer) IsBackoffEnabled() bool {
	return d.backoffEnabled
}

// GetActivityCount returns the number of activities in the current window
func (d *AdvancedDebouncer) GetActivityCount() int {
	d.activityMu.RLock()
	defer d.activityMu.RUnlock()
	return len(d.activityHistory)
}

// TriggerManualSyncWithContext executes a manual sync with context support for cancellation
func (d *AdvancedDebouncer) TriggerManualSyncWithContext(ctx context.Context, key string, fn func()) error {
	result := make(chan error, 1)

	sent, err := d.triggerManualSyncInternal(ctx, key, fn, result)
	if err != nil {
		return err
	}

	if !sent {
		// Execute directly since queue was full or context cancelled
		go d.handleManualSync(manualSyncRequest{
			fn:        fn,
			key:       key,
			immediate: true,
			result:    result,
		})
	}

	// Wait for result, timeout, or context cancellation
	select {
	case err := <-result:
		return err
	case <-time.After(d.manualSyncTimeout):
		return fmt.Errorf("manual sync timeout after %v", d.manualSyncTimeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SetManualSyncTimeout sets the timeout for manual sync operations
func (d *AdvancedDebouncer) SetManualSyncTimeout(timeout time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.manualSyncTimeout = timeout
}

// SetMaxActivityHistory sets the maximum number of activity entries to track
func (d *AdvancedDebouncer) SetMaxActivityHistory(max int) {
	d.activityMu.Lock()
	defer d.activityMu.Unlock()
	d.maxActivityHistory = max

	// Trim existing history if needed
	if len(d.activityHistory) > max {
		excess := len(d.activityHistory) - max
		d.activityHistory = d.activityHistory[excess:]
	}
}

// GetManualSyncTimeout returns the current manual sync timeout
func (d *AdvancedDebouncer) GetManualSyncTimeout() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.manualSyncTimeout
}

// GetStats returns detailed statistics about the debouncer state
func (d *AdvancedDebouncer) GetStats() map[string]interface{} {
	d.mu.RLock()
	d.activityMu.RLock()
	defer d.mu.RUnlock()
	defer d.activityMu.RUnlock()

	return map[string]interface{}{
		"pending":             len(d.timers),
		"base_delay":          d.baseDelay.String(),
		"current_delay":       d.currentDelay.String(),
		"max_delay":           d.maxDelay.String(),
		"backoff_count":       d.backoffCount,
		"backoff_multiplier":  d.backoffMult,
		"is_churn_mode":       d.isChurnDetected(),
		"activity_count":      len(d.activityHistory),
		"churn_threshold":     d.churnThreshold,
		"churn_window":        d.churnWindow.String(),
		"last_activity":       d.lastActivityTime.Format(time.RFC3339),
		"time_since_activity": time.Since(d.lastActivityTime).String(),
		"manual_sync_timeout": d.manualSyncTimeout.String(),
	}
}

// Stop stops the debouncer and cleans up resources with graceful shutdown
// Returns any error encountered during shutdown
func (d *AdvancedDebouncer) Stop() error {
	var shutdownErrors []error

	d.stopOnce.Do(func() {
		// Set stopped flag to prevent new operations
		d.manualSyncMu.Lock()
		d.stopped = true
		d.manualSyncMu.Unlock()

	// Cancel all pending timers and operations with error collection
		if errs := d.cancelAllTimers(); len(errs) > 0 {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("errors cancelling timers during shutdown [%s]: %w", ErrIDDebouncerTimerCancelFailed, errors.Join(errs...)))
		}

	// Cancel the context to stop any pending callbacks
	d.cancel()

	// Reset started flag to allow restart if needed
		atomic.StoreInt32(&d.started, 0)

	// Signal shutdown to queue processor
	safeCloseChannel(d.done, "done", &shutdownErrors)

	// Close the queue to unblock any remaining operations
	// This is safe because manualSyncQueue is protected by the stopped flag
	safeCloseManualSyncQueue(d.manualSyncQueue, "manual sync", &shutdownErrors)

	// Wait for the goroutine to finish processing
	// This ensures clean shutdown without relying on arbitrary timeouts
	d.shutdownWG.Wait()

	// The actual goroutine should exit quickly due to the done signal and queue closure
	// Any remaining operations will naturally complete or timeout
	})

	if len(shutdownErrors) > 0 {
		if len(shutdownErrors) == 1 {
			return shutdownErrors[0]
		}
		return fmt.Errorf("multiple errors during debouncer shutdown: %v", shutdownErrors)
	}

	return nil
}

// Start starts the manual sync queue processor.
//
// This method launches a goroutine to process manual sync requests.
// The goroutine starts immediately and runs asynchronously, so manual sync
// operations can be triggered right after calling Start().
//
// Note: The manual sync queue processor is designed to be robust and will
// handle requests even if called immediately after Start(). The queue has
// a buffer of 100 items to handle rapid successive requests.
//
// This method is thread-safe and will only start the goroutine once,
// even if called multiple times from different goroutines.
func (d *AdvancedDebouncer) Start() {
	// Use atomic check first for fast path (avoid lock if already started)
	if atomic.LoadInt32(&d.started) == 1 {
		return // Already started
	}

	// Use mutex to ensure only one goroutine is started
	d.startMu.Lock()
	defer d.startMu.Unlock()

	// Double-check with mutex protection
	if atomic.LoadInt32(&d.started) == 1 {
		return // Already started (another goroutine won the race)
	}

	// Reset stopped state to allow restart after Stop()
	d.manualSyncMu.Lock()
	if d.stopped {
		// Recreate channels that were closed during Stop()
		d.done = make(chan struct{})
		d.manualSyncQueue = make(chan manualSyncRequest, 100)
		d.stopped = false
		// Reset the sync.Once for shutdown
		d.stopOnce = sync.Once{}
	}
	d.manualSyncMu.Unlock()

	// Mark as started before launching goroutine
	atomic.StoreInt32(&d.started, 1)

	// Start the goroutine with WaitGroup coordination
	d.shutdownWG.Add(1)
	go func() {
		defer d.shutdownWG.Done()
		d.processManualSyncQueue()
	}()
}

// SetErrorHandler sets a callback function to handle errors that occur during debounced operations.
// This is useful for monitoring and logging purposes.
func (d *AdvancedDebouncer) SetErrorHandler(onError func(error)) {
	d.onError = onError
}

// safeCloseChannel safely closes a channel with panic recovery
func safeCloseChannel(ch chan struct{}, context string, shutdownErrors *[]error) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic during %s channel closure [%s]: %v", context, "DEBOUNCER_CHANNEL_PANIC", r)
			*shutdownErrors = append(*shutdownErrors, err)
			log.Printf("ERROR: %v", err)
		}
	}()
	close(ch)
}

// safeCloseManualSyncQueue safely closes the manual sync queue with panic recovery
func safeCloseManualSyncQueue(ch chan manualSyncRequest, context string, shutdownErrors *[]error) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic during %s queue closure [%s]: %v", context, "DEBOUNCER_QUEUE_PANIC", r)
			*shutdownErrors = append(*shutdownErrors, err)
			log.Printf("ERROR: %v", err)
		}
	}()
	close(ch)
}
