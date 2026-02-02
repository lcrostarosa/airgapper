package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCronField_SingleValue(t *testing.T) {
	field, err := ParseCronField("5", 0, 59)
	require.NoError(t, err, "unexpected error")

	assert.False(t, field.Any, "should not be 'any'")
	assert.Equal(t, []int{5}, field.Values)
	assert.True(t, field.Contains(5), "should contain 5")
	assert.False(t, field.Contains(6), "should not contain 6")
}

func TestParseCronField_Any(t *testing.T) {
	field, err := ParseCronField("*", 0, 59)
	require.NoError(t, err, "unexpected error")

	assert.True(t, field.Any, "should be 'any'")
	assert.True(t, field.Contains(0), "'any' should contain all values")
	assert.True(t, field.Contains(30), "'any' should contain all values")
	assert.True(t, field.Contains(59), "'any' should contain all values")
}

func TestParseCronField_Range(t *testing.T) {
	field, err := ParseCronField("1-5", 0, 59)
	require.NoError(t, err, "unexpected error")

	expected := []int{1, 2, 3, 4, 5}
	assert.Equal(t, expected, field.Values)
}

func TestParseCronField_Step(t *testing.T) {
	field, err := ParseCronField("*/15", 0, 59)
	require.NoError(t, err, "unexpected error")

	expected := []int{0, 15, 30, 45}
	assert.Equal(t, expected, field.Values)
}

func TestParseCronField_RangeWithStep(t *testing.T) {
	field, err := ParseCronField("0-30/10", 0, 59)
	require.NoError(t, err, "unexpected error")

	expected := []int{0, 10, 20, 30}
	assert.Equal(t, expected, field.Values)
}

func TestParseCronField_List(t *testing.T) {
	field, err := ParseCronField("0,15,30,45", 0, 59)
	require.NoError(t, err, "unexpected error")

	expected := []int{0, 15, 30, 45}
	assert.Equal(t, expected, field.Values)
}

func TestParseCronField_Mixed(t *testing.T) {
	// Mixed: ranges, steps, and single values
	field, err := ParseCronField("1-5,10,20-25/2", 0, 59)
	require.NoError(t, err, "unexpected error")

	// 1-5 = 1,2,3,4,5
	// 10 = 10
	// 20-25/2 = 20,22,24
	// Combined and sorted: 1,2,3,4,5,10,20,22,24
	expected := []int{1, 2, 3, 4, 5, 10, 20, 22, 24}
	assert.Equal(t, expected, field.Values)
}

func TestParseCronField_Invalid(t *testing.T) {
	tests := []struct {
		field string
		min   int
		max   int
	}{
		{"60", 0, 59},   // out of range
		{"-1", 0, 59},   // out of range
		{"abc", 0, 59},  // not a number
		{"5-3", 0, 59},  // invalid range
		{"*/0", 0, 59},  // step of 0
		{"1-70", 0, 59}, // range end out of range
	}

	for _, tt := range tests {
		_, err := ParseCronField(tt.field, tt.min, tt.max)
		assert.Error(t, err, "expected error for field %q", tt.field)
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
		assert.Equal(t, tt.expected, result, "Next(%d)", tt.val)
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
		assert.NoError(t, err, "ParseScheduleEnhanced(%q)", tt.expr)
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
		require.NoError(t, err, "ParseScheduleEnhanced(%q)", tt.expr)
		assert.NotNil(t, s.minuteField, "ParseScheduleEnhanced(%q) minuteField is nil", tt.expr)
	}
}

func TestParseScheduleEnhanced_NextRun(t *testing.T) {
	// Test "every 15 minutes"
	s, _ := ParseScheduleEnhanced("*/15 * * * *")

	// From 10:07, next should be 10:15
	base := time.Date(2024, 1, 15, 10, 7, 0, 0, time.UTC)
	next := s.NextRun(base)
	assert.Equal(t, 15, next.Minute())
	assert.Equal(t, 10, next.Hour())

	// From 10:45, next should be 11:00
	base = time.Date(2024, 1, 15, 10, 45, 0, 0, time.UTC)
	next = s.NextRun(base)
	assert.Equal(t, 0, next.Minute())
	assert.Equal(t, 11, next.Hour())
}

func TestParseScheduleEnhanced_WorkdayHours(t *testing.T) {
	// "At minute 0, 9-17 hours, Mon-Fri"
	s, err := ParseScheduleEnhanced("0 9-17 * * 1-5")
	require.NoError(t, err, "unexpected error")

	// Test that it matches 9 AM Monday
	monday9am := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC) // Monday
	assert.True(t, s.matchesEnhanced(monday9am), "should match Monday 9 AM")

	// Test that it doesn't match Saturday
	saturday9am := time.Date(2024, 1, 20, 9, 0, 0, 0, time.UTC) // Saturday
	assert.False(t, s.matchesEnhanced(saturday9am), "should not match Saturday 9 AM")

	// Test that it doesn't match 8 AM
	monday8am := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	assert.False(t, s.matchesEnhanced(monday8am), "should not match Monday 8 AM")
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
	assert.Equal(t, time.Sunday, next.Weekday())
	assert.Equal(t, 2, next.Hour())
	assert.Equal(t, 0, next.Minute())

	// Verify it was reasonably fast (should be <100ms even on slow systems)
	if elapsed > 100*time.Millisecond {
		t.Logf("Warning: NextRun took %v (expected <100ms)", elapsed)
	}
}

func TestSchedule_BackwardCompatibility(t *testing.T) {
	// Ensure old ParseSchedule still works
	s, err := ParseSchedule("0 2 * * *")
	require.NoError(t, err, "ParseSchedule error")

	// Should work with linear search
	base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	next := s.NextRun(base)
	assert.Equal(t, 2, next.Hour())
	assert.Equal(t, 0, next.Minute())
}
