package runner

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// FlagSet provides type-safe flag extraction with error accumulation.
// Errors are collected and can be checked at the end with Err().
type FlagSet struct {
	flags *pflag.FlagSet
	errs  []error
}

// Flags creates a new FlagSet for the given command.
func Flags(cmd *cobra.Command) *FlagSet {
	return &FlagSet{flags: cmd.Flags()}
}

// String extracts a string flag value. Errors are accumulated.
func (f *FlagSet) String(name string) string {
	val, err := f.flags.GetString(name)
	if err != nil {
		f.errs = append(f.errs, fmt.Errorf("flag %s: %w", name, err))
	}
	return val
}

// StringSlice extracts a string slice flag value. Errors are accumulated.
func (f *FlagSet) StringSlice(name string) []string {
	val, err := f.flags.GetStringSlice(name)
	if err != nil {
		f.errs = append(f.errs, fmt.Errorf("flag %s: %w", name, err))
	}
	return val
}

// Int extracts an int flag value. Errors are accumulated.
func (f *FlagSet) Int(name string) int {
	val, err := f.flags.GetInt(name)
	if err != nil {
		f.errs = append(f.errs, fmt.Errorf("flag %s: %w", name, err))
	}
	return val
}

// Int64 extracts an int64 flag value. Errors are accumulated.
func (f *FlagSet) Int64(name string) int64 {
	val, err := f.flags.GetInt64(name)
	if err != nil {
		f.errs = append(f.errs, fmt.Errorf("flag %s: %w", name, err))
	}
	return val
}

// Bool extracts a bool flag value. Errors are accumulated.
func (f *FlagSet) Bool(name string) bool {
	val, err := f.flags.GetBool(name)
	if err != nil {
		f.errs = append(f.errs, fmt.Errorf("flag %s: %w", name, err))
	}
	return val
}

// Duration extracts a duration flag value. Errors are accumulated.
func (f *FlagSet) Duration(name string) string {
	// Duration flags are stored as strings in this codebase
	val, err := f.flags.GetString(name)
	if err != nil {
		f.errs = append(f.errs, fmt.Errorf("flag %s: %w", name, err))
	}
	return val
}

// Changed returns true if the flag was explicitly set.
func (f *FlagSet) Changed(name string) bool {
	return f.flags.Changed(name)
}

// Err returns any accumulated errors joined together.
// Returns nil if no errors occurred.
func (f *FlagSet) Err() error {
	return errors.Join(f.errs...)
}

// HasErrors returns true if any errors have been accumulated.
func (f *FlagSet) HasErrors() bool {
	return len(f.errs) > 0
}
