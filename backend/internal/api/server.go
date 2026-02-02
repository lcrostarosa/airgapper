// Package api provides the HTTP control plane for Airgapper
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// Server is the HTTP API server
type Server struct {
	httpServer              *http.Server
	storageServer           *storage.Server
	integrityChecker        *integrity.Checker
	managedScheduledChecker *integrity.ManagedScheduledChecker
	addr                    string

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
	return NewServerWithOptions(cfg, addr, nil)
}

// NewServerWithOptions creates a new API server with optional pre-initialized components
func NewServerWithOptions(cfg *config.Config, addr string, opts *ServerOptions) *Server {
	consentMgr := consent.NewManager(cfg.ConfigDir)

	s := &Server{
		cfg:        cfg, // Internal use only - handlers use services
		addr:       addr,
		vaultSvc:   service.NewVaultService(cfg),
		hostSvc:    service.NewHostService(cfg),
		consentSvc: service.NewConsentService(cfg, consentMgr),
		statusSvc:  service.NewStatusService(cfg),
	}

	// Apply pre-initialized components from options
	if opts != nil {
		s.storageServer = opts.StorageServer
		s.integrityChecker = opts.IntegrityChecker
		s.managedScheduledChecker = opts.ScheduledChecker
	}

	// Initialize storage components if not provided via options
	if s.storageServer == nil && cfg.StoragePath != "" {
		storageOpts, _ := InitStorageComponents(cfg)
		s.storageServer = storageOpts.StorageServer
		s.integrityChecker = storageOpts.IntegrityChecker
		s.managedScheduledChecker = storageOpts.ScheduledChecker

		// Auto-start storage components
		if s.storageServer != nil {
			StartStorageComponents(storageOpts)
			logging.Infof("Storage server started at /storage/ (path: %s)", cfg.StoragePath)
		}
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

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

// Addr returns the server's listen address
func (s *Server) Addr() string {
	return s.addr
}

// Handler returns the server's HTTP handler
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
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
