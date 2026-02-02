package policy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	assert.Equal(t, 1, p.Version)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "Alice", p.OwnerName)
	assert.Equal(t, "Bob", p.HostName)
	assert.Equal(t, 30, p.RetentionDays)
	assert.Equal(t, DeletionBothRequired, p.DeletionMode)
	assert.True(t, p.AppendOnlyLocked)
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
	assert.False(t, p.IsFullySigned(), "policy should not be fully signed initially")

	// Owner signs
	require.NoError(t, p.SignAsOwner(ownerPriv), "owner sign failed")
	assert.NotEmpty(t, p.OwnerSignature, "owner signature should be set")

	// Verify owner signature
	assert.NoError(t, p.VerifyOwnerSignature(), "owner signature verification failed")

	// Still not fully signed
	assert.False(t, p.IsFullySigned(), "policy should not be fully signed with only owner signature")

	// Host signs
	require.NoError(t, p.SignAsHost(hostPriv), "host sign failed")

	// Now fully signed
	assert.True(t, p.IsFullySigned(), "policy should be fully signed")

	// Verify both
	assert.NoError(t, p.Verify(), "full verification failed")
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
	require.NoError(t, p.Verify(), "initial verification failed")

	// Tamper with retention
	p.RetentionDays = 1

	// Verification should fail
	assert.Error(t, p.Verify(), "verification should fail after tampering")
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

			assert.Equal(t, tt.wantAllowed, allowed)
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
	assert.True(t, p.IsActive(), "new policy should be active")

	// Set effective date in future
	p.EffectiveAt = time.Now().Add(24 * time.Hour)
	assert.False(t, p.IsActive(), "policy should not be active before effective date")

	// Set effective date in past, expiry in future
	p.EffectiveAt = time.Now().Add(-24 * time.Hour)
	p.ExpiresAt = time.Now().Add(24 * time.Hour)
	assert.True(t, p.IsActive(), "policy should be active between effective and expiry")

	// Set expiry in past
	p.ExpiresAt = time.Now().Add(-1 * time.Hour)
	assert.False(t, p.IsActive(), "policy should not be active after expiry")
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
	require.NoError(t, err, "ToJSON failed")

	// Deserialize
	p2, err := FromJSON(data)
	require.NoError(t, err, "FromJSON failed")

	// Verify
	assert.NoError(t, p2.Verify(), "verification after round-trip failed")
	assert.Equal(t, "Test Policy", p2.Name)
	assert.Equal(t, 90, p2.RetentionDays)
	assert.Equal(t, DeletionOwnerOnly, p2.DeletionMode)
}
