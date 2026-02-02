package scheduler

// SchedulerCallbacks provides hooks for backup lifecycle events.
// All callbacks are optional - nil callbacks are simply not called.
type SchedulerCallbacks struct {
	// OnBackupStart is called when a backup attempt begins
	OnBackupStart func(result *BackupResult)

	// OnBackupSuccess is called when a backup completes successfully
	OnBackupSuccess func(result *BackupResult)

	// OnBackupFailure is called when a backup attempt fails
	// willRetry indicates if another attempt will be made
	OnBackupFailure func(result *BackupResult)

	// OnRetryExhausted is called when all retry attempts have failed
	// results contains all failed attempts
	OnRetryExhausted func(results []*BackupResult)

	// OnScheduleChange is called when the schedule is updated via hot-reload
	OnScheduleChange func(old, new *Schedule)
}

// DefaultCallbacks returns empty callbacks (logging only, no notifications)
func DefaultCallbacks() *SchedulerCallbacks {
	return &SchedulerCallbacks{}
}

// callOnBackupStart safely calls the OnBackupStart callback if set
func (c *SchedulerCallbacks) callOnBackupStart(result *BackupResult) {
	if c != nil && c.OnBackupStart != nil {
		c.OnBackupStart(result)
	}
}

// callOnBackupSuccess safely calls the OnBackupSuccess callback if set
func (c *SchedulerCallbacks) callOnBackupSuccess(result *BackupResult) {
	if c != nil && c.OnBackupSuccess != nil {
		c.OnBackupSuccess(result)
	}
}

// callOnBackupFailure safely calls the OnBackupFailure callback if set
func (c *SchedulerCallbacks) callOnBackupFailure(result *BackupResult) {
	if c != nil && c.OnBackupFailure != nil {
		c.OnBackupFailure(result)
	}
}

// callOnRetryExhausted safely calls the OnRetryExhausted callback if set
func (c *SchedulerCallbacks) callOnRetryExhausted(results []*BackupResult) {
	if c != nil && c.OnRetryExhausted != nil {
		c.OnRetryExhausted(results)
	}
}

// callOnScheduleChange safely calls the OnScheduleChange callback if set
func (c *SchedulerCallbacks) callOnScheduleChange(old, new *Schedule) {
	if c != nil && c.OnScheduleChange != nil {
		c.OnScheduleChange(old, new)
	}
}

// LoggingCallbacks returns callbacks that log events (for debugging)
func LoggingCallbacks(logf func(format string, args ...interface{})) *SchedulerCallbacks {
	return &SchedulerCallbacks{
		OnBackupStart: func(result *BackupResult) {
			logf("Backup starting (attempt %d)", result.Attempt)
		},
		OnBackupSuccess: func(result *BackupResult) {
			logf("Backup succeeded in %v (attempt %d)", result.Duration(), result.Attempt)
		},
		OnBackupFailure: func(result *BackupResult) {
			if result.WillRetry {
				logf("Backup failed (attempt %d), will retry: %v", result.Attempt, result.Error)
			} else {
				logf("Backup failed (attempt %d): %v", result.Attempt, result.Error)
			}
		},
		OnRetryExhausted: func(results []*BackupResult) {
			logf("All %d backup attempts failed", len(results))
		},
		OnScheduleChange: func(old, new *Schedule) {
			logf("Schedule changed from %q to %q", old.Expression, new.Expression)
		},
	}
}

// ChainCallbacks combines multiple callback handlers
func ChainCallbacks(callbacks ...*SchedulerCallbacks) *SchedulerCallbacks {
	return &SchedulerCallbacks{
		OnBackupStart: func(result *BackupResult) {
			for _, c := range callbacks {
				c.callOnBackupStart(result)
			}
		},
		OnBackupSuccess: func(result *BackupResult) {
			for _, c := range callbacks {
				c.callOnBackupSuccess(result)
			}
		},
		OnBackupFailure: func(result *BackupResult) {
			for _, c := range callbacks {
				c.callOnBackupFailure(result)
			}
		},
		OnRetryExhausted: func(results []*BackupResult) {
			for _, c := range callbacks {
				c.callOnRetryExhausted(results)
			}
		},
		OnScheduleChange: func(old, new *Schedule) {
			for _, c := range callbacks {
				c.callOnScheduleChange(old, new)
			}
		},
	}
}
