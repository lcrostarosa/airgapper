package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// loadRecords loads verification records from disk
func (c *Checker) loadRecords() {
	data, err := os.ReadFile(c.recordsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.Debug("failed to read verification records", logging.Err(err))
		}
		return
	}

	var records []*VerificationRecord
	if err := json.Unmarshal(data, &records); err != nil {
		logging.Debug("failed to parse verification records", logging.Err(err))
		return
	}

	for _, r := range records {
		c.records[r.SnapshotID] = r
	}
}

// saveRecords saves verification records to disk
func (c *Checker) saveRecords() error {
	c.mu.RLock()
	records := make([]*VerificationRecord, 0, len(c.records))
	for _, r := range c.records {
		records = append(records, r)
	}
	c.mu.RUnlock()

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.recordsPath, data, 0600)
}

// AddVerificationRecord adds a signed verification record
func (c *Checker) AddVerificationRecord(record *VerificationRecord, ownerPubKey []byte) error {
	// Verify signature
	if record.Signature == "" {
		return fmt.Errorf("record must be signed")
	}

	// Compute expected hash
	hash, err := c.hashRecord(record)
	if err != nil {
		return err
	}

	sig, err := hex.DecodeString(record.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !crypto.Verify(ownerPubKey, hash, sig) {
		return fmt.Errorf("signature verification failed")
	}

	c.mu.Lock()
	c.records[record.SnapshotID] = record
	c.mu.Unlock()

	return c.saveRecords()
}

// hashRecord computes the hash for signing
func (c *Checker) hashRecord(r *VerificationRecord) ([]byte, error) {
	// Create canonical data for hashing (exclude signature)
	data := struct {
		ID             string   `json:"id"`
		SnapshotID     string   `json:"snapshotId"`
		CreatedAt      int64    `json:"createdAt"`
		OwnerKeyID     string   `json:"ownerKeyId"`
		ConfigHash     string   `json:"configHash"`
		KeyHashes      []string `json:"keyHashes"`
		SnapshotHash   string   `json:"snapshotHash"`
		DataMerkleRoot string   `json:"dataMerkleRoot"`
		DataFileCount  int      `json:"dataFileCount"`
	}{
		ID:             r.ID,
		SnapshotID:     r.SnapshotID,
		CreatedAt:      r.CreatedAt.Unix(),
		OwnerKeyID:     r.OwnerKeyID,
		ConfigHash:     r.ConfigHash,
		KeyHashes:      r.KeyHashes,
		SnapshotHash:   r.SnapshotHash,
		DataMerkleRoot: r.DataMerkleRoot,
		DataFileCount:  r.DataFileCount,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(jsonBytes)
	return hash[:], nil
}

// GetVerificationRecord returns a verification record for a snapshot
func (c *Checker) GetVerificationRecord(snapshotID string) *VerificationRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.records[snapshotID]
}

// CreateVerificationRecord creates a new verification record for a snapshot
// The owner should sign this after receiving it
func (c *Checker) CreateVerificationRecord(repoName, snapshotID, ownerKeyID string) (*VerificationRecord, error) {
	repoPath := filepath.Join(c.basePath, repoName)

	record := &VerificationRecord{
		ID:         fmt.Sprintf("%x", sha256.Sum256([]byte(time.Now().String())))[:16],
		SnapshotID: snapshotID,
		CreatedAt:  time.Now(),
		OwnerKeyID: ownerKeyID,
	}

	// Hash config file
	configHash, err := hashFile(filepath.Join(repoPath, "config"))
	if err != nil {
		return nil, fmt.Errorf("failed to hash config: %w", err)
	}
	record.ConfigHash = configHash

	// Hash snapshot file
	snapshotHash, err := hashFile(filepath.Join(repoPath, "snapshots", snapshotID))
	if err != nil {
		return nil, fmt.Errorf("failed to hash snapshot: %w", err)
	}
	record.SnapshotHash = snapshotHash

	// Collect key file hashes
	keysPath := filepath.Join(repoPath, "keys")
	entries, err := os.ReadDir(keysPath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				record.KeyHashes = append(record.KeyHashes, entry.Name())
			}
		}
		sort.Strings(record.KeyHashes)
	}

	// Compute data merkle root
	record.DataMerkleRoot, record.DataFileCount = computeDataMerkleRoot(filepath.Join(repoPath, "data"))

	return record, nil
}
