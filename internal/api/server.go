// Package api provides the HTTP control plane for Airgapper
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/consent"
)

// Server is the HTTP API server
type Server struct {
	cfg        *config.Config
	consentMgr *consent.Manager
	httpServer *http.Server
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
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/status", s.handleStatus)

	// Restore requests
	mux.HandleFunc("GET /api/requests", s.handleListRequests)
	mux.HandleFunc("POST /api/requests", s.handleCreateRequest)
	mux.HandleFunc("GET /api/requests/{id}", s.handleGetRequest)
	mux.HandleFunc("POST /api/requests/{id}/approve", s.handleApprove)
	mux.HandleFunc("POST /api/requests/{id}/deny", s.handleDeny)

	// Snapshots
	mux.HandleFunc("GET /api/snapshots", s.handleSnapshots)

	// Peer share exchange
	mux.HandleFunc("POST /api/share", s.handleReceiveShare)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      withLogging(withCORS(mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return s
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
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	pending, _ := s.consentMgr.ListPending()

	status := map[string]interface{}{
		"name":            s.cfg.Name,
		"role":            s.cfg.Role,
		"repo_url":        s.cfg.RepoURL,
		"has_share":       s.cfg.LocalShare != nil,
		"share_index":     s.cfg.ShareIndex,
		"pending_requests": len(pending),
	}

	if s.cfg.Peer != nil {
		status["peer"] = map[string]string{
			"name":    s.cfg.Peer.Name,
			"address": s.cfg.Peer.Address,
		}
	}

	jsonResponse(w, http.StatusOK, status)
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

func (s *Server) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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

type ApproveBody struct {
	// Share is optional - if not provided, server uses its local share
	Share      []byte `json:"share,omitempty"`
	ShareIndex byte   `json:"share_index,omitempty"`
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

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

func (s *Server) handleDeny(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.consentMgr.Deny(id, s.cfg.Name); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "denied"})
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
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
