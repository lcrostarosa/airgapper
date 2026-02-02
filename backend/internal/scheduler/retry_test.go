package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultRetryStrategy(t *testing.T) {
	r := DefaultRetryStrategy()

	assert.Equal(t, 3, r.MaxRetries)
	assert.Equal(t, 5*time.Minute, r.InitialDelay)
	assert.Equal(t, 1*time.Hour, r.MaxDelay)
	assert.Equal(t, 2.0, r.BackoffFactor)
}

func TestNoRetry(t *testing.T) {
	r := NoRetry()

	assert.Equal(t, 0, r.MaxRetries)
	assert.False(t, r.ShouldRetry(1), "NoRetry should never allow retries")
}

func TestRetryStrategy_NextDelay(t *testing.T) {
	r := &RetryStrategy{
		MaxRetries:    3,
		InitialDelay:  1 * time.Minute,
		MaxDelay:      1 * time.Hour,
		BackoffFactor: 2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Minute}, // 1 * 2^0 = 1
		{2, 2 * time.Minute}, // 1 * 2^1 = 2
		{3, 4 * time.Minute}, // 1 * 2^2 = 4
		{0, 0},               // invalid attempt
		{4, 0},               // beyond max retries
	}

	for _, tt := range tests {
		result := r.NextDelay(tt.attempt)
		assert.Equal(t, tt.expected, result, "NextDelay(%d)", tt.attempt)
	}
}

func TestRetryStrategy_NextDelay_CapsAtMax(t *testing.T) {
	r := &RetryStrategy{
		MaxRetries:    10,
		InitialDelay:  10 * time.Minute,
		MaxDelay:      30 * time.Minute,
		BackoffFactor: 2.0,
	}

	// Attempt 3: 10 * 2^2 = 40 minutes, should cap at 30
	result := r.NextDelay(3)
	assert.Equal(t, 30*time.Minute, result, "expected 30m (capped)")
}

func TestRetryStrategy_ShouldRetry(t *testing.T) {
	r := &RetryStrategy{MaxRetries: 3}

	tests := []struct {
		attempt  int
		expected bool
	}{
		{1, true},  // can retry after 1st attempt
		{2, true},  // can retry after 2nd attempt
		{3, false}, // no more retries after 3rd attempt
		{4, false}, // definitely no more
	}

	for _, tt := range tests {
		result := r.ShouldRetry(tt.attempt)
		assert.Equal(t, tt.expected, result, "ShouldRetry(%d)", tt.attempt)
	}
}

func TestRetryStrategy_ShouldRetry_Nil(t *testing.T) {
	var r *RetryStrategy
	assert.False(t, r.ShouldRetry(1), "nil RetryStrategy should never allow retries")
}

func TestBackupResult_Duration(t *testing.T) {
	r := &BackupResult{
		StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 1, 1, 10, 5, 30, 0, time.UTC),
	}

	assert.Equal(t, 5*time.Minute+30*time.Second, r.Duration())
}

func TestBackupResult_IsRetry(t *testing.T) {
	r1 := &BackupResult{Attempt: 1}
	assert.False(t, r1.IsRetry(), "Attempt 1 should not be a retry")

	r2 := &BackupResult{Attempt: 2}
	assert.True(t, r2.IsRetry(), "Attempt 2 should be a retry")
}
