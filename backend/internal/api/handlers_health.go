package api

import (
	"net/http"
)

// handleHealth returns a simple health check response
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleStatus returns the current system status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	pending, _ := s.consentSvc.ListPendingRequests()
	sysStatus := s.statusSvc.GetSystemStatus(len(pending))

	// Build response from service data
	status := SystemStatusDTO{
		Name:            sysStatus.Name,
		Role:            sysStatus.Role,
		RepoURL:         sysStatus.RepoURL,
		HasShare:        sysStatus.HasShare,
		ShareIndex:      int(sysStatus.ShareIndex),
		PendingRequests: sysStatus.PendingRequests,
		BackupPaths:     sysStatus.BackupPaths,
		Mode:            sysStatus.Mode,
	}

	if sysStatus.Peer != nil {
		status.Peer = &PeerDTO{
			Name:    sysStatus.Peer.Name,
			Address: sysStatus.Peer.Address,
		}
	}

	if sysStatus.Consensus != nil {
		holders := make([]ConsensusKeyHolderDTO, len(sysStatus.Consensus.KeyHolders))
		for i, kh := range sysStatus.Consensus.KeyHolders {
			holders[i] = ConsensusKeyHolderDTO{
				ID:      kh.ID,
				Name:    kh.Name,
				IsOwner: kh.IsOwner,
			}
		}
		status.Consensus = &ConsensusDTO{
			Threshold:       sysStatus.Consensus.Threshold,
			TotalKeys:       sysStatus.Consensus.TotalKeys,
			KeyHolders:      holders,
			RequireApproval: sysStatus.Consensus.RequireApproval,
		}
	}

	if sysStatus.Scheduler != nil {
		status.Scheduler = sysStatus.Scheduler
	}

	jsonResponse(w, http.StatusOK, status)
}

// handleSnapshots returns snapshot information
func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	// For now, just return a message about needing password
	// In production, this could list snapshots if we have the backup password
	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Snapshot listing requires restore approval",
	})
}
