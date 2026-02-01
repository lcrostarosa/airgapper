// Package storage implements an embedded restic REST server
// This allows the Airgapper host to serve as a backup storage target
// without requiring a separate restic-rest-server installation.
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/policy"
)

// Valid restic file types
var validTypes = map[string]bool{
	"data":      true,
	"keys":      true,
	"locks":     true,
	"snapshots": true,
	"index":     true,
}

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

	// Audit logging
	auditLog        []AuditEntry
	auditMu         sync.RWMutex
	maxAuditEntries int

	// Stats
	totalBytes   int64
	requestCount int64
}

// Config for creating a new storage server
type Config struct {
	BasePath         string
	AppendOnly       bool
	QuotaBytes       int64          // Per-repo quota (0 = unlimited)
	Policy           *policy.Policy // Optional policy for enforcement
	MaxDiskUsagePct  int            // Max disk usage percentage (0 = use default 95%)
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
		basePath:        cfg.BasePath,
		appendOnly:      cfg.AppendOnly,
		quotaBytes:      cfg.QuotaBytes,
		maxDiskUsagePct: maxDiskPct,
		policy:          cfg.Policy,
		maxAuditEntries: 10000, // Keep last 10k audit entries
	}

	// Load policy from disk if exists and not provided in config
	if s.policy == nil {
		s.loadPolicy()
	}

	// Load audit log from disk
	s.loadAuditLog()

	return s, nil
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

func (s *Server) calculateUsedSpace() int64 {
	var total int64
	filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// getDiskUsage returns total bytes, free bytes, and usage percentage for the disk
func (s *Server) getDiskUsage() (total int64, free int64, usedPct int) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.basePath, &stat); err != nil {
		return 0, 0, 0
	}

	total = int64(stat.Blocks) * int64(stat.Bsize)
	free = int64(stat.Bavail) * int64(stat.Bsize)
	used := total - free

	if total > 0 {
		usedPct = int((used * 100) / total)
	}

	return total, free, usedPct
}

// checkDiskSpace returns true if there's enough disk space for the write
func (s *Server) checkDiskSpace(bytesToWrite int64) (bool, string) {
	_, free, usedPct := s.getDiskUsage()

	// Check if write would exceed max disk usage
	if usedPct >= s.maxDiskUsagePct {
		return false, fmt.Sprintf("disk usage at %d%% (max %d%%)", usedPct, s.maxDiskUsagePct)
	}

	// Check if there's enough free space (with some buffer)
	minFreeBytes := int64(100 * 1024 * 1024) // 100MB minimum
	if free-bytesToWrite < minFreeBytes {
		return false, fmt.Sprintf("insufficient disk space: %d bytes free, need %d", free, bytesToWrite+minFreeBytes)
	}

	return true, ""
}

// Policy management

func (s *Server) policyPath() string {
	return filepath.Join(s.basePath, ".airgapper-policy.json")
}

func (s *Server) auditLogPath() string {
	return filepath.Join(s.basePath, ".airgapper-audit.json")
}

// loadPolicy loads the policy from disk if it exists
func (s *Server) loadPolicy() {
	data, err := os.ReadFile(s.policyPath())
	if err != nil {
		return // Policy doesn't exist yet
	}

	p, err := policy.FromJSON(data)
	if err != nil {
		log.Printf("[storage] Warning: failed to parse policy: %v", err)
		return
	}

	// Verify the policy signatures
	if err := p.Verify(); err != nil {
		log.Printf("[storage] Warning: policy signature invalid: %v", err)
		return
	}

	s.policy = p
	log.Printf("[storage] Loaded policy %s (retention: %d days, deletion: %s)",
		p.ID, p.RetentionDays, p.DeletionMode)
}

// SetPolicy sets and persists the policy
// The policy must be fully signed by both parties
func (s *Server) SetPolicy(p *policy.Policy) error {
	if p == nil {
		return fmt.Errorf("policy cannot be nil")
	}

	// Verify signatures
	if err := p.Verify(); err != nil {
		return fmt.Errorf("policy verification failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// If we already have a policy, check if it can be replaced
	if s.policy != nil {
		// Policy can only be replaced if both old and new are signed by same parties
		if s.policy.OwnerKeyID != p.OwnerKeyID || s.policy.HostKeyID != p.HostKeyID {
			return fmt.Errorf("policy can only be replaced by same parties")
		}
	}

	// Persist to disk
	data, err := p.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize policy: %w", err)
	}

	if err := os.WriteFile(s.policyPath(), data, 0600); err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	s.policy = p

	// Log the policy change
	s.audit("POLICY_SET", "", fmt.Sprintf("Policy %s set (retention: %d days)", p.ID, p.RetentionDays), true, "")

	// If policy locks append-only mode, enforce it
	if p.AppendOnlyLocked {
		s.appendOnly = true
	}

	return nil
}

// GetPolicy returns the current policy
func (s *Server) GetPolicy() *policy.Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policy
}

// Audit logging

func (s *Server) loadAuditLog() {
	data, err := os.ReadFile(s.auditLogPath())
	if err != nil {
		return
	}

	var entries []AuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Printf("[storage] Warning: failed to parse audit log: %v", err)
		return
	}

	s.auditLog = entries
}

func (s *Server) saveAuditLog() {
	data, err := json.MarshalIndent(s.auditLog, "", "  ")
	if err != nil {
		log.Printf("[storage] Warning: failed to serialize audit log: %v", err)
		return
	}

	if err := os.WriteFile(s.auditLogPath(), data, 0600); err != nil {
		log.Printf("[storage] Warning: failed to save audit log: %v", err)
	}
}

func (s *Server) audit(operation, path, details string, success bool, errMsg string) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	entry := AuditEntry{
		Timestamp: time.Now(),
		Operation: operation,
		Path:      path,
		Details:   details,
		Success:   success,
		Error:     errMsg,
	}

	s.auditLog = append(s.auditLog, entry)

	// Trim if too large
	if len(s.auditLog) > s.maxAuditEntries {
		s.auditLog = s.auditLog[len(s.auditLog)-s.maxAuditEntries:]
	}

	// Persist
	s.saveAuditLog()

	// Also log to stdout
	if success {
		log.Printf("[storage-audit] %s %s %s", operation, path, details)
	} else {
		log.Printf("[storage-audit] %s %s FAILED: %s", operation, path, errMsg)
	}
}

// GetAuditLog returns the audit log entries
func (s *Server) GetAuditLog(limit int) []AuditEntry {
	s.auditMu.RLock()
	defer s.auditMu.RUnlock()

	if limit <= 0 || limit > len(s.auditLog) {
		limit = len(s.auditLog)
	}

	// Return most recent entries
	start := len(s.auditLog) - limit
	result := make([]AuditEntry, limit)
	copy(result, s.auditLog[start:])
	return result
}

// checkDeleteAllowed checks if deletion is allowed based on policy
func (s *Server) checkDeleteAllowed(filePath string) (bool, string) {
	// Always check append-only first
	if s.appendOnly {
		return false, "delete not allowed in append-only mode"
	}

	// If no policy, allow (legacy behavior)
	if s.policy == nil {
		return true, ""
	}

	// Get file creation time
	info, err := os.Stat(filePath)
	if err != nil {
		return false, "file not found"
	}

	// Use modification time as proxy for creation time
	// (Note: Go doesn't have a portable way to get creation time)
	fileTime := info.ModTime()

	return s.policy.CanDelete(fileTime)
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requestCount++
	running := s.running
	s.mu.Unlock()

	if !running {
		http.Error(w, "Storage server not running", http.StatusServiceUnavailable)
		return
	}

	// Parse the path: /{repo}/{type}/{name} or /{repo}/{type}/ or /{repo}/config
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "Repository name required", http.StatusBadRequest)
		return
	}

	repo := parts[0]

	// Validate repo name (alphanumeric, hyphens, underscores only)
	if !isValidRepoName(repo) {
		http.Error(w, "Invalid repository name", http.StatusBadRequest)
		return
	}

	// Handle different path patterns
	if len(parts) == 1 || (len(parts) == 2 && parts[1] == "") {
		// /{repo}/ - Repository root
		s.handleRepo(w, r, repo)
		return
	}

	if parts[1] == "config" {
		// /{repo}/config - Repository config file
		s.handleConfig(w, r, repo)
		return
	}

	fileType := parts[1]
	if !validTypes[fileType] {
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 || (len(parts) == 3 && parts[2] == "") {
		// /{repo}/{type}/ - List files
		s.handleList(w, r, repo, fileType)
		return
	}

	// /{repo}/{type}/{name} - Individual file
	fileName := parts[2]
	s.handleFile(w, r, repo, fileType, fileName)
}

func (s *Server) handleRepo(w http.ResponseWriter, r *http.Request, repo string) {
	repoPath := filepath.Join(s.basePath, repo)

	switch r.Method {
	case http.MethodPost:
		// Create repository
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			http.Error(w, "Failed to create repository", http.StatusInternalServerError)
			return
		}
		// Create subdirectories
		for fileType := range validTypes {
			if err := os.MkdirAll(filepath.Join(repoPath, fileType), 0755); err != nil {
				http.Error(w, "Failed to create repository", http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodHead:
		// Check if repo exists
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			http.Error(w, "Repository not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request, repo string) {
	repoPath := filepath.Join(s.basePath, repo)
	configPath := filepath.Join(repoPath, "config")

	switch r.Method {
	case http.MethodHead:
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			http.Error(w, "Config not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		data, err := os.ReadFile(configPath)
		if os.IsNotExist(err) {
			http.Error(w, "Config not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to read config", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(data)

	case http.MethodPost:
		// Config can only be created once
		if _, err := os.Stat(configPath); err == nil {
			http.Error(w, "Config already exists", http.StatusForbidden)
			return
		}

		// Ensure repo directory exists
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			http.Error(w, "Failed to create repository", http.StatusInternalServerError)
			return
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			http.Error(w, "Failed to write config", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		allowed, reason := s.checkDeleteAllowed(configPath)
		if !allowed {
			s.audit("DELETE_DENIED", configPath, reason, false, reason)
			http.Error(w, reason, http.StatusForbidden)
			return
		}
		if err := os.Remove(configPath); err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Config not found", http.StatusNotFound)
				return
			}
			s.audit("DELETE", configPath, "", false, err.Error())
			http.Error(w, "Failed to delete config", http.StatusInternalServerError)
			return
		}
		s.audit("DELETE", configPath, "config deleted", true, "")
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request, repo, fileType string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dirPath := filepath.Join(s.basePath, repo, fileType)

	// For data directory, we need to look in subdirectories
	type fileEntry struct {
		name string
		size int64
	}
	var files []fileEntry

	if fileType == "data" {
		// Data files are stored in subdirectories by first 2 chars of hash
		filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				files = append(files, fileEntry{name: info.Name(), size: info.Size()})
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(dirPath)
		if os.IsNotExist(err) {
			// Return empty list for missing directory
			w.Header().Set("Content-Type", "application/vnd.x.restic.rest.v2")
			fmt.Fprint(w, "[]")
			return
		}
		if err != nil {
			http.Error(w, "Failed to list directory", http.StatusInternalServerError)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileEntry{name: entry.Name(), size: info.Size()})
		}
	}

	// Build JSON response
	w.Header().Set("Content-Type", "application/vnd.x.restic.rest.v2")
	fmt.Fprint(w, "[")
	for i, f := range files {
		if i > 0 {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, `{"name":%q,"size":%d}`, f.name, f.size)
	}
	fmt.Fprint(w, "]")
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request, repo, fileType, fileName string) {
	// Validate filename (should be hex for most types)
	if !isValidFileName(fileName) {
		http.Error(w, "Invalid file name", http.StatusBadRequest)
		return
	}

	// For data files, use subdirectory structure (first 2 chars)
	var filePath string
	if fileType == "data" && len(fileName) >= 2 {
		subdir := fileName[:2]
		filePath = filepath.Join(s.basePath, repo, fileType, subdir, fileName)
	} else {
		filePath = filepath.Join(s.basePath, repo, fileType, fileName)
	}

	switch r.Method {
	case http.MethodHead:
		info, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to stat file", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		file, err := os.Open(filePath)
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to open file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		info, _ := file.Stat()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		io.Copy(w, file)

	case http.MethodPost:
		contentLength := r.ContentLength
		if contentLength < 0 {
			contentLength = 0 // Unknown size
		}

		// Check system disk space first
		if ok, reason := s.checkDiskSpace(contentLength); !ok {
			s.audit("WRITE_DENIED", filePath, reason, false, reason)
			http.Error(w, reason, http.StatusInsufficientStorage)
			return
		}

		// Check per-repo quota
		if s.quotaBytes > 0 && contentLength > 0 {
			currentUsed := s.calculateUsedSpace()
			if currentUsed+contentLength > s.quotaBytes {
				http.Error(w, "Storage quota exceeded", http.StatusInsufficientStorage)
				return
			}
		}

		// Ensure directory exists
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}

		// Write to temp file first, then rename (atomic write)
		tmpPath := filePath + ".tmp"
		file, err := os.Create(tmpPath)
		if err != nil {
			http.Error(w, "Failed to create file", http.StatusInternalServerError)
			return
		}

		hash := sha256.New()
		written, err := io.Copy(io.MultiWriter(file, hash), r.Body)
		file.Close()

		if err != nil {
			os.Remove(tmpPath)
			http.Error(w, "Failed to write file", http.StatusInternalServerError)
			return
		}

		// For data blobs, verify the hash matches the filename
		if fileType == "data" {
			expectedHash := fileName
			actualHash := hex.EncodeToString(hash.Sum(nil))
			if actualHash != expectedHash {
				os.Remove(tmpPath)
				http.Error(w, "Hash mismatch", http.StatusBadRequest)
				return
			}
		}

		// Rename temp file to final name
		if err := os.Rename(tmpPath, filePath); err != nil {
			os.Remove(tmpPath)
			http.Error(w, "Failed to finalize file", http.StatusInternalServerError)
			return
		}

		s.mu.Lock()
		s.totalBytes += written
		s.mu.Unlock()

		// Audit file creation for snapshots (to track what backups exist)
		if fileType == "snapshots" {
			s.audit("SNAPSHOT_CREATE", filePath, fmt.Sprintf("snapshot %s created (%d bytes)", fileName, written), true, "")
		}

		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		allowed, reason := s.checkDeleteAllowed(filePath)
		if !allowed {
			s.audit("DELETE_DENIED", filePath, reason, false, reason)
			http.Error(w, reason, http.StatusForbidden)
			return
		}

		if err := os.Remove(filePath); err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			s.audit("DELETE", filePath, "", false, err.Error())
			http.Error(w, "Failed to delete file", http.StatusInternalServerError)
			return
		}
		s.audit("DELETE", filePath, fmt.Sprintf("%s/%s deleted", fileType, fileName), true, "")
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Validation helpers

func isValidRepoName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func isValidFileName(name string) bool {
	if name == "" || len(name) > 256 {
		return false
	}
	// Allow alphanumeric, hyphen, underscore, and dot
	// Restic uses various naming conventions for different file types
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	// Don't allow path traversal
	if strings.Contains(name, "..") {
		return false
	}
	return true
}

// Logging middleware for storage requests
func WithLogging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler.ServeHTTP(w, r)
		log.Printf("[storage] %s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}
