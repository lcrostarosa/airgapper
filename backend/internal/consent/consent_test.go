package consent

import (
	"os"
	"testing"
)

func TestRestoreRequest(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create request
	req, err := m.CreateRequest("alice", "latest", "need to restore files", nil)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	if req.ID == "" {
		t.Error("Request should have an ID")
	}
	if req.Status != StatusPending {
		t.Errorf("Expected status pending, got %s", req.Status)
	}

	// Get request
	got, err := m.GetRequest(req.ID)
	if err != nil {
		t.Fatalf("GetRequest failed: %v", err)
	}
	if got.Requester != "alice" {
		t.Errorf("Expected requester alice, got %s", got.Requester)
	}

	// List pending
	pending, err := m.ListPending()
	if err != nil {
		t.Fatalf("ListPending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending request, got %d", len(pending))
	}

	// Approve
	shareData := []byte("secret share")
	if err := m.Approve(req.ID, "bob", shareData); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	// Verify approved
	got, _ = m.GetRequest(req.ID)
	if got.Status != StatusApproved {
		t.Errorf("Expected status approved, got %s", got.Status)
	}
	if string(got.ShareData) != "secret share" {
		t.Error("Share data mismatch")
	}
}

func TestRestoreRequestDeny(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequest("alice", "latest", "need files", nil)

	if err := m.Deny(req.ID, "bob"); err != nil {
		t.Fatalf("Deny failed: %v", err)
	}

	got, _ := m.GetRequest(req.ID)
	if got.Status != StatusDenied {
		t.Errorf("Expected status denied, got %s", got.Status)
	}
}

func TestDeletionRequest(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create deletion request
	req, err := m.CreateDeletionRequest(
		"alice",
		DeletionTypeSnapshot,
		[]string{"snap1", "snap2"},
		nil,
		"need to free space",
		2, // Require 2 approvals
	)
	if err != nil {
		t.Fatalf("CreateDeletionRequest failed: %v", err)
	}

	if req.ID == "" {
		t.Error("Request should have an ID")
	}
	if req.Status != StatusPending {
		t.Errorf("Expected status pending, got %s", req.Status)
	}
	if req.DeletionType != DeletionTypeSnapshot {
		t.Errorf("Expected type snapshot, got %s", req.DeletionType)
	}

	// Get request
	got, err := m.GetDeletionRequest(req.ID)
	if err != nil {
		t.Fatalf("GetDeletionRequest failed: %v", err)
	}
	if len(got.SnapshotIDs) != 2 {
		t.Errorf("Expected 2 snapshot IDs, got %d", len(got.SnapshotIDs))
	}

	// List pending
	pending, err := m.ListPendingDeletions()
	if err != nil {
		t.Fatalf("ListPendingDeletions failed: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending deletion, got %d", len(pending))
	}

	// First approval - not enough yet
	sig1 := []byte("signature1")
	if err := m.ApproveDeletion(req.ID, "alice-key", "Alice", sig1); err != nil {
		t.Fatalf("First approval failed: %v", err)
	}

	current, required, _ := m.GetDeletionApprovalProgress(req.ID)
	if current != 1 || required != 2 {
		t.Errorf("Expected 1/2 approvals, got %d/%d", current, required)
	}

	got, _ = m.GetDeletionRequest(req.ID)
	if got.Status != StatusPending {
		t.Errorf("Should still be pending with 1/2 approvals, got %s", got.Status)
	}

	// Second approval - should approve
	sig2 := []byte("signature2")
	if err := m.ApproveDeletion(req.ID, "bob-key", "Bob", sig2); err != nil {
		t.Fatalf("Second approval failed: %v", err)
	}

	got, _ = m.GetDeletionRequest(req.ID)
	if got.Status != StatusApproved {
		t.Errorf("Should be approved with 2/2 approvals, got %s", got.Status)
	}
	if len(got.Approvals) != 2 {
		t.Errorf("Expected 2 approvals, got %d", len(got.Approvals))
	}
}

func TestDeletionRequestDeny(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateDeletionRequest(
		"alice",
		DeletionTypeSnapshot,
		[]string{"snap1"},
		nil,
		"free space",
		1,
	)

	if err := m.DenyDeletion(req.ID, "bob"); err != nil {
		t.Fatalf("DenyDeletion failed: %v", err)
	}

	got, _ := m.GetDeletionRequest(req.ID)
	if got.Status != StatusDenied {
		t.Errorf("Expected status denied, got %s", got.Status)
	}
}

func TestDeletionDuplicateApproval(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateDeletionRequest(
		"alice",
		DeletionTypeSnapshot,
		[]string{"snap1"},
		nil,
		"free space",
		2,
	)

	// First approval
	m.ApproveDeletion(req.ID, "alice-key", "Alice", []byte("sig"))

	// Duplicate approval should fail
	err := m.ApproveDeletion(req.ID, "alice-key", "Alice", []byte("sig2"))
	if err == nil {
		t.Error("Duplicate approval should fail")
	}
}

func TestMarkDeletionExecuted(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateDeletionRequest(
		"alice",
		DeletionTypeSnapshot,
		[]string{"snap1"},
		nil,
		"free space",
		1,
	)

	// Can't mark as executed before approval
	err := m.MarkDeletionExecuted(req.ID)
	if err == nil {
		t.Error("Should not be able to mark unapproved deletion as executed")
	}

	// Approve
	m.ApproveDeletion(req.ID, "alice-key", "Alice", []byte("sig"))

	// Now can mark as executed
	if err := m.MarkDeletionExecuted(req.ID); err != nil {
		t.Fatalf("MarkDeletionExecuted failed: %v", err)
	}

	got, _ := m.GetDeletionRequest(req.ID)
	if got.ExecutedAt == nil {
		t.Error("ExecutedAt should be set")
	}
}

func TestDeletionPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create with one manager
	m1 := NewManager(tmpDir)
	req, _ := m1.CreateDeletionRequest(
		"alice",
		DeletionTypeAll,
		nil,
		nil,
		"cleanup",
		1,
	)

	// Load with another manager
	m2 := NewManager(tmpDir)
	got, err := m2.GetDeletionRequest(req.ID)
	if err != nil {
		t.Fatalf("GetDeletionRequest failed: %v", err)
	}

	if got.DeletionType != DeletionTypeAll {
		t.Errorf("DeletionType mismatch: got %s", got.DeletionType)
	}
}

// Silence unused warning
var _ = os.TempDir
