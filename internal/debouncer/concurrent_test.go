package debouncer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAdvancedDebouncer_ConcurrentManualSync(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           1 * time.Second,
		BackoffEnabled:     false,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     5,
		ChurnWindow:        200 * time.Millisecond,
		DecayResetDuration: 1 * time.Second,
		ManualSyncTimeout:  5 * time.Second,
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10
	syncsPerGoroutine := 5

	var successCount int64
	var mu sync.Mutex

	// Launch multiple goroutines doing manual syncs concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < syncsPerGoroutine; j++ {
				key := fmt.Sprintf("sync-%d-%d", id, j)

				err := debouncer.TriggerManualSync(key, func() {
					mu.Lock()
					successCount++
					mu.Unlock()
				})

				if err != nil {
					t.Errorf("Unexpected error in goroutine %d, sync %d: %v", id, j, err)
				}

				// Small delay between syncs
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	expectedCount := int64(numGoroutines * syncsPerGoroutine)
	if successCount != expectedCount {
		t.Errorf("Expected %d successful syncs, got %d", expectedCount, successCount)
	}
}

func TestAdvancedDebouncer_ConcurrentMixedOperations(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          100 * time.Millisecond,
		MaxDelay:           1 * time.Second,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     5,
		ChurnWindow:        200 * time.Millisecond,
		DecayResetDuration: 1 * time.Second,
		ManualSyncTimeout:  5 * time.Second,
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var wg sync.WaitGroup
	var debouncedCount int64
	var manualCount int64
	var mu sync.Mutex

	// Goroutine for debounced operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			key := fmt.Sprintf("debounced-%d", i)
			debouncer.Add(key, func() {
				mu.Lock()
				debouncedCount++
				mu.Unlock()
			})
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Goroutine for manual sync operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("manual-%d", i)
			_ = debouncer.TriggerManualSync(key, func() {
				mu.Lock()
				manualCount++
				mu.Unlock()
			})
			time.Sleep(40 * time.Millisecond)
		}
	}()

	// Goroutine for immediate operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("immediate-%d", i)
			debouncer.AddImmediate(key, func() {
				mu.Lock()
				manualCount++
				mu.Unlock()
			})
			time.Sleep(80 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Wait for all debounced operations to complete
	time.Sleep(500 * time.Millisecond)

	if debouncedCount == 0 {
		t.Error("Expected at least some debounced operations to execute")
	}

	if manualCount == 0 {
		t.Error("Expected at least some manual/immediate operations to execute")
	}

	t.Logf("Debounced operations: %d, Manual/Immediate operations: %d", debouncedCount, manualCount)
}

func TestAdvancedDebouncer_ConcurrentQueueOverflow(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           1 * time.Second,
		BackoffEnabled:     false,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     5,
		ChurnWindow:        200 * time.Millisecond,
		DecayResetDuration: 1 * time.Second,
		ManualSyncTimeout:  100 * time.Millisecond, // Short timeout for this test
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var wg sync.WaitGroup
	numGoroutines := 150 // More than the queue capacity of 100

	var successCount int64
	var timeoutCount int64
	var mu sync.Mutex

	// Launch more goroutines than queue capacity to test overflow handling
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			key := fmt.Sprintf("overflow-test-%d", id)

			err := debouncer.TriggerManualSync(key, func() {
				// Simulate some work
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				successCount++
				mu.Unlock()
			})

			if err != nil {
				mu.Lock()
				if err.Error() == "manual sync timeout after 100ms" {
					timeoutCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Some operations should succeed, some might timeout due to the queue overflow
	// The exact numbers depend on timing, but we should see both success and timeout
	t.Logf("Successful syncs: %d, Timeout syncs: %d", successCount, timeoutCount)

	if successCount == 0 {
		t.Error("Expected at least some successful syncs despite queue overflow")
	}
}

func TestAdvancedDebouncer_ConcurrentStop(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()

	var wg sync.WaitGroup
	numGoroutines := 20

	// Launch goroutines that will be running when we call Stop()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			key := fmt.Sprintf("stop-test-%d", id)

			// These might be interrupted by Stop()
			_ = debouncer.TriggerManualSync(key, func() {
				time.Sleep(50 * time.Millisecond)
			})
		}(i)
	}

	// Give the goroutines a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop the debouncer while operations are in progress
	start := time.Now()
	debouncer.Stop()
	stopDuration := time.Since(start)

	// Stop should complete quickly even with operations in progress
	if stopDuration > 1*time.Second {
		t.Errorf("Stop took too long: %v", stopDuration)
	}

	wg.Wait()

	t.Logf("Stop completed in %v with %d concurrent operations", stopDuration, numGoroutines)
}

func TestAdvancedDebouncer_ConcurrentStatsAccess(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 20

	// Goroutines performing operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("stats-test-%d-%d", id, j)
				if j%3 == 0 {
					_ = debouncer.TriggerManualSync(key, func() {})
				} else {
					debouncer.Add(key, func() {})
				}
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	// Goroutines accessing stats concurrently
	statsWg := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		statsWg.Add(1)
		go func() {
			defer statsWg.Done()
			for j := 0; j < 50; j++ {
				stats := debouncer.GetStats()
				if stats == nil {
					t.Error("Stats should not be nil")
				}
				if stats["pending"] == nil {
					t.Error("Pending count should be available")
				}
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	statsWg.Wait()

	// Final stats check
	finalStats := debouncer.GetStats()
	t.Logf("Final stats: pending=%v, activity_count=%v",
		finalStats["pending"], finalStats["activity_count"])
}
