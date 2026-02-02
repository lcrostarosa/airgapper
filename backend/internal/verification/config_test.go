package verification

import (
	"testing"
)

func TestDefaultConfigs(t *testing.T) {
	// Test default audit chain config
	ac := DefaultAuditChainConfig()
	if !ac.Enabled {
		t.Error("default audit chain should be enabled")
	}
	if !ac.SignEntries {
		t.Error("default audit chain should sign entries")
	}
	if ac.RetentionDays != 365 {
		t.Errorf("expected 365 retention days, got %d", ac.RetentionDays)
	}

	// Test default ticket config
	tc := DefaultTicketConfig()
	if !tc.Enabled {
		t.Error("default ticket should be enabled")
	}
	if !tc.RequireForSnapshots {
		t.Error("default should require tickets for snapshots")
	}
	if !tc.RequireForPrune {
		t.Error("default should require tickets for prune")
	}
	if tc.ValidityDays != 7 {
		t.Errorf("expected 7 validity days, got %d", tc.ValidityDays)
	}

	// Test default verification config
	vc := DefaultVerificationConfig()
	if !vc.Enabled {
		t.Error("default verification should be enabled")
	}
	if vc.AuditChain == nil {
		t.Error("default verification should have audit chain config")
	}
	if vc.Tickets == nil {
		t.Error("default verification should have ticket config")
	}
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
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
