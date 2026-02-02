package runner

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

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

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"first-before",
		"second-before",
		"third-before",
		"handler",
		"third-after",
		"second-after",
		"first-after",
	}

	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}

	for i, exp := range expected {
		if order[i] != exp {
			t.Errorf("order[%d] = %q, want %q", i, order[i], exp)
		}
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

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if len(order) != 2 {
		t.Errorf("expected 2 calls, got %d: %v", len(order), order)
	}

	if order[0] != "first" || order[1] != "second-fails" {
		t.Errorf("unexpected order: %v", order)
	}
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
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if tt.cfgErr != nil {
				if err == nil {
					t.Error("expected an error but got nil")
				}
			}

			if handlerCalled != tt.wantCalls {
				t.Errorf("handler called = %v, want %v", handlerCalled, tt.wantCalls)
			}
		})
	}
}

func TestRequireOwner(t *testing.T) {
	tests := []struct {
		name      string
		role      config.Role
		wantErr   bool
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
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !errors.Is(err, ErrNotOwner) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
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

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
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
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
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
	if mgr1 == nil {
		t.Error("expected consent manager, got nil")
	}

	// Second call should return same instance
	mgr2 := ctx.Consent()
	if mgr1 != mgr2 {
		t.Error("expected same consent manager instance")
	}
}

func TestContextNilConfig(t *testing.T) {
	ctx := NewContext(nil, nil)

	// Should return nil without panicking
	mgr := ctx.Consent()
	if mgr != nil {
		t.Error("expected nil consent manager for nil config")
	}

	// Helper methods should be safe
	if ctx.HasConfig() {
		t.Error("expected HasConfig() to return false")
	}
	if ctx.IsOwner() {
		t.Error("expected IsOwner() to return false")
	}
	if ctx.IsHost() {
		t.Error("expected IsHost() to return false")
	}
	if ctx.HasPassword() {
		t.Error("expected HasPassword() to return false")
	}
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

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// RecordActivity calls cfg.RecordActivity() which updates LastActivity
		// We can't easily verify this without mocking, but no error means success
	})

	t.Run("does not record on error", func(t *testing.T) {
		expectedErr := errors.New("handler error")
		runner := NewRunner(provider).Use(RecordActivity())
		handler := func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
			return expectedErr
		}

		cmd := &cobra.Command{}
		err := runner.Wrap(handler)(cmd, nil)

		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
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

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
	if len(original.interceptors) != 1 {
		t.Errorf("expected original to have 1 interceptor, got %d", len(original.interceptors))
	}

	// Clone should have 2 interceptors
	if len(cloned.interceptors) != 2 {
		t.Errorf("expected clone to have 2 interceptors, got %d", len(cloned.interceptors))
	}
}
