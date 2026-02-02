package testutil

import (
	"crypto/sha256"
	"fmt"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// CryptoKeyFixture represents a test Ed25519 key pair
type CryptoKeyFixture struct {
	// PublicKey is the raw public key bytes
	PublicKey []byte
	// PrivateKey is the raw private key bytes
	PrivateKey []byte
	// KeyID is the fingerprint/ID derived from the public key
	KeyID string
	// PubHex is the hex-encoded public key
	PubHex string
	// PrivHex is the hex-encoded private key
	PrivHex string
	// Name is an optional friendly name for this key holder
	Name string
}

// NewCryptoKeyFixture generates a new Ed25519 key pair fixture
func NewCryptoKeyFixture(name string) (*CryptoKeyFixture, error) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	return &CryptoKeyFixture{
		PublicKey:  pub,
		PrivateKey: priv,
		KeyID:      crypto.KeyID(pub),
		PubHex:     crypto.EncodePublicKey(pub),
		PrivHex:    crypto.EncodePrivateKey(priv),
		Name:       name,
	}, nil
}

// MustNewCryptoKeyFixture generates a key fixture or panics
func MustNewCryptoKeyFixture(name string) *CryptoKeyFixture {
	f, err := NewCryptoKeyFixture(name)
	if err != nil {
		panic(fmt.Sprintf("failed to create crypto key fixture: %v", err))
	}
	return f
}

// Sign signs data with this fixture's private key
func (k *CryptoKeyFixture) Sign(data []byte) ([]byte, error) {
	return crypto.Sign(k.PrivateKey, data)
}

// Verify verifies a signature against this fixture's public key
func (k *CryptoKeyFixture) Verify(data, sig []byte) bool {
	return crypto.Verify(k.PublicKey, data, sig)
}

// SignRestoreRequest signs a restore request with this key
func (k *CryptoKeyFixture) SignRestoreRequest(requestID, requester, snapshotID, reason string, paths []string, createdAt int64) ([]byte, error) {
	return crypto.SignRestoreRequest(
		k.PrivateKey, requestID, requester, snapshotID, reason, k.KeyID, paths, createdAt,
	)
}

// VerifyRestoreRequest verifies a restore request signature
func (k *CryptoKeyFixture) VerifyRestoreRequest(sig []byte, requestID, requester, snapshotID, reason, keyID string, paths []string, createdAt int64) (bool, error) {
	return crypto.VerifyRestoreRequestSignature(
		k.PublicKey, sig, requestID, requester, snapshotID, reason, keyID, paths, createdAt,
	)
}

// Hash returns SHA256 hash of the public key
func (k *CryptoKeyFixture) Hash() [32]byte {
	return sha256.Sum256(k.PublicKey)
}

// EncodeDecodeRoundTrip tests that keys survive encode/decode
func (k *CryptoKeyFixture) EncodeDecodeRoundTrip() error {
	// Decode public key
	decodedPub, err := crypto.DecodePublicKey(k.PubHex)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %w", err)
	}

	// Decode private key
	decodedPriv, err := crypto.DecodePrivateKey(k.PrivHex)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	// Verify key IDs match
	decodedKeyID := crypto.KeyID(decodedPub)
	if decodedKeyID != k.KeyID {
		return fmt.Errorf("key ID mismatch: %s != %s", decodedKeyID, k.KeyID)
	}

	// Verify sign/verify works with decoded keys
	testMsg := []byte("test message for round-trip verification")
	sig, err := crypto.Sign(decodedPriv, testMsg)
	if err != nil {
		return fmt.Errorf("signing with decoded key failed: %w", err)
	}

	if !crypto.Verify(decodedPub, testMsg, sig) {
		return fmt.Errorf("verification with decoded keys failed")
	}

	return nil
}

// KeyHoldersFixture represents multiple key holders for consensus testing
type KeyHoldersFixture struct {
	// Holders is the list of key fixtures
	Holders []*CryptoKeyFixture
	// ByName allows lookup by name
	ByName map[string]*CryptoKeyFixture
	// Names is the ordered list of holder names
	Names []string
}

// NewKeyHoldersFixture creates fixtures for multiple key holders
func NewKeyHoldersFixture(names ...string) (*KeyHoldersFixture, error) {
	kf := &KeyHoldersFixture{
		Holders: make([]*CryptoKeyFixture, len(names)),
		ByName:  make(map[string]*CryptoKeyFixture),
		Names:   names,
	}

	for i, name := range names {
		key, err := NewCryptoKeyFixture(name)
		if err != nil {
			return nil, fmt.Errorf("failed to create key for %s: %w", name, err)
		}
		kf.Holders[i] = key
		kf.ByName[name] = key
	}

	return kf, nil
}

// MustNewKeyHoldersFixture creates key holders or panics
func MustNewKeyHoldersFixture(names ...string) *KeyHoldersFixture {
	kf, err := NewKeyHoldersFixture(names...)
	if err != nil {
		panic(fmt.Sprintf("failed to create key holders fixture: %v", err))
	}
	return kf
}

// Get returns the key fixture for a holder by name
func (kf *KeyHoldersFixture) Get(name string) *CryptoKeyFixture {
	return kf.ByName[name]
}

// GetByIndex returns the key fixture at the given index
func (kf *KeyHoldersFixture) GetByIndex(index int) *CryptoKeyFixture {
	if index < 0 || index >= len(kf.Holders) {
		return nil
	}
	return kf.Holders[index]
}

// PublicKeys returns all public keys
func (kf *KeyHoldersFixture) PublicKeys() [][]byte {
	keys := make([][]byte, len(kf.Holders))
	for i, h := range kf.Holders {
		keys[i] = h.PublicKey
	}
	return keys
}

// KeyIDs returns all key IDs
func (kf *KeyHoldersFixture) KeyIDs() []string {
	ids := make([]string, len(kf.Holders))
	for i, h := range kf.Holders {
		ids[i] = h.KeyID
	}
	return ids
}

// RestoreRequestFixture holds parameters for testing restore request signing
type RestoreRequestFixture struct {
	RequestID  string
	Requester  string
	SnapshotID string
	Reason     string
	Paths      []string
	CreatedAt  int64
}

// NewRestoreRequestFixture creates a restore request fixture with defaults
func NewRestoreRequestFixture() *RestoreRequestFixture {
	return &RestoreRequestFixture{
		RequestID:  "test-request-" + HashHex(generateRandomBytes(nil, 4))[:8],
		Requester:  "alice",
		SnapshotID: "latest",
		Reason:     "Need to restore files after system failure",
		Paths:      []string{"/home/user/Documents", "/home/user/Pictures"},
		CreatedAt:  1706745600, // Fixed timestamp for reproducibility
	}
}

// Sign signs this request with the given key fixture
func (r *RestoreRequestFixture) Sign(key *CryptoKeyFixture) ([]byte, error) {
	return key.SignRestoreRequest(r.RequestID, r.Requester, r.SnapshotID, r.Reason, r.Paths, r.CreatedAt)
}

// Verify verifies a signature against this request using the given key
func (r *RestoreRequestFixture) Verify(key *CryptoKeyFixture, sig []byte) (bool, error) {
	return key.VerifyRestoreRequest(sig, r.RequestID, r.Requester, r.SnapshotID, r.Reason, key.KeyID, r.Paths, r.CreatedAt)
}

// WithTamperedReason returns a copy with modified reason
func (r *RestoreRequestFixture) WithTamperedReason(reason string) *RestoreRequestFixture {
	return &RestoreRequestFixture{
		RequestID:  r.RequestID,
		Requester:  r.Requester,
		SnapshotID: r.SnapshotID,
		Reason:     reason,
		Paths:      r.Paths,
		CreatedAt:  r.CreatedAt,
	}
}

// WithTamperedPaths returns a copy with modified paths
func (r *RestoreRequestFixture) WithTamperedPaths(paths []string) *RestoreRequestFixture {
	return &RestoreRequestFixture{
		RequestID:  r.RequestID,
		Requester:  r.Requester,
		SnapshotID: r.SnapshotID,
		Reason:     r.Reason,
		Paths:      paths,
		CreatedAt:  r.CreatedAt,
	}
}

// WithTamperedSnapshot returns a copy with modified snapshot ID
func (r *RestoreRequestFixture) WithTamperedSnapshot(snapshotID string) *RestoreRequestFixture {
	return &RestoreRequestFixture{
		RequestID:  r.RequestID,
		Requester:  r.Requester,
		SnapshotID: snapshotID,
		Reason:     r.Reason,
		Paths:      r.Paths,
		CreatedAt:  r.CreatedAt,
	}
}
