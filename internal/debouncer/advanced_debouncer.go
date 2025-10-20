package debouncer

import (
	"context"
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
)

// AdvancedDebouncer provides configurable debounce with exponential backoff
// and rapid file churn detection
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
	done chan struct{}

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Start/stop state management
	started int32 // atomic.Bool replacement: 1 = started, 0 = not started
	startMu sync.Mutex
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

	// Initialize context for cancellation
	debouncer.ctx, debouncer.cancel = context.WithCancel(context.Background())

	return debouncer
}

// Add adds a function to be debounced with advanced backoff logic
func (d *AdvancedDebouncer) Add(key string, fn func()) {
	d.AddWithContext(context.Background(), key, fn)
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
		// Check if context is cancelled before proceeding
		select {
		case <-capturedCtx.Done():
			// Context cancelled, don't execute callback
			d.mu.Lock()
			delete(d.timers, key)
			delete(d.callback, key)
			d.mu.Unlock()
			return
		case <-d.ctx.Done():
			// Debouncer context cancelled, don't execute callback
			d.mu.Lock()
			delete(d.timers, key)
			delete(d.callback, key)
			d.mu.Unlock()
			return
		default:
			// Both contexts are still valid, proceed
		}

		d.mu.Lock()
		defer d.mu.Unlock()

		// Double-check context cancellation after acquiring lock
		select {
		case <-capturedCtx.Done():
			// Context cancelled, clean up and return
			delete(d.timers, key)
			delete(d.callback, key)
			return
		case <-d.ctx.Done():
			// Debouncer context cancelled, clean up and return
			delete(d.timers, key)
			delete(d.callback, key)
			return
		default:
			// Both contexts are still valid, proceed
		}

		// Execute the captured callback with panic recovery and context awareness
		defer func() {
			if r := recover(); r != nil {
				// Log panic if needed, but don't crash
				log.Printf("Warning: panic in debounced callback for key %s: %v", key, r)
			}
		}()

		// Execute callback in a separate goroutine to allow context cancellation
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Warning: panic in debounced callback goroutine for key %s: %v", key, r)
				}
			}()
			capturedFn()
		}()

		// Wait for callback completion or context cancellation
		select {
		case <-done:
			// Callback completed successfully
		case <-capturedCtx.Done():
			// Context cancelled during execution
			log.Printf("Callback for key %s cancelled by context", key)
			// Note: We can't interrupt the running goroutine, but we can clean up
		case <-d.ctx.Done():
			// Debouncer context cancelled during execution
			log.Printf("Callback for key %s cancelled by debouncer context", key)
			// Note: We can't interrupt the running goroutine, but we can clean up
		}

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
				// Log panic if needed, but don't crash
				log.Printf("Warning: panic in AddImmediate for key %s: %v", key, r)
			}
		}()
		fn()
	}()
}

// TriggerManualSync executes a manual sync with proper queue handling
func (d *AdvancedDebouncer) TriggerManualSync(key string, fn func()) error {
	result := make(chan error, 1)

	request := manualSyncRequest{
		fn:        fn,
		key:       key,
		immediate: true,
		result:    result,
	}

	// Send request to manual sync queue
	select {
	case d.manualSyncQueue <- request:
		// Request queued successfully
	default:
		// Queue is full, execute directly
		go d.handleManualSync(request)
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
			if request.result != nil {
				request.result <- fmt.Errorf("panic during manual sync: %v", r)
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

// GetActivityCount returns the number of activities in the current window
func (d *AdvancedDebouncer) GetActivityCount() int {
	d.activityMu.RLock()
	defer d.activityMu.RUnlock()
	return len(d.activityHistory)
}

// TriggerManualSyncWithContext executes a manual sync with context support for cancellation
func (d *AdvancedDebouncer) TriggerManualSyncWithContext(ctx context.Context, key string, fn func()) error {
	result := make(chan error, 1)

	request := manualSyncRequest{
		fn:        fn,
		key:       key,
		immediate: true,
		result:    result,
	}

	// Send request to manual sync queue
	select {
	case d.manualSyncQueue <- request:
		// Request queued successfully
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full, execute directly
		go d.handleManualSync(request)
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
func (d *AdvancedDebouncer) Stop() {
	d.CancelAll()

	// Cancel the context to stop any pending callbacks
	d.cancel()

	// Reset started flag to allow restart if needed
	atomic.StoreInt32(&d.started, 0)

	// Signal shutdown to queue processor
	close(d.done)

	// Close the queue to unblock any remaining operations
	close(d.manualSyncQueue)

	// Give a brief moment for clean shutdown
	time.Sleep(DefaultShutdownTimeout)

	// The actual goroutine should exit quickly due to the done signal and queue closure
	// Any remaining operations will naturally complete or timeout
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

	// Mark as started before launching goroutine
	atomic.StoreInt32(&d.started, 1)

	// Start the goroutine
	go d.processManualSyncQueue()
}
