package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func TestManifestManager(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "manifest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate keys
	pub, priv, _ := crypto.GenerateKeyPair()
	keyID := crypto.KeyID(pub)

	// Create manager
	mm, err := NewManifestManager(tmpDir, keyID, priv, pub)
	if err != nil {
		t.Fatalf("failed to create manifest manager: %v", err)
	}

	// Initialize
	if err := mm.Initialize("policy-123"); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	// Verify manifest was created
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest file should exist")
	}

	// Add snapshots
	snap1 := SnapshotEntry{
		ID:        "abc123",
		CreatedAt: time.Now(),
		Paths:     []string{"/home/user"},
		Size:      1024,
	}
	if err := mm.AddSnapshot(snap1); err != nil {
		t.Fatalf("failed to add snapshot: %v", err)
	}

	snap2 := SnapshotEntry{
		ID:        "def456",
		CreatedAt: time.Now(),
		Paths:     []string{"/home/user", "/etc"},
		Size:      2048,
	}
	if err := mm.AddSnapshot(snap2); err != nil {
		t.Fatalf("failed to add snapshot: %v", err)
	}

	// Check count
	if mm.SnapshotCount() != 2 {
		t.Errorf("expected 2 snapshots, got %d", mm.SnapshotCount())
	}

	// Verify
	if err := mm.Verify(); err != nil {
		t.Errorf("verification failed: %v", err)
	}

	// Get specific snapshot
	s := mm.GetSnapshot("abc123")
	if s == nil {
		t.Error("expected to find snapshot abc123")
	}
	if s.Size != 1024 {
		t.Errorf("expected size 1024, got %d", s.Size)
	}

	// Remove snapshot
	if err := mm.RemoveSnapshot("abc123"); err != nil {
		t.Fatalf("failed to remove snapshot: %v", err)
	}

	if mm.SnapshotCount() != 1 {
		t.Errorf("expected 1 snapshot after removal, got %d", mm.SnapshotCount())
	}

	// Verify still works after removal
	if err := mm.Verify(); err != nil {
		t.Errorf("verification failed after removal: %v", err)
	}
}

func TestManifestDuplicatePrevention(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "manifest-test-*")
	defer os.RemoveAll(tmpDir)

	pub, priv, _ := crypto.GenerateKeyPair()
	mm, _ := NewManifestManager(tmpDir, crypto.KeyID(pub), priv, pub)
	mm.Initialize("policy-123")

	snap := SnapshotEntry{ID: "abc123", CreatedAt: time.Now()}
	mm.AddSnapshot(snap)

	// Try to add duplicate
	err := mm.AddSnapshot(snap)
	if err == nil {
		t.Error("expected error when adding duplicate snapshot")
	}
}

func TestManifestPersistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "manifest-test-*")
	defer os.RemoveAll(tmpDir)

	pub, priv, _ := crypto.GenerateKeyPair()
	keyID := crypto.KeyID(pub)

	// Create and initialize
	mm1, _ := NewManifestManager(tmpDir, keyID, priv, pub)
	mm1.Initialize("policy-123")
	mm1.AddSnapshot(SnapshotEntry{ID: "snap1", CreatedAt: time.Now(), Size: 100})
	mm1.AddSnapshot(SnapshotEntry{ID: "snap2", CreatedAt: time.Now(), Size: 200})

	// Create new manager pointing to same directory
	mm2, err := NewManifestManager(tmpDir, keyID, priv, pub)
	if err != nil {
		t.Fatalf("failed to create second manager: %v", err)
	}

	// Should have loaded existing manifest
	if mm2.SnapshotCount() != 2 {
		t.Errorf("expected 2 snapshots loaded, got %d", mm2.SnapshotCount())
	}

	// Verify integrity
	if err := mm2.Verify(); err != nil {
		t.Errorf("verification of loaded manifest failed: %v", err)
	}
}

func TestManifestTamperDetection(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "manifest-test-*")
	defer os.RemoveAll(tmpDir)

	pub, priv, _ := crypto.GenerateKeyPair()
	mm, _ := NewManifestManager(tmpDir, crypto.KeyID(pub), priv, pub)
	mm.Initialize("policy-123")
	mm.AddSnapshot(SnapshotEntry{ID: "snap1", CreatedAt: time.Now()})

	// Tamper with the manifest directly
	mm.manifest.Snapshots[0].ID = "tampered"

	// Verification should fail
	if err := mm.Verify(); err == nil {
		t.Error("verification should fail after tampering")
	}
}

func TestIntegrityCheck(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "manifest-test-*")
	defer os.RemoveAll(tmpDir)

	pub, priv, _ := crypto.GenerateKeyPair()
	mm, _ := NewManifestManager(tmpDir, crypto.KeyID(pub), priv, pub)
	mm.Initialize("policy-123")

	// Add snapshots to manifest
	mm.AddSnapshot(SnapshotEntry{ID: "snap1", CreatedAt: time.Now()})
	mm.AddSnapshot(SnapshotEntry{ID: "snap2", CreatedAt: time.Now()})
	mm.AddSnapshot(SnapshotEntry{ID: "snap3", CreatedAt: time.Now()})

	// Mock storage that returns only snap1 and snap4 (snap2, snap3 missing, snap4 unexpected)
	mockStorage := func() ([]string, error) {
		return []string{"snap1", "snap4"}, nil
	}

	report, err := mm.CheckIntegrity(mockStorage)
	if err != nil {
		t.Fatalf("CheckIntegrity failed: %v", err)
	}

	if report.Verified {
		t.Error("integrity check should fail with missing snapshots")
	}

	if report.TotalInManifest != 3 {
		t.Errorf("expected 3 in manifest, got %d", report.TotalInManifest)
	}

	if report.TotalOnStorage != 2 {
		t.Errorf("expected 2 on storage, got %d", report.TotalOnStorage)
	}

	// Check missing
	if len(report.Missing) != 2 {
		t.Errorf("expected 2 missing, got %d", len(report.Missing))
	}

	// Check unexpected
	if len(report.Unexpected) != 1 {
		t.Errorf("expected 1 unexpected, got %d", len(report.Unexpected))
	}
	if len(report.Unexpected) > 0 && report.Unexpected[0] != "snap4" {
		t.Errorf("expected unexpected snap4, got %s", report.Unexpected[0])
	}
}

func TestIntegrityCheckAllPresent(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "manifest-test-*")
	defer os.RemoveAll(tmpDir)

	pub, priv, _ := crypto.GenerateKeyPair()
	mm, _ := NewManifestManager(tmpDir, crypto.KeyID(pub), priv, pub)
	mm.Initialize("policy-123")

	mm.AddSnapshot(SnapshotEntry{ID: "snap1", CreatedAt: time.Now()})
	mm.AddSnapshot(SnapshotEntry{ID: "snap2", CreatedAt: time.Now()})

	// Mock storage returns all expected snapshots
	mockStorage := func() ([]string, error) {
		return []string{"snap1", "snap2"}, nil
	}

	report, _ := mm.CheckIntegrity(mockStorage)

	if !report.Verified {
		t.Error("integrity check should pass when all snapshots present")
	}

	if len(report.Missing) != 0 {
		t.Errorf("expected no missing, got %d", len(report.Missing))
	}

	if len(report.Unexpected) != 0 {
		t.Errorf("expected no unexpected, got %d", len(report.Unexpected))
	}
}

func TestMerkleRoot(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "manifest-test-*")
	defer os.RemoveAll(tmpDir)

	pub, priv, _ := crypto.GenerateKeyPair()
	mm, _ := NewManifestManager(tmpDir, crypto.KeyID(pub), priv, pub)
	mm.Initialize("policy-123")

	mm.AddSnapshot(SnapshotEntry{ID: "aaa", CreatedAt: time.Now()})
	root1 := mm.manifest.MerkleRoot

	mm.AddSnapshot(SnapshotEntry{ID: "bbb", CreatedAt: time.Now()})
	root2 := mm.manifest.MerkleRoot

	// Root should change when snapshots added
	if root1 == root2 {
		t.Error("merkle root should change when snapshots added")
	}

	// Empty manifest should have empty root
	tmpDir2, _ := os.MkdirTemp("", "manifest-test-*")
	defer os.RemoveAll(tmpDir2)
	mm2, _ := NewManifestManager(tmpDir2, crypto.KeyID(pub), priv, pub)
	mm2.Initialize("policy-456")

	if mm2.manifest.MerkleRoot != "" {
		t.Error("empty manifest should have empty merkle root")
	}
}
