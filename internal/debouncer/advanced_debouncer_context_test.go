package debouncer

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestAdvancedDebouncer_ContextCancellation ensures that callbacks respect context cancellation
func TestAdvancedDebouncer_ContextCancellation(t *testing.T) {
	config := DefaultAdvancedConfig()
	config.BaseDelay = 50 * time.Millisecond // Short delay for testing

	debouncer := NewAdvanced(config)

	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	// Track callback execution
	var callbackExecuted bool
	var callbackMu sync.Mutex

	// Add a callback with the context that will be cancelled
	debouncer.AddWithContext(ctx, "test", func() {
		callbackMu.Lock()
		callbackExecuted = true
		callbackMu.Unlock()
	})

	// Wait for longer than the context timeout
	time.Sleep(100 * time.Millisecond)

	// Check if callback was executed (it shouldn't be due to context cancellation)
	callbackMu.Lock()
	executed := callbackExecuted
	callbackMu.Unlock()

	if executed {
		t.Error("Callback should not have been executed due to context cancellation")
	}

	// Clean up
	if err := debouncer.Stop(); err != nil {
		t.Errorf("Failed to stop debouncer: %v", err)
	}
}

// TestAdvancedDebouncer_ContextCancellationMidExecution tests cancellation during callback execution
func TestAdvancedDebouncer_ContextCancellationMidExecution(t *testing.T) {
	config := DefaultAdvancedConfig()
	config.BaseDelay = 10 * time.Millisecond // Very short delay

	debouncer := NewAdvanced(config)

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Track callback state
	var callbackStarted bool
	var callbackCompleted bool
	var callbackMu sync.Mutex

	// Add a long-running callback that can be cancelled
	debouncer.AddWithContext(ctx, "long_test", func() {
		callbackMu.Lock()
		callbackStarted = true
		callbackMu.Unlock()

		// Simulate long work that can be interrupted
		select {
		case <-time.After(200 * time.Millisecond):
			// This should not complete due to context cancellation
			callbackMu.Lock()
			callbackCompleted = true
			callbackMu.Unlock()
		case <-ctx.Done():
			// Context was cancelled, exit early
			return
		}
	})

	// Wait for callback to potentially start
	time.Sleep(20 * time.Millisecond)

	// Check if callback started
	callbackMu.Lock()
	started := callbackStarted
	callbackMu.Unlock()

	if !started {
		t.Error("Callback should have started before context cancellation")
	}

	// Wait for context to cancel and callback to finish
	time.Sleep(100 * time.Millisecond)

	// Check callback state
	callbackMu.Lock()
	completed := callbackCompleted
	callbackMu.Unlock()

	if completed {
		t.Error("Callback should not have completed due to context cancellation")
	}

	// Clean up
	if err := debouncer.Stop(); err != nil {
		t.Errorf("Failed to stop debouncer: %v", err)
	}
}

// TestAdvancedDebouncer_ContextNotCancelled ensures callbacks execute normally when context is not cancelled
func TestAdvancedDebouncer_ContextNotCancelled(t *testing.T) {
	config := DefaultAdvancedConfig()
	config.BaseDelay = 20 * time.Millisecond // Short delay for testing

	debouncer := NewAdvanced(config)

	// Create a context with long timeout (won't be cancelled)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Track callback execution
	var callbackExecuted bool
	var callbackMu sync.Mutex

	// Add a callback with the context
	debouncer.AddWithContext(ctx, "normal_test", func() {
		callbackMu.Lock()
		callbackExecuted = true
		callbackMu.Unlock()
	})

	// Wait for callback to execute
	time.Sleep(100 * time.Millisecond)

	// Check if callback was executed
	callbackMu.Lock()
	executed := callbackExecuted
	callbackMu.Unlock()

	if !executed {
		t.Error("Callback should have been executed when context is not cancelled")
	}

	// Clean up
	if err := debouncer.Stop(); err != nil {
		t.Errorf("Failed to stop debouncer: %v", err)
	}
}

// TestAdvancedDebouncer_DebouncerContextCancellation ensures that debouncer context cancels all callbacks
func TestAdvancedDebouncer_DebouncerContextCancellation(t *testing.T) {
	config := DefaultAdvancedConfig()
	config.BaseDelay = 100 * time.Millisecond // Longer delay

	debouncer := NewAdvanced(config)

	// Track multiple callback executions
	var callbackCount int
	var callbackMu sync.Mutex

	// Add multiple callbacks
	for i := 0; i < 5; i++ {
		debouncer.Add("test_"+string(rune('A'+i)), func() {
			callbackMu.Lock()
			callbackCount++
			callbackMu.Unlock()
		})
	}

	// Stop the debouncer immediately (should cancel all pending callbacks)
	go func() {
		time.Sleep(10 * time.Millisecond) // Let debouncer start
		if err := debouncer.Stop(); err != nil {
			t.Errorf("Failed to stop debouncer: %v", err)
		}
	}()

	// Wait for everything to settle
	time.Sleep(200 * time.Millisecond)

	// Check callback executions
	callbackMu.Lock()
	executed := callbackCount
	callbackMu.Unlock()

	if executed > 0 {
		t.Errorf("No callbacks should have executed due to debouncer context cancellation, got %d", executed)
	}
}

// TestAdvancedDebouncer_BackwardsCompatibility ensures the original Add method still works
func TestAdvancedDebouncer_BackwardsCompatibility(t *testing.T) {
	config := DefaultAdvancedConfig()
	config.BaseDelay = 20 * time.Millisecond

	debouncer := NewAdvanced(config)

	// Track callback execution
	var callbackExecuted bool
	var callbackMu sync.Mutex

	// Use the original Add method (should work with background context)
	debouncer.Add("legacy_test", func() {
		callbackMu.Lock()
		callbackExecuted = true
		callbackMu.Unlock()
	})

	// Wait for callback to execute
	time.Sleep(100 * time.Millisecond)

	// Check if callback was executed
	callbackMu.Lock()
	executed := callbackExecuted
	callbackMu.Unlock()

	if !executed {
		t.Error("Callback should have been executed with legacy Add method")
	}

	// Clean up
	if err := debouncer.Stop(); err != nil {
		t.Errorf("Failed to stop debouncer: %v", err)
	}
}