// Package emergency provides emergency recovery functionality for Airgapper.
// This includes recovery shares, dead man's switch, notifications, and overrides.
package emergency

// Config holds all emergency recovery configuration
type Config struct {
	Recovery      *RecoveryConfig      `json:"recovery,omitempty"`
	DeadManSwitch *DeadManSwitchConfig `json:"dead_man_switch,omitempty"`
	Override      *OverrideConfig      `json:"override,omitempty"`
	Notify        *NotifyConfig        `json:"notify,omitempty"`
}

// NewConfig creates a new emergency config with defaults
func NewConfig() *Config {
	return &Config{}
}

// GetRecovery returns recovery config (nil-safe)
func (c *Config) GetRecovery() *RecoveryConfig {
	if c == nil {
		return nil
	}
	return c.Recovery
}

// GetDeadManSwitch returns dead man's switch config (nil-safe)
func (c *Config) GetDeadManSwitch() *DeadManSwitchConfig {
	if c == nil {
		return nil
	}
	return c.DeadManSwitch
}

// GetOverride returns override config (nil-safe)
func (c *Config) GetOverride() *OverrideConfig {
	if c == nil {
		return nil
	}
	return c.Override
}

// GetNotify returns notify config (nil-safe)
func (c *Config) GetNotify() *NotifyConfig {
	if c == nil {
		return nil
	}
	return c.Notify
}
