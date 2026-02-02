// Package integrity provides mechanisms for verifying backup data integrity
// and detecting tampering or corruption.
package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// CheckResult represents the result of an integrity check
type CheckResult struct {
	Timestamp    time.Time `json:"timestamp"`
	RepoPath     string    `json:"repoPath"`
	TotalFiles   int       `json:"totalFiles"`
	CheckedFiles int       `json:"checkedFiles"`
	CorruptFiles int       `json:"corruptFiles"`
	MissingFiles int       `json:"missingFiles"`
	Errors       []string  `json:"errors,omitempty"`
	Duration     string    `json:"duration"`
	Passed       bool      `json:"passed"`
}

// VerificationRecord is a signed record of expected backup state
// Created by the owner after each successful backup
type VerificationRecord struct {
	ID         string    `json:"id"`
	SnapshotID string    `json:"snapshotId"`
	CreatedAt  time.Time `json:"createdAt"`
	OwnerKeyID string    `json:"ownerKeyId"`

	// Hashes of critical files
	ConfigHash   string   `json:"configHash"`
	KeyHashes    []string `json:"keyHashes"`    // Sorted hashes of key files
	SnapshotHash string   `json:"snapshotHash"` // Hash of the snapshot file

	// Merkle root of all data blob names (not contents - too expensive)
	DataMerkleRoot string `json:"dataMerkleRoot"`
	DataFileCount  int    `json:"dataFileCount"`

	// Owner signature over this record
	Signature string `json:"signature,omitempty"`
}

// Checker performs integrity verification
type Checker struct {
	basePath string
	mu       sync.RWMutex

	// History of check results
	checkHistory []CheckResult
	maxHistory   int

	// Verification records (owner-signed)
	records     map[string]*VerificationRecord // keyed by snapshot ID
	recordsPath string
}

// NewChecker creates a new integrity checker
func NewChecker(basePath string) (*Checker, error) {
	if basePath == "" {
		return nil, fmt.Errorf("base path required")
	}

	c := &Checker{
		basePath:    basePath,
		maxHistory:  100,
		records:     make(map[string]*VerificationRecord),
		recordsPath: filepath.Join(basePath, ".airgapper-verification-records.json"),
	}

	// Load existing records
	c.loadRecords()

	return c, nil
}

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

// CheckDataIntegrity verifies that all data blobs match their SHA256 names
// This is the most thorough check - verifies actual file contents
func (c *Checker) CheckDataIntegrity(repoName string) (*CheckResult, error) {
	start := time.Now()
	result := &CheckResult{
		Timestamp: start,
		RepoPath:  filepath.Join(c.basePath, repoName),
	}

	dataPath := filepath.Join(c.basePath, repoName, "data")

	// Walk all data files
	err := filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("walk error: %v", err))
			return nil
		}

		if info.IsDir() {
			return nil
		}

		result.TotalFiles++

		// The filename should be the SHA256 hash of the content
		expectedHash := info.Name()

		// Compute actual hash
		actualHash, err := hashFile(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("hash error %s: %v", info.Name(), err))
			return nil
		}

		result.CheckedFiles++

		if actualHash != expectedHash {
			result.CorruptFiles++
			result.Errors = append(result.Errors,
				fmt.Sprintf("CORRUPT: %s (expected hash doesn't match content)", info.Name()))
		}

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("walk failed: %v", err))
	}

	result.Duration = time.Since(start).String()
	result.Passed = result.CorruptFiles == 0 && result.MissingFiles == 0

	c.addToHistory(*result)

	return result, nil
}

// QuickCheck performs a fast check without reading file contents
// Verifies expected files exist based on verification record
func (c *Checker) QuickCheck(repoName, snapshotID string) (*CheckResult, error) {
	start := time.Now()
	result := &CheckResult{
		Timestamp: start,
		RepoPath:  filepath.Join(c.basePath, repoName),
	}

	record := c.GetVerificationRecord(snapshotID)
	if record == nil {
		result.Errors = append(result.Errors, "no verification record for snapshot")
		result.Passed = false
		result.Duration = time.Since(start).String()
		return result, nil
	}

	repoPath := filepath.Join(c.basePath, repoName)

	// Check config hash
	configPath := filepath.Join(repoPath, "config")
	if configHash, err := hashFile(configPath); err != nil {
		result.MissingFiles++
		result.Errors = append(result.Errors, "config file missing or unreadable")
	} else if configHash != record.ConfigHash {
		result.CorruptFiles++
		result.Errors = append(result.Errors, "config file hash mismatch")
	}
	result.TotalFiles++
	result.CheckedFiles++

	// Check snapshot file
	snapshotPath := filepath.Join(repoPath, "snapshots", snapshotID)
	if snapshotHash, err := hashFile(snapshotPath); err != nil {
		result.MissingFiles++
		result.Errors = append(result.Errors, fmt.Sprintf("snapshot %s missing", snapshotID))
	} else if snapshotHash != record.SnapshotHash {
		result.CorruptFiles++
		result.Errors = append(result.Errors, "snapshot file hash mismatch")
	}
	result.TotalFiles++
	result.CheckedFiles++

	// Check key files
	for _, expectedHash := range record.KeyHashes {
		keyPath := filepath.Join(repoPath, "keys", expectedHash)
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			result.MissingFiles++
			result.Errors = append(result.Errors, fmt.Sprintf("key file %s missing", expectedHash))
		}
		result.TotalFiles++
		result.CheckedFiles++
	}

	// Check data file count
	dataCount := c.countDataFiles(filepath.Join(repoPath, "data"))
	if dataCount < record.DataFileCount {
		result.MissingFiles += record.DataFileCount - dataCount
		result.Errors = append(result.Errors,
			fmt.Sprintf("data files missing: expected %d, found %d", record.DataFileCount, dataCount))
	}
	result.TotalFiles += record.DataFileCount
	result.CheckedFiles += dataCount

	result.Duration = time.Since(start).String()
	result.Passed = result.CorruptFiles == 0 && result.MissingFiles == 0

	c.addToHistory(*result)

	return result, nil
}

// VerifyAgainstRecord verifies current state matches a verification record
// Returns detailed comparison
func (c *Checker) VerifyAgainstRecord(repoName string, record *VerificationRecord) (*CheckResult, error) {
	start := time.Now()
	result := &CheckResult{
		Timestamp: start,
		RepoPath:  filepath.Join(c.basePath, repoName),
	}

	repoPath := filepath.Join(c.basePath, repoName)

	// Compute current merkle root of data files
	currentMerkle, currentCount := c.computeDataMerkleRoot(filepath.Join(repoPath, "data"))

	if currentMerkle != record.DataMerkleRoot {
		result.Errors = append(result.Errors,
			fmt.Sprintf("data merkle root mismatch: expected %s, got %s",
				record.DataMerkleRoot, currentMerkle))
		result.CorruptFiles++
	}

	if currentCount != record.DataFileCount {
		result.Errors = append(result.Errors,
			fmt.Sprintf("data file count mismatch: expected %d, got %d",
				record.DataFileCount, currentCount))
		if currentCount < record.DataFileCount {
			result.MissingFiles = record.DataFileCount - currentCount
		}
	}

	result.TotalFiles = record.DataFileCount
	result.CheckedFiles = currentCount
	result.Duration = time.Since(start).String()
	result.Passed = len(result.Errors) == 0

	c.addToHistory(*result)

	return result, nil
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
	record.DataMerkleRoot, record.DataFileCount = c.computeDataMerkleRoot(filepath.Join(repoPath, "data"))

	return record, nil
}

// computeDataMerkleRoot computes a merkle root of all data file names
func (c *Checker) computeDataMerkleRoot(dataPath string) (string, int) {
	var names []string

	filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		names = append(names, info.Name())
		return nil
	})

	if len(names) == 0 {
		return "", 0
	}

	sort.Strings(names)

	// Build merkle tree
	hashes := make([][]byte, len(names))
	for i, name := range names {
		h := sha256.Sum256([]byte(name))
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

	return hex.EncodeToString(hashes[0]), len(names)
}

// countDataFiles counts files in the data directory
func (c *Checker) countDataFiles(dataPath string) int {
	count := 0
	filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		count++
		return nil
	})
	return count
}

// addToHistory adds a result to the check history
func (c *Checker) addToHistory(result CheckResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.checkHistory = append(c.checkHistory, result)
	if len(c.checkHistory) > c.maxHistory {
		c.checkHistory = c.checkHistory[1:]
	}
}

// GetHistory returns recent check results
func (c *Checker) GetHistory(limit int) []CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.checkHistory) {
		limit = len(c.checkHistory)
	}

	start := len(c.checkHistory) - limit
	result := make([]CheckResult, limit)
	copy(result, c.checkHistory[start:])
	return result
}

// hashFile computes SHA256 hash of a file
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// ScheduledChecker runs periodic integrity checks
type ScheduledChecker struct {
	checker  *Checker
	repoName string
	interval time.Duration
	stopChan chan struct{}
	running  bool
	mu       sync.Mutex

	// Callback for alerts
	onCorruption func(result *CheckResult)
}

// NewScheduledChecker creates a scheduled checker
func NewScheduledChecker(checker *Checker, repoName string, interval time.Duration) *ScheduledChecker {
	return &ScheduledChecker{
		checker:  checker,
		repoName: repoName,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// SetCorruptionCallback sets a callback to be called when corruption is detected
func (sc *ScheduledChecker) SetCorruptionCallback(cb func(result *CheckResult)) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.onCorruption = cb
}

// Start begins scheduled checking
func (sc *ScheduledChecker) Start() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.running {
		return
	}
	sc.running = true

	go sc.run()
}

// Stop stops scheduled checking
func (sc *ScheduledChecker) Stop() {
	sc.mu.Lock()
	if !sc.running {
		sc.mu.Unlock()
		return
	}
	sc.running = false
	sc.mu.Unlock()

	close(sc.stopChan)
}

func (sc *ScheduledChecker) run() {
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	// Run initial check
	sc.runCheck()

	for {
		select {
		case <-ticker.C:
			sc.runCheck()
		case <-sc.stopChan:
			return
		}
	}
}

func (sc *ScheduledChecker) runCheck() {
	result, err := sc.checker.CheckDataIntegrity(sc.repoName)
	if err != nil {
		return
	}

	if !result.Passed && sc.onCorruption != nil {
		sc.onCorruption(result)
	}
}
