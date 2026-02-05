// Package grpc provides Connect-RPC handlers for the Airgapper API.
// This package wraps the service layer and exposes it via Connect protocol,
// which supports HTTP/1.1, HTTP/2, and gRPC-compatible clients.
package grpc

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
	"github.com/lcrostarosa/airgapper/backend/internal/verification"
)

// Server wraps all Connect-RPC service handlers
type Server struct {
	// Services (business logic layer)
	vaultSvc   *service.VaultService
	hostSvc    *service.HostService
	consentSvc *service.ConsentService
	statusSvc  *service.StatusService

	// Infrastructure
	cfg                     *config.Config
	consentMgr              *consent.Manager
	storageServer           *storage.Server
	integrityChecker        *integrity.Checker
	managedScheduledChecker *integrity.ManagedScheduledChecker
	scheduler               *scheduler.Scheduler

	// Verification components
	auditChain      *verification.AuditChain
	ticketManager   *verification.TicketManager
	verificationCfg *verification.VerificationSystemConfig
}

// ServerOptions contains optional pre-initialized components
type ServerOptions struct {
	StorageServer    *storage.Server
	IntegrityChecker *integrity.Checker
	ScheduledChecker *integrity.ManagedScheduledChecker
	Scheduler        *scheduler.Scheduler

	// Verification components
	AuditChain      *verification.AuditChain
	TicketManager   *verification.TicketManager
	VerificationCfg *verification.VerificationSystemConfig
}

// NewServer creates a new Connect-RPC server with all service handlers
func NewServer(cfg *config.Config, opts *ServerOptions) *Server {
	consentMgr := consent.NewManager(cfg.ConfigDir)

	s := &Server{
		cfg:        cfg,
		consentMgr: consentMgr,
		vaultSvc:   service.NewVaultService(cfg),
		hostSvc:    service.NewHostService(cfg),
		consentSvc: service.NewConsentService(cfg, consentMgr),
		statusSvc:  service.NewStatusService(cfg),
	}

	if opts != nil {
		s.storageServer = opts.StorageServer
		s.integrityChecker = opts.IntegrityChecker
		s.managedScheduledChecker = opts.ScheduledChecker
		s.scheduler = opts.Scheduler

		// Verification components
		s.auditChain = opts.AuditChain
		s.ticketManager = opts.TicketManager
		s.verificationCfg = opts.VerificationCfg
	}

	return s
}

// SetScheduler sets the backup scheduler
func (s *Server) SetScheduler(sched *scheduler.Scheduler) {
	s.scheduler = sched
	s.statusSvc.SetScheduler(sched)
}

// RegisterHandlers registers all Connect-RPC handlers with the given mux.
// The prefix should typically be empty or "/" - handlers will be mounted at their
// canonical paths (e.g., /airgapper.v1.HealthService/Check).
func (s *Server) RegisterHandlers(mux *http.ServeMux) {
	// Create auth config from server config
	authConfig := &AuthConfig{
		APIKey:  s.cfg.APIKey,
		DevMode: s.cfg.DevMode,
	}

	// Create interceptors for logging, auth, error handling, etc.
	interceptors := connect.WithInterceptors(
		newLoggingInterceptor(),
		newAuthInterceptor(authConfig),
	)

	// Health service
	healthPath, healthHandler := airgapperv1connect.NewHealthServiceHandler(
		newHealthServer(s),
		interceptors,
	)
	mux.Handle(healthPath, healthHandler)

	// Vault service
	vaultPath, vaultHandler := airgapperv1connect.NewVaultServiceHandler(
		newVaultServer(s),
		interceptors,
	)
	mux.Handle(vaultPath, vaultHandler)

	// Host service
	hostPath, hostHandler := airgapperv1connect.NewHostServiceHandler(
		newHostServer(s),
		interceptors,
	)
	mux.Handle(hostPath, hostHandler)

	// Restore request service
	requestsPath, requestsHandler := airgapperv1connect.NewRestoreRequestServiceHandler(
		newRequestsServer(s),
		interceptors,
	)
	mux.Handle(requestsPath, requestsHandler)

	// Deletion service
	deletionsPath, deletionsHandler := airgapperv1connect.NewDeletionServiceHandler(
		newDeletionsServer(s),
		interceptors,
	)
	mux.Handle(deletionsPath, deletionsHandler)

	// Key holder service
	keyholdersPath, keyholdersHandler := airgapperv1connect.NewKeyHolderServiceHandler(
		newKeyHoldersServer(s),
		interceptors,
	)
	mux.Handle(keyholdersPath, keyholdersHandler)

	// Schedule service
	schedulePath, scheduleHandler := airgapperv1connect.NewScheduleServiceHandler(
		newScheduleServer(s),
		interceptors,
	)
	mux.Handle(schedulePath, scheduleHandler)

	// Storage service
	storagePath, storageHandler := airgapperv1connect.NewStorageServiceHandler(
		newStorageServer(s),
		interceptors,
	)
	mux.Handle(storagePath, storageHandler)

	// Policy service
	policyPath, policyHandler := airgapperv1connect.NewPolicyServiceHandler(
		newPolicyServer(s),
		interceptors,
	)
	mux.Handle(policyPath, policyHandler)

	// Integrity service
	integrityPath, integrityHandler := airgapperv1connect.NewIntegrityServiceHandler(
		newIntegrityServer(s),
		interceptors,
	)
	mux.Handle(integrityPath, integrityHandler)

	// Verification service
	verificationPath, verificationHandler := airgapperv1connect.NewVerificationServiceHandler(
		newVerificationServer(s),
		interceptors,
	)
	mux.Handle(verificationPath, verificationHandler)

	// Network service
	networkPath, networkHandler := airgapperv1connect.NewNetworkServiceHandler(
		newNetworkServer(s),
		interceptors,
	)
	mux.Handle(networkPath, networkHandler)
}

// StorageServer returns the storage server instance (may be nil)
func (s *Server) StorageServer() *storage.Server {
	return s.storageServer
}

// IntegrityChecker returns the integrity checker instance (may be nil)
func (s *Server) IntegrityChecker() *integrity.Checker {
	return s.integrityChecker
}

// ManagedScheduledChecker returns the scheduled checker instance (may be nil)
func (s *Server) ManagedScheduledChecker() *integrity.ManagedScheduledChecker {
	return s.managedScheduledChecker
}

// AuditChain returns the audit chain instance (may be nil)
func (s *Server) AuditChain() *verification.AuditChain {
	return s.auditChain
}

// TicketManager returns the ticket manager instance (may be nil)
func (s *Server) TicketManager() *verification.TicketManager {
	return s.ticketManager
}

// VerificationConfig returns the verification config (may be nil)
func (s *Server) VerificationConfig() *verification.VerificationSystemConfig {
	return s.verificationCfg
}
