package verification

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfigs(t *testing.T) {
	// Test default audit chain config
	ac := DefaultAuditChainConfig()
	assert.True(t, ac.Enabled, "default audit chain should be enabled")
	assert.True(t, ac.SignEntries, "default audit chain should sign entries")
	assert.Equal(t, 365, ac.RetentionDays)

	// Test default ticket config
	tc := DefaultTicketConfig()
	assert.True(t, tc.Enabled, "default ticket should be enabled")
	assert.True(t, tc.RequireForSnapshots, "default should require tickets for snapshots")
	assert.True(t, tc.RequireForPrune, "default should require tickets for prune")
	assert.Equal(t, 7, tc.ValidityDays)

	// Test default verification config
	vc := DefaultVerificationConfig()
	assert.True(t, vc.Enabled, "default verification should be enabled")
	assert.NotNil(t, vc.AuditChain, "default verification should have audit chain config")
	assert.NotNil(t, vc.Tickets, "default verification should have ticket config")
}

func TestVerificationSystemConfig_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   *VerificationSystemConfig
		method   func(*VerificationSystemConfig) bool
		expected bool
	}{
		{
			name:     "nil config - audit chain",
			config:   nil,
			method:   (*VerificationSystemConfig).IsAuditChainEnabled,
			expected: false,
		},
		{
			name:     "disabled config - audit chain",
			config:   &VerificationSystemConfig{Enabled: false},
			method:   (*VerificationSystemConfig).IsAuditChainEnabled,
			expected: false,
		},
		{
			name: "enabled config but nil audit chain",
			config: &VerificationSystemConfig{
				Enabled:    true,
				AuditChain: nil,
			},
			method:   (*VerificationSystemConfig).IsAuditChainEnabled,
			expected: false,
		},
		{
			name: "enabled config with disabled audit chain",
			config: &VerificationSystemConfig{
				Enabled:    true,
				AuditChain: &AuditChainConfig{Enabled: false},
			},
			method:   (*VerificationSystemConfig).IsAuditChainEnabled,
			expected: false,
		},
		{
			name: "fully enabled audit chain",
			config: &VerificationSystemConfig{
				Enabled:    true,
				AuditChain: &AuditChainConfig{Enabled: true},
			},
			method:   (*VerificationSystemConfig).IsAuditChainEnabled,
			expected: true,
		},
		{
			name:     "nil config - tickets",
			config:   nil,
			method:   (*VerificationSystemConfig).IsTicketsEnabled,
			expected: false,
		},
		{
			name: "fully enabled tickets",
			config: &VerificationSystemConfig{
				Enabled: true,
				Tickets: &TicketConfig{Enabled: true},
			},
			method:   (*VerificationSystemConfig).IsTicketsEnabled,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}
