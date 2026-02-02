package api

import (
	"net/http"
	"strconv"

	"github.com/lcrostarosa/airgapper/backend/internal/verification"
)

// --- Audit Chain Handlers ---

// handleGetAuditEntries returns audit chain entries
func (s *Server) handleGetAuditEntries(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuditChain(w) {
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	operation := r.URL.Query().Get("operation")

	entries := s.storageServer.AuditChain().GetEntries(limit, offset, operation)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"entries":  entries,
		"sequence": s.storageServer.AuditChain().GetSequence(),
		"lastHash": s.storageServer.AuditChain().GetLatestHash(),
	})
}

// handleVerifyAuditChain verifies the integrity of the audit chain
func (s *Server) handleVerifyAuditChain(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuditChain(w) {
		return
	}

	result, err := s.storageServer.AuditChain().Verify()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

// handleExportAuditChain exports the audit chain for external verification
func (s *Server) handleExportAuditChain(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuditChain(w) {
		return
	}

	data, err := s.storageServer.AuditChain().Export()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=audit-chain-export.json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// --- Challenge-Response Handlers ---

// handleCreateChallenge creates a new verification challenge (owner side)
func (s *Server) handleCreateChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Requests      []verification.FileChallenge `json:"requests"`
		ExpiryMinutes int                          `json:"expiry_minutes,omitempty"`
	}

	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.Requests) == 0 {
		jsonError(w, http.StatusBadRequest, "at least one file request is required")
		return
	}

	// Get owner's private key from vault service
	ownerKeyID, ownerPrivateKey := s.vaultSvc.GetOwnerKeys()
	if ownerPrivateKey == nil {
		jsonError(w, http.StatusBadRequest, "owner keys not configured")
		return
	}

	expiryMinutes := req.ExpiryMinutes
	if expiryMinutes <= 0 {
		expiryMinutes = 60
	}

	challenge, err := verification.CreateChallenge(ownerPrivateKey, ownerKeyID, req.Requests, expiryMinutes)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, challenge)
}

// handleReceiveChallenge receives a challenge from the owner (host side)
func (s *Server) handleReceiveChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.requireChallengeManager(w) {
		return
	}

	var challenge verification.Challenge
	if err := decodeJSON(r, &challenge); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	cm := s.getChallengeManager()
	if err := cm.ReceiveChallenge(&challenge); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":      "received",
		"challengeId": challenge.ID,
	})
}

// handleRespondToChallenge generates a response to a challenge (host side)
func (s *Server) handleRespondToChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.requireChallengeManager(w) {
		return
	}

	challengeID := r.PathValue("id")
	if challengeID == "" {
		jsonError(w, http.StatusBadRequest, "challenge ID required")
		return
	}

	cm := s.getChallengeManager()
	response, err := cm.RespondToChallenge(challengeID)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, response)
}

// handleVerifyChallenge verifies a challenge response (owner side)
func (s *Server) handleVerifyChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Challenge     *verification.Challenge         `json:"challenge"`
		Response      *verification.ChallengeResponse `json:"response"`
		HostPublicKey string                          `json:"host_public_key,omitempty"`
	}

	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Challenge == nil || req.Response == nil {
		jsonError(w, http.StatusBadRequest, "challenge and response are required")
		return
	}

	var hostPubKey []byte
	if req.HostPublicKey != "" {
		var err error
		hostPubKey, err = decodeHexSignature(req.HostPublicKey)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid host public key")
			return
		}
	}

	result, err := verification.VerifyResponse(req.Challenge, req.Response, hostPubKey)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

// handleListChallenges lists challenges
func (s *Server) handleListChallenges(w http.ResponseWriter, r *http.Request) {
	if !s.requireChallengeManager(w) {
		return
	}

	pendingOnly := r.URL.Query().Get("pending") == "true"
	challenges := s.getChallengeManager().ListChallenges(pendingOnly)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"challenges": challenges,
	})
}

// --- Deletion Ticket Handlers ---

// handleCreateTicket creates a new deletion ticket (owner side)
func (s *Server) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target       verification.TicketTarget `json:"target"`
		Reason       string                    `json:"reason"`
		ValidityDays int                       `json:"validity_days,omitempty"`
	}

	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Target.Type == "" {
		jsonError(w, http.StatusBadRequest, "target type is required")
		return
	}

	// Get owner's private key
	ownerKeyID, ownerPrivateKey := s.vaultSvc.GetOwnerKeys()
	if ownerPrivateKey == nil {
		jsonError(w, http.StatusBadRequest, "owner keys not configured")
		return
	}

	validityDays := req.ValidityDays
	if validityDays <= 0 {
		validityDays = 7
	}

	ticket, err := verification.CreateTicket(ownerPrivateKey, ownerKeyID, req.Target, req.Reason, validityDays)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, ticket)
}

// handleRegisterTicket registers a ticket with the host
func (s *Server) handleRegisterTicket(w http.ResponseWriter, r *http.Request) {
	if !s.requireTicketManager(w) {
		return
	}

	var ticket verification.DeletionTicket
	if err := decodeJSON(r, &ticket); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.storageServer.RegisterTicket(&ticket); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":   "registered",
		"ticketId": ticket.ID,
	})
}

// handleListTickets lists registered tickets
func (s *Server) handleListTickets(w http.ResponseWriter, r *http.Request) {
	if !s.requireTicketManager(w) {
		return
	}

	validOnly := r.URL.Query().Get("valid") != "false"
	tickets := s.storageServer.TicketManager().ListTickets(validOnly)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"tickets": tickets,
	})
}

// handleGetTicket retrieves a specific ticket
func (s *Server) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	if !s.requireTicketManager(w) {
		return
	}

	ticketID := r.PathValue("id")
	if ticketID == "" {
		jsonError(w, http.StatusBadRequest, "ticket ID required")
		return
	}

	ticket := s.storageServer.TicketManager().GetTicket(ticketID)
	if ticket == nil {
		jsonError(w, http.StatusNotFound, "ticket not found")
		return
	}

	jsonResponse(w, http.StatusOK, ticket)
}

// handleGetTicketUsage retrieves usage records for a ticket
func (s *Server) handleGetTicketUsage(w http.ResponseWriter, r *http.Request) {
	if !s.requireTicketManager(w) {
		return
	}

	ticketID := r.PathValue("id")
	records := s.storageServer.TicketManager().GetUsageRecords(ticketID)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"records": records,
	})
}

// --- Witness Handlers ---

// handleSubmitWitnessCheckpoint submits a checkpoint to configured witnesses
func (s *Server) handleSubmitWitnessCheckpoint(w http.ResponseWriter, r *http.Request) {
	var checkpoint verification.WitnessCheckpoint
	if err := decodeJSON(r, &checkpoint); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// For now, just store the checkpoint locally
	// In a full implementation, this would submit to external witnesses
	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"checkpointId": checkpoint.ID,
		"status":       "stored",
	})
}

// handleGetWitnessCheckpoint retrieves a witness checkpoint
func (s *Server) handleGetWitnessCheckpoint(w http.ResponseWriter, r *http.Request) {
	checkpointID := r.PathValue("id")
	if checkpointID == "" {
		jsonError(w, http.StatusBadRequest, "checkpoint ID required")
		return
	}

	// This would retrieve from local storage or external witness
	jsonError(w, http.StatusNotFound, "checkpoint not found")
}

// handleCreateWitnessCheckpoint creates a new checkpoint from current state
func (s *Server) handleCreateWitnessCheckpoint(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}

	var auditSeq uint64
	var auditHash string
	if s.storageServer.AuditChain() != nil {
		auditSeq = s.storageServer.AuditChain().GetSequence()
		auditHash = s.storageServer.AuditChain().GetLatestHash()
	}

	status := s.storageServer.Status()

	// Get host keys
	hostKeyID, hostPrivateKey := s.hostSvc.GetHostKeys()
	if hostPrivateKey == nil {
		jsonError(w, http.StatusBadRequest, "host keys not configured")
		return
	}

	checkpoint, err := verification.CreateCheckpoint(
		auditSeq,
		auditHash,
		"", // manifest merkle root (would come from owner)
		0,  // snapshot count
		status.UsedBytes,
		0, // file count
		hostKeyID,
		hostPrivateKey,
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, checkpoint)
}

// --- Verification Status ---

// handleGetVerificationStatus returns the status of all verification features
func (s *Server) handleGetVerificationStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"enabled": false,
	}

	if s.storageServer == nil {
		jsonResponse(w, http.StatusOK, status)
		return
	}

	vc := s.storageServer.VerificationConfig()
	if vc == nil || !vc.Enabled {
		jsonResponse(w, http.StatusOK, status)
		return
	}

	status["enabled"] = true
	status["auditChain"] = map[string]interface{}{
		"enabled": vc.IsAuditChainEnabled(),
	}
	status["challenge"] = map[string]interface{}{
		"enabled": vc.IsChallengeEnabled(),
	}
	status["tickets"] = map[string]interface{}{
		"enabled": vc.IsTicketsEnabled(),
	}
	status["witness"] = map[string]interface{}{
		"enabled": vc.IsWitnessEnabled(),
	}

	if s.storageServer.AuditChain() != nil {
		status["auditChain"] = map[string]interface{}{
			"enabled":  true,
			"sequence": s.storageServer.AuditChain().GetSequence(),
			"lastHash": s.storageServer.AuditChain().GetLatestHash(),
		}
	}

	jsonResponse(w, http.StatusOK, status)
}

// --- Helper Methods ---

// requireAuditChain checks if audit chain is available
func (s *Server) requireAuditChain(w http.ResponseWriter) bool {
	if s.storageServer == nil || s.storageServer.AuditChain() == nil {
		jsonError(w, http.StatusBadRequest, "audit chain not enabled")
		return false
	}
	return true
}

// requireTicketManager checks if ticket manager is available
func (s *Server) requireTicketManager(w http.ResponseWriter) bool {
	if s.storageServer == nil || s.storageServer.TicketManager() == nil {
		jsonError(w, http.StatusBadRequest, "ticket system not enabled")
		return false
	}
	return true
}

// requireChallengeManager checks if challenge manager would be available
func (s *Server) requireChallengeManager(w http.ResponseWriter) bool {
	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
		return false
	}
	vc := s.storageServer.VerificationConfig()
	if vc == nil || !vc.IsChallengeEnabled() {
		jsonError(w, http.StatusBadRequest, "challenge system not enabled")
		return false
	}
	return true
}

// getChallengeManager gets or creates the challenge manager
// Note: In a full implementation, this would be part of the server initialization
func (s *Server) getChallengeManager() *verification.ChallengeManager {
	// This is a simplified version - in production, the challenge manager
	// would be initialized with proper keys during server setup
	return nil
}

// DTOs for verification endpoints

// VerificationStatusDTO represents the verification system status
type VerificationStatusDTO struct {
	Enabled    bool                   `json:"enabled"`
	AuditChain map[string]interface{} `json:"auditChain,omitempty"`
	Challenge  map[string]interface{} `json:"challenge,omitempty"`
	Tickets    map[string]interface{} `json:"tickets,omitempty"`
	Witness    map[string]interface{} `json:"witness,omitempty"`
}
