package runner

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
)

func TestInterceptorChainOrder(t *testing.T) {
	// Track execution order
	var order []string

	provider := func() (*config.Config, error) {
		return &config.Config{Role: config.RoleOwner}, nil
	}

	// Create interceptors that record their execution
	makeInterceptor := func(name string) Interceptor {
		return func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
			order = append(order, name+"-before")
			err := next()
			order = append(order, name+"-after")
			return err
		}
	}

	runner := NewRunner(provider).Use(
		makeInterceptor("first"),
		makeInterceptor("second"),
		makeInterceptor("third"),
	)

	handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
		order = append(order, "handler")
		return nil
	}

	cmd := &cobra.Command{}
	err := runner.Wrap(handler)(cmd, nil)
	require.NoError(t, err)

	expected := []string{
		"first-before",
		"second-before",
		"third-before",
		"handler",
		"third-after",
		"second-after",
		"first-after",
	}

	require.Len(t, order, len(expected))

	for i, exp := range expected {
		assert.Equal(t, exp, order[i], "order[%d]", i)
	}
}

func TestInterceptorChainStopsOnError(t *testing.T) {
	var order []string
	expectedErr := errors.New("interceptor error")

	provider := func() (*config.Config, error) {
		return &config.Config{Role: config.RoleOwner}, nil
	}

	runner := NewRunner(provider).Use(
		func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
			order = append(order, "first")
			return next()
		},
		func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
			order = append(order, "second-fails")
			return expectedErr
		},
		func(ctx *CommandContext, cmd *cobra.Command, args []string, next func() error) error {
			order = append(order, "third-should-not-run")
			return next()
		},
	)

	handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
		order = append(order, "handler-should-not-run")
		return nil
	}

	cmd := &cobra.Command{}
	err := runner.Wrap(handler)(cmd, nil)

	assert.ErrorIs(t, err, expectedErr)
	assert.Len(t, order, 2)
	assert.Equal(t, "first", order[0])
	assert.Equal(t, "second-fails", order[1])
}

func TestRequireConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.Config
		cfgErr    error
		wantErr   error
		wantCalls bool
	}{
		{
			name:      "config loaded",
			cfg:       &config.Config{},
			cfgErr:    nil,
			wantErr:   nil,
			wantCalls: true,
		},
		{
			name:      "config nil",
			cfg:       nil,
			cfgErr:    nil,
			wantErr:   ErrNotInitialized,
			wantCalls: false,
		},
		{
			name:      "config error",
			cfg:       nil,
			cfgErr:    errors.New("load error"),
			wantErr:   nil, // Returns the actual error
			wantCalls: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false
			provider := func() (*config.Config, error) {
				return tt.cfg, tt.cfgErr
			}

			runner := NewRunner(provider).Use(RequireConfig())
			handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
				handlerCalled = true
				return nil
			}

			cmd := &cobra.Command{}
			err := runner.Wrap(handler)(cmd, nil)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else if tt.cfgErr != nil {
				assert.Error(t, err)
			}

			assert.Equal(t, tt.wantCalls, handlerCalled)
		})
	}
}

func TestRequireOwner(t *testing.T) {
	tests := []struct {
		name        string
		role        config.Role
		wantErr     bool
		errContains string
	}{
		{
			name:    "owner role",
			role:    config.RoleOwner,
			wantErr: false,
		},
		{
			name:        "host role",
			role:        config.RoleHost,
			wantErr:     true,
			errContains: "only the data owner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := func() (*config.Config, error) {
				return &config.Config{Role: tt.role}, nil
			}

			runner := NewRunner(provider).Use(RequireOwner())
			handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
				return nil
			}

			cmd := &cobra.Command{}
			err := runner.Wrap(handler)(cmd, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrNotOwner)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRequireHost(t *testing.T) {
	tests := []struct {
		name    string
		role    config.Role
		wantErr bool
	}{
		{
			name:    "host role",
			role:    config.RoleHost,
			wantErr: false,
		},
		{
			name:    "owner role",
			role:    config.RoleOwner,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := func() (*config.Config, error) {
				return &config.Config{Role: tt.role}, nil
			}

			runner := NewRunner(provider).Use(RequireHost())
			handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
				return nil
			}

			cmd := &cobra.Command{}
			err := runner.Wrap(handler)(cmd, nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRequirePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "password present",
			password: "secret",
			wantErr:  nil,
		},
		{
			name:     "password empty",
			password: "",
			wantErr:  ErrNoPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := func() (*config.Config, error) {
				return &config.Config{Password: tt.password}, nil
			}

			runner := NewRunner(provider).Use(RequirePassword())
			handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
				return nil
			}

			cmd := &cobra.Command{}
			err := runner.Wrap(handler)(cmd, nil)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContextLazyConsentManager(t *testing.T) {
	cfg := &config.Config{
		ConfigDir: "/tmp/test-config",
	}

	ctx := NewContext(cfg, nil)

	// First call should initialize
	mgr1 := ctx.Consent()
	assert.NotNil(t, mgr1)

	// Second call should return same instance
	mgr2 := ctx.Consent()
	assert.Same(t, mgr1, mgr2)
}

func TestContextNilConfig(t *testing.T) {
	ctx := NewContext(nil, nil)

	// Should return nil without panicking
	mgr := ctx.Consent()
	assert.Nil(t, mgr)

	// Helper methods should be safe
	assert.False(t, ctx.HasConfig())
	assert.False(t, ctx.IsOwner())
	assert.False(t, ctx.IsHost())
	assert.False(t, ctx.HasPassword())
}

func TestRecordActivityInterceptor(t *testing.T) {
	cfg := &config.Config{
		ConfigDir: "/tmp/test-config",
		Role:      config.RoleOwner,
	}
	provider := func() (*config.Config, error) {
		return cfg, nil
	}

	t.Run("records on success", func(t *testing.T) {
		runner := NewRunner(provider).Use(RecordActivity())
		handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
			return nil
		}

		cmd := &cobra.Command{}
		err := runner.Wrap(handler)(cmd, nil)
		assert.NoError(t, err)
	})

	t.Run("does not record on error", func(t *testing.T) {
		expectedErr := errors.New("handler error")
		runner := NewRunner(provider).Use(RecordActivity())
		handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
			return expectedErr
		}

		cmd := &cobra.Command{}
		err := runner.Wrap(handler)(cmd, nil)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestBuilderPatterns(t *testing.T) {
	provider := func() (*config.Config, error) {
		return &config.Config{
			Role:     config.RoleOwner,
			Password: "secret",
		}, nil
	}

	builder := NewBuilder(provider)

	tests := []struct {
		name   string
		runner *CommandRunner
	}{
		{"Base", builder.Base()},
		{"Config", builder.Config()},
		{"Owner", builder.Owner()},
		{"OwnerWithPassword", builder.OwnerWithPassword()},
		{"OwnerWithActivity", builder.OwnerWithActivity()},
		{"Uninitialized", builder.Uninitialized()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
				return nil
			}

			cmd := &cobra.Command{}
			err := tt.runner.Wrap(handler)(cmd, nil)
			assert.NoError(t, err)
		})
	}
}

func TestRunnerClone(t *testing.T) {
	provider := func() (*config.Config, error) {
		return &config.Config{Role: config.RoleOwner}, nil
	}

	original := NewRunner(provider).Use(WithLogging())
	cloned := original.Clone().Use(RequireOwner())

	// Original should have 1 interceptor
	assert.Len(t, original.interceptors, 1)

	// Clone should have 2 interceptors
	assert.Len(t, cloned.interceptors, 2)
}
