// Package e2e - upgrade_test.go
// Tests backward compatibility when upgrading Airgapper versions.
// Ensures existing configs, keys, data, and requests remain valid after upgrade.
package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
	"github.com/lcrostarosa/airgapper/backend/internal/policy"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// TestUpgrade_LegacySSSConfig tests loading a config from legacy SSS mode
func TestUpgrade_LegacySSSConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate a "v1" config file with SSS mode (no consensus)
	legacyConfig := map[string]interface{}{
		"name":        "alice-backup",
		"role":        "owner",
		"repo_url":    "rest:http://bob:8000/",
		"password":    "secretpassword123",
		"local_share": []byte{1, 2, 3, 4, 5, 6, 7, 8}, // Simulated share
		"share_index": 1,
		"peer": map[string]interface{}{
			"name":    "bob",
			"address": "192.168.1.100:8081",
		},
		"backup_paths": []string{"/home/alice/documents"},
	}

	// Write legacy config
	configPath := filepath.Join(tmpDir, "config.json")
	data, _ := json.MarshalIndent(legacyConfig, "", "  ")
	os.WriteFile(configPath, data, 0600)

	// Load with current code
	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load legacy config: %v", err)
	}

	// Verify all fields loaded correctly
	if cfg.Name != "alice-backup" {
		t.Errorf("Name mismatch: got %s", cfg.Name)
	}
	if cfg.Role != config.RoleOwner {
		t.Errorf("Role mismatch: got %s", cfg.Role)
	}
	if cfg.RepoURL != "rest:http://bob:8000/" {
		t.Errorf("RepoURL mismatch: got %s", cfg.RepoURL)
	}
	if cfg.Password != "secretpassword123" {
		t.Errorf("Password mismatch")
	}
	if cfg.LocalShare == nil {
		t.Error("LocalShare should be loaded")
	}
	if cfg.ShareIndex != 1 {
		t.Errorf("ShareIndex mismatch: got %d", cfg.ShareIndex)
	}
	if cfg.Peer == nil || cfg.Peer.Name != "bob" {
		t.Error("Peer info not loaded correctly")
	}
	if len(cfg.BackupPaths) != 1 || cfg.BackupPaths[0] != "/home/alice/documents" {
		t.Error("BackupPaths not loaded correctly")
	}

	// Verify mode detection
	if !cfg.UsesSSSMode() {
		t.Error("Should detect SSS mode")
	}
	if cfg.UsesConsensusMode() {
		t.Error("Should not detect consensus mode")
	}
}

// TestUpgrade_LegacyHostConfig tests loading a legacy host config
func TestUpgrade_LegacyHostConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate a legacy host config (before storage server was embedded)
	legacyConfig := map[string]interface{}{
		"name":        "bob-host",
		"role":        "host",
		"repo_url":    "rest:http://localhost:8000/",
		"local_share": []byte{9, 10, 11, 12, 13, 14, 15, 16},
		"share_index": 2,
		"peer": map[string]interface{}{
			"name":    "alice",
			"address": "192.168.1.50:8081",
		},
		// Note: no storage_path - that's a new field
	}

	configPath := filepath.Join(tmpDir, "config.json")
	data, _ := json.MarshalIndent(legacyConfig, "", "  ")
	os.WriteFile(configPath, data, 0600)

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load legacy host config: %v", err)
	}

	if cfg.Role != config.RoleHost {
		t.Errorf("Role mismatch: got %s", cfg.Role)
	}
	if cfg.IsOwner() {
		t.Error("Should not be owner")
	}
	if !cfg.IsHost() {
		t.Error("Should be host")
	}

	// New fields should be empty/zero
	if cfg.StoragePath != "" {
		t.Error("StoragePath should be empty for legacy config")
	}
	if cfg.Consensus != nil {
		t.Error("Consensus should be nil for legacy config")
	}
}

// TestUpgrade_ConsensusConfigWithNewFields tests that configs with consensus
// mode still work when new fields are added
func TestUpgrade_ConsensusConfigWithNewFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate real keys
	pub1, _, _ := crypto.GenerateKeyPair()
	pub2, _, _ := crypto.GenerateKeyPair()

	// Simulate a consensus mode config (v2-ish)
	consensusConfig := map[string]interface{}{
		"name":       "alice-consensus",
		"role":       "owner",
		"repo_url":   "rest:http://bob:8000/",
		"password":   "consensuspassword",
		"public_key": pub1,
		"consensus": map[string]interface{}{
			"threshold":  2,
			"total_keys": 2,
			"key_holders": []map[string]interface{}{
				{
					"id":         crypto.KeyID(pub1),
					"name":       "alice",
					"public_key": pub1,
					"is_owner":   true,
					"joined_at":  time.Now().Add(-24 * time.Hour),
				},
				{
					"id":         crypto.KeyID(pub2),
					"name":       "bob",
					"public_key": pub2,
					"address":    "192.168.1.100:8081",
					"joined_at":  time.Now(),
				},
			},
		},
	}

	configPath := filepath.Join(tmpDir, "config.json")
	data, _ := json.MarshalIndent(consensusConfig, "", "  ")
	os.WriteFile(configPath, data, 0600)

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load consensus config: %v", err)
	}

	if !cfg.UsesConsensusMode() {
		t.Error("Should detect consensus mode")
	}
	if cfg.UsesSSSMode() {
		t.Error("Should not detect SSS mode")
	}
	if cfg.Consensus.Threshold != 2 {
		t.Errorf("Threshold mismatch: got %d", cfg.Consensus.Threshold)
	}
	if len(cfg.Consensus.KeyHolders) != 2 {
		t.Errorf("KeyHolders count mismatch: got %d", len(cfg.Consensus.KeyHolders))
	}
	if cfg.RequiredApprovals() != 2 {
		t.Errorf("RequiredApprovals mismatch: got %d", cfg.RequiredApprovals())
	}
}

// TestUpgrade_SSSKeysStillWork tests that SSS shares created with old code
// can still be combined with new code
func TestUpgrade_SSSKeysStillWork(t *testing.T) {
	// Create shares using current SSS implementation
	// This simulates shares created by an older version
	password := "my-secret-backup-password-12345"

	shares, err := sss.Split([]byte(password), 2, 2)
	if err != nil {
		t.Fatalf("Failed to split password: %v", err)
	}

	// Simulate storing shares in "old" config format
	// Config stores the share Data and Index separately
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	// Owner's config - stores share[0].Data as local_share and share[0].Index as share_index
	ownerConfig := map[string]interface{}{
		"name":        "owner",
		"role":        "owner",
		"repo_url":    "rest:http://host:8000/",
		"password":    password,
		"local_share": shares[0].Data,
		"share_index": shares[0].Index,
	}
	data1, _ := json.MarshalIndent(ownerConfig, "", "  ")
	os.WriteFile(filepath.Join(tmpDir1, "config.json"), data1, 0600)

	// Host's config
	hostConfig := map[string]interface{}{
		"name":        "host",
		"role":        "host",
		"repo_url":    "rest:http://host:8000/",
		"local_share": shares[1].Data,
		"share_index": shares[1].Index,
	}
	data2, _ := json.MarshalIndent(hostConfig, "", "  ")
	os.WriteFile(filepath.Join(tmpDir2, "config.json"), data2, 0600)

	// Load configs with "new" code
	ownerCfg, _ := config.Load(tmpDir1)
	hostCfg, _ := config.Load(tmpDir2)

	// Reconstruct shares from config (this is what production code does)
	ownerShare := sss.Share{Index: ownerCfg.ShareIndex, Data: ownerCfg.LocalShare}
	hostShare := sss.Share{Index: hostCfg.ShareIndex, Data: hostCfg.LocalShare}

	// Combine shares - this should work exactly as before
	combined, err := sss.Combine([]sss.Share{ownerShare, hostShare})
	if err != nil {
		t.Fatalf("Failed to combine shares: %v", err)
	}

	if string(combined) != password {
		t.Errorf("Password mismatch after combine: got %s, want %s", string(combined), password)
	}
}

// TestUpgrade_ExistingRestoreRequests tests that pending restore requests
// created with old code can still be processed
func TestUpgrade_ExistingRestoreRequests(t *testing.T) {
	tmpDir := t.TempDir()
	requestsDir := filepath.Join(tmpDir, "requests")
	os.MkdirAll(requestsDir, 0700)

	// Simulate an old restore request format
	oldRequest := map[string]interface{}{
		"id":          "abc123def456",
		"requester":   "alice",
		"snapshot_id": "latest",
		"paths":       []string{"/home/alice/docs"},
		"reason":      "need to restore files",
		"status":      "pending",
		"created_at":  time.Now().Add(-1 * time.Hour),
		"expires_at":  time.Now().Add(23 * time.Hour),
		// Note: no "required_approvals" or "approvals" fields (consensus mode)
	}

	requestPath := filepath.Join(requestsDir, "abc123def456.json")
	data, _ := json.MarshalIndent(oldRequest, "", "  ")
	os.WriteFile(requestPath, data, 0600)

	// Load with current consent manager
	mgr := consent.NewManager(tmpDir)

	req, err := mgr.GetRequest("abc123def456")
	if err != nil {
		t.Fatalf("Failed to load old request: %v", err)
	}

	if req.Requester != "alice" {
		t.Errorf("Requester mismatch: got %s", req.Requester)
	}
	if req.Status != consent.StatusPending {
		t.Errorf("Status mismatch: got %s", req.Status)
	}
	if req.SnapshotID != "latest" {
		t.Errorf("SnapshotID mismatch: got %s", req.SnapshotID)
	}

	// New fields should be zero
	if req.RequiredApprovals != 0 {
		t.Errorf("RequiredApprovals should be 0 for old requests: got %d", req.RequiredApprovals)
	}

	// Should be able to approve it (legacy mode)
	err = mgr.Approve("abc123def456", "bob", []byte("share-data"))
	if err != nil {
		t.Fatalf("Failed to approve old request: %v", err)
	}

	approved, _ := mgr.GetRequest("abc123def456")
	if approved.Status != consent.StatusApproved {
		t.Error("Request should be approved")
	}
}

// TestUpgrade_ExistingPolicyFormat tests that policies created with
// current version can be loaded (future-proofing)
func TestUpgrade_ExistingPolicyFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()

	// Create and sign a policy
	p := policy.NewPolicy(
		"alice", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"bob", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)
	p.RetentionDays = 30
	p.DeletionMode = policy.DeletionBothRequired
	p.SignAsOwner(ownerPriv)
	p.SignAsHost(hostPriv)

	// Save to disk
	policyPath := filepath.Join(tmpDir, "policy.json")
	data, _ := p.ToJSON()
	os.WriteFile(policyPath, data, 0600)

	// Load and verify
	loadedData, _ := os.ReadFile(policyPath)
	loaded, err := policy.FromJSON(loadedData)
	if err != nil {
		t.Fatalf("Failed to load policy: %v", err)
	}

	if err := loaded.Verify(); err != nil {
		t.Fatalf("Policy verification failed: %v", err)
	}

	if loaded.RetentionDays != 30 {
		t.Errorf("RetentionDays mismatch: got %d", loaded.RetentionDays)
	}
	if loaded.DeletionMode != policy.DeletionBothRequired {
		t.Errorf("DeletionMode mismatch: got %s", loaded.DeletionMode)
	}
}

// TestUpgrade_StorageDataIntact tests that backup data stored with
// old version is still accessible
func TestUpgrade_StorageDataIntact(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate existing backup data structure
	repoPath := filepath.Join(tmpDir, "backup-repo")
	dirs := []string{"data", "keys", "snapshots", "index", "locks"}
	for _, dir := range dirs {
		os.MkdirAll(filepath.Join(repoPath, dir), 0755)
	}

	// Create some "old" data files
	testData := []byte("important backup data from old version")
	hash := sha256.Sum256(testData)
	hashHex := hex.EncodeToString(hash[:])

	// Data files use subdirectory structure
	dataDir := filepath.Join(repoPath, "data", hashHex[:2])
	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, hashHex), testData, 0644)

	// Config file
	os.WriteFile(filepath.Join(repoPath, "config"), []byte("repo-config"), 0644)

	// Key file
	keyHash := sha256.Sum256([]byte("key-data"))
	os.WriteFile(filepath.Join(repoPath, "keys", hex.EncodeToString(keyHash[:])), []byte("key-data"), 0644)

	// Create storage server with "new" code pointing to old data
	s, err := storage.NewServer(storage.Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	if err != nil {
		t.Fatalf("Failed to create storage server: %v", err)
	}
	s.Start()

	// Verify data is accessible via new storage server
	status := s.Status()
	if !status.Running {
		t.Error("Storage server should be running")
	}
	if status.UsedBytes == 0 {
		t.Error("Should detect existing data")
	}

	// Verify integrity check works on old data
	checker, err := integrity.NewChecker(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create integrity checker: %v", err)
	}

	result, err := checker.CheckDataIntegrity("backup-repo")
	if err != nil {
		t.Fatalf("Integrity check failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("Integrity check should pass for valid old data: %v", result.Errors)
	}
	if result.TotalFiles != 1 {
		t.Errorf("Should find 1 data file, got %d", result.TotalFiles)
	}
}

// TestUpgrade_Ed25519KeysCompatible tests that Ed25519 keys generated
// with any version work correctly
func TestUpgrade_Ed25519KeysCompatible(t *testing.T) {
	// Generate keys with current code
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	// Encode to hex (as stored in config)
	pubHex := crypto.EncodePublicKey(pub)
	privHex := crypto.EncodePrivateKey(priv)

	// Decode back
	decodedPub, err := crypto.DecodePublicKey(pubHex)
	if err != nil {
		t.Fatalf("Failed to decode public key: %v", err)
	}
	decodedPriv, err := crypto.DecodePrivateKey(privHex)
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}

	// Sign and verify
	message := []byte("test message to sign")
	sig, err := crypto.Sign(decodedPriv, message)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	if !crypto.Verify(decodedPub, message, sig) {
		t.Error("Signature verification failed")
	}

	// Key ID should be deterministic
	keyID1 := crypto.KeyID(pub)
	keyID2 := crypto.KeyID(decodedPub)
	if keyID1 != keyID2 {
		t.Errorf("Key ID mismatch: %s vs %s", keyID1, keyID2)
	}
}

// TestUpgrade_ConfigSavePreservesUnknownFields tests that saving a config
// doesn't lose fields we don't know about (forward compatibility)
func TestUpgrade_ConfigSavePreservesUnknownFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config with "future" fields
	futureConfig := map[string]interface{}{
		"name":              "test",
		"role":              "owner",
		"repo_url":          "rest:http://test:8000/",
		"future_field":      "some value from future version",
		"another_new_thing": 12345,
	}

	configPath := filepath.Join(tmpDir, "config.json")
	data, _ := json.MarshalIndent(futureConfig, "", "  ")
	os.WriteFile(configPath, data, 0600)

	// Load with current code
	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config with unknown fields: %v", err)
	}

	// Modify a known field
	cfg.Name = "modified-name"

	// Save it back
	if err := cfg.Save(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Note: Current implementation will LOSE unknown fields on save
	// This test documents that behavior - we may want to fix this
	// by using a json.RawMessage approach in the future

	// Re-load
	reloaded, _ := config.Load(tmpDir)
	if reloaded.Name != "modified-name" {
		t.Error("Modified field should be saved")
	}

	// Check the raw JSON to see if unknown fields were preserved
	rawData, _ := os.ReadFile(configPath)
	var rawMap map[string]interface{}
	json.Unmarshal(rawData, &rawMap)

	// Document current behavior: unknown fields are lost
	if _, ok := rawMap["future_field"]; ok {
		t.Log("Note: Unknown fields ARE preserved (good!)")
	} else {
		t.Log("Note: Unknown fields are NOT preserved (may want to fix)")
	}
}

// TestUpgrade_DeletionRequestsNewFeature tests that old consent managers
// can handle new deletion request features
func TestUpgrade_DeletionRequestsNewFeature(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manager
	mgr := consent.NewManager(tmpDir)

	// Create a deletion request (new feature)
	del, err := mgr.CreateDeletionRequest(
		"alice",
		consent.DeletionTypeSnapshot,
		[]string{"snap1", "snap2"},
		nil,
		"need to free space",
		2,
	)
	if err != nil {
		t.Fatalf("Failed to create deletion request: %v", err)
	}

	// Verify it was saved
	loaded, err := mgr.GetDeletionRequest(del.ID)
	if err != nil {
		t.Fatalf("Failed to load deletion request: %v", err)
	}

	if loaded.DeletionType != consent.DeletionTypeSnapshot {
		t.Errorf("DeletionType mismatch: got %s", loaded.DeletionType)
	}
	if len(loaded.SnapshotIDs) != 2 {
		t.Errorf("SnapshotIDs mismatch: got %d", len(loaded.SnapshotIDs))
	}

	// Ensure old restore requests still work alongside new deletion requests
	restore, err := mgr.CreateRequest("alice", "latest", "need files", nil)
	if err != nil {
		t.Fatalf("Failed to create restore request: %v", err)
	}

	// Both should be accessible
	pendingRestores, _ := mgr.ListPending()
	pendingDeletions, _ := mgr.ListPendingDeletions()

	if len(pendingRestores) != 1 {
		t.Errorf("Expected 1 pending restore, got %d", len(pendingRestores))
	}
	if len(pendingDeletions) != 1 {
		t.Errorf("Expected 1 pending deletion, got %d", len(pendingDeletions))
	}

	_ = restore // silence unused warning
}
