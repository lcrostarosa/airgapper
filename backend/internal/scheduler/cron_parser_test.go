package scheduler

import (
	"testing"
	"time"
)

func TestParseCronField_SingleValue(t *testing.T) {
	field, err := ParseCronField("5", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if field.Any {
		t.Error("should not be 'any'")
	}
	if len(field.Values) != 1 || field.Values[0] != 5 {
		t.Errorf("expected [5], got %v", field.Values)
	}
	if !field.Contains(5) {
		t.Error("should contain 5")
	}
	if field.Contains(6) {
		t.Error("should not contain 6")
	}
}

func TestParseCronField_Any(t *testing.T) {
	field, err := ParseCronField("*", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !field.Any {
		t.Error("should be 'any'")
	}
	if !field.Contains(0) || !field.Contains(30) || !field.Contains(59) {
		t.Error("'any' should contain all values")
	}
}

func TestParseCronField_Range(t *testing.T) {
	field, err := ParseCronField("1-5", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{1, 2, 3, 4, 5}
	if len(field.Values) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, field.Values)
	}
	for i, v := range expected {
		if field.Values[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, field.Values[i])
		}
	}
}

func TestParseCronField_Step(t *testing.T) {
	field, err := ParseCronField("*/15", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{0, 15, 30, 45}
	if len(field.Values) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, field.Values)
	}
	for i, v := range expected {
		if field.Values[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, field.Values[i])
		}
	}
}

func TestParseCronField_RangeWithStep(t *testing.T) {
	field, err := ParseCronField("0-30/10", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{0, 10, 20, 30}
	if len(field.Values) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, field.Values)
	}
	for i, v := range expected {
		if field.Values[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, field.Values[i])
		}
	}
}

func TestParseCronField_List(t *testing.T) {
	field, err := ParseCronField("0,15,30,45", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{0, 15, 30, 45}
	if len(field.Values) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, field.Values)
	}
}

func TestParseCronField_Mixed(t *testing.T) {
	// Mixed: ranges, steps, and single values
	field, err := ParseCronField("1-5,10,20-25/2", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1-5 = 1,2,3,4,5
	// 10 = 10
	// 20-25/2 = 20,22,24
	// Combined and sorted: 1,2,3,4,5,10,20,22,24
	expected := []int{1, 2, 3, 4, 5, 10, 20, 22, 24}
	if len(field.Values) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, field.Values)
	}
	for i, v := range expected {
		if field.Values[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, field.Values[i])
		}
	}
}

func TestParseCronField_Invalid(t *testing.T) {
	tests := []struct {
		field string
		min   int
		max   int
	}{
		{"60", 0, 59},    // out of range
		{"-1", 0, 59},    // out of range
		{"abc", 0, 59},   // not a number
		{"5-3", 0, 59},   // invalid range
		{"*/0", 0, 59},   // step of 0
		{"1-70", 0, 59},  // range end out of range
	}

	for _, tt := range tests {
		_, err := ParseCronField(tt.field, tt.min, tt.max)
		if err == nil {
			t.Errorf("expected error for field %q", tt.field)
		}
	}
}

func TestCronField_Next(t *testing.T) {
	field, _ := ParseCronField("0,15,30,45", 0, 59)

	tests := []struct {
		val      int
		expected int
	}{
		{0, 0},
		{1, 15},
		{14, 15},
		{15, 15},
		{16, 30},
		{45, 45},
		{46, -1}, // wrap needed
	}

	for _, tt := range tests {
		result := field.Next(tt.val)
		if result != tt.expected {
			t.Errorf("Next(%d) = %d, want %d", tt.val, result, tt.expected)
		}
	}
}

func TestParseScheduleEnhanced_Simple(t *testing.T) {
	tests := []struct {
		expr string
	}{
		{"hourly"},
		{"daily"},
		{"weekly"},
		{"every 4h"},
		{"every 30m"},
	}

	for _, tt := range tests {
		_, err := ParseScheduleEnhanced(tt.expr)
		if err != nil {
			t.Errorf("ParseScheduleEnhanced(%q) error: %v", tt.expr, err)
		}
	}
}

func TestParseScheduleEnhanced_CronWithRanges(t *testing.T) {
	tests := []struct {
		expr        string
		description string
	}{
		{"0-30 2 * * *", "minutes 0-30 at 2 AM"},
		{"*/15 * * * *", "every 15 minutes"},
		{"0,15,30,45 * * * *", "specific minutes"},
		{"0 9-17 * * 1-5", "9-5 on weekdays"},
		{"0-30/10 2,14 * * 1-5", "complex"},
	}

	for _, tt := range tests {
		s, err := ParseScheduleEnhanced(tt.expr)
		if err != nil {
			t.Errorf("ParseScheduleEnhanced(%q) error: %v", tt.expr, err)
			continue
		}
		if s.minuteField == nil {
			t.Errorf("ParseScheduleEnhanced(%q) minuteField is nil", tt.expr)
		}
	}
}

func TestParseScheduleEnhanced_NextRun(t *testing.T) {
	// Test "every 15 minutes"
	s, _ := ParseScheduleEnhanced("*/15 * * * *")

	// From 10:07, next should be 10:15
	base := time.Date(2024, 1, 15, 10, 7, 0, 0, time.UTC)
	next := s.NextRun(base)

	if next.Minute() != 15 || next.Hour() != 10 {
		t.Errorf("expected 10:15, got %v", next)
	}

	// From 10:45, next should be 11:00
	base = time.Date(2024, 1, 15, 10, 45, 0, 0, time.UTC)
	next = s.NextRun(base)

	if next.Minute() != 0 || next.Hour() != 11 {
		t.Errorf("expected 11:00, got %v", next)
	}
}

func TestParseScheduleEnhanced_WorkdayHours(t *testing.T) {
	// "At minute 0, 9-17 hours, Mon-Fri"
	s, err := ParseScheduleEnhanced("0 9-17 * * 1-5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test that it matches 9 AM Monday
	monday9am := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC) // Monday
	if !s.matchesEnhanced(monday9am) {
		t.Error("should match Monday 9 AM")
	}

	// Test that it doesn't match Saturday
	saturday9am := time.Date(2024, 1, 20, 9, 0, 0, 0, time.UTC) // Saturday
	if s.matchesEnhanced(saturday9am) {
		t.Error("should not match Saturday 9 AM")
	}

	// Test that it doesn't match 8 AM
	monday8am := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	if s.matchesEnhanced(monday8am) {
		t.Error("should not match Monday 8 AM")
	}
}

func TestSchedule_NextRun_Efficiency(t *testing.T) {
	// Verify that enhanced schedules use efficient algorithm

	// This schedule only runs at 2 AM on Sunday
	s, _ := ParseScheduleEnhanced("0 2 * * 0")

	// Start from a Friday - should jump to Sunday
	friday := time.Date(2024, 1, 19, 10, 0, 0, 0, time.UTC) // Friday
	start := time.Now()
	next := s.NextRun(friday)
	elapsed := time.Since(start)

	// Verify it found Sunday
	if next.Weekday() != time.Sunday {
		t.Errorf("expected Sunday, got %v", next.Weekday())
	}
	if next.Hour() != 2 || next.Minute() != 0 {
		t.Errorf("expected 2:00, got %d:%d", next.Hour(), next.Minute())
	}

	// Verify it was reasonably fast (should be <100ms even on slow systems)
	if elapsed > 100*time.Millisecond {
		t.Logf("Warning: NextRun took %v (expected <100ms)", elapsed)
	}
}

func TestSchedule_BackwardCompatibility(t *testing.T) {
	// Ensure old ParseSchedule still works
	s, err := ParseSchedule("0 2 * * *")
	if err != nil {
		t.Fatalf("ParseSchedule error: %v", err)
	}

	// Should work with linear search
	base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	next := s.NextRun(base)

	if next.Hour() != 2 || next.Minute() != 0 {
		t.Errorf("expected 2:00, got %d:%d", next.Hour(), next.Minute())
	}
}
