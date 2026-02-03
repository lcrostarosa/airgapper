package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// TicketTargetType defines what a deletion ticket authorizes.
type TicketTargetType string

const (
	TicketTargetSnapshot TicketTargetType = "snapshot" // Delete specific snapshots
	TicketTargetFile     TicketTargetType = "file"     // Delete specific files
	TicketTargetPrune    TicketTargetType = "prune"    // Prune old snapshots
)

// TicketTarget specifies what the ticket authorizes for deletion.
type TicketTarget struct {
	Type        TicketTargetType `json:"type"`
	SnapshotIDs []string         `json:"snapshot_ids,omitempty"` // For snapshot type
	Paths       []string         `json:"paths,omitempty"`        // For file type
	OlderThan   *time.Time       `json:"older_than,omitempty"`   // For prune type
}

// DeletionTicket authorizes specific deletions on the host.
// Only the owner can create valid tickets; the host validates them before deleting.
type DeletionTicket struct {
	ID             string       `json:"id"`
	OwnerKeyID     string       `json:"owner_key_id"`
	Target         TicketTarget `json:"target"`
	Reason         string       `json:"reason"`
	CreatedAt      time.Time    `json:"created_at"`
	ExpiresAt      time.Time    `json:"expires_at,omitempty"` // 0 = no expiry
	OwnerSignature string       `json:"owner_signature"`
}

// TicketUsageRecord records when and how a ticket was used.
type TicketUsageRecord struct {
	TicketID      string    `json:"ticket_id"`
	UsedAt        time.Time `json:"used_at"`
	DeletedPaths  []string  `json:"deleted_paths"`
	HostSignature string    `json:"host_signature"`
	HostKeyID     string    `json:"host_key_id"`
}

// TicketManager manages deletion tickets and their usage.
type TicketManager struct {
	basePath       string
	ownerPublicKey []byte
	hostPrivateKey []byte
	hostPublicKey  []byte
	hostKeyID      string
	validityDays   int

	mu           sync.RWMutex
	tickets      map[string]*DeletionTicket
	usageRecords []TicketUsageRecord
}

// NewTicketManager creates a new ticket manager.
func NewTicketManager(basePath string, ownerPublicKey, hostPrivateKey, hostPublicKey []byte, hostKeyID string, validityDays int) (*TicketManager, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create ticket directory: %w", err)
	}

	if validityDays <= 0 {
		validityDays = 7
	}

	tm := &TicketManager{
		basePath:       basePath,
		ownerPublicKey: ownerPublicKey,
		hostPrivateKey: hostPrivateKey,
		hostPublicKey:  hostPublicKey,
		hostKeyID:      hostKeyID,
		validityDays:   validityDays,
		tickets:        make(map[string]*DeletionTicket),
	}

	if err := tm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load tickets: %w", err)
	}

	return tm, nil
}

// ticketsPath returns the path to the tickets file.
func (tm *TicketManager) ticketsPath() string {
	return filepath.Join(tm.basePath, "tickets.json")
}

// usagePath returns the path to the usage records file.
func (tm *TicketManager) usagePath() string {
	return filepath.Join(tm.basePath, "ticket-usage.json")
}

// load reads tickets and usage records from disk.
func (tm *TicketManager) load() error {
	// Load tickets
	data, err := os.ReadFile(tm.ticketsPath())
	if err == nil {
		var tickets []*DeletionTicket
		if err := json.Unmarshal(data, &tickets); err == nil {
			for _, t := range tickets {
				tm.tickets[t.ID] = t
			}
		}
	}

	// Load usage records
	usageData, err := os.ReadFile(tm.usagePath())
	if err == nil {
		if unmarshalErr := json.Unmarshal(usageData, &tm.usageRecords); unmarshalErr != nil {
			logging.Warn("Failed to unmarshal usage records", logging.Err(unmarshalErr))
		}
	}

	return nil
}

// save writes tickets and usage records to disk.
func (tm *TicketManager) save() error {
	// Save tickets
	tickets := make([]*DeletionTicket, 0, len(tm.tickets))
	for _, t := range tm.tickets {
		tickets = append(tickets, t)
	}

	data, err := json.MarshalIndent(tickets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tickets: %w", err)
	}

	if err := os.WriteFile(tm.ticketsPath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write tickets: %w", err)
	}

	// Save usage records
	usageData, err := json.MarshalIndent(tm.usageRecords, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal usage: %w", err)
	}

	if err := os.WriteFile(tm.usagePath(), usageData, 0600); err != nil {
		return fmt.Errorf("failed to write usage: %w", err)
	}

	return nil
}

// CreateTicket creates a new deletion ticket (owner side).
// The owner signs the ticket with their private key.
func CreateTicket(ownerPrivateKey []byte, ownerKeyID string, target TicketTarget, reason string, validityDays int) (*DeletionTicket, error) {
	now := time.Now()

	ticket := &DeletionTicket{
		ID:         generateTicketID(),
		OwnerKeyID: ownerKeyID,
		Target:     target,
		Reason:     reason,
		CreatedAt:  now,
	}

	if validityDays > 0 {
		ticket.ExpiresAt = now.Add(time.Duration(validityDays) * 24 * time.Hour)
	}

	// Compute and sign the ticket
	hash, err := computeTicketHash(ticket)
	if err != nil {
		return nil, fmt.Errorf("failed to compute ticket hash: %w", err)
	}

	sig, err := crypto.Sign(ownerPrivateKey, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ticket: %w", err)
	}

	ticket.OwnerSignature = hex.EncodeToString(sig)

	return ticket, nil
}

// generateTicketID creates a unique ticket ID.
func generateTicketID() string {
	data := fmt.Sprintf("%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "tkt-" + hex.EncodeToString(hash[:8])
}

// computeTicketHash creates a deterministic hash of ticket content.
func computeTicketHash(ticket *DeletionTicket) ([]byte, error) {
	// Sort paths for canonical ordering
	sortedPaths := make([]string, len(ticket.Target.Paths))
	copy(sortedPaths, ticket.Target.Paths)
	sort.Strings(sortedPaths)

	sortedSnapshots := make([]string, len(ticket.Target.SnapshotIDs))
	copy(sortedSnapshots, ticket.Target.SnapshotIDs)
	sort.Strings(sortedSnapshots)

	var olderThanUnix int64
	if ticket.Target.OlderThan != nil {
		olderThanUnix = ticket.Target.OlderThan.Unix()
	}

	hashData := struct {
		ID          string           `json:"id"`
		OwnerKeyID  string           `json:"owner_key_id"`
		TargetType  TicketTargetType `json:"target_type"`
		SnapshotIDs []string         `json:"snapshot_ids"`
		Paths       []string         `json:"paths"`
		OlderThan   int64            `json:"older_than"`
		Reason      string           `json:"reason"`
		CreatedAt   int64            `json:"created_at"`
		ExpiresAt   int64            `json:"expires_at"`
	}{
		ID:          ticket.ID,
		OwnerKeyID:  ticket.OwnerKeyID,
		TargetType:  ticket.Target.Type,
		SnapshotIDs: sortedSnapshots,
		Paths:       sortedPaths,
		OlderThan:   olderThanUnix,
		Reason:      ticket.Reason,
		CreatedAt:   ticket.CreatedAt.Unix(),
		ExpiresAt:   ticket.ExpiresAt.Unix(),
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// RegisterTicket adds a ticket to the manager (host side).
// Validates the owner signature before accepting.
func (tm *TicketManager) RegisterTicket(ticket *DeletionTicket) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Verify signature
	if err := tm.verifyTicket(ticket); err != nil {
		return fmt.Errorf("ticket verification failed: %w", err)
	}

	// Check if already registered
	if _, exists := tm.tickets[ticket.ID]; exists {
		return fmt.Errorf("ticket %s already registered", ticket.ID)
	}

	// Check expiry
	if !ticket.ExpiresAt.IsZero() && time.Now().After(ticket.ExpiresAt) {
		return errors.New("ticket has expired")
	}

	tm.tickets[ticket.ID] = ticket

	return tm.save()
}

// verifyTicket verifies the owner signature on a ticket.
func (tm *TicketManager) verifyTicket(ticket *DeletionTicket) error {
	if tm.ownerPublicKey == nil {
		return errors.New("owner public key not configured")
	}

	if ticket.OwnerSignature == "" {
		return errors.New("ticket not signed")
	}

	hash, err := computeTicketHash(ticket)
	if err != nil {
		return fmt.Errorf("failed to compute ticket hash: %w", err)
	}

	sig, err := hex.DecodeString(ticket.OwnerSignature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !crypto.Verify(tm.ownerPublicKey, hash, sig) {
		return errors.New("signature verification failed")
	}

	return nil
}

// ValidateDelete checks if a deletion is authorized by a valid ticket.
// Returns the ticket ID if authorized, or an error if not.
func (tm *TicketManager) ValidateDelete(path string, snapshotID string) (string, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	now := time.Now()

	for _, ticket := range tm.tickets {
		// Check expiry
		if !ticket.ExpiresAt.IsZero() && now.After(ticket.ExpiresAt) {
			continue
		}

		// Check if this ticket authorizes the deletion
		switch ticket.Target.Type {
		case TicketTargetSnapshot:
			if snapshotID != "" {
				for _, id := range ticket.Target.SnapshotIDs {
					if id == snapshotID {
						return ticket.ID, nil
					}
				}
			}
		case TicketTargetFile:
			for _, p := range ticket.Target.Paths {
				if p == path || matchPath(p, path) {
					return ticket.ID, nil
				}
			}
		case TicketTargetPrune:
			// Prune tickets authorize deletion of old snapshots
			// This would need file metadata to verify age
			if ticket.Target.OlderThan != nil {
				return ticket.ID, nil
			}
		}
	}

	return "", errors.New("no valid ticket found for this deletion")
}

// matchPath checks if a pattern matches a path.
func matchPath(pattern, path string) bool {
	// Simple prefix matching for now
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		return len(path) >= len(pattern)-1 && path[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return pattern == path
}

// RecordUsage records that a ticket was used for deletion.
func (tm *TicketManager) RecordUsage(ticketID string, deletedPaths []string) (*TicketUsageRecord, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	record := TicketUsageRecord{
		TicketID:     ticketID,
		UsedAt:       time.Now(),
		DeletedPaths: deletedPaths,
		HostKeyID:    tm.hostKeyID,
	}

	// Sign the usage record
	if tm.hostPrivateKey != nil {
		hash, err := computeUsageHash(&record)
		if err != nil {
			return nil, fmt.Errorf("failed to compute usage hash: %w", err)
		}

		sig, err := crypto.Sign(tm.hostPrivateKey, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to sign usage: %w", err)
		}

		record.HostSignature = hex.EncodeToString(sig)
	}

	tm.usageRecords = append(tm.usageRecords, record)

	if err := tm.save(); err != nil {
		return nil, err
	}

	return &record, nil
}

// computeUsageHash creates a deterministic hash of usage record.
func computeUsageHash(record *TicketUsageRecord) ([]byte, error) {
	sortedPaths := make([]string, len(record.DeletedPaths))
	copy(sortedPaths, record.DeletedPaths)
	sort.Strings(sortedPaths)

	hashData := struct {
		TicketID     string   `json:"ticket_id"`
		UsedAt       int64    `json:"used_at"`
		DeletedPaths []string `json:"deleted_paths"`
		HostKeyID    string   `json:"host_key_id"`
	}{
		TicketID:     record.TicketID,
		UsedAt:       record.UsedAt.Unix(),
		DeletedPaths: sortedPaths,
		HostKeyID:    record.HostKeyID,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// GetTicket retrieves a ticket by ID.
func (tm *TicketManager) GetTicket(id string) *DeletionTicket {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.tickets[id]
}

// ListTickets returns all tickets, optionally filtering by validity.
func (tm *TicketManager) ListTickets(validOnly bool) []*DeletionTicket {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	now := time.Now()
	var result []*DeletionTicket

	for _, t := range tm.tickets {
		if validOnly {
			if !t.ExpiresAt.IsZero() && now.After(t.ExpiresAt) {
				continue
			}
		}
		result = append(result, t)
	}

	return result
}

// GetUsageRecords returns usage records, optionally filtered by ticket ID.
func (tm *TicketManager) GetUsageRecords(ticketID string) []TicketUsageRecord {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if ticketID == "" {
		result := make([]TicketUsageRecord, len(tm.usageRecords))
		copy(result, tm.usageRecords)
		return result
	}

	var result []TicketUsageRecord
	for _, r := range tm.usageRecords {
		if r.TicketID == ticketID {
			result = append(result, r)
		}
	}
	return result
}

// RevokeTicket removes a ticket (for administrative cleanup).
func (tm *TicketManager) RevokeTicket(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tickets[id]; !exists {
		return fmt.Errorf("ticket %s not found", id)
	}

	delete(tm.tickets, id)
	return tm.save()
}

// CleanupExpired removes expired tickets.
func (tm *TicketManager) CleanupExpired() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, t := range tm.tickets {
		if !t.ExpiresAt.IsZero() && now.After(t.ExpiresAt) {
			delete(tm.tickets, id)
			removed++
		}
	}

	if removed > 0 {
		if err := tm.save(); err != nil {
			logging.Warn("Failed to save after cleanup", logging.Err(err))
		}
	}

	return removed
}
