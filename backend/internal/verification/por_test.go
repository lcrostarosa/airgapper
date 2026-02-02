package verification

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func TestPORManager_CreateChallenge(t *testing.T) {
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)

	files := []FileBlockInfo{
		{Path: "/data/file1", Size: 10240},
		{Path: "/data/file2", Size: 20480},
	}

	challenge, err := CreatePORChallenge(ownerPriv, ownerKeyID, files, 3, 4096, 60)
	if err != nil {
		t.Fatalf("failed to create challenge: %v", err)
	}

	if challenge.ID == "" {
		t.Error("challenge should have ID")
	}

	if challenge.OwnerSignature == "" {
		t.Error("challenge should be signed")
	}

	// Should have challenges for blocks
	if len(challenge.Challenges) == 0 {
		t.Error("challenge should have block challenges")
	}

	// Each challenge should have nonce
	for _, bc := range challenge.Challenges {
		if bc.Nonce == "" {
			t.Error("block challenge should have nonce")
		}
	}
}

func TestPORManager_ReceiveAndRespond(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "por-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	storageDir := filepath.Join(tempDir, "storage")
	os.MkdirAll(storageDir, 0755)

	testFile := filepath.Join(storageDir, "testfile")
	testContent := make([]byte, 16384) // 16KB
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}
	os.WriteFile(testFile, testContent, 0644)

	// Create keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	// Create manager
	config := DefaultPORConfig()
	pm, err := NewPORManager(tempDir, storageDir, config, hostPriv, hostPub, ownerPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create POR manager: %v", err)
	}

	// Create challenge
	files := []FileBlockInfo{
		{Path: "testfile", Size: 16384},
	}
	challenge, err := CreatePORChallenge(ownerPriv, ownerKeyID, files, 3, 4096, 60)
	if err != nil {
		t.Fatalf("failed to create challenge: %v", err)
	}

	// Receive challenge
	err = pm.ReceivePORChallenge(challenge)
	if err != nil {
		t.Fatalf("failed to receive challenge: %v", err)
	}

	// Respond to challenge
	response, err := pm.RespondToPORChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("failed to respond to challenge: %v", err)
	}

	if response.HostSignature == "" {
		t.Error("response should be signed")
	}

	if len(response.Proofs) == 0 {
		t.Error("response should have proofs")
	}

	// Check proofs
	for _, proof := range response.Proofs {
		if proof.Error != "" {
			t.Errorf("proof error: %s", proof.Error)
		}
		if proof.BlockHash == "" {
			t.Error("proof should have block hash")
		}
		if proof.CombinedHash == "" {
			t.Error("proof should have combined hash")
		}
	}
}

func TestPORManager_VerifyResponse(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "por-verify-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	storageDir := filepath.Join(tempDir, "storage")
	os.MkdirAll(storageDir, 0755)

	testFile := filepath.Join(storageDir, "testfile")
	testContent := make([]byte, 8192)
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}
	os.WriteFile(testFile, testContent, 0644)

	// Create keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	// Create manager
	config := DefaultPORConfig()
	pm, err := NewPORManager(tempDir, storageDir, config, hostPriv, hostPub, ownerPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create POR manager: %v", err)
	}

	// Create and receive challenge
	files := []FileBlockInfo{{Path: "testfile", Size: 8192}}
	challenge, _ := CreatePORChallenge(ownerPriv, ownerKeyID, files, 2, 4096, 60)
	pm.ReceivePORChallenge(challenge)

	// Respond
	response, _ := pm.RespondToPORChallenge(challenge.ID)

	// Verify without block verifier (just checks structure)
	result, err := VerifyPORResponse(challenge, response, hostPub, nil)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("verification should pass: %v", result.Errors)
	}

	if result.ValidProofs == 0 {
		t.Error("should have valid proofs")
	}
}

func TestPORManager_InvalidSignature(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "por-invalid-sig-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create keys
	ownerPub, _, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	otherPub, otherPriv, _ := crypto.GenerateKeyPair() // Different key
	otherKeyID := crypto.KeyID(otherPub)
	hostKeyID := crypto.KeyID(hostPub)

	// Create manager with owner's public key
	config := DefaultPORConfig()
	pm, err := NewPORManager(tempDir, "", config, hostPriv, hostPub, ownerPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create POR manager: %v", err)
	}

	// Create challenge signed with different key
	files := []FileBlockInfo{{Path: "testfile", Size: 8192}}
	challenge, _ := CreatePORChallenge(otherPriv, otherKeyID, files, 2, 4096, 60)

	// Should fail verification
	err = pm.ReceivePORChallenge(challenge)
	if err == nil {
		t.Error("should reject challenge with invalid signature")
	}
}

func TestPORManager_ExpiredChallenge(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "por-expired-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultPORConfig()
	pm, err := NewPORManager(tempDir, "", config, hostPriv, hostPub, ownerPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create POR manager: %v", err)
	}

	// Create challenge with 0 expiry (immediate expiration)
	files := []FileBlockInfo{{Path: "testfile", Size: 8192}}
	challenge, _ := CreatePORChallenge(ownerPriv, ownerKeyID, files, 2, 4096, -1)

	// Should fail due to expiration
	err = pm.ReceivePORChallenge(challenge)
	if err == nil {
		t.Error("should reject expired challenge")
	}
}

func TestPORManager_ListChallenges(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "por-list-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "storage")
	os.MkdirAll(storageDir, 0755)
	os.WriteFile(filepath.Join(storageDir, "testfile"), make([]byte, 4096), 0644)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultPORConfig()
	pm, err := NewPORManager(tempDir, storageDir, config, hostPriv, hostPub, ownerPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create POR manager: %v", err)
	}

	// Create multiple challenges
	files := []FileBlockInfo{{Path: "testfile", Size: 4096}}
	c1, _ := CreatePORChallenge(ownerPriv, ownerKeyID, files, 1, 4096, 60)
	c2, _ := CreatePORChallenge(ownerPriv, ownerKeyID, files, 1, 4096, 60)

	pm.ReceivePORChallenge(c1)
	pm.ReceivePORChallenge(c2)

	// Respond to one
	pm.RespondToPORChallenge(c1.ID)

	// List all
	all := pm.ListChallenges(false)
	if len(all) != 2 {
		t.Errorf("expected 2 challenges, got %d", len(all))
	}

	// List pending only
	pending := pm.ListChallenges(true)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending challenge, got %d", len(pending))
	}
}

func TestPORManager_GetStats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "por-stats-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "storage")
	os.MkdirAll(storageDir, 0755)
	os.WriteFile(filepath.Join(storageDir, "testfile"), make([]byte, 4096), 0644)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultPORConfig()
	pm, err := NewPORManager(tempDir, storageDir, config, hostPriv, hostPub, ownerPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create POR manager: %v", err)
	}

	// Create and respond to challenge
	files := []FileBlockInfo{{Path: "testfile", Size: 4096}}
	c, _ := CreatePORChallenge(ownerPriv, ownerKeyID, files, 1, 4096, 60)
	pm.ReceivePORChallenge(c)
	pm.RespondToPORChallenge(c.ID)

	stats := pm.GetStats()

	if stats["total_challenges"].(int) != 1 {
		t.Errorf("expected 1 total challenge, got %d", stats["total_challenges"].(int))
	}

	if stats["responded"].(int) != 1 {
		t.Errorf("expected 1 responded, got %d", stats["responded"].(int))
	}

	if stats["pending"].(int) != 0 {
		t.Errorf("expected 0 pending, got %d", stats["pending"].(int))
	}
}

func TestPORManager_MissingFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "por-missing-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "storage")
	os.MkdirAll(storageDir, 0755)
	// Don't create the test file

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultPORConfig()
	pm, err := NewPORManager(tempDir, storageDir, config, hostPriv, hostPub, ownerPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create POR manager: %v", err)
	}

	// Create challenge for non-existent file
	files := []FileBlockInfo{{Path: "nonexistent", Size: 4096}}
	c, _ := CreatePORChallenge(ownerPriv, ownerKeyID, files, 1, 4096, 60)
	pm.ReceivePORChallenge(c)

	// Respond should work but proofs should have errors
	response, err := pm.RespondToPORChallenge(c.ID)
	if err != nil {
		t.Fatalf("respond should not error: %v", err)
	}

	// Proofs should have error messages
	hasError := false
	for _, proof := range response.Proofs {
		if proof.Error != "" {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("proofs for missing file should have errors")
	}
}

func TestGenerateRandomIndices(t *testing.T) {
	indices := generateRandomIndices(100, 10)

	if len(indices) != 10 {
		t.Errorf("expected 10 indices, got %d", len(indices))
	}

	// Check all indices are within range
	for _, idx := range indices {
		if idx < 0 || idx >= 100 {
			t.Errorf("index %d out of range [0, 100)", idx)
		}
	}

	// Check no duplicates
	seen := make(map[int64]bool)
	for _, idx := range indices {
		if seen[idx] {
			t.Errorf("duplicate index %d", idx)
		}
		seen[idx] = true
	}
}

func TestGenerateRandomIndices_MoreThanMax(t *testing.T) {
	// Request more indices than available
	indices := generateRandomIndices(5, 10)

	if len(indices) != 5 {
		t.Errorf("expected 5 indices (max available), got %d", len(indices))
	}
}
