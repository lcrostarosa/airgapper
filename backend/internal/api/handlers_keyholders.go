package api

import (
	"net/http"

	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// handleListKeyHolders lists all key holders in consensus mode
func (s *Server) handleListKeyHolders(w http.ResponseWriter, r *http.Request) {
	info, err := s.vaultSvc.GetConsensusInfo()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ConsensusInfoDTO{
		Threshold:       info.Threshold,
		TotalKeys:       info.TotalKeys,
		KeyHolders:      ToKeyHolderDTOs(info.KeyHolders),
		RequireApproval: info.RequireApproval,
	})
}

// handleRegisterKeyHolder registers a new key holder
func (s *Server) handleRegisterKeyHolder(w http.ResponseWriter, r *http.Request) {
	var body RegisterKeyHolderBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := s.vaultSvc.RegisterKeyHolder(service.RegisterKeyHolderParams{
		Name:      body.Name,
		PublicKey: body.PublicKey,
		Address:   body.Address,
	})
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, KeyHolderRegisteredDTO{
		ID:       result.ID,
		Name:     result.Name,
		JoinedAt: result.JoinedAt,
	})
}

// handleGetKeyHolder returns a specific key holder
func (s *Server) handleGetKeyHolder(w http.ResponseWriter, r *http.Request) {
	id, ok := RequirePathParam(w, r, "id")
	if !ok {
		return
	}

	holder, err := s.vaultSvc.GetKeyHolder(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ToKeyHolderDTO(holder))
}
