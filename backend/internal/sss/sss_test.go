package sss

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitAndCombine(t *testing.T) {
	tests := []struct {
		name   string
		secret []byte
		k, n   int
	}{
		{
			name:   "simple 2-of-2",
			secret: []byte("hello world"),
			k:      2,
			n:      2,
		},
		{
			name:   "hex password",
			secret: []byte("a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd"),
			k:      2,
			n:      2,
		},
		{
			name:   "2-of-3",
			secret: []byte("test secret"),
			k:      2,
			n:      3,
		},
		{
			name:   "3-of-5",
			secret: []byte("more complex sharing"),
			k:      3,
			n:      5,
		},
		{
			name:   "single byte",
			secret: []byte{0x42},
			k:      2,
			n:      2,
		},
		{
			name:   "all zeros",
			secret: make([]byte, 32),
			k:      2,
			n:      2,
		},
		{
			name:   "all ones",
			secret: bytes.Repeat([]byte{0xff}, 32),
			k:      2,
			n:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shares, err := Split(tt.secret, tt.k, tt.n)
			require.NoError(t, err, "Split failed")
			assert.Len(t, shares, tt.n)

			// Verify each share has correct length
			for i, share := range shares {
				assert.Len(t, share.Data, len(tt.secret), "Share %d has wrong length", i)
			}

			// Combine with exactly k shares
			result, err := Combine(shares[:tt.k])
			require.NoError(t, err, "Combine failed")
			assert.Equal(t, tt.secret, result, "Reconstructed secret doesn't match")
		})
	}
}

func TestCombineWithDifferentShareSubsets(t *testing.T) {
	secret := []byte("test secret for subset verification")

	shares, err := Split(secret, 2, 3)
	require.NoError(t, err, "Split failed")

	// Try all 2-share combinations
	combinations := [][2]int{
		{0, 1},
		{0, 2},
		{1, 2},
	}

	for _, combo := range combinations {
		subset := []Share{shares[combo[0]], shares[combo[1]]}
		result, err := Combine(subset)
		require.NoError(t, err, "Combine with shares %v failed", combo)
		assert.Equal(t, secret, result, "Combine with shares %v gave wrong result", combo)
	}
}

func TestCombineWithMoreThanK(t *testing.T) {
	secret := []byte("test with extra shares")

	shares, err := Split(secret, 2, 3)
	require.NoError(t, err, "Split failed")

	// Combine with all 3 shares (more than k=2)
	result, err := Combine(shares)
	require.NoError(t, err, "Combine failed")
	assert.Equal(t, secret, result, "Reconstructed secret doesn't match")
}

func TestSplitErrors(t *testing.T) {
	tests := []struct {
		name   string
		secret []byte
		k, n   int
	}{
		{
			name:   "k is zero",
			secret: []byte("test"),
			k:      0,
			n:      2,
		},
		{
			name:   "n less than k",
			secret: []byte("test"),
			k:      3,
			n:      2,
		},
		{
			name:   "n too large",
			secret: []byte("test"),
			k:      2,
			n:      256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Split(tt.secret, tt.k, tt.n)
			assert.Error(t, err, "Expected error, got nil")
		})
	}
}

func TestSplit1ofN(t *testing.T) {
	// Test that 1-of-n schemes work (for single-party mode)
	secret := []byte("test secret for 1-of-n")

	shares, err := Split(secret, 1, 3)
	require.NoError(t, err, "Split 1-of-3 failed")
	assert.Len(t, shares, 3)

	// Any single share should recover the secret
	for i, share := range shares {
		result, err := Combine([]Share{share})
		require.NoError(t, err, "Combine single share %d failed", i)
		assert.Equal(t, secret, result, "Single share %d gave wrong result", i)
	}
}

func TestCombineErrors(t *testing.T) {
	t.Run("no shares", func(t *testing.T) {
		shares := []Share{}
		_, err := Combine(shares)
		assert.Error(t, err, "Expected error for no shares")
	})

	t.Run("mismatched lengths", func(t *testing.T) {
		shares := []Share{
			{Index: 1, Data: []byte("short")},
			{Index: 2, Data: []byte("longer data")},
		}
		_, err := Combine(shares)
		assert.Error(t, err, "Expected error for mismatched lengths")
	})
}

func TestRandomSecrets(t *testing.T) {
	for i := 0; i < 10; i++ {
		secret := make([]byte, 32)
		_, err := rand.Read(secret)
		require.NoError(t, err, "failed to generate random secret")

		shares, err := Split(secret, 2, 2)
		require.NoError(t, err, "Split failed")

		result, err := Combine(shares)
		require.NoError(t, err, "Combine failed")
		assert.Equal(t, secret, result, "Random test %d failed", i)
	}
}

func TestGF256Operations(t *testing.T) {
	// Test addition (XOR)
	assert.Equal(t, byte(0x99), gfAdd(0x53, 0xca), "GF add failed")

	// Test that a + a = 0
	for i := 0; i < 256; i++ {
		assert.Equal(t, byte(0), gfAdd(byte(i), byte(i)), "GF add self failed for %d", i)
	}

	// Test that a * 1 = a
	for i := 0; i < 256; i++ {
		assert.Equal(t, byte(i), gfMul(byte(i), 1), "GF mul by 1 failed for %d", i)
	}

	// Test that a * 0 = 0
	for i := 0; i < 256; i++ {
		assert.Equal(t, byte(0), gfMul(byte(i), 0), "GF mul by 0 failed for %d", i)
	}

	// Test inverse: a * a^(-1) = 1 for a != 0
	for i := 1; i < 256; i++ {
		inv := gfInverse(byte(i))
		assert.Equal(t, byte(1), gfMul(byte(i), inv), "GF inverse failed for %d: %d * %d != 1", i, i, inv)
	}
}

func BenchmarkSplit(b *testing.B) {
	secret := make([]byte, 64)
	_, _ = rand.Read(secret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Split(secret, 2, 2)
	}
}

func BenchmarkCombine(b *testing.B) {
	secret := make([]byte, 64)
	_, _ = rand.Read(secret)
	shares, _ := Split(secret, 2, 2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Combine(shares)
	}
}
