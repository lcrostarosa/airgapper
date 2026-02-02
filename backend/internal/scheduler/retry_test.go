package scheduler

import (
	"testing"
	"time"
)

func TestDefaultRetryStrategy(t *testing.T) {
	r := DefaultRetryStrategy()

	if r.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", r.MaxRetries)
	}
	if r.InitialDelay != 5*time.Minute {
		t.Errorf("expected InitialDelay=5m, got %v", r.InitialDelay)
	}
	if r.MaxDelay != 1*time.Hour {
		t.Errorf("expected MaxDelay=1h, got %v", r.MaxDelay)
	}
	if r.BackoffFactor != 2.0 {
		t.Errorf("expected BackoffFactor=2.0, got %f", r.BackoffFactor)
	}
}

func TestNoRetry(t *testing.T) {
	r := NoRetry()

	if r.MaxRetries != 0 {
		t.Errorf("expected MaxRetries=0, got %d", r.MaxRetries)
	}
	if r.ShouldRetry(1) {
		t.Error("NoRetry should never allow retries")
	}
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
		{1, 1 * time.Minute},  // 1 * 2^0 = 1
		{2, 2 * time.Minute},  // 1 * 2^1 = 2
		{3, 4 * time.Minute},  // 1 * 2^2 = 4
		{0, 0},                // invalid attempt
		{4, 0},                // beyond max retries
	}

	for _, tt := range tests {
		result := r.NextDelay(tt.attempt)
		if result != tt.expected {
			t.Errorf("NextDelay(%d) = %v, want %v", tt.attempt, result, tt.expected)
		}
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
	if result != 30*time.Minute {
		t.Errorf("expected 30m (capped), got %v", result)
	}
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
		if result != tt.expected {
			t.Errorf("ShouldRetry(%d) = %v, want %v", tt.attempt, result, tt.expected)
		}
	}
}

func TestRetryStrategy_ShouldRetry_Nil(t *testing.T) {
	var r *RetryStrategy

	if r.ShouldRetry(1) {
		t.Error("nil RetryStrategy should never allow retries")
	}
}

func TestBackupResult_Duration(t *testing.T) {
	r := &BackupResult{
		StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 1, 1, 10, 5, 30, 0, time.UTC),
	}

	if r.Duration() != 5*time.Minute+30*time.Second {
		t.Errorf("expected 5m30s, got %v", r.Duration())
	}
}

func TestBackupResult_IsRetry(t *testing.T) {
	r1 := &BackupResult{Attempt: 1}
	if r1.IsRetry() {
		t.Error("Attempt 1 should not be a retry")
	}

	r2 := &BackupResult{Attempt: 2}
	if !r2.IsRetry() {
		t.Error("Attempt 2 should be a retry")
	}
}
