package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// computeDataMerkleRoot computes a merkle root of all data file names
func computeDataMerkleRoot(dataPath string) (string, int) {
	var names []string

	_ = filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
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
func countDataFiles(dataPath string) int {
	count := 0
	_ = filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		count++
		return nil
	})
	return count
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
