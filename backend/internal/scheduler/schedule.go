package scheduler

import (
	"time"
)

// Schedule represents a backup schedule with parsed cron fields.
// Extracted from scheduler.go for better separation of concerns.
type Schedule struct {
	// Expression is the original schedule expression (cron or simple)
	Expression string

	// Parsed cron fields (-1 means "any")
	minute int // 0-59, -1 for any
	hour   int // 0-23, -1 for any
	dom    int // 1-31, -1 for any
	month  int // 1-12, -1 for any
	dow    int // 0-6 (Sunday=0), -1 for any

	// For interval-based schedules
	interval time.Duration

	// Enhanced cron fields for ranges/steps/lists support
	minuteField *CronField
	hourField   *CronField
	domField    *CronField
	monthField  *CronField
	dowField    *CronField
}

// IsInterval returns true if this is an interval-based schedule
func (s *Schedule) IsInterval() bool {
	return s.interval > 0
}

// IsCron returns true if this is a cron-based schedule
func (s *Schedule) IsCron() bool {
	return !s.IsInterval()
}

// Interval returns the interval duration (0 if not interval-based)
func (s *Schedule) Interval() time.Duration {
	return s.interval
}

// String returns the schedule expression
func (s *Schedule) String() string {
	return s.Expression
}

// matches checks if a given time matches this schedule
func (s *Schedule) matches(t time.Time) bool {
	// Use enhanced fields if available
	if s.minuteField != nil {
		return s.matchesEnhanced(t)
	}

	// Fall back to simple matching
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

// matchesEnhanced uses CronField for matching
func (s *Schedule) matchesEnhanced(t time.Time) bool {
	if s.minuteField != nil && !s.minuteField.Contains(t.Minute()) {
		return false
	}
	if s.hourField != nil && !s.hourField.Contains(t.Hour()) {
		return false
	}
	if s.domField != nil && !s.domField.Contains(t.Day()) {
		return false
	}
	if s.monthField != nil && !s.monthField.Contains(int(t.Month())) {
		return false
	}
	if s.dowField != nil && !s.dowField.Contains(int(t.Weekday())) {
		return false
	}
	return true
}

// NextRun calculates the next run time after 'after'.
// Uses efficient field-jumping for cron schedules instead of minute-by-minute search.
func (s *Schedule) NextRun(after time.Time) time.Time {
	// Interval-based: simple addition
	if s.interval > 0 {
		return after.Add(s.interval)
	}

	// Use efficient algorithm if enhanced fields are available
	if s.minuteField != nil {
		return s.nextRunEfficient(after)
	}

	// Fall back to linear search for simple schedules
	return s.nextRunLinear(after)
}

// nextRunLinear is the original O(minutes) algorithm
func (s *Schedule) nextRunLinear(after time.Time) time.Time {
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

// nextRunEfficient uses field-level jumps for O(fields) complexity
func (s *Schedule) nextRunEfficient(after time.Time) time.Time {
	// Start from next minute
	t := after.Add(time.Minute).Truncate(time.Minute)

	// Try up to 4 years (accounts for leap years)
	maxIterations := 365 * 4 * 24 * 60

	for i := 0; i < maxIterations; i++ {
		// Check month first (largest jump)
		if s.monthField != nil && !s.monthField.Any {
			month := int(t.Month())
			nextMonth := s.monthField.Next(month)
			if nextMonth == -1 {
				// Wrap to next year
				t = time.Date(t.Year()+1, time.Month(s.monthField.Values[0]), 1, 0, 0, 0, 0, t.Location())
				continue
			} else if nextMonth > month {
				t = time.Date(t.Year(), time.Month(nextMonth), 1, 0, 0, 0, 0, t.Location())
				continue
			}
		}

		// Check day of month
		if s.domField != nil && !s.domField.Any {
			day := t.Day()
			nextDay := s.domField.Next(day)
			daysInMonth := daysIn(t.Month(), t.Year())

			if nextDay == -1 || nextDay > daysInMonth {
				// Move to first valid day of next month
				t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
				continue
			} else if nextDay > day {
				t = time.Date(t.Year(), t.Month(), nextDay, 0, 0, 0, 0, t.Location())
				continue
			}
		}

		// Check day of week
		if s.dowField != nil && !s.dowField.Any {
			dow := int(t.Weekday())
			if !s.dowField.Contains(dow) {
				// Jump to next matching day of week
				nextDow := s.dowField.Next(dow)
				var daysToAdd int
				if nextDow == -1 {
					daysToAdd = (7 - dow) + s.dowField.Values[0]
				} else {
					daysToAdd = nextDow - dow
				}
				t = time.Date(t.Year(), t.Month(), t.Day()+daysToAdd, 0, 0, 0, 0, t.Location())
				continue
			}
		}

		// Check hour
		if s.hourField != nil && !s.hourField.Any {
			hour := t.Hour()
			nextHour := s.hourField.Next(hour)
			if nextHour == -1 {
				// Move to next day, first valid hour
				t = time.Date(t.Year(), t.Month(), t.Day()+1, s.hourField.Values[0], 0, 0, 0, t.Location())
				continue
			} else if nextHour > hour {
				t = time.Date(t.Year(), t.Month(), t.Day(), nextHour, 0, 0, 0, t.Location())
				continue
			}
		}

		// Check minute
		if s.minuteField != nil && !s.minuteField.Any {
			minute := t.Minute()
			nextMinute := s.minuteField.Next(minute)
			if nextMinute == -1 {
				// Move to next hour, first valid minute
				t = t.Add(time.Hour).Truncate(time.Hour)
				t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), s.minuteField.Values[0], 0, 0, t.Location())
				continue
			} else if nextMinute > minute {
				t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), nextMinute, 0, 0, t.Location())
				continue
			}
		}

		// All fields match
		if s.matchesEnhanced(t) {
			return t
		}

		// Increment by one minute and try again
		t = t.Add(time.Minute)
	}

	// Fallback
	return after.Add(24 * time.Hour)
}

// daysIn returns the number of days in a given month
func daysIn(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
