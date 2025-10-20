package debouncer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdvancedDebouncer_New(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)

	if debouncer.GetBaseDelay() != config.BaseDelay {
		t.Errorf("Expected base delay %v, got %v", config.BaseDelay, debouncer.GetBaseDelay())
	}

	if debouncer.GetMaxDelay() != config.MaxDelay {
		t.Errorf("Expected max delay %v, got %v", config.MaxDelay, debouncer.GetMaxDelay())
	}

	if debouncer.Pending() != 0 {
		t.Errorf("Expected 0 pending operations, got %d", debouncer.Pending())
	}
}

func TestAdvancedDebouncer_BasicDebounce(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          100 * time.Millisecond,
		MaxDelay:           5 * time.Minute,
		BackoffEnabled:     false,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     10,
		ChurnWindow:        1 * time.Minute,
		DecayResetDuration: 5 * time.Minute,
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Add multiple operations rapidly
	for i := 0; i < 5; i++ {
		debouncer.Add("test", fn)
		time.Sleep(10 * time.Millisecond) // Small delay between additions
	}

	// Wait for debounce to complete
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
	mu.Unlock()

	if debouncer.Pending() != 0 {
		t.Errorf("Expected 0 pending operations after completion, got %d", debouncer.Pending())
	}
}

func TestAdvancedDebouncer_ImmediateExecution(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          1 * time.Second,
		MaxDelay:           5 * time.Minute,
		BackoffEnabled:     false,
		ChurnThreshold:     10,
		ChurnWindow:        1 * time.Minute,
		DecayResetDuration: 5 * time.Minute,
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Add immediate execution
	debouncer.AddImmediate("test", fn)

	// Should be called immediately
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if callCount != 1 {
		t.Errorf("Expected 1 immediate call, got %d", callCount)
	}
	mu.Unlock()
}

func TestAdvancedDebouncer_TriggerManualSync(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Trigger manual sync
	err := debouncer.TriggerManualSync("test", fn)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if callCount != 1 {
		t.Errorf("Expected 1 manual sync call, got %d", callCount)
	}
	mu.Unlock()
}

func TestAdvancedDebouncer_ChurnDetection(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           5 * time.Minute,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     5, // Low threshold for testing
		ChurnWindow:        200 * time.Millisecond,
		DecayResetDuration: 1 * time.Second, // Shorter for testing
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Generate rapid activity to trigger churn
	for i := 0; i < config.ChurnThreshold; i++ {
		debouncer.Add("test", fn)
		time.Sleep(10 * time.Millisecond)
	}

	// Check if churn is detected
	if !debouncer.IsChurnMode() {
		t.Error("Expected churn mode to be detected")
	}

	if debouncer.GetActivityCount() < config.ChurnThreshold {
		t.Errorf("Expected activity count >= %d, got %d", config.ChurnThreshold, debouncer.GetActivityCount())
	}

	// Add one more operation to trigger backoff calculation
	debouncer.Add("test_final", fn)

	// Check that backoff is applied
	currentDelay := debouncer.GetDelay()
	if currentDelay <= config.BaseDelay {
		t.Errorf("Expected backoff delay > base delay, got %v <= %v", currentDelay, config.BaseDelay)
	}

	// Wait for activity to decay
	time.Sleep(config.DecayResetDuration + 200*time.Millisecond)

	// Add another operation - should return to base delay
	debouncer.Add("test2", fn)

	newDelay := debouncer.GetDelay()
	if newDelay != config.BaseDelay {
		t.Errorf("Expected delay to reset to base delay after decay, got %v", newDelay)
	}
}

func TestAdvancedDebouncer_ExponentialBackoff(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           500 * time.Millisecond,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     3,
		ChurnWindow:        200 * time.Millisecond,
		DecayResetDuration: 5 * time.Minute,
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// First operation - should use base delay
	debouncer.Add("test", fn)
	firstDelay := debouncer.GetDelay()
	if firstDelay != config.BaseDelay {
		t.Errorf("Expected first delay to be base delay, got %v", firstDelay)
	}

	// Generate more activity to trigger backoff
	for i := 0; i < config.ChurnThreshold; i++ {
		debouncer.Add("test", fn)
		time.Sleep(10 * time.Millisecond)
	}

	// Check backoff is applied
	backoffDelay := debouncer.GetDelay()
	if backoffDelay <= config.BaseDelay {
		t.Errorf("Expected backoff delay > base delay, got %v", backoffDelay)
	}

	// Check backoff count increases
	if debouncer.GetBackoffCount() <= 0 {
		t.Errorf("Expected backoff count > 0, got %d", debouncer.GetBackoffCount())
	}
}

func TestAdvancedDebouncer_Cancel(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Add operation
	debouncer.Add("test", fn)

	// Cancel immediately
	debouncer.Cancel("test")

	// Wait to ensure it doesn't execute
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if callCount != 0 {
		t.Errorf("Expected 0 calls after cancel, got %d", callCount)
	}
	mu.Unlock()

	if debouncer.Pending() != 0 {
		t.Errorf("Expected 0 pending operations after cancel, got %d", debouncer.Pending())
	}
}

func TestAdvancedDebouncer_CancelAll(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Add multiple operations
	for i := 0; i < 3; i++ {
		debouncer.Add(string(rune('a'+i)), fn)
	}

	// Cancel all
	debouncer.CancelAll()

	// Wait to ensure none execute
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if callCount != 0 {
		t.Errorf("Expected 0 calls after cancel all, got %d", callCount)
	}
	mu.Unlock()

	if debouncer.Pending() != 0 {
		t.Errorf("Expected 0 pending operations after cancel all, got %d", debouncer.Pending())
	}
}

func TestAdvancedDebouncer_GetStats(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var callCount int
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	// Add some activity
	for i := 0; i < 3; i++ {
		debouncer.Add("test", fn)
		time.Sleep(10 * time.Millisecond)
	}

	stats := debouncer.GetStats()

	// Check required stats fields
	requiredFields := []string{
		"pending", "base_delay", "current_delay", "max_delay",
		"backoff_count", "backoff_multiplier", "is_churn_mode",
		"activity_count", "churn_threshold", "churn_window",
		"last_activity", "time_since_activity",
	}

	for _, field := range requiredFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Missing required stat field: %s", field)
		}
	}

	// Check some specific values
	if stats["base_delay"] != config.BaseDelay.String() {
		t.Errorf("Expected base_delay %s, got %v", config.BaseDelay.String(), stats["base_delay"])
	}

	if stats["max_delay"] != config.MaxDelay.String() {
		t.Errorf("Expected max_delay %s, got %v", config.MaxDelay.String(), stats["max_delay"])
	}

	if stats["backoff_multiplier"] != config.BackoffMultiplier {
		t.Errorf("Expected backoff_multiplier %f, got %v", config.BackoffMultiplier, stats["backoff_multiplier"])
	}
}

func TestAdvancedDebouncer_ConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config AdvancedDebouncerConfig
	}{
		{
			name: "zero base delay",
			config: AdvancedDebouncerConfig{
				BaseDelay: 0,
			},
		},
		{
			name: "negative max delay",
			config: AdvancedDebouncerConfig{
				BaseDelay: 30 * time.Second,
				MaxDelay:  -1 * time.Second,
			},
		},
		{
			name: "invalid backoff multiplier",
			config: AdvancedDebouncerConfig{
				BaseDelay:         30 * time.Second,
				MaxDelay:          5 * time.Minute,
				BackoffMultiplier: 0.5,
			},
		},
		{
			name: "zero churn threshold",
			config: AdvancedDebouncerConfig{
				BaseDelay:      30 * time.Second,
				MaxDelay:       5 * time.Minute,
				ChurnThreshold: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			debouncer := NewAdvanced(tt.config)

			// Should use default values for invalid config
			defaultConfig := DefaultAdvancedConfig()

			if debouncer.GetBaseDelay() != defaultConfig.BaseDelay {
				t.Errorf("Expected default base delay for invalid config, got %v", debouncer.GetBaseDelay())
			}

			if debouncer.GetMaxDelay() != defaultConfig.MaxDelay {
				t.Errorf("Expected default max delay for invalid config, got %v", debouncer.GetMaxDelay())
			}
		})
	}
}

func TestAdvancedDebouncer_ConcurrentAccess(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           5 * time.Minute,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     10,
		ChurnWindow:        1 * time.Minute,
		DecayResetDuration: 5 * time.Minute,
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10
	operationsPerGoroutine := 5

	fn := func() {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
	}

	// Launch multiple goroutines adding operations concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("key-%d", id)
				debouncer.Add(key, fn)
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all operations to complete
	time.Sleep(300 * time.Millisecond)

	// Should not have panicked and should have some activity
	stats := debouncer.GetStats()
	if stats["activity_count"] == nil {
		t.Error("Expected activity count in stats")
	}
}

func TestAdvancedDebouncer_ConcurrentStopNew(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()

	// Test concurrent Stop calls
	numGoroutines := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			debouncer.Stop()
		}()
	}

	wg.Wait()

	// Additional verification: multiple calls should be safe
	// Call Stop a few more times sequentially
	for i := 0; i < 3; i++ {
		debouncer.Stop() // Should not panic
	}

	t.Log("Concurrent Stop test completed successfully")
}

func TestAdvancedDebouncer_MultipleStopCalls(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()

	// Add some pending operations
	var callCount int64
	var mu sync.Mutex

	fn := func() {
		mu.Lock()
		defer mu.Unlock()
		atomic.AddInt64(&callCount, 1)
	}

	// Add some operations
	for i := 0; i < 3; i++ {
		debouncer.Add(fmt.Sprintf("test-%d", i), fn)
	}

	// Test multiple sequential Stop() calls
	for i := 0; i < 5; i++ {
		debouncer.Stop() // Should not panic, cleanup should happen only once
	}

	// Wait a bit for any pending operations to complete
	time.Sleep(50 * time.Millisecond)

	t.Log("Multiple Stop calls test completed successfully")
}

func TestAdvancedDebouncer_StopIdempotency(t *testing.T) {
	config := DefaultAdvancedConfig()
	debouncer := NewAdvanced(config)
	debouncer.Start()

	// First stop should cancel all operations and close channels
	debouncer.Stop()

	// Wait for cleanup to complete
	time.Sleep(DefaultShutdownTimeout + 50*time.Millisecond)

	// Multiple subsequent stops should not panic
	for i := 0; i < 10; i++ {
		debouncer.Stop()
	}

	// The debouncer should be in a consistent state
	// Note: After stop, the debouncer is in a shutdown state and cannot be restarted
	// This is expected behavior

	t.Log("Stop idempotency test completed successfully")
}

func TestAdvancedDebouncer_ConcurrentStopWithOperations(t *testing.T) {
	config := AdvancedDebouncerConfig{
		BaseDelay:          50 * time.Millisecond,
		MaxDelay:           1 * time.Second,
		BackoffEnabled:     false,
		ChurnThreshold:     10,
		ChurnWindow:        200 * time.Millisecond,
		DecayResetDuration: 1 * time.Second,
		ManualSyncTimeout:  100 * time.Millisecond,
	}

	debouncer := NewAdvanced(config)
	debouncer.Start()
	defer debouncer.Stop()

	var operationCount int64
	var wg sync.WaitGroup

	// Start adding operations concurrently
	numOperationGoroutines := 5
	for i := 0; i < numOperationGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				fn := func() {
					atomic.AddInt64(&operationCount, 1)
				}
				debouncer.Add(fmt.Sprintf("concurrent-test-%d-%d", id, j), fn)
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	// Wait for some operations to be added
	time.Sleep(20 * time.Millisecond)

	// Start concurrent Stop calls
	numStopGoroutines := 3
	var stopWg sync.WaitGroup
	for i := 0; i < numStopGoroutines; i++ {
		stopWg.Add(1)
		go func() {
			defer stopWg.Done()
			debouncer.Stop()
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	stopWg.Wait()

	// Wait for cleanup
	time.Sleep(DefaultShutdownTimeout + 50*time.Millisecond)

	// Test should complete without panics
	t.Logf("Concurrent Stop with operations test completed successfully. Operations: %d", atomic.LoadInt64(&operationCount))
}
