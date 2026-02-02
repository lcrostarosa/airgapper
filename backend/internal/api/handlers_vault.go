package api

import (
	"net/http"

	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// handleVaultInit initializes a new vault
func (s *Server) handleVaultInit(w http.ResponseWriter, r *http.Request) {
	var body VaultInitBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := s.vaultSvc.Init(service.InitParams{
		Name:            body.Name,
		RepoURL:         body.RepoURL,
		Threshold:       body.Threshold,
		TotalKeys:       body.TotalKeys,
		BackupPaths:     body.BackupPaths,
		RequireApproval: body.RequireApproval,
	})
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, VaultInitResponseDTO{
		Name:      result.Name,
		KeyID:     result.KeyID,
		PublicKey: result.PublicKey,
		Threshold: result.Threshold,
		TotalKeys: result.TotalKeys,
	})
}
