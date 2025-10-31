package sync

// Error IDs for monitoring and debugging
// These IDs help track specific error types in monitoring systems like Sentry
//
// Usage Example:
//   return fmt.Errorf("operation failed [%s]: %w", ErrIDManualSyncFailed, err)
//
// This pattern enables:
// - Structured monitoring and alerting
// - Easy error type classification
// - Consistent error tracking across the system
// - Integration with observability platforms

const (
	// Service lifecycle errors
	ErrIDServiceAlreadyRunning     = "SYNC_SERVICE_ALREADY_RUNNING"
	ErrIDServiceStopRaceCondition   = "SYNC_SERVICE_STOP_RACE_CONDITION"
	ErrIDServiceRestartNotSupported = "SYNC_SERVICE_RESTART_NOT_SUPPORTED"
	ErrIDServiceAlreadyStopped      = "SYNC_SERVICE_ALREADY_STOPPED"

	// Watcher errors
	ErrIDWatcherCreateFailed    = "SYNC_WATCHER_CREATE_FAILED"
	ErrIDWatcherCloseFailed     = "SYNC_WATCHER_CLOSE_FAILED"
	ErrIDWatcherAddPathFailed   = "SYNC_WATCHER_ADD_PATH_FAILED"

	// Debouncer errors
	ErrIDAdvancedDebouncerStopFailed = "SYNC_ADVANCED_DEBOUNCER_STOP_FAILED"
	ErrIDManualSyncFailed       = "SYNC_MANUAL_SYNC_FAILED"
	ErrIDDebouncerCallbackPanic  = "SYNC_DEBOUNCER_CALLBACK_PANIC"
	ErrIDDebouncerTimerCancelFailed = "SYNC_DEBOUNCER_TIMER_CANCEL_FAILED"

// Repository errors
	ErrIDRepoWatchRecursive    = "SYNC_REPO_WATCH_RECURSIVE_FAILED"

	// Configuration errors
	ErrIDConfigRequired         = "SYNC_CONFIG_REQUIRED"
	ErrIDConfigRepoPathRequired = "SYNC_CONFIG_REPO_PATH_REQUIRED"

	// General errors
	ErrIDShutdownError          = "SYNC_SHUTDOWN_ERROR"
	ErrIDMultipleShutdownErrors = "SYNC_MULTIPLE_SHUTDOWN_ERRORS"
	ErrIDReloadIgnorePatterns   = "SYNC_RELOAD_IGNORE_PATTERNS_FAILED"
	ErrIDResourceCleanupPanic   = "SYNC_RESOURCE_CLEANUP_PANIC"
	ErrIDShutdownTimeout        = "SYNC_SHUTDOWN_TIMEOUT"
)