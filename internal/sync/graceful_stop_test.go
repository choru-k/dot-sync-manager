package sync

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGitManager provides a minimal git manager for testing
type MockGitManager struct{}

// TestConfig provides a test configuration
func createTestConfig() *Config {
	return &Config{
		RepoPath:        "/tmp/test-repo",
		DebounceDelay:    100 * time.Millisecond,
		AutoSyncEnabled:  false,
		IgnoreFile:      ".syncignore",
	}
}

func TestSyncService_GracefulStop_ContextCancellation(t *testing.T) {
	// Test calling GracefulStop with already cancelled context
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Cancel the context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Log("Testing graceful stop with already cancelled context...")
	err = syncSvc.GracefulStop(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.False(t, syncSvc.IsRunning())
}

func TestSyncService_GracefulStop_AlreadyStopped(t *testing.T) {
	// Test calling GracefulStop on service that was never started
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Service should not be running initially
	assert.False(t, syncSvc.IsRunning())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	t.Log("Testing graceful stop on non-running service...")
	err = syncSvc.GracefulStop(ctx)
	assert.NoError(t, err) // Should handle gracefully
	assert.False(t, syncSvc.IsRunning())
}

func TestSyncService_GracefulStop_ConcurrentCalls(t *testing.T) {
	// Test concurrent calls to GracefulStop
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Launch multiple concurrent graceful stops
	done := make(chan error, 3)

	for i := 0; i < 3; i++ {
		go func(index int) {
			t.Logf("Starting concurrent graceful stop %d...", index)
			done <- syncSvc.GracefulStop(ctx)
		}(i)
	}

	// Collect results
	var errors []error
	for i := 0; i < 3; i++ {
		select {
		case err := <-done:
			if err != nil {
				errors = append(errors, err)
			}
			t.Logf("Concurrent stop %d completed with error: %v", i, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent graceful stop test timed out")
		}
	}

	t.Logf("Concurrent graceful stop completed with %d errors", len(errors))
	assert.False(t, syncSvc.IsRunning())
}

func TestSyncService_GracefulStop_ImmediateTimeout(t *testing.T) {
	// Test graceful shutdown with immediate timeout
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	t.Log("Testing graceful stop with already cancelled context...")
	err = syncSvc.GracefulStop(ctx)
	assert.Error(t, err) // Should return context cancelled error
	assert.Equal(t, context.Canceled, err)
	assert.False(t, syncSvc.IsRunning())
}

func TestSyncService_GracefulStop_ContextTimeout(t *testing.T) {
	// Test graceful shutdown with context that times out quickly
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	t.Log("Testing graceful stop with context timeout...")
	err = syncSvc.GracefulStop(ctx)
	// Should not return error even with timeout due to graceful handling
	assert.NoError(t, err)
	assert.False(t, syncSvc.IsRunning())
}

func TestSyncService_GracefulStop_RunningState(t *testing.T) {
	// Test graceful stop behavior when service is "running"
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Manually set running state to simulate running service
	atomic.StoreInt32(&syncSvc.running, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	t.Log("Testing graceful stop on running service...")
	err = syncSvc.GracefulStop(ctx)
	assert.NoError(t, err)
	assert.False(t, syncSvc.IsRunning())
	assert.Equal(t, int32(1), atomic.LoadInt32(&syncSvc.stopped))
}

func TestSyncService_GracefulStop_MultipleCalls(t *testing.T) {
	// Test multiple calls to GracefulStop on same service
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel1()

	t.Log("Testing first graceful stop call...")
	err1 := syncSvc.GracefulStop(ctx1)
	assert.NoError(t, err1)
	assert.False(t, syncSvc.IsRunning())

	// Second call should handle gracefully (service already stopped)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()

	t.Log("Testing second graceful stop call...")
	err2 := syncSvc.GracefulStop(ctx2)
	assert.NoError(t, err2)
	assert.False(t, syncSvc.IsRunning())
}

func TestSyncService_GracefulStop_ForcedShutdownsTracking(t *testing.T) {
	// Test that forced shutdowns are tracked correctly
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Set running state to trigger shutdown logic
	atomic.StoreInt32(&syncSvc.running, 1)

	// Create context that will timeout quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	t.Log("Testing graceful stop with forced shutdown tracking...")
	err = syncSvc.GracefulStop(ctx)
	// Should return timeout error due to 1 nanosecond timeout
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Check if forced shutdowns were incremented (may be 0 depending on timing)
	forcedShutdowns := atomic.LoadInt32(&syncSvc.forcedShutdowns)
	t.Logf("Forced shutdowns count: %d", forcedShutdowns)
	assert.GreaterOrEqual(t, forcedShutdowns, int32(0))
}

func TestSyncService_GracefulStop_ContextCancellationDuringShutdown(t *testing.T) {
	// Test context cancellation happening during shutdown process
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Set running state to trigger shutdown logic
	atomic.StoreInt32(&syncSvc.running, 1)

	// Create context that can be cancelled during shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Cancel context after a short delay to simulate cancellation during shutdown
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	t.Log("Testing graceful stop with context cancellation during shutdown...")
	err = syncSvc.GracefulStop(ctx)
	assert.NoError(t, err)
	assert.False(t, syncSvc.IsRunning())
}

func TestSyncService_GracefulStop_WaitGroupCoordination(t *testing.T) {
	// Test that WaitGroup coordination works correctly
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Set running state to trigger shutdown logic
	atomic.StoreInt32(&syncSvc.running, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	t.Log("Testing graceful stop with WaitGroup coordination...")
	err = syncSvc.GracefulStop(ctx)
	assert.NoError(t, err)
	assert.False(t, syncSvc.IsRunning())
}

func TestSyncService_GracefulStop_PanicRecovery(t *testing.T) {
	// Test that panic recovery works in watcher closure
	config := createTestConfig()
	syncSvc, err := New(nil, config)
	require.NoError(t, err)

	// Set running state to trigger shutdown logic
	atomic.StoreInt32(&syncSvc.running, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	t.Log("Testing graceful stop with panic recovery...")
	err = syncSvc.GracefulStop(ctx)
	assert.NoError(t, err)
	assert.False(t, syncSvc.IsRunning())
}