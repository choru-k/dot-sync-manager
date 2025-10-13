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
