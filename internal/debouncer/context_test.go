package debouncer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdvancedDebouncer_TriggerManualSyncWithContext(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer func() {
		if err := debouncer.Stop(); err != nil {
			t.Logf("Deferred Stop() error: %v", err)
		}
	}()

	t.Run("Context success", func(t *testing.T) {
		var called int32 // atomic bool
		err := debouncer.TriggerManualSyncWithContext(context.Background(), "test", func() {
			atomic.StoreInt32(&called, 1)
		})

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if atomic.LoadInt32(&called) == 0 {
			t.Error("Expected function to be called")
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		// Create a context that's already cancelled for deterministic behavior
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		var called int32 // atomic bool
		err := debouncer.TriggerManualSyncWithContext(ctx, "cancel-test", func() {
			atomic.StoreInt32(&called, 1)
		})

		if err == nil {
			t.Error("Expected context cancellation error")
		}

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}

		if atomic.LoadInt32(&called) != 0 {
			t.Error("Function should not have been called due to context cancellation")
		}
	})

	t.Run("Context timeout", func(t *testing.T) {
		// Create a context with very short timeout that will expire during the call
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		var called int32 // atomic bool
		err := debouncer.TriggerManualSyncWithContext(ctx, "timeout-test", func() {
			// Function that takes longer than the context timeout
			time.Sleep(50 * time.Millisecond)
			atomic.StoreInt32(&called, 1)
		})

		if err == nil {
			t.Error("Expected context timeout error")
		}

		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}

		if atomic.LoadInt32(&called) != 0 {
			t.Error("Function should not have been called due to context timeout")
		}
	})

	t.Run("Manual sync timeout still works", func(t *testing.T) {
		// Create a debouncer with very short manual sync timeout
		config := DefaultAdvancedConfig()
		config.ManualSyncTimeout = 10 * time.Millisecond

		debouncer := NewAdvanced(config)
		debouncer.Start()
		defer func() {
			if err := debouncer.Stop(); err != nil {
				t.Logf("Deferred Stop() error: %v", err)
			}
		}()

		var called int32 // atomic bool
		err := debouncer.TriggerManualSyncWithContext(context.Background(), "timeout-manual", func() {
			// Simulate a long operation
			time.Sleep(50 * time.Millisecond)
			atomic.StoreInt32(&called, 1)
		})

		if err == nil {
			t.Error("Expected manual sync timeout error")
		}

		if atomic.LoadInt32(&called) != 0 {
			t.Error("Function should have been interrupted by timeout")
		}
	})
}
