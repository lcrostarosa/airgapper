package integrity

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
	dataCount := countDataFiles(filepath.Join(repoPath, "data"))
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
	currentMerkle, currentCount := computeDataMerkleRoot(filepath.Join(repoPath, "data"))

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
