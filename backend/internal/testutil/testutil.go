// Package testutil provides shared test fixtures and utilities for Airgapper tests.
// It reduces duplication across test files by providing common patterns for:
// - Password/secret generation with deterministic seeding
// - SSS (Shamir's Secret Sharing) operations with builder pattern
// - Cryptographic key generation and signing
// - Repository setup and configuration
// - Multi-party workflow simulation
package testutil

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	mrand "math/rand"
	"os"
	"testing"
)

// FixtureOption configures fixture creation behavior
type FixtureOption func(*fixtureConfig)

type fixtureConfig struct {
	seed   int64
	seeded bool
}

// WithSeed provides a deterministic seed for reproducible tests.
// When a test fails, the seed is logged so the failure can be reproduced.
func WithSeed(seed int64) FixtureOption {
	return func(c *fixtureConfig) {
		c.seed = seed
		c.seeded = true
	}
}

// GetTestSeed returns a seed for deterministic testing.
// It checks AIRGAPPER_TEST_SEED env var first, otherwise generates a random seed.
// The seed is logged so failures can be reproduced.
func GetTestSeed(t *testing.T) int64 {
	t.Helper()

	if seedStr := os.Getenv("AIRGAPPER_TEST_SEED"); seedStr != "" {
		var seed int64
		if _, err := fmt.Sscanf(seedStr, "%d", &seed); err == nil {
			t.Logf("Using seed from AIRGAPPER_TEST_SEED: %d", seed)
			return seed
		}
	}

	// Generate random seed
	n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("Failed to generate random seed: %v", err)
	}
	seed := n.Int64()
	t.Logf("Generated test seed: %d (set AIRGAPPER_TEST_SEED=%d to reproduce)", seed, seed)
	return seed
}

// newRand creates a new random source, using seed if provided, otherwise crypto/rand
func newRand(opts ...FixtureOption) *mrand.Rand {
	cfg := &fixtureConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.seeded {
		return mrand.New(mrand.NewSource(cfg.seed))
	}
	// Use crypto/rand to seed
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return mrand.New(mrand.NewSource(n.Int64()))
}

// generateRandomBytes generates n random bytes using the provided source
func generateRandomBytes(r *mrand.Rand, n int) []byte {
	if r == nil {
		// Use crypto/rand for secure randomness
		b := make([]byte, n)
		rand.Read(b)
		return b
	}
	b := make([]byte, n)
	r.Read(b)
	return b
}

// HashData returns SHA256 hash of the data
func HashData(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// HashHex returns hex-encoded SHA256 hash of the data
func HashHex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// CompareHashes returns true if two hashes are identical
func CompareHashes(a, b [32]byte) bool {
	return a == b
}

// ValidateHash checks if data matches the expected hash
func ValidateHash(data []byte, expected [32]byte) bool {
	actual := sha256.Sum256(data)
	return actual == expected
}
