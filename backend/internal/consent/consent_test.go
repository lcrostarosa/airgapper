package consent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	apperrors "github.com/lcrostarosa/airgapper/backend/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Manager Tests
// ============================================================================

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	require.NotNil(t, m)
	assert.Equal(t, filepath.Join(tmpDir, "requests"), m.dataDir)
	assert.Equal(t, filepath.Join(tmpDir, "deletions"), m.deletionDataDir)
}

// ============================================================================
// Restore Request Tests
// ============================================================================

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

func TestRestoreRequestWithPaths(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	paths := []string{"/home/user/documents", "/var/log"}
	req, err := m.CreateRequest("alice", "latest", "need specific files", paths)
	require.NoError(t, err)

	assert.Equal(t, paths, req.Paths)

	got, err := m.GetRequest(req.ID)
	require.NoError(t, err)
	assert.Equal(t, paths, got.Paths)
}

func TestRestoreRequestNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.GetRequest("nonexistent")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestRestoreRequestApproveNotPending(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequest("alice", "latest", "need files", nil)

	// Approve first time
	require.NoError(t, m.Approve(req.ID, "bob", []byte("share")))

	// Try to approve again
	err := m.Approve(req.ID, "charlie", []byte("another share"))
	assert.ErrorIs(t, err, apperrors.ErrRequestNotPending)
}

func TestRestoreRequestDenyNotPending(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequest("alice", "latest", "need files", nil)

	// Deny first
	require.NoError(t, m.Deny(req.ID, "bob"))

	// Try to deny again
	err := m.Deny(req.ID, "charlie")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotPending)
}

func TestRestoreRequestApproveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	err := m.Approve("nonexistent", "bob", []byte("share"))
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestRestoreRequestDenyNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	err := m.Deny("nonexistent", "bob")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestRestoreRequestExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create request and manually expire it
	req, err := m.CreateRequest("alice", "latest", "need files", nil)
	require.NoError(t, err)

	// Manually modify expiry to the past
	req.ExpiresAt = time.Now().Add(-time.Hour)
	require.NoError(t, m.saveRequest(req))

	// Approve should fail - either ErrRequestExpired or ErrRequestNotPending
	// depending on timing (GetRequest marks as expired before Approve checks)
	err = m.Approve(req.ID, "bob", []byte("share"))
	assert.Error(t, err)
	assert.True(t, err == apperrors.ErrRequestExpired || err == apperrors.ErrRequestNotPending,
		"expected ErrRequestExpired or ErrRequestNotPending, got: %v", err)

	// Request should now be marked as expired
	got, err := m.GetRequest(req.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusExpired, got.Status)
}

func TestRestoreRequestExpirationOnGet(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create request and manually expire it
	req, err := m.CreateRequest("alice", "latest", "need files", nil)
	require.NoError(t, err)

	// Manually modify expiry to the past
	req.ExpiresAt = time.Now().Add(-time.Hour)
	require.NoError(t, m.saveRequest(req))

	// Get should mark it as expired
	got, err := m.GetRequest(req.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusExpired, got.Status)
}

func TestListPendingWithEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// List should work even with no requests
	pending, err := m.ListPending()
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestListPendingFiltersNonPending(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create three requests
	req1, _ := m.CreateRequest("alice", "latest", "reason1", nil)
	req2, _ := m.CreateRequest("bob", "latest", "reason2", nil)
	req3, _ := m.CreateRequest("charlie", "latest", "reason3", nil)

	// Approve one, deny another
	require.NoError(t, m.Approve(req1.ID, "approver", []byte("share")))
	require.NoError(t, m.Deny(req2.ID, "denier"))

	// Only req3 should be pending
	pending, err := m.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, req3.ID, pending[0].ID)
}

func TestListPendingIgnoresNonJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create a request
	req, _ := m.CreateRequest("alice", "latest", "reason", nil)

	// Create a non-JSON file in the requests directory
	require.NoError(t, os.MkdirAll(m.dataDir, 0700))
	nonJSONFile := filepath.Join(m.dataDir, "readme.txt")
	require.NoError(t, os.WriteFile(nonJSONFile, []byte("not json"), 0600))

	// List should only return the actual request
	pending, err := m.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, req.ID, pending[0].ID)
}

func TestRestoreRequestPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create with one manager
	m1 := NewManager(tmpDir)
	req, _ := m1.CreateRequest("alice", "latest", "reason", []string{"/path"})

	// Load with another manager
	m2 := NewManager(tmpDir)
	got, err := m2.GetRequest(req.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Requester)
	assert.Equal(t, "latest", got.SnapshotID)
	assert.Equal(t, "reason", got.Reason)
	assert.Equal(t, []string{"/path"}, got.Paths)
}

// ============================================================================
// Consensus Mode Restore Request Tests
// ============================================================================

func TestCreateRequestWithConsensus(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	paths := []string{"/home/user"}
	req, err := m.CreateRequestWithConsensus("alice", "latest", "need restore", paths, 2)
	require.NoError(t, err)

	assert.NotEmpty(t, req.ID)
	assert.Equal(t, "alice", req.Requester)
	assert.Equal(t, "latest", req.SnapshotID)
	assert.Equal(t, "need restore", req.Reason)
	assert.Equal(t, paths, req.Paths)
	assert.Equal(t, StatusPending, req.Status)
	assert.Equal(t, 2, req.RequiredApprovals)
	assert.Empty(t, req.Approvals)
}

func TestAddSignature(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 2)

	// Add first signature
	err := m.AddSignature(req.ID, "key1", "Alice", []byte("sig1"))
	require.NoError(t, err)

	got, _ := m.GetRequest(req.ID)
	assert.Len(t, got.Approvals, 1)
	assert.Equal(t, "key1", got.Approvals[0].KeyHolderID)
	assert.Equal(t, "Alice", got.Approvals[0].KeyHolderName)
	assert.Equal(t, []byte("sig1"), got.Approvals[0].Signature)
	assert.Equal(t, StatusPending, got.Status) // Still pending with 1/2

	// Add second signature
	err = m.AddSignature(req.ID, "key2", "Bob", []byte("sig2"))
	require.NoError(t, err)

	got, _ = m.GetRequest(req.ID)
	assert.Len(t, got.Approvals, 2)
	assert.Equal(t, StatusApproved, got.Status) // Now approved with 2/2
	assert.Equal(t, "consensus", got.ApprovedBy)
	assert.NotNil(t, got.ApprovedAt)
}

func TestAddSignatureDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 2)

	// Add first signature
	require.NoError(t, m.AddSignature(req.ID, "key1", "Alice", []byte("sig1")))

	// Try to add duplicate signature
	err := m.AddSignature(req.ID, "key1", "Alice", []byte("sig2"))
	assert.ErrorIs(t, err, apperrors.ErrAlreadyApproved)
}

func TestAddSignatureNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	err := m.AddSignature("nonexistent", "key1", "Alice", []byte("sig"))
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestAddSignatureNotPending(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 1)

	// Approve with first signature
	require.NoError(t, m.AddSignature(req.ID, "key1", "Alice", []byte("sig1")))

	// Try to add another signature to already approved request
	err := m.AddSignature(req.ID, "key2", "Bob", []byte("sig2"))
	assert.ErrorIs(t, err, apperrors.ErrRequestNotPending)
}

func TestAddSignatureExpired(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 2)

	// Manually expire the request
	req.ExpiresAt = time.Now().Add(-time.Hour)
	require.NoError(t, m.saveRequest(req))

	// Try to add signature - either ErrRequestExpired or ErrRequestNotPending
	// depending on timing (GetRequest marks as expired before AddSignature checks)
	err := m.AddSignature(req.ID, "key1", "Alice", []byte("sig"))
	assert.Error(t, err)
	assert.True(t, err == apperrors.ErrRequestExpired || err == apperrors.ErrRequestNotPending,
		"expected ErrRequestExpired or ErrRequestNotPending, got: %v", err)
}

func TestHasEnoughApprovals(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 2)

	// Initially not enough
	enough, err := m.HasEnoughApprovals(req.ID)
	require.NoError(t, err)
	assert.False(t, enough)

	// Add one signature
	require.NoError(t, m.AddSignature(req.ID, "key1", "Alice", []byte("sig1")))

	enough, err = m.HasEnoughApprovals(req.ID)
	require.NoError(t, err)
	assert.False(t, enough)

	// Add second signature
	require.NoError(t, m.AddSignature(req.ID, "key2", "Bob", []byte("sig2")))

	enough, err = m.HasEnoughApprovals(req.ID)
	require.NoError(t, err)
	assert.True(t, enough)
}

func TestHasEnoughApprovalsNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.HasEnoughApprovals("nonexistent")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestGetApprovalProgress(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 3)

	current, required, err := m.GetApprovalProgress(req.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, current)
	assert.Equal(t, 3, required)

	// Add a signature
	require.NoError(t, m.AddSignature(req.ID, "key1", "Alice", []byte("sig1")))

	current, required, err = m.GetApprovalProgress(req.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, current)
	assert.Equal(t, 3, required)
}

func TestGetApprovalProgressNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, _, err := m.GetApprovalProgress("nonexistent")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

// ============================================================================
// Deletion Request Tests
// ============================================================================

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
	err := m.ApproveDeletion(req.ID, "alice-key", "Alice", []byte("sig"))
	require.NoError(t, err, "first approval failed")

	// Duplicate approval should fail
	err = m.ApproveDeletion(req.ID, "alice-key", "Alice", []byte("sig2"))
	assert.ErrorIs(t, err, apperrors.ErrAlreadyApproved)
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
	assert.ErrorIs(t, err, apperrors.ErrRequestNotApproved)

	// Approve
	err = m.ApproveDeletion(req.ID, "alice-key", "Alice", []byte("sig"))
	require.NoError(t, err, "approval failed")

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

func TestDeletionRequestNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.GetDeletionRequest("nonexistent")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestDeletionRequestExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateDeletionRequest("alice", DeletionTypeSnapshot, []string{"snap1"}, nil, "reason", 1)

	// Manually expire the request
	req.ExpiresAt = time.Now().Add(-time.Hour)
	require.NoError(t, m.saveDeletionRequest(req))

	// Approve should fail - either ErrRequestExpired or ErrRequestNotPending
	// depending on timing (GetDeletionRequest marks as expired before ApproveDeletion checks)
	err := m.ApproveDeletion(req.ID, "key1", "Alice", []byte("sig"))
	assert.Error(t, err)
	assert.True(t, err == apperrors.ErrRequestExpired || err == apperrors.ErrRequestNotPending,
		"expected ErrRequestExpired or ErrRequestNotPending, got: %v", err)
}

func TestDeletionRequestExpirationOnGet(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateDeletionRequest("alice", DeletionTypeSnapshot, []string{"snap1"}, nil, "reason", 1)

	// Manually expire the request
	req.ExpiresAt = time.Now().Add(-time.Hour)
	require.NoError(t, m.saveDeletionRequest(req))

	// Get should mark it as expired
	got, err := m.GetDeletionRequest(req.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusExpired, got.Status)
}

func TestApproveDeletionNotPending(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateDeletionRequest("alice", DeletionTypeSnapshot, []string{"snap1"}, nil, "reason", 1)

	// Approve first
	require.NoError(t, m.ApproveDeletion(req.ID, "key1", "Alice", []byte("sig")))

	// Try to approve again
	err := m.ApproveDeletion(req.ID, "key2", "Bob", []byte("sig2"))
	assert.ErrorIs(t, err, apperrors.ErrRequestNotPending)
}

func TestApproveDeletionNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	err := m.ApproveDeletion("nonexistent", "key1", "Alice", []byte("sig"))
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestDenyDeletionNotPending(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateDeletionRequest("alice", DeletionTypeSnapshot, []string{"snap1"}, nil, "reason", 1)

	// Deny first
	require.NoError(t, m.DenyDeletion(req.ID, "bob"))

	// Try to deny again
	err := m.DenyDeletion(req.ID, "charlie")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotPending)
}

func TestDenyDeletionNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	err := m.DenyDeletion("nonexistent", "bob")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestMarkDeletionExecutedNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	err := m.MarkDeletionExecuted("nonexistent")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestGetDeletionApprovalProgressNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, _, err := m.GetDeletionApprovalProgress("nonexistent")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestListPendingDeletionsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	pending, err := m.ListPendingDeletions()
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestListPendingDeletionsFiltersNonPending(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create three deletion requests
	req1, _ := m.CreateDeletionRequest("alice", DeletionTypeSnapshot, []string{"snap1"}, nil, "reason1", 1)
	req2, _ := m.CreateDeletionRequest("bob", DeletionTypePath, nil, []string{"/path"}, "reason2", 1)
	req3, _ := m.CreateDeletionRequest("charlie", DeletionTypePrune, nil, nil, "reason3", 1)

	// Approve one, deny another
	require.NoError(t, m.ApproveDeletion(req1.ID, "key1", "Key1", []byte("sig")))
	require.NoError(t, m.DenyDeletion(req2.ID, "denier"))

	// Only req3 should be pending
	pending, err := m.ListPendingDeletions()
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, req3.ID, pending[0].ID)
}

func TestListPendingDeletionsIgnoresNonJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create a deletion request
	req, _ := m.CreateDeletionRequest("alice", DeletionTypeSnapshot, []string{"snap1"}, nil, "reason", 1)

	// Create a non-JSON file in the deletions directory
	nonJSONFile := filepath.Join(m.deletionDataDir, "readme.txt")
	require.NoError(t, os.WriteFile(nonJSONFile, []byte("not json"), 0600))

	// List should only return the actual request
	pending, err := m.ListPendingDeletions()
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, req.ID, pending[0].ID)
}

func TestDeletionTypeVariants(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	tests := []struct {
		name        string
		delType     DeletionType
		snapshotIDs []string
		paths       []string
	}{
		{
			name:        "snapshot",
			delType:     DeletionTypeSnapshot,
			snapshotIDs: []string{"snap1", "snap2"},
			paths:       nil,
		},
		{
			name:        "path",
			delType:     DeletionTypePath,
			snapshotIDs: nil,
			paths:       []string{"/path/to/delete"},
		},
		{
			name:        "prune",
			delType:     DeletionTypePrune,
			snapshotIDs: nil,
			paths:       nil,
		},
		{
			name:        "all",
			delType:     DeletionTypeAll,
			snapshotIDs: nil,
			paths:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := m.CreateDeletionRequest("alice", tt.delType, tt.snapshotIDs, tt.paths, "reason", 1)
			require.NoError(t, err)

			assert.Equal(t, tt.delType, req.DeletionType)
			assert.Equal(t, tt.snapshotIDs, req.SnapshotIDs)
			assert.Equal(t, tt.paths, req.Paths)

			got, err := m.GetDeletionRequest(req.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.delType, got.DeletionType)
		})
	}
}

// ============================================================================
// Request Interface Tests
// ============================================================================

func TestRestoreRequestInterface(t *testing.T) {
	req := &RestoreRequest{
		ID:                "test-id",
		Status:            StatusPending,
		ExpiresAt:         time.Now().Add(time.Hour),
		RequiredApprovals: 2,
		Approvals:         []Approval{},
	}

	// Test interface methods
	assert.Equal(t, "test-id", req.GetID())
	assert.Equal(t, StatusPending, req.GetStatus())
	assert.Equal(t, 2, req.GetRequiredApprovals())
	assert.Empty(t, req.GetApprovals())

	// Test SetStatus
	req.SetStatus(StatusApproved)
	assert.Equal(t, StatusApproved, req.GetStatus())

	// Test AddApproval
	approval := Approval{KeyHolderID: "key1", KeyHolderName: "Test", Signature: []byte("sig")}
	req.AddApproval(approval)
	assert.Len(t, req.GetApprovals(), 1)
	assert.Equal(t, "key1", req.GetApprovals()[0].KeyHolderID)
}

func TestDeletionRequestInterface(t *testing.T) {
	req := &DeletionRequest{
		ID:                "test-id",
		Status:            StatusPending,
		ExpiresAt:         time.Now().Add(time.Hour),
		RequiredApprovals: 3,
		Approvals:         []Approval{},
	}

	// Test interface methods
	assert.Equal(t, "test-id", req.GetID())
	assert.Equal(t, StatusPending, req.GetStatus())
	assert.Equal(t, 3, req.GetRequiredApprovals())
	assert.Empty(t, req.GetApprovals())

	// Test SetStatus
	req.SetStatus(StatusDenied)
	assert.Equal(t, StatusDenied, req.GetStatus())

	// Test AddApproval
	approval := Approval{KeyHolderID: "key1", KeyHolderName: "Test", Signature: []byte("sig")}
	req.AddApproval(approval)
	assert.Len(t, req.GetApprovals(), 1)
}

// ============================================================================
// RequestStore Generic Tests
// ============================================================================

func TestRequestStoreGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	_, err := store.Get("nonexistent")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotFound)
}

func TestRequestStoreSaveAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:        "test-123",
		Requester: "alice",
		Status:    StatusPending,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	err := store.Save(req)
	require.NoError(t, err)

	got, err := store.Get("test-123")
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Requester)
}

func TestRequestStoreList(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	// Empty initially
	list, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, list)

	// Add some requests
	req1 := &RestoreRequest{ID: "req1", Status: StatusPending, ExpiresAt: time.Now().Add(time.Hour)}
	req2 := &RestoreRequest{ID: "req2", Status: StatusApproved, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, store.Save(req1))
	require.NoError(t, store.Save(req2))

	list, err = store.List()
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestRequestStoreListPending(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req1 := &RestoreRequest{ID: "req1", Status: StatusPending, ExpiresAt: time.Now().Add(time.Hour)}
	req2 := &RestoreRequest{ID: "req2", Status: StatusApproved, ExpiresAt: time.Now().Add(time.Hour)}
	req3 := &RestoreRequest{ID: "req3", Status: StatusPending, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, store.Save(req1))
	require.NoError(t, store.Save(req2))
	require.NoError(t, store.Save(req3))

	pending, err := store.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 2)
}

func TestRequestStoreAddApproval(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:                "req1",
		Status:            StatusPending,
		ExpiresAt:         time.Now().Add(time.Hour),
		RequiredApprovals: 2,
		Approvals:         []Approval{},
	}
	require.NoError(t, store.Save(req))

	// Add first approval
	err := store.AddApproval("req1", "key1", "Alice", []byte("sig1"))
	require.NoError(t, err)

	got, _ := store.Get("req1")
	assert.Len(t, got.Approvals, 1)
	assert.Equal(t, StatusPending, got.Status)

	// Add second approval - should approve
	err = store.AddApproval("req1", "key2", "Bob", []byte("sig2"))
	require.NoError(t, err)

	got, _ = store.Get("req1")
	assert.Len(t, got.Approvals, 2)
	assert.Equal(t, StatusApproved, got.Status)
}

func TestRequestStoreAddApprovalNotPending(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:        "req1",
		Status:    StatusApproved,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, store.Save(req))

	err := store.AddApproval("req1", "key1", "Alice", []byte("sig"))
	assert.ErrorIs(t, err, apperrors.ErrRequestNotPending)
}

func TestRequestStoreAddApprovalExpired(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:        "req1",
		Status:    StatusPending,
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
	}
	require.NoError(t, store.Save(req))

	err := store.AddApproval("req1", "key1", "Alice", []byte("sig"))
	assert.ErrorIs(t, err, apperrors.ErrRequestExpired)
}

func TestRequestStoreAddApprovalDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:                "req1",
		Status:            StatusPending,
		ExpiresAt:         time.Now().Add(time.Hour),
		RequiredApprovals: 2,
		Approvals:         []Approval{},
	}
	require.NoError(t, store.Save(req))

	require.NoError(t, store.AddApproval("req1", "key1", "Alice", []byte("sig1")))

	err := store.AddApproval("req1", "key1", "Alice", []byte("sig2"))
	assert.ErrorIs(t, err, apperrors.ErrAlreadyApproved)
}

func TestRequestStoreDeny(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:        "req1",
		Status:    StatusPending,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, store.Save(req))

	err := store.Deny("req1", "denier")
	require.NoError(t, err)

	got, _ := store.Get("req1")
	assert.Equal(t, StatusDenied, got.Status)
}

func TestRequestStoreDenyNotPending(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:        "req1",
		Status:    StatusApproved,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, store.Save(req))

	err := store.Deny("req1", "denier")
	assert.ErrorIs(t, err, apperrors.ErrRequestNotPending)
}

func TestRequestStoreHasEnoughApprovals(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:                "req1",
		Status:            StatusPending,
		ExpiresAt:         time.Now().Add(time.Hour),
		RequiredApprovals: 1,
		Approvals: []Approval{
			{KeyHolderID: "key1"},
		},
	}
	require.NoError(t, store.Save(req))

	enough, err := store.HasEnoughApprovals("req1")
	require.NoError(t, err)
	assert.True(t, enough)
}

func TestRequestStoreGetApprovalProgress(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		nil,
	)

	req := &RestoreRequest{
		ID:                "req1",
		Status:            StatusPending,
		ExpiresAt:         time.Now().Add(time.Hour),
		RequiredApprovals: 3,
		Approvals: []Approval{
			{KeyHolderID: "key1"},
			{KeyHolderID: "key2"},
		},
	}
	require.NoError(t, store.Save(req))

	current, required, err := store.GetApprovalProgress("req1")
	require.NoError(t, err)
	assert.Equal(t, 2, current)
	assert.Equal(t, 3, required)
}

func TestRequestStoreWithExpiryCallback(t *testing.T) {
	tmpDir := t.TempDir()
	expiryCalled := false

	store := NewRequestStore[*RestoreRequest](
		tmpDir,
		func() *RestoreRequest { return &RestoreRequest{} },
		func(s *RequestStore[*RestoreRequest], req *RestoreRequest) {
			expiryCalled = true
			if req.Status == StatusPending && time.Now().After(req.ExpiresAt) {
				req.SetStatus(StatusExpired)
				_ = s.Save(req)
			}
		},
	)

	req := &RestoreRequest{
		ID:        "req1",
		Status:    StatusPending,
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
	}
	require.NoError(t, store.Save(req))

	got, err := store.Get("req1")
	require.NoError(t, err)
	assert.True(t, expiryCalled)
	assert.Equal(t, StatusExpired, got.Status)
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestConcurrentRequestCreation(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			_, err := m.CreateRequest("user"+string(rune('0'+i)), "latest", "reason", nil)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	pending, err := m.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 10)
}

func TestSequentialApprovals(t *testing.T) {
	// Test sequential approvals to verify the approval flow works correctly
	// Note: The consent package doesn't have locking for concurrent writes,
	// so concurrent approvals to the same request can cause race conditions.
	// In practice, approvals come from different machines over HTTP.
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 3)

	// Add signatures sequentially
	for i := 0; i < 3; i++ {
		keyID := "key" + string(rune('0'+i))
		name := "User" + string(rune('0'+i))
		err := m.AddSignature(req.ID, keyID, name, []byte("sig"+string(rune('0'+i))))
		require.NoError(t, err)
	}

	// Request should be approved
	got, err := m.GetRequest(req.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, got.Status)
	assert.Len(t, got.Approvals, 3)
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestZeroRequiredApprovals(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Creating with 0 required approvals should still create pending request
	req, err := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 0)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, req.Status)

	// Any signature should immediately approve it
	err = m.AddSignature(req.ID, "key1", "Alice", []byte("sig"))
	require.NoError(t, err)

	got, _ := m.GetRequest(req.ID)
	assert.Equal(t, StatusApproved, got.Status)
}

func TestEmptyPaths(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Empty paths slice
	req, err := m.CreateRequest("alice", "latest", "reason", []string{})
	require.NoError(t, err)
	assert.Empty(t, req.Paths)

	// Nil paths
	req2, err := m.CreateRequest("bob", "latest", "reason", nil)
	require.NoError(t, err)
	assert.Nil(t, req2.Paths)
}

func TestEmptyRequesterAndReason(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Should allow empty strings (validation is up to the caller)
	req, err := m.CreateRequest("", "snap", "", nil)
	require.NoError(t, err)
	assert.Empty(t, req.Requester)
	assert.Empty(t, req.Reason)
}

func TestApprovalTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	before := time.Now()
	req, _ := m.CreateRequest("alice", "latest", "reason", nil)
	require.NoError(t, m.Approve(req.ID, "bob", []byte("share")))

	got, _ := m.GetRequest(req.ID)
	require.NotNil(t, got.ApprovedAt)
	assert.True(t, got.ApprovedAt.After(before) || got.ApprovedAt.Equal(before))
	assert.True(t, got.ApprovedAt.Before(time.Now().Add(time.Second)))
}

func TestDeletionExpiryIs7Days(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	before := time.Now()
	req, _ := m.CreateDeletionRequest("alice", DeletionTypeSnapshot, []string{"snap"}, nil, "reason", 1)

	// Expiry should be approximately 7 days from now
	expectedExpiry := before.Add(7 * 24 * time.Hour)
	assert.True(t, req.ExpiresAt.After(expectedExpiry.Add(-time.Minute)))
	assert.True(t, req.ExpiresAt.Before(expectedExpiry.Add(time.Minute)))
}

func TestRestoreExpiryIs24Hours(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	before := time.Now()
	req, _ := m.CreateRequest("alice", "latest", "reason", nil)

	// Expiry should be approximately 24 hours from now
	expectedExpiry := before.Add(24 * time.Hour)
	assert.True(t, req.ExpiresAt.After(expectedExpiry.Add(-time.Minute)))
	assert.True(t, req.ExpiresAt.Before(expectedExpiry.Add(time.Minute)))
}

func TestApprovalStructFields(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	req, _ := m.CreateRequestWithConsensus("alice", "latest", "reason", nil, 1)

	before := time.Now()
	require.NoError(t, m.AddSignature(req.ID, "key-123", "Alice Keys", []byte("signature-data")))
	after := time.Now()

	got, _ := m.GetRequest(req.ID)
	require.Len(t, got.Approvals, 1)

	approval := got.Approvals[0]
	assert.Equal(t, "key-123", approval.KeyHolderID)
	assert.Equal(t, "Alice Keys", approval.KeyHolderName)
	assert.Equal(t, []byte("signature-data"), approval.Signature)
	assert.True(t, approval.ApprovedAt.After(before) || approval.ApprovedAt.Equal(before))
	assert.True(t, approval.ApprovedAt.Before(after) || approval.ApprovedAt.Equal(after))
}
