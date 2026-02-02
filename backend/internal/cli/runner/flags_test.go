package runner

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestFlagSetString(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("name", "default", "test flag")
	cmd.Flags().Set("name", "alice")

	flags := Flags(cmd)
	val := flags.String("name")

	assert.Equal(t, "alice", val)
	assert.NoError(t, flags.Err())
}

func TestFlagSetInt(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int("count", 0, "test flag")
	cmd.Flags().Set("count", "42")

	flags := Flags(cmd)
	val := flags.Int("count")

	assert.Equal(t, 42, val)
	assert.NoError(t, flags.Err())
}

func TestFlagSetBool(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("verbose", false, "test flag")
	cmd.Flags().Set("verbose", "true")

	flags := Flags(cmd)
	val := flags.Bool("verbose")

	assert.True(t, val)
	assert.NoError(t, flags.Err())
}

func TestFlagSetStringSlice(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("items", nil, "test flag")
	cmd.Flags().Set("items", "a,b,c")

	flags := Flags(cmd)
	val := flags.StringSlice("items")

	assert.Len(t, val, 3)
	assert.Equal(t, []string{"a", "b", "c"}, val)
	assert.NoError(t, flags.Err())
}

func TestFlagSetChanged(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("changed", "default", "test flag")
	cmd.Flags().String("unchanged", "default", "test flag")
	cmd.Flags().Set("changed", "new")

	flags := Flags(cmd)

	assert.True(t, flags.Changed("changed"), "expected 'changed' to be changed")
	assert.False(t, flags.Changed("unchanged"), "expected 'unchanged' to not be changed")
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
	assert.Equal(t, "default", val)

	// Should have errors
	assert.True(t, flags.HasErrors())
	assert.Error(t, flags.Err())
}

func TestFlagSetNoErrors(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("name", "default", "test flag")

	flags := Flags(cmd)
	_ = flags.String("name")

	assert.False(t, flags.HasErrors())
	assert.NoError(t, flags.Err())
}

func TestFlagSetInt64(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int64("size", 0, "test flag")
	cmd.Flags().Set("size", "9223372036854775807") // max int64

	flags := Flags(cmd)
	val := flags.Int64("size")

	assert.Equal(t, int64(9223372036854775807), val)
	assert.NoError(t, flags.Err())
}
