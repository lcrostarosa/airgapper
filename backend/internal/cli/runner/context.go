package runner

import (
	"fmt"
	"sync"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
)

// CommandContext provides shared dependencies to command handlers.
// Dependencies are lazily initialized on first access to avoid unnecessary work.
type CommandContext struct {
	// Config is the loaded configuration (may be nil if not initialized)
	Config *config.Config

	// ConfigErr is the error from loading config, if any
	ConfigErr error

	consentMgr  *consent.Manager
	consentOnce sync.Once
}

// NewContext creates a new CommandContext with the given config.
func NewContext(cfg *config.Config, cfgErr error) *CommandContext {
	return &CommandContext{
		Config:    cfg,
		ConfigErr: cfgErr,
	}
}

// Consent returns a lazily-initialized consent manager.
// Returns nil if config is not loaded.
func (c *CommandContext) Consent() *consent.Manager {
	c.consentOnce.Do(func() {
		if c.Config != nil && c.Config.ConfigDir != "" {
			c.consentMgr = consent.NewManager(c.Config.ConfigDir)
		}
	})
	return c.consentMgr
}

// SaveConfig saves the configuration with standardized error wrapping.
func (c *CommandContext) SaveConfig() error {
	if c.Config == nil {
		return ErrNotInitialized
	}
	if err := c.Config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// HasConfig returns true if config is loaded successfully.
func (c *CommandContext) HasConfig() bool {
	return c.Config != nil && c.ConfigErr == nil
}

// IsOwner returns true if the current role is owner.
func (c *CommandContext) IsOwner() bool {
	return c.Config != nil && c.Config.IsOwner()
}

// IsHost returns true if the current role is host.
func (c *CommandContext) IsHost() bool {
	return c.Config != nil && c.Config.IsHost()
}

// HasPassword returns true if a password is available.
func (c *CommandContext) HasPassword() bool {
	return c.Config != nil && c.Config.Password != ""
}

// HasPrivateKey returns true if a private key is available.
func (c *CommandContext) HasPrivateKey() bool {
	return c.Config != nil && c.Config.PrivateKey != nil
}

// HasShare returns true if a local share is available.
func (c *CommandContext) HasShare() bool {
	return c.Config != nil && c.Config.LocalShare != nil
}
