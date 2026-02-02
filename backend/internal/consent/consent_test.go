package consent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestoreRequest(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create request
	req, err := m.CreateRequest("alice", "latest", "need to restore files", nil)
	require.NoError(t, err, "CreateRequest failed")

	assert.NotEmpty(t, req.ID, "Request should have an ID")
	assert.Equal(t, StatusPending, req.Status)

	// Get request
	got, err := m.GetRequest(req.ID)
	require.NoError(t, err, "GetRequest failed")
	assert.Equal(t, "alice", got.Requester)

	// List pending
	pending, err := m.ListPending()
	require.NoError(t, err, "ListPending failed")
	assert.Len(t, pending, 1)

	// Approve
	shareData := []byte("secret share")
	require.NoError(t, m.Approve(req.ID, "bob", shareData), "Approve failed")

	// Verify approved
	got, _ = m.GetRequest(req.ID)
	assert.Equal(t, StatusApproved, got.Status)
	assert.Equal(t, "secret share", string(got.ShareData))
}

func TestRestoreRequestDeny(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequest("alice", "latest", "need files", nil)

	require.NoError(t, m.Deny(req.ID, "bob"), "Deny failed")

	got, _ := m.GetRequest(req.ID)
	assert.Equal(t, StatusDenied, got.Status)
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
	require.NoError(t, err, "CreateDeletionRequest failed")

	assert.NotEmpty(t, req.ID)
	assert.Equal(t, StatusPending, req.Status)
	assert.Equal(t, DeletionTypeSnapshot, req.DeletionType)

	// Get request
	got, err := m.GetDeletionRequest(req.ID)
	require.NoError(t, err, "GetDeletionRequest failed")
	assert.Len(t, got.SnapshotIDs, 2)

	// List pending
	pending, err := m.ListPendingDeletions()
	require.NoError(t, err, "ListPendingDeletions failed")
	assert.Len(t, pending, 1)

	// First approval - not enough yet
	sig1 := []byte("signature1")
	require.NoError(t, m.ApproveDeletion(req.ID, "alice-key", "Alice", sig1), "First approval failed")

	current, required, _ := m.GetDeletionApprovalProgress(req.ID)
	assert.Equal(t, 1, current)
	assert.Equal(t, 2, required)

	got, _ = m.GetDeletionRequest(req.ID)
	assert.Equal(t, StatusPending, got.Status, "Should still be pending with 1/2 approvals")

	// Second approval - should approve
	sig2 := []byte("signature2")
	require.NoError(t, m.ApproveDeletion(req.ID, "bob-key", "Bob", sig2), "Second approval failed")

	got, _ = m.GetDeletionRequest(req.ID)
	assert.Equal(t, StatusApproved, got.Status, "Should be approved with 2/2 approvals")
	assert.Len(t, got.Approvals, 2)
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

	require.NoError(t, m.DenyDeletion(req.ID, "bob"), "DenyDeletion failed")

	got, _ := m.GetDeletionRequest(req.ID)
	assert.Equal(t, StatusDenied, got.Status)
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
	assert.Error(t, err, "Duplicate approval should fail")
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
	assert.Error(t, err, "Should not be able to mark unapproved deletion as executed")

	// Approve
	m.ApproveDeletion(req.ID, "alice-key", "Alice", []byte("sig"))

	// Now can mark as executed
	require.NoError(t, m.MarkDeletionExecuted(req.ID), "MarkDeletionExecuted failed")

	got, _ := m.GetDeletionRequest(req.ID)
	assert.NotNil(t, got.ExecutedAt, "ExecutedAt should be set")
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
	require.NoError(t, err, "GetDeletionRequest failed")
	assert.Equal(t, DeletionTypeAll, got.DeletionType)
}
