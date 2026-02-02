package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/policy"
)

func (s *Server) policyPath() string {
	return filepath.Join(s.basePath, ".airgapper-policy.json")
}

// loadPolicy loads the policy from disk if it exists
func (s *Server) loadPolicy() {
	data, err := os.ReadFile(s.policyPath())
	if err != nil {
		return // Policy doesn't exist yet
	}

	p, err := policy.FromJSON(data)
	if err != nil {
		logging.Warnf("[storage] failed to parse policy: %v", err)
		return
	}

	// Verify the policy signatures
	if err := p.Verify(); err != nil {
		logging.Warnf("[storage] policy signature invalid: %v", err)
		return
	}

	s.policy = p
	logging.Infof("[storage] Loaded policy %s (retention: %d days, deletion: %s)",
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
