// Package verification implements host verification features including
// cryptographic audit chains and deletion tickets.
// All features are optional and disabled by default.
package verification

// VerificationSystemConfig contains all verification feature settings.
// All features are disabled by default for backward compatibility.
type VerificationSystemConfig struct {
	Enabled    bool              `json:"enabled"`
	AuditChain *AuditChainConfig `json:"audit_chain,omitempty"`
	Tickets    *TicketConfig     `json:"tickets,omitempty"`
}

// AuditChainConfig configures the cryptographic audit chain feature.
// When enabled, all operations are logged with hash-chaining and optional signatures.
type AuditChainConfig struct {
	Enabled       bool `json:"enabled"`
	SignEntries   bool `json:"sign_entries"`   // Sign each entry with host key
	RetentionDays int  `json:"retention_days"` // Days to retain entries (0 = forever)
}

// TicketConfig configures the deletion ticket system.
// When enabled, deletions require owner-signed tickets.
type TicketConfig struct {
	Enabled             bool `json:"enabled"`
	RequireForSnapshots bool `json:"require_for_snapshots"` // Require tickets for snapshot deletion
	RequireForPrune     bool `json:"require_for_prune"`     // Require tickets for prune operations
	ValidityDays        int  `json:"validity_days"`         // Ticket validity (default: 7)
}

// DefaultAuditChainConfig returns sensible defaults for audit chain.
func DefaultAuditChainConfig() *AuditChainConfig {
	return &AuditChainConfig{
		Enabled:       true,
		SignEntries:   true,
		RetentionDays: 365, // 1 year
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

// DefaultVerificationConfig returns a default verification config with
// core features enabled.
func DefaultVerificationConfig() *VerificationSystemConfig {
	return &VerificationSystemConfig{
		Enabled:    true,
		AuditChain: DefaultAuditChainConfig(),
		Tickets:    DefaultTicketConfig(),
	}
}

// IsAuditChainEnabled returns true if audit chain is configured and enabled.
func (c *VerificationSystemConfig) IsAuditChainEnabled() bool {
	return c != nil && c.Enabled && c.AuditChain != nil && c.AuditChain.Enabled
}

// IsTicketsEnabled returns true if deletion tickets are configured and enabled.
func (c *VerificationSystemConfig) IsTicketsEnabled() bool {
	return c != nil && c.Enabled && c.Tickets != nil && c.Tickets.Enabled
}
