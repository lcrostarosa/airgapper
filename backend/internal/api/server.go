// Package api provides the HTTP control plane for Airgapper
package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/policy"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// Server is the HTTP API server
type Server struct {
	httpServer               *http.Server
	storageServer            *storage.Server
	integrityChecker         *integrity.Checker
	managedScheduledChecker  *integrity.ManagedScheduledChecker
	addr                     string

	// Services (business logic layer) - handlers MUST use these, not direct data access
	vaultSvc   *service.VaultService
	hostSvc    *service.HostService
	consentSvc *service.ConsentService
	statusSvc  *service.StatusService

	// cfg is for internal server initialization only (storage, integrity).
	// HTTP handlers must NOT use this directly - use services instead.
	cfg *config.Config
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, addr string) *Server {
	consentMgr := consent.NewManager(cfg.ConfigDir)

	s := &Server{
		cfg:        cfg, // Internal use only - handlers use services
		addr:       addr,
		vaultSvc:   service.NewVaultService(cfg),
		hostSvc:    service.NewHostService(cfg),
		consentSvc: service.NewConsentService(cfg, consentMgr),
		statusSvc:  service.NewStatusService(cfg),
	}

	mux := http.NewServeMux()

	// Health & status
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/status", s.handleStatus)

	// Restore requests
	mux.HandleFunc("/api/requests", s.handleRequests)
	mux.HandleFunc("/api/requests/", s.handleRequestByID)

	// Snapshots
	mux.HandleFunc("/api/snapshots", s.handleSnapshots)

	// Peer share exchange (legacy SSS mode)
	mux.HandleFunc("/api/share", s.handleReceiveShare)

	// Scheduler control
	mux.HandleFunc("/api/schedule", s.handleSchedule)

	// Filesystem browsing
	mux.HandleFunc("/api/filesystem/browse", s.handleFilesystemBrowse)

	// Key holder management (consensus mode)
	mux.HandleFunc("/api/keyholders", s.handleKeyHolders)
	mux.HandleFunc("/api/keyholders/", s.handleKeyHolderByID)

	// Vault initialization (for UI)
	mux.HandleFunc("/api/vault/init", s.handleVaultInit)

	// Host initialization (for UI)
	mux.HandleFunc("/api/host/init", s.handleHostInit)

	// Storage server management (host only)
	mux.HandleFunc("/api/storage/status", s.handleStorageStatus)
	mux.HandleFunc("/api/storage/start", s.handleStorageStart)
	mux.HandleFunc("/api/storage/stop", s.handleStorageStop)

	// Network utilities
	mux.HandleFunc("/api/network/local-ip", s.handleLocalIP)

	// Policy management
	mux.HandleFunc("/api/policy", s.handlePolicy)
	mux.HandleFunc("/api/policy/sign", s.handlePolicySign)

	// Deletion requests
	mux.HandleFunc("/api/deletions", s.handleDeletions)
	mux.HandleFunc("/api/deletions/", s.handleDeletionByID)

	// Integrity verification
	mux.HandleFunc("/api/integrity/check", s.handleIntegrityCheck)
	mux.HandleFunc("/api/integrity/full", s.handleIntegrityFullCheck)
	mux.HandleFunc("/api/integrity/records", s.handleIntegrityRecords)
	mux.HandleFunc("/api/integrity/history", s.handleIntegrityHistory)
	mux.HandleFunc("/api/integrity/verification-config", s.handleVerificationConfig)
	mux.HandleFunc("/api/integrity/run-check", s.handleRunManualCheck)

	// Initialize and start storage server if configured
	if cfg.StoragePath != "" {
		storageServer, err := storage.NewServer(storage.Config{
			BasePath:   cfg.StoragePath,
			AppendOnly: cfg.StorageAppendOnly,
			QuotaBytes: cfg.StorageQuotaBytes,
		})
		if err != nil {
			logging.Warnf("failed to initialize storage server: %v", err)
		} else {
			s.storageServer = storageServer
			s.storageServer.Start() // Auto-start on server startup
			logging.Infof("Storage server started at /storage/ (path: %s)", cfg.StoragePath)
			// Mount storage server at /storage/
			mux.Handle("/storage/", http.StripPrefix("/storage", storage.WithLogging(s.storageServer.Handler())))

			// Initialize integrity checker
			integrityChecker, err := integrity.NewChecker(cfg.StoragePath)
			if err != nil {
				logging.Warnf("failed to initialize integrity checker: %v", err)
			} else {
				s.integrityChecker = integrityChecker
				logging.Info("Integrity checker initialized")
			}

			// Initialize managed scheduled checker for scheduled verification
			managedChecker, err := integrity.NewManagedScheduledChecker(cfg.StoragePath)
			if err != nil {
				logging.Warnf("failed to initialize scheduled checker: %v", err)
			} else {
				s.managedScheduledChecker = managedChecker
				// Start if enabled in config
				if err := s.managedScheduledChecker.Start(); err != nil {
					logging.Warnf("failed to start scheduled verification: %v", err)
				} else {
					verifyConfig := s.managedScheduledChecker.GetConfig()
					if verifyConfig.Enabled {
						logging.Infof("Scheduled verification started (interval: %s, type: %s)",
							verifyConfig.Interval, verifyConfig.CheckType)
					}
				}
			}
		}
	}

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      withLogging(withCORS(mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return s
}

// SetScheduler sets the backup scheduler
func (s *Server) SetScheduler(sched *scheduler.Scheduler) {
	s.statusSvc.SetScheduler(sched)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	logging.Infof("Starting Airgapper API server on %s", s.addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Response helpers

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: status < 400, Data: data})
}

func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: false, Error: message})
}

// --- Validation helpers ---

// Validator interface for request body validation
type Validator interface {
	Validate() error
}

// decodeAndValidate decodes JSON body and validates it
func decodeAndValidate(r *http.Request, v Validator) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid request body")
	}
	return v.Validate()
}

// requireMethod checks HTTP method and returns false if wrong
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

// requireStorageServer checks storage server exists
func (s *Server) requireStorageServer(w http.ResponseWriter) bool {
	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
		return false
	}
	return true
}

// requireIntegrityChecker checks integrity checker exists
func (s *Server) requireIntegrityChecker(w http.ResponseWriter) bool {
	if s.integrityChecker == nil {
		jsonError(w, http.StatusBadRequest, "integrity checker not configured")
		return false
	}
	return true
}

// requireScheduledChecker checks managed scheduled checker exists
func (s *Server) requireScheduledChecker(w http.ResponseWriter) bool {
	if s.managedScheduledChecker == nil {
		jsonError(w, http.StatusBadRequest, "scheduled verification not configured")
		return false
	}
	return true
}

// requireConsensus checks consensus mode is configured
func (s *Server) requireConsensus(w http.ResponseWriter) bool {
	if !s.vaultSvc.HasConsensus() {
		jsonError(w, http.StatusBadRequest, "consensus mode not configured")
		return false
	}
	return true
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

// required returns an error if value is empty
func required(field, value string) error {
	if value == "" {
		return ValidationError{Field: field, Message: field + " is required"}
	}
	return nil
}

// requiredInt returns an error if value is < min
func requiredInt(field string, value, min int) error {
	if value < min {
		return ValidationError{Field: field, Message: fmt.Sprintf("%s must be at least %d", field, min)}
	}
	return nil
}

// Handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	pending, _ := s.consentSvc.ListPendingRequests()
	sysStatus := s.statusSvc.GetSystemStatus(len(pending))

	// Build response from service data
	status := map[string]interface{}{
		"name":             sysStatus.Name,
		"role":             sysStatus.Role,
		"repo_url":         sysStatus.RepoURL,
		"has_share":        sysStatus.HasShare,
		"share_index":      sysStatus.ShareIndex,
		"pending_requests": sysStatus.PendingRequests,
		"backup_paths":     sysStatus.BackupPaths,
		"mode":             sysStatus.Mode,
	}

	if sysStatus.Peer != nil {
		status["peer"] = map[string]string{
			"name":    sysStatus.Peer.Name,
			"address": sysStatus.Peer.Address,
		}
	}

	if sysStatus.Consensus != nil {
		holders := make([]map[string]interface{}, len(sysStatus.Consensus.KeyHolders))
		for i, kh := range sysStatus.Consensus.KeyHolders {
			holders[i] = map[string]interface{}{
				"id":      kh.ID,
				"name":    kh.Name,
				"isOwner": kh.IsOwner,
			}
		}
		status["consensus"] = map[string]interface{}{
			"threshold":       sysStatus.Consensus.Threshold,
			"totalKeys":       sysStatus.Consensus.TotalKeys,
			"keyHolders":      holders,
			"requireApproval": sysStatus.Consensus.RequireApproval,
		}
	}

	if sysStatus.Scheduler != nil {
		status["scheduler"] = sysStatus.Scheduler
	}

	jsonResponse(w, http.StatusOK, status)
}

func (s *Server) handleRequests(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListRequests(w, r)
	case http.MethodPost:
		s.handleCreateRequest(w, r)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleListRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := s.consentSvc.ListPendingRequests()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to API format (hide sensitive share data)
	apiRequests := make([]map[string]interface{}, len(requests))
	for i, req := range requests {
		apiRequests[i] = map[string]interface{}{
			"id":          req.ID,
			"requester":   req.Requester,
			"snapshot_id": req.SnapshotID,
			"paths":       req.Paths,
			"reason":      req.Reason,
			"status":      req.Status,
			"created_at":  req.CreatedAt,
			"expires_at":  req.ExpiresAt,
		}
	}

	jsonResponse(w, http.StatusOK, apiRequests)
}

type CreateRequestBody struct {
	SnapshotID string   `json:"snapshot_id"`
	Paths      []string `json:"paths"`
	Reason     string   `json:"reason"`
}

func (b *CreateRequestBody) Validate() error {
	if err := required("reason", b.Reason); err != nil {
		return err
	}
	if b.SnapshotID == "" {
		b.SnapshotID = "latest"
	}
	return nil
}

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

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":         req.ID,
		"status":     req.Status,
		"expires_at": req.ExpiresAt,
	})
}

func (s *Server) handleRequestByID(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/requests/{id} or /api/requests/{id}/approve or /api/requests/{id}/deny
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		jsonError(w, http.StatusBadRequest, "request id required")
		return
	}

	id := parts[0]

	if len(parts) == 1 {
		// GET /api/requests/{id}
		if r.Method != http.MethodGet {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleGetRequest(w, r, id)
		return
	}

	if len(parts) == 2 {
		switch parts[1] {
		case "approve":
			if r.Method != http.MethodPost {
				jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			s.handleApprove(w, r, id)
			return
		case "deny":
			if r.Method != http.MethodPost {
				jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			s.handleDeny(w, r, id)
			return
		case "sign":
			s.handleSignRequest(w, r, id)
			return
		}
	}

	jsonError(w, http.StatusNotFound, "not found")
}

func (s *Server) handleGetRequest(w http.ResponseWriter, r *http.Request, id string) {
	req, err := s.consentSvc.GetRequest(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":          req.ID,
		"requester":   req.Requester,
		"snapshot_id": req.SnapshotID,
		"paths":       req.Paths,
		"reason":      req.Reason,
		"status":      req.Status,
		"created_at":  req.CreatedAt,
		"expires_at":  req.ExpiresAt,
		"approved_at": req.ApprovedAt,
		"approved_by": req.ApprovedBy,
	})
}

type ApproveBody struct {
	// Share is optional - if not provided, server uses its local share
	Share      []byte `json:"share,omitempty"`
	ShareIndex byte   `json:"share_index,omitempty"`
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request, id string) {
	var body ApproveBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Empty body is OK - we use local share
	}

	if err := s.consentSvc.ApproveRequest(id, body.Share); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "approved",
		"message": "Key share released",
	})
}

func (s *Server) handleDeny(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.consentSvc.DenyRequest(id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "denied"})
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// For now, just return a message about needing password
	// In production, this could list snapshots if we have the backup password
	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Snapshot listing requires restore approval",
	})
}

type ReceiveShareBody struct {
	Share      []byte `json:"share"`
	ShareIndex byte   `json:"share_index"`
	RepoURL    string `json:"repo_url"`
	PeerName   string `json:"peer_name"`
}

func (b *ReceiveShareBody) Validate() error {
	if b.Share == nil {
		return ValidationError{Field: "share", Message: "share is required"}
	}
	if err := required("repo_url", b.RepoURL); err != nil {
		return err
	}
	return required("peer_name", b.PeerName)
}

func (s *Server) handleReceiveShare(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body ReceiveShareBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.hostSvc.ReceiveShare(body.Share, body.ShareIndex, body.RepoURL, body.PeerName); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "received",
		"message": "Share stored successfully",
	})
}

func (s *Server) handleSchedule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		info := s.statusSvc.GetScheduleInfo()
		jsonResponse(w, http.StatusOK, info)

	case http.MethodPost:
		var body struct {
			Schedule string   `json:"schedule"`
			Paths    []string `json:"paths"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.Schedule != "" {
			if _, err := scheduler.ParseSchedule(body.Schedule); err != nil {
				jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid schedule: %v", err))
				return
			}
		}

		if err := s.statusSvc.UpdateSchedule(body.Schedule, body.Paths); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}

		jsonResponse(w, http.StatusOK, map[string]string{
			"status":  "updated",
			"message": "Schedule updated. Restart server to apply.",
		})

	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// Key Holder Handlers

func (s *Server) handleKeyHolders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListKeyHolders(w, r)
	case http.MethodPost:
		s.handleRegisterKeyHolder(w, r)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleListKeyHolders(w http.ResponseWriter, r *http.Request) {
	info, err := s.vaultSvc.GetConsensusInfo()
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Return key holders without private key data
	holders := make([]map[string]interface{}, len(info.KeyHolders))
	for i, kh := range info.KeyHolders {
		holders[i] = map[string]interface{}{
			"id":        kh.ID,
			"name":      kh.Name,
			"publicKey": crypto.EncodePublicKey(kh.PublicKey),
			"address":   kh.Address,
			"joinedAt":  kh.JoinedAt,
			"isOwner":   kh.IsOwner,
		}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"threshold":       info.Threshold,
		"totalKeys":       info.TotalKeys,
		"keyHolders":      holders,
		"requireApproval": info.RequireApproval,
	})
}

type RegisterKeyHolderBody struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"` // Hex encoded
	Address   string `json:"address,omitempty"`
}

func (b *RegisterKeyHolderBody) Validate() error {
	if err := required("name", b.Name); err != nil {
		return err
	}
	return required("publicKey", b.PublicKey)
}

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

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":       result.ID,
		"name":     result.Name,
		"joinedAt": result.JoinedAt,
	})
}

func (s *Server) handleKeyHolderByID(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	// Parse path: /api/keyholders/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/keyholders/")
	if id == "" {
		jsonError(w, http.StatusBadRequest, "key holder id required")
		return
	}

	holder, err := s.vaultSvc.GetKeyHolder(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":        holder.ID,
		"name":      holder.Name,
		"publicKey": crypto.EncodePublicKey(holder.PublicKey),
		"address":   holder.Address,
		"joinedAt":  holder.JoinedAt,
		"isOwner":   holder.IsOwner,
	})
}

// Vault initialization handler

type VaultInitBody struct {
	Name        string   `json:"name"`
	RepoURL     string   `json:"repoUrl"`
	Threshold   int      `json:"threshold"`   // m in m-of-n
	TotalKeys   int      `json:"totalKeys"`   // n in m-of-n
	BackupPaths []string `json:"backupPaths,omitempty"`
	RequireApproval bool `json:"requireApproval,omitempty"` // For 1/1 solo mode
}

func (b *VaultInitBody) Validate() error {
	if err := required("name", b.Name); err != nil {
		return err
	}
	if err := required("repoUrl", b.RepoURL); err != nil {
		return err
	}
	if err := requiredInt("threshold", b.Threshold, 1); err != nil {
		return err
	}
	if b.TotalKeys < b.Threshold {
		return ValidationError{Field: "totalKeys", Message: "totalKeys must be >= threshold"}
	}
	return nil
}

func (s *Server) handleVaultInit(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

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

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"name":      result.Name,
		"keyId":     result.KeyID,
		"publicKey": result.PublicKey,
		"threshold": result.Threshold,
		"totalKeys": result.TotalKeys,
	})
}

// Signature submission handler for request approval

type SignRequestBody struct {
	KeyHolderID string `json:"keyHolderId"`
	Signature   string `json:"signature"` // Hex encoded
}

func (b *SignRequestBody) Validate() error {
	if err := required("keyHolderId", b.KeyHolderID); err != nil {
		return err
	}
	return required("signature", b.Signature)
}

func (s *Server) handleSignRequest(w http.ResponseWriter, r *http.Request, id string) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body SignRequestBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Decode signature
	sigBytes, err := crypto.DecodePrivateKey(body.Signature) // Reusing decode function for hex
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

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":            "signature_added",
		"currentApprovals":  progress.Current,
		"requiredApprovals": progress.Required,
		"isApproved":        progress.IsApproved,
	})
}

// Network utilities

func (s *Server) handleLocalIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ip := getLocalIP()
	if ip == "" {
		ip = "127.0.0.1"
	}

	jsonResponse(w, http.StatusOK, map[string]string{"ip": ip})
}

// getLocalIP returns the server's local IP (method for convenience)
func (s *Server) getLocalIP() string {
	ip := getLocalIP()
	if ip == "" {
		return "localhost"
	}
	return ip
}

// getPort returns the server's port
func (s *Server) getPort() string {
	if s.addr == "" {
		return "8081"
	}
	parts := strings.Split(s.addr, ":")
	if len(parts) > 1 && parts[len(parts)-1] != "" {
		return parts[len(parts)-1]
	}
	return "8081"
}

// getLocalIP returns the best guess at the local IP address
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	// Prefer private IPs (192.168.x.x, 10.x.x.x, 172.16-31.x.x)
	var candidates []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				// Check for private IP ranges
				if strings.HasPrefix(ip, "192.168.") ||
					strings.HasPrefix(ip, "10.") ||
					strings.HasPrefix(ip, "172.") {
					candidates = append(candidates, ip)
				}
			}
		}
	}

	if len(candidates) > 0 {
		// Prefer 192.168.x.x if available
		for _, ip := range candidates {
			if strings.HasPrefix(ip, "192.168.") {
				return ip
			}
		}
		return candidates[0]
	}

	return ""
}

// Middleware

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logging.Debugf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Storage server handlers

func (s *Server) handleStorageStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	if s.storageServer == nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"configured": false,
			"running":    false,
		})
		return
	}

	status := s.storageServer.Status()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"configured":       true,
		"running":          status.Running,
		"startTime":        status.StartTime,
		"basePath":         status.BasePath,
		"appendOnly":       status.AppendOnly,
		"quotaBytes":       status.QuotaBytes,
		"usedBytes":        status.UsedBytes,
		"requestCount":     status.RequestCount,
		"hasPolicy":        status.HasPolicy,
		"policyId":         status.PolicyID,
		"maxDiskUsagePct":  status.MaxDiskUsagePct,
		"diskUsagePct":     status.DiskUsagePct,
		"diskFreeBytes":    status.DiskFreeBytes,
		"diskTotalBytes":   status.DiskTotalBytes,
	})
}

func (s *Server) handleStorageStart(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !s.requireStorageServer(w) {
		return
	}
	s.storageServer.Start()
	jsonResponse(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) handleStorageStop(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !s.requireStorageServer(w) {
		return
	}
	s.storageServer.Stop()
	jsonResponse(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// Host initialization handler

type HostInitBody struct {
	Name           string `json:"name"`
	StoragePath    string `json:"storagePath"`
	StorageQuota   int64  `json:"storageQuotaBytes,omitempty"`
	AppendOnly     bool   `json:"appendOnly"`
	RestoreApproval string `json:"restoreApproval"` // "both-required", "either", "owner-only", "host-only"
	RetentionDays  int    `json:"retentionDays,omitempty"`
}

func (b *HostInitBody) Validate() error {
	if err := required("name", b.Name); err != nil {
		return err
	}
	return required("storagePath", b.StoragePath)
}

func (s *Server) handleHostInit(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body HostInitBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Initialize host config via service
	result, err := s.hostSvc.Init(service.HostInitParams{
		Name:         body.Name,
		StoragePath:  body.StoragePath,
		StorageQuota: body.StorageQuota,
		AppendOnly:   body.AppendOnly,
	})
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Initialize storage server (runtime component stays in handler)
	storageServer, err := storage.NewServer(storage.Config{
		BasePath:   body.StoragePath,
		AppendOnly: body.AppendOnly,
		QuotaBytes: body.StorageQuota,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to initialize storage: %v", err))
		return
	}
	s.storageServer = storageServer
	s.hostSvc.SetStorageServer(storageServer)
	s.storageServer.Start()

	// Build storage URL
	storageURL := fmt.Sprintf("http://%s:%s/storage/", s.getLocalIP(), s.getPort())

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"name":        result.Name,
		"keyId":       result.KeyID,
		"publicKey":   result.PublicKey,
		"storageUrl":  storageURL,
		"storagePath": result.StoragePath,
	})
}

// ============================================================================
// Policy Handlers
// ============================================================================

func (s *Server) handlePolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetPolicy(w, r)
	case http.MethodPost:
		s.handleCreatePolicy(w, r)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}

	p := s.storageServer.GetPolicy()
	if p == nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"hasPolicy": false,
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"hasPolicy":        true,
		"policy":           p,
		"isFullySigned":    p.IsFullySigned(),
		"isActive":         p.IsActive(),
	})
}

type CreatePolicyBody struct {
	OwnerName     string `json:"ownerName"`
	OwnerKeyID    string `json:"ownerKeyId"`
	OwnerPubKey   string `json:"ownerPublicKey"`
	HostName      string `json:"hostName"`
	HostKeyID     string `json:"hostKeyId"`
	HostPubKey    string `json:"hostPublicKey"`
	RetentionDays int    `json:"retentionDays"`
	DeletionMode  string `json:"deletionMode"` // "both-required", "owner-only", "time-lock-only", "never"
	MaxStorageBytes int64 `json:"maxStorageBytes,omitempty"`
	// Signatures (optional - can be added later)
	OwnerSignature string `json:"ownerSignature,omitempty"`
	HostSignature  string `json:"hostSignature,omitempty"`
}

func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}

	var body CreatePolicyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if body.OwnerName == "" || body.OwnerKeyID == "" || body.OwnerPubKey == "" {
		jsonError(w, http.StatusBadRequest, "owner information required")
		return
	}
	if body.HostName == "" || body.HostKeyID == "" || body.HostPubKey == "" {
		jsonError(w, http.StatusBadRequest, "host information required")
		return
	}

	// Create policy
	p := policy.NewPolicy(
		body.OwnerName, body.OwnerKeyID, body.OwnerPubKey,
		body.HostName, body.HostKeyID, body.HostPubKey,
	)

	// Set terms
	if body.RetentionDays > 0 {
		p.RetentionDays = body.RetentionDays
	}
	if body.DeletionMode != "" {
		p.DeletionMode = policy.DeletionMode(body.DeletionMode)
	}
	if body.MaxStorageBytes > 0 {
		p.MaxStorageBytes = body.MaxStorageBytes
	}

	// Apply signatures if provided
	if body.OwnerSignature != "" {
		p.OwnerSignature = body.OwnerSignature
	}
	if body.HostSignature != "" {
		p.HostSignature = body.HostSignature
	}

	// If both signatures present, set the policy
	if p.IsFullySigned() {
		if err := s.storageServer.SetPolicy(p); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Return policy data for signing
	policyJSON, _ := p.ToJSON()
	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"policy":        p,
		"policyJSON":    string(policyJSON),
		"isFullySigned": p.IsFullySigned(),
	})
}

type PolicySignBody struct {
	PolicyJSON string `json:"policyJson"`
	Signature  string `json:"signature"`
	SignerRole string `json:"signerRole"` // "owner" or "host"
}

func (s *Server) handlePolicySign(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !s.requireStorageServer(w) {
		return
	}

	var body PolicySignBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Parse the policy
	p, err := policy.FromJSON([]byte(body.PolicyJSON))
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid policy JSON: %v", err))
		return
	}

	// Apply signature
	switch body.SignerRole {
	case "owner":
		p.OwnerSignature = body.Signature
	case "host":
		p.HostSignature = body.Signature
	default:
		jsonError(w, http.StatusBadRequest, "signerRole must be 'owner' or 'host'")
		return
	}

	// If both signatures present, verify and set
	if p.IsFullySigned() {
		if err := p.Verify(); err != nil {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("signature verification failed: %v", err))
			return
		}
		if err := s.storageServer.SetPolicy(p); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	policyJSON, _ := p.ToJSON()
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"policy":        p,
		"policyJSON":    string(policyJSON),
		"isFullySigned": p.IsFullySigned(),
	})
}

// ============================================================================
// Deletion Request Handlers
// ============================================================================

func (s *Server) handleDeletions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListDeletions(w, r)
	case http.MethodPost:
		s.handleCreateDeletion(w, r)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleListDeletions(w http.ResponseWriter, r *http.Request) {
	deletions, err := s.consentSvc.ListPendingDeletions()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	apiDeletions := make([]map[string]interface{}, len(deletions))
	for i, del := range deletions {
		apiDeletions[i] = map[string]interface{}{
			"id":                del.ID,
			"requester":         del.Requester,
			"deletionType":      del.DeletionType,
			"snapshotIds":       del.SnapshotIDs,
			"paths":             del.Paths,
			"reason":            del.Reason,
			"status":            del.Status,
			"createdAt":         del.CreatedAt,
			"expiresAt":         del.ExpiresAt,
			"requiredApprovals": del.RequiredApprovals,
			"currentApprovals":  len(del.Approvals),
		}
	}

	jsonResponse(w, http.StatusOK, apiDeletions)
}

type CreateDeletionBody struct {
	DeletionType      string   `json:"deletionType"` // "snapshot", "path", "prune", "all"
	SnapshotIDs       []string `json:"snapshotIds,omitempty"`
	Paths             []string `json:"paths,omitempty"`
	Reason            string   `json:"reason"`
	RequiredApprovals int      `json:"requiredApprovals"`
}

func (b *CreateDeletionBody) Validate() error {
	if err := required("reason", b.Reason); err != nil {
		return err
	}
	if err := required("deletionType", b.DeletionType); err != nil {
		return err
	}
	if b.RequiredApprovals < 1 {
		b.RequiredApprovals = 2 // Default to both parties
	}
	return nil
}

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

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":        del.ID,
		"status":    del.Status,
		"expiresAt": del.ExpiresAt,
	})
}

func (s *Server) handleDeletionByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/deletions/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		jsonError(w, http.StatusBadRequest, "deletion id required")
		return
	}

	id := parts[0]

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleGetDeletion(w, r, id)
		return
	}

	if len(parts) == 2 {
		switch parts[1] {
		case "approve":
			if r.Method != http.MethodPost {
				jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			s.handleApproveDeletion(w, r, id)
			return
		case "deny":
			if r.Method != http.MethodPost {
				jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			s.handleDenyDeletion(w, r, id)
			return
		}
	}

	jsonError(w, http.StatusNotFound, "not found")
}

func (s *Server) handleGetDeletion(w http.ResponseWriter, r *http.Request, id string) {
	del, err := s.consentSvc.GetDeletionRequest(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":                del.ID,
		"requester":         del.Requester,
		"deletionType":      del.DeletionType,
		"snapshotIds":       del.SnapshotIDs,
		"paths":             del.Paths,
		"reason":            del.Reason,
		"status":            del.Status,
		"createdAt":         del.CreatedAt,
		"expiresAt":         del.ExpiresAt,
		"approvedAt":        del.ApprovedAt,
		"executedAt":        del.ExecutedAt,
		"requiredApprovals": del.RequiredApprovals,
		"approvals":         del.Approvals,
	})
}

type ApproveDeletionBody struct {
	KeyHolderID string `json:"keyHolderId"`
	Signature   string `json:"signature"` // Hex encoded
}

func (b *ApproveDeletionBody) Validate() error {
	if err := required("keyHolderId", b.KeyHolderID); err != nil {
		return err
	}
	return required("signature", b.Signature)
}

func (s *Server) handleApproveDeletion(w http.ResponseWriter, r *http.Request, id string) {
	var body ApproveDeletionBody
	if err := decodeAndValidate(r, &body); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	sigBytes, err := hex.DecodeString(body.Signature)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}

	progress, err := s.consentSvc.ApproveDeletion(id, body.KeyHolderID, sigBytes)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":            "signature_added",
		"currentApprovals":  progress.Current,
		"requiredApprovals": progress.Required,
		"isApproved":        progress.IsApproved,
	})
}

func (s *Server) handleDenyDeletion(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.consentSvc.DenyDeletion(id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "denied"})
}

// ============================================================================
// Integrity Check Handler
// ============================================================================

func (s *Server) handleIntegrityCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) || !s.requireStorageServer(w) {
		return
	}

	status := s.storageServer.Status()

	response := map[string]interface{}{
		"status":         "ok",
		"storageRunning": status.Running,
		"usedBytes":      status.UsedBytes,
		"diskUsagePct":   status.DiskUsagePct,
		"diskFreeBytes":  status.DiskFreeBytes,
		"hasPolicy":      status.HasPolicy,
		"policyId":       status.PolicyID,
		"requestCount":   status.RequestCount,
	}

	// Include last integrity check if available
	if s.integrityChecker != nil {
		history := s.integrityChecker.GetHistory(1)
		if len(history) > 0 {
			lastCheck := history[0]
			response["lastCheck"] = map[string]interface{}{
				"timestamp":    lastCheck.Timestamp,
				"passed":       lastCheck.Passed,
				"totalFiles":   lastCheck.TotalFiles,
				"corruptFiles": lastCheck.CorruptFiles,
				"duration":     lastCheck.Duration,
			}
		}
	}

	jsonResponse(w, http.StatusOK, response)
}

// handleIntegrityFullCheck runs a full integrity check on all data
func (s *Server) handleIntegrityFullCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !s.requireIntegrityChecker(w) {
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

// handleIntegrityRecords manages verification records
func (s *Server) handleIntegrityRecords(w http.ResponseWriter, r *http.Request) {
	if !s.requireIntegrityChecker(w) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get a verification record
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

	case http.MethodPost:
		// Create a new verification record (unsigned - owner needs to sign)
		var body struct {
			RepoName   string `json:"repoName"`
			SnapshotID string `json:"snapshotId"`
			OwnerKeyID string `json:"ownerKeyId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
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

	case http.MethodPut:
		// Add a signed verification record
		var record integrity.VerificationRecord
		if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
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

		jsonResponse(w, http.StatusOK, map[string]string{
			"status":  "added",
			"message": "Verification record added and verified",
		})

	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleIntegrityHistory returns recent integrity check history
func (s *Server) handleIntegrityHistory(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) || !s.requireIntegrityChecker(w) {
		return
	}

	limit := 20 // Default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	history := s.integrityChecker.GetHistory(limit)
	jsonResponse(w, http.StatusOK, history)
}

// ============================================================================
// Scheduled Verification Configuration Handlers
// ============================================================================

// handleVerificationConfig manages scheduled verification settings
func (s *Server) handleVerificationConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireScheduledChecker(w) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleGetVerificationConfig(w, r)
	case http.MethodPost, http.MethodPut:
		s.handleUpdateVerificationConfig(w, r)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleGetVerificationConfig(w http.ResponseWriter, r *http.Request) {
	config := s.managedScheduledChecker.GetConfig()

	response := map[string]interface{}{
		"enabled":             config.Enabled,
		"interval":            config.Interval,
		"checkType":           config.CheckType,
		"repoName":            config.RepoName,
		"snapshotId":          config.SnapshotID,
		"alertOnCorruption":   config.AlertOnCorruption,
		"alertWebhook":        config.AlertWebhook,
		"consecutiveFailures": config.ConsecutiveFailures,
	}

	if config.LastCheck != nil {
		response["lastCheck"] = config.LastCheck
	}
	if config.LastResult != nil {
		response["lastResult"] = map[string]interface{}{
			"timestamp":    config.LastResult.Timestamp,
			"passed":       config.LastResult.Passed,
			"totalFiles":   config.LastResult.TotalFiles,
			"checkedFiles": config.LastResult.CheckedFiles,
			"corruptFiles": config.LastResult.CorruptFiles,
			"missingFiles": config.LastResult.MissingFiles,
			"duration":     config.LastResult.Duration,
			"errors":       config.LastResult.Errors,
		}
	}

	jsonResponse(w, http.StatusOK, response)
}

type UpdateVerificationConfigBody struct {
	Enabled           *bool   `json:"enabled,omitempty"`
	Interval          string  `json:"interval,omitempty"`
	CheckType         string  `json:"checkType,omitempty"`
	RepoName          string  `json:"repoName,omitempty"`
	SnapshotID        string  `json:"snapshotId,omitempty"`
	AlertOnCorruption *bool   `json:"alertOnCorruption,omitempty"`
	AlertWebhook      string  `json:"alertWebhook,omitempty"`
}

func (s *Server) handleUpdateVerificationConfig(w http.ResponseWriter, r *http.Request) {
	var body UpdateVerificationConfigBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
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

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "updated",
		"message": "Verification configuration updated",
		"config":  newConfig,
	})
}

// handleRunManualCheck triggers a manual integrity check
func (s *Server) handleRunManualCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) || !s.requireScheduledChecker(w) {
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

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":   "completed",
		"checkType": checkType,
		"result":   result,
	})
}
