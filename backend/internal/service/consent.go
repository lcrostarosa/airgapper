package service

import (
	"errors"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// ConsentService handles restore/deletion consent business logic
type ConsentService struct {
	cfg        *config.Config
	consentMgr *consent.Manager
}

// NewConsentService creates a new consent service
func NewConsentService(cfg *config.Config, mgr *consent.Manager) *ConsentService {
	return &ConsentService{cfg: cfg, consentMgr: mgr}
}

// CreateRestoreRequestParams contains parameters for creating a restore request
type CreateRestoreRequestParams struct {
	SnapshotID string
	Paths      []string
	Reason     string
}

// CreateRestoreRequest creates a new restore request
func (s *ConsentService) CreateRestoreRequest(params CreateRestoreRequestParams) (*consent.RestoreRequest, error) {
	snapshotID := params.SnapshotID
	if snapshotID == "" {
		snapshotID = "latest"
	}
	return s.consentMgr.CreateRequest(s.cfg.Name, snapshotID, params.Reason, params.Paths)
}

// ListPendingRequests returns all pending restore requests
func (s *ConsentService) ListPendingRequests() ([]*consent.RestoreRequest, error) {
	return s.consentMgr.ListPending()
}

// GetRequest returns a specific request by ID
func (s *ConsentService) GetRequest(id string) (*consent.RestoreRequest, error) {
	return s.consentMgr.GetRequest(id)
}

// ApproveRequest approves a restore request with the local share
func (s *ConsentService) ApproveRequest(id string, share []byte) error {
	if share == nil {
		localShare, _, err := s.cfg.LoadShare()
		if err != nil {
			return errors.New("no share available")
		}
		share = localShare
	}
	return s.consentMgr.Approve(id, s.cfg.Name, share)
}

// DenyRequest denies a restore request
func (s *ConsentService) DenyRequest(id string) error {
	return s.consentMgr.Deny(id, s.cfg.Name)
}

// SignRequestParams contains parameters for signing a request
type SignRequestParams struct {
	RequestID   string
	KeyHolderID string
	Signature   []byte
}

// SignRequest adds a signature to a restore request (consensus mode)
func (s *ConsentService) SignRequest(params SignRequestParams) (*ApprovalProgress, error) {
	// Verify key holder exists
	holder := s.cfg.GetKeyHolder(params.KeyHolderID)
	if holder == nil {
		return nil, errors.New("unknown key holder")
	}

	// Get the request
	req, err := s.consentMgr.GetRequest(params.RequestID)
	if err != nil {
		return nil, err
	}

	// Verify signature
	valid, err := crypto.VerifyRestoreRequestSignature(
		holder.PublicKey,
		params.Signature,
		req.ID,
		req.Requester,
		req.SnapshotID,
		req.Reason,
		params.KeyHolderID,
		req.Paths,
		req.CreatedAt.Unix(),
	)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errors.New("invalid signature")
	}

	// Add the signature
	if err := s.consentMgr.AddSignature(params.RequestID, params.KeyHolderID, holder.Name, params.Signature); err != nil {
		return nil, err
	}

	return s.GetApprovalProgress(params.RequestID)
}

// ApprovalProgress represents the approval status of a request
type ApprovalProgress struct {
	Current    int
	Required   int
	IsApproved bool
}

// GetApprovalProgress returns the approval progress for a request
func (s *ConsentService) GetApprovalProgress(id string) (*ApprovalProgress, error) {
	current, required, err := s.consentMgr.GetApprovalProgress(id)
	if err != nil {
		return nil, err
	}
	return &ApprovalProgress{
		Current:    current,
		Required:   required,
		IsApproved: current >= required,
	}, nil
}

// --- Deletion Requests ---

// CreateDeletionRequestParams contains parameters for creating a deletion request
type CreateDeletionRequestParams struct {
	DeletionType      consent.DeletionType
	SnapshotIDs       []string
	Paths             []string
	Reason            string
	RequiredApprovals int
}

// CreateDeletionRequest creates a new deletion request
func (s *ConsentService) CreateDeletionRequest(params CreateDeletionRequestParams) (*consent.DeletionRequest, error) {
	return s.consentMgr.CreateDeletionRequest(
		s.cfg.Name,
		params.DeletionType,
		params.SnapshotIDs,
		params.Paths,
		params.Reason,
		params.RequiredApprovals,
	)
}

// ListPendingDeletions returns all pending deletion requests
func (s *ConsentService) ListPendingDeletions() ([]*consent.DeletionRequest, error) {
	return s.consentMgr.ListPendingDeletions()
}

// GetDeletionRequest returns a specific deletion request by ID
func (s *ConsentService) GetDeletionRequest(id string) (*consent.DeletionRequest, error) {
	return s.consentMgr.GetDeletionRequest(id)
}

// ApproveDeletion approves a deletion request
func (s *ConsentService) ApproveDeletion(id, keyHolderID string, signature []byte) (*ApprovalProgress, error) {
	// Get key holder name
	keyHolderName := keyHolderID
	if s.cfg.Consensus != nil {
		for _, kh := range s.cfg.Consensus.KeyHolders {
			if kh.ID == keyHolderID {
				keyHolderName = kh.Name
				break
			}
		}
	}

	if err := s.consentMgr.ApproveDeletion(id, keyHolderID, keyHolderName, signature); err != nil {
		return nil, err
	}

	current, required, err := s.consentMgr.GetDeletionApprovalProgress(id)
	if err != nil {
		return nil, err
	}

	return &ApprovalProgress{
		Current:    current,
		Required:   required,
		IsApproved: current >= required,
	}, nil
}

// DenyDeletion denies a deletion request
func (s *ConsentService) DenyDeletion(id string) error {
	return s.consentMgr.DenyDeletion(id, s.cfg.Name)
}
