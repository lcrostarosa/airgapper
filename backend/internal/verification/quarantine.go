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

// QuarantineStatus represents the state of a quarantined deletion.
type QuarantineStatus string

const (
	QuarantinePending   QuarantineStatus = "pending"   // Waiting for delay to expire
	QuarantineApproved  QuarantineStatus = "approved"  // Delay expired, ready to execute
	QuarantineCancelled QuarantineStatus = "cancelled" // Owner cancelled the deletion
	QuarantineExecuted  QuarantineStatus = "executed"  // Deletion was performed
)

// QuarantinedDeletion represents a deletion that's held for a mandatory delay.
type QuarantinedDeletion struct {
	ID             string           `json:"id"`
	TicketID       string           `json:"ticket_id,omitempty"` // Associated deletion ticket
	Paths          []string         `json:"paths"`               // Files/snapshots to delete
	Reason         string           `json:"reason"`
	RequestedAt    time.Time        `json:"requested_at"`
	ExecutableAt   time.Time        `json:"executable_at"` // When deletion can proceed
	Status         QuarantineStatus `json:"status"`
	RequestedBy    string           `json:"requested_by"`
	CancelledAt    *time.Time       `json:"cancelled_at,omitempty"`
	CancelledBy    string           `json:"cancelled_by,omitempty"`
	CancelReason   string           `json:"cancel_reason,omitempty"`
	ExecutedAt     *time.Time       `json:"executed_at,omitempty"`
	OwnerSignature string           `json:"owner_signature"` // Owner must sign to request
	HostSignature  string           `json:"host_signature,omitempty"`
}

// QuarantineConfig configures the time-delayed deletion system.
type QuarantineConfig struct {
	Enabled           bool `json:"enabled"`
	DefaultDelayHours int  `json:"default_delay_hours"` // Default: 72 hours (3 days)
	MinDelayHours     int  `json:"min_delay_hours"`     // Minimum: 24 hours
	MaxDelayHours     int  `json:"max_delay_hours"`     // Maximum: 720 hours (30 days)
	AllowCancel       bool `json:"allow_cancel"`        // Allow owner to cancel during delay
}

// DefaultQuarantineConfig returns sensible defaults.
func DefaultQuarantineConfig() *QuarantineConfig {
	return &QuarantineConfig{
		Enabled:           true,
		DefaultDelayHours: 72,  // 3 days
		MinDelayHours:     24,  // 1 day minimum
		MaxDelayHours:     720, // 30 days maximum
		AllowCancel:       true,
	}
}

// QuarantineManager manages time-delayed deletions.
type QuarantineManager struct {
	basePath       string
	config         *QuarantineConfig
	ownerPublicKey []byte
	hostPrivateKey []byte
	hostPublicKey  []byte
	hostKeyID      string

	mu          sync.RWMutex
	quarantined map[string]*QuarantinedDeletion
}

// NewQuarantineManager creates a new quarantine manager.
func NewQuarantineManager(basePath string, config *QuarantineConfig, ownerPublicKey, hostPrivateKey, hostPublicKey []byte, hostKeyID string) (*QuarantineManager, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if config == nil {
		config = DefaultQuarantineConfig()
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create quarantine directory: %w", err)
	}

	qm := &QuarantineManager{
		basePath:       basePath,
		config:         config,
		ownerPublicKey: ownerPublicKey,
		hostPrivateKey: hostPrivateKey,
		hostPublicKey:  hostPublicKey,
		hostKeyID:      hostKeyID,
		quarantined:    make(map[string]*QuarantinedDeletion),
	}

	if err := qm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load quarantine: %w", err)
	}

	return qm, nil
}

func (qm *QuarantineManager) quarantinePath() string {
	return filepath.Join(qm.basePath, "quarantine.json")
}

func (qm *QuarantineManager) load() error {
	data, err := os.ReadFile(qm.quarantinePath())
	if err != nil {
		return err
	}

	var deletions []*QuarantinedDeletion
	if err := json.Unmarshal(data, &deletions); err != nil {
		return fmt.Errorf("failed to parse quarantine: %w", err)
	}

	for _, d := range deletions {
		qm.quarantined[d.ID] = d
	}

	return nil
}

func (qm *QuarantineManager) save() error {
	deletions := make([]*QuarantinedDeletion, 0, len(qm.quarantined))
	for _, d := range qm.quarantined {
		deletions = append(deletions, d)
	}

	data, err := json.MarshalIndent(deletions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal quarantine: %w", err)
	}

	return os.WriteFile(qm.quarantinePath(), data, 0600)
}

// RequestDeletion creates a new quarantined deletion request (owner side).
func RequestDeletion(ownerPrivateKey []byte, ownerKeyID string, paths []string, reason string, delayHours int) (*QuarantinedDeletion, error) {
	now := time.Now()

	if delayHours <= 0 {
		delayHours = 72 // Default 3 days
	}

	qd := &QuarantinedDeletion{
		ID:           generateQuarantineID(),
		Paths:        paths,
		Reason:       reason,
		RequestedAt:  now,
		ExecutableAt: now.Add(time.Duration(delayHours) * time.Hour),
		Status:       QuarantinePending,
		RequestedBy:  ownerKeyID,
	}

	// Sign the request
	hash, err := computeQuarantineHash(qd)
	if err != nil {
		return nil, fmt.Errorf("failed to compute hash: %w", err)
	}

	sig, err := crypto.Sign(ownerPrivateKey, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	qd.OwnerSignature = hex.EncodeToString(sig)

	return qd, nil
}

func generateQuarantineID() string {
	data := fmt.Sprintf("%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "qrn-" + hex.EncodeToString(hash[:8])
}

func computeQuarantineHash(qd *QuarantinedDeletion) ([]byte, error) {
	hashData := struct {
		ID           string   `json:"id"`
		Paths        []string `json:"paths"`
		Reason       string   `json:"reason"`
		RequestedAt  int64    `json:"requested_at"`
		ExecutableAt int64    `json:"executable_at"`
		RequestedBy  string   `json:"requested_by"`
	}{
		ID:           qd.ID,
		Paths:        qd.Paths,
		Reason:       qd.Reason,
		RequestedAt:  qd.RequestedAt.Unix(),
		ExecutableAt: qd.ExecutableAt.Unix(),
		RequestedBy:  qd.RequestedBy,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)
	return hash[:], nil
}

// RegisterDeletion accepts a quarantined deletion request (host side).
func (qm *QuarantineManager) RegisterDeletion(qd *QuarantinedDeletion) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// Verify owner signature
	if err := qm.verifyRequest(qd); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Enforce minimum delay
	minExecutable := time.Now().Add(time.Duration(qm.config.MinDelayHours) * time.Hour)
	if qd.ExecutableAt.Before(minExecutable) {
		qd.ExecutableAt = minExecutable
	}

	// Enforce maximum delay
	maxExecutable := time.Now().Add(time.Duration(qm.config.MaxDelayHours) * time.Hour)
	if qd.ExecutableAt.After(maxExecutable) {
		qd.ExecutableAt = maxExecutable
	}

	// Add host acknowledgment signature
	if qm.hostPrivateKey != nil {
		hash, err := computeQuarantineHash(qd)
		if err != nil {
			return err
		}
		sig, err := crypto.Sign(qm.hostPrivateKey, hash)
		if err != nil {
			return err
		}
		qd.HostSignature = hex.EncodeToString(sig)
	}

	qm.quarantined[qd.ID] = qd

	return qm.save()
}

func (qm *QuarantineManager) verifyRequest(qd *QuarantinedDeletion) error {
	if qm.ownerPublicKey == nil {
		return errors.New("owner public key not configured")
	}

	if qd.OwnerSignature == "" {
		return errors.New("request not signed")
	}

	hash, err := computeQuarantineHash(qd)
	if err != nil {
		return err
	}

	sig, err := hex.DecodeString(qd.OwnerSignature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !crypto.Verify(qm.ownerPublicKey, hash, sig) {
		return errors.New("signature verification failed")
	}

	return nil
}

// CancelDeletion cancels a pending quarantined deletion (owner side).
func (qm *QuarantineManager) CancelDeletion(id, cancelledBy, reason string, ownerSignature []byte) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if !qm.config.AllowCancel {
		return errors.New("cancellation not allowed by policy")
	}

	qd, exists := qm.quarantined[id]
	if !exists {
		return fmt.Errorf("quarantine %s not found", id)
	}

	if qd.Status != QuarantinePending {
		return fmt.Errorf("cannot cancel: status is %s", qd.Status)
	}

	// Verify owner signature on cancellation
	cancelHash := sha256.Sum256([]byte(fmt.Sprintf("cancel:%s:%s", id, reason)))
	if !crypto.Verify(qm.ownerPublicKey, cancelHash[:], ownerSignature) {
		return errors.New("invalid cancellation signature")
	}

	now := time.Now()
	qd.Status = QuarantineCancelled
	qd.CancelledAt = &now
	qd.CancelledBy = cancelledBy
	qd.CancelReason = reason

	return qm.save()
}

// GetReadyDeletions returns deletions that have passed their delay period.
func (qm *QuarantineManager) GetReadyDeletions() []*QuarantinedDeletion {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	now := time.Now()
	var ready []*QuarantinedDeletion

	for _, qd := range qm.quarantined {
		if qd.Status == QuarantinePending && now.After(qd.ExecutableAt) {
			qd.Status = QuarantineApproved
			ready = append(ready, qd)
		}
	}

	return ready
}

// MarkExecuted marks a quarantined deletion as executed.
func (qm *QuarantineManager) MarkExecuted(id string) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	qd, exists := qm.quarantined[id]
	if !exists {
		return fmt.Errorf("quarantine %s not found", id)
	}

	if qd.Status != QuarantineApproved && qd.Status != QuarantinePending {
		return fmt.Errorf("cannot execute: status is %s", qd.Status)
	}

	now := time.Now()
	qd.Status = QuarantineExecuted
	qd.ExecutedAt = &now

	return qm.save()
}

// GetPending returns all pending quarantined deletions.
func (qm *QuarantineManager) GetPending() []*QuarantinedDeletion {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	var pending []*QuarantinedDeletion
	for _, qd := range qm.quarantined {
		if qd.Status == QuarantinePending {
			pending = append(pending, qd)
		}
	}
	return pending
}

// Get retrieves a quarantined deletion by ID.
func (qm *QuarantineManager) Get(id string) *QuarantinedDeletion {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.quarantined[id]
}

// TimeUntilExecutable returns how long until a deletion can proceed.
func (qd *QuarantinedDeletion) TimeUntilExecutable() time.Duration {
	if qd.Status != QuarantinePending {
		return 0
	}
	remaining := time.Until(qd.ExecutableAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}
