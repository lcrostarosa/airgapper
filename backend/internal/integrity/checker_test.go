package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(t *testing.T, basePath, repoName string) {
	// Create repo structure
	repoPath := filepath.Join(basePath, repoName)
	dirs := []string{"data", "keys", "snapshots", "index", "locks"}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(repoPath, dir), 0755)
		require.NoError(t, err, "failed to create dir %s", dir)
	}

	// Create config file
	configData := []byte("test config data")
	err := os.WriteFile(filepath.Join(repoPath, "config"), configData, 0644)
	require.NoError(t, err, "failed to write config")

	// Create some data files (content-addressable)
	for i := 0; i < 5; i++ {
		content := []byte{byte(i), byte(i + 1), byte(i + 2)}
		hash := sha256.Sum256(content)
		hashHex := hex.EncodeToString(hash[:])

		// Data files go in subdirectories by first 2 chars
		subdir := filepath.Join(repoPath, "data", hashHex[:2])
		err = os.MkdirAll(subdir, 0755)
		require.NoError(t, err, "failed to create subdir")
		err = os.WriteFile(filepath.Join(subdir, hashHex), content, 0644)
		require.NoError(t, err, "failed to write data file")
	}

	// Create a key file
	keyData := []byte("key data")
	keyHash := sha256.Sum256(keyData)
	err = os.WriteFile(filepath.Join(repoPath, "keys", hex.EncodeToString(keyHash[:])), keyData, 0644)
	require.NoError(t, err, "failed to write key file")

	// Create a snapshot file
	snapshotData := []byte("snapshot data")
	err = os.WriteFile(filepath.Join(repoPath, "snapshots", "snap123"), snapshotData, 0644)
	require.NoError(t, err, "failed to write snapshot file")
}

func TestCheckDataIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, err := NewChecker(tmpDir)
	require.NoError(t, err, "failed to create checker")

	result, err := checker.CheckDataIntegrity("testrepo")
	require.NoError(t, err, "CheckDataIntegrity failed")

	assert.True(t, result.Passed, "expected check to pass, got errors: %v", result.Errors)
	assert.Equal(t, 5, result.TotalFiles, "expected 5 data files")
	assert.Equal(t, 0, result.CorruptFiles, "expected 0 corrupt files")
}

func TestCheckDataIntegrity_CorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	// Corrupt a data file (write different content with same name)
	dataPath := filepath.Join(tmpDir, "testrepo", "data")
	entries, _ := os.ReadDir(dataPath)
	if len(entries) > 0 {
		subdir := entries[0].Name()
		subEntries, _ := os.ReadDir(filepath.Join(dataPath, subdir))
		if len(subEntries) > 0 {
			// Write garbage to file
			filePath := filepath.Join(dataPath, subdir, subEntries[0].Name())
			_ = os.WriteFile(filePath, []byte("CORRUPTED DATA"), 0644)
		}
	}

	checker, _ := NewChecker(tmpDir)
	result, _ := checker.CheckDataIntegrity("testrepo")

	assert.False(t, result.Passed, "expected check to fail due to corruption")
	assert.NotEqual(t, 0, result.CorruptFiles, "expected corrupt files to be detected")
}

func TestCreateVerificationRecord(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, _ := NewChecker(tmpDir)

	record, err := checker.CreateVerificationRecord("testrepo", "snap123", "owner-key-123")
	require.NoError(t, err, "CreateVerificationRecord failed")

	assert.Equal(t, "snap123", record.SnapshotID, "expected snapshot ID snap123")
	assert.NotEmpty(t, record.ConfigHash, "expected config hash to be set")
	assert.NotEmpty(t, record.SnapshotHash, "expected snapshot hash to be set")
	assert.NotEmpty(t, record.DataMerkleRoot, "expected data merkle root to be set")
	assert.Equal(t, 5, record.DataFileCount, "expected 5 data files")
}

func TestSignedVerificationRecord(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, _ := NewChecker(tmpDir)

	// Generate owner keys
	pubKey, privKey, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(pubKey)

	// Create record
	record, _ := checker.CreateVerificationRecord("testrepo", "snap123", ownerKeyID)

	// Sign it
	hash, _ := checker.hashRecord(record)
	sig, _ := crypto.Sign(privKey, hash)
	record.Signature = hex.EncodeToString(sig)

	// Add to checker (should verify signature)
	err := checker.AddVerificationRecord(record, pubKey)
	require.NoError(t, err, "AddVerificationRecord failed")

	// Retrieve it
	retrieved := checker.GetVerificationRecord("snap123")
	require.NotNil(t, retrieved, "expected to retrieve record")

	assert.Equal(t, record.ConfigHash, retrieved.ConfigHash, "config hash mismatch")
}

func TestSignedVerificationRecord_TamperedSignature(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, _ := NewChecker(tmpDir)

	pubKey, privKey, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(pubKey)

	record, _ := checker.CreateVerificationRecord("testrepo", "snap123", ownerKeyID)

	// Sign it
	hash, _ := checker.hashRecord(record)
	sig, _ := crypto.Sign(privKey, hash)
	record.Signature = hex.EncodeToString(sig)

	// Tamper with the record after signing
	record.DataFileCount = 9999

	// Should fail verification
	err := checker.AddVerificationRecord(record, pubKey)
	assert.Error(t, err, "expected verification to fail after tampering")
}

func TestVerifyAgainstRecord(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, _ := NewChecker(tmpDir)

	pubKey, _, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(pubKey)

	// Create record of current state
	record, _ := checker.CreateVerificationRecord("testrepo", "snap123", ownerKeyID)

	// Verify against record (should pass)
	result, err := checker.VerifyAgainstRecord("testrepo", record)
	require.NoError(t, err, "VerifyAgainstRecord failed")

	assert.True(t, result.Passed, "expected verification to pass, got errors: %v", result.Errors)
}

func TestVerifyAgainstRecord_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, _ := NewChecker(tmpDir)

	pubKey, _, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(pubKey)

	// Create record of current state
	record, _ := checker.CreateVerificationRecord("testrepo", "snap123", ownerKeyID)

	// Delete some data files
	dataPath := filepath.Join(tmpDir, "testrepo", "data")
	entries, _ := os.ReadDir(dataPath)
	if len(entries) > 0 {
		_ = os.RemoveAll(filepath.Join(dataPath, entries[0].Name()))
	}

	// Verify against record (should fail)
	result, _ := checker.VerifyAgainstRecord("testrepo", record)

	assert.False(t, result.Passed, "expected verification to fail after deleting files")
	assert.NotEmpty(t, result.Errors, "expected errors to be reported")
}

func TestCheckHistory(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, _ := NewChecker(tmpDir)

	// Run a few checks
	_, err := checker.CheckDataIntegrity("testrepo")
	require.NoError(t, err, "first check failed")
	_, err = checker.CheckDataIntegrity("testrepo")
	require.NoError(t, err, "second check failed")
	_, err = checker.CheckDataIntegrity("testrepo")
	require.NoError(t, err, "third check failed")

	history := checker.GetHistory(10)
	assert.Len(t, history, 3, "expected 3 history entries")

	// All should pass
	for _, h := range history {
		assert.True(t, h.Passed, "expected all checks to pass")
	}
}

func TestRecordPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	pubKey, privKey, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(pubKey)

	// Create checker and add record
	checker1, _ := NewChecker(tmpDir)
	record, _ := checker1.CreateVerificationRecord("testrepo", "snap123", ownerKeyID)

	hash, _ := checker1.hashRecord(record)
	sig, _ := crypto.Sign(privKey, hash)
	record.Signature = hex.EncodeToString(sig)

	err := checker1.AddVerificationRecord(record, pubKey)
	require.NoError(t, err, "failed to add verification record")

	// Create new checker (should load persisted records)
	checker2, _ := NewChecker(tmpDir)

	retrieved := checker2.GetVerificationRecord("snap123")
	require.NotNil(t, retrieved, "expected record to be persisted and loaded")

	assert.Equal(t, record.ConfigHash, retrieved.ConfigHash, "persisted record doesn't match")
}
