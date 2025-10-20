package debouncer

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// AdvancedDebouncerSimplified is a simplified version with single mutex
// and direct manual sync execution
type AdvancedDebouncerSimplified struct {
	// Single mutex for all state
	mu sync.RWMutex

	// Configuration
	baseDelay      time.Duration
	maxDelay       time.Duration
	backoffMult    float64
	backoffEnabled bool

	// Churn detection
	churnThreshold     int
	churnWindow        time.Duration
	decayResetDuration time.Duration

	// State protected by mu
	timers          map[string]*time.Timer
	callback        map[string]func()
	activityHistory []time.Time
	backoffCount    int
	lastActivity    time.Time

	// Manual sync
	manualSyncTimeout time.Duration

	// Activity tracking
	maxActivityHistory int

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAdvancedSimplified creates a new simplified advanced debouncer
func NewAdvancedSimplified(config AdvancedDebouncerConfig) *AdvancedDebouncerSimplified {
	// Apply defaults
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

	ctx, cancel := context.WithCancel(context.Background())

	return &AdvancedDebouncerSimplified{
		baseDelay:          config.BaseDelay,
		currentDelay:       config.BaseDelay, // Will be set in calculateDelay
		maxDelay:           config.MaxDelay,
		backoffMult:        config.BackoffMultiplier,
		backoffEnabled:     config.BackoffEnabled,
		churnThreshold:     config.ChurnThreshold,
		churnWindow:        config.ChurnWindow,
		decayResetDuration: config.DecayResetDuration,
		timers:             make(map[string]*time.Timer),
		callback:           make(map[string]func()),
		manualSyncTimeout:  config.ManualSyncTimeout,
		maxActivityHistory: DefaultMaxActivityHistory,
		ctx:                ctx,
		cancel:             cancel,
		lastActivity:       time.Now(),
	}
}

// Add adds a function to be debounced with advanced backoff logic
func (d *AdvancedDebouncerSimplified) Add(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if stopped
	select {
	case <-d.ctx.Done():
		return
	default:
	}

	// Cancel existing timer for this key
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}

	// Record activity
	d.recordActivityLocked()

	// Calculate delay
	delay := d.calculateDelayLocked()

	// Store callback
	d.callback[key] = fn

	// Create timer with panic recovery
	d.timers[key] = time.AfterFunc(delay, func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		// Get and execute callback
		if callback, exists := d.callback[key]; exists {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Warning: panic in debounced callback for key %s: %v", key, r)
					}
				}()
				callback()
			}()

			// Clean up
			delete(d.timers, key)
			delete(d.callback, key)

			// Reset backoff on successful execution
			d.backoffCount = 0
		}
	})
}

// Cancel cancels a pending debounced function for the given key
func (d *AdvancedDebouncerSimplified) Cancel(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if timer, exists := d.timers[key]; exists {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}
}

// CancelAll cancels all pending debounced functions
func (d *AdvancedDebouncerSimplified) CancelAll() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, timer := range d.timers {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}
}

// Pending returns the number of pending debounced functions
func (d *AdvancedDebouncerSimplified) Pending() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.timers)
}

// TriggerManualSync executes a manual sync immediately with proper timeout
func (d *AdvancedDebouncerSimplified) TriggerManualSync(key string, fn func()) error {
	d.mu.Lock()

	// Check if stopped
	select {
	case <-d.ctx.Done():
		d.mu.Unlock()
		return fmt.Errorf("debouncer is stopped")
	default:
	}

	// Cancel any existing operation for this key
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}

	d.mu.Unlock()

	// Execute with timeout using context
	ctx, cancel := context.WithTimeout(d.ctx, d.manualSyncTimeout)
	defer cancel()

	done := make(chan struct{})
	var panicErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = fmt.Errorf("panic during manual sync: %v", r)
			}
			close(done)
		}()
		fn()
	}()

	select {
	case <-done:
		return panicErr
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("manual sync timeout after %v", d.manualSyncTimeout)
		}
		return fmt.Errorf("debouncer stopped")
	}
}

// Stop stops the debouncer and cleans up resources
func (d *AdvancedDebouncerSimplified) Stop() {
	d.cancel()
	d.CancelAll()
}

// recordActivityLocked records activity. Must be called with mu held
func (d *AdvancedDebouncerSimplified) recordActivityLocked() {
	now := time.Now()
	d.activityHistory = append(d.activityHistory, now)
	d.lastActivity = now

	// Clean old activity
	cutoff := now.Add(-d.churnWindow)
	for len(d.activityHistory) > 0 && d.activityHistory[0].Before(cutoff) {
		d.activityHistory = d.activityHistory[1:]
	}

	// Prevent unbounded growth
	if len(d.activityHistory) > d.maxActivityHistory {
		excess := len(d.activityHistory) - d.maxActivityHistory
		d.activityHistory = d.activityHistory[excess:]
	}
}

// calculateDelayLocked calculates the delay. Must be called with mu held
func (d *AdvancedDebouncerSimplified) calculateDelayLocked() time.Duration {
	// Check for backoff reset due to inactivity
	if time.Since(d.lastActivity) > d.decayResetDuration {
		d.backoffCount = 0
		return d.baseDelay
	}

	// Check for churn
	if len(d.activityHistory) >= d.churnThreshold && d.backoffEnabled && d.backoffMult > 1.0 {
		d.backoffCount++

		// Calculate exponential backoff
		backoffDelay := time.Duration(float64(d.baseDelay) * math.Pow(d.backoffMult, float64(d.backoffCount)))

		// Cap at max
		if backoffDelay > d.maxDelay {
			backoffDelay = d.maxDelay
		}

		return backoffDelay
	}

	// No churn, use base delay
	return d.baseDelay
}

// GetDelay returns the current delay
func (d *AdvancedDebouncerSimplified) GetDelay() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.calculateDelayLocked()
}

// GetStats returns statistics
func (d *AdvancedDebouncerSimplified) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"pending":            len(d.timers),
		"base_delay":         d.baseDelay.String(),
		"current_delay":      d.calculateDelayLocked().String(),
		"max_delay":          d.maxDelay.String(),
		"backoff_count":      d.backoffCount,
		"backoff_multiplier": d.backoffMult,
		"is_churn_mode":      len(d.activityHistory) >= d.churnThreshold,
		"activity_count":     len(d.activityHistory),
		"churn_threshold":    d.churnThreshold,
		"churn_window":       d.churnWindow.String(),
		"last_activity":      d.lastActivity.Format(time.RFC3339),
		"time_since_activity": time.Since(d.lastActivity).String(),
		"manual_sync_timeout": d.manualSyncTimeout.String(),
	}
}