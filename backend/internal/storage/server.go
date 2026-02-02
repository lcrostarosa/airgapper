// Package storage implements an embedded restic REST server
// This allows the Airgapper host to serve as a backup storage target
// without requiring a separate restic-rest-server installation.
package storage

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/policy"
	"github.com/lcrostarosa/airgapper/backend/internal/verification"
)

// timeNow is a variable for testing purposes
var timeNow = time.Now

// AuditEntry records a significant operation for audit trail
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Operation string    `json:"operation"` // CREATE, DELETE, POLICY_SET, etc.
	Path      string    `json:"path,omitempty"`
	Details   string    `json:"details,omitempty"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// DefaultMaxDiskUsagePct is the default max disk usage (95%)
const DefaultMaxDiskUsagePct = 95

// Server implements the restic REST server protocol
type Server struct {
	basePath        string
	appendOnly      bool
	quotaBytes      int64 // 0 = unlimited per-repo
	maxDiskUsagePct int   // Max system disk usage percentage
	mu              sync.RWMutex
	running         bool
	startTime       time.Time

	// Policy enforcement
	policy *policy.Policy

	// Audit logging (legacy)
	auditLog        []AuditEntry
	auditMu         sync.RWMutex
	maxAuditEntries int

	// Verification features (optional)
	verificationConfig *verification.VerificationSystemConfig
	auditChain         *verification.AuditChain
	ticketManager      *verification.TicketManager

	// Stats
	totalBytes   int64
	requestCount int64
}

// Config for creating a new storage server
type Config struct {
	BasePath        string
	AppendOnly      bool
	QuotaBytes      int64          // Per-repo quota (0 = unlimited)
	Policy          *policy.Policy // Optional policy for enforcement
	MaxDiskUsagePct int            // Max disk usage percentage (0 = use default 95%)

	// Verification features (optional)
	Verification   *verification.VerificationSystemConfig
	HostKeyID      string // Host key ID for signing audit entries
	HostPrivateKey []byte // Host private key for signatures
	HostPublicKey  []byte // Host public key for verification
	OwnerPublicKey []byte // Owner public key for ticket verification
}

// NewServer creates a new storage server
func NewServer(cfg Config) (*Server, error) {
	if cfg.BasePath == "" {
		return nil, fmt.Errorf("storage path is required")
	}

	// Ensure base path exists
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	maxDiskPct := cfg.MaxDiskUsagePct
	if maxDiskPct <= 0 || maxDiskPct > 100 {
		maxDiskPct = DefaultMaxDiskUsagePct
	}

	s := &Server{
		basePath:           cfg.BasePath,
		appendOnly:         cfg.AppendOnly,
		quotaBytes:         cfg.QuotaBytes,
		maxDiskUsagePct:    maxDiskPct,
		policy:             cfg.Policy,
		maxAuditEntries:    10000, // Keep last 10k audit entries
		verificationConfig: cfg.Verification,
	}

	// Load policy from disk if exists and not provided in config
	if s.policy == nil {
		s.loadPolicy()
	}

	// Load audit log from disk
	s.loadAuditLog()

	// Initialize verification features if enabled
	if err := s.initVerification(cfg); err != nil {
		logging.Warnf("[storage] verification initialization failed: %v", err)
	}

	return s, nil
}

// initVerification initializes verification features based on config.
func (s *Server) initVerification(cfg Config) error {
	if cfg.Verification == nil || !cfg.Verification.Enabled {
		return nil
	}

	verifyPath := filepath.Join(cfg.BasePath, ".airgapper-verification")

	// Initialize audit chain if enabled
	if cfg.Verification.IsAuditChainEnabled() {
		signEntries := cfg.Verification.AuditChain.SignEntries
		chain, err := verification.NewAuditChain(
			filepath.Join(verifyPath, "audit"),
			cfg.HostKeyID,
			cfg.HostPrivateKey,
			cfg.HostPublicKey,
			signEntries,
		)
		if err != nil {
			return fmt.Errorf("failed to initialize audit chain: %w", err)
		}
		s.auditChain = chain
		logging.Infof("[storage] Cryptographic audit chain enabled (signing: %v)", signEntries)
	}

	// Initialize ticket manager if enabled
	if cfg.Verification.IsTicketsEnabled() {
		validityDays := cfg.Verification.Tickets.ValidityDays
		if validityDays <= 0 {
			validityDays = 7
		}
		tm, err := verification.NewTicketManager(
			filepath.Join(verifyPath, "tickets"),
			cfg.OwnerPublicKey,
			cfg.HostPrivateKey,
			cfg.HostPublicKey,
			cfg.HostKeyID,
			validityDays,
		)
		if err != nil {
			return fmt.Errorf("failed to initialize ticket manager: %w", err)
		}
		s.ticketManager = tm
		logging.Infof("[storage] Deletion ticket system enabled (require for snapshots: %v)",
			cfg.Verification.Tickets.RequireForSnapshots)
	}

	return nil
}

// Handler returns an http.Handler for the storage server
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.handleRequest)
}

// Start marks the server as running
func (s *Server) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	s.startTime = time.Now()
}

// Stop marks the server as stopped
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}

// Status returns the current server status
type Status struct {
	Running         bool      `json:"running"`
	StartTime       time.Time `json:"startTime,omitempty"`
	BasePath        string    `json:"basePath"`
	AppendOnly      bool      `json:"appendOnly"`
	QuotaBytes      int64     `json:"quotaBytes,omitempty"`
	UsedBytes       int64     `json:"usedBytes"`
	RequestCount    int64     `json:"requestCount"`
	HasPolicy       bool      `json:"hasPolicy"`
	PolicyID        string    `json:"policyId,omitempty"`
	MaxDiskUsagePct int       `json:"maxDiskUsagePct"`
	DiskUsagePct    int       `json:"diskUsagePct"`
	DiskFreeBytes   int64     `json:"diskFreeBytes"`
	DiskTotalBytes  int64     `json:"diskTotalBytes"`
}

func (s *Server) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	used := s.calculateUsedSpace()
	diskTotal, diskFree, diskUsedPct := s.getDiskUsage()

	status := Status{
		Running:         s.running,
		StartTime:       s.startTime,
		BasePath:        s.basePath,
		AppendOnly:      s.appendOnly,
		QuotaBytes:      s.quotaBytes,
		UsedBytes:       used,
		RequestCount:    s.requestCount,
		HasPolicy:       s.policy != nil,
		MaxDiskUsagePct: s.maxDiskUsagePct,
		DiskUsagePct:    diskUsedPct,
		DiskFreeBytes:   diskFree,
		DiskTotalBytes:  diskTotal,
	}

	if s.policy != nil {
		status.PolicyID = s.policy.ID
	}

	return status
}

// --- Verification Component Accessors ---

// AuditChain returns the audit chain (may be nil if not enabled).
func (s *Server) AuditChain() *verification.AuditChain {
	return s.auditChain
}

// TicketManager returns the ticket manager (may be nil if not enabled).
func (s *Server) TicketManager() *verification.TicketManager {
	return s.ticketManager
}

// VerificationConfig returns the verification configuration (may be nil).
func (s *Server) VerificationConfig() *verification.VerificationSystemConfig {
	return s.verificationConfig
}

// RegisterTicket registers a deletion ticket with the storage server.
func (s *Server) RegisterTicket(ticket *verification.DeletionTicket) error {
	if s.ticketManager == nil {
		return fmt.Errorf("ticket system not enabled")
	}
	return s.ticketManager.RegisterTicket(ticket)
}

// Logging middleware for storage requests
func WithLogging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler.ServeHTTP(w, r)
		logging.Debugf("[storage] %s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}
