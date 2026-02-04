// Package integrity provides mechanisms for verifying backup data integrity
// and detecting tampering or corruption.
package integrity

import (
	"time"
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
