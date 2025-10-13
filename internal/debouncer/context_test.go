package debouncer

import (
	"context"
	"testing"
	"time"
)

func TestAdvancedDebouncer_TriggerManualSyncWithContext(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	t.Run("Context success", func(t *testing.T) {
		var called bool
		err := debouncer.TriggerManualSyncWithContext(context.Background(), "test", func() {
			called = true
		})

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !called {
			t.Error("Expected function to be called")
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel the context immediately
		cancel()

		var called bool
		err := debouncer.TriggerManualSyncWithContext(ctx, "cancel-test", func() {
			called = true
		})

		if err == nil {
			t.Error("Expected context cancellation error")
		}

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}

		if called {
			t.Error("Function should not have been called due to context cancellation")
		}
	})

	t.Run("Context timeout", func(t *testing.T) {
		// Create a context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Wait for context to timeout
		time.Sleep(10 * time.Millisecond)

		var called bool
		err := debouncer.TriggerManualSyncWithContext(ctx, "timeout-test", func() {
			called = true
		})

		if err == nil {
			t.Error("Expected context timeout error")
		}

		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
		}

		if called {
			t.Error("Function should not have been called due to context timeout")
		}
	})

	t.Run("Manual sync timeout still works", func(t *testing.T) {
		// Create a debouncer with very short manual sync timeout
		config := DefaultAdvancedConfig()
		config.ManualSyncTimeout = 10 * time.Millisecond

		debouncer := NewAdvanced(config)
		debouncer.Start()
		defer debouncer.Stop()

		var called bool
		err := debouncer.TriggerManualSyncWithContext(context.Background(), "timeout-manual", func() {
			// Simulate a long operation
			time.Sleep(50 * time.Millisecond)
			called = true
		})

		if err == nil {
			t.Error("Expected manual sync timeout error")
		}

		if called {
			t.Error("Function should have been interrupted by timeout")
		}
	})
}