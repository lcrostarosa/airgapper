// Package scheduler handles scheduled backup operations
package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// ParseSchedule parses a schedule expression.
// Supports:
// - Simple: "hourly", "daily", "weekly"
// - Intervals: "every 4h", "every 30m"
// - Cron (simple): "0 2 * * *" (single values only)
// - Cron (enhanced): "0-30 2 * * *" (ranges, steps, lists)
//
// This is an alias for ParseScheduleEnhanced which provides full cron syntax support.
func ParseSchedule(expr string) (*Schedule, error) {
	return ParseScheduleEnhanced(expr)
}

// SchedulerConfig holds configuration for creating a new scheduler
type SchedulerConfig struct {
	// Schedule is the backup schedule
	Schedule *Schedule
	// BackupFunc is the function to call when running backups
	BackupFunc func() error
	// Retry configures retry behavior (nil = no retries, backward compatible)
	Retry *RetryStrategy
	// Callbacks hooks for backup lifecycle events (nil = logging only)
	Callbacks *SchedulerCallbacks
}

// Scheduler runs scheduled backups
type Scheduler struct {
	schedule   *Schedule
	backupFunc func() error
	retry      *RetryStrategy
	callbacks  *SchedulerCallbacks
	paths      []string
	stop       chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
	lastRun    time.Time
	lastError  error
	history    []*BackupResult
	historyMax int
}

// NewScheduler creates a new scheduler.
// This is the backward-compatible constructor.
func NewScheduler(schedule *Schedule, backupFunc func() error) *Scheduler {
	return &Scheduler{
		schedule:   schedule,
		backupFunc: backupFunc,
		stop:       make(chan struct{}),
		historyMax: 100,
	}
}

// NewSchedulerWithConfig creates a scheduler with full configuration.
func NewSchedulerWithConfig(config SchedulerConfig) *Scheduler {
	return &Scheduler{
		schedule:   config.Schedule,
		backupFunc: config.BackupFunc,
		retry:      config.Retry,
		callbacks:  config.Callbacks,
		stop:       make(chan struct{}),
		historyMax: 100,
	}
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stop)
	s.wg.Wait()
}

// Status returns scheduler status
func (s *Scheduler) Status() (lastRun time.Time, lastError error, nextRun time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastRun = s.lastRun
	lastError = s.lastError
	if s.running {
		if lastRun.IsZero() {
			nextRun = s.schedule.NextRun(time.Now())
		} else {
			nextRun = s.schedule.NextRun(lastRun)
		}
	}
	return
}

// GetHistory returns recent backup results
func (s *Scheduler) GetHistory(limit int) []*BackupResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}

	// Return most recent entries
	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*BackupResult, limit)
	copy(result, s.history[start:])
	return result
}

// UpdateSchedule changes the schedule without stopping the scheduler (hot-reload)
func (s *Scheduler) UpdateSchedule(schedule *Schedule) {
	s.mu.Lock()
	oldSchedule := s.schedule
	s.schedule = schedule
	s.mu.Unlock()

	if s.callbacks != nil {
		s.callbacks.callOnScheduleChange(oldSchedule, schedule)
	}

	logging.Infof("Schedule updated from %q to %q", oldSchedule.Expression, schedule.Expression)
}

// GetSchedule returns the current schedule
func (s *Scheduler) GetSchedule() *Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.schedule
}

func (s *Scheduler) run() {
	defer s.wg.Done()

	// Calculate first run
	now := time.Now()
	nextRun := s.schedule.NextRun(now)

	logging.Infof("Scheduler started. Next backup at %s", nextRun.Format("2006-01-02 15:04:05"))

	for {
		waitDuration := time.Until(nextRun)
		if waitDuration < 0 {
			waitDuration = time.Second
		}

		select {
		case <-s.stop:
			logging.Info("Scheduler stopped")
			return
		case <-time.After(waitDuration):
			s.runBackupWithRetry(nextRun)

			// Get the schedule (may have been updated)
			s.mu.Lock()
			schedule := s.schedule
			s.mu.Unlock()

			nextRun = schedule.NextRun(time.Now())
			logging.Infof("Next backup at %s", nextRun.Format("2006-01-02 15:04:05"))
		}
	}
}

func (s *Scheduler) runBackupWithRetry(scheduledTime time.Time) {
	var results []*BackupResult
	maxAttempts := 1

	if s.retry != nil && s.retry.MaxRetries > 0 {
		maxAttempts = s.retry.MaxRetries + 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result := s.runSingleBackup(scheduledTime, attempt, maxAttempts)
		results = append(results, result)

		// Record in history
		s.mu.Lock()
		s.history = append(s.history, result)
		if len(s.history) > s.historyMax {
			s.history = s.history[1:]
		}
		s.lastRun = result.EndTime
		s.lastError = result.Error
		s.mu.Unlock()

		if result.Success {
			return
		}

		if !result.WillRetry {
			break
		}

		// Wait before retry
		delay := s.retry.NextDelay(attempt)
		logging.Infof("Retrying backup in %v (attempt %d/%d)", delay, attempt+1, maxAttempts)

		select {
		case <-s.stop:
			return
		case <-time.After(delay):
		}
	}

	// All attempts exhausted
	if len(results) > 1 && s.callbacks != nil {
		s.callbacks.callOnRetryExhausted(results)
	}
}

func (s *Scheduler) runSingleBackup(scheduledTime time.Time, attempt, maxAttempts int) *BackupResult {
	result := &BackupResult{
		ScheduledTime: scheduledTime,
		StartTime:     time.Now(),
		Attempt:       attempt,
	}

	// Notify start
	if s.callbacks != nil {
		s.callbacks.callOnBackupStart(result)
	}

	logging.Info("Running scheduled backup...")

	// Run backup
	err := s.backupFunc()
	result.EndTime = time.Now()
	result.Error = err
	result.Success = err == nil
	result.WillRetry = !result.Success && s.retry != nil && s.retry.ShouldRetry(attempt)

	// Notify completion
	if result.Success {
		if s.callbacks != nil {
			s.callbacks.callOnBackupSuccess(result)
		}
		logging.Info("Scheduled backup completed successfully")
	} else {
		if s.callbacks != nil {
			s.callbacks.callOnBackupFailure(result)
		}
		logging.Errorf("Scheduled backup failed: %v", err)
	}

	return result
}

// FormatDuration formats a duration nicely
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}
