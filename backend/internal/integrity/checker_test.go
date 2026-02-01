package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func setupTestRepo(t *testing.T, basePath, repoName string) {
	// Create repo structure
	repoPath := filepath.Join(basePath, repoName)
	dirs := []string{"data", "keys", "snapshots", "index", "locks"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(repoPath, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create config file
	configData := []byte("test config data")
	os.WriteFile(filepath.Join(repoPath, "config"), configData, 0644)

	// Create some data files (content-addressable)
	for i := 0; i < 5; i++ {
		content := []byte{byte(i), byte(i + 1), byte(i + 2)}
		hash := sha256.Sum256(content)
		hashHex := hex.EncodeToString(hash[:])

		// Data files go in subdirectories by first 2 chars
		subdir := filepath.Join(repoPath, "data", hashHex[:2])
		os.MkdirAll(subdir, 0755)
		os.WriteFile(filepath.Join(subdir, hashHex), content, 0644)
	}

	// Create a key file
	keyData := []byte("key data")
	keyHash := sha256.Sum256(keyData)
	os.WriteFile(filepath.Join(repoPath, "keys", hex.EncodeToString(keyHash[:])), keyData, 0644)

	// Create a snapshot file
	snapshotData := []byte("snapshot data")
	os.WriteFile(filepath.Join(repoPath, "snapshots", "snap123"), snapshotData, 0644)
}

func TestCheckDataIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, err := NewChecker(tmpDir)
	if err != nil {
		t.Fatalf("failed to create checker: %v", err)
	}

	result, err := checker.CheckDataIntegrity("testrepo")
	if err != nil {
		t.Fatalf("CheckDataIntegrity failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected check to pass, got errors: %v", result.Errors)
	}

	if result.TotalFiles != 5 {
		t.Errorf("expected 5 data files, got %d", result.TotalFiles)
	}

	if result.CorruptFiles != 0 {
		t.Errorf("expected 0 corrupt files, got %d", result.CorruptFiles)
	}
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
			os.WriteFile(filePath, []byte("CORRUPTED DATA"), 0644)
		}
	}

	checker, _ := NewChecker(tmpDir)
	result, _ := checker.CheckDataIntegrity("testrepo")

	if result.Passed {
		t.Error("expected check to fail due to corruption")
	}

	if result.CorruptFiles == 0 {
		t.Error("expected corrupt files to be detected")
	}
}

func TestCreateVerificationRecord(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, _ := NewChecker(tmpDir)

	record, err := checker.CreateVerificationRecord("testrepo", "snap123", "owner-key-123")
	if err != nil {
		t.Fatalf("CreateVerificationRecord failed: %v", err)
	}

	if record.SnapshotID != "snap123" {
		t.Errorf("expected snapshot ID snap123, got %s", record.SnapshotID)
	}

	if record.ConfigHash == "" {
		t.Error("expected config hash to be set")
	}

	if record.SnapshotHash == "" {
		t.Error("expected snapshot hash to be set")
	}

	if record.DataMerkleRoot == "" {
		t.Error("expected data merkle root to be set")
	}

	if record.DataFileCount != 5 {
		t.Errorf("expected 5 data files, got %d", record.DataFileCount)
	}
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
	if err != nil {
		t.Fatalf("AddVerificationRecord failed: %v", err)
	}

	// Retrieve it
	retrieved := checker.GetVerificationRecord("snap123")
	if retrieved == nil {
		t.Fatal("expected to retrieve record")
	}

	if retrieved.ConfigHash != record.ConfigHash {
		t.Error("config hash mismatch")
	}
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
	if err == nil {
		t.Error("expected verification to fail after tampering")
	}
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
	if err != nil {
		t.Fatalf("VerifyAgainstRecord failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected verification to pass, got errors: %v", result.Errors)
	}
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
		os.RemoveAll(filepath.Join(dataPath, entries[0].Name()))
	}

	// Verify against record (should fail)
	result, _ := checker.VerifyAgainstRecord("testrepo", record)

	if result.Passed {
		t.Error("expected verification to fail after deleting files")
	}

	if len(result.Errors) == 0 {
		t.Error("expected errors to be reported")
	}
}

func TestCheckHistory(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	checker, _ := NewChecker(tmpDir)

	// Run a few checks
	checker.CheckDataIntegrity("testrepo")
	checker.CheckDataIntegrity("testrepo")
	checker.CheckDataIntegrity("testrepo")

	history := checker.GetHistory(10)
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}

	// All should pass
	for _, h := range history {
		if !h.Passed {
			t.Error("expected all checks to pass")
		}
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

	checker1.AddVerificationRecord(record, pubKey)

	// Create new checker (should load persisted records)
	checker2, _ := NewChecker(tmpDir)

	retrieved := checker2.GetVerificationRecord("snap123")
	if retrieved == nil {
		t.Fatal("expected record to be persisted and loaded")
	}

	if retrieved.ConfigHash != record.ConfigHash {
		t.Error("persisted record doesn't match")
	}
}
