package debouncer

import (
	"sync"
	"testing"
	"time"
)

func TestAdvancedDebouncer_BackoffEnabledFlag(t *testing.T) {
	tests := []struct {
		name           string
		backoffEnabled bool
		expectBackoff  bool
	}{
		{
			name:           "backoff enabled",
			backoffEnabled: true,
			expectBackoff:  true,
		},
		{
			name:           "backoff disabled",
			backoffEnabled: false,
			expectBackoff:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AdvancedDebouncerConfig{
				BaseDelay:          10 * time.Millisecond,
				MaxDelay:           100 * time.Millisecond,
				BackoffEnabled:     tt.backoffEnabled,
				BackoffMultiplier:  2.0,
				ChurnThreshold:     3,
				ChurnWindow:        50 * time.Millisecond,
				DecayResetDuration: 100 * time.Millisecond,
			}

			debouncer := NewAdvanced(config)

			var executionCount int
			var mu sync.Mutex

			// Trigger multiple rapid changes to induce churn
			for i := 0; i < 5; i++ {
				debouncer.Add("test", func() {
					mu.Lock()
					executionCount++
					mu.Unlock()
				})
				time.Sleep(5 * time.Millisecond) // Small delay between triggers
			}

			// Wait for all executions to complete
			time.Sleep(200 * time.Millisecond)

			mu.Lock()
			finalCount := executionCount
			mu.Unlock()

			// We should have exactly 1 execution (debounced) regardless of backoff setting
			if finalCount != 1 {
				t.Errorf("Expected 1 execution, got %d", finalCount)
			}

			// Test that the backoff setting is respected internally
			if tt.expectBackoff {
				// With backoff enabled, the field should be true
				if !debouncer.IsBackoffEnabled() {
					t.Error("Expected backoff to be enabled but it was disabled")
				}
			} else {
				// With backoff disabled, the field should be false
				if debouncer.IsBackoffEnabled() {
					t.Error("Expected backoff to be disabled but it was enabled")
				}
			}
		})
	}
}
