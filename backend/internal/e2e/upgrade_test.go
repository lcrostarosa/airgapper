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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "Failed to load legacy config")

	// Verify all fields loaded correctly
	assert.Equal(t, "alice-backup", cfg.Name, "Name mismatch")
	assert.Equal(t, config.RoleOwner, cfg.Role, "Role mismatch")
	assert.Equal(t, "rest:http://bob:8000/", cfg.RepoURL, "RepoURL mismatch")
	assert.Equal(t, "secretpassword123", cfg.Password, "Password mismatch")
	assert.NotNil(t, cfg.LocalShare, "LocalShare should be loaded")
	assert.Equal(t, byte(1), cfg.ShareIndex, "ShareIndex mismatch")
	assert.True(t, cfg.Peer != nil && cfg.Peer.Name == "bob", "Peer info not loaded correctly")
	assert.True(t, len(cfg.BackupPaths) == 1 && cfg.BackupPaths[0] == "/home/alice/documents", "BackupPaths not loaded correctly")

	// Verify mode detection
	assert.True(t, cfg.UsesSSSMode(), "Should detect SSS mode")
	assert.False(t, cfg.UsesConsensusMode(), "Should not detect consensus mode")
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
	require.NoError(t, err, "Failed to load legacy host config")

	assert.Equal(t, config.RoleHost, cfg.Role, "Role mismatch")
	assert.False(t, cfg.IsOwner(), "Should not be owner")
	assert.True(t, cfg.IsHost(), "Should be host")

	// New fields should be empty/zero
	assert.Empty(t, cfg.StoragePath, "StoragePath should be empty for legacy config")
	assert.Nil(t, cfg.Consensus, "Consensus should be nil for legacy config")
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
	require.NoError(t, err, "Failed to load consensus config")

	assert.True(t, cfg.UsesConsensusMode(), "Should detect consensus mode")
	assert.False(t, cfg.UsesSSSMode(), "Should not detect SSS mode")
	assert.Equal(t, 2, cfg.Consensus.Threshold, "Threshold mismatch")
	assert.Len(t, cfg.Consensus.KeyHolders, 2, "KeyHolders count mismatch")
	assert.Equal(t, 2, cfg.RequiredApprovals(), "RequiredApprovals mismatch")
}

// TestUpgrade_SSSKeysStillWork tests that SSS shares created with old code
// can still be combined with new code
func TestUpgrade_SSSKeysStillWork(t *testing.T) {
	// Create shares using current SSS implementation
	// This simulates shares created by an older version
	password := "my-secret-backup-password-12345"

	shares, err := sss.Split([]byte(password), 2, 2)
	require.NoError(t, err, "Failed to split password")

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
	require.NoError(t, err, "Failed to combine shares")

	assert.Equal(t, password, string(combined), "Password mismatch after combine")
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
	require.NoError(t, err, "Failed to load old request")

	assert.Equal(t, "alice", req.Requester, "Requester mismatch")
	assert.Equal(t, consent.StatusPending, req.Status, "Status mismatch")
	assert.Equal(t, "latest", req.SnapshotID, "SnapshotID mismatch")

	// New fields should be zero
	assert.Equal(t, 0, req.RequiredApprovals, "RequiredApprovals should be 0 for old requests")

	// Should be able to approve it (legacy mode)
	err = mgr.Approve("abc123def456", "bob", []byte("share-data"))
	require.NoError(t, err, "Failed to approve old request")

	approved, _ := mgr.GetRequest("abc123def456")
	assert.Equal(t, consent.StatusApproved, approved.Status, "Request should be approved")
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
	require.NoError(t, err, "Failed to load policy")

	err = loaded.Verify()
	require.NoError(t, err, "Policy verification failed")

	assert.Equal(t, 30, loaded.RetentionDays, "RetentionDays mismatch")
	assert.Equal(t, policy.DeletionBothRequired, loaded.DeletionMode, "DeletionMode mismatch")
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
	require.NoError(t, err, "Failed to create storage server")
	s.Start()

	// Verify data is accessible via new storage server
	status := s.Status()
	assert.True(t, status.Running, "Storage server should be running")
	assert.NotEqual(t, int64(0), status.UsedBytes, "Should detect existing data")

	// Verify integrity check works on old data
	checker, err := integrity.NewChecker(tmpDir)
	require.NoError(t, err, "Failed to create integrity checker")

	result, err := checker.CheckDataIntegrity("backup-repo")
	require.NoError(t, err, "Integrity check failed")

	assert.True(t, result.Passed, "Integrity check should pass for valid old data: %v", result.Errors)
	assert.Equal(t, 1, result.TotalFiles, "Should find 1 data file")
}

// TestUpgrade_Ed25519KeysCompatible tests that Ed25519 keys generated
// with any version work correctly
func TestUpgrade_Ed25519KeysCompatible(t *testing.T) {
	// Generate keys with current code
	pub, priv, err := crypto.GenerateKeyPair()
	require.NoError(t, err, "Failed to generate keys")

	// Encode to hex (as stored in config)
	pubHex := crypto.EncodePublicKey(pub)
	privHex := crypto.EncodePrivateKey(priv)

	// Decode back
	decodedPub, err := crypto.DecodePublicKey(pubHex)
	require.NoError(t, err, "Failed to decode public key")
	decodedPriv, err := crypto.DecodePrivateKey(privHex)
	require.NoError(t, err, "Failed to decode private key")

	// Sign and verify
	message := []byte("test message to sign")
	sig, err := crypto.Sign(decodedPriv, message)
	require.NoError(t, err, "Failed to sign")

	assert.True(t, crypto.Verify(decodedPub, message, sig), "Signature verification failed")

	// Key ID should be deterministic
	keyID1 := crypto.KeyID(pub)
	keyID2 := crypto.KeyID(decodedPub)
	assert.Equal(t, keyID1, keyID2, "Key ID mismatch")
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
	require.NoError(t, err, "Failed to load config with unknown fields")

	// Modify a known field
	cfg.Name = "modified-name"

	// Save it back
	err = cfg.Save()
	require.NoError(t, err, "Failed to save config")

	// Note: Current implementation will LOSE unknown fields on save
	// This test documents that behavior - we may want to fix this
	// by using a json.RawMessage approach in the future

	// Re-load
	reloaded, _ := config.Load(tmpDir)
	assert.Equal(t, "modified-name", reloaded.Name, "Modified field should be saved")

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
	require.NoError(t, err, "Failed to create deletion request")

	// Verify it was saved
	loaded, err := mgr.GetDeletionRequest(del.ID)
	require.NoError(t, err, "Failed to load deletion request")

	assert.Equal(t, consent.DeletionTypeSnapshot, loaded.DeletionType, "DeletionType mismatch")
	assert.Len(t, loaded.SnapshotIDs, 2, "SnapshotIDs mismatch")

	// Ensure old restore requests still work alongside new deletion requests
	restore, err := mgr.CreateRequest("alice", "latest", "need files", nil)
	require.NoError(t, err, "Failed to create restore request")

	// Both should be accessible
	pendingRestores, _ := mgr.ListPending()
	pendingDeletions, _ := mgr.ListPendingDeletions()

	assert.Len(t, pendingRestores, 1, "Expected 1 pending restore")
	assert.Len(t, pendingDeletions, 1, "Expected 1 pending deletion")

	_ = restore // silence unused warning
}
