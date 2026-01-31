// Package api provides the HTTP control plane for Airgapper
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
)

// Server is the HTTP API server
type Server struct {
	cfg        *config.Config
	consentMgr *consent.Manager
	httpServer *http.Server
	scheduler  *scheduler.Scheduler
	addr       string
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

	// Peer share exchange
	mux.HandleFunc("/api/share", s.handleReceiveShare)

	// Scheduler control
	mux.HandleFunc("/api/schedule", s.handleSchedule)

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
	}

	if s.cfg.Peer != nil {
		status["peer"] = map[string]string{
			"name":    s.cfg.Peer.Name,
			"address": s.cfg.Peer.Address,
		}
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
