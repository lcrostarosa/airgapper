package scheduler

import (
	"math"
	"time"
)

// RetryStrategy defines the retry behavior for failed backups
type RetryStrategy struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries)
	MaxRetries int
	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// BackoffFactor is the multiplier applied to delay after each attempt
	BackoffFactor float64
}

// DefaultRetryStrategy returns a sensible default retry configuration
func DefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:    3,
		InitialDelay:  5 * time.Minute,
		MaxDelay:      1 * time.Hour,
		BackoffFactor: 2.0,
	}
}

// NoRetry returns a strategy that never retries
func NoRetry() *RetryStrategy {
	return &RetryStrategy{
		MaxRetries: 0,
	}
}

// NextDelay calculates the delay before the next retry attempt.
// attempt is 1-indexed (1 = first retry)
func (r *RetryStrategy) NextDelay(attempt int) time.Duration {
	if attempt < 1 || attempt > r.MaxRetries {
		return 0
	}

	// Calculate delay with exponential backoff
	delay := float64(r.InitialDelay) * math.Pow(r.BackoffFactor, float64(attempt-1))

	// Cap at max delay
	if time.Duration(delay) > r.MaxDelay {
		return r.MaxDelay
	}

	return time.Duration(delay)
}

// ShouldRetry returns true if another retry should be attempted
func (r *RetryStrategy) ShouldRetry(attempt int) bool {
	return r != nil && attempt < r.MaxRetries
}

// BackupResult holds the result of a backup attempt
type BackupResult struct {
	// ScheduledTime is when the backup was originally scheduled
	ScheduledTime time.Time
	// StartTime is when this attempt started
	StartTime time.Time
	// EndTime is when this attempt completed
	EndTime time.Time
	// Success indicates if the backup succeeded
	Success bool
	// Error holds any error that occurred
	Error error
	// Attempt is the attempt number (1 = first attempt, 2+ = retries)
	Attempt int
	// WillRetry indicates if another retry will be attempted
	WillRetry bool
}

// Duration returns how long the backup took
func (r *BackupResult) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// IsRetry returns true if this was a retry attempt
func (r *BackupResult) IsRetry() bool {
	return r.Attempt > 1
}
