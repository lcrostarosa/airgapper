package runner

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// Interceptor is a function that wraps command execution.
// It mirrors the Connect-RPC interceptor pattern for CLI commands.
type Interceptor func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error

// RequireConfig ensures the configuration is loaded before executing the command.
func RequireConfig() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		if ctx.ConfigErr != nil {
			return ctx.ConfigErr
		}
		if ctx.Config == nil {
			return ErrNotInitialized
		}
		return next()
	}
}

// RequireOwner ensures the caller has the owner role.
// Implicitly requires config to be loaded.
func RequireOwner() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		if ctx.ConfigErr != nil {
			return ctx.ConfigErr
		}
		if ctx.Config == nil {
			return ErrNotInitialized
		}
		if !ctx.Config.IsOwner() {
			return fmt.Errorf("%w (you are: %s)", ErrNotOwner, ctx.Config.Role)
		}
		return next()
	}
}

// RequireHost ensures the caller has the host role.
// Implicitly requires config to be loaded.
func RequireHost() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		if ctx.ConfigErr != nil {
			return ctx.ConfigErr
		}
		if ctx.Config == nil {
			return ErrNotInitialized
		}
		if !ctx.Config.IsHost() {
			return fmt.Errorf("%w (you are: %s)", ErrNotHost, ctx.Config.Role)
		}
		return next()
	}
}

// RequirePassword ensures a password is available.
// Implicitly requires config to be loaded.
func RequirePassword() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		if ctx.ConfigErr != nil {
			return ctx.ConfigErr
		}
		if ctx.Config == nil {
			return ErrNotInitialized
		}
		if ctx.Config.Password == "" {
			return ErrNoPassword
		}
		return next()
	}
}

// RequirePrivateKey ensures a private key is available.
// Implicitly requires config to be loaded.
func RequirePrivateKey() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		if ctx.ConfigErr != nil {
			return ctx.ConfigErr
		}
		if ctx.Config == nil {
			return ErrNotInitialized
		}
		if ctx.Config.PrivateKey == nil {
			return ErrNoPrivateKey
		}
		return next()
	}
}

// RequireShare ensures a local key share is available.
// Implicitly requires config to be loaded.
func RequireShare() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		if ctx.ConfigErr != nil {
			return ctx.ConfigErr
		}
		if ctx.Config == nil {
			return ErrNotInitialized
		}
		if ctx.Config.LocalShare == nil {
			return ErrNoShare
		}
		return next()
	}
}

// RecordActivity updates the last activity timestamp after successful command execution.
// This is used for the dead man's switch feature.
func RecordActivity() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		err := next()
		if err == nil && ctx.Config != nil {
			ctx.Config.RecordActivity()
		}
		return err
	}
}

// WithLogging logs command execution, mirroring the gRPC loggingInterceptor.
func WithLogging() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		logging.Debug("CLI command", logging.String("cmd", cmd.Name()))
		err := next()
		if err != nil {
			logging.Debug("CLI error", logging.String("cmd", cmd.Name()), logging.Err(err))
		}
		return err
	}
}

// AllowUninitialized marks that this command can run without initialization.
// This is a no-op interceptor that documents intent.
func AllowUninitialized() Interceptor {
	return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
		return next()
	}
}
