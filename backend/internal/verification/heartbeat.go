package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// HeartbeatStatus represents the status of heartbeat monitoring.
type HeartbeatStatus string

const (
	HeartbeatStatusHealthy  HeartbeatStatus = "healthy"  // Regular heartbeats received
	HeartbeatStatusWarning  HeartbeatStatus = "warning"  // Heartbeat delayed but not critical
	HeartbeatStatusCritical HeartbeatStatus = "critical" // Heartbeat missed, possible attack
	HeartbeatStatusDead     HeartbeatStatus = "dead"     // Dead man's switch triggered
)

// Heartbeat represents a single heartbeat signal.
type Heartbeat struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Sequence      uint64    `json:"sequence"`

	// System state snapshot
	AuditChainHash   string `json:"audit_chain_hash,omitempty"`
	AuditChainSeq    uint64 `json:"audit_chain_seq,omitempty"`
	SnapshotCount    int    `json:"snapshot_count,omitempty"`
	TotalBytes       int64  `json:"total_bytes,omitempty"`
	CanaryStatus     string `json:"canary_status,omitempty"`

	// Proof this heartbeat is fresh
	Nonce         string `json:"nonce"`          // Random nonce
	PreviousHash  string `json:"previous_hash"`  // Hash of previous heartbeat
	ContentHash   string `json:"content_hash"`   // Hash of this heartbeat content
	HostSignature string `json:"host_signature"` // Host signs the heartbeat
	HostKeyID     string `json:"host_key_id"`
}

// DeadManSwitch tracks whether the system is still alive.
type DeadManSwitch struct {
	Enabled           bool          `json:"enabled"`
	LastCheckIn       time.Time     `json:"last_check_in"`
	ExpectedInterval  time.Duration `json:"expected_interval"`
	GracePeriod       time.Duration `json:"grace_period"`
	Status            HeartbeatStatus `json:"status"`
	MissedCount       int           `json:"missed_count"`
	TriggeredAt       *time.Time    `json:"triggered_at,omitempty"`
	AlertSent         bool          `json:"alert_sent"`
	AlertSentAt       *time.Time    `json:"alert_sent_at,omitempty"`
	RecoveryCode      string        `json:"recovery_code,omitempty"` // Required to reset after trigger
}

// HeartbeatConfig configures heartbeat monitoring.
type HeartbeatConfig struct {
	Enabled              bool   `json:"enabled"`
	IntervalSeconds      int    `json:"interval_seconds"`       // How often to send heartbeats
	WarningThreshold     int    `json:"warning_threshold"`      // Missed beats before warning
	CriticalThreshold    int    `json:"critical_threshold"`     // Missed beats before critical
	DeadManThreshold     int    `json:"dead_man_threshold"`     // Missed beats to trigger dead man switch
	RequireOwnerResponse bool   `json:"require_owner_response"` // Owner must acknowledge heartbeats
	AlertWebhookURL      string `json:"alert_webhook_url,omitempty"`
	AlertEmailAddress    string `json:"alert_email,omitempty"`
}

// DefaultHeartbeatConfig returns sensible defaults.
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Enabled:              true,
		IntervalSeconds:      300,  // 5 minutes
		WarningThreshold:     3,    // 15 minutes without heartbeat
		CriticalThreshold:    6,    // 30 minutes without heartbeat
		DeadManThreshold:     12,   // 1 hour without heartbeat
		RequireOwnerResponse: false,
	}
}

// HeartbeatMonitor manages heartbeat generation and dead man's switch.
type HeartbeatMonitor struct {
	basePath       string
	config         *HeartbeatConfig
	hostPrivateKey []byte
	hostPublicKey  []byte
	hostKeyID      string

	mu           sync.RWMutex
	heartbeats   []Heartbeat
	deadManSwitch *DeadManSwitch
	sequence     uint64
	lastHash     string

	// State providers
	getAuditState  func() (string, uint64)
	getStorageState func() (int, int64)
	getCanaryState func() string

	// Alert callbacks
	onWarning  func(status HeartbeatStatus, missedCount int)
	onCritical func(status HeartbeatStatus, missedCount int)
	onDeadMan  func(dms *DeadManSwitch)
}

// NewHeartbeatMonitor creates a new heartbeat monitor.
func NewHeartbeatMonitor(basePath string, config *HeartbeatConfig, hostPrivateKey, hostPublicKey []byte, hostKeyID string) (*HeartbeatMonitor, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if config == nil {
		config = DefaultHeartbeatConfig()
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create heartbeat directory: %w", err)
	}

	hm := &HeartbeatMonitor{
		basePath:       basePath,
		config:         config,
		hostPrivateKey: hostPrivateKey,
		hostPublicKey:  hostPublicKey,
		hostKeyID:      hostKeyID,
		heartbeats:     []Heartbeat{},
		lastHash:       "genesis",
		deadManSwitch: &DeadManSwitch{
			Enabled:          config.Enabled,
			ExpectedInterval: time.Duration(config.IntervalSeconds) * time.Second,
			GracePeriod:      time.Duration(config.IntervalSeconds*config.WarningThreshold) * time.Second,
			Status:           HeartbeatStatusHealthy,
		},
	}

	if err := hm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load heartbeat state: %w", err)
	}

	return hm, nil
}

func (hm *HeartbeatMonitor) heartbeatPath() string {
	return filepath.Join(hm.basePath, "heartbeats.json")
}

func (hm *HeartbeatMonitor) dmsPath() string {
	return filepath.Join(hm.basePath, "deadman-switch.json")
}

func (hm *HeartbeatMonitor) load() error {
	// Load heartbeats
	data, err := os.ReadFile(hm.heartbeatPath())
	if err == nil {
		json.Unmarshal(data, &hm.heartbeats)
		if len(hm.heartbeats) > 0 {
			last := hm.heartbeats[len(hm.heartbeats)-1]
			hm.sequence = last.Sequence
			hm.lastHash = last.ContentHash
		}
	}

	// Load dead man switch state
	dmsData, err := os.ReadFile(hm.dmsPath())
	if err == nil {
		json.Unmarshal(dmsData, &hm.deadManSwitch)
	}

	return nil
}

func (hm *HeartbeatMonitor) save() error {
	// Keep only last 1000 heartbeats
	if len(hm.heartbeats) > 1000 {
		hm.heartbeats = hm.heartbeats[len(hm.heartbeats)-1000:]
	}

	data, err := json.MarshalIndent(hm.heartbeats, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(hm.heartbeatPath(), data, 0600); err != nil {
		return err
	}

	dmsData, err := json.MarshalIndent(hm.deadManSwitch, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(hm.dmsPath(), dmsData, 0600)
}

// SetStateProviders sets functions to get current system state.
func (hm *HeartbeatMonitor) SetStateProviders(
	auditState func() (string, uint64),
	storageState func() (int, int64),
	canaryState func() string,
) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.getAuditState = auditState
	hm.getStorageState = storageState
	hm.getCanaryState = canaryState
}

// SetAlertCallbacks sets callback functions for alerts.
func (hm *HeartbeatMonitor) SetAlertCallbacks(
	onWarning func(status HeartbeatStatus, missedCount int),
	onCritical func(status HeartbeatStatus, missedCount int),
	onDeadMan func(dms *DeadManSwitch),
) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.onWarning = onWarning
	hm.onCritical = onCritical
	hm.onDeadMan = onDeadMan
}

// GenerateHeartbeat creates a new heartbeat signal.
func (hm *HeartbeatMonitor) GenerateHeartbeat() (*Heartbeat, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.config.Enabled {
		return nil, errors.New("heartbeat monitoring disabled")
	}

	hm.sequence++
	now := time.Now()

	// Generate nonce
	nonceBytes := make([]byte, 16)
	if _, err := cryptoRandRead(nonceBytes); err != nil {
		return nil, err
	}

	hb := Heartbeat{
		ID:           fmt.Sprintf("hb-%d-%d", now.Unix(), hm.sequence),
		Timestamp:    now,
		Sequence:     hm.sequence,
		Nonce:        hex.EncodeToString(nonceBytes),
		PreviousHash: hm.lastHash,
		HostKeyID:    hm.hostKeyID,
	}

	// Get system state
	if hm.getAuditState != nil {
		hb.AuditChainHash, hb.AuditChainSeq = hm.getAuditState()
	}
	if hm.getStorageState != nil {
		hb.SnapshotCount, hb.TotalBytes = hm.getStorageState()
	}
	if hm.getCanaryState != nil {
		hb.CanaryStatus = hm.getCanaryState()
	}

	// Compute content hash
	contentHash, err := computeHeartbeatHash(&hb)
	if err != nil {
		return nil, err
	}
	hb.ContentHash = contentHash

	// Sign heartbeat
	if hm.hostPrivateKey != nil {
		sig, err := crypto.Sign(hm.hostPrivateKey, []byte(contentHash))
		if err != nil {
			return nil, fmt.Errorf("failed to sign heartbeat: %w", err)
		}
		hb.HostSignature = hex.EncodeToString(sig)
	}

	// Update state
	hm.lastHash = contentHash
	hm.heartbeats = append(hm.heartbeats, hb)
	hm.deadManSwitch.LastCheckIn = now
	hm.deadManSwitch.MissedCount = 0
	hm.deadManSwitch.Status = HeartbeatStatusHealthy

	if err := hm.save(); err != nil {
		return nil, err
	}

	return &hb, nil
}

// Helper for crypto rand
func cryptoRandRead(b []byte) (int, error) {
	return rand.Read(b)
}

func computeHeartbeatHash(hb *Heartbeat) (string, error) {
	hashData := struct {
		ID             string `json:"id"`
		Timestamp      int64  `json:"timestamp"`
		Sequence       uint64 `json:"sequence"`
		AuditChainHash string `json:"audit_chain_hash"`
		AuditChainSeq  uint64 `json:"audit_chain_seq"`
		SnapshotCount  int    `json:"snapshot_count"`
		TotalBytes     int64  `json:"total_bytes"`
		CanaryStatus   string `json:"canary_status"`
		Nonce          string `json:"nonce"`
		PreviousHash   string `json:"previous_hash"`
		HostKeyID      string `json:"host_key_id"`
	}{
		ID:             hb.ID,
		Timestamp:      hb.Timestamp.Unix(),
		Sequence:       hb.Sequence,
		AuditChainHash: hb.AuditChainHash,
		AuditChainSeq:  hb.AuditChainSeq,
		SnapshotCount:  hb.SnapshotCount,
		TotalBytes:     hb.TotalBytes,
		CanaryStatus:   hb.CanaryStatus,
		Nonce:          hb.Nonce,
		PreviousHash:   hb.PreviousHash,
		HostKeyID:      hb.HostKeyID,
	}

	data, err := json.Marshal(hashData)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// CheckDeadManSwitch evaluates if the dead man's switch should trigger.
func (hm *HeartbeatMonitor) CheckDeadManSwitch() *DeadManSwitch {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.config.Enabled || hm.deadManSwitch.Status == HeartbeatStatusDead {
		return hm.deadManSwitch
	}

	now := time.Now()
	timeSinceLastCheckIn := now.Sub(hm.deadManSwitch.LastCheckIn)
	expectedInterval := time.Duration(hm.config.IntervalSeconds) * time.Second

	// Calculate missed heartbeats
	missedCount := int(timeSinceLastCheckIn / expectedInterval)
	hm.deadManSwitch.MissedCount = missedCount

	previousStatus := hm.deadManSwitch.Status

	// Update status based on missed count
	switch {
	case missedCount >= hm.config.DeadManThreshold:
		hm.deadManSwitch.Status = HeartbeatStatusDead
		if previousStatus != HeartbeatStatusDead {
			now := time.Now()
			hm.deadManSwitch.TriggeredAt = &now
			// Generate recovery code
			recoveryBytes := make([]byte, 16)
			rand.Read(recoveryBytes)
			hm.deadManSwitch.RecoveryCode = hex.EncodeToString(recoveryBytes)
			if hm.onDeadMan != nil {
				go hm.onDeadMan(hm.deadManSwitch)
			}
		}

	case missedCount >= hm.config.CriticalThreshold:
		hm.deadManSwitch.Status = HeartbeatStatusCritical
		if previousStatus != HeartbeatStatusCritical && hm.onCritical != nil {
			go hm.onCritical(HeartbeatStatusCritical, missedCount)
		}

	case missedCount >= hm.config.WarningThreshold:
		hm.deadManSwitch.Status = HeartbeatStatusWarning
		if previousStatus != HeartbeatStatusWarning && hm.onWarning != nil {
			go hm.onWarning(HeartbeatStatusWarning, missedCount)
		}

	default:
		hm.deadManSwitch.Status = HeartbeatStatusHealthy
	}

	hm.save()
	return hm.deadManSwitch
}

// ResetDeadManSwitch resets the dead man's switch after trigger (requires recovery code).
func (hm *HeartbeatMonitor) ResetDeadManSwitch(recoveryCode string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if hm.deadManSwitch.Status != HeartbeatStatusDead {
		return errors.New("dead man switch not triggered")
	}

	if hm.deadManSwitch.RecoveryCode == "" || recoveryCode != hm.deadManSwitch.RecoveryCode {
		return errors.New("invalid recovery code")
	}

	hm.deadManSwitch.Status = HeartbeatStatusHealthy
	hm.deadManSwitch.LastCheckIn = time.Now()
	hm.deadManSwitch.MissedCount = 0
	hm.deadManSwitch.TriggeredAt = nil
	hm.deadManSwitch.RecoveryCode = ""
	hm.deadManSwitch.AlertSent = false
	hm.deadManSwitch.AlertSentAt = nil

	return hm.save()
}

// GetLatestHeartbeat returns the most recent heartbeat.
func (hm *HeartbeatMonitor) GetLatestHeartbeat() *Heartbeat {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if len(hm.heartbeats) == 0 {
		return nil
	}
	return &hm.heartbeats[len(hm.heartbeats)-1]
}

// GetHeartbeats returns recent heartbeats.
func (hm *HeartbeatMonitor) GetHeartbeats(limit int) []Heartbeat {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if limit <= 0 || limit > len(hm.heartbeats) {
		limit = len(hm.heartbeats)
	}

	result := make([]Heartbeat, limit)
	copy(result, hm.heartbeats[len(hm.heartbeats)-limit:])
	return result
}

// GetDeadManSwitchStatus returns the current dead man's switch status.
func (hm *HeartbeatMonitor) GetDeadManSwitchStatus() *DeadManSwitch {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	// Return a copy
	dms := *hm.deadManSwitch
	return &dms
}

// VerifyHeartbeat verifies a heartbeat's signature and chain integrity.
func VerifyHeartbeat(hb *Heartbeat, previousHash string, hostPublicKey []byte) error {
	// Verify chain
	if hb.PreviousHash != previousHash {
		return fmt.Errorf("chain broken: expected previous hash %s, got %s", previousHash, hb.PreviousHash)
	}

	// Verify content hash
	computed, err := computeHeartbeatHash(hb)
	if err != nil {
		return err
	}
	if computed != hb.ContentHash {
		return errors.New("content hash mismatch")
	}

	// Verify signature
	if hb.HostSignature != "" && hostPublicKey != nil {
		sig, err := hex.DecodeString(hb.HostSignature)
		if err != nil {
			return fmt.Errorf("invalid signature encoding: %w", err)
		}
		if !crypto.Verify(hostPublicKey, []byte(hb.ContentHash), sig) {
			return errors.New("signature verification failed")
		}
	}

	return nil
}

// GetStats returns heartbeat monitoring statistics.
func (hm *HeartbeatMonitor) GetStats() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var lastHeartbeatTime *time.Time
	if len(hm.heartbeats) > 0 {
		t := hm.heartbeats[len(hm.heartbeats)-1].Timestamp
		lastHeartbeatTime = &t
	}

	return map[string]interface{}{
		"enabled":              hm.config.Enabled,
		"total_heartbeats":     len(hm.heartbeats),
		"current_sequence":     hm.sequence,
		"last_heartbeat_time":  lastHeartbeatTime,
		"dead_man_status":      hm.deadManSwitch.Status,
		"missed_count":         hm.deadManSwitch.MissedCount,
		"last_check_in":        hm.deadManSwitch.LastCheckIn,
	}
}
