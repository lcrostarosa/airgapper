package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

func (s *Server) auditLogPath() string {
	return filepath.Join(s.basePath, ".airgapper-audit.json")
}

func (s *Server) loadAuditLog() {
	data, err := os.ReadFile(s.auditLogPath())
	if err != nil {
		return
	}

	var entries []AuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		logging.Warnf("[storage] failed to parse audit log: %v", err)
		return
	}

	s.auditLog = entries
}

func (s *Server) saveAuditLog() {
	data, err := json.MarshalIndent(s.auditLog, "", "  ")
	if err != nil {
		logging.Warnf("[storage] failed to serialize audit log: %v", err)
		return
	}

	if err := os.WriteFile(s.auditLogPath(), data, 0600); err != nil {
		logging.Warnf("[storage] failed to save audit log: %v", err)
	}
}

func (s *Server) audit(operation, path, details string, success bool, errMsg string) {
	// Use cryptographic audit chain if enabled
	if s.auditChain != nil {
		_, err := s.auditChain.Record(operation, path, details, success, errMsg)
		if err != nil {
			logging.Warnf("[storage] failed to record to audit chain: %v", err)
		}
		// Still log to stdout
		if success {
			logging.Debugf("[storage-audit] %s %s %s", operation, path, details)
		} else {
			logging.Warnf("[storage-audit] %s %s FAILED: %s", operation, path, errMsg)
		}
		return
	}

	// Legacy audit logging
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	entry := AuditEntry{
		Timestamp: timeNow(),
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
		logging.Debugf("[storage-audit] %s %s %s", operation, path, details)
	} else {
		logging.Warnf("[storage-audit] %s %s FAILED: %s", operation, path, errMsg)
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
