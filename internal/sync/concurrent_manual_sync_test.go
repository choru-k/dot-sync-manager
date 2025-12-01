package sync

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
	"github.com/stretchr/testify/require"
)

func TestSyncService_ConcurrentManualSyncAndStop(t *testing.T) {
	// Create sync service with test configuration
	config := &Config{
		RepoPath:        t.TempDir(),
		DebounceDelay:   50 * time.Millisecond,
		AutoSyncEnabled: true,
	}

	// Create git manager config
	gitConfig := gitmanager.Config{
		RepoPath:    config.RepoPath,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	// Create git manager
	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	require.NoError(t, err)

	service, err := New(gitMgr, config)
	require.NoError(t, err)

	// Start the service
	err = service.Start()
	require.NoError(t, err)

	var wg sync.WaitGroup
	numGoroutines := 20
	var successCount int64
	var errorCount int64

	// Launch goroutines performing ManualSync while Stop() is called
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			err := service.ManualSync()
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				// Expected errors include "service is stopped" etc.
				require.Contains(t, err.Error(), "service is")
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	// Give goroutines time to start
	time.Sleep(10 * time.Millisecond)

	// Stop the service while operations are in progress
	stopStart := time.Now()
	err = service.Stop(context.Background())
	stopDuration := time.Since(stopStart)
	require.NoError(t, err)

	// Stop should complete quickly
	require.Less(t, stopDuration, 1*time.Second)

	wg.Wait()

	// Verify results - some should succeed, some should fail with service stopped errors
	totalOps := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&errorCount)
	require.Equal(t, int64(numGoroutines), totalOps)

	t.Logf("Concurrent test: %d successful, %d errors, Stop took: %v",
		successCount, errorCount, stopDuration)
}

func TestSyncService_ConcurrentManualSyncAndStop_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Create sync service with test configuration
	config := &Config{
		RepoPath:        t.TempDir(),
		DebounceDelay:   10 * time.Millisecond, // Faster for stress test
		AutoSyncEnabled: true,
	}

	// Create git manager config
	gitConfig := gitmanager.Config{
		RepoPath:    config.RepoPath,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	// Create git manager
	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	require.NoError(t, err)

	service, err := New(gitMgr, config)
	require.NoError(t, err)

	err = service.Start()
	require.NoError(t, err)

	var wg sync.WaitGroup
	numGoroutines := 100
	operationsPerGoroutine := 5
	var successCount int64
	var errorCount int64

	// Launch many goroutines performing multiple ManualSync operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				err := service.ManualSync()
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					// Any error is acceptable during stress test
				} else {
					atomic.AddInt64(&successCount, 1)
				}
				// Small delay between operations
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Let operations run for a bit
	time.Sleep(50 * time.Millisecond)

	// Stop the service while many operations are in progress
	stopStart := time.Now()
	err = service.Stop(context.Background())
	stopDuration := time.Since(stopStart)
	require.NoError(t, err)

	// Stop should complete quickly even under stress
	require.Less(t, stopDuration, 2*time.Second)

	wg.Wait()

	totalOps := atomic.LoadInt64(&successCount) + atomic.LoadInt64(&errorCount)
	require.Equal(t, int64(numGoroutines*operationsPerGoroutine), totalOps)

	t.Logf("Stress test: %d successful, %d errors, Stop took: %v",
		successCount, errorCount, stopDuration)
}
