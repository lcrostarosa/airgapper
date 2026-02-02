package runner

import (
	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
)

// ConfigProvider is a function that returns the current config and any load error.
// This allows the runner to be decoupled from the global config state.
type ConfigProvider func() (*config.Config, error)

// CommandRunner chains interceptors for CLI command execution.
// It mirrors Connect-RPC's interceptor pattern.
type CommandRunner struct {
	interceptors   []Interceptor
	configProvider ConfigProvider
}

// NewRunner creates a new CommandRunner with the given config provider.
func NewRunner(provider ConfigProvider) *CommandRunner {
	return &CommandRunner{
		configProvider: provider,
	}
}

// Use adds interceptors to the chain. Returns self for chaining.
func (r *CommandRunner) Use(interceptors ...Interceptor) *CommandRunner {
	r.interceptors = append(r.interceptors, interceptors...)
	return r
}

// Clone creates a copy of this runner with its own interceptor chain.
// The config provider is shared.
func (r *CommandRunner) Clone() *CommandRunner {
	cloned := &CommandRunner{
		interceptors:   make([]Interceptor, len(r.interceptors)),
		configProvider: r.configProvider,
	}
	copy(cloned.interceptors, r.interceptors)
	return cloned
}

// CommandFunc is the signature for command handler functions.
type CommandFunc func(ctx *CommandContext, cmd *cobra.Command, args []string) error

// Wrap creates a cobra.RunE function with the interceptor chain applied.
func (r *CommandRunner) Wrap(fn CommandFunc) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Get config from provider
		cfg, cfgErr := r.configProvider()
		ctx := NewContext(cfg, cfgErr)

		// Build the chain: interceptors wrap the handler
		chain := func() error { return fn(ctx, cmd, args) }

		// Wrap in reverse order so first interceptor runs first
		for i := len(r.interceptors) - 1; i >= 0; i-- {
			interceptor := r.interceptors[i]
			next := chain
			chain = func() error { return interceptor(ctx, cmd, args, next) }
		}

		return chain()
	}
}

// Pre-configured runners for common use cases.
// These are created by the cli package with the actual config provider.

// Builder helps construct runners with common interceptor patterns.
type Builder struct {
	provider ConfigProvider
}

// NewBuilder creates a new runner builder with the given config provider.
func NewBuilder(provider ConfigProvider) *Builder {
	return &Builder{provider: provider}
}

// Base creates a runner with just logging.
func (b *Builder) Base() *CommandRunner {
	return NewRunner(b.provider).Use(WithLogging())
}

// Config creates a runner that requires config to be loaded.
func (b *Builder) Config() *CommandRunner {
	return NewRunner(b.provider).Use(
		WithLogging(),
		RequireConfig(),
	)
}

// Owner creates a runner that requires owner role.
func (b *Builder) Owner() *CommandRunner {
	return NewRunner(b.provider).Use(
		WithLogging(),
		RequireOwner(),
	)
}

// OwnerWithPassword creates a runner that requires owner role and password.
func (b *Builder) OwnerWithPassword() *CommandRunner {
	return NewRunner(b.provider).Use(
		WithLogging(),
		RequireOwner(),
		RequirePassword(),
	)
}

// OwnerWithActivity creates a runner that requires owner role and records activity.
func (b *Builder) OwnerWithActivity() *CommandRunner {
	return NewRunner(b.provider).Use(
		WithLogging(),
		RequireOwner(),
		RequirePassword(),
		RecordActivity(),
	)
}

// Host creates a runner that requires host role.
func (b *Builder) Host() *CommandRunner {
	return NewRunner(b.provider).Use(
		WithLogging(),
		RequireHost(),
	)
}

// Uninitialized creates a runner that can run without initialization.
func (b *Builder) Uninitialized() *CommandRunner {
	return NewRunner(b.provider).Use(
		WithLogging(),
		AllowUninitialized(),
	)
}
