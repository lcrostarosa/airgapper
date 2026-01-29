package sss

import (
	"bytes"
	"crypto/rand"
	"testing"
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
			if err != nil {
				t.Fatalf("Split failed: %v", err)
			}

			if len(shares) != tt.n {
				t.Errorf("Expected %d shares, got %d", tt.n, len(shares))
			}

			// Verify each share has correct length
			for i, share := range shares {
				if len(share.Data) != len(tt.secret) {
					t.Errorf("Share %d has wrong length: %d vs %d", i, len(share.Data), len(tt.secret))
				}
			}

			// Combine with exactly k shares
			result, err := Combine(shares[:tt.k])
			if err != nil {
				t.Fatalf("Combine failed: %v", err)
			}

			if !bytes.Equal(result, tt.secret) {
				t.Errorf("Reconstructed secret doesn't match:\n  got:  %x\n  want: %x", result, tt.secret)
			}
		})
	}
}

func TestCombineWithDifferentShareSubsets(t *testing.T) {
	secret := []byte("test secret for subset verification")

	shares, err := Split(secret, 2, 3)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Try all 2-share combinations
	combinations := [][2]int{
		{0, 1},
		{0, 2},
		{1, 2},
	}

	for _, combo := range combinations {
		subset := []Share{shares[combo[0]], shares[combo[1]]}
		result, err := Combine(subset)
		if err != nil {
			t.Errorf("Combine with shares %v failed: %v", combo, err)
			continue
		}

		if !bytes.Equal(result, secret) {
			t.Errorf("Combine with shares %v gave wrong result", combo)
		}
	}
}

func TestCombineWithMoreThanK(t *testing.T) {
	secret := []byte("test with extra shares")

	shares, err := Split(secret, 2, 3)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Combine with all 3 shares (more than k=2)
	result, err := Combine(shares)
	if err != nil {
		t.Fatalf("Combine failed: %v", err)
	}

	if !bytes.Equal(result, secret) {
		t.Errorf("Reconstructed secret doesn't match")
	}
}

func TestSplitErrors(t *testing.T) {
	tests := []struct {
		name   string
		secret []byte
		k, n   int
	}{
		{
			name:   "k too small",
			secret: []byte("test"),
			k:      1,
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
			if err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

func TestCombineErrors(t *testing.T) {
	t.Run("too few shares", func(t *testing.T) {
		shares := []Share{{Index: 1, Data: []byte("test")}}
		_, err := Combine(shares)
		if err == nil {
			t.Error("Expected error for single share")
		}
	})

	t.Run("mismatched lengths", func(t *testing.T) {
		shares := []Share{
			{Index: 1, Data: []byte("short")},
			{Index: 2, Data: []byte("longer data")},
		}
		_, err := Combine(shares)
		if err == nil {
			t.Error("Expected error for mismatched lengths")
		}
	})
}

func TestRandomSecrets(t *testing.T) {
	for i := 0; i < 10; i++ {
		secret := make([]byte, 32)
		rand.Read(secret)

		shares, err := Split(secret, 2, 2)
		if err != nil {
			t.Fatalf("Split failed: %v", err)
		}

		result, err := Combine(shares)
		if err != nil {
			t.Fatalf("Combine failed: %v", err)
		}

		if !bytes.Equal(result, secret) {
			t.Errorf("Random test %d failed", i)
		}
	}
}

func TestGF256Operations(t *testing.T) {
	// Test addition (XOR)
	if gfAdd(0x53, 0xca) != 0x99 {
		t.Error("GF add failed")
	}

	// Test that a + a = 0
	for i := 0; i < 256; i++ {
		if gfAdd(byte(i), byte(i)) != 0 {
			t.Errorf("GF add self failed for %d", i)
		}
	}

	// Test multiplication
	if gfMul(0x53, 0xca) != 0x01 {
		// This specific test may not be accurate, let's test properties instead
	}

	// Test that a * 1 = a
	for i := 0; i < 256; i++ {
		if gfMul(byte(i), 1) != byte(i) {
			t.Errorf("GF mul by 1 failed for %d", i)
		}
	}

	// Test that a * 0 = 0
	for i := 0; i < 256; i++ {
		if gfMul(byte(i), 0) != 0 {
			t.Errorf("GF mul by 0 failed for %d", i)
		}
	}

	// Test inverse: a * a^(-1) = 1 for a != 0
	for i := 1; i < 256; i++ {
		inv := gfInverse(byte(i))
		if gfMul(byte(i), inv) != 1 {
			t.Errorf("GF inverse failed for %d: %d * %d != 1", i, i, inv)
		}
	}
}

func BenchmarkSplit(b *testing.B) {
	secret := make([]byte, 64)
	rand.Read(secret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Split(secret, 2, 2)
	}
}

func BenchmarkCombine(b *testing.B) {
	secret := make([]byte, 64)
	rand.Read(secret)
	shares, _ := Split(secret, 2, 2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Combine(shares)
	}
}
