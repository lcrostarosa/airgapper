// Package crypto provides cryptographic utilities for Airgapper
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// GenerateKeyPair generates a new Ed25519 key pair
func GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return pub, priv, nil
}

// Sign signs a message with an Ed25519 private key
func Sign(privateKey, message []byte) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key size")
	}
	return ed25519.Sign(privateKey, message), nil
}

// Verify verifies a signature against a public key and message
func Verify(publicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	if len(signature) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(publicKey, message, signature)
}

// KeyID generates a deterministic identifier from a public key
// Returns the first 16 hex characters of SHA256(publicKey)
func KeyID(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	return hex.EncodeToString(hash[:8])
}

// RestoreRequestSignData holds the data that gets signed for restore request approval
type RestoreRequestSignData struct {
	RequestID   string   `json:"request_id"`
	Requester   string   `json:"requester"`
	SnapshotID  string   `json:"snapshot_id"`
	Paths       []string `json:"paths,omitempty"`
	Reason      string   `json:"reason"`
	CreatedAt   int64    `json:"created_at"` // Unix timestamp
	KeyHolderID string   `json:"key_holder_id"`
}

// HashRestoreRequest creates a canonical hash of a restore request for signing
func HashRestoreRequest(requestID, requester, snapshotID, reason, keyHolderID string, paths []string, createdAt int64) ([]byte, error) {
	// Sort paths for canonical ordering
	sortedPaths := make([]string, len(paths))
	copy(sortedPaths, paths)
	sort.Strings(sortedPaths)

	data := RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  snapshotID,
		Paths:       sortedPaths,
		Reason:      reason,
		CreatedAt:   createdAt,
		KeyHolderID: keyHolderID,
	}

	// Create canonical JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}

	// Hash the JSON
	hash := sha256.Sum256(jsonBytes)
	return hash[:], nil
}

// SignRestoreRequest signs a restore request approval
func SignRestoreRequest(privateKey []byte, requestID, requester, snapshotID, reason, keyHolderID string, paths []string, createdAt int64) ([]byte, error) {
	hash, err := HashRestoreRequest(requestID, requester, snapshotID, reason, keyHolderID, paths, createdAt)
	if err != nil {
		return nil, err
	}
	return Sign(privateKey, hash)
}

// VerifyRestoreRequestSignature verifies a signature on a restore request
func VerifyRestoreRequestSignature(publicKey, signature []byte, requestID, requester, snapshotID, reason, keyHolderID string, paths []string, createdAt int64) (bool, error) {
	hash, err := HashRestoreRequest(requestID, requester, snapshotID, reason, keyHolderID, paths, createdAt)
	if err != nil {
		return false, err
	}
	return Verify(publicKey, hash, signature), nil
}

// EncodePublicKey encodes a public key as hex
func EncodePublicKey(publicKey []byte) string {
	return hex.EncodeToString(publicKey)
}

// DecodePublicKey decodes a hex-encoded public key
func DecodePublicKey(encoded string) ([]byte, error) {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid hex encoding: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(decoded))
	}
	return decoded, nil
}

// EncodePrivateKey encodes a private key as hex
func EncodePrivateKey(privateKey []byte) string {
	return hex.EncodeToString(privateKey)
}

// DecodePrivateKey decodes a hex-encoded private key
func DecodePrivateKey(encoded string) ([]byte, error) {
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid hex encoding: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(decoded))
	}
	return decoded, nil
}
