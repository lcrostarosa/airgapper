// Package policy implements cryptographically signed contracts between
// backup owner and host that define retention periods, deletion rules,
// and data protection terms.
package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// DeletionMode defines when and how data can be deleted
type DeletionMode string

const (
	// DeletionBothRequired - Both owner and host must approve deletion
	DeletionBothRequired DeletionMode = "both-required"

	// DeletionOwnerOnly - Only owner can approve deletion (after retention)
	DeletionOwnerOnly DeletionMode = "owner-only"

	// DeletionTimeLockOnly - No approval needed after retention period
	DeletionTimeLockOnly DeletionMode = "time-lock-only"

	// DeletionNever - Data can never be deleted (archival mode)
	DeletionNever DeletionMode = "never"
)

// Policy defines the terms agreed upon by owner and host
type Policy struct {
	// Version for future compatibility
	Version int `json:"version"`

	// Unique identifier for this policy
	ID string `json:"id"`

	// Human-readable policy name
	Name string `json:"name,omitempty"`

	// Parties involved
	OwnerName    string `json:"owner_name"`
	OwnerKeyID   string `json:"owner_key_id"`
	OwnerPubKey  string `json:"owner_public_key"` // hex-encoded Ed25519
	HostName     string `json:"host_name"`
	HostKeyID    string `json:"host_key_id"`
	HostPubKey   string `json:"host_public_key"` // hex-encoded Ed25519

	// Data protection terms
	RetentionDays    int          `json:"retention_days"`     // Minimum days before deletion allowed
	DeletionMode     DeletionMode `json:"deletion_mode"`      // How deletion is authorized
	AppendOnlyLocked bool         `json:"append_only_locked"` // If true, append-only cannot be disabled

	// Storage terms
	MaxStorageBytes int64 `json:"max_storage_bytes,omitempty"` // 0 = unlimited

	// Timestamps
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"` // Optional expiry
	EffectiveAt time.Time `json:"effective_at"`        // When policy becomes active

	// Signatures (set after both parties sign)
	OwnerSignature string `json:"owner_signature,omitempty"` // hex-encoded
	HostSignature  string `json:"host_signature,omitempty"`  // hex-encoded
}

// PolicySignData is the canonical data structure for signing
// (excludes signatures themselves)
type PolicySignData struct {
	Version          int          `json:"version"`
	ID               string       `json:"id"`
	Name             string       `json:"name,omitempty"`
	OwnerName        string       `json:"owner_name"`
	OwnerKeyID       string       `json:"owner_key_id"`
	OwnerPubKey      string       `json:"owner_public_key"`
	HostName         string       `json:"host_name"`
	HostKeyID        string       `json:"host_key_id"`
	HostPubKey       string       `json:"host_public_key"`
	RetentionDays    int          `json:"retention_days"`
	DeletionMode     DeletionMode `json:"deletion_mode"`
	AppendOnlyLocked bool         `json:"append_only_locked"`
	MaxStorageBytes  int64        `json:"max_storage_bytes,omitempty"`
	CreatedAt        int64        `json:"created_at"`  // Unix timestamp
	ExpiresAt        int64        `json:"expires_at"`  // Unix timestamp, 0 if not set
	EffectiveAt      int64        `json:"effective_at"` // Unix timestamp
}

// NewPolicy creates a new unsigned policy
func NewPolicy(ownerName, ownerKeyID, ownerPubKey, hostName, hostKeyID, hostPubKey string) *Policy {
	now := time.Now()
	id := generatePolicyID()

	return &Policy{
		Version:          1,
		ID:               id,
		OwnerName:        ownerName,
		OwnerKeyID:       ownerKeyID,
		OwnerPubKey:      ownerPubKey,
		HostName:         hostName,
		HostKeyID:        hostKeyID,
		HostPubKey:       hostPubKey,
		RetentionDays:    30, // Default: 30 days
		DeletionMode:     DeletionBothRequired,
		AppendOnlyLocked: true,
		CreatedAt:        now,
		EffectiveAt:      now,
	}
}

func generatePolicyID() string {
	// Generate a random policy ID using timestamp
	data := fmt.Sprintf("%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// Hash creates a canonical hash of the policy for signing
func (p *Policy) Hash() ([]byte, error) {
	signData := PolicySignData{
		Version:          p.Version,
		ID:               p.ID,
		Name:             p.Name,
		OwnerName:        p.OwnerName,
		OwnerKeyID:       p.OwnerKeyID,
		OwnerPubKey:      p.OwnerPubKey,
		HostName:         p.HostName,
		HostKeyID:        p.HostKeyID,
		HostPubKey:       p.HostPubKey,
		RetentionDays:    p.RetentionDays,
		DeletionMode:     p.DeletionMode,
		AppendOnlyLocked: p.AppendOnlyLocked,
		MaxStorageBytes:  p.MaxStorageBytes,
		CreatedAt:        p.CreatedAt.Unix(),
		EffectiveAt:      p.EffectiveAt.Unix(),
	}

	if !p.ExpiresAt.IsZero() {
		signData.ExpiresAt = p.ExpiresAt.Unix()
	}

	jsonBytes, err := json.Marshal(signData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal policy: %w", err)
	}

	hash := sha256.Sum256(jsonBytes)
	return hash[:], nil
}

// SignAsOwner signs the policy as the owner
func (p *Policy) SignAsOwner(privateKey []byte) error {
	hash, err := p.Hash()
	if err != nil {
		return err
	}

	sig, err := crypto.Sign(privateKey, hash)
	if err != nil {
		return fmt.Errorf("failed to sign policy: %w", err)
	}

	p.OwnerSignature = hex.EncodeToString(sig)
	return nil
}

// SignAsHost signs the policy as the host
func (p *Policy) SignAsHost(privateKey []byte) error {
	hash, err := p.Hash()
	if err != nil {
		return err
	}

	sig, err := crypto.Sign(privateKey, hash)
	if err != nil {
		return fmt.Errorf("failed to sign policy: %w", err)
	}

	p.HostSignature = hex.EncodeToString(sig)
	return nil
}

// VerifyOwnerSignature verifies the owner's signature
func (p *Policy) VerifyOwnerSignature() error {
	if p.OwnerSignature == "" {
		return errors.New("no owner signature")
	}

	pubKey, err := hex.DecodeString(p.OwnerPubKey)
	if err != nil {
		return fmt.Errorf("invalid owner public key: %w", err)
	}

	sig, err := hex.DecodeString(p.OwnerSignature)
	if err != nil {
		return fmt.Errorf("invalid owner signature encoding: %w", err)
	}

	hash, err := p.Hash()
	if err != nil {
		return err
	}

	if !crypto.Verify(pubKey, hash, sig) {
		return errors.New("owner signature verification failed")
	}

	return nil
}

// VerifyHostSignature verifies the host's signature
func (p *Policy) VerifyHostSignature() error {
	if p.HostSignature == "" {
		return errors.New("no host signature")
	}

	pubKey, err := hex.DecodeString(p.HostPubKey)
	if err != nil {
		return fmt.Errorf("invalid host public key: %w", err)
	}

	sig, err := hex.DecodeString(p.HostSignature)
	if err != nil {
		return fmt.Errorf("invalid host signature encoding: %w", err)
	}

	hash, err := p.Hash()
	if err != nil {
		return err
	}

	if !crypto.Verify(pubKey, hash, sig) {
		return errors.New("host signature verification failed")
	}

	return nil
}

// IsFullySigned returns true if both parties have signed
func (p *Policy) IsFullySigned() bool {
	return p.OwnerSignature != "" && p.HostSignature != ""
}

// Verify checks both signatures
func (p *Policy) Verify() error {
	if err := p.VerifyOwnerSignature(); err != nil {
		return fmt.Errorf("owner signature: %w", err)
	}
	if err := p.VerifyHostSignature(); err != nil {
		return fmt.Errorf("host signature: %w", err)
	}
	return nil
}

// IsActive returns true if the policy is currently active
func (p *Policy) IsActive() bool {
	now := time.Now()

	// Not yet effective
	if now.Before(p.EffectiveAt) {
		return false
	}

	// Expired
	if !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt) {
		return false
	}

	return true
}

// CanDelete checks if deletion is allowed based on policy and file age
func (p *Policy) CanDelete(fileCreatedAt time.Time) (bool, string) {
	if p.DeletionMode == DeletionNever {
		return false, "policy prohibits deletion"
	}

	// Check retention period
	age := time.Since(fileCreatedAt)
	minRetention := time.Duration(p.RetentionDays) * 24 * time.Hour

	if age < minRetention {
		remaining := minRetention - age
		return false, fmt.Sprintf("retention period not met: %d days remaining", int(remaining.Hours()/24)+1)
	}

	// Retention period passed
	switch p.DeletionMode {
	case DeletionTimeLockOnly:
		return true, "retention period passed"
	case DeletionOwnerOnly:
		return false, "requires owner approval"
	case DeletionBothRequired:
		return false, "requires both owner and host approval"
	default:
		return false, "unknown deletion mode"
	}
}

// ToJSON serializes the policy to JSON
func (p *Policy) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// FromJSON deserializes a policy from JSON
func FromJSON(data []byte) (*Policy, error) {
	var p Policy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse policy: %w", err)
	}
	return &p, nil
}
