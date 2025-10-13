package debouncer

import (
	"sync"
	"time"
)

// Debouncer delays execution of functions until a period of inactivity
type Debouncer struct {
	delay    time.Duration
	timers   map[string]*time.Timer
	mu       sync.RWMutex
	callback map[string]func()
}

// New creates a new debouncer with the specified delay
func New(delay time.Duration) *Debouncer {
	return &Debouncer{
		delay:    delay,
		timers:   make(map[string]*time.Timer),
		callback: make(map[string]func()),
	}
}

// Add adds a function to be debounced. If the same key is used multiple times,
// the previous function is cancelled and the new one will be executed after the delay.
func (d *Debouncer) Add(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel existing timer for this key if it exists
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
	}

	// Store the callback
	d.callback[key] = fn

	// Create new timer
	d.timers[key] = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		// Execute callback if it still exists
		if callback, exists := d.callback[key]; exists {
			callback()
		}

		// Clean up
		delete(d.timers, key)
		delete(d.callback, key)
	})
}

// Cancel cancels a pending debounced function for the given key
func (d *Debouncer) Cancel(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if timer, exists := d.timers[key]; exists {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}
}

// CancelAll cancels all pending debounced functions
func (d *Debouncer) CancelAll() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, timer := range d.timers {
		timer.Stop()
		delete(d.timers, key)
		delete(d.callback, key)
	}
}

// Pending returns the number of pending debounced functions
func (d *Debouncer) Pending() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.timers)
}

// SetDelay updates the debounce delay
func (d *Debouncer) SetDelay(delay time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.delay = delay
}

// GetDelay returns the current debounce delay
func (d *Debouncer) GetDelay() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.delay
}
