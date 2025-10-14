package debouncer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAdvancedDebouncer_ManualSyncTimeout(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	// Set a very short timeout for testing
	debouncer.SetManualSyncTimeout(50 * time.Millisecond)

	var callCount int
	var mu sync.Mutex

	// Create a function that blocks longer than timeout
	blockingChan := make(chan struct{})
	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
		// Block until test completes (or timeout occurs)
		<-blockingChan
	}

	// Trigger manual sync in a goroutine
	resultChan := make(chan error, 1)
	go func() {
		err := debouncer.TriggerManualSync("test", fn)
		resultChan <- err
	}()

	// Wait for timeout
	select {
	case err := <-resultChan:
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
		if !contains(err.Error(), "timeout") {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Expected timeout within 200ms")
	}

	// Unblock the function to clean up
	close(blockingChan)
	time.Sleep(10 * time.Millisecond)

	// The function might have been called, but that's ok - the important thing
	// is that the timeout was properly detected and reported
}

func TestAdvancedDebouncer_ManualSyncQueueOverflow(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           5 * time.Minute,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     10,
		ChurnWindow:        1 * time.Minute,
		DecayResetDuration: 5 * time.Minute,
	}

	// Create debouncer with small queue size for testing
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Small delay to keep queue full
	}

	// Fill the queue beyond capacity (queue size is 100)
	for i := 0; i < 150; i++ {
		go func(id int) {
			err := debouncer.TriggerManualSync(fmt.Sprintf("test-%d", id), fn)
			// Some should succeed, some should be handled directly
			// We don't check errors here since the behavior is implementation-specific
			_ = err
		}(i)
	}

	// Wait for all operations to complete
	time.Sleep(500 * time.Millisecond)

	// Verify some operations were executed
	mu.Lock()
	if callCount == 0 {
		t.Error("Expected some operations to be executed")
	}
	mu.Unlock()
}

func TestAdvancedDebouncer_ActivityHistoryBounds(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	// Set a small max history for testing
	debouncer.SetMaxActivityHistory(10)

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Generate more activity than the max history
	for i := 0; i < 50; i++ {
		debouncer.Add("test", fn)
		time.Sleep(1 * time.Millisecond)
	}

	// Check that activity history is bounded
	stats := debouncer.GetStats()
	activityCount, ok := stats["activity_count"].(int)
	if !ok {
		t.Error("Expected activity_count to be an int")
	}

	if activityCount > 10 {
		t.Errorf("Expected activity count <= 10, got %d", activityCount)
	}
}

func TestAdvancedDebouncer_PanicRecovery(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	// Function that panics
	panicFn := func() {
		panic("test panic")
	}

	// This should not crash the debouncer
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Debouncer should have recovered from panic, but got: %v", r)
		}
	}()

	// AddImmediate should recover from panic
	debouncer.AddImmediate("test", panicFn)

	// Wait a bit for the panic to be handled
	time.Sleep(50 * time.Millisecond)

	// Debouncer should still be functional
	var callCount int
	var mu sync.Mutex

	normalFn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	debouncer.AddImmediate("test2", normalFn)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if callCount != 1 {
		t.Errorf("Expected 1 call after panic recovery, got %d", callCount)
	}
	mu.Unlock()
}

func TestAdvancedDebouncer_ConfigurableTimeout(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	// Test default timeout
	if debouncer.GetManualSyncTimeout() != 10*time.Second {
		t.Errorf("Expected default timeout 10s, got %v", debouncer.GetManualSyncTimeout())
	}

	// Test setting custom timeout
	customTimeout := 5 * time.Second
	debouncer.SetManualSyncTimeout(customTimeout)

	if debouncer.GetManualSyncTimeout() != customTimeout {
		t.Errorf("Expected custom timeout %v, got %v", customTimeout, debouncer.GetManualSyncTimeout())
	}

	// Verify timeout is reflected in stats
	stats := debouncer.GetStats()
	timeout, ok := stats["manual_sync_timeout"].(string)
	if !ok {
		t.Error("Expected manual_sync_timeout in stats")
	}

	if timeout != customTimeout.String() {
		t.Errorf("Expected timeout %s in stats, got %s", customTimeout.String(), timeout)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
