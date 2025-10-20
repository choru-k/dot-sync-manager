package debouncer

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSimplifiedStateTransitions(t *testing.T) {
	config := DefaultAdvancedConfig()
	d := NewAdvancedSimplified(config)

	// Test initial state
	if d.Pending() != 0 {
		t.Errorf("Expected 0 pending, got %d", d.Pending())
	}

	// Add and cancel
	executed := false
	d.Add("test", func() { executed = true })
	if d.Pending() != 1 {
		t.Errorf("Expected 1 pending, got %d", d.Pending())
	}

	d.Cancel("test")
	if d.Pending() != 0 {
		t.Errorf("Expected 0 pending after cancel, got %d", d.Pending())
	}

	// Stop
	if err := d.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestSimplifiedManualSync(t *testing.T) {
	config := DefaultAdvancedConfig()
	d := NewAdvancedSimplified(config)

	// Test successful manual sync
	err := d.TriggerManualSync("manual", func() {
		// Do nothing
	})
	if err != nil {
		t.Errorf("Manual sync failed: %v", err)
	}

	// Test manual sync with panic
	err = d.TriggerManualSync("panic", func() {
		panic("test panic")
	})
	if err == nil {
		t.Error("Expected error from panic")
	}
	if err.Error() != "panic during manual sync: test panic" {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Stop and test
	if err := d.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
	err = d.TriggerManualSync("after_stop", func() {})
	if err == nil {
		t.Error("Expected error when triggering manual sync after stop")
	}
}

func TestSimplifiedConcurrentAccess(t *testing.T) {
	config := DefaultAdvancedConfig()
	d := NewAdvancedSimplified(config)
	defer func() {
		if err := d.Stop(); err != nil {
			t.Errorf("Failed to stop debouncer: %v", err)
		}
	}()

	// Test concurrent adds
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			d.Add(string(rune(id)), func() {})
		}(i)
	}
	wg.Wait()

	// Test concurrent manual syncs
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = d.TriggerManualSync(string(rune(id)), func() {})
		}(i)
	}
	wg.Wait()
}

func TestSimplifiedChurnDetection(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          10 * time.Millisecond,
		MaxDelay:           100 * time.Millisecond,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     5,
		ChurnWindow:        100 * time.Millisecond,
		DecayResetDuration: 200 * time.Millisecond,
	}
	d := NewAdvancedSimplified(config)
	defer func() {
		if err := d.Stop(); err != nil {
			t.Errorf("Failed to stop debouncer: %v", err)
		}
	}()

	// Trigger multiple adds to detect churn
	for i := 0; i < 5; i++ {
		d.Add("churn", func() {})
		time.Sleep(10 * time.Millisecond)
	}

	// Check if backoff is applied
	delay := d.GetDelay()
	if delay <= config.BaseDelay {
		t.Errorf("Expected backoff delay > base delay, got %v", delay)
	}
}

func TestSimplifiedTimeout(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:         10 * time.Millisecond,
		ManualSyncTimeout: 50 * time.Millisecond,
	}
	d := NewAdvancedSimplified(config)
	defer func() {
		if err := d.Stop(); err != nil {
			t.Errorf("Failed to stop debouncer: %v", err)
		}
	}()

	// Test timeout
	err := d.TriggerManualSync("timeout", func() {
		time.Sleep(100 * time.Millisecond) // Longer than timeout
	})
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func BenchmarkSimplifiedAdd(b *testing.B) {
	config := DefaultAdvancedConfig()
	d := NewAdvancedSimplified(config)
	defer func() {
		if err := d.Stop(); err != nil {
			t.Errorf("Failed to stop debouncer: %v", err)
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			d.Add(string(rune(i%256)), func() {})
			i++
		}
	})
}

func BenchmarkSimplifiedManualSync(b *testing.B) {
	config := DefaultAdvancedConfig()
	d := NewAdvancedSimplified(config)
	defer func() {
		if err := d.Stop(); err != nil {
			t.Errorf("Failed to stop debouncer: %v", err)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.TriggerManualSync("bench", func() {})
	}
}