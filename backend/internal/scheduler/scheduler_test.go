package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSchedule(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"daily", "daily", false},
		{"hourly", "hourly", false},
		{"weekly", "weekly", false},
		{"every 1h", "every 1h", false},
		{"every 30m", "every 30m", false},
		{"every 4h", "every 4h", false},
		{"cron midnight", "0 0 * * *", false},
		{"cron 2am", "0 2 * * *", false},
		{"cron specific", "30 14 1 * *", false},
		{"invalid cron", "60 2 * * *", true},    // minute > 59
		{"invalid interval", "every 30s", true}, // too short
		{"invalid format", "sometimes", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchedule(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScheduleNextRun(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		expr     string
		after    time.Time
		wantHour int
		wantMin  int
	}{
		{
			name:     "daily runs at 2am",
			expr:     "daily",
			after:    now,
			wantHour: 2,
			wantMin:  0,
		},
		{
			name:     "cron 3am",
			expr:     "0 3 * * *",
			after:    now,
			wantHour: 3,
			wantMin:  0,
		},
		{
			name:     "cron 3:30am",
			expr:     "30 3 * * *",
			after:    now,
			wantHour: 3,
			wantMin:  30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := ParseSchedule(tt.expr)
			require.NoError(t, err, "ParseSchedule failed")

			next := sched.NextRun(tt.after)
			assert.Equal(t, tt.wantHour, next.Hour())
			assert.Equal(t, tt.wantMin, next.Minute())
			assert.True(t, next.After(tt.after), "NextRun %s should be after %s", next, tt.after)
		})
	}
}

func TestScheduleInterval(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	sched, err := ParseSchedule("every 4h")
	require.NoError(t, err, "ParseSchedule failed")

	next := sched.NextRun(now)
	expected := now.Add(4 * time.Hour)
	assert.True(t, next.Equal(expected), "NextRun = %s, want %s", next, expected)
}

func TestSchedulerBackupCalled(t *testing.T) {
	called := make(chan struct{}, 1)
	backupFunc := func() error {
		called <- struct{}{}
		return nil
	}

	// Use a very short interval for testing
	sched := &Schedule{interval: 100 * time.Millisecond}
	s := NewScheduler(sched, backupFunc)

	s.Start()
	defer s.Stop()

	// Wait for backup to be called
	select {
	case <-called:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("Backup function was not called within timeout")
	}
}

func TestSchedulerStop(t *testing.T) {
	callCount := 0
	backupFunc := func() error {
		callCount++
		return nil
	}

	sched := &Schedule{interval: 50 * time.Millisecond}
	s := NewScheduler(sched, backupFunc)

	s.Start()
	time.Sleep(150 * time.Millisecond) // Let it run a couple times
	s.Stop()

	countAtStop := callCount
	time.Sleep(150 * time.Millisecond) // Wait more

	assert.Equal(t, countAtStop, callCount, "Scheduler continued after Stop")
}

func TestSchedulerStatus(t *testing.T) {
	backupFunc := func() error {
		return nil
	}

	sched := &Schedule{interval: 50 * time.Millisecond}
	s := NewScheduler(sched, backupFunc)

	// Before start
	lastRun, nextRun, lastErr := s.Status()
	assert.True(t, lastRun.IsZero(), "lastRun should be zero before any run")
	assert.Nil(t, lastErr, "lastErr should be nil before any run")
	assert.True(t, nextRun.IsZero(), "nextRun should be zero when not running")

	s.Start()
	time.Sleep(100 * time.Millisecond) // Let it run
	s.Stop()

	lastRun, _, lastErr = s.Status()
	assert.False(t, lastRun.IsZero(), "lastRun should be set after run")
	assert.Nil(t, lastErr, "lastErr should be nil after successful run")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30 seconds"},
		{5 * time.Minute, "5 minutes"},
		{2 * time.Hour, "2.0 hours"},
		{36 * time.Hour, "1.5 days"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatDuration(tt.d))
		})
	}
}
