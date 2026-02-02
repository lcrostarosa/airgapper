// Package verification implements host verification features including
// cryptographic audit chains, challenge-response protocols, deletion tickets,
// and external witness services. All features are optional and disabled by default.
package verification

// VerificationSystemConfig contains all verification feature settings.
// All features are disabled by default for backward compatibility.
type VerificationSystemConfig struct {
	Enabled    bool               `json:"enabled"`
	AuditChain *AuditChainConfig  `json:"audit_chain,omitempty"`
	Challenge  *ChallengeConfig   `json:"challenge,omitempty"`
	Tickets    *TicketConfig      `json:"tickets,omitempty"`
	Witness    *WitnessConfig     `json:"witness,omitempty"`

	// Additional defenses
	Quarantine *QuarantineConfig       `json:"quarantine,omitempty"`
	RateLimit  *RateLimitConfig        `json:"rate_limit,omitempty"`
	Anomaly    *AnomalyConfig          `json:"anomaly,omitempty"`
	Immutable  *ImmutableStorageConfig `json:"immutable,omitempty"`

	// Advanced detection features
	Canary    *CanaryConfig    `json:"canary,omitempty"`
	Heartbeat *HeartbeatConfig `json:"heartbeat,omitempty"`
	POR       *PORConfig       `json:"por,omitempty"`
}

// AuditChainConfig configures the cryptographic audit chain feature.
// When enabled, all operations are logged with hash-chaining and optional signatures.
type AuditChainConfig struct {
	Enabled       bool `json:"enabled"`
	SignEntries   bool `json:"sign_entries"`    // Sign each entry with host key
	RetentionDays int  `json:"retention_days"`  // Days to retain entries (0 = forever)
}

// ChallengeConfig configures the challenge-response protocol.
// Allows owner to verify host is storing files correctly.
type ChallengeConfig struct {
	Enabled       bool `json:"enabled"`
	ExpiryMinutes int  `json:"expiry_minutes"` // Challenge validity (default: 60)
}

// TicketConfig configures the deletion ticket system.
// When enabled, deletions require owner-signed tickets.
type TicketConfig struct {
	Enabled             bool `json:"enabled"`
	RequireForSnapshots bool `json:"require_for_snapshots"` // Require tickets for snapshot deletion
	RequireForPrune     bool `json:"require_for_prune"`     // Require tickets for prune operations
	ValidityDays        int  `json:"validity_days"`         // Ticket validity (default: 7)
}

// WitnessConfig configures the external witness service.
// Allows independent third-party verification of audit chain.
type WitnessConfig struct {
	Enabled         bool              `json:"enabled"`
	AutoSubmit      bool              `json:"auto_submit"`       // Automatically submit checkpoints
	IntervalMinutes int               `json:"interval_minutes"`  // Auto-submit interval (default: 60)
	Providers       []WitnessProvider `json:"providers,omitempty"`
}

// WitnessProvider defines an external witness service endpoint.
type WitnessProvider struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`               // "http" or "airgapper"
	URL      string            `json:"url"`
	APIKey   string            `json:"api_key,omitempty"`  // For authenticated services
	Headers  map[string]string `json:"headers,omitempty"`  // Custom headers
	Enabled  bool              `json:"enabled"`
}

// Note: CanaryConfig is defined in canary.go
// Note: HeartbeatConfig is defined in heartbeat.go
// Note: PORConfig is defined in por.go

// DefaultAuditChainConfig returns sensible defaults for audit chain.
func DefaultAuditChainConfig() *AuditChainConfig {
	return &AuditChainConfig{
		Enabled:       true,
		SignEntries:   true,
		RetentionDays: 365, // 1 year
	}
}

// DefaultChallengeConfig returns sensible defaults for challenges.
func DefaultChallengeConfig() *ChallengeConfig {
	return &ChallengeConfig{
		Enabled:       true,
		ExpiryMinutes: 60,
	}
}

// DefaultTicketConfig returns sensible defaults for tickets.
func DefaultTicketConfig() *TicketConfig {
	return &TicketConfig{
		Enabled:             true,
		RequireForSnapshots: true,
		RequireForPrune:     true,
		ValidityDays:        7,
	}
}

// DefaultWitnessConfig returns sensible defaults for witness service.
func DefaultWitnessConfig() *WitnessConfig {
	return &WitnessConfig{
		Enabled:         false, // Disabled by default as it requires external setup
		AutoSubmit:      true,
		IntervalMinutes: 60,
		Providers:       []WitnessProvider{},
	}
}

// DefaultVerificationConfig returns a default verification config with
// core features enabled but witness disabled (requires external setup).
func DefaultVerificationConfig() *VerificationSystemConfig {
	return &VerificationSystemConfig{
		Enabled:    true,
		AuditChain: DefaultAuditChainConfig(),
		Challenge:  DefaultChallengeConfig(),
		Tickets:    DefaultTicketConfig(),
		Witness:    DefaultWitnessConfig(),
		// Advanced detection features (disabled by default for backward compatibility)
		Canary:    nil,
		Heartbeat: nil,
		POR:       nil,
	}
}

// DefaultAdvancedVerificationConfig returns a verification config with
// all features enabled including advanced detection mechanisms.
func DefaultAdvancedVerificationConfig() *VerificationSystemConfig {
	return &VerificationSystemConfig{
		Enabled:    true,
		AuditChain: DefaultAuditChainConfig(),
		Challenge:  DefaultChallengeConfig(),
		Tickets:    DefaultTicketConfig(),
		Witness:    DefaultWitnessConfig(),
		// Advanced detection features
		Canary:    DefaultCanaryConfig(),
		Heartbeat: DefaultHeartbeatConfig(),
		POR:       DefaultPORConfig(),
	}
}

// IsAuditChainEnabled returns true if audit chain is configured and enabled.
func (c *VerificationSystemConfig) IsAuditChainEnabled() bool {
	return c != nil && c.Enabled && c.AuditChain != nil && c.AuditChain.Enabled
}

// IsChallengeEnabled returns true if challenge-response is configured and enabled.
func (c *VerificationSystemConfig) IsChallengeEnabled() bool {
	return c != nil && c.Enabled && c.Challenge != nil && c.Challenge.Enabled
}

// IsTicketsEnabled returns true if deletion tickets are configured and enabled.
func (c *VerificationSystemConfig) IsTicketsEnabled() bool {
	return c != nil && c.Enabled && c.Tickets != nil && c.Tickets.Enabled
}

// IsWitnessEnabled returns true if witness service is configured and enabled.
func (c *VerificationSystemConfig) IsWitnessEnabled() bool {
	return c != nil && c.Enabled && c.Witness != nil && c.Witness.Enabled
}

// IsQuarantineEnabled returns true if time-delayed deletions are configured and enabled.
func (c *VerificationSystemConfig) IsQuarantineEnabled() bool {
	return c != nil && c.Enabled && c.Quarantine != nil && c.Quarantine.Enabled
}

// IsRateLimitEnabled returns true if rate limiting is configured and enabled.
func (c *VerificationSystemConfig) IsRateLimitEnabled() bool {
	return c != nil && c.Enabled && c.RateLimit != nil && c.RateLimit.Enabled
}

// IsAnomalyDetectionEnabled returns true if anomaly detection is configured and enabled.
func (c *VerificationSystemConfig) IsAnomalyDetectionEnabled() bool {
	return c != nil && c.Enabled && c.Anomaly != nil && c.Anomaly.Enabled
}

// IsImmutableStorageEnabled returns true if immutable storage is configured and enabled.
func (c *VerificationSystemConfig) IsImmutableStorageEnabled() bool {
	return c != nil && c.Enabled && c.Immutable != nil && c.Immutable.Enabled
}

// IsCanaryEnabled returns true if canary detection is configured and enabled.
func (c *VerificationSystemConfig) IsCanaryEnabled() bool {
	return c != nil && c.Enabled && c.Canary != nil && c.Canary.Enabled
}

// IsHeartbeatEnabled returns true if heartbeat monitoring is configured and enabled.
func (c *VerificationSystemConfig) IsHeartbeatEnabled() bool {
	return c != nil && c.Enabled && c.Heartbeat != nil && c.Heartbeat.Enabled
}

// IsPOREnabled returns true if Proof of Retrievability is configured and enabled.
func (c *VerificationSystemConfig) IsPOREnabled() bool {
	return c != nil && c.Enabled && c.POR != nil && c.POR.Enabled
}
