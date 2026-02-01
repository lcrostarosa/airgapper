// Package consent handles restore and deletion approval workflows
package consent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// RequestStatus represents the status of a restore request
type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusApproved RequestStatus = "approved"
	StatusDenied   RequestStatus = "denied"
	StatusExpired  RequestStatus = "expired"
)

// Approval represents a cryptographic approval from a key holder
type Approval struct {
	KeyHolderID string    `json:"key_holder_id"` // ID of the key holder who approved
	KeyHolderName string  `json:"key_holder_name,omitempty"` // Name of the key holder
	Signature   []byte    `json:"signature"`     // Ed25519 signature over request hash
	ApprovedAt  time.Time `json:"approved_at"`
}

// RestoreRequest represents a request to restore data
type RestoreRequest struct {
	ID          string        `json:"id"`
	Requester   string        `json:"requester"`    // Name of requesting party
	SnapshotID  string        `json:"snapshot_id"`  // Restic snapshot to restore
	Paths       []string      `json:"paths"`        // Specific paths (optional)
	Reason      string        `json:"reason"`       // Why restore is needed
	Status      RequestStatus `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	ExpiresAt   time.Time     `json:"expires_at"`
	ApprovedAt  *time.Time    `json:"approved_at,omitempty"`
	ApprovedBy  string        `json:"approved_by,omitempty"`
	ShareData   []byte        `json:"share_data,omitempty"` // Released share (only after approval) - legacy SSS mode

	// Consensus mode fields
	RequiredApprovals int        `json:"required_approvals,omitempty"` // Number of approvals needed (m in m-of-n)
	Approvals         []Approval `json:"approvals,omitempty"`          // Collected cryptographic approvals
}

// DeletionType specifies what is being deleted
type DeletionType string

const (
	DeletionTypeSnapshot DeletionType = "snapshot" // Delete specific snapshot(s)
	DeletionTypePath     DeletionType = "path"     // Delete specific paths
	DeletionTypePrune    DeletionType = "prune"    // Prune old snapshots
	DeletionTypeAll      DeletionType = "all"      // Delete entire repository
)

// DeletionRequest represents a request to delete backup data
type DeletionRequest struct {
	ID             string        `json:"id"`
	Requester      string        `json:"requester"`       // Name of requesting party
	DeletionType   DeletionType  `json:"deletion_type"`   // What to delete
	SnapshotIDs    []string      `json:"snapshot_ids"`    // Specific snapshots (for snapshot type)
	Paths          []string      `json:"paths"`           // Specific paths (for path type)
	Reason         string        `json:"reason"`          // Why deletion is needed
	Status         RequestStatus `json:"status"`
	CreatedAt      time.Time     `json:"created_at"`
	ExpiresAt      time.Time     `json:"expires_at"`
	ApprovedAt     *time.Time    `json:"approved_at,omitempty"`
	ApprovedBy     string        `json:"approved_by,omitempty"`
	ExecutedAt     *time.Time    `json:"executed_at,omitempty"` // When deletion was performed

	// Consensus mode fields
	RequiredApprovals int        `json:"required_approvals,omitempty"`
	Approvals         []Approval `json:"approvals,omitempty"`
}

// Manager handles consent operations
type Manager struct {
	dataDir         string
	deletionDataDir string
}

// NewManager creates a consent manager
func NewManager(dataDir string) *Manager {
	return &Manager{
		dataDir:         filepath.Join(dataDir, "requests"),
		deletionDataDir: filepath.Join(dataDir, "deletions"),
	}
}

// CreateRequest creates a new restore request
func (m *Manager) CreateRequest(requester, snapshotID, reason string, paths []string) (*RestoreRequest, error) {
	// Generate unique ID
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}

	req := &RestoreRequest{
		ID:         hex.EncodeToString(idBytes),
		Requester:  requester,
		SnapshotID: snapshotID,
		Paths:      paths,
		Reason:     reason,
		Status:     StatusPending,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour), // 24 hour expiry
	}

	if err := m.saveRequest(req); err != nil {
		return nil, err
	}

	return req, nil
}

// GetRequest retrieves a request by ID
func (m *Manager) GetRequest(id string) (*RestoreRequest, error) {
	path := filepath.Join(m.dataDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("request not found")
		}
		return nil, err
	}

	var req RestoreRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}

	// Check expiry
	if req.Status == StatusPending && time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		m.saveRequest(&req)
	}

	return &req, nil
}

// ListPending returns all pending requests
func (m *Manager) ListPending() ([]*RestoreRequest, error) {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(m.dataDir)
	if err != nil {
		return nil, err
	}

	var requests []*RestoreRequest
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5]
		req, err := m.GetRequest(id)
		if err != nil {
			continue
		}

		if req.Status == StatusPending {
			requests = append(requests, req)
		}
	}

	return requests, nil
}

// Approve approves a request and attaches the share data
func (m *Manager) Approve(id, approver string, shareData []byte) error {
	req, err := m.GetRequest(id)
	if err != nil {
		return err
	}

	if req.Status != StatusPending {
		return errors.New("request is not pending")
	}

	if time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		m.saveRequest(req)
		return errors.New("request has expired")
	}

	now := time.Now()
	req.Status = StatusApproved
	req.ApprovedAt = &now
	req.ApprovedBy = approver
	req.ShareData = shareData

	return m.saveRequest(req)
}

// Deny denies a request
func (m *Manager) Deny(id, denier string) error {
	req, err := m.GetRequest(id)
	if err != nil {
		return err
	}

	if req.Status != StatusPending {
		return errors.New("request is not pending")
	}

	req.Status = StatusDenied
	now := time.Now()
	req.ApprovedAt = &now
	req.ApprovedBy = denier

	return m.saveRequest(req)
}

func (m *Manager) saveRequest(req *RestoreRequest) error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.dataDir, req.ID+".json")
	return os.WriteFile(path, data, 0600)
}

// CreateRequestWithConsensus creates a new restore request with consensus requirements
func (m *Manager) CreateRequestWithConsensus(requester, snapshotID, reason string, paths []string, requiredApprovals int) (*RestoreRequest, error) {
	// Generate unique ID
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}

	req := &RestoreRequest{
		ID:                hex.EncodeToString(idBytes),
		Requester:         requester,
		SnapshotID:        snapshotID,
		Paths:             paths,
		Reason:            reason,
		Status:            StatusPending,
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		RequiredApprovals: requiredApprovals,
		Approvals:         []Approval{},
	}

	if err := m.saveRequest(req); err != nil {
		return nil, err
	}

	return req, nil
}

// AddSignature adds a cryptographic signature/approval to a request
func (m *Manager) AddSignature(id, keyHolderID, keyHolderName string, signature []byte) error {
	req, err := m.GetRequest(id)
	if err != nil {
		return err
	}

	if req.Status != StatusPending {
		return errors.New("request is not pending")
	}

	if time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		m.saveRequest(req)
		return errors.New("request has expired")
	}

	// Check if this key holder already approved
	for _, approval := range req.Approvals {
		if approval.KeyHolderID == keyHolderID {
			return errors.New("key holder already approved this request")
		}
	}

	// Add the approval
	approval := Approval{
		KeyHolderID:   keyHolderID,
		KeyHolderName: keyHolderName,
		Signature:     signature,
		ApprovedAt:    time.Now(),
	}
	req.Approvals = append(req.Approvals, approval)

	// Check if we have enough approvals
	if len(req.Approvals) >= req.RequiredApprovals {
		now := time.Now()
		req.Status = StatusApproved
		req.ApprovedAt = &now
		req.ApprovedBy = "consensus"
	}

	return m.saveRequest(req)
}

// HasEnoughApprovals checks if a request has sufficient approvals
func (m *Manager) HasEnoughApprovals(id string) (bool, error) {
	req, err := m.GetRequest(id)
	if err != nil {
		return false, err
	}
	return len(req.Approvals) >= req.RequiredApprovals, nil
}

// GetApprovalProgress returns current approvals and required count
func (m *Manager) GetApprovalProgress(id string) (current int, required int, err error) {
	req, err := m.GetRequest(id)
	if err != nil {
		return 0, 0, err
	}
	return len(req.Approvals), req.RequiredApprovals, nil
}

// ============================================================================
// Deletion Request Operations
// ============================================================================

// CreateDeletionRequest creates a new deletion request
// Deletion requests have a longer expiry (7 days) than restore requests
func (m *Manager) CreateDeletionRequest(requester string, deletionType DeletionType, snapshotIDs, paths []string, reason string, requiredApprovals int) (*DeletionRequest, error) {
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}

	req := &DeletionRequest{
		ID:                hex.EncodeToString(idBytes),
		Requester:         requester,
		DeletionType:      deletionType,
		SnapshotIDs:       snapshotIDs,
		Paths:             paths,
		Reason:            reason,
		Status:            StatusPending,
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(7 * 24 * time.Hour), // 7 day expiry
		RequiredApprovals: requiredApprovals,
		Approvals:         []Approval{},
	}

	if err := m.saveDeletionRequest(req); err != nil {
		return nil, err
	}

	return req, nil
}

// GetDeletionRequest retrieves a deletion request by ID
func (m *Manager) GetDeletionRequest(id string) (*DeletionRequest, error) {
	path := filepath.Join(m.deletionDataDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("deletion request not found")
		}
		return nil, err
	}

	var req DeletionRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}

	// Check expiry
	if req.Status == StatusPending && time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		m.saveDeletionRequest(&req)
	}

	return &req, nil
}

// ListPendingDeletions returns all pending deletion requests
func (m *Manager) ListPendingDeletions() ([]*DeletionRequest, error) {
	if err := os.MkdirAll(m.deletionDataDir, 0700); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(m.deletionDataDir)
	if err != nil {
		return nil, err
	}

	var requests []*DeletionRequest
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5]
		req, err := m.GetDeletionRequest(id)
		if err != nil {
			continue
		}

		if req.Status == StatusPending {
			requests = append(requests, req)
		}
	}

	return requests, nil
}

// ApproveDeletion approves a deletion request with a signature
func (m *Manager) ApproveDeletion(id, keyHolderID, keyHolderName string, signature []byte) error {
	req, err := m.GetDeletionRequest(id)
	if err != nil {
		return err
	}

	if req.Status != StatusPending {
		return errors.New("deletion request is not pending")
	}

	if time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		m.saveDeletionRequest(req)
		return errors.New("deletion request has expired")
	}

	// Check if this key holder already approved
	for _, approval := range req.Approvals {
		if approval.KeyHolderID == keyHolderID {
			return errors.New("key holder already approved this deletion request")
		}
	}

	// Add the approval
	approval := Approval{
		KeyHolderID:   keyHolderID,
		KeyHolderName: keyHolderName,
		Signature:     signature,
		ApprovedAt:    time.Now(),
	}
	req.Approvals = append(req.Approvals, approval)

	// Check if we have enough approvals
	if len(req.Approvals) >= req.RequiredApprovals {
		now := time.Now()
		req.Status = StatusApproved
		req.ApprovedAt = &now
		req.ApprovedBy = "consensus"
	}

	return m.saveDeletionRequest(req)
}

// DenyDeletion denies a deletion request
func (m *Manager) DenyDeletion(id, denier string) error {
	req, err := m.GetDeletionRequest(id)
	if err != nil {
		return err
	}

	if req.Status != StatusPending {
		return errors.New("deletion request is not pending")
	}

	req.Status = StatusDenied
	now := time.Now()
	req.ApprovedAt = &now
	req.ApprovedBy = denier

	return m.saveDeletionRequest(req)
}

// MarkDeletionExecuted marks a deletion request as executed
func (m *Manager) MarkDeletionExecuted(id string) error {
	req, err := m.GetDeletionRequest(id)
	if err != nil {
		return err
	}

	if req.Status != StatusApproved {
		return errors.New("deletion request is not approved")
	}

	now := time.Now()
	req.ExecutedAt = &now

	return m.saveDeletionRequest(req)
}

// GetDeletionApprovalProgress returns current approvals and required count for a deletion
func (m *Manager) GetDeletionApprovalProgress(id string) (current int, required int, err error) {
	req, err := m.GetDeletionRequest(id)
	if err != nil {
		return 0, 0, err
	}
	return len(req.Approvals), req.RequiredApprovals, nil
}

func (m *Manager) saveDeletionRequest(req *DeletionRequest) error {
	if err := os.MkdirAll(m.deletionDataDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.deletionDataDir, req.ID+".json")
	return os.WriteFile(path, data, 0600)
}
