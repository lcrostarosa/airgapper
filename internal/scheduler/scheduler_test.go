package scheduler

import (
	"testing"
	"time"
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
		{"invalid cron", "60 2 * * *", true},      // minute > 59
		{"invalid interval", "every 30s", true},   // too short
		{"invalid format", "sometimes", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchedule(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSchedule(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
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
			if err != nil {
				t.Fatalf("ParseSchedule failed: %v", err)
			}

			next := sched.NextRun(tt.after)
			if next.Hour() != tt.wantHour || next.Minute() != tt.wantMin {
				t.Errorf("NextRun = %s, want hour=%d min=%d", next.Format("15:04"), tt.wantHour, tt.wantMin)
			}
			if !next.After(tt.after) {
				t.Errorf("NextRun %s should be after %s", next, tt.after)
			}
		})
	}
}

func TestScheduleInterval(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	sched, err := ParseSchedule("every 4h")
	if err != nil {
		t.Fatalf("ParseSchedule failed: %v", err)
	}

	next := sched.NextRun(now)
	expected := now.Add(4 * time.Hour)
	if !next.Equal(expected) {
		t.Errorf("NextRun = %s, want %s", next, expected)
	}
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

	if callCount != countAtStop {
		t.Errorf("Scheduler continued after Stop: count went from %d to %d", countAtStop, callCount)
	}
}

func TestSchedulerStatus(t *testing.T) {
	backupFunc := func() error {
		return nil
	}

	sched := &Schedule{interval: 50 * time.Millisecond}
	s := NewScheduler(sched, backupFunc)

	// Before start
	lastRun, lastErr, nextRun := s.Status()
	if !lastRun.IsZero() {
		t.Error("lastRun should be zero before any run")
	}
	if lastErr != nil {
		t.Error("lastErr should be nil before any run")
	}
	if !nextRun.IsZero() {
		t.Error("nextRun should be zero when not running")
	}

	s.Start()
	time.Sleep(100 * time.Millisecond) // Let it run
	s.Stop()

	lastRun, lastErr, _ = s.Status()
	if lastRun.IsZero() {
		t.Error("lastRun should be set after run")
	}
	if lastErr != nil {
		t.Error("lastErr should be nil after successful run")
	}
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
			got := FormatDuration(tt.d)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
