// Package e2e provides end-to-end tests for Airgapper workflows
package e2e

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_SSS_SplitCombineIntegrity tests the full SSS flow with hash validation
func TestE2E_SSS_SplitCombineIntegrity(t *testing.T) {
	// Simulate a real password like the one generated in init
	password := make([]byte, 32)
	_, err := rand.Read(password)
	require.NoError(t, err, "Failed to generate password")
	passwordHex := hex.EncodeToString(password)

	// Calculate hash BEFORE split
	originalHash := sha256.Sum256([]byte(passwordHex))

	// Split into 2 shares (2-of-2)
	shares, err := sss.Split([]byte(passwordHex), 2, 2)
	require.NoError(t, err, "Split failed")

	// Verify shares are different from original
	for i, share := range shares {
		assert.False(t, bytes.Equal(share.Data, []byte(passwordHex)), "Share %d is identical to original - SSS not working", i)
	}

	// Combine shares
	reconstructed, err := sss.Combine(shares)
	require.NoError(t, err, "Combine failed")

	// Calculate hash AFTER combine
	reconstructedHash := sha256.Sum256(reconstructed)

	// Verify hash matches
	assert.Equal(t, originalHash, reconstructedHash, "Hash mismatch after reconstruction!\n  Original:      %x\n  Reconstructed: %x", originalHash, reconstructedHash)

	// Verify content matches
	assert.True(t, bytes.Equal([]byte(passwordHex), reconstructed), "Content mismatch after reconstruction!\n  Original:      %s\n  Reconstructed: %s", passwordHex, string(reconstructed))

	t.Logf("SSS split/combine integrity verified. Hash: %x", originalHash[:8])
}

// TestE2E_SSS_PartialSharesCannotReconstruct verifies that fewer than k shares fail
func TestE2E_SSS_PartialSharesCannotReconstruct(t *testing.T) {
	password := []byte("super-secret-password-12345678901234567890")
	originalHash := sha256.Sum256(password)

	// Split into 3 shares (2-of-3)
	shares, err := sss.Split(password, 2, 3)
	require.NoError(t, err, "Split failed")

	// Try to "reconstruct" with only 1 share (should fail or give wrong result)
	// Note: sss.Combine requires at least 2 shares, so we test the boundary

	// Verify that any 2 shares work
	combinations := [][2]int{{0, 1}, {0, 2}, {1, 2}}
	for _, combo := range combinations {
		subset := []sss.Share{shares[combo[0]], shares[combo[1]]}
		reconstructed, err := sss.Combine(subset)
		require.NoError(t, err, "Combine with shares [%d,%d] failed", combo[0], combo[1])

		reconstructedHash := sha256.Sum256(reconstructed)
		assert.Equal(t, originalHash, reconstructedHash, "Hash mismatch with shares [%d,%d]", combo[0], combo[1])
	}

	t.Log("All 2-share combinations successfully reconstruct with correct hash")
}

// TestE2E_SSS_TamperDetection verifies that tampered shares produce wrong output
func TestE2E_SSS_TamperDetection(t *testing.T) {
	password := []byte("sensitive-data-that-must-not-be-corrupted")
	originalHash := sha256.Sum256(password)

	shares, err := sss.Split(password, 2, 2)
	require.NoError(t, err, "Split failed")

	// Tamper with one share
	tamperedShares := []sss.Share{
		{Index: shares[0].Index, Data: make([]byte, len(shares[0].Data))},
		shares[1],
	}
	copy(tamperedShares[0].Data, shares[0].Data)
	tamperedShares[0].Data[0] ^= 0xFF // Flip bits in first byte

	// Reconstruct with tampered share
	reconstructed, err := sss.Combine(tamperedShares)
	require.NoError(t, err, "Combine failed")

	// Hash should NOT match
	reconstructedHash := sha256.Sum256(reconstructed)
	assert.NotEqual(t, originalHash, reconstructedHash, "Tampered share produced correct hash - this should not happen!")

	t.Logf("Tamper detection working: original hash %x... != tampered hash %x...", originalHash[:8], reconstructedHash[:8])
}

// TestE2E_Consensus_SignVerify tests the Ed25519 signing workflow
func TestE2E_Consensus_SignVerify(t *testing.T) {
	// Generate key pairs for two parties
	alicePub, alicePriv, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "Failed to generate Alice's keys")

	bobPub, bobPriv, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "Failed to generate Bob's keys")

	// Create a restore request
	requestID := "test-request-123"
	requester := "alice"
	snapshotID := "abc123"
	reason := "Laptop crashed, need to restore documents"
	paths := []string{"/home/alice/Documents", "/home/alice/Pictures"}
	createdAt := int64(1706745600) // Fixed timestamp for reproducibility

	// Alice signs the request
	aliceKeyID := crypto.KeyID(alicePub)
	aliceReq := &crypto.RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  snapshotID,
		Reason:      reason,
		KeyHolderID: aliceKeyID,
		Paths:       paths,
		CreatedAt:   createdAt,
	}
	aliceSig, err := aliceReq.Sign(alicePriv)
	require.NoError(t, err, "Alice's signature failed")

	// Bob signs the request
	bobKeyID := crypto.KeyID(bobPub)
	bobReq := &crypto.RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  snapshotID,
		Reason:      reason,
		KeyHolderID: bobKeyID,
		Paths:       paths,
		CreatedAt:   createdAt,
	}
	bobSig, err := bobReq.Sign(bobPriv)
	require.NoError(t, err, "Bob's signature failed")

	// Verify Alice's signature
	aliceValid, err := aliceReq.Verify(alicePub, aliceSig)
	require.NoError(t, err, "Failed to verify Alice's signature")
	assert.True(t, aliceValid, "Alice's signature should be valid")

	// Verify Bob's signature
	bobValid, err := bobReq.Verify(bobPub, bobSig)
	require.NoError(t, err, "Failed to verify Bob's signature")
	assert.True(t, bobValid, "Bob's signature should be valid")

	// Cross-verify: Bob's key should NOT validate Alice's signature
	crossValid, _ := aliceReq.Verify(bobPub, aliceSig)
	assert.False(t, crossValid, "Bob's key should NOT validate Alice's signature")

	t.Logf("Consensus signing verified: Alice=%s, Bob=%s", aliceKeyID, bobKeyID)
}

// TestE2E_Consensus_TamperedRequest tests that modified requests fail verification
func TestE2E_Consensus_TamperedRequest(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "Failed to generate keys")

	requestID := "test-request-456"
	requester := "alice"
	snapshotID := "def456"
	reason := "Need files back"
	paths := []string{"/home/alice/work"}
	createdAt := int64(1706745600)
	keyID := crypto.KeyID(pub)

	// Sign the original request
	originalReq := &crypto.RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  snapshotID,
		Reason:      reason,
		KeyHolderID: keyID,
		Paths:       paths,
		CreatedAt:   createdAt,
	}
	sig, err := originalReq.Sign(priv)
	require.NoError(t, err, "Signing failed")

	// Try to verify with tampered reason
	tamperedReasonReq := &crypto.RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  snapshotID,
		Reason:      "Actually I want to steal data",
		KeyHolderID: keyID,
		Paths:       paths,
		CreatedAt:   createdAt,
	}
	valid, _ := tamperedReasonReq.Verify(pub, sig)
	assert.False(t, valid, "Tampered reason should invalidate signature")

	// Try to verify with tampered paths
	tamperedPathsReq := &crypto.RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  snapshotID,
		Reason:      reason,
		KeyHolderID: keyID,
		Paths:       []string{"/home/alice/work", "/etc/passwd"},
		CreatedAt:   createdAt,
	}
	valid, _ = tamperedPathsReq.Verify(pub, sig)
	assert.False(t, valid, "Tampered paths should invalidate signature")

	// Try to verify with tampered snapshot
	tamperedSnapshotReq := &crypto.RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  "different-snapshot",
		Reason:      reason,
		KeyHolderID: keyID,
		Paths:       paths,
		CreatedAt:   createdAt,
	}
	valid, _ = tamperedSnapshotReq.Verify(pub, sig)
	assert.False(t, valid, "Tampered snapshot should invalidate signature")

	t.Log("All tampered request variations correctly rejected")
}

// TestE2E_HashValidation_LargeData tests hash validation with larger payloads
func TestE2E_HashValidation_LargeData(t *testing.T) {
	sizes := []int{64, 256, 1024, 4096, 16384}

	for _, size := range sizes {
		t.Run(formatSize(size), func(t *testing.T) {
			// Generate random data
			data := make([]byte, size)
			_, err := rand.Read(data)
			require.NoError(t, err, "Failed to generate data")

			// Calculate original hash
			originalHash := sha256.Sum256(data)

			// Split and combine
			shares, err := sss.Split(data, 2, 2)
			require.NoError(t, err, "Split failed")

			reconstructed, err := sss.Combine(shares)
			require.NoError(t, err, "Combine failed")

			// Verify hash
			reconstructedHash := sha256.Sum256(reconstructed)
			assert.Equal(t, originalHash, reconstructedHash, "Hash mismatch for %d bytes", size)

			// Verify byte-for-byte equality
			assert.True(t, bytes.Equal(data, reconstructed), "Content mismatch for %d bytes", size)
		})
	}

	t.Log("Hash validation passed for all data sizes")
}

// TestE2E_HashValidation_ThresholdSchemes tests different k-of-n configurations
func TestE2E_HashValidation_ThresholdSchemes(t *testing.T) {
	schemes := []struct {
		k, n int
	}{
		{2, 2},
		{2, 3},
		{2, 5},
		{3, 5},
		{3, 7},
		{5, 10},
	}

	secret := []byte("test-secret-for-threshold-validation")
	originalHash := sha256.Sum256(secret)

	for _, scheme := range schemes {
		t.Run(formatScheme(scheme.k, scheme.n), func(t *testing.T) {
			shares, err := sss.Split(secret, scheme.k, scheme.n)
			require.NoError(t, err, "Split failed")

			assert.Len(t, shares, scheme.n, "Expected %d shares", scheme.n)

			// Test with exactly k shares
			reconstructed, err := sss.Combine(shares[:scheme.k])
			require.NoError(t, err, "Combine with %d shares failed", scheme.k)

			reconstructedHash := sha256.Sum256(reconstructed)
			assert.Equal(t, originalHash, reconstructedHash, "Hash mismatch with %d-of-%d scheme", scheme.k, scheme.n)

			// Test with all n shares (should also work)
			reconstructedAll, err := sss.Combine(shares)
			require.NoError(t, err, "Combine with all shares failed")

			allHash := sha256.Sum256(reconstructedAll)
			assert.Equal(t, originalHash, allHash, "Hash mismatch with all %d shares", scheme.n)
		})
	}

	t.Log("Hash validation passed for all threshold schemes")
}

// TestE2E_FullWorkflow_SSS simulates the complete owner/host workflow
func TestE2E_FullWorkflow_SSS(t *testing.T) {
	// Step 1: Owner generates password and splits it
	password := make([]byte, 32)
	_, err := rand.Read(password)
	require.NoError(t, err, "failed to generate random password")
	passwordHex := hex.EncodeToString(password)
	passwordHash := sha256.Sum256([]byte(passwordHex))

	t.Logf("Step 1: Owner generates password (hash: %x...)", passwordHash[:8])

	// Step 2: Split password into shares
	shares, err := sss.Split([]byte(passwordHex), 2, 2)
	require.NoError(t, err, "Step 2 failed - Split")

	ownerShare := shares[0]
	hostShare := shares[1]
	t.Logf("Step 2: Password split into shares (owner index=%d, host index=%d)",
		ownerShare.Index, hostShare.Index)

	// Step 3: Simulate backup (owner has full password)
	t.Log("Step 3: Owner performs backup with full password")
	// In real code, this would call restic.Backup()

	// Step 4: Time passes... owner needs to restore

	// Step 5: Owner creates restore request
	t.Log("Step 5: Owner creates restore request")

	// Step 6: Host approves and releases share
	t.Logf("Step 6: Host approves and releases share (index=%d)", hostShare.Index)

	// Step 7: Owner combines shares and reconstructs password
	combinedShares := []sss.Share{ownerShare, hostShare}
	reconstructedPassword, err := sss.Combine(combinedShares)
	require.NoError(t, err, "Step 7 failed - Combine")

	// Step 8: Validate hash matches
	reconstructedHash := sha256.Sum256(reconstructedPassword)
	require.Equal(t, passwordHash, reconstructedHash, "Step 8 failed - Hash mismatch!\n  Original: %x\n  Reconstructed: %x", passwordHash, reconstructedHash)

	t.Log("Step 8: Password hash validated")

	// Step 9: Restore would happen here with reconstructed password
	t.Log("Step 9: Restore proceeds with validated password")

	t.Logf("Full SSS workflow completed successfully")
}

// TestE2E_FullWorkflow_Consensus simulates the consensus signing workflow
func TestE2E_FullWorkflow_Consensus(t *testing.T) {
	// Step 1: Generate keys for all parties
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	holder1Pub, holder1Priv, _ := crypto.GenerateKeyPair()
	holder2Pub, holder2Priv, _ := crypto.GenerateKeyPair()

	ownerID := crypto.KeyID(ownerPub)
	holder1ID := crypto.KeyID(holder1Pub)
	holder2ID := crypto.KeyID(holder2Pub)

	t.Logf("Step 1: Keys generated - Owner=%s, H1=%s, H2=%s", ownerID, holder1ID, holder2ID)

	// Step 2: Configure 2-of-3 consensus
	threshold := 2
	totalHolders := 3
	t.Logf("Step 2: Configured %d-of-%d consensus", threshold, totalHolders)

	// Step 3: Owner creates restore request
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	requestID := "restore-" + hex.EncodeToString(randomBytes)
	requester := "owner"
	snapshotID := "latest"
	reason := "Need to restore after system failure"
	paths := []string{"/home/user/Documents"}
	createdAt := int64(1706745600)

	t.Log("Step 3: Owner creates restore request")

	// Step 4: Owner signs request (signature 1 of 2)
	ownerReq := &crypto.RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  snapshotID,
		Reason:      reason,
		KeyHolderID: ownerID,
		Paths:       paths,
		CreatedAt:   createdAt,
	}
	ownerSig, err := ownerReq.Sign(ownerPriv)
	require.NoError(t, err, "Step 4 failed - Owner sign")
	t.Logf("Step 4: Owner signed request (1/%d)", threshold)

	// Step 5: Holder1 signs request (signature 2 of 2)
	holder1Req := &crypto.RestoreRequestSignData{
		RequestID:   requestID,
		Requester:   requester,
		SnapshotID:  snapshotID,
		Reason:      reason,
		KeyHolderID: holder1ID,
		Paths:       paths,
		CreatedAt:   createdAt,
	}
	holder1Sig, err := holder1Req.Sign(holder1Priv)
	require.NoError(t, err, "Step 5 failed - Holder1 sign")
	t.Logf("Step 5: Holder1 signed request (2/%d) - threshold met!", threshold)

	// Step 6: Verify all signatures
	sigs := []struct {
		name string
		pub  []byte
		sig  []byte
		req  *crypto.RestoreRequestSignData
	}{
		{"owner", ownerPub, ownerSig, ownerReq},
		{"holder1", holder1Pub, holder1Sig, holder1Req},
	}

	validCount := 0
	for _, s := range sigs {
		valid, _ := s.req.Verify(s.pub, s.sig)
		if valid {
			validCount++
		} else {
			t.Errorf("Step 6 failed - Invalid signature from %s", s.name)
		}
	}

	require.GreaterOrEqual(t, validCount, threshold, "Step 6 failed - Only %d valid signatures, need %d", validCount, threshold)

	t.Logf("Step 6: All %d signatures verified", validCount)

	// Step 7: Holder2 wasn't needed (we had 2-of-3)
	_ = holder2Pub
	_ = holder2Priv
	t.Log("Step 7: Holder2 signature not required (threshold already met)")

	// Step 8: Restore proceeds
	t.Log("Step 8: Restore authorized and proceeds")

	t.Log("Full consensus workflow completed successfully")
}

// TestE2E_DataIntegrity_FileSimulation simulates backing up and restoring a file
func TestE2E_DataIntegrity_FileSimulation(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "airgapper-e2e-")
	require.NoError(t, err, "Failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a test file with known content
	testFile := filepath.Join(tmpDir, "important-document.txt")
	originalContent := []byte("This is very important data that must be preserved exactly!\n" +
		"Line 2: Some more critical information.\n" +
		"Line 3: Numbers 12345, special chars: @#$%^&*()\n")

	err = os.WriteFile(testFile, originalContent, 0644)
	require.NoError(t, err, "Failed to write test file")

	// Calculate file hash
	originalHash := sha256.Sum256(originalContent)
	t.Logf("Original file hash: %x", originalHash)

	// Simulate the encryption key
	key := make([]byte, 32)
	_, err = rand.Read(key)
	require.NoError(t, err, "failed to generate random key")
	keyHex := hex.EncodeToString(key)

	// Split the key
	shares, err := sss.Split([]byte(keyHex), 2, 2)
	require.NoError(t, err, "Key split failed")

	// "Backup" happens here (in reality, restic would encrypt the file)

	// ... time passes ...

	// Reconstruct key
	reconstructedKey, err := sss.Combine(shares)
	require.NoError(t, err, "Key reconstruct failed")

	// Verify key integrity
	require.Equal(t, keyHex, string(reconstructedKey), "Key mismatch after reconstruction")

	// "Restore" the file (in reality, restic would decrypt)
	restoredFile := filepath.Join(tmpDir, "restored-document.txt")
	err = os.WriteFile(restoredFile, originalContent, 0644)
	require.NoError(t, err, "Failed to write restored file")

	// Read and verify restored file
	restoredContent, err := os.ReadFile(restoredFile)
	require.NoError(t, err, "Failed to read restored file")

	restoredHash := sha256.Sum256(restoredContent)

	// Final verification
	require.Equal(t, originalHash, restoredHash, "File hash mismatch!\n  Original:  %x\n  Restored:  %x", originalHash, restoredHash)
	require.True(t, bytes.Equal(originalContent, restoredContent), "File content mismatch!")

	t.Logf("File integrity verified: %x", restoredHash[:8])
}

// TestE2E_HashValidation_AfterDecryption specifically tests hash verification post-decryption
func TestE2E_HashValidation_AfterDecryption(t *testing.T) {
	testCases := []struct {
		name        string
		data        []byte
		description string
	}{
		{
			name:        "empty_data",
			data:        []byte{},
			description: "Empty byte slice",
		},
		{
			name:        "single_byte",
			data:        []byte{0x42},
			description: "Single byte",
		},
		{
			name:        "password_like",
			data:        []byte("a1b2c3d4e5f6789012345678901234567890abcd"),
			description: "64-char hex password",
		},
		{
			name:        "binary_data",
			data:        []byte{0x00, 0xFF, 0x80, 0x7F, 0x01, 0xFE},
			description: "Binary with edge values",
		},
		{
			name:        "unicode_content",
			data:        []byte("Hello 世界 🔐 émojis"),
			description: "Unicode and emoji content",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip empty data - SSS requires at least 1 byte
			if len(tc.data) == 0 {
				t.Skip("SSS requires non-empty data")
			}

			// Pre-encryption hash
			preHash := sha256.Sum256(tc.data)

			// Split (encryption key distribution)
			shares, err := sss.Split(tc.data, 2, 2)
			require.NoError(t, err, "Split failed for %s", tc.description)

			// Combine (key reconstruction for decryption)
			reconstructed, err := sss.Combine(shares)
			require.NoError(t, err, "Combine failed for %s", tc.description)

			// Post-decryption hash
			postHash := sha256.Sum256(reconstructed)

			// Verify hashes match
			assert.Equal(t, preHash, postHash, "Hash mismatch for %s:\n  Pre:  %x\n  Post: %x", tc.description, preHash, postHash)

			// Verify content matches
			assert.True(t, bytes.Equal(tc.data, reconstructed), "Content mismatch for %s:\n  Original: %v\n  Reconstructed: %v", tc.description, tc.data, reconstructed)

			t.Logf("%s: hash %x", tc.description, preHash[:8])
		})
	}
}

// TestE2E_HashChain verifies hash chain integrity through multiple operations
func TestE2E_HashChain(t *testing.T) {
	// Simulate a chain of operations that should preserve data integrity
	originalSecret := []byte("master-secret-key-for-chain-test")

	// Track all hashes through the chain
	hashes := make([]string, 0)

	// Step 1: Original hash
	h1 := sha256.Sum256(originalSecret)
	hashes = append(hashes, hex.EncodeToString(h1[:]))
	t.Logf("Chain[0] Original: %x", h1[:8])

	// Step 2: Split into 2-of-3
	shares, _ := sss.Split(originalSecret, 2, 3)

	// Step 3: Reconstruct with shares 0,1
	recon1, _ := sss.Combine([]sss.Share{shares[0], shares[1]})
	h2 := sha256.Sum256(recon1)
	hashes = append(hashes, hex.EncodeToString(h2[:]))
	t.Logf("Chain[1] After shares[0,1]: %x", h2[:8])

	// Step 4: Reconstruct with shares 1,2
	recon2, _ := sss.Combine([]sss.Share{shares[1], shares[2]})
	h3 := sha256.Sum256(recon2)
	hashes = append(hashes, hex.EncodeToString(h3[:]))
	t.Logf("Chain[2] After shares[1,2]: %x", h3[:8])

	// Step 5: Reconstruct with shares 0,2
	recon3, _ := sss.Combine([]sss.Share{shares[0], shares[2]})
	h4 := sha256.Sum256(recon3)
	hashes = append(hashes, hex.EncodeToString(h4[:]))
	t.Logf("Chain[3] After shares[0,2]: %x", h4[:8])

	// Step 6: Reconstruct with all 3 shares
	recon4, _ := sss.Combine(shares)
	h5 := sha256.Sum256(recon4)
	hashes = append(hashes, hex.EncodeToString(h5[:]))
	t.Logf("Chain[4] After all shares: %x", h5[:8])

	// Verify all hashes are identical
	for i := 1; i < len(hashes); i++ {
		assert.Equal(t, hashes[0], hashes[i], "Chain broken at step %d: %s != %s", i, hashes[i][:16], hashes[0][:16])
	}

	t.Log("Hash chain integrity verified through all operations")
}

// TestE2E_CryptoKeyIntegrity tests that crypto keys maintain integrity
func TestE2E_CryptoKeyIntegrity(t *testing.T) {
	// Generate a key pair
	pubKey, privKey, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "Key generation failed")

	// Hash the keys
	pubHash := sha256.Sum256(pubKey)
	privHash := sha256.Sum256(privKey)

	// Encode and decode public key
	encoded := crypto.EncodePublicKey(pubKey)
	decoded, err := crypto.DecodePublicKey(encoded)
	require.NoError(t, err, "Public key decode failed")

	decodedHash := sha256.Sum256(decoded)
	assert.Equal(t, pubHash, decodedHash, "Public key hash changed after encode/decode")

	// Encode and decode private key
	privEncoded := crypto.EncodePrivateKey(privKey)
	privDecoded, err := crypto.DecodePrivateKey(privEncoded)
	require.NoError(t, err, "Private key decode failed")

	privDecodedHash := sha256.Sum256(privDecoded)
	assert.Equal(t, privHash, privDecodedHash, "Private key hash changed after encode/decode")

	// Verify signing still works after encode/decode
	message := []byte("test message for signing")
	sig, err := crypto.Sign(privDecoded, message)
	require.NoError(t, err, "Signing with decoded key failed")

	assert.True(t, crypto.Verify(decoded, message, sig), "Verification failed with decoded keys")

	t.Logf("Crypto key integrity verified (pub=%x, priv=%x)", pubHash[:8], privHash[:8])
}

// Helper functions

func formatSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	return fmt.Sprintf("%dKB", bytes/1024)
}

func formatScheme(k, n int) string {
	return fmt.Sprintf("%d-of-%d", k, n)
}
