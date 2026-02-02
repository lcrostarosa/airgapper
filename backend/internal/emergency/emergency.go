// Package emergency provides emergency recovery functionality for Airgapper.
// This includes recovery shares, dead man's switch, notifications, and overrides.
package emergency

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// Config holds all emergency recovery configuration
type Config struct {
	Recovery      *RecoveryConfig      `json:"recovery,omitempty"`
	DeadManSwitch *DeadManSwitchConfig `json:"dead_man_switch,omitempty"`
	Override      *OverrideConfig      `json:"override,omitempty"`
	Notify        *NotifyConfig        `json:"notify,omitempty"`
}

// RecoveryConfig defines m-of-n recovery share settings
type RecoveryConfig struct {
	Enabled      bool        `json:"enabled"`
	Threshold    int         `json:"threshold"`    // k shares needed
	TotalShares  int         `json:"total_shares"` // n total shares
	Custodians   []Custodian `json:"custodians,omitempty"`
	ShareIndexes []byte      `json:"share_indexes,omitempty"`
}

// Custodian represents a third-party holding a recovery share
type Custodian struct {
	Name       string `json:"name"`
	Contact    string `json:"contact,omitempty"`
	ShareIndex byte   `json:"share_index"`
	ExportedAt string `json:"exported_at,omitempty"`
}

// DeadManSwitchConfig defines inactivity-triggered actions
type DeadManSwitchConfig struct {
	Enabled        bool          `json:"enabled"`
	InactivityDays int           `json:"inactivity_days"`
	WarningDays    int           `json:"warning_days"`
	LastActivity   time.Time     `json:"last_activity"`
	OnTrigger      TriggerAction `json:"on_trigger"`
}

// TriggerAction defines what happens when dead man's switch triggers
type TriggerAction struct {
	Action        string   `json:"action"` // "notify", "unlock-escrow", "auto-approve"
	NotifyEmails  []string `json:"notify_emails,omitempty"`
	NotifyWebhook string   `json:"notify_webhook,omitempty"`
}

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

// NotifyConfig defines notification settings
type NotifyConfig struct {
	Enabled   bool                `json:"enabled"`
	Providers map[string]Provider `json:"providers"`
	Events    EventConfig         `json:"events"`
}

// Provider defines a notification provider
type Provider struct {
	Type     string            `json:"type"`
	Enabled  bool              `json:"enabled"`
	Settings map[string]string `json:"settings"`
	Priority string            `json:"priority"`
}

// EventConfig defines which events trigger notifications
type EventConfig struct {
	BackupStarted      bool `json:"backup_started"`
	BackupCompleted    bool `json:"backup_completed"`
	BackupFailed       bool `json:"backup_failed"`
	RestoreRequested   bool `json:"restore_requested"`
	RestoreApproved    bool `json:"restore_approved"`
	RestoreDenied      bool `json:"restore_denied"`
	DeletionRequested  bool `json:"deletion_requested"`
	DeletionApproved   bool `json:"deletion_approved"`
	ConsensusReceived  bool `json:"consensus_received"`
	EmergencyTriggered bool `json:"emergency_triggered"`
	DeadManWarning     bool `json:"dead_man_warning"`
	HeartbeatMissed    bool `json:"heartbeat_missed"`
}

// NewConfig creates a new emergency config with defaults
func NewConfig() *Config {
	return &Config{}
}

// --- Nil-safe accessor methods for Config ---

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

// WithRecovery configures m-of-n recovery shares
func (c *Config) WithRecovery(threshold, totalShares int, custodians []string) *Config {
	indexes := make([]byte, totalShares)
	for i := 0; i < totalShares; i++ {
		indexes[i] = byte(i + 1)
	}

	c.Recovery = &RecoveryConfig{
		Enabled:      true,
		Threshold:    threshold,
		TotalShares:  totalShares,
		ShareIndexes: indexes,
	}

	// Add custodians (they get shares starting at index 3)
	now := time.Now().Format(time.RFC3339)
	for i, name := range custodians {
		if i+3 <= totalShares {
			c.Recovery.Custodians = append(c.Recovery.Custodians, Custodian{
				Name:       name,
				ShareIndex: byte(i + 3),
				ExportedAt: now,
			})
		}
	}

	return c
}

// WithDeadManSwitch configures the dead man's switch
func (c *Config) WithDeadManSwitch(inactivityDays int, contacts []string) *Config {
	warningDays := 7
	if inactivityDays < 30 {
		warningDays = inactivityDays / 4
	}

	c.DeadManSwitch = &DeadManSwitchConfig{
		Enabled:        true,
		InactivityDays: inactivityDays,
		WarningDays:    warningDays,
		LastActivity:   time.Now(),
		OnTrigger: TriggerAction{
			Action:       "notify",
			NotifyEmails: contacts,
		},
	}

	return c
}

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

// --- RecoveryConfig methods (nil-safe) ---

// IsEnabled returns true if recovery is enabled (nil-safe)
func (r *RecoveryConfig) IsEnabled() bool {
	return r != nil && r.Enabled
}

// GetThreshold returns threshold or default (nil-safe)
func (r *RecoveryConfig) GetThreshold() int {
	if r == nil || !r.Enabled {
		return 2
	}
	return r.Threshold
}

// GetTotalShares returns total shares or default (nil-safe)
func (r *RecoveryConfig) GetTotalShares() int {
	if r == nil || !r.Enabled {
		return 2
	}
	return r.TotalShares
}

// Validate checks if recovery config is valid
func (r *RecoveryConfig) Validate() error {
	if r == nil || !r.Enabled {
		return nil
	}
	if r.Threshold < 1 {
		return errors.New("threshold must be at least 1")
	}
	if r.TotalShares < r.Threshold {
		return errors.New("total shares must be >= threshold")
	}
	if r.TotalShares > 255 {
		return errors.New("total shares must be <= 255")
	}
	return nil
}

// --- DeadManSwitchConfig methods (nil-safe) ---

// IsEnabled returns true if dead man's switch is enabled (nil-safe)
func (d *DeadManSwitchConfig) IsEnabled() bool {
	return d != nil && d.Enabled
}

// RecordActivity updates the last activity timestamp (nil-safe)
func (d *DeadManSwitchConfig) RecordActivity() {
	if d != nil {
		d.LastActivity = time.Now()
	}
}

// DaysSinceActivity returns days since last recorded activity (nil-safe)
func (d *DeadManSwitchConfig) DaysSinceActivity() int {
	if d == nil || d.LastActivity.IsZero() {
		return -1
	}
	return int(time.Since(d.LastActivity).Hours() / 24)
}

// IsTriggered returns true if the switch should trigger (nil-safe)
func (d *DeadManSwitchConfig) IsTriggered() bool {
	if !d.IsEnabled() {
		return false
	}
	return d.DaysSinceActivity() >= d.InactivityDays
}

// IsWarning returns true if we're in the warning period (nil-safe)
func (d *DeadManSwitchConfig) IsWarning() bool {
	if !d.IsEnabled() {
		return false
	}
	days := d.DaysSinceActivity()
	threshold := d.InactivityDays - d.WarningDays
	return days >= threshold && days < d.InactivityDays
}

// DaysUntilTrigger returns days until the switch triggers (nil-safe)
func (d *DeadManSwitchConfig) DaysUntilTrigger() int {
	if d == nil {
		return -1
	}
	return d.InactivityDays - d.DaysSinceActivity()
}

// --- OverrideConfig methods (nil-safe) ---

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

// --- NotifyConfig methods (nil-safe) ---

// IsEnabled returns true if notifications are enabled (nil-safe)
func (n *NotifyConfig) IsEnabled() bool {
	return n != nil && n.Enabled
}

// HasProviders returns true if providers are configured (nil-safe)
func (n *NotifyConfig) HasProviders() bool {
	return n != nil && len(n.Providers) > 0
}

// ProviderCount returns the number of configured providers (nil-safe)
func (n *NotifyConfig) ProviderCount() int {
	if n == nil {
		return 0
	}
	return len(n.Providers)
}

// AddProvider adds a notification provider (nil-safe - no-op if nil)
func (n *NotifyConfig) AddProvider(id string, p Provider) {
	if n == nil {
		return
	}
	if n.Providers == nil {
		n.Providers = make(map[string]Provider)
	}
	n.Providers[id] = p
	n.Enabled = true
}

// RemoveProvider removes a notification provider (nil-safe)
func (n *NotifyConfig) RemoveProvider(id string) {
	if n == nil {
		return
	}
	delete(n.Providers, id)
	if len(n.Providers) == 0 {
		n.Enabled = false
	}
}

// EnableAllEvents enables all notification events (nil-safe)
func (n *NotifyConfig) EnableAllEvents() {
	if n == nil {
		return
	}
	n.Events = EventConfig{
		BackupStarted:      true,
		BackupCompleted:    true,
		BackupFailed:       true,
		RestoreRequested:   true,
		RestoreApproved:    true,
		RestoreDenied:      true,
		DeletionRequested:  true,
		DeletionApproved:   true,
		ConsensusReceived:  true,
		EmergencyTriggered: true,
		DeadManWarning:     true,
		HeartbeatMissed:    true,
	}
}

// DisableAllEvents disables all notification events (nil-safe)
func (n *NotifyConfig) DisableAllEvents() {
	if n == nil {
		return
	}
	n.Events = EventConfig{}
}
