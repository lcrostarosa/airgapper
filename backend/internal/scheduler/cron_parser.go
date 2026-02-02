package scheduler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CronField represents a parsed cron field that can contain:
// - Single values: "5"
// - Ranges: "1-5"
// - Steps: "*/15" or "1-30/5"
// - Lists: "1,5,10,15"
// - Any: "*"
type CronField struct {
	// Values contains all allowed values (sorted)
	Values []int
	// Any is true if the field matches any value ("*")
	Any bool
}

// Contains checks if a value is allowed by this field
func (f *CronField) Contains(val int) bool {
	if f.Any {
		return true
	}
	// Binary search since Values is sorted
	i := sort.SearchInts(f.Values, val)
	return i < len(f.Values) && f.Values[i] == val
}

// Next returns the next valid value >= val, or -1 if none exists
func (f *CronField) Next(val int) int {
	if f.Any {
		return val
	}
	// Binary search for first value >= val
	i := sort.SearchInts(f.Values, val)
	if i < len(f.Values) {
		return f.Values[i]
	}
	return -1 // Wrap around needed
}

// First returns the smallest valid value
func (f *CronField) First() int {
	if f.Any || len(f.Values) == 0 {
		return 0
	}
	return f.Values[0]
}

// ParseCronField parses a single cron field with full syntax support.
// Supports:
//   - "*" (any)
//   - "5" (single value)
//   - "1-10" (range)
//   - "*/5" (step from min)
//   - "1-30/5" (range with step)
//   - "1,5,10" (list)
//   - "1-5,10-15,20" (mixed)
func ParseCronField(field string, min, max int) (*CronField, error) {
	field = strings.TrimSpace(field)

	if field == "*" {
		return &CronField{Any: true}, nil
	}

	cf := &CronField{}
	valuesSet := make(map[int]bool)

	// Split by comma for lists
	parts := strings.Split(field, ",")
	for _, part := range parts {
		values, err := parseFieldPart(part, min, max)
		if err != nil {
			return nil, err
		}
		for _, v := range values {
			valuesSet[v] = true
		}
	}

	// Convert set to sorted slice
	cf.Values = make([]int, 0, len(valuesSet))
	for v := range valuesSet {
		cf.Values = append(cf.Values, v)
	}
	sort.Ints(cf.Values)

	if len(cf.Values) == 0 {
		return nil, fmt.Errorf("no valid values in field: %s", field)
	}

	return cf, nil
}

// parseFieldPart handles a single part (no commas) of a cron field
func parseFieldPart(part string, min, max int) ([]int, error) {
	part = strings.TrimSpace(part)

	// Check for step
	var stepStr string
	if idx := strings.Index(part, "/"); idx != -1 {
		stepStr = part[idx+1:]
		part = part[:idx]
	}

	// Parse the range or single value
	var rangeStart, rangeEnd int
	var err error

	if part == "*" {
		rangeStart = min
		rangeEnd = max
	} else if idx := strings.Index(part, "-"); idx != -1 {
		// Range: "1-10"
		rangeStart, err = strconv.Atoi(strings.TrimSpace(part[:idx]))
		if err != nil {
			return nil, fmt.Errorf("invalid range start: %s", part[:idx])
		}
		rangeEnd, err = strconv.Atoi(strings.TrimSpace(part[idx+1:]))
		if err != nil {
			return nil, fmt.Errorf("invalid range end: %s", part[idx+1:])
		}
	} else {
		// Single value
		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %s", part)
		}
		rangeStart = val
		rangeEnd = val
	}

	// Validate range
	if rangeStart < min || rangeEnd > max {
		return nil, fmt.Errorf("value out of range [%d-%d]: %d-%d", min, max, rangeStart, rangeEnd)
	}
	if rangeStart > rangeEnd {
		return nil, fmt.Errorf("invalid range: %d > %d", rangeStart, rangeEnd)
	}

	// Parse step
	step := 1
	if stepStr != "" {
		step, err = strconv.Atoi(stepStr)
		if err != nil || step < 1 {
			return nil, fmt.Errorf("invalid step: %s", stepStr)
		}
	}

	// Generate values
	var values []int
	for v := rangeStart; v <= rangeEnd; v += step {
		values = append(values, v)
	}

	return values, nil
}

// ParseScheduleEnhanced parses a schedule expression with full cron syntax support.
// Supports:
// - Simple: "hourly", "daily", "weekly"
// - Intervals: "every 4h", "every 30m"
// - Cron (simple): "0 2 * * *" (single values only)
// - Cron (enhanced): "0-30 2 * * *" (ranges, steps, lists)
func ParseScheduleEnhanced(expr string) (*Schedule, error) {
	expr = strings.TrimSpace(strings.ToLower(expr))
	s := &Schedule{Expression: expr}

	// Simple keywords
	switch expr {
	case "hourly":
		s.interval = 3600_000_000_000 // time.Hour
		return s, nil
	case "daily":
		s.hour = 2
		s.minute = 0
		s.dom = -1
		s.month = -1
		s.dow = -1
		// Also set enhanced fields for consistent behavior
		s.hourField = &CronField{Values: []int{2}}
		s.minuteField = &CronField{Values: []int{0}}
		s.domField = &CronField{Any: true}
		s.monthField = &CronField{Any: true}
		s.dowField = &CronField{Any: true}
		return s, nil
	case "weekly":
		s.hour = 2
		s.minute = 0
		s.dom = -1
		s.month = -1
		s.dow = 0
		s.hourField = &CronField{Values: []int{2}}
		s.minuteField = &CronField{Values: []int{0}}
		s.domField = &CronField{Any: true}
		s.monthField = &CronField{Any: true}
		s.dowField = &CronField{Values: []int{0}}
		return s, nil
	}

	// Interval format: "every Xh", "every Xm"
	if strings.HasPrefix(expr, "every ") {
		intervalStr := strings.TrimPrefix(expr, "every ")
		dur, err := parseIntervalDuration(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %s", intervalStr)
		}
		if dur < 60_000_000_000 { // time.Minute
			return nil, fmt.Errorf("interval must be at least 1 minute")
		}
		s.interval = dur
		return s, nil
	}

	// Cron format: "minute hour dom month dow"
	parts := strings.Fields(expr)
	if len(parts) == 5 {
		var err error

		s.minuteField, err = ParseCronField(parts[0], 0, 59)
		if err != nil {
			return nil, fmt.Errorf("invalid minute: %w", err)
		}

		s.hourField, err = ParseCronField(parts[1], 0, 23)
		if err != nil {
			return nil, fmt.Errorf("invalid hour: %w", err)
		}

		s.domField, err = ParseCronField(parts[2], 1, 31)
		if err != nil {
			return nil, fmt.Errorf("invalid day of month: %w", err)
		}

		s.monthField, err = ParseCronField(parts[3], 1, 12)
		if err != nil {
			return nil, fmt.Errorf("invalid month: %w", err)
		}

		s.dowField, err = ParseCronField(parts[4], 0, 6)
		if err != nil {
			return nil, fmt.Errorf("invalid day of week: %w", err)
		}

		// Also set simple fields for backward compatibility
		if !s.minuteField.Any && len(s.minuteField.Values) == 1 {
			s.minute = s.minuteField.Values[0]
		} else {
			s.minute = -1
		}
		if !s.hourField.Any && len(s.hourField.Values) == 1 {
			s.hour = s.hourField.Values[0]
		} else {
			s.hour = -1
		}
		if !s.domField.Any && len(s.domField.Values) == 1 {
			s.dom = s.domField.Values[0]
		} else {
			s.dom = -1
		}
		if !s.monthField.Any && len(s.monthField.Values) == 1 {
			s.month = s.monthField.Values[0]
		} else {
			s.month = -1
		}
		if !s.dowField.Any && len(s.dowField.Values) == 1 {
			s.dow = s.dowField.Values[0]
		} else {
			s.dow = -1
		}

		return s, nil
	}

	return nil, fmt.Errorf("unrecognized schedule format: %s", expr)
}

// parseIntervalDuration parses duration strings like "4h", "30m", "1h30m"
func parseIntervalDuration(s string) (time.Duration, error) {
	// Try standard Go duration parsing
	dur, err := parseDuration(s)
	if err == nil {
		return dur, nil
	}

	// Handle simple cases without units
	if strings.HasSuffix(s, "h") {
		hours, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err == nil {
			return time.Duration(hours) * time.Hour, nil
		}
	}
	if strings.HasSuffix(s, "m") {
		mins, err := strconv.Atoi(strings.TrimSuffix(s, "m"))
		if err == nil {
			return time.Duration(mins) * time.Minute, nil
		}
	}

	return 0, fmt.Errorf("cannot parse duration: %s", s)
}

// parseDuration is a simple duration parser (re-implementation to avoid import cycle)
func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
