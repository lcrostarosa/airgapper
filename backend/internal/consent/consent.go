// Package consent handles restore approval workflows
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
	ShareData   []byte        `json:"share_data,omitempty"` // Released share (only after approval)
}

// Manager handles consent operations
type Manager struct {
	dataDir string
}

// NewManager creates a consent manager
func NewManager(dataDir string) *Manager {
	return &Manager{dataDir: filepath.Join(dataDir, "requests")}
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
