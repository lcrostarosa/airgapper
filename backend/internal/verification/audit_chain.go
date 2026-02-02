package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// ChainedAuditEntry represents a single entry in the cryptographic audit chain.
// Each entry is linked to the previous via hash chaining, making tampering detectable.
type ChainedAuditEntry struct {
	ID            string    `json:"id"`
	Sequence      uint64    `json:"sequence"`
	Timestamp     time.Time `json:"timestamp"`
	Operation     string    `json:"operation"` // CREATE, DELETE, POLICY_SET, etc.
	Path          string    `json:"path,omitempty"`
	Details       string    `json:"details,omitempty"`
	Success       bool      `json:"success"`
	Error         string    `json:"error,omitempty"`

	// Chaining fields
	PreviousHash  string `json:"previous_hash"`  // SHA256 of previous entry
	ContentHash   string `json:"content_hash"`   // SHA256 of this entry's content (excluding signatures)
	HostSignature string `json:"host_signature"` // Ed25519 signature by host
	HostKeyID     string `json:"host_key_id"`    // ID of signing key
}

// AuditChain manages a cryptographic audit chain with hash-chaining and signatures.
type AuditChain struct {
	basePath   string
	hostKeyID  string
	privateKey []byte
	publicKey  []byte
	signEntries bool

	mu        sync.RWMutex
	entries   []ChainedAuditEntry
	sequence  uint64
	lastHash  string
	maxEntries int
}

// AuditChainState persists chain metadata for reload.
type AuditChainState struct {
	Sequence   uint64 `json:"sequence"`
	LastHash   string `json:"last_hash"`
	EntryCount int    `json:"entry_count"`
}

// NewAuditChain creates a new audit chain manager.
func NewAuditChain(basePath, hostKeyID string, privateKey, publicKey []byte, signEntries bool) (*AuditChain, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit chain directory: %w", err)
	}

	ac := &AuditChain{
		basePath:    basePath,
		hostKeyID:   hostKeyID,
		privateKey:  privateKey,
		publicKey:   publicKey,
		signEntries: signEntries,
		maxEntries:  10000, // Keep last 10k entries in memory
		lastHash:    "genesis", // Initial hash for first entry
	}

	// Load existing chain
	if err := ac.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load audit chain: %w", err)
	}

	return ac, nil
}

// chainPath returns the path to the chain file.
func (ac *AuditChain) chainPath() string {
	return filepath.Join(ac.basePath, "audit-chain.json")
}

// statePath returns the path to the state file.
func (ac *AuditChain) statePath() string {
	return filepath.Join(ac.basePath, "audit-chain-state.json")
}

// load reads the chain from disk.
func (ac *AuditChain) load() error {
	// Load state first
	stateData, err := os.ReadFile(ac.statePath())
	if err == nil {
		var state AuditChainState
		if err := json.Unmarshal(stateData, &state); err == nil {
			ac.sequence = state.Sequence
			ac.lastHash = state.LastHash
		}
	}

	// Load entries
	data, err := os.ReadFile(ac.chainPath())
	if err != nil {
		return err
	}

	var entries []ChainedAuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to parse audit chain: %w", err)
	}

	ac.entries = entries

	// If we have entries, update state from them
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		ac.sequence = last.Sequence
		ac.lastHash = last.ContentHash
	}

	return nil
}

// save writes the chain to disk.
func (ac *AuditChain) save() error {
	// Save entries
	data, err := json.MarshalIndent(ac.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal audit chain: %w", err)
	}

	if err := os.WriteFile(ac.chainPath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write audit chain: %w", err)
	}

	// Save state
	state := AuditChainState{
		Sequence:   ac.sequence,
		LastHash:   ac.lastHash,
		EntryCount: len(ac.entries),
	}

	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(ac.statePath(), stateData, 0600); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

// Record adds a new entry to the audit chain.
func (ac *AuditChain) Record(operation, path, details string, success bool, errMsg string) (*ChainedAuditEntry, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.sequence++

	entry := ChainedAuditEntry{
		ID:           generateEntryID(ac.sequence),
		Sequence:     ac.sequence,
		Timestamp:    time.Now(),
		Operation:    operation,
		Path:         path,
		Details:      details,
		Success:      success,
		Error:        errMsg,
		PreviousHash: ac.lastHash,
		HostKeyID:    ac.hostKeyID,
	}

	// Compute content hash (excluding signature)
	contentHash, err := ac.computeContentHash(&entry)
	if err != nil {
		return nil, fmt.Errorf("failed to compute content hash: %w", err)
	}
	entry.ContentHash = contentHash

	// Sign the entry if enabled
	if ac.signEntries && ac.privateKey != nil {
		sig, err := crypto.Sign(ac.privateKey, []byte(contentHash))
		if err != nil {
			return nil, fmt.Errorf("failed to sign entry: %w", err)
		}
		entry.HostSignature = hex.EncodeToString(sig)
	}

	// Update chain
	ac.lastHash = contentHash
	ac.entries = append(ac.entries, entry)

	// Trim if too large
	if len(ac.entries) > ac.maxEntries {
		ac.entries = ac.entries[len(ac.entries)-ac.maxEntries:]
	}

	// Persist
	if err := ac.save(); err != nil {
		return nil, fmt.Errorf("failed to save audit chain: %w", err)
	}

	return &entry, nil
}

// computeContentHash creates a deterministic hash of entry content.
func (ac *AuditChain) computeContentHash(entry *ChainedAuditEntry) (string, error) {
	// Create canonical structure for hashing (excludes signature)
	hashData := struct {
		ID           string `json:"id"`
		Sequence     uint64 `json:"sequence"`
		Timestamp    int64  `json:"timestamp"`
		Operation    string `json:"operation"`
		Path         string `json:"path"`
		Details      string `json:"details"`
		Success      bool   `json:"success"`
		Error        string `json:"error"`
		PreviousHash string `json:"previous_hash"`
		HostKeyID    string `json:"host_key_id"`
	}{
		ID:           entry.ID,
		Sequence:     entry.Sequence,
		Timestamp:    entry.Timestamp.Unix(),
		Operation:    entry.Operation,
		Path:         entry.Path,
		Details:      entry.Details,
		Success:      entry.Success,
		Error:        entry.Error,
		PreviousHash: entry.PreviousHash,
		HostKeyID:    entry.HostKeyID,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// generateEntryID creates a unique entry ID.
func generateEntryID(sequence uint64) string {
	data := fmt.Sprintf("%d-%d", time.Now().UnixNano(), sequence)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// Verify checks the integrity of the entire chain.
func (ac *AuditChain) Verify() (*ChainVerificationResult, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	result := &ChainVerificationResult{
		VerifiedAt:   time.Now(),
		TotalEntries: len(ac.entries),
	}

	if len(ac.entries) == 0 {
		result.Valid = true
		return result, nil
	}

	expectedPrevHash := "genesis"

	for i, entry := range ac.entries {
		// Verify hash chain
		if entry.PreviousHash != expectedPrevHash {
			result.Valid = false
			result.FirstBrokenAt = &i
			result.Errors = append(result.Errors, fmt.Sprintf(
				"chain broken at entry %d: expected previous_hash %s, got %s",
				i, expectedPrevHash, entry.PreviousHash))
			return result, nil
		}

		// Verify content hash
		computedHash, err := ac.computeContentHash(&entry)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf(
				"failed to compute hash for entry %d: %v", i, err))
			continue
		}

		if computedHash != entry.ContentHash {
			result.Valid = false
			result.FirstBrokenAt = &i
			result.Errors = append(result.Errors, fmt.Sprintf(
				"content tampered at entry %d: computed %s, stored %s",
				i, computedHash, entry.ContentHash))
			return result, nil
		}

		// Verify signature if present
		if entry.HostSignature != "" && ac.publicKey != nil {
			sig, err := hex.DecodeString(entry.HostSignature)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf(
					"invalid signature encoding at entry %d", i))
				continue
			}

			if !crypto.Verify(ac.publicKey, []byte(entry.ContentHash), sig) {
				result.Valid = false
				result.FirstBrokenAt = &i
				result.Errors = append(result.Errors, fmt.Sprintf(
					"signature verification failed at entry %d", i))
				return result, nil
			}
			result.SignedEntries++
		}

		expectedPrevHash = entry.ContentHash
		result.ValidEntries++
	}

	result.Valid = len(result.Errors) == 0
	result.LastSequence = ac.sequence
	result.LastHash = ac.lastHash

	return result, nil
}

// ChainVerificationResult contains the result of verifying the audit chain.
type ChainVerificationResult struct {
	Valid         bool      `json:"valid"`
	VerifiedAt    time.Time `json:"verified_at"`
	TotalEntries  int       `json:"total_entries"`
	ValidEntries  int       `json:"valid_entries"`
	SignedEntries int       `json:"signed_entries"`
	LastSequence  uint64    `json:"last_sequence"`
	LastHash      string    `json:"last_hash"`
	FirstBrokenAt *int      `json:"first_broken_at,omitempty"`
	Errors        []string  `json:"errors,omitempty"`
}

// GetEntries returns audit entries with optional filtering.
func (ac *AuditChain) GetEntries(limit int, offset int, operation string) []ChainedAuditEntry {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	var filtered []ChainedAuditEntry

	// Apply operation filter
	if operation != "" {
		for _, e := range ac.entries {
			if e.Operation == operation {
				filtered = append(filtered, e)
			}
		}
	} else {
		filtered = ac.entries
	}

	// Apply offset and limit
	if offset >= len(filtered) {
		return []ChainedAuditEntry{}
	}

	start := len(filtered) - offset - 1
	if start < 0 {
		start = 0
	}

	end := start - limit + 1
	if end < 0 {
		end = 0
	}

	// Return in reverse order (newest first)
	result := make([]ChainedAuditEntry, start-end+1)
	for i, j := start, 0; i >= end; i, j = i-1, j+1 {
		result[j] = filtered[i]
	}

	return result
}

// GetLatestHash returns the hash of the most recent entry.
func (ac *AuditChain) GetLatestHash() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.lastHash
}

// GetSequence returns the current sequence number.
func (ac *AuditChain) GetSequence() uint64 {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.sequence
}

// Export returns all entries for external verification.
func (ac *AuditChain) Export() ([]byte, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	export := struct {
		ExportedAt time.Time           `json:"exported_at"`
		HostKeyID  string              `json:"host_key_id"`
		PublicKey  string              `json:"public_key,omitempty"`
		Sequence   uint64              `json:"sequence"`
		LastHash   string              `json:"last_hash"`
		Entries    []ChainedAuditEntry `json:"entries"`
	}{
		ExportedAt: time.Now(),
		HostKeyID:  ac.hostKeyID,
		Sequence:   ac.sequence,
		LastHash:   ac.lastHash,
		Entries:    ac.entries,
	}

	if ac.publicKey != nil {
		export.PublicKey = hex.EncodeToString(ac.publicKey)
	}

	return json.MarshalIndent(export, "", "  ")
}

// GetEntrySince returns entries since a given sequence number.
func (ac *AuditChain) GetEntrySince(sequence uint64) []ChainedAuditEntry {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	var result []ChainedAuditEntry
	for _, e := range ac.entries {
		if e.Sequence > sequence {
			result = append(result, e)
		}
	}
	return result
}
