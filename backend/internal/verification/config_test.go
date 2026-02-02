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

	// Test default challenge config
	cc := DefaultChallengeConfig()
	if !cc.Enabled {
		t.Error("default challenge should be enabled")
	}
	if cc.ExpiryMinutes != 60 {
		t.Errorf("expected 60 expiry minutes, got %d", cc.ExpiryMinutes)
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

	// Test default witness config
	wc := DefaultWitnessConfig()
	if wc.Enabled {
		t.Error("default witness should be disabled")
	}
	if !wc.AutoSubmit {
		t.Error("default witness should have auto-submit enabled when used")
	}
	if wc.IntervalMinutes != 60 {
		t.Errorf("expected 60 interval minutes, got %d", wc.IntervalMinutes)
	}

	// Test default verification config
	vc := DefaultVerificationConfig()
	if !vc.Enabled {
		t.Error("default verification should be enabled")
	}
	if vc.AuditChain == nil {
		t.Error("default verification should have audit chain config")
	}
	if vc.Challenge == nil {
		t.Error("default verification should have challenge config")
	}
	if vc.Tickets == nil {
		t.Error("default verification should have ticket config")
	}
	if vc.Witness == nil {
		t.Error("default verification should have witness config")
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
			name:     "nil config - challenge",
			config:   nil,
			method:   (*VerificationSystemConfig).IsChallengeEnabled,
			expected: false,
		},
		{
			name: "fully enabled challenge",
			config: &VerificationSystemConfig{
				Enabled:   true,
				Challenge: &ChallengeConfig{Enabled: true},
			},
			method:   (*VerificationSystemConfig).IsChallengeEnabled,
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
		{
			name:     "nil config - witness",
			config:   nil,
			method:   (*VerificationSystemConfig).IsWitnessEnabled,
			expected: false,
		},
		{
			name: "fully enabled witness",
			config: &VerificationSystemConfig{
				Enabled: true,
				Witness: &WitnessConfig{Enabled: true},
			},
			method:   (*VerificationSystemConfig).IsWitnessEnabled,
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

func TestWitnessProvider(t *testing.T) {
	provider := WitnessProvider{
		Name:    "test-witness",
		Type:    "http",
		URL:     "https://witness.example.com/api",
		APIKey:  "secret-key",
		Headers: map[string]string{"X-Custom": "value"},
		Enabled: true,
	}

	if provider.Name != "test-witness" {
		t.Errorf("expected name 'test-witness', got '%s'", provider.Name)
	}
	if provider.Type != "http" {
		t.Errorf("expected type 'http', got '%s'", provider.Type)
	}
	if provider.URL != "https://witness.example.com/api" {
		t.Errorf("unexpected URL: %s", provider.URL)
	}
	if !provider.Enabled {
		t.Error("provider should be enabled")
	}
}
