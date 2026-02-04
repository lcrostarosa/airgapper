// Package integrity provides mechanisms for verifying backup data integrity
// and detecting tampering or corruption.
package integrity

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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
