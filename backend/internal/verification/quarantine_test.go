package verification

import (
	"os"
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func TestRequestDeletion(t *testing.T) {
	pubKey, privKey, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(pubKey)

	qd, err := RequestDeletion(privKey, ownerKeyID, []string{"/repo/snapshots/abc"}, "cleanup old data", 72)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if qd.ID == "" {
		t.Error("request ID should not be empty")
	}

	if qd.Status != QuarantinePending {
		t.Errorf("expected status pending, got %s", qd.Status)
	}

	if qd.OwnerSignature == "" {
		t.Error("request should be signed")
	}

	// Check delay is approximately 72 hours
	expectedDelay := time.Duration(72) * time.Hour
	actualDelay := qd.ExecutableAt.Sub(qd.RequestedAt)
	if actualDelay < expectedDelay-time.Minute || actualDelay > expectedDelay+time.Minute {
		t.Errorf("unexpected delay: got %v, expected ~%v", actualDelay, expectedDelay)
	}
}

func TestQuarantineManager_RegisterDeletion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "quarantine-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultQuarantineConfig()
	qm, err := NewQuarantineManager(tempDir, config, ownerPub, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create and register a deletion request
	qd, _ := RequestDeletion(ownerPriv, ownerKeyID, []string{"/repo/data/file1"}, "test", 48)

	err = qm.RegisterDeletion(qd)
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Verify host signature was added
	if qd.HostSignature == "" {
		t.Error("host should have signed acknowledgment")
	}

	// Verify it's stored
	stored := qm.Get(qd.ID)
	if stored == nil {
		t.Fatal("deletion should be stored")
	}
}

func TestQuarantineManager_EnforceMinDelay(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "quarantine-min-delay-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultQuarantineConfig()
	config.MinDelayHours = 24 // Minimum 24 hours

	qm, _ := NewQuarantineManager(tempDir, config, ownerPub, hostPriv, hostPub, hostKeyID)

	// Try to create with only 1 hour delay
	qd, _ := RequestDeletion(ownerPriv, ownerKeyID, []string{"/repo/data/file1"}, "test", 1)

	err = qm.RegisterDeletion(qd)
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Executable time should be enforced to minimum
	minExpected := time.Now().Add(time.Duration(config.MinDelayHours) * time.Hour)
	if qd.ExecutableAt.Before(minExpected.Add(-time.Minute)) {
		t.Errorf("delay should be enforced to minimum %d hours", config.MinDelayHours)
	}
}

func TestQuarantineManager_GetReadyDeletions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "quarantine-ready-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultQuarantineConfig()
	config.MinDelayHours = 0 // Disable minimum for test

	qm, _ := NewQuarantineManager(tempDir, config, ownerPub, hostPriv, hostPub, hostKeyID)

	// Create a deletion request
	qd, _ := RequestDeletion(ownerPriv, ownerKeyID, []string{"/repo/data/file1"}, "test", 0)
	qm.RegisterDeletion(qd)

	// After registration, directly set the ExecutableAt to the past
	// This simulates a deletion that has passed its quarantine period
	stored := qm.Get(qd.ID)
	if stored == nil {
		t.Fatal("deletion should be stored")
	}
	stored.ExecutableAt = time.Now().Add(-time.Hour) // Set to past after registration

	ready := qm.GetReadyDeletions()
	if len(ready) != 1 {
		t.Errorf("expected 1 ready deletion, got %d", len(ready))
	}

	if len(ready) > 0 && ready[0].Status != QuarantineApproved {
		t.Errorf("status should be approved, got %s", ready[0].Status)
	}
}

func TestQuarantineManager_MarkExecuted(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "quarantine-execute-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	ownerKeyID := crypto.KeyID(ownerPub)
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultQuarantineConfig()
	config.MinDelayHours = 0

	qm, _ := NewQuarantineManager(tempDir, config, ownerPub, hostPriv, hostPub, hostKeyID)

	qd, _ := RequestDeletion(ownerPriv, ownerKeyID, []string{"/repo/data/file1"}, "test", 0)
	qm.RegisterDeletion(qd)

	// After registration, directly set the ExecutableAt to the past
	stored := qm.Get(qd.ID)
	if stored == nil {
		t.Fatal("deletion should be stored")
	}
	stored.ExecutableAt = time.Now().Add(-time.Hour)

	// Get ready and mark executed
	ready := qm.GetReadyDeletions()
	if len(ready) == 0 {
		t.Fatal("expected at least 1 ready deletion")
	}

	err = qm.MarkExecuted(ready[0].ID)
	if err != nil {
		t.Fatalf("failed to mark executed: %v", err)
	}

	stored = qm.Get(ready[0].ID)
	if stored.Status != QuarantineExecuted {
		t.Errorf("status should be executed, got %s", stored.Status)
	}

	if stored.ExecutedAt == nil {
		t.Error("executed_at should be set")
	}
}

func TestQuarantinedDeletion_TimeUntilExecutable(t *testing.T) {
	qd := &QuarantinedDeletion{
		Status:       QuarantinePending,
		ExecutableAt: time.Now().Add(2 * time.Hour),
	}

	remaining := qd.TimeUntilExecutable()
	if remaining < time.Hour*2-time.Minute || remaining > time.Hour*2+time.Minute {
		t.Errorf("expected ~2 hours remaining, got %v", remaining)
	}

	// Test executed status
	qd.Status = QuarantineExecuted
	remaining = qd.TimeUntilExecutable()
	if remaining != 0 {
		t.Errorf("executed deletion should have 0 remaining, got %v", remaining)
	}
}
