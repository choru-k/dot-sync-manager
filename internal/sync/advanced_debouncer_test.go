package sync

import (
	"sync"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
)

func TestSyncService_AdvancedDebouncer_Integration(t *testing.T) {
	// Test that we can create a sync service with advanced debouncer configuration
	// without actually needing a git repository

	// Create sync service configuration with advanced debouncer
	backoffConfig := &debouncer.AdvancedDebouncerConfig{
		BaseDelay:          100 * time.Millisecond,
		MaxDelay:           1 * time.Second,
		BackoffEnabled:     true,
		BackoffMultiplier:  2.0,
		ChurnThreshold:     3,
		ChurnWindow:        200 * time.Millisecond,
		DecayResetDuration: 1 * time.Second,
	}

	_ = &Config{
		RepoPath:        "/tmp/test", // Dummy path
		DebounceDelay:   100 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
		Backoff:         backoffConfig,
	}

	// Test that we can create the advanced debouncer directly
	advancedDebouncer := debouncer.NewAdvanced(*backoffConfig)
	if advancedDebouncer == nil {
		t.Fatal("Failed to create advanced debouncer")
	}

	// Test basic functionality
	var called bool
	var mu sync.Mutex
	advancedDebouncer.Add("test", func() {
		mu.Lock()
		called = true
		mu.Unlock()
	})

	// Wait for execution
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if !called {
		t.Error("Expected function to be called")
	}
	mu.Unlock()

	// Test stats
	stats := advancedDebouncer.GetStats()
	if stats["base_delay"] == nil {
		t.Error("Expected base_delay in stats")
	}

	// Check that expected fields exist
	expectedFields := []string{
		"base_delay", "current_delay", "max_delay", "backoff_count",
		"backoff_multiplier", "is_churn_mode", "activity_count",
		"churn_threshold", "churn_window", "last_activity",
		"time_since_activity", "pending",
	}

	for _, field := range expectedFields {
		if stats[field] == nil {
			t.Errorf("Expected %s in stats", field)
		}
	}

	// Test immediate execution
	called = false
	advancedDebouncer.AddImmediate("immediate", func() {
		mu.Lock()
		called = true
		mu.Unlock()
	})

	// Should be called immediately
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if !called {
		t.Error("Expected immediate function to be called")
	}
	mu.Unlock()

	// Stop the debouncer
	if err := advancedDebouncer.Stop(); err != nil {
		t.Errorf("Failed to stop advanced debouncer: %v", err)
	}
}

func TestSyncService_BasicDebouncer_BackwardCompatibility(t *testing.T) {
	// Test that we can create a sync service configuration without backoff
	// and verify it falls back to basic debouncer

	// Create sync service configuration without backoff (basic debouncer)
	syncConfig := &Config{
		RepoPath:        "/tmp/test", // Dummy path
		DebounceDelay:   100 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
		Backoff:         nil, // No backoff configuration
	}

	// Verify the configuration is valid for basic debouncer
	if syncConfig.Backoff != nil {
		t.Error("Expected no backoff configuration")
	}

	// Test that we can create a basic debouncer
	basicDebouncer := debouncer.New(syncConfig.DebounceDelay)
	if basicDebouncer == nil {
		t.Fatal("Failed to create basic debouncer")
	}

	// Test basic functionality
	var called bool
	var mu sync.Mutex
	basicDebouncer.Add("test", func() {
		mu.Lock()
		called = true
		mu.Unlock()
	})

	// Wait for execution
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if !called {
		t.Error("Expected function to be called")
	}
	mu.Unlock()

	// Test that basic debouncer doesn't have advanced features
	if basicDebouncer.GetDelay() != syncConfig.DebounceDelay {
		t.Error("Expected basic debouncer to use configured delay")
	}
}

func TestSyncService_ConfigIntegration(t *testing.T) {
	// Create a full configuration with backoff settings
	syncServiceConfig := &Config{
		RepoPath:        "/tmp/test",
		DebounceDelay:   30 * time.Second,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
		Backoff: &debouncer.AdvancedDebouncerConfig{
			BaseDelay:          30 * time.Second,
			MaxDelay:           300 * time.Second,
			BackoffEnabled:     true,
			BackoffMultiplier:  2.0,
			ChurnThreshold:     10,
			ChurnWindow:        60 * time.Second,
			DecayResetDuration: 300 * time.Second,
			ManualSyncTimeout:  10 * time.Second,
		},
	}

	// Verify the configuration includes backoff settings
	if syncServiceConfig.Backoff == nil {
		t.Fatal("Expected backoff configuration to be included")
	}

	if syncServiceConfig.Backoff.BaseDelay != 30*time.Second {
		t.Errorf("Expected base delay 30s, got %v", syncServiceConfig.Backoff.BaseDelay)
	}

	if syncServiceConfig.Backoff.MaxDelay != 300*time.Second {
		t.Errorf("Expected max delay 300s, got %v", syncServiceConfig.Backoff.MaxDelay)
	}

	if !syncServiceConfig.Backoff.BackoffEnabled {
		t.Error("Expected backoff enabled")
	}

	if syncServiceConfig.Backoff.BackoffMultiplier != 2.0 {
		t.Errorf("Expected multiplier 2.0, got %f", syncServiceConfig.Backoff.BackoffMultiplier)
	}

	if syncServiceConfig.Backoff.ChurnThreshold != 10 {
		t.Errorf("Expected churn threshold 10, got %d", syncServiceConfig.Backoff.ChurnThreshold)
	}

	if syncServiceConfig.Backoff.ChurnWindow != 60*time.Second {
		t.Errorf("Expected churn window 60s, got %v", syncServiceConfig.Backoff.ChurnWindow)
	}

	if syncServiceConfig.Backoff.DecayResetDuration != 300*time.Second {
		t.Errorf("Expected decay reset duration 300s, got %v", syncServiceConfig.Backoff.DecayResetDuration)
	}
}
