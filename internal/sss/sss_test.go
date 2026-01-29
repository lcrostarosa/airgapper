package sss

import (
	"bytes"
	"testing"
)

func TestSplitAndCombine(t *testing.T) {
	secret := []byte("my-super-secret-restic-password!")

	// Split into 2 shares, requiring 2 to reconstruct
	shares, err := Split(secret, 2, 2)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(shares) != 2 {
		t.Fatalf("Expected 2 shares, got %d", len(shares))
	}

	// Combine both shares
	reconstructed, err := Combine(shares)
	if err != nil {
		t.Fatalf("Combine failed: %v", err)
	}

	if !bytes.Equal(secret, reconstructed) {
		t.Fatalf("Reconstructed secret doesn't match: got %q, want %q", reconstructed, secret)
	}
}

func TestSplitAndCombine3of5(t *testing.T) {
	secret := []byte("another-secret-key-for-testing")

	// Split into 5 shares, requiring 3 to reconstruct
	shares, err := Split(secret, 3, 5)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(shares) != 5 {
		t.Fatalf("Expected 5 shares, got %d", len(shares))
	}

	// Combine any 3 shares (use shares 0, 2, 4)
	subset := []Share{shares[0], shares[2], shares[4]}
	reconstructed, err := Combine(subset)
	if err != nil {
		t.Fatalf("Combine failed: %v", err)
	}

	if !bytes.Equal(secret, reconstructed) {
		t.Fatalf("Reconstructed secret doesn't match: got %q, want %q", reconstructed, secret)
	}
}

func TestCombineInsufficientShares(t *testing.T) {
	secret := []byte("secret")

	shares, err := Split(secret, 2, 2)
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// Try to combine with only 1 share
	_, err = Combine(shares[:1])
	if err == nil {
		t.Fatal("Expected error when combining with insufficient shares")
	}
}

func TestDifferentSecretLengths(t *testing.T) {
	testCases := []string{
		"a",
		"short",
		"medium-length-secret",
		"this-is-a-much-longer-secret-that-tests-the-implementation-with-more-bytes",
	}

	for _, secret := range testCases {
		shares, err := Split([]byte(secret), 2, 2)
		if err != nil {
			t.Fatalf("Split failed for %q: %v", secret, err)
		}

		reconstructed, err := Combine(shares)
		if err != nil {
			t.Fatalf("Combine failed for %q: %v", secret, err)
		}

		if string(reconstructed) != secret {
			t.Fatalf("Mismatch for %q: got %q", secret, reconstructed)
		}
	}
}
