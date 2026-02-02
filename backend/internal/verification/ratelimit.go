package verification

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RateLimitConfig configures rate limiting for destructive operations.
type RateLimitConfig struct {
	Enabled bool `json:"enabled"`

	// Deletion limits
	MaxDeletionsPerHour int `json:"max_deletions_per_hour"` // Max files deleted per hour
	MaxDeletionsPerDay  int `json:"max_deletions_per_day"`  // Max files deleted per day
	MaxBytesPerHour     int64 `json:"max_bytes_per_hour"`   // Max bytes deleted per hour
	MaxBytesPerDay      int64 `json:"max_bytes_per_day"`    // Max bytes deleted per day

	// Snapshot-specific limits
	MaxSnapshotsPerDay int `json:"max_snapshots_per_day"` // Max snapshots deleted per day

	// Burst allowance
	BurstMultiplier float64 `json:"burst_multiplier"` // Allow burst up to this multiple of hourly limit

	// Lockout configuration
	LockoutDurationMinutes int `json:"lockout_duration_minutes"` // How long to lock after limit exceeded
}

// DefaultRateLimitConfig returns sensible defaults.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:                true,
		MaxDeletionsPerHour:    100,
		MaxDeletionsPerDay:     500,
		MaxBytesPerHour:        10 * 1024 * 1024 * 1024, // 10 GB per hour
		MaxBytesPerDay:         50 * 1024 * 1024 * 1024, // 50 GB per day
		MaxSnapshotsPerDay:     10,
		BurstMultiplier:        2.0,
		LockoutDurationMinutes: 60,
	}
}

// DeletionRecord tracks a single deletion for rate limiting.
type DeletionRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	Path       string    `json:"path"`
	Bytes      int64     `json:"bytes"`
	IsSnapshot bool      `json:"is_snapshot"`
}

// RateLimitState tracks current rate limit state.
type RateLimitState struct {
	Records       []DeletionRecord `json:"records"`
	LockedUntil   *time.Time       `json:"locked_until,omitempty"`
	LockReason    string           `json:"lock_reason,omitempty"`
	TotalDeleted  int64            `json:"total_deleted"`
	TotalBytes    int64            `json:"total_bytes"`
}

// RateLimiter enforces rate limits on destructive operations.
type RateLimiter struct {
	basePath string
	config   *RateLimitConfig

	mu    sync.RWMutex
	state *RateLimitState
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(basePath string, config *RateLimitConfig) (*RateLimiter, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if config == nil {
		config = DefaultRateLimitConfig()
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create rate limit directory: %w", err)
	}

	rl := &RateLimiter{
		basePath: basePath,
		config:   config,
		state:    &RateLimitState{Records: []DeletionRecord{}},
	}

	if err := rl.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load rate limit state: %w", err)
	}

	return rl, nil
}

func (rl *RateLimiter) statePath() string {
	return filepath.Join(rl.basePath, "ratelimit-state.json")
}

func (rl *RateLimiter) load() error {
	data, err := os.ReadFile(rl.statePath())
	if err != nil {
		return err
	}

	var state RateLimitState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state: %w", err)
	}

	rl.state = &state
	return nil
}

func (rl *RateLimiter) save() error {
	data, err := json.MarshalIndent(rl.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return os.WriteFile(rl.statePath(), data, 0600)
}

// RateLimitResult contains the result of a rate limit check.
type RateLimitResult struct {
	Allowed         bool          `json:"allowed"`
	Reason          string        `json:"reason,omitempty"`
	RetryAfter      time.Duration `json:"retry_after,omitempty"`
	HourlyUsed      int           `json:"hourly_used"`
	HourlyLimit     int           `json:"hourly_limit"`
	DailyUsed       int           `json:"daily_used"`
	DailyLimit      int           `json:"daily_limit"`
	BytesHourlyUsed int64         `json:"bytes_hourly_used"`
	BytesDailyUsed  int64         `json:"bytes_daily_used"`
}

// CheckDelete checks if a deletion is allowed under rate limits.
func (rl *RateLimiter) CheckDelete(path string, bytes int64, isSnapshot bool) *RateLimitResult {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	result := &RateLimitResult{
		Allowed:     true,
		HourlyLimit: rl.config.MaxDeletionsPerHour,
		DailyLimit:  rl.config.MaxDeletionsPerDay,
	}

	if !rl.config.Enabled {
		return result
	}

	now := time.Now()

	// Check if we're in lockout
	if rl.state.LockedUntil != nil && now.Before(*rl.state.LockedUntil) {
		result.Allowed = false
		result.Reason = fmt.Sprintf("rate limit lockout: %s", rl.state.LockReason)
		result.RetryAfter = time.Until(*rl.state.LockedUntil)
		return result
	}

	// Clean old records and count recent activity
	hourAgo := now.Add(-time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	var hourlyCount, dailyCount int
	var hourlyBytes, dailyBytes int64
	var snapshotsToday int

	for _, r := range rl.state.Records {
		if r.Timestamp.After(dayAgo) {
			dailyCount++
			dailyBytes += r.Bytes
			if r.IsSnapshot {
				snapshotsToday++
			}

			if r.Timestamp.After(hourAgo) {
				hourlyCount++
				hourlyBytes += r.Bytes
			}
		}
	}

	result.HourlyUsed = hourlyCount
	result.DailyUsed = dailyCount
	result.BytesHourlyUsed = hourlyBytes
	result.BytesDailyUsed = dailyBytes

	// Check hourly file count limit (with burst allowance)
	burstLimit := int(float64(rl.config.MaxDeletionsPerHour) * rl.config.BurstMultiplier)
	if hourlyCount >= burstLimit {
		result.Allowed = false
		result.Reason = fmt.Sprintf("hourly deletion limit exceeded (%d/%d)", hourlyCount, rl.config.MaxDeletionsPerHour)
		result.RetryAfter = time.Until(rl.oldestRecordInWindow(hourAgo).Add(time.Hour))
		return result
	}

	// Check daily file count limit
	if dailyCount >= rl.config.MaxDeletionsPerDay {
		result.Allowed = false
		result.Reason = fmt.Sprintf("daily deletion limit exceeded (%d/%d)", dailyCount, rl.config.MaxDeletionsPerDay)
		result.RetryAfter = time.Until(rl.oldestRecordInWindow(dayAgo).Add(24 * time.Hour))
		return result
	}

	// Check hourly bytes limit
	if rl.config.MaxBytesPerHour > 0 && hourlyBytes+bytes > rl.config.MaxBytesPerHour {
		result.Allowed = false
		result.Reason = fmt.Sprintf("hourly bytes limit exceeded (%d/%d bytes)", hourlyBytes, rl.config.MaxBytesPerHour)
		result.RetryAfter = time.Until(rl.oldestRecordInWindow(hourAgo).Add(time.Hour))
		return result
	}

	// Check daily bytes limit
	if rl.config.MaxBytesPerDay > 0 && dailyBytes+bytes > rl.config.MaxBytesPerDay {
		result.Allowed = false
		result.Reason = fmt.Sprintf("daily bytes limit exceeded (%d/%d bytes)", dailyBytes, rl.config.MaxBytesPerDay)
		result.RetryAfter = time.Until(rl.oldestRecordInWindow(dayAgo).Add(24 * time.Hour))
		return result
	}

	// Check snapshot-specific limit
	if isSnapshot && snapshotsToday >= rl.config.MaxSnapshotsPerDay {
		result.Allowed = false
		result.Reason = fmt.Sprintf("daily snapshot deletion limit exceeded (%d/%d)", snapshotsToday, rl.config.MaxSnapshotsPerDay)
		result.RetryAfter = time.Until(rl.oldestRecordInWindow(dayAgo).Add(24 * time.Hour))
		return result
	}

	return result
}

func (rl *RateLimiter) oldestRecordInWindow(windowStart time.Time) time.Time {
	oldest := time.Now()
	for _, r := range rl.state.Records {
		if r.Timestamp.After(windowStart) && r.Timestamp.Before(oldest) {
			oldest = r.Timestamp
		}
	}
	return oldest
}

// RecordDelete records a deletion for rate limiting.
func (rl *RateLimiter) RecordDelete(path string, bytes int64, isSnapshot bool) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	record := DeletionRecord{
		Timestamp:  time.Now(),
		Path:       path,
		Bytes:      bytes,
		IsSnapshot: isSnapshot,
	}

	rl.state.Records = append(rl.state.Records, record)
	rl.state.TotalDeleted++
	rl.state.TotalBytes += bytes

	// Clean old records (keep last 7 days)
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	var newRecords []DeletionRecord
	for _, r := range rl.state.Records {
		if r.Timestamp.After(weekAgo) {
			newRecords = append(newRecords, r)
		}
	}
	rl.state.Records = newRecords

	return rl.save()
}

// TriggerLockout manually triggers a lockout (e.g., when anomaly detected).
func (rl *RateLimiter) TriggerLockout(reason string, duration time.Duration) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	lockUntil := time.Now().Add(duration)
	rl.state.LockedUntil = &lockUntil
	rl.state.LockReason = reason

	return rl.save()
}

// ClearLockout removes an active lockout.
func (rl *RateLimiter) ClearLockout() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.state.LockedUntil = nil
	rl.state.LockReason = ""

	return rl.save()
}

// GetState returns the current rate limit state.
func (rl *RateLimiter) GetState() *RateLimitState {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// Return a copy
	stateCopy := *rl.state
	stateCopy.Records = make([]DeletionRecord, len(rl.state.Records))
	copy(stateCopy.Records, rl.state.Records)

	return &stateCopy
}

// IsLocked returns true if currently in lockout.
func (rl *RateLimiter) IsLocked() bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return rl.state.LockedUntil != nil && time.Now().Before(*rl.state.LockedUntil)
}
