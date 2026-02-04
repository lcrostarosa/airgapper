// Package crypto provides cryptographic utilities for Airgapper
package crypto

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKeyPair(t *testing.T) {
	t.Run("generates valid key pair", func(t *testing.T) {
		pub, priv, err := GenerateKeyPair()
		require.NoError(t, err)
		assert.Len(t, pub, ed25519.PublicKeySize)
		assert.Len(t, priv, ed25519.PrivateKeySize)
	})

	t.Run("generates unique key pairs", func(t *testing.T) {
		pub1, priv1, err1 := GenerateKeyPair()
		pub2, priv2, err2 := GenerateKeyPair()
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.False(t, bytes.Equal(pub1, pub2), "public keys should be different")
		assert.False(t, bytes.Equal(priv1, priv2), "private keys should be different")
	})

	t.Run("generated keys work with sign/verify", func(t *testing.T) {
		pub, priv, err := GenerateKeyPair()
		require.NoError(t, err)

		message := []byte("test message")
		sig, err := Sign(priv, message)
		require.NoError(t, err)
		assert.True(t, Verify(pub, message, sig))
	})
}

func TestSign(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	require.NoError(t, err)

	t.Run("signs message successfully", func(t *testing.T) {
		message := []byte("test message")
		sig, err := Sign(priv, message)
		require.NoError(t, err)
		assert.Len(t, sig, ed25519.SignatureSize)
		assert.True(t, Verify(pub, message, sig))
	})

	t.Run("returns error for invalid private key size", func(t *testing.T) {
		message := []byte("test message")
		invalidKey := []byte("too short")
		sig, err := Sign(invalidKey, message)
		assert.Error(t, err)
		assert.Nil(t, sig)
		assert.Contains(t, err.Error(), "invalid private key size")
	})

	t.Run("signs empty message", func(t *testing.T) {
		message := []byte{}
		sig, err := Sign(priv, message)
		require.NoError(t, err)
		assert.True(t, Verify(pub, message, sig))
	})

	t.Run("different messages produce different signatures", func(t *testing.T) {
		msg1 := []byte("message 1")
		msg2 := []byte("message 2")
		sig1, err1 := Sign(priv, msg1)
		sig2, err2 := Sign(priv, msg2)
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.False(t, bytes.Equal(sig1, sig2))
	})
}

func TestVerify(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	require.NoError(t, err)
	message := []byte("test message")
	sig, err := Sign(priv, message)
	require.NoError(t, err)

	t.Run("verifies valid signature", func(t *testing.T) {
		assert.True(t, Verify(pub, message, sig))
	})

	t.Run("rejects invalid public key size", func(t *testing.T) {
		invalidPub := []byte("too short")
		assert.False(t, Verify(invalidPub, message, sig))
	})

	t.Run("rejects invalid signature size", func(t *testing.T) {
		invalidSig := []byte("too short")
		assert.False(t, Verify(pub, message, invalidSig))
	})

	t.Run("rejects tampered message", func(t *testing.T) {
		tamperedMessage := []byte("tampered message")
		assert.False(t, Verify(pub, tamperedMessage, sig))
	})

	t.Run("rejects tampered signature", func(t *testing.T) {
		tamperedSig := make([]byte, len(sig))
		copy(tamperedSig, sig)
		tamperedSig[0] ^= 0xFF // flip bits
		assert.False(t, Verify(pub, message, tamperedSig))
	})

	t.Run("rejects wrong public key", func(t *testing.T) {
		pub2, _, err := GenerateKeyPair()
		require.NoError(t, err)
		assert.False(t, Verify(pub2, message, sig))
	})
}

func TestKeyID(t *testing.T) {
	t.Run("generates deterministic ID", func(t *testing.T) {
		pub, _, err := GenerateKeyPair()
		require.NoError(t, err)
		id1 := KeyID(pub)
		id2 := KeyID(pub)
		assert.Equal(t, id1, id2)
	})

	t.Run("returns 16 hex characters", func(t *testing.T) {
		pub, _, err := GenerateKeyPair()
		require.NoError(t, err)
		id := KeyID(pub)
		assert.Len(t, id, 16)
		// Verify it's valid hex
		for _, c := range id {
			assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
				"character %c should be hex", c)
		}
	})

	t.Run("different keys produce different IDs", func(t *testing.T) {
		pub1, _, _ := GenerateKeyPair()
		pub2, _, _ := GenerateKeyPair()
		id1 := KeyID(pub1)
		id2 := KeyID(pub2)
		assert.NotEqual(t, id1, id2)
	})

	t.Run("works with arbitrary byte slice", func(t *testing.T) {
		// KeyID doesn't validate input, it just hashes
		id := KeyID([]byte("any data"))
		assert.Len(t, id, 16)
	})
}

func TestRestoreRequestSignData_Hash(t *testing.T) {
	t.Run("produces deterministic hash", func(t *testing.T) {
		data := RestoreRequestSignData{
			RequestID:   "req-123",
			Requester:   "user1",
			SnapshotID:  "snap-456",
			Paths:       []string{"/path/a", "/path/b"},
			Reason:      "restore for testing",
			CreatedAt:   1234567890,
			KeyHolderID: "holder-789",
		}
		hash1, err1 := data.Hash()
		hash2, err2 := data.Hash()
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("path order does not affect hash (canonical ordering)", func(t *testing.T) {
		data1 := RestoreRequestSignData{
			RequestID:   "req-123",
			Requester:   "user1",
			SnapshotID:  "snap-456",
			Paths:       []string{"/path/b", "/path/a", "/path/c"},
			Reason:      "restore",
			CreatedAt:   1234567890,
			KeyHolderID: "holder-789",
		}
		data2 := RestoreRequestSignData{
			RequestID:   "req-123",
			Requester:   "user1",
			SnapshotID:  "snap-456",
			Paths:       []string{"/path/a", "/path/c", "/path/b"},
			Reason:      "restore",
			CreatedAt:   1234567890,
			KeyHolderID: "holder-789",
		}
		hash1, err1 := data1.Hash()
		hash2, err2 := data2.Hash()
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, hash1, hash2, "paths should be sorted before hashing")
	})

	t.Run("different data produces different hashes", func(t *testing.T) {
		data1 := RestoreRequestSignData{
			RequestID: "req-123",
			Requester: "user1",
		}
		data2 := RestoreRequestSignData{
			RequestID: "req-456",
			Requester: "user1",
		}
		hash1, _ := data1.Hash()
		hash2, _ := data2.Hash()
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("hash is 32 bytes (SHA256)", func(t *testing.T) {
		data := RestoreRequestSignData{RequestID: "test"}
		hash, err := data.Hash()
		require.NoError(t, err)
		assert.Len(t, hash, 32)
	})

	t.Run("empty paths handled correctly", func(t *testing.T) {
		data := RestoreRequestSignData{
			RequestID: "req-123",
			Paths:     nil,
		}
		hash, err := data.Hash()
		require.NoError(t, err)
		assert.Len(t, hash, 32)
	})

	t.Run("does not modify original paths slice", func(t *testing.T) {
		original := []string{"/c", "/a", "/b"}
		data := RestoreRequestSignData{
			RequestID: "req-123",
			Paths:     original,
		}
		_, err := data.Hash()
		require.NoError(t, err)
		assert.Equal(t, []string{"/c", "/a", "/b"}, original, "original paths should not be modified")
	})
}

func TestRestoreRequestSignData_SignAndVerify(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	require.NoError(t, err)

	data := RestoreRequestSignData{
		RequestID:   "req-123",
		Requester:   "user1",
		SnapshotID:  "snap-456",
		Paths:       []string{"/path/a"},
		Reason:      "restore for testing",
		CreatedAt:   1234567890,
		KeyHolderID: "holder-789",
	}

	t.Run("sign and verify round-trip", func(t *testing.T) {
		sig, err := data.Sign(priv)
		require.NoError(t, err)
		assert.Len(t, sig, ed25519.SignatureSize)

		valid, err := data.Verify(pub, sig)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("sign returns error for invalid private key", func(t *testing.T) {
		sig, err := data.Sign([]byte("invalid"))
		assert.Error(t, err)
		assert.Nil(t, sig)
	})

	t.Run("verify rejects wrong public key", func(t *testing.T) {
		sig, err := data.Sign(priv)
		require.NoError(t, err)

		pub2, _, _ := GenerateKeyPair()
		valid, err := data.Verify(pub2, sig)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("verify rejects tampered data", func(t *testing.T) {
		sig, err := data.Sign(priv)
		require.NoError(t, err)

		tamperedData := data
		tamperedData.RequestID = "tampered"
		valid, err := tamperedData.Verify(pub, sig)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("verify with different path order still works", func(t *testing.T) {
		dataWithPaths := RestoreRequestSignData{
			RequestID: "req-123",
			Paths:     []string{"/b", "/a", "/c"},
		}
		sig, err := dataWithPaths.Sign(priv)
		require.NoError(t, err)

		// Verify with paths in different order
		dataReordered := RestoreRequestSignData{
			RequestID: "req-123",
			Paths:     []string{"/a", "/c", "/b"},
		}
		valid, err := dataReordered.Verify(pub, sig)
		require.NoError(t, err)
		assert.True(t, valid, "canonical ordering should make path order irrelevant")
	})
}

func TestEncodeDecodePublicKey(t *testing.T) {
	t.Run("round-trip encoding", func(t *testing.T) {
		pub, _, err := GenerateKeyPair()
		require.NoError(t, err)

		encoded := EncodePublicKey(pub)
		decoded, err := DecodePublicKey(encoded)
		require.NoError(t, err)
		assert.Equal(t, pub, decoded)
	})

	t.Run("encoded key is hex string", func(t *testing.T) {
		pub, _, err := GenerateKeyPair()
		require.NoError(t, err)

		encoded := EncodePublicKey(pub)
		assert.Len(t, encoded, ed25519.PublicKeySize*2) // hex doubles length
	})

	t.Run("decode rejects invalid hex", func(t *testing.T) {
		_, err := DecodePublicKey("not-valid-hex!")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid hex encoding")
	})

	t.Run("decode rejects wrong size", func(t *testing.T) {
		// Valid hex but wrong length
		_, err := DecodePublicKey("aabbccdd")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid public key size")
	})
}

func TestEncodeDecodePrivateKey(t *testing.T) {
	t.Run("round-trip encoding", func(t *testing.T) {
		_, priv, err := GenerateKeyPair()
		require.NoError(t, err)

		encoded := EncodePrivateKey(priv)
		decoded, err := DecodePrivateKey(encoded)
		require.NoError(t, err)
		assert.Equal(t, priv, decoded)
	})

	t.Run("encoded key is hex string", func(t *testing.T) {
		_, priv, err := GenerateKeyPair()
		require.NoError(t, err)

		encoded := EncodePrivateKey(priv)
		assert.Len(t, encoded, ed25519.PrivateKeySize*2) // hex doubles length
	})

	t.Run("decode rejects invalid hex", func(t *testing.T) {
		_, err := DecodePrivateKey("not-valid-hex!")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid hex encoding")
	})

	t.Run("decode rejects wrong size", func(t *testing.T) {
		// Valid hex but wrong length
		_, err := DecodePrivateKey("aabbccdd")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid private key size")
	})
}

func TestKeyPairIntegration(t *testing.T) {
	t.Run("full workflow: generate, encode, decode, sign, verify", func(t *testing.T) {
		// Generate
		pub, priv, err := GenerateKeyPair()
		require.NoError(t, err)

		// Encode and decode (simulating storage/transmission)
		pubEncoded := EncodePublicKey(pub)
		privEncoded := EncodePrivateKey(priv)

		pubDecoded, err := DecodePublicKey(pubEncoded)
		require.NoError(t, err)
		privDecoded, err := DecodePrivateKey(privEncoded)
		require.NoError(t, err)

		// Sign with decoded private key
		message := []byte("important message")
		sig, err := Sign(privDecoded, message)
		require.NoError(t, err)

		// Verify with decoded public key
		assert.True(t, Verify(pubDecoded, message, sig))

		// Also verify KeyID is consistent
		assert.Equal(t, KeyID(pub), KeyID(pubDecoded))
	})
}
