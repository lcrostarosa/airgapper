package consent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	apperrors "github.com/lcrostarosa/airgapper/backend/internal/errors"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// RequestStore provides generic storage operations for request types.
// T must be a pointer type that implements the Request interface.
type RequestStore[T Request] struct {
	dataDir string
	// newRequest creates a new zero value of type T for unmarshaling.
	newRequest func() T
	// checkExpiry updates status if expired and saves.
	checkExpiry func(store *RequestStore[T], req T)
}

// NewRequestStore creates a new request store.
func NewRequestStore[T Request](dataDir string, newFn func() T, expiryFn func(*RequestStore[T], T)) *RequestStore[T] {
	return &RequestStore[T]{
		dataDir:     dataDir,
		newRequest:  newFn,
		checkExpiry: expiryFn,
	}
}

// Get retrieves a request by ID.
func (s *RequestStore[T]) Get(id string) (T, error) {
	path := filepath.Join(s.dataDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		var zero T
		if os.IsNotExist(err) {
			return zero, apperrors.ErrRequestNotFound
		}
		return zero, err
	}

	req := s.newRequest()
	if err := json.Unmarshal(data, req); err != nil {
		var zero T
		return zero, err
	}

	// Check expiry
	if s.checkExpiry != nil {
		s.checkExpiry(s, req)
	}

	return req, nil
}

// List returns all requests (regardless of status).
func (s *RequestStore[T]) List() ([]T, error) {
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, err
	}

	var requests []T
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5]
		req, err := s.Get(id)
		if err != nil {
			continue
		}
		requests = append(requests, req)
	}

	return requests, nil
}

// ListPending returns all pending requests.
func (s *RequestStore[T]) ListPending() ([]T, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}

	var pending []T
	for _, req := range all {
		if req.GetStatus() == StatusPending {
			pending = append(pending, req)
		}
	}
	return pending, nil
}

// Save persists a request to disk.
func (s *RequestStore[T]) Save(req T) error {
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.dataDir, req.GetID()+".json")
	return os.WriteFile(path, data, 0600)
}

// AddApproval adds an approval to a request with validation.
// Returns apperrors.ErrRequestNotPending, apperrors.ErrRequestExpired, or apperrors.ErrAlreadyApproved on failure.
func (s *RequestStore[T]) AddApproval(id, keyHolderID, keyHolderName string, signature []byte) error {
	req, err := s.Get(id)
	if err != nil {
		return err
	}

	if req.GetStatus() != StatusPending {
		return apperrors.ErrRequestNotPending
	}

	if time.Now().After(req.GetExpiresAt()) {
		req.SetStatus(StatusExpired)
		if err := s.Save(req); err != nil {
			logging.Warn("Failed to save expired request", logging.Err(err))
		}
		return apperrors.ErrRequestExpired
	}

	// Check if this key holder already approved
	for _, approval := range req.GetApprovals() {
		if approval.KeyHolderID == keyHolderID {
			return apperrors.ErrAlreadyApproved
		}
	}

	// Add the approval
	approval := Approval{
		KeyHolderID:   keyHolderID,
		KeyHolderName: keyHolderName,
		Signature:     signature,
		ApprovedAt:    time.Now(),
	}
	req.AddApproval(approval)

	// Check if we have enough approvals
	if len(req.GetApprovals()) >= req.GetRequiredApprovals() {
		req.SetStatus(StatusApproved)
	}

	return s.Save(req)
}

// Deny denies a request.
func (s *RequestStore[T]) Deny(id, denier string) error {
	req, err := s.Get(id)
	if err != nil {
		return err
	}

	if req.GetStatus() != StatusPending {
		return apperrors.ErrRequestNotPending
	}

	req.SetStatus(StatusDenied)
	return s.Save(req)
}

// HasEnoughApprovals checks if a request has sufficient approvals.
func (s *RequestStore[T]) HasEnoughApprovals(id string) (bool, error) {
	req, err := s.Get(id)
	if err != nil {
		return false, err
	}
	return len(req.GetApprovals()) >= req.GetRequiredApprovals(), nil
}

// GetApprovalProgress returns current approvals and required count.
func (s *RequestStore[T]) GetApprovalProgress(id string) (current int, required int, err error) {
	req, err := s.Get(id)
	if err != nil {
		return 0, 0, err
	}
	return len(req.GetApprovals()), req.GetRequiredApprovals(), nil
}
