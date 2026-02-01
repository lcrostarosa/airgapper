package policy

import (
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func TestNewPolicy(t *testing.T) {
	// Generate keys for owner and host
	ownerPub, _, _ := crypto.GenerateKeyPair()
	hostPub, _, _ := crypto.GenerateKeyPair()

	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	p := NewPolicy(
		"Alice", ownerKeyID, crypto.EncodePublicKey(ownerPub),
		"Bob", hostKeyID, crypto.EncodePublicKey(hostPub),
	)

	if p.Version != 1 {
		t.Errorf("expected version 1, got %d", p.Version)
	}
	if p.ID == "" {
		t.Error("expected policy ID to be set")
	}
	if p.OwnerName != "Alice" {
		t.Errorf("expected owner name Alice, got %s", p.OwnerName)
	}
	if p.HostName != "Bob" {
		t.Errorf("expected host name Bob, got %s", p.HostName)
	}
	if p.RetentionDays != 30 {
		t.Errorf("expected default retention 30 days, got %d", p.RetentionDays)
	}
	if p.DeletionMode != DeletionBothRequired {
		t.Errorf("expected default deletion mode both-required, got %s", p.DeletionMode)
	}
	if !p.AppendOnlyLocked {
		t.Error("expected append-only to be locked by default")
	}
}

func TestPolicySignAndVerify(t *testing.T) {
	// Generate keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()

	p := NewPolicy(
		"Alice", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"Bob", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)

	// Initially not signed
	if p.IsFullySigned() {
		t.Error("policy should not be fully signed initially")
	}

	// Owner signs
	if err := p.SignAsOwner(ownerPriv); err != nil {
		t.Fatalf("owner sign failed: %v", err)
	}

	if p.OwnerSignature == "" {
		t.Error("owner signature should be set")
	}

	// Verify owner signature
	if err := p.VerifyOwnerSignature(); err != nil {
		t.Errorf("owner signature verification failed: %v", err)
	}

	// Still not fully signed
	if p.IsFullySigned() {
		t.Error("policy should not be fully signed with only owner signature")
	}

	// Host signs
	if err := p.SignAsHost(hostPriv); err != nil {
		t.Fatalf("host sign failed: %v", err)
	}

	// Now fully signed
	if !p.IsFullySigned() {
		t.Error("policy should be fully signed")
	}

	// Verify both
	if err := p.Verify(); err != nil {
		t.Errorf("full verification failed: %v", err)
	}
}

func TestPolicyTamperDetection(t *testing.T) {
	// Generate keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()

	p := NewPolicy(
		"Alice", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"Bob", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)

	// Sign by both
	p.SignAsOwner(ownerPriv)
	p.SignAsHost(hostPriv)

	// Verify works
	if err := p.Verify(); err != nil {
		t.Fatalf("initial verification failed: %v", err)
	}

	// Tamper with retention
	p.RetentionDays = 1

	// Verification should fail
	if err := p.Verify(); err == nil {
		t.Error("verification should fail after tampering")
	}
}

func TestPolicyCanDelete(t *testing.T) {
	ownerPub, _, _ := crypto.GenerateKeyPair()
	hostPub, _, _ := crypto.GenerateKeyPair()

	tests := []struct {
		name          string
		retentionDays int
		deletionMode  DeletionMode
		fileAge       time.Duration
		wantAllowed   bool
	}{
		{
			name:          "retention not met",
			retentionDays: 30,
			deletionMode:  DeletionTimeLockOnly,
			fileAge:       15 * 24 * time.Hour,
			wantAllowed:   false,
		},
		{
			name:          "retention met, time-lock-only",
			retentionDays: 30,
			deletionMode:  DeletionTimeLockOnly,
			fileAge:       35 * 24 * time.Hour,
			wantAllowed:   true,
		},
		{
			name:          "retention met, owner-only",
			retentionDays: 30,
			deletionMode:  DeletionOwnerOnly,
			fileAge:       35 * 24 * time.Hour,
			wantAllowed:   false, // Requires approval
		},
		{
			name:          "retention met, both-required",
			retentionDays: 30,
			deletionMode:  DeletionBothRequired,
			fileAge:       35 * 24 * time.Hour,
			wantAllowed:   false, // Requires approval
		},
		{
			name:          "never delete",
			retentionDays: 0,
			deletionMode:  DeletionNever,
			fileAge:       365 * 24 * time.Hour,
			wantAllowed:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPolicy(
				"Alice", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
				"Bob", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
			)
			p.RetentionDays = tt.retentionDays
			p.DeletionMode = tt.deletionMode

			fileCreated := time.Now().Add(-tt.fileAge)
			allowed, _ := p.CanDelete(fileCreated)

			if allowed != tt.wantAllowed {
				t.Errorf("CanDelete() = %v, want %v", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestPolicyIsActive(t *testing.T) {
	ownerPub, _, _ := crypto.GenerateKeyPair()
	hostPub, _, _ := crypto.GenerateKeyPair()

	p := NewPolicy(
		"Alice", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"Bob", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)

	// Should be active by default
	if !p.IsActive() {
		t.Error("new policy should be active")
	}

	// Set effective date in future
	p.EffectiveAt = time.Now().Add(24 * time.Hour)
	if p.IsActive() {
		t.Error("policy should not be active before effective date")
	}

	// Set effective date in past, expiry in future
	p.EffectiveAt = time.Now().Add(-24 * time.Hour)
	p.ExpiresAt = time.Now().Add(24 * time.Hour)
	if !p.IsActive() {
		t.Error("policy should be active between effective and expiry")
	}

	// Set expiry in past
	p.ExpiresAt = time.Now().Add(-1 * time.Hour)
	if p.IsActive() {
		t.Error("policy should not be active after expiry")
	}
}

func TestPolicyJSON(t *testing.T) {
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()

	p := NewPolicy(
		"Alice", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"Bob", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)
	p.Name = "Test Policy"
	p.RetentionDays = 90
	p.DeletionMode = DeletionOwnerOnly

	p.SignAsOwner(ownerPriv)
	p.SignAsHost(hostPriv)

	// Serialize
	data, err := p.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize
	p2, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	// Verify
	if err := p2.Verify(); err != nil {
		t.Errorf("verification after round-trip failed: %v", err)
	}

	if p2.Name != "Test Policy" {
		t.Errorf("name mismatch: got %s", p2.Name)
	}
	if p2.RetentionDays != 90 {
		t.Errorf("retention mismatch: got %d", p2.RetentionDays)
	}
	if p2.DeletionMode != DeletionOwnerOnly {
		t.Errorf("deletion mode mismatch: got %s", p2.DeletionMode)
	}
}
