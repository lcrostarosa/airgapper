package testutil

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"github.com/lcrostarosa/airgapper/backend/internal/sss"
)

// SSSFixture provides a complete SSS split/combine test setup
type SSSFixture struct {
	// Secret is the original secret data
	Secret []byte
	// SecretHash is SHA256 of the secret
	SecretHash [32]byte
	// Shares is the result of splitting the secret
	Shares []sss.Share
	// Threshold is the minimum shares needed (k)
	Threshold int
	// TotalShares is the total number of shares (n)
	TotalShares int
}

// SSSFixtureBuilder constructs SSSFixture with a fluent API
type SSSFixtureBuilder struct {
	secret    []byte
	threshold int
	total     int
	opts      []FixtureOption
	err       error
}

// NewSSSFixture starts building an SSS fixture
func NewSSSFixture() *SSSFixtureBuilder {
	return &SSSFixtureBuilder{
		threshold: 2,
		total:     2,
	}
}

// WithSecret sets a specific secret for the fixture
func (b *SSSFixtureBuilder) WithSecret(secret []byte) *SSSFixtureBuilder {
	b.secret = secret
	return b
}

// WithRandomSecret generates a random secret of the specified byte size
func (b *SSSFixtureBuilder) WithRandomSecret(size int) *SSSFixtureBuilder {
	r := newRand(b.opts...)
	b.secret = generateRandomBytes(r, size)
	return b
}

// WithHexPassword generates a random password and uses its hex encoding as secret
func (b *SSSFixtureBuilder) WithHexPassword(byteSize int) *SSSFixtureBuilder {
	pf := NewPasswordFixture(b.opts...)
	if byteSize != 32 {
		r := newRand(b.opts...)
		pf = NewPasswordFixtureWithSize(byteSize, r)
	}
	b.secret = pf.Bytes()
	return b
}

// WithThreshold sets the k-of-n threshold scheme
func (b *SSSFixtureBuilder) WithThreshold(k, n int) *SSSFixtureBuilder {
	if k < 2 {
		b.err = fmt.Errorf("threshold k must be at least 2, got %d", k)
		return b
	}
	if n < k {
		b.err = fmt.Errorf("total n must be >= threshold k, got k=%d, n=%d", k, n)
		return b
	}
	b.threshold = k
	b.total = n
	return b
}

// WithSeed sets deterministic seeding for reproducible tests
func (b *SSSFixtureBuilder) WithSeed(seed int64) *SSSFixtureBuilder {
	b.opts = append(b.opts, WithSeed(seed))
	return b
}

// Build creates the SSSFixture, performing the split operation
func (b *SSSFixtureBuilder) Build() (*SSSFixture, error) {
	if b.err != nil {
		return nil, b.err
	}

	if b.secret == nil {
		// Default: random 32-byte hex password
		pf := NewPasswordFixture(b.opts...)
		b.secret = pf.Bytes()
	}

	if len(b.secret) == 0 {
		return nil, fmt.Errorf("secret cannot be empty")
	}

	shares, err := sss.Split(b.secret, b.threshold, b.total)
	if err != nil {
		return nil, fmt.Errorf("SSS split failed: %w", err)
	}

	return &SSSFixture{
		Secret:      b.secret,
		SecretHash:  sha256.Sum256(b.secret),
		Shares:      shares,
		Threshold:   b.threshold,
		TotalShares: b.total,
	}, nil
}

// MustBuild creates the fixture or panics (for use in test setup)
func (b *SSSFixtureBuilder) MustBuild() *SSSFixture {
	f, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("SSSFixture build failed: %v", err))
	}
	return f
}

// Combine reconstructs the secret using the specified share indices
func (f *SSSFixture) Combine(indices ...int) ([]byte, error) {
	if len(indices) < f.Threshold {
		return nil, fmt.Errorf("need at least %d shares, got %d", f.Threshold, len(indices))
	}

	subset := make([]sss.Share, len(indices))
	for i, idx := range indices {
		if idx < 0 || idx >= len(f.Shares) {
			return nil, fmt.Errorf("invalid share index %d (have %d shares)", idx, len(f.Shares))
		}
		subset[i] = f.Shares[idx]
	}

	return sss.Combine(subset)
}

// ValidateReconstruction combines shares and verifies the hash matches
func (f *SSSFixture) ValidateReconstruction(indices ...int) error {
	reconstructed, err := f.Combine(indices...)
	if err != nil {
		return fmt.Errorf("combine failed: %w", err)
	}

	reconstructedHash := sha256.Sum256(reconstructed)
	if reconstructedHash != f.SecretHash {
		return fmt.Errorf("hash mismatch: expected %x, got %x", f.SecretHash[:8], reconstructedHash[:8])
	}

	if !bytes.Equal(f.Secret, reconstructed) {
		return fmt.Errorf("content mismatch")
	}

	return nil
}

// AllCombinations returns all valid k-combinations of share indices
func (f *SSSFixture) AllCombinations() [][]int {
	return combinations(f.TotalShares, f.Threshold)
}

// combinations generates all k-combinations from n items (0..n-1)
func combinations(n, k int) [][]int {
	var result [][]int
	indices := make([]int, k)
	for i := range indices {
		indices[i] = i
	}

	for {
		// Add current combination
		combo := make([]int, k)
		copy(combo, indices)
		result = append(result, combo)

		// Find rightmost element that can be incremented
		i := k - 1
		for i >= 0 && indices[i] == n-k+i {
			i--
		}
		if i < 0 {
			break
		}

		indices[i]++
		for j := i + 1; j < k; j++ {
			indices[j] = indices[j-1] + 1
		}
	}

	return result
}

// TamperedShare returns a copy of share at index with data tampered
func (f *SSSFixture) TamperedShare(index int) sss.Share {
	if index < 0 || index >= len(f.Shares) {
		panic(fmt.Sprintf("invalid share index %d", index))
	}

	original := f.Shares[index]
	tampered := sss.Share{
		Index: original.Index,
		Data:  make([]byte, len(original.Data)),
	}
	copy(tampered.Data, original.Data)

	// Flip bits in first byte
	if len(tampered.Data) > 0 {
		tampered.Data[0] ^= 0xFF
	}

	return tampered
}

// CombineWithTamperedShare attempts to reconstruct using one tampered share
func (f *SSSFixture) CombineWithTamperedShare(tamperedIndex int, otherIndices ...int) ([]byte, error) {
	tampered := f.TamperedShare(tamperedIndex)

	subset := []sss.Share{tampered}
	for _, idx := range otherIndices {
		if idx == tamperedIndex {
			continue // Skip the tampered one, we already added it
		}
		if idx < 0 || idx >= len(f.Shares) {
			return nil, fmt.Errorf("invalid share index %d", idx)
		}
		subset = append(subset, f.Shares[idx])
	}

	if len(subset) < f.Threshold {
		return nil, fmt.Errorf("need at least %d shares, got %d", f.Threshold, len(subset))
	}

	return sss.Combine(subset)
}
