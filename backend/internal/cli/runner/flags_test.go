package runner

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestFlagSetString(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("name", "default", "test flag")
	cmd.Flags().Set("name", "alice")

	flags := Flags(cmd)
	val := flags.String("name")

	if val != "alice" {
		t.Errorf("expected 'alice', got %q", val)
	}
	if flags.Err() != nil {
		t.Errorf("unexpected error: %v", flags.Err())
	}
}

func TestFlagSetInt(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int("count", 0, "test flag")
	cmd.Flags().Set("count", "42")

	flags := Flags(cmd)
	val := flags.Int("count")

	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
	if flags.Err() != nil {
		t.Errorf("unexpected error: %v", flags.Err())
	}
}

func TestFlagSetBool(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("verbose", false, "test flag")
	cmd.Flags().Set("verbose", "true")

	flags := Flags(cmd)
	val := flags.Bool("verbose")

	if !val {
		t.Error("expected true, got false")
	}
	if flags.Err() != nil {
		t.Errorf("unexpected error: %v", flags.Err())
	}
}

func TestFlagSetStringSlice(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("items", nil, "test flag")
	cmd.Flags().Set("items", "a,b,c")

	flags := Flags(cmd)
	val := flags.StringSlice("items")

	if len(val) != 3 {
		t.Errorf("expected 3 items, got %d", len(val))
	}
	if val[0] != "a" || val[1] != "b" || val[2] != "c" {
		t.Errorf("unexpected values: %v", val)
	}
	if flags.Err() != nil {
		t.Errorf("unexpected error: %v", flags.Err())
	}
}

func TestFlagSetChanged(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("changed", "default", "test flag")
	cmd.Flags().String("unchanged", "default", "test flag")
	cmd.Flags().Set("changed", "new")

	flags := Flags(cmd)

	if !flags.Changed("changed") {
		t.Error("expected 'changed' to be changed")
	}
	if flags.Changed("unchanged") {
		t.Error("expected 'unchanged' to not be changed")
	}
}

func TestFlagSetErrorAccumulation(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("valid", "default", "test flag")
	// No flags named "invalid1" or "invalid2"

	flags := Flags(cmd)

	// These should accumulate errors
	_ = flags.String("invalid1")
	_ = flags.Int("invalid2")

	// Valid flag should still work
	val := flags.String("valid")
	if val != "default" {
		t.Errorf("expected 'default', got %q", val)
	}

	// Should have errors
	if !flags.HasErrors() {
		t.Error("expected HasErrors() to return true")
	}
	if flags.Err() == nil {
		t.Error("expected Err() to return error")
	}
}

func TestFlagSetNoErrors(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("name", "default", "test flag")

	flags := Flags(cmd)
	_ = flags.String("name")

	if flags.HasErrors() {
		t.Error("expected HasErrors() to return false")
	}
	if flags.Err() != nil {
		t.Errorf("expected Err() to return nil, got %v", flags.Err())
	}
}

func TestFlagSetInt64(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int64("size", 0, "test flag")
	cmd.Flags().Set("size", "9223372036854775807") // max int64

	flags := Flags(cmd)
	val := flags.Int64("size")

	if val != 9223372036854775807 {
		t.Errorf("expected max int64, got %d", val)
	}
	if flags.Err() != nil {
		t.Errorf("unexpected error: %v", flags.Err())
	}
}
