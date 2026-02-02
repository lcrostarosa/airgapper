package api

import (
	"net/http"

	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// handleListDeletions lists all pending deletion requests
func (s *Server) handleListDeletions(w http.ResponseWriter, r *http.Request) {
	deletions, err := s.consentSvc.ListPendingDeletions()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ToDeletionRequestDTOs(deletions))
}

// handleCreateDeletion creates a new deletion request
func (s *Server) handleCreateDeletion(w http.ResponseWriter, r *http.Request) {
	var body CreateDeletionBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	del, err := s.consentSvc.CreateDeletionRequest(service.CreateDeletionRequestParams{
		DeletionType:      consent.DeletionType(body.DeletionType),
		SnapshotIDs:       body.SnapshotIDs,
		Paths:             body.Paths,
		Reason:            body.Reason,
		RequiredApprovals: body.RequiredApprovals,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, ToDeletionRequestCreatedDTO(del))
}

// handleGetDeletion returns a specific deletion request
func (s *Server) handleGetDeletion(w http.ResponseWriter, r *http.Request) {
	id, ok := RequirePathParam(w, r, "id")
	if !ok {
		return
	}

	del, err := s.consentSvc.GetDeletionRequest(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ToDeletionRequestDetailDTO(del))
}

// handleApproveDeletion approves a deletion request
func (s *Server) handleApproveDeletion(w http.ResponseWriter, r *http.Request) {
	id, ok := RequirePathParam(w, r, "id")
	if !ok {
		return
	}

	var body ApproveDeletionBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	sigBytes, err := decodeHexSignature(body.Signature)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}

	progress, err := s.consentSvc.ApproveDeletion(id, body.KeyHolderID, sigBytes)
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

// handleDenyDeletion denies a deletion request
func (s *Server) handleDenyDeletion(w http.ResponseWriter, r *http.Request) {
	id, ok := RequirePathParam(w, r, "id")
	if !ok {
		return
	}

	if err := s.consentSvc.DenyDeletion(id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, NewStatusMessage("denied", ""))
}
