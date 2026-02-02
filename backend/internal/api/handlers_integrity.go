package api

import (
	"fmt"
	"net/http"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
)

// handleIntegrityCheck returns basic integrity status
func (s *Server) handleIntegrityCheck(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}

	status := s.storageServer.Status()

	// Get last integrity check if available
	var lastCheck *integrity.CheckResult
	if s.integrityChecker != nil {
		history := s.integrityChecker.GetHistory(1)
		if len(history) > 0 {
			lastCheck = &history[0]
		}
	}

	response := ToIntegrityCheckDTO(status, lastCheck)
	jsonResponse(w, http.StatusOK, response)
}

// handleIntegrityFullCheck runs a full integrity check
func (s *Server) handleIntegrityFullCheck(w http.ResponseWriter, r *http.Request) {
	if !s.requireIntegrityChecker(w) {
		return
	}

	// Get repo name from query param
	repoName := r.URL.Query().Get("repo")
	if repoName == "" {
		repoName = "default"
	}

	result, err := s.integrityChecker.CheckDataIntegrity(repoName)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

// handleGetIntegrityRecord returns a verification record
func (s *Server) handleGetIntegrityRecord(w http.ResponseWriter, r *http.Request) {
	if !s.requireIntegrityChecker(w) {
		return
	}

	snapshotID := r.URL.Query().Get("snapshotId")
	if snapshotID == "" {
		jsonError(w, http.StatusBadRequest, "snapshotId required")
		return
	}

	record := s.integrityChecker.GetVerificationRecord(snapshotID)
	if record == nil {
		jsonError(w, http.StatusNotFound, "record not found")
		return
	}

	jsonResponse(w, http.StatusOK, record)
}

// handleCreateIntegrityRecord creates a new verification record
func (s *Server) handleCreateIntegrityRecord(w http.ResponseWriter, r *http.Request) {
	if !s.requireIntegrityChecker(w) {
		return
	}

	var body CreateIntegrityRecordBody
	if err := decodeJSON(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if body.SnapshotID == "" || body.OwnerKeyID == "" {
		jsonError(w, http.StatusBadRequest, "snapshotId and ownerKeyId required")
		return
	}

	if body.RepoName == "" {
		body.RepoName = "default"
	}

	record, err := s.integrityChecker.CreateVerificationRecord(body.RepoName, body.SnapshotID, body.OwnerKeyID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, record)
}

// handleAddIntegrityRecord adds a signed verification record
func (s *Server) handleAddIntegrityRecord(w http.ResponseWriter, r *http.Request) {
	if !s.requireIntegrityChecker(w) {
		return
	}

	var record integrity.VerificationRecord
	if err := decodeJSON(r, &record); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get owner's public key
	ownerPubKey := r.URL.Query().Get("ownerPublicKey")
	if ownerPubKey == "" {
		jsonError(w, http.StatusBadRequest, "ownerPublicKey required in query params")
		return
	}

	pubKey, err := crypto.DecodePublicKey(ownerPubKey)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid public key")
		return
	}

	if err := s.integrityChecker.AddVerificationRecord(&record, pubKey); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, RecordAddedDTO{
		Status:  "added",
		Message: "Verification record added and verified",
	})
}

// handleIntegrityHistory returns recent integrity check history
func (s *Server) handleIntegrityHistory(w http.ResponseWriter, r *http.Request) {
	if !s.requireIntegrityChecker(w) {
		return
	}

	limit := 20 // Default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	history := s.integrityChecker.GetHistory(limit)
	jsonResponse(w, http.StatusOK, history)
}

// handleGetVerificationConfig returns verification configuration
func (s *Server) handleGetVerificationConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduledChecker(w) {
		return
	}

	cfg := s.managedScheduledChecker.GetConfig()
	response := ToVerificationConfigDTO(cfg)
	jsonResponse(w, http.StatusOK, response)
}

// handleUpdateVerificationConfig updates verification configuration
func (s *Server) handleUpdateVerificationConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduledChecker(w) {
		return
	}

	var body UpdateVerificationConfigBody
	if err := decodeJSON(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get current config
	currentConfig := s.managedScheduledChecker.GetConfig()

	// Apply updates
	newConfig := &integrity.VerificationConfig{
		Enabled:             currentConfig.Enabled,
		Interval:            currentConfig.Interval,
		CheckType:           currentConfig.CheckType,
		RepoName:            currentConfig.RepoName,
		SnapshotID:          currentConfig.SnapshotID,
		AlertOnCorruption:   currentConfig.AlertOnCorruption,
		AlertWebhook:        currentConfig.AlertWebhook,
		LastCheck:           currentConfig.LastCheck,
		LastResult:          currentConfig.LastResult,
		ConsecutiveFailures: currentConfig.ConsecutiveFailures,
	}

	if body.Enabled != nil {
		newConfig.Enabled = *body.Enabled
	}
	if body.Interval != "" {
		newConfig.Interval = body.Interval
	}
	if body.CheckType != "" {
		newConfig.CheckType = body.CheckType
	}
	if body.RepoName != "" {
		newConfig.RepoName = body.RepoName
	}
	if body.SnapshotID != "" {
		newConfig.SnapshotID = body.SnapshotID
	}
	if body.AlertOnCorruption != nil {
		newConfig.AlertOnCorruption = *body.AlertOnCorruption
	}
	if body.AlertWebhook != "" {
		newConfig.AlertWebhook = body.AlertWebhook
	}

	// Update configuration
	if err := s.managedScheduledChecker.UpdateConfig(newConfig); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	configDTO := ToVerificationConfigDTO(newConfig)
	jsonResponse(w, http.StatusOK, VerificationConfigUpdatedDTO{
		Status:  "updated",
		Message: "Verification configuration updated",
		Config:  &configDTO,
	})
}

// handleRunManualCheck triggers a manual integrity check
func (s *Server) handleRunManualCheck(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduledChecker(w) {
		return
	}

	// Get check type from query param (default to full)
	checkType := r.URL.Query().Get("type")
	if checkType == "" {
		checkType = "full"
	}

	result, err := s.managedScheduledChecker.RunManualCheck(checkType)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, ManualCheckResponseDTO{
		Status:    "completed",
		CheckType: checkType,
		Result:    ToIntegrityCheckResultDTO(result),
	})
}
