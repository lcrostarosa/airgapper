package api

import (
	"encoding/json"
	"net/http"

	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// handleListRequests lists all pending restore requests
func (s *Server) handleListRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := s.consentSvc.ListPendingRequests()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ToRestoreRequestDTOs(requests))
}

// handleCreateRequest creates a new restore request
func (s *Server) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	var body CreateRequestBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	req, err := s.consentSvc.CreateRestoreRequest(service.CreateRestoreRequestParams{
		SnapshotID: body.SnapshotID,
		Paths:      body.Paths,
		Reason:     body.Reason,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, ToRestoreRequestCreatedDTO(req))
}

// handleGetRequest returns a specific restore request
func (s *Server) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := RequirePathParam(w, r, "id")
	if !ok {
		return
	}

	req, err := s.consentSvc.GetRequest(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ToRestoreRequestDetailDTO(req))
}

// handleApproveRequest approves a restore request
func (s *Server) handleApproveRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := RequirePathParam(w, r, "id")
	if !ok {
		return
	}

	var body ApproveBody
	// Empty body is OK - we use local share
	_ = json.NewDecoder(r.Body).Decode(&body)

	if err := s.consentSvc.ApproveRequest(id, body.Share); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, NewStatusMessage("approved", "Key share released"))
}

// handleDenyRequest denies a restore request
func (s *Server) handleDenyRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := RequirePathParam(w, r, "id")
	if !ok {
		return
	}

	if err := s.consentSvc.DenyRequest(id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, NewStatusMessage("denied", ""))
}

// handleSignRequest handles signature submission for a restore request
func (s *Server) handleSignRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := RequirePathParam(w, r, "id")
	if !ok {
		return
	}

	var body SignRequestBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Decode signature from hex
	sigBytes, err := decodeHexSignature(body.Signature)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}

	// Sign via service (handles verification internally)
	progress, err := s.consentSvc.SignRequest(service.SignRequestParams{
		RequestID:   id,
		KeyHolderID: body.KeyHolderID,
		Signature:   sigBytes,
	})
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ApprovalProgressDTO{
		Status:            "signature_added",
		CurrentApprovals:  progress.Current,
		RequiredApprovals: progress.Required,
		IsApproved:        progress.IsApproved,
	})
}
