package debouncer

import (
	"sync"
	"testing"
	"time"
)

func TestDebouncer_BasicFunctionality(t *testing.T) {
	d := New(100 * time.Millisecond)
	defer d.CancelAll()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		defer mu.Unlock()
		callCount++
	}

	// Add function multiple times rapidly
	d.Add("test1", fn)
	d.Add("test1", fn)
	d.Add("test1", fn)

	// Should not be called immediately
	mu.Lock()
	if callCount != 0 {
		t.Errorf("Expected 0 calls, got %d", callCount)
	}
	mu.Unlock()

	// Wait for debounce
	time.Sleep(150 * time.Millisecond)

	// Should be called exactly once
	mu.Lock()
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
	mu.Unlock()
}

func TestDebouncer_MultipleKeys(t *testing.T) {
	d := New(50 * time.Millisecond)
	defer d.CancelAll()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		defer mu.Unlock()
		callCount++
	}

	// Add functions with different keys
	d.Add("key1", fn)
	d.Add("key2", fn)

	time.Sleep(100 * time.Millisecond)

	// Both should be called
	mu.Lock()
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
	mu.Unlock()
}

func TestDebouncer_Cancel(t *testing.T) {
	d := New(100 * time.Millisecond)
	defer d.CancelAll()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		defer mu.Unlock()
		callCount++
	}

	d.Add("test", fn)
	d.Cancel("test")

	// Wait longer than debounce
	time.Sleep(150 * time.Millisecond)

	// Should not be called
	mu.Lock()
	if callCount != 0 {
		t.Errorf("Expected 0 calls after cancel, got %d", callCount)
	}
	mu.Unlock()
}

func TestDebouncer_Pending(t *testing.T) {
	d := New(100 * time.Millisecond)
	defer d.CancelAll()

	if d.Pending() != 0 {
		t.Errorf("Expected 0 pending, got %d", d.Pending())
	}

	d.Add("test1", func() {})
	if d.Pending() != 1 {
		t.Errorf("Expected 1 pending, got %d", d.Pending())
	}

	d.Add("test2", func() {})
	if d.Pending() != 2 {
		t.Errorf("Expected 2 pending, got %d", d.Pending())
	}

	d.CancelAll()
	if d.Pending() != 0 {
		t.Errorf("Expected 0 pending after CancelAll, got %d", d.Pending())
	}
}

func TestDebouncer_SetDelay(t *testing.T) {
	d := New(100 * time.Millisecond)
	defer d.CancelAll()

	if d.GetDelay() != 100*time.Millisecond {
		t.Errorf("Expected 100ms delay, got %v", d.GetDelay())
	}

	d.SetDelay(200 * time.Millisecond)
	if d.GetDelay() != 200*time.Millisecond {
		t.Errorf("Expected 200ms delay, got %v", d.GetDelay())
	}
}

// TestDebouncer_NoDoubleExecution verifies that the race condition fix works:
// when a new callback is added while an old timer is about to fire, the old
// callback should not execute and only the new callback should run once.
func TestDebouncer_NoDoubleExecution(t *testing.T) {
	d := New(50 * time.Millisecond)
	defer d.CancelAll()

	var fn1CallCount, fn2CallCount int
	var mu sync.Mutex

	fn1 := func() {
		mu.Lock()
		defer mu.Unlock()
		fn1CallCount++
	}

	fn2 := func() {
		mu.Lock()
		defer mu.Unlock()
		fn2CallCount++
	}

	// Add first callback
	d.Add("test", fn1)

	// Wait until just before the timer fires
	time.Sleep(40 * time.Millisecond)

	// Add second callback to replace the first
	// This should cancel the first timer and schedule a new one
	d.Add("test", fn2)

	// Wait for the original delay to pass (fn1 would have fired here if not cancelled)
	time.Sleep(20 * time.Millisecond)

	// fn1 should not have been called
	mu.Lock()
	if fn1CallCount != 0 {
		t.Errorf("Expected fn1 to be called 0 times, got %d", fn1CallCount)
	}
	if fn2CallCount != 0 {
		t.Errorf("Expected fn2 to be called 0 times at this point, got %d", fn2CallCount)
	}
	mu.Unlock()

	// Wait for fn2's delay to complete
	time.Sleep(60 * time.Millisecond)

	// fn2 should be called exactly once
	mu.Lock()
	if fn1CallCount != 0 {
		t.Errorf("Expected fn1 to be called 0 times (cancelled), got %d", fn1CallCount)
	}
	if fn2CallCount != 1 {
		t.Errorf("Expected fn2 to be called exactly 1 time, got %d", fn2CallCount)
	}
	mu.Unlock()
}

// TestDebouncer_RapidReplacement tests that rapidly replacing callbacks
// results in only the final callback being executed once.
func TestDebouncer_RapidReplacement(t *testing.T) {
	d := New(100 * time.Millisecond)
	defer d.CancelAll()

	var lastCallbackID int
	var callCount int
	var mu sync.Mutex

	// Rapidly add 10 different callbacks with the same key
	for i := 1; i <= 10; i++ {
		id := i
		d.Add("test", func() {
			mu.Lock()
			defer mu.Unlock()
			lastCallbackID = id
			callCount++
		})
		time.Sleep(5 * time.Millisecond) // Small delay between additions
	}

	// Wait for debounce to complete
	time.Sleep(150 * time.Millisecond)

	// Only the last callback (id=10) should have been called exactly once
	mu.Lock()
	if callCount != 1 {
		t.Errorf("Expected exactly 1 call, got %d", callCount)
	}
	if lastCallbackID != 10 {
		t.Errorf("Expected last callback (id=10) to be called, got id=%d", lastCallbackID)
	}
	mu.Unlock()
}
