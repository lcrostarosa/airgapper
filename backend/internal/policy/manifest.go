// Package policy - manifest.go
// Implements backup manifests for integrity verification.
// Owner maintains a signed record of all backups, allowing detection
// of unauthorized deletions by the host.

package policy

import (
	"time"
)

// SnapshotEntry represents a single backup snapshot in the manifest
type SnapshotEntry struct {
	ID        string    `json:"id"`         // Restic snapshot ID
	CreatedAt time.Time `json:"created_at"` // When backup was created
	Paths     []string  `json:"paths"`      // Paths that were backed up
	Tags      []string  `json:"tags,omitempty"`
	Size      int64     `json:"size"` // Total size in bytes

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

// IntegrityReport contains the result of verifying backups against manifest
type IntegrityReport struct {
	Verified        bool      `json:"verified"`
	CheckedAt       time.Time `json:"checked_at"`
	TotalInManifest int       `json:"total_in_manifest"`
	TotalOnStorage  int       `json:"total_on_storage"`
	Missing         []string  `json:"missing,omitempty"`    // In manifest but not on storage
	Unexpected      []string  `json:"unexpected,omitempty"` // On storage but not in manifest
	Errors          []string  `json:"errors,omitempty"`
}
