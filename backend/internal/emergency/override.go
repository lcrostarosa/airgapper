package emergency

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// OverrideConfig defines emergency bypass settings
type OverrideConfig struct {
	Enabled         bool           `json:"enabled"`
	KeyHash         string         `json:"key_hash,omitempty"`
	AllowedTypes    []OverrideType `json:"allowed_types"`
	RequireReason   bool           `json:"require_reason"`
	CooldownMinutes int            `json:"cooldown_minutes,omitempty"`
	NotifyOnUse     bool           `json:"notify_on_use"`
}

// OverrideType defines types of emergency overrides
type OverrideType string

const (
	OverrideRestoreWithoutConsensus OverrideType = "restore-without-consensus"
	OverrideDeleteWithoutConsensus  OverrideType = "delete-without-consensus"
	OverrideBypassRetention         OverrideType = "bypass-retention"
	OverrideBypassDeadManSwitch     OverrideType = "bypass-dead-man-switch"
	OverrideForceUnlock             OverrideType = "force-unlock"
)

// WithOverrides enables the override system
func (c *Config) WithOverrides() *Config {
	c.Override = &OverrideConfig{
		Enabled: true,
		AllowedTypes: []OverrideType{
			OverrideRestoreWithoutConsensus,
		},
		RequireReason: true,
		NotifyOnUse:   true,
	}

	return c
}

// IsEnabled returns true if overrides are enabled (nil-safe)
func (o *OverrideConfig) IsEnabled() bool {
	return o != nil && o.Enabled
}

// HasKey returns true if an override key is configured (nil-safe)
func (o *OverrideConfig) HasKey() bool {
	return o != nil && o.KeyHash != ""
}

// GenerateKey creates a new override key and returns it (stores hash)
func (o *OverrideConfig) GenerateKey() (string, error) {
	if o == nil {
		return "", errors.New("override config is nil")
	}
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}

	key := "ov_" + hex.EncodeToString(keyBytes)
	o.KeyHash = hex.EncodeToString(keyBytes) // Simple hash for now
	o.Enabled = true

	return key, nil
}

// IsTypeAllowed checks if a specific override type is permitted (nil-safe)
func (o *OverrideConfig) IsTypeAllowed(t OverrideType) bool {
	if !o.IsEnabled() {
		return false
	}
	for _, allowed := range o.AllowedTypes {
		if allowed == t {
			return true
		}
	}
	return false
}

// AllowType adds an override type to the allowed list (nil-safe)
func (o *OverrideConfig) AllowType(t OverrideType) {
	if o == nil {
		return
	}
	for _, existing := range o.AllowedTypes {
		if existing == t {
			return
		}
	}
	o.AllowedTypes = append(o.AllowedTypes, t)
}

// DenyType removes an override type from the allowed list (nil-safe)
func (o *OverrideConfig) DenyType(t OverrideType) {
	if o == nil {
		return
	}
	filtered := make([]OverrideType, 0, len(o.AllowedTypes))
	for _, existing := range o.AllowedTypes {
		if existing != t {
			filtered = append(filtered, existing)
		}
	}
	o.AllowedTypes = filtered
}
