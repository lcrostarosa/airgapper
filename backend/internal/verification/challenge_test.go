package verification

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func TestCreateChallenge(t *testing.T) {
	// Generate owner keys
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}
	ownerKeyID := crypto.KeyID(pubKey)

	// Create challenge
	requests := []FileChallenge{
		{Path: "/repo/data/abc123", ExpectedHash: "deadbeef"},
		{Path: "/repo/snapshots/snap1"},
	}

	challenge, err := CreateChallenge(privKey, ownerKeyID, requests, 60)
	if err != nil {
		t.Fatalf("failed to create challenge: %v", err)
	}

	if challenge.ID == "" {
		t.Error("challenge ID should not be empty")
	}

	if challenge.OwnerKeyID != ownerKeyID {
		t.Errorf("owner key ID mismatch: expected %s, got %s", ownerKeyID, challenge.OwnerKeyID)
	}

	if challenge.OwnerSignature == "" {
		t.Error("challenge should be signed")
	}

	if len(challenge.Requests) != 2 {
		t.Errorf("expected 2 requests, got %d", len(challenge.Requests))
	}

	if challenge.ExpiresAt.IsZero() {
		t.Error("challenge should have expiry set")
	}
}

func TestChallengeManager_ReceiveAndRespond(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "challenge-manager-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files in storage
	storageDir := filepath.Join(tempDir, "storage")
	os.MkdirAll(filepath.Join(storageDir, "repo", "data"), 0755)

	testFile := filepath.Join(storageDir, "repo", "data", "testfile")
	testContent := []byte("test content for hashing")
	os.WriteFile(testFile, testContent, 0644)

	// Generate keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	// Create challenge manager
	cm, err := NewChallengeManager(
		filepath.Join(tempDir, "challenges"),
		storageDir,
		hostPriv, hostPub, ownerPub,
		hostKeyID, 60,
	)
	if err != nil {
		t.Fatalf("failed to create challenge manager: %v", err)
	}

	// Create a challenge
	requests := []FileChallenge{
		{Path: "repo/data/testfile"},
		{Path: "repo/data/nonexistent"},
	}

	challenge, err := CreateChallenge(ownerPriv, ownerKeyID, requests, 60)
	if err != nil {
		t.Fatalf("failed to create challenge: %v", err)
	}

	// Receive challenge
	err = cm.ReceiveChallenge(challenge)
	if err != nil {
		t.Fatalf("failed to receive challenge: %v", err)
	}

	// Respond to challenge
	response, err := cm.RespondToChallenge(challenge.ID)
	if err != nil {
		t.Fatalf("failed to respond to challenge: %v", err)
	}

	if response.ChallengeID != challenge.ID {
		t.Error("response should reference challenge")
	}

	if len(response.Proofs) != 2 {
		t.Errorf("expected 2 proofs, got %d", len(response.Proofs))
	}

	// Check proofs
	var existingProof, missingProof *FileProof
	for i := range response.Proofs {
		if response.Proofs[i].Path == "repo/data/testfile" {
			existingProof = &response.Proofs[i]
		} else if response.Proofs[i].Path == "repo/data/nonexistent" {
			missingProof = &response.Proofs[i]
		}
	}

	if existingProof == nil {
		t.Fatal("missing proof for existing file")
	}
	if !existingProof.Exists {
		t.Error("existing file should be marked as exists")
	}
	if existingProof.Hash == "" {
		t.Error("existing file should have hash")
	}
	if existingProof.Size == 0 {
		t.Error("existing file should have size")
	}

	if missingProof == nil {
		t.Fatal("missing proof for nonexistent file")
	}
	if missingProof.Exists {
		t.Error("nonexistent file should be marked as not exists")
	}
}

func TestVerifyResponse(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "verify-response-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	storageDir := filepath.Join(tempDir, "storage")
	os.MkdirAll(filepath.Join(storageDir, "data"), 0755)
	testFile := filepath.Join(storageDir, "data", "file1")
	os.WriteFile(testFile, []byte("content"), 0644)

	// Generate keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	// Create challenge manager
	cm, err := NewChallengeManager(
		filepath.Join(tempDir, "challenges"),
		storageDir,
		hostPriv, hostPub, ownerPub,
		hostKeyID, 60,
	)
	if err != nil {
		t.Fatalf("failed to create challenge manager: %v", err)
	}

	// Create challenge with expected hash
	requests := []FileChallenge{
		{Path: "data/file1", ExpectedHash: "ed7002b439e9ac845f22357d822bac1444730fbdb6016d3ec9432297b9ec9f73"}, // SHA256 of "content"
	}

	challenge, _ := CreateChallenge(ownerPriv, ownerKeyID, requests, 60)
	cm.ReceiveChallenge(challenge)
	response, _ := cm.RespondToChallenge(challenge.ID)

	// Verify response
	result, err := VerifyResponse(challenge, response, hostPub)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid result, got errors: %v", result.Errors)
	}

	if result.TotalFiles != 1 {
		t.Errorf("expected 1 total file, got %d", result.TotalFiles)
	}

	if result.ExistingFiles != 1 {
		t.Errorf("expected 1 existing file, got %d", result.ExistingFiles)
	}

	if result.HashMatches != 1 {
		t.Errorf("expected 1 hash match, got %d", result.HashMatches)
	}
}

func TestVerifyResponse_HashMismatch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "verify-mismatch-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	storageDir := filepath.Join(tempDir, "storage")
	os.MkdirAll(filepath.Join(storageDir, "data"), 0755)
	testFile := filepath.Join(storageDir, "data", "file1")
	os.WriteFile(testFile, []byte("actual content"), 0644)

	// Generate keys
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	// Create challenge manager
	cm, err := NewChallengeManager(
		filepath.Join(tempDir, "challenges"),
		storageDir,
		hostPriv, hostPub, ownerPub,
		hostKeyID, 60,
	)
	if err != nil {
		t.Fatalf("failed to create challenge manager: %v", err)
	}

	// Create challenge with wrong expected hash
	requests := []FileChallenge{
		{Path: "data/file1", ExpectedHash: "wronghash123456789"},
	}

	challenge, _ := CreateChallenge(ownerPriv, ownerKeyID, requests, 60)
	cm.ReceiveChallenge(challenge)
	response, _ := cm.RespondToChallenge(challenge.ID)

	// Verify response
	result, err := VerifyResponse(challenge, response, hostPub)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result due to hash mismatch")
	}

	if result.HashMismatches != 1 {
		t.Errorf("expected 1 hash mismatch, got %d", result.HashMismatches)
	}

	if len(result.MismatchPaths) != 1 {
		t.Errorf("expected 1 mismatch path, got %d", len(result.MismatchPaths))
	}
}

func TestChallengeManager_InvalidSignature(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "challenge-invalid-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, _, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	// Use wrong key to sign
	_, wrongPriv, _ := crypto.GenerateKeyPair()
	wrongKeyID := "wrong"

	cm, err := NewChallengeManager(
		filepath.Join(tempDir, "challenges"),
		tempDir,
		hostPriv, hostPub, ownerPub,
		hostKeyID, 60,
	)
	if err != nil {
		t.Fatalf("failed to create challenge manager: %v", err)
	}

	// Create challenge with wrong signature
	requests := []FileChallenge{{Path: "test"}}
	challenge, _ := CreateChallenge(wrongPriv, wrongKeyID, requests, 60)

	// Should reject due to invalid signature
	err = cm.ReceiveChallenge(challenge)
	if err == nil {
		t.Error("should reject challenge with invalid signature")
	}
}

func TestChallengeManager_ListChallenges(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "challenge-list-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	cm, err := NewChallengeManager(
		filepath.Join(tempDir, "challenges"),
		tempDir,
		hostPriv, hostPub, ownerPub,
		hostKeyID, 60,
	)
	if err != nil {
		t.Fatalf("failed to create challenge manager: %v", err)
	}

	// Create and receive multiple challenges
	for i := 0; i < 3; i++ {
		requests := []FileChallenge{{Path: "test"}}
		challenge, _ := CreateChallenge(ownerPriv, ownerKeyID, requests, 60)
		cm.ReceiveChallenge(challenge)
	}

	// List all challenges
	all := cm.ListChallenges(false)
	if len(all) != 3 {
		t.Errorf("expected 3 challenges, got %d", len(all))
	}

	// Respond to one
	cm.RespondToChallenge(all[0].ID)

	// List pending only
	pending := cm.ListChallenges(true)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending challenges, got %d", len(pending))
	}
}
