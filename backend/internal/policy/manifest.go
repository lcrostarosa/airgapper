// Package policy - manifest.go
// Implements backup manifests for integrity verification.
// Owner maintains a signed record of all backups, allowing detection
// of unauthorized deletions by the host.

package policy

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
)

// SnapshotEntry represents a single backup snapshot in the manifest
type SnapshotEntry struct {
	ID        string    `json:"id"`         // Restic snapshot ID
	CreatedAt time.Time `json:"created_at"` // When backup was created
	Paths     []string  `json:"paths"`      // Paths that were backed up
	Tags      []string  `json:"tags,omitempty"`
	Size      int64     `json:"size"`       // Total size in bytes

	// Integrity data
	TreeHash string `json:"tree_hash,omitempty"` // Hash of snapshot tree
}

// Manifest tracks all backup snapshots with cryptographic integrity
type Manifest struct {
	// Metadata
	Version   int       `json:"version"`
	PolicyID  string    `json:"policy_id"` // Associated policy
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Snapshots ordered by creation time
	Snapshots []SnapshotEntry `json:"snapshots"`

	// Merkle root of all snapshot IDs for efficient verification
	MerkleRoot string `json:"merkle_root"`

	// Owner signature over the manifest hash
	Signature string `json:"signature,omitempty"`
}

// ManifestManager handles manifest operations
type ManifestManager struct {
	storePath  string
	ownerKeyID string
	privateKey []byte
	publicKey  []byte
	mu         sync.RWMutex
	manifest   *Manifest
}

// NewManifestManager creates a new manifest manager
func NewManifestManager(storePath, ownerKeyID string, privateKey, publicKey []byte) (*ManifestManager, error) {
	if storePath == "" {
		return nil, errors.New("store path required")
	}

	if err := os.MkdirAll(storePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create manifest directory: %w", err)
	}

	mm := &ManifestManager{
		storePath:  storePath,
		ownerKeyID: ownerKeyID,
		privateKey: privateKey,
		publicKey:  publicKey,
	}

	// Try to load existing manifest
	if err := mm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	return mm, nil
}

// manifestPath returns the path to the manifest file
func (mm *ManifestManager) manifestPath() string {
	return filepath.Join(mm.storePath, "manifest.json")
}

// load reads the manifest from disk
func (mm *ManifestManager) load() error {
	data, err := os.ReadFile(mm.manifestPath())
	if err != nil {
		return err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	mm.manifest = &m
	return nil
}

// save writes the manifest to disk
func (mm *ManifestManager) save() error {
	if mm.manifest == nil {
		return errors.New("no manifest to save")
	}

	data, err := json.MarshalIndent(mm.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return os.WriteFile(mm.manifestPath(), data, 0600)
}

// Initialize creates a new manifest
func (mm *ManifestManager) Initialize(policyID string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	now := time.Now()
	mm.manifest = &Manifest{
		Version:   1,
		PolicyID:  policyID,
		OwnerID:   mm.ownerKeyID,
		CreatedAt: now,
		UpdatedAt: now,
		Snapshots: []SnapshotEntry{},
	}

	return mm.signAndSave()
}

// AddSnapshot adds a new snapshot to the manifest
func (mm *ManifestManager) AddSnapshot(entry SnapshotEntry) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.manifest == nil {
		return errors.New("manifest not initialized")
	}

	// Check for duplicate
	for _, s := range mm.manifest.Snapshots {
		if s.ID == entry.ID {
			return fmt.Errorf("snapshot %s already in manifest", entry.ID)
		}
	}

	mm.manifest.Snapshots = append(mm.manifest.Snapshots, entry)
	mm.manifest.UpdatedAt = time.Now()

	return mm.signAndSave()
}

// RemoveSnapshot removes a snapshot from the manifest (for approved deletions)
func (mm *ManifestManager) RemoveSnapshot(snapshotID string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.manifest == nil {
		return errors.New("manifest not initialized")
	}

	found := false
	newSnapshots := make([]SnapshotEntry, 0, len(mm.manifest.Snapshots)-1)
	for _, s := range mm.manifest.Snapshots {
		if s.ID == snapshotID {
			found = true
			continue
		}
		newSnapshots = append(newSnapshots, s)
	}

	if !found {
		return fmt.Errorf("snapshot %s not found in manifest", snapshotID)
	}

	mm.manifest.Snapshots = newSnapshots
	mm.manifest.UpdatedAt = time.Now()

	return mm.signAndSave()
}

// signAndSave computes merkle root, signs, and saves the manifest
func (mm *ManifestManager) signAndSave() error {
	// Compute merkle root
	mm.manifest.MerkleRoot = mm.computeMerkleRoot()

	// Compute manifest hash
	hash, err := mm.manifestHash()
	if err != nil {
		return err
	}

	// Sign
	if mm.privateKey != nil {
		sig, err := crypto.Sign(mm.privateKey, hash)
		if err != nil {
			return fmt.Errorf("failed to sign manifest: %w", err)
		}
		mm.manifest.Signature = hex.EncodeToString(sig)
	}

	return mm.save()
}

// manifestHash computes hash of manifest data (excluding signature)
func (mm *ManifestManager) manifestHash() ([]byte, error) {
	// Create a copy without signature for hashing
	data := struct {
		Version    int               `json:"version"`
		PolicyID   string            `json:"policy_id"`
		OwnerID    string            `json:"owner_id"`
		CreatedAt  int64             `json:"created_at"`
		UpdatedAt  int64             `json:"updated_at"`
		MerkleRoot string            `json:"merkle_root"`
		Snapshots  []SnapshotEntry   `json:"snapshots"`
	}{
		Version:    mm.manifest.Version,
		PolicyID:   mm.manifest.PolicyID,
		OwnerID:    mm.manifest.OwnerID,
		CreatedAt:  mm.manifest.CreatedAt.Unix(),
		UpdatedAt:  mm.manifest.UpdatedAt.Unix(),
		MerkleRoot: mm.manifest.MerkleRoot,
		Snapshots:  mm.manifest.Snapshots,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(jsonBytes)
	return hash[:], nil
}

// computeMerkleRoot creates a merkle tree of snapshot IDs
func (mm *ManifestManager) computeMerkleRoot() string {
	if len(mm.manifest.Snapshots) == 0 {
		return ""
	}

	// Get sorted list of snapshot IDs
	ids := make([]string, len(mm.manifest.Snapshots))
	for i, s := range mm.manifest.Snapshots {
		ids[i] = s.ID
	}
	sort.Strings(ids)

	// Build merkle tree
	hashes := make([][]byte, len(ids))
	for i, id := range ids {
		h := sha256.Sum256([]byte(id))
		hashes[i] = h[:]
	}

	for len(hashes) > 1 {
		var newHashes [][]byte
		for i := 0; i < len(hashes); i += 2 {
			if i+1 < len(hashes) {
				combined := append(hashes[i], hashes[i+1]...)
				h := sha256.Sum256(combined)
				newHashes = append(newHashes, h[:])
			} else {
				newHashes = append(newHashes, hashes[i])
			}
		}
		hashes = newHashes
	}

	return hex.EncodeToString(hashes[0])
}

// Verify checks the manifest signature
func (mm *ManifestManager) Verify() error {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.manifest == nil {
		return errors.New("no manifest loaded")
	}

	if mm.manifest.Signature == "" {
		return errors.New("manifest not signed")
	}

	// Verify merkle root
	expectedRoot := mm.computeMerkleRoot()
	if mm.manifest.MerkleRoot != expectedRoot {
		return errors.New("merkle root mismatch - manifest may be corrupted")
	}

	// Verify signature
	hash, err := mm.manifestHash()
	if err != nil {
		return err
	}

	sig, err := hex.DecodeString(mm.manifest.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !crypto.Verify(mm.publicKey, hash, sig) {
		return errors.New("signature verification failed")
	}

	return nil
}

// IntegrityReport contains the result of verifying backups against manifest
type IntegrityReport struct {
	Verified    bool              `json:"verified"`
	CheckedAt   time.Time         `json:"checked_at"`
	TotalInManifest int           `json:"total_in_manifest"`
	TotalOnStorage  int           `json:"total_on_storage"`
	Missing     []string          `json:"missing,omitempty"`     // In manifest but not on storage
	Unexpected  []string          `json:"unexpected,omitempty"`  // On storage but not in manifest
	Errors      []string          `json:"errors,omitempty"`
}

// CheckIntegrity compares manifest against actual storage
// Takes a function that returns the list of snapshot IDs currently on storage
func (mm *ManifestManager) CheckIntegrity(getStorageSnapshots func() ([]string, error)) (*IntegrityReport, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	report := &IntegrityReport{
		CheckedAt: time.Now(),
	}

	if mm.manifest == nil {
		report.Errors = append(report.Errors, "no manifest loaded")
		return report, nil
	}

	// Verify manifest signature first
	if err := mm.Verify(); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("manifest verification failed: %v", err))
	}

	// Get expected snapshots from manifest
	expectedSet := make(map[string]bool)
	for _, s := range mm.manifest.Snapshots {
		expectedSet[s.ID] = true
	}
	report.TotalInManifest = len(expectedSet)

	// Get actual snapshots from storage
	actualSnapshots, err := getStorageSnapshots()
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("failed to list storage: %v", err))
		return report, nil
	}

	actualSet := make(map[string]bool)
	for _, id := range actualSnapshots {
		actualSet[id] = true
	}
	report.TotalOnStorage = len(actualSet)

	// Find missing (in manifest but not on storage)
	for id := range expectedSet {
		if !actualSet[id] {
			report.Missing = append(report.Missing, id)
		}
	}

	// Find unexpected (on storage but not in manifest)
	for id := range actualSet {
		if !expectedSet[id] {
			report.Unexpected = append(report.Unexpected, id)
		}
	}

	// Sort for consistent output
	sort.Strings(report.Missing)
	sort.Strings(report.Unexpected)

	// Verification passes if no missing and no errors
	report.Verified = len(report.Missing) == 0 && len(report.Errors) == 0

	return report, nil
}

// GetManifest returns a copy of the current manifest
func (mm *ManifestManager) GetManifest() *Manifest {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.manifest == nil {
		return nil
	}

	// Return a copy
	copy := *mm.manifest
	copy.Snapshots = make([]SnapshotEntry, len(mm.manifest.Snapshots))
	for i, s := range mm.manifest.Snapshots {
		copy.Snapshots[i] = s
	}

	return &copy
}

// GetSnapshot returns a specific snapshot entry
func (mm *ManifestManager) GetSnapshot(id string) *SnapshotEntry {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.manifest == nil {
		return nil
	}

	for _, s := range mm.manifest.Snapshots {
		if s.ID == id {
			entry := s // copy
			return &entry
		}
	}

	return nil
}

// SnapshotCount returns the number of snapshots in the manifest
func (mm *ManifestManager) SnapshotCount() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.manifest == nil {
		return 0
	}

	return len(mm.manifest.Snapshots)
}
