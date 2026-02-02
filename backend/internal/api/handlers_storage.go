package api

import (
	"net/http"
)

// handleStorageStatus returns the storage server status
func (s *Server) handleStorageStatus(w http.ResponseWriter, r *http.Request) {
	if s.storageServer == nil {
		jsonResponse(w, http.StatusOK, StorageStatusDTO{
			Configured: false,
			Running:    false,
		})
		return
	}

	status := s.storageServer.Status()
	jsonResponse(w, http.StatusOK, ToStorageStatusDTO(status))
}

// handleStorageStart starts the storage server
func (s *Server) handleStorageStart(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}
	s.storageServer.Start()
	jsonResponse(w, http.StatusOK, map[string]string{"status": "started"})
}

// handleStorageStop stops the storage server
func (s *Server) handleStorageStop(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}
	s.storageServer.Stop()
	jsonResponse(w, http.StatusOK, map[string]string{"status": "stopped"})
}
