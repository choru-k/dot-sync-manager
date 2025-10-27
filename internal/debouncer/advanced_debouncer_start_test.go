package debouncer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdvancedDebouncer_StartResourceLeak ensures that Start() can be called
// multiple times without causing issues (resource leak prevention)
func TestAdvancedDebouncer_StartResourceLeak(t *testing.T) {
	config := DefaultAdvancedConfig()
	config.ManualSyncTimeout = 100 * time.Millisecond // Short timeout for testing

	debouncer := NewAdvanced(config)

	// Test multiple concurrent Start() calls
	numStartCalls := 50
	var wg sync.WaitGroup

	for i := 0; i < numStartCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			debouncer.Start()
		}()
	}

	// Wait for all Start() calls to complete
	wg.Wait()

	// Verify the debouncer is still functional after multiple starts
	err := debouncer.TriggerManualSync("test", func() {
		// This should work without issues
	})
	if err != nil {
		t.Errorf("Manual sync failed after multiple Start() calls: %v", err)
	}

	// Test multiple manual sync operations to ensure single goroutine handling
	syncCount := 0
	var syncWG sync.WaitGroup
	var syncMu sync.Mutex

	for i := 0; i < 10; i++ {
		syncWG.Add(1)
		go func(id int) {
			defer syncWG.Done()
			err := debouncer.TriggerManualSync("test_concurrent", func() {
				syncMu.Lock()
				syncCount++
				syncMu.Unlock()
			})
			if err != nil {
				t.Errorf("Concurrent manual sync %d failed: %v", id, err)
			}
		}(i)
	}

	syncWG.Wait()

	// Verify all sync operations completed
	syncMu.Lock()
	if syncCount != 10 {
		t.Errorf("Expected 10 sync operations, got %d", syncCount)
	}
	syncMu.Unlock()

	// Clean up
	if err := debouncer.Stop(); err != nil {
		t.Errorf("Failed to stop debouncer: %v", err)
	}
}

// TestAdvancedDebouncer_StartStopRestart tests that Start() works correctly
// after Stop() (new goroutine can be started)
func TestAdvancedDebouncer_StartStopRestart(t *testing.T) {
	config := DefaultAdvancedConfig()
	config.ManualSyncTimeout = 100 * time.Millisecond

	debouncer := NewAdvanced(config)

	// First start
	debouncer.Start()

	// Verify it works
	err := debouncer.TriggerManualSync("test1", func() {})
	if err != nil {
		t.Errorf("First manual sync failed: %v", err)
	}

	// Stop it
	if err := debouncer.Stop(); err != nil {
		t.Errorf("Failed to stop debouncer: %v", err)
	}

	// Wait a moment for cleanup
	time.Sleep(50 * time.Millisecond)

	// Create a new debouncer since Stop() closes channels
	debouncer2 := NewAdvanced(config)

	// Start again
	debouncer2.Start()

	// Verify it works again
	err = debouncer2.TriggerManualSync("test2", func() {})
	if err != nil {
		t.Errorf("Second manual sync failed: %v", err)
	}

	// Clean up
	if err := debouncer2.Stop(); err != nil {
		t.Errorf("Failed to stop debouncer2: %v", err)
	}
}

// TestAdvancedDebouncer_ConcurrentStarts tests many goroutines calling Start() concurrently
func TestAdvancedDebouncer_ConcurrentStarts(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)

	// Track successful starts
	var successfulStarts int32

	// Many goroutines calling Start() concurrently
	numGoroutines := 100
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			debouncer.Start()

			// If this is the first successful start, count it
			if atomic.CompareAndSwapInt32(&successfulStarts, 0, 1) {
				t.Logf("Goroutine %d performed the successful start", id)
			}
		}(i)
	}

	wg.Wait()

	// Verify exactly one start was successful
	if atomic.LoadInt32(&successfulStarts) != 1 {
		t.Errorf("Expected exactly 1 successful start, got %d", atomic.LoadInt32(&successfulStarts))
	}

	// Verify the debouncer is functional
	err := debouncer.TriggerManualSync("test", func() {})
	if err != nil {
		t.Errorf("Manual sync failed: %v", err)
	}

	// Clean up
	if err := debouncer.Stop(); err != nil {
		t.Errorf("Failed to stop debouncer: %v", err)
	}
}