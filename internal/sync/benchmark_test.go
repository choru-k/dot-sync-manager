package sync

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/choru-k/dot-sync-manager/internal/debouncer"
	"github.com/choru-k/dot-sync-manager/internal/gitmanager"
)

// BenchmarkSyncService_StartStop measures the performance of start/stop operations
func BenchmarkSyncService_StartStop(b *testing.B) {
	tmpDir := b.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		b.Fatalf("Failed to create git manager: %v", err)
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   100 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		b.Fatalf("Failed to create sync service: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Start
		err := service.Start()
		if err != nil {
			b.Fatalf("Failed to start service: %v", err)
		}

		// Small delay to let it start
		runtime.Gosched()

		// Stop
		err = service.Stop(context.Background())
		if err != nil {
			b.Fatalf("Failed to stop service: %v", err)
		}

		// Small delay to let it stop
		runtime.Gosched()
	}
}

// BenchmarkSyncService_ConcurrentStartStop measures performance under concurrent start/stop operations
func BenchmarkSyncService_ConcurrentStartStop(b *testing.B) {
	tmpDir := b.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		b.Fatalf("Failed to create git manager: %v", err)
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   50 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		b.Fatalf("Failed to create sync service: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Start
			err := service.Start()
			if err != nil {
				b.Fatalf("Failed to start service: %v", err)
			}

			// Small delay
			runtime.Gosched()

			// Stop
			err = service.Stop(context.Background())
			if err != nil {
				b.Fatalf("Failed to stop service: %v", err)
			}

			// Small delay
			runtime.Gosched()
		}
	})
}

// BenchmarkSyncService_IsRunning measures the performance of the atomic IsRunning() check
func BenchmarkSyncService_IsRunning(b *testing.B) {
	tmpDir := b.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		b.Fatalf("Failed to create git manager: %v", err)
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   100 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		b.Fatalf("Failed to create sync service: %v", err)
	}

	// Start the service so IsRunning() returns true
	err = service.Start()
	if err != nil {
		b.Fatalf("Failed to start service: %v", err)
	}
	defer func() {
		if err := service.Stop(context.Background()); err != nil {
			b.Fatalf("Failed to stop service: %v", err)
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = service.IsRunning()
	}
}

// BenchmarkSyncService_ManualSync measures the performance of manual sync operations
func BenchmarkSyncService_ManualSync(b *testing.B) {
	tmpDir := b.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		b.Fatalf("Failed to create git manager: %v", err)
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   10 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		b.Fatalf("Failed to create sync service: %v", err)
	}

	err = service.Start()
	if err != nil {
		b.Fatalf("Failed to start service: %v", err)
	}
	defer func() {
		if err := service.Stop(context.Background()); err != nil {
			b.Fatalf("Failed to stop service: %v", err)
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := service.ManualSync()
		if err != nil {
			b.Fatalf("Failed to perform manual sync: %v", err)
		}
	}
}

// BenchmarkSyncService_WithAdvancedDebouncer measures performance with AdvancedDebouncer
func BenchmarkSyncService_WithAdvancedDebouncer(b *testing.B) {
	tmpDir := b.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		b.Fatalf("Failed to create git manager: %v", err)
	}

	// Create config with AdvancedDebouncer
	backoffConfig := &debouncer.AdvancedDebouncerConfig{
		BaseDelay:          10 * time.Millisecond,
		MaxDelay:           50 * time.Millisecond,
		BackoffEnabled:     true,
		ChurnThreshold:     5,
		ChurnWindow:        50 * time.Millisecond,
		DecayResetDuration: 200 * time.Millisecond,
		ManualSyncTimeout:  50 * time.Millisecond,
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   10 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
		Backoff:         backoffConfig,
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		b.Fatalf("Failed to create sync service: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Start
		err := service.Start()
		if err != nil {
			b.Fatalf("Failed to start service: %v", err)
		}

		// Small delay
		runtime.Gosched()

		// Stop
		err = service.Stop(context.Background())
		if err != nil {
			b.Fatalf("Failed to stop service: %v", err)
		}

		// Small delay
		runtime.Gosched()
	}
}

// BenchmarkSyncService_StopContention measures performance when multiple goroutines call Stop() concurrently
func BenchmarkSyncService_StopContention(b *testing.B) {
	tmpDir := b.TempDir()

	gitConfig := gitmanager.Config{
		RepoPath:    tmpDir,
		RemoteURL:   "https://github.com/test/test.git",
		RemoteName:  "origin",
		AuthorName:  "Test User",
		AuthorEmail: "test@example.com",
		AuthType:    gitmanager.AuthStrategyNone,
	}

	gitMgr, err := gitmanager.NewGitManager(context.Background(), gitConfig)
	if err != nil {
		b.Fatalf("Failed to create git manager: %v", err)
	}

	syncConfig := &Config{
		RepoPath:        tmpDir,
		DebounceDelay:   50 * time.Millisecond,
		AutoSyncEnabled: true,
		IgnoreFile:      ".syncignore",
	}

	service, err := New(gitMgr, syncConfig)
	if err != nil {
		b.Fatalf("Failed to create sync service: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Start the service
		err := service.Start()
		if err != nil {
			b.Fatalf("Failed to start service: %v", err)
		}

		// Multiple goroutines call Stop() concurrently
		var wg sync.WaitGroup
		numGoroutines := 4

		for j := 0; j < numGoroutines; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = service.Stop(context.Background())
			}()
		}

		wg.Wait()
	}
}
