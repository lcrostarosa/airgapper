// Package scheduler handles scheduled backup operations
package scheduler

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Schedule represents a backup schedule
type Schedule struct {
	// Cron expression or simple interval (daily, hourly, etc.)
	Expression string

	// Parsed schedule
	minute   int  // 0-59, -1 for any
	hour     int  // 0-23, -1 for any
	dom      int  // 1-31, -1 for any
	month    int  // 1-12, -1 for any
	dow      int  // 0-6 (Sunday=0), -1 for any
	interval time.Duration // For simple intervals
}

// ParseSchedule parses a schedule expression
// Supports:
// - Simple: "hourly", "daily", "weekly"
// - Intervals: "every 4h", "every 30m"
// - Cron: "0 2 * * *" (minute hour dom month dow)
func ParseSchedule(expr string) (*Schedule, error) {
	expr = strings.TrimSpace(strings.ToLower(expr))
	s := &Schedule{Expression: expr}

	// Simple keywords
	switch expr {
	case "hourly":
		s.interval = time.Hour
		return s, nil
	case "daily":
		s.hour = 2 // 2 AM
		s.minute = 0
		s.dom = -1
		s.month = -1
		s.dow = -1
		return s, nil
	case "weekly":
		s.hour = 2
		s.minute = 0
		s.dom = -1
		s.month = -1
		s.dow = 0 // Sunday
		return s, nil
	}

	// Interval format: "every Xh", "every Xm"
	if strings.HasPrefix(expr, "every ") {
		intervalStr := strings.TrimPrefix(expr, "every ")
		dur, err := time.ParseDuration(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %s", intervalStr)
		}
		if dur < time.Minute {
			return nil, fmt.Errorf("interval must be at least 1 minute")
		}
		s.interval = dur
		return s, nil
	}

	// Cron format: "minute hour dom month dow"
	parts := strings.Fields(expr)
	if len(parts) == 5 {
		var err error
		s.minute, err = parseCronField(parts[0], 0, 59)
		if err != nil {
			return nil, fmt.Errorf("invalid minute: %w", err)
		}
		s.hour, err = parseCronField(parts[1], 0, 23)
		if err != nil {
			return nil, fmt.Errorf("invalid hour: %w", err)
		}
		s.dom, err = parseCronField(parts[2], 1, 31)
		if err != nil {
			return nil, fmt.Errorf("invalid day of month: %w", err)
		}
		s.month, err = parseCronField(parts[3], 1, 12)
		if err != nil {
			return nil, fmt.Errorf("invalid month: %w", err)
		}
		s.dow, err = parseCronField(parts[4], 0, 6)
		if err != nil {
			return nil, fmt.Errorf("invalid day of week: %w", err)
		}
		return s, nil
	}

	return nil, fmt.Errorf("unrecognized schedule format: %s", expr)
}

func parseCronField(field string, min, max int) (int, error) {
	if field == "*" {
		return -1, nil
	}
	val, err := strconv.Atoi(field)
	if err != nil {
		return 0, err
	}
	if val < min || val > max {
		return 0, fmt.Errorf("value %d out of range [%d, %d]", val, min, max)
	}
	return val, nil
}

// NextRun calculates the next run time after 'after'
func (s *Schedule) NextRun(after time.Time) time.Time {
	// Interval-based
	if s.interval > 0 {
		return after.Add(s.interval)
	}

	// Cron-based - find next matching time
	next := after.Add(time.Minute).Truncate(time.Minute)

	// Search up to 1 year ahead
	maxSearch := after.Add(365 * 24 * time.Hour)
	for next.Before(maxSearch) {
		if s.matches(next) {
			return next
		}
		next = next.Add(time.Minute)
	}

	// Fallback to 24h if no match found
	return after.Add(24 * time.Hour)
}

func (s *Schedule) matches(t time.Time) bool {
	if s.minute != -1 && t.Minute() != s.minute {
		return false
	}
	if s.hour != -1 && t.Hour() != s.hour {
		return false
	}
	if s.dom != -1 && t.Day() != s.dom {
		return false
	}
	if s.month != -1 && int(t.Month()) != s.month {
		return false
	}
	if s.dow != -1 && int(t.Weekday()) != s.dow {
		return false
	}
	return true
}

// Scheduler runs scheduled backups
type Scheduler struct {
	schedule   *Schedule
	backupFunc func() error
	paths      []string
	stop       chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	running    bool
	lastRun    time.Time
	lastError  error
}

// NewScheduler creates a new scheduler
func NewScheduler(schedule *Schedule, backupFunc func() error) *Scheduler {
	return &Scheduler{
		schedule:   schedule,
		backupFunc: backupFunc,
		stop:       make(chan struct{}),
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

func (s *Scheduler) run() {
	defer s.wg.Done()

	// Calculate first run
	now := time.Now()
	nextRun := s.schedule.NextRun(now)

	log.Printf("Scheduler started. Next backup at %s", nextRun.Format("2006-01-02 15:04:05"))

	for {
		waitDuration := time.Until(nextRun)
		if waitDuration < 0 {
			waitDuration = time.Second
		}

		select {
		case <-s.stop:
			log.Println("Scheduler stopped")
			return
		case <-time.After(waitDuration):
			log.Println("Running scheduled backup...")

			err := s.backupFunc()

			s.mu.Lock()
			s.lastRun = time.Now()
			s.lastError = err
			s.mu.Unlock()

			if err != nil {
				log.Printf("Scheduled backup failed: %v", err)
			} else {
				log.Println("Scheduled backup completed successfully")
			}

			nextRun = s.schedule.NextRun(time.Now())
			log.Printf("Next backup at %s", nextRun.Format("2006-01-02 15:04:05"))
		}
	}
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
