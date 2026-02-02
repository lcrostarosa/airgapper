package api

import (
	"fmt"
	"net/http"

	"github.com/lcrostarosa/airgapper/backend/internal/service"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// handleHostInit initializes a host with storage
func (s *Server) handleHostInit(w http.ResponseWriter, r *http.Request) {
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

	jsonResponse(w, http.StatusCreated, HostInitResponseDTO{
		Name:        result.Name,
		KeyID:       result.KeyID,
		PublicKey:   result.PublicKey,
		StorageURL:  storageURL,
		StoragePath: result.StoragePath,
	})
}

// handleReceiveShare handles receiving a key share from the owner
func (s *Server) handleReceiveShare(w http.ResponseWriter, r *http.Request) {
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
