package api

import (
	"net/http"

	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// registerRoutes sets up all API routes using Go 1.22+ method-based routing
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health & status
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/status", s.handleStatus)

	// Restore requests
	mux.HandleFunc("GET /api/requests", s.handleListRequests)
	mux.HandleFunc("POST /api/requests", s.handleCreateRequest)
	mux.HandleFunc("GET /api/requests/{id}", s.handleGetRequest)
	mux.HandleFunc("POST /api/requests/{id}/approve", s.handleApproveRequest)
	mux.HandleFunc("POST /api/requests/{id}/deny", s.handleDenyRequest)
	mux.HandleFunc("POST /api/requests/{id}/sign", s.handleSignRequest)

	// Snapshots
	mux.HandleFunc("GET /api/snapshots", s.handleSnapshots)

	// Peer share exchange (legacy SSS mode)
	mux.HandleFunc("POST /api/share", s.handleReceiveShare)

	// Scheduler control
	mux.HandleFunc("GET /api/schedule", s.handleGetSchedule)
	mux.HandleFunc("POST /api/schedule", s.handleUpdateSchedule)
	mux.HandleFunc("GET /api/schedule/history", s.handleGetBackupHistory)

	// Key holder management (consensus mode)
	mux.HandleFunc("GET /api/keyholders", s.handleListKeyHolders)
	mux.HandleFunc("POST /api/keyholders", s.handleRegisterKeyHolder)
	mux.HandleFunc("GET /api/keyholders/{id}", s.handleGetKeyHolder)

	// Vault initialization (for UI)
	mux.HandleFunc("POST /api/vault/init", s.handleVaultInit)

	// Host initialization (for UI)
	mux.HandleFunc("POST /api/host/init", s.handleHostInit)

	// Storage server management (host only)
	mux.HandleFunc("GET /api/storage/status", s.handleStorageStatus)
	mux.HandleFunc("POST /api/storage/start", s.handleStorageStart)
	mux.HandleFunc("POST /api/storage/stop", s.handleStorageStop)

	// Network utilities
	mux.HandleFunc("GET /api/network/local-ip", s.handleLocalIP)

	// Policy management
	mux.HandleFunc("GET /api/policy", s.handleGetPolicy)
	mux.HandleFunc("POST /api/policy", s.handleCreatePolicy)
	mux.HandleFunc("POST /api/policy/sign", s.handlePolicySign)

	// Deletion requests
	mux.HandleFunc("GET /api/deletions", s.handleListDeletions)
	mux.HandleFunc("POST /api/deletions", s.handleCreateDeletion)
	mux.HandleFunc("GET /api/deletions/{id}", s.handleGetDeletion)
	mux.HandleFunc("POST /api/deletions/{id}/approve", s.handleApproveDeletion)
	mux.HandleFunc("POST /api/deletions/{id}/deny", s.handleDenyDeletion)

	// Integrity verification
	mux.HandleFunc("GET /api/integrity/check", s.handleIntegrityCheck)
	mux.HandleFunc("POST /api/integrity/full", s.handleIntegrityFullCheck)
	mux.HandleFunc("GET /api/integrity/records", s.handleGetIntegrityRecord)
	mux.HandleFunc("POST /api/integrity/records", s.handleCreateIntegrityRecord)
	mux.HandleFunc("PUT /api/integrity/records", s.handleAddIntegrityRecord)
	mux.HandleFunc("GET /api/integrity/history", s.handleIntegrityHistory)
	mux.HandleFunc("GET /api/integrity/verification-config", s.handleGetVerificationConfig)
	mux.HandleFunc("POST /api/integrity/verification-config", s.handleUpdateVerificationConfig)
	mux.HandleFunc("PUT /api/integrity/verification-config", s.handleUpdateVerificationConfig)
	mux.HandleFunc("POST /api/integrity/run-check", s.handleRunManualCheck)

	// Host verification system
	mux.HandleFunc("GET /api/verification/status", s.handleGetVerificationStatus)

	// Audit chain
	mux.HandleFunc("GET /api/audit/entries", s.handleGetAuditEntries)
	mux.HandleFunc("GET /api/audit/verify", s.handleVerifyAuditChain)
	mux.HandleFunc("GET /api/audit/export", s.handleExportAuditChain)

	// Challenge-response
	mux.HandleFunc("POST /api/challenge", s.handleCreateChallenge)
	mux.HandleFunc("POST /api/challenge/receive", s.handleReceiveChallenge)
	mux.HandleFunc("GET /api/challenge", s.handleListChallenges)
	mux.HandleFunc("POST /api/challenge/{id}/respond", s.handleRespondToChallenge)
	mux.HandleFunc("POST /api/challenge/verify", s.handleVerifyChallenge)

	// Deletion tickets
	mux.HandleFunc("POST /api/tickets", s.handleCreateTicket)
	mux.HandleFunc("POST /api/tickets/register", s.handleRegisterTicket)
	mux.HandleFunc("GET /api/tickets", s.handleListTickets)
	mux.HandleFunc("GET /api/tickets/{id}", s.handleGetTicket)
	mux.HandleFunc("GET /api/tickets/{id}/usage", s.handleGetTicketUsage)

	// Witness
	mux.HandleFunc("POST /api/witness/checkpoint", s.handleSubmitWitnessCheckpoint)
	mux.HandleFunc("POST /api/witness/checkpoint/create", s.handleCreateWitnessCheckpoint)
	mux.HandleFunc("GET /api/witness/checkpoint/{id}", s.handleGetWitnessCheckpoint)

	// Mount storage server if configured
	if s.storageServer != nil {
		mux.Handle("/storage/", http.StripPrefix("/storage", storage.WithLogging(s.storageServer.Handler())))
	}
}
