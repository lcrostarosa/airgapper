// Package api provides the HTTP control plane for Airgapper
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
	"github.com/lcrostarosa/airgapper/backend/internal/policy"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// Server is the HTTP API server
type Server struct {
	cfg                      *config.Config
	consentMgr               *consent.Manager
	httpServer               *http.Server
	scheduler                *scheduler.Scheduler
	storageServer            *storage.Server
	integrityChecker         *integrity.Checker
	managedScheduledChecker  *integrity.ManagedScheduledChecker
	addr                     string
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, addr string) *Server {
	s := &Server{
		cfg:        cfg,
		consentMgr: consent.NewManager(cfg.ConfigDir),
		addr:       addr,
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
			log.Printf("Warning: failed to initialize storage server: %v", err)
		} else {
			s.storageServer = storageServer
			s.storageServer.Start() // Auto-start on server startup
			log.Printf("Storage server started at /storage/ (path: %s)", cfg.StoragePath)
			// Mount storage server at /storage/
			mux.Handle("/storage/", http.StripPrefix("/storage", storage.WithLogging(s.storageServer.Handler())))

			// Initialize integrity checker
			integrityChecker, err := integrity.NewChecker(cfg.StoragePath)
			if err != nil {
				log.Printf("Warning: failed to initialize integrity checker: %v", err)
			} else {
				s.integrityChecker = integrityChecker
				log.Printf("Integrity checker initialized")
			}

			// Initialize managed scheduled checker for scheduled verification
			managedChecker, err := integrity.NewManagedScheduledChecker(cfg.StoragePath)
			if err != nil {
				log.Printf("Warning: failed to initialize scheduled checker: %v", err)
			} else {
				s.managedScheduledChecker = managedChecker
				// Start if enabled in config
				if err := s.managedScheduledChecker.Start(); err != nil {
					log.Printf("Warning: failed to start scheduled verification: %v", err)
				} else {
					verifyConfig := s.managedScheduledChecker.GetConfig()
					if verifyConfig.Enabled {
						log.Printf("Scheduled verification started (interval: %s, type: %s)",
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
	s.scheduler = sched
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting Airgapper API server on %s", s.addr)
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

// Handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	pending, _ := s.consentMgr.ListPending()

	status := map[string]interface{}{
		"name":             s.cfg.Name,
		"role":             s.cfg.Role,
		"repo_url":         s.cfg.RepoURL,
		"has_share":        s.cfg.LocalShare != nil,
		"share_index":      s.cfg.ShareIndex,
		"pending_requests": len(pending),
		"backup_paths":     s.cfg.BackupPaths,
	}

	if s.cfg.Peer != nil {
		status["peer"] = map[string]string{
			"name":    s.cfg.Peer.Name,
			"address": s.cfg.Peer.Address,
		}
	}

	// Add consensus info if configured
	if s.cfg.Consensus != nil {
		holders := make([]map[string]interface{}, len(s.cfg.Consensus.KeyHolders))
		for i, kh := range s.cfg.Consensus.KeyHolders {
			holders[i] = map[string]interface{}{
				"id":      kh.ID,
				"name":    kh.Name,
				"isOwner": kh.IsOwner,
			}
		}
		status["consensus"] = map[string]interface{}{
			"threshold":       s.cfg.Consensus.Threshold,
			"totalKeys":       s.cfg.Consensus.TotalKeys,
			"keyHolders":      holders,
			"requireApproval": s.cfg.Consensus.RequireApproval,
		}
		status["mode"] = "consensus"
	} else if s.cfg.LocalShare != nil {
		status["mode"] = "sss"
	} else {
		status["mode"] = "none"
	}

	// Add scheduler status if available
	if s.scheduler != nil {
		lastRun, lastErr, nextRun := s.scheduler.Status()
		schedStatus := map[string]interface{}{
			"enabled":  true,
			"schedule": s.cfg.BackupSchedule,
			"paths":    s.cfg.BackupPaths,
		}
		if !lastRun.IsZero() {
			schedStatus["last_run"] = lastRun
			if lastErr != nil {
				schedStatus["last_error"] = lastErr.Error()
			}
		}
		if !nextRun.IsZero() {
			schedStatus["next_run"] = nextRun
		}
		status["scheduler"] = schedStatus
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
	requests, err := s.consentMgr.ListPending()
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

func (s *Server) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	var body CreateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Reason == "" {
		jsonError(w, http.StatusBadRequest, "reason is required")
		return
	}

	if body.SnapshotID == "" {
		body.SnapshotID = "latest"
	}

	req, err := s.consentMgr.CreateRequest(s.cfg.Name, body.SnapshotID, body.Reason, body.Paths)
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
	req, err := s.consentMgr.GetRequest(id)
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

	share := body.Share
	if share == nil {
		// Use our local share
		localShare, _, err := s.cfg.LoadShare()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "no share available")
			return
		}
		share = localShare
	}

	if err := s.consentMgr.Approve(id, s.cfg.Name, share); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "approved",
		"message": "Key share released",
	})
}

func (s *Server) handleDeny(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.consentMgr.Deny(id, s.cfg.Name); err != nil {
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

func (s *Server) handleReceiveShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body ReceiveShareBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Share == nil || body.RepoURL == "" || body.PeerName == "" {
		jsonError(w, http.StatusBadRequest, "share, repo_url, and peer_name are required")
		return
	}

	// Store the share
	s.cfg.LocalShare = body.Share
	s.cfg.ShareIndex = body.ShareIndex
	s.cfg.RepoURL = body.RepoURL
	s.cfg.Peer = &config.PeerInfo{
		Name: body.PeerName,
	}

	if err := s.cfg.Save(); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
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
		// Get current schedule
		schedInfo := map[string]interface{}{
			"schedule": s.cfg.BackupSchedule,
			"paths":    s.cfg.BackupPaths,
			"enabled":  s.scheduler != nil,
		}
		if s.scheduler != nil {
			lastRun, lastErr, nextRun := s.scheduler.Status()
			if !lastRun.IsZero() {
				schedInfo["last_run"] = lastRun
				if lastErr != nil {
					schedInfo["last_error"] = lastErr.Error()
				}
			}
			if !nextRun.IsZero() {
				schedInfo["next_run"] = nextRun
			}
		}
		jsonResponse(w, http.StatusOK, schedInfo)

	case http.MethodPost:
		// Update schedule
		var body struct {
			Schedule string   `json:"schedule"`
			Paths    []string `json:"paths"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.Schedule != "" {
			// Validate schedule
			if _, err := scheduler.ParseSchedule(body.Schedule); err != nil {
				jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid schedule: %v", err))
				return
			}
		}

		if err := s.cfg.SetSchedule(body.Schedule, body.Paths); err != nil {
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
	if s.cfg.Consensus == nil {
		jsonError(w, http.StatusBadRequest, "consensus mode not configured")
		return
	}

	// Return key holders without private key data
	holders := make([]map[string]interface{}, len(s.cfg.Consensus.KeyHolders))
	for i, kh := range s.cfg.Consensus.KeyHolders {
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
		"threshold":   s.cfg.Consensus.Threshold,
		"totalKeys":   s.cfg.Consensus.TotalKeys,
		"keyHolders":  holders,
		"requireApproval": s.cfg.Consensus.RequireApproval,
	})
}

type RegisterKeyHolderBody struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"` // Hex encoded
	Address   string `json:"address,omitempty"`
}

func (s *Server) handleRegisterKeyHolder(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Consensus == nil {
		jsonError(w, http.StatusBadRequest, "consensus mode not configured")
		return
	}

	var body RegisterKeyHolderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.PublicKey == "" {
		jsonError(w, http.StatusBadRequest, "publicKey is required")
		return
	}

	// Decode public key
	pubKey, err := crypto.DecodePublicKey(body.PublicKey)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid public key: %v", err))
		return
	}

	// Check if we have room for more key holders
	if len(s.cfg.Consensus.KeyHolders) >= s.cfg.Consensus.TotalKeys {
		jsonError(w, http.StatusBadRequest, "maximum number of key holders reached")
		return
	}

	// Create key holder
	holder := config.KeyHolder{
		ID:        crypto.KeyID(pubKey),
		Name:      body.Name,
		PublicKey: pubKey,
		Address:   body.Address,
		JoinedAt:  time.Now(),
		IsOwner:   false,
	}

	if err := s.cfg.AddKeyHolder(holder); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":       holder.ID,
		"name":     holder.Name,
		"joinedAt": holder.JoinedAt,
	})
}

func (s *Server) handleKeyHolderByID(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/keyholders/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/keyholders/")
	if path == "" {
		jsonError(w, http.StatusBadRequest, "key holder id required")
		return
	}

	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	holder := s.cfg.GetKeyHolder(path)
	if holder == nil {
		jsonError(w, http.StatusNotFound, "key holder not found")
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

func (s *Server) handleVaultInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if already initialized
	if s.cfg.Name != "" {
		jsonError(w, http.StatusBadRequest, "vault already initialized")
		return
	}

	var body VaultInitBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.RepoURL == "" {
		jsonError(w, http.StatusBadRequest, "repoUrl is required")
		return
	}
	if body.Threshold < 1 {
		jsonError(w, http.StatusBadRequest, "threshold must be at least 1")
		return
	}
	if body.TotalKeys < body.Threshold {
		jsonError(w, http.StatusBadRequest, "totalKeys must be >= threshold")
		return
	}

	// Generate owner's key pair
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to generate keys: %v", err))
		return
	}

	// Generate repository password using crypto/rand
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to generate password")
		return
	}
	password := hex.EncodeToString(passwordBytes)
	s.cfg.Password = password

	// Set up config
	s.cfg.Name = body.Name
	s.cfg.Role = config.RoleOwner
	s.cfg.RepoURL = body.RepoURL
	s.cfg.PublicKey = pubKey
	s.cfg.PrivateKey = privKey
	s.cfg.BackupPaths = body.BackupPaths

	// Set up consensus
	ownerKeyHolder := config.KeyHolder{
		ID:        crypto.KeyID(pubKey),
		Name:      body.Name,
		PublicKey: pubKey,
		JoinedAt:  time.Now(),
		IsOwner:   true,
	}

	s.cfg.Consensus = &config.ConsensusConfig{
		Threshold:       body.Threshold,
		TotalKeys:       body.TotalKeys,
		KeyHolders:      []config.KeyHolder{ownerKeyHolder},
		RequireApproval: body.RequireApproval,
	}

	if err := s.cfg.Save(); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"name":      s.cfg.Name,
		"keyId":     ownerKeyHolder.ID,
		"publicKey": crypto.EncodePublicKey(pubKey),
		"threshold": body.Threshold,
		"totalKeys": body.TotalKeys,
	})
}

// Signature submission handler for request approval

type SignRequestBody struct {
	KeyHolderID string `json:"keyHolderId"`
	Signature   string `json:"signature"` // Hex encoded
}

func (s *Server) handleSignRequest(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body SignRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.KeyHolderID == "" {
		jsonError(w, http.StatusBadRequest, "keyHolderId is required")
		return
	}
	if body.Signature == "" {
		jsonError(w, http.StatusBadRequest, "signature is required")
		return
	}

	// Verify key holder exists
	holder := s.cfg.GetKeyHolder(body.KeyHolderID)
	if holder == nil {
		jsonError(w, http.StatusBadRequest, "unknown key holder")
		return
	}

	// Get the request
	req, err := s.consentMgr.GetRequest(id)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	// Decode signature
	sigBytes, err := crypto.DecodePrivateKey(body.Signature) // Reusing decode function for hex
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}

	// Verify signature
	valid, err := crypto.VerifyRestoreRequestSignature(
		holder.PublicKey,
		sigBytes,
		req.ID,
		req.Requester,
		req.SnapshotID,
		req.Reason,
		body.KeyHolderID,
		req.Paths,
		req.CreatedAt.Unix(),
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("verification error: %v", err))
		return
	}
	if !valid {
		jsonError(w, http.StatusBadRequest, "invalid signature")
		return
	}

	// Add the signature
	if err := s.consentMgr.AddSignature(id, body.KeyHolderID, holder.Name, sigBytes); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check approval status
	current, required, _ := s.consentMgr.GetApprovalProgress(id)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":           "signature_added",
		"currentApprovals": current,
		"requiredApprovals": required,
		"isApproved":       current >= required,
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
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
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
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
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
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
		return
	}

	s.storageServer.Start()
	jsonResponse(w, http.StatusOK, map[string]string{
		"status": "started",
	})
}

func (s *Server) handleStorageStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
		return
	}

	s.storageServer.Stop()
	jsonResponse(w, http.StatusOK, map[string]string{
		"status": "stopped",
	})
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

func (s *Server) handleHostInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if already initialized
	if s.cfg.Name != "" {
		jsonError(w, http.StatusBadRequest, "host already initialized")
		return
	}

	var body HostInitBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.StoragePath == "" {
		jsonError(w, http.StatusBadRequest, "storagePath is required")
		return
	}

	// Generate host's key pair
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to generate keys: %v", err))
		return
	}

	// Set up config
	s.cfg.Name = body.Name
	s.cfg.Role = config.RoleHost
	s.cfg.PublicKey = pubKey
	s.cfg.PrivateKey = privKey
	s.cfg.StoragePath = body.StoragePath
	s.cfg.StorageQuotaBytes = body.StorageQuota
	s.cfg.StorageAppendOnly = body.AppendOnly

	if err := s.cfg.Save(); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
		return
	}

	// Initialize storage server
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
	s.storageServer.Start()

	// Get local IP for the storage URL
	localIP := getLocalIP()
	if localIP == "" {
		localIP = "localhost"
	}

	// Parse the port from the server address
	port := "8081"
	if s.addr != "" {
		parts := strings.Split(s.addr, ":")
		if len(parts) > 1 && parts[len(parts)-1] != "" {
			port = parts[len(parts)-1]
		}
	}

	storageURL := fmt.Sprintf("http://%s:%s/storage/", localIP, port)

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"name":        s.cfg.Name,
		"keyId":       crypto.KeyID(pubKey),
		"publicKey":   crypto.EncodePublicKey(pubKey),
		"storageUrl":  storageURL,
		"storagePath": body.StoragePath,
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
	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
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
	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
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
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
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
	deletions, err := s.consentMgr.ListPendingDeletions()
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

func (s *Server) handleCreateDeletion(w http.ResponseWriter, r *http.Request) {
	var body CreateDeletionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Reason == "" {
		jsonError(w, http.StatusBadRequest, "reason is required")
		return
	}
	if body.DeletionType == "" {
		jsonError(w, http.StatusBadRequest, "deletionType is required")
		return
	}
	if body.RequiredApprovals < 1 {
		body.RequiredApprovals = 2 // Default to both parties
	}

	del, err := s.consentMgr.CreateDeletionRequest(
		s.cfg.Name,
		consent.DeletionType(body.DeletionType),
		body.SnapshotIDs,
		body.Paths,
		body.Reason,
		body.RequiredApprovals,
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":         del.ID,
		"status":     del.Status,
		"expiresAt":  del.ExpiresAt,
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
	del, err := s.consentMgr.GetDeletionRequest(id)
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

func (s *Server) handleApproveDeletion(w http.ResponseWriter, r *http.Request, id string) {
	var body ApproveDeletionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.KeyHolderID == "" {
		jsonError(w, http.StatusBadRequest, "keyHolderId is required")
		return
	}
	if body.Signature == "" {
		jsonError(w, http.StatusBadRequest, "signature is required")
		return
	}

	sigBytes, err := hex.DecodeString(body.Signature)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid signature encoding")
		return
	}

	// Get key holder name
	keyHolderName := body.KeyHolderID
	if s.cfg.Consensus != nil {
		for _, kh := range s.cfg.Consensus.KeyHolders {
			if kh.ID == body.KeyHolderID {
				keyHolderName = kh.Name
				break
			}
		}
	}

	if err := s.consentMgr.ApproveDeletion(id, body.KeyHolderID, keyHolderName, sigBytes); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	current, required, _ := s.consentMgr.GetDeletionApprovalProgress(id)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":            "signature_added",
		"currentApprovals":  current,
		"requiredApprovals": required,
		"isApproved":        current >= required,
	})
}

func (s *Server) handleDenyDeletion(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.consentMgr.DenyDeletion(id, s.cfg.Name); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "denied"})
}

// ============================================================================
// Integrity Check Handler
// ============================================================================

func (s *Server) handleIntegrityCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
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
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.integrityChecker == nil {
		jsonError(w, http.StatusBadRequest, "integrity checker not configured")
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
	if s.integrityChecker == nil {
		jsonError(w, http.StatusBadRequest, "integrity checker not configured")
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
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.integrityChecker == nil {
		jsonError(w, http.StatusBadRequest, "integrity checker not configured")
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
	if s.managedScheduledChecker == nil {
		jsonError(w, http.StatusBadRequest, "scheduled verification not configured")
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
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if s.managedScheduledChecker == nil {
		jsonError(w, http.StatusBadRequest, "scheduled verification not configured")
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
