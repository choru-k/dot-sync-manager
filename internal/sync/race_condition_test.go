package sync

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

// TestSyncService_StopStartRaceConditionRegression tests the specific race condition
// identified by Codex where Stop() sets state to StateStopped before shutdownWG.Wait(),
// allowing Start() to slip in and wedge the shutdown.
func TestSyncService_StopStartRaceConditionRegression(t *testing.T) {
	gitMgr := &gitmanager.GitManager{}
	syncConfig := &Config{
		RepoPath:        "/tmp/test-stop-start-race",
		DebounceDelay:   10 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service
	if err := service.Start(); err != nil {
		// Expected to fail due to missing repo, but lifecycle should still work
		t.Logf("Start failed as expected: %v", err)
	}

	// Give it a moment to initialize
	time.Sleep(1 * time.Millisecond)

	// Test the specific race condition scenario
	const iterations = 100
	t.Logf("Running %d concurrent Stop/Start iterations to test race condition", iterations)

	for i := 0; i < iterations; i++ {
		var wg sync.WaitGroup

		// Launch Stop() in a goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := service.Stop(context.Background()); err != nil {
				t.Logf("Stop failed (iteration %d): %v", i, err)
			}
		}()

		// Small delay to increase chance of race condition
		time.Sleep(10 * time.Microsecond)

		// Launch Start() in a goroutine that should NOT be able to wedge the shutdown
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := service.Start(); err != nil {
				t.Logf("Start failed (iteration %d): %v", i, err)
			}
		}()

		// Wait for both operations to complete with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Both operations completed successfully
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Deadlock detected in iteration %d - Stop/Start operations did not complete within timeout", i)
			return
		}

		// Verify service is in a consistent state
		state := atomic.LoadInt32(&service.state)
		if state != int32(StateStopped) && state != int32(StateRunning) {
			t.Errorf("Service in inconsistent state %d in iteration %d", state, i)
		}
	}

	t.Logf("Successfully completed %d Stop/Start race condition iterations without deadlock", iterations)
}

// TestSyncService_ConcurrentStopStartStressTest puts the race condition under heavy stress
func TestSyncService_ConcurrentStopStartStressTest(t *testing.T) {
	gitMgr := &gitmanager.GitManager{}
	syncConfig := &Config{
		RepoPath:        "/tmp/test-concurrent-stress",
		DebounceDelay:   5 * time.Millisecond, // Very short for stress testing
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service
	if err := service.Start(); err != nil {
		t.Logf("Start failed as expected: %v", err)
	}

	// Stress test with many concurrent operations
	const numGoroutines = 10
	const operationsPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*operationsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Alternate between Stop and Start operations
				if j%2 == 0 {
					if err := service.Stop(context.Background()); err != nil {
						errors <- err
					}
				} else {
					if err := service.Start(); err != nil {
						errors <- err
					}
				}

				// Small random delay to increase contention
				time.Sleep(time.Duration(1+j%3) * time.Millisecond)
			}
		}(i)
	}

	// Wait for all operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Logf("Stress test completed: %d goroutines × %d operations each", numGoroutines, operationsPerGoroutine)
	case <-time.After(5 * time.Second):
		t.Error("Stress test timed out - potential deadlock detected")
		return
	}

	// Check for any errors (expect some failures due to missing repo, but no deadlocks)
	errorCount := len(errors)
	t.Logf("Encountered %d errors during stress test (expected due to missing repo)", errorCount)

	// Verify service is in a stable final state
	finalState := atomic.LoadInt32(&service.state)
	if finalState != int32(StateStopped) {
		t.Errorf("Service not in stable stopped state after stress test, state: %d", finalState)
	}
}
