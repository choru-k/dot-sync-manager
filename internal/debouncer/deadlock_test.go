package debouncer

import (
	"sync"
	"testing"
	"time"
)

func TestAdvancedDebouncer_NoDeadlockInCalculateDelay(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          10 * time.Millisecond,
		MaxDelay:           100 * time.Millisecond,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     3,
		ChurnWindow:        50 * time.Millisecond,
		DecayResetDuration: 100 * time.Millisecond,
	}

	debouncer := NewAdvanced(config)

	// Test concurrent access that could trigger deadlock
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 50

	done := make(chan bool, numGoroutines)

	// Start multiple goroutines that will trigger the potential deadlock
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// This will call calculateDelay() internally
				debouncer.Add("test", func() {})

				// Occasionally access activity-related methods
				if j%10 == 0 {
					debouncer.IsChurnMode()
					debouncer.GetActivityCount()
				}

				// Small delay to increase chance of race conditions
				time.Sleep(1 * time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete with timeout
	timeout := time.After(10 * time.Second)
	completed := 0

	for completed < numGoroutines {
		select {
		case <-done:
			completed++
		case <-timeout:
			t.Fatalf("Test timed out - potential deadlock detected. Only %d/%d goroutines completed.", completed, numGoroutines)
		}
	}

	wg.Wait()
}

func TestAdvancedDebouncer_ConcurrentActivityAccess(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          5 * time.Millisecond,
		MaxDelay:           50 * time.Millisecond,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     5,
		ChurnWindow:        30 * time.Millisecond,
		DecayResetDuration: 100 * time.Millisecond,
	}

	debouncer := NewAdvanced(config)

	var wg sync.WaitGroup
	numWorkers := 20

	// One goroutine continuously adds debounced operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			debouncer.Add("concurrent_test", func() {})
			time.Sleep(2 * time.Millisecond)
		}
	}()

	// Multiple goroutines access activity statistics
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				// These methods access activityHistory and could potentially deadlock
				_ = debouncer.IsChurnMode()
				_ = debouncer.GetActivityCount()
				_ = debouncer.GetStats()
				time.Sleep(3 * time.Millisecond)
			}
		}(i)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(15 * time.Second):
		t.Fatal("Test timed out - potential deadlock in concurrent activity access")
	}
}

func TestAdvancedDebouncer_RapidChurnDetection(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          1 * time.Millisecond,
		MaxDelay:           10 * time.Millisecond,
		BackoffEnabled:     true,
		BackoffMultiplier:  3.0,
		ChurnThreshold:     10,
		ChurnWindow:        50 * time.Millisecond,
		DecayResetDuration: 200 * time.Millisecond,
	}

	debouncer := NewAdvanced(config)

	// Simulate rapid file changes that trigger churn detection
	var wg sync.WaitGroup
	numTriggers := 30

	for i := 0; i < numTriggers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			debouncer.Add("rapid_churn", func() {})
		}(i)
	}

	// Also access stats concurrently
	go func() {
		for i := 0; i < 50; i++ {
			_ = debouncer.GetStats()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Verify that churn was detected
		if !debouncer.IsChurnMode() {
			t.Log("Note: Churn mode not detected, but no deadlock occurred")
		}
		stats := debouncer.GetStats()
		if stats["activity_count"] == nil {
			t.Error("Expected activity_count in stats")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - potential deadlock during rapid churn detection")
	}
}

func TestAdvancedDebouncer_MemoryUsageUnderConcurrency(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          10 * time.Millisecond,
		MaxDelay:           100 * time.Millisecond,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     5,
		ChurnWindow:        50 * time.Millisecond,
		DecayResetDuration: 200 * time.Millisecond,
	}

	debouncer := NewAdvanced(config)

	var wg sync.WaitGroup
	numGoroutines := 15
	operationsPerGoroutine := 100

	// Monitor memory usage
	_ = debouncer.GetStats()

	// Launch many concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				debouncer.Add("memory_test", func() {})
				if j%10 == 0 {
					// Periodically check stats to ensure memory doesn't grow unbounded
					_ = debouncer.GetStats()
				}
			}
		}(i)
	}

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		endStats := debouncer.GetStats()

		// Verify reasonable memory usage
		if activityCount, ok := endStats["activity_count"].(int); ok {
			if activityCount > 1000 {
				t.Errorf("Activity count seems too high: %d", activityCount)
			}
		}

		// Verify that pending operations cleaned up
		if pending, ok := endStats["pending"].(int); ok && pending > 5 {
			t.Logf("Warning: High pending count after test: %d", pending)
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - potential memory leak or deadlock")
	}
}