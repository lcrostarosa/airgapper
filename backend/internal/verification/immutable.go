package verification

import (
	"context"
	"errors"
	"time"
)

// ImmutableStorage defines an interface for write-once storage backends.
// Implementations can wrap S3 Object Lock, Azure Immutable Blob Storage,
// or dedicated WORM storage systems.
type ImmutableStorage interface {
	// Write stores data with an immutability lock.
	// The data cannot be deleted or modified until the retention period expires.
	Write(ctx context.Context, key string, data []byte, retention time.Duration) error

	// Read retrieves data by key.
	Read(ctx context.Context, key string) ([]byte, error)

	// GetRetention returns when the object's immutability lock expires.
	GetRetention(ctx context.Context, key string) (time.Time, error)

	// Exists checks if a key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// List returns keys matching a prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// Verify confirms the data hasn't been tampered with (if supported).
	Verify(ctx context.Context, key string, expectedHash string) (bool, error)
}

// ImmutableStorageConfig configures immutable storage backends.
type ImmutableStorageConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"` // "s3", "azure", "gcs", "local"

	// Provider-specific settings
	Endpoint  string `json:"endpoint,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	Region    string `json:"region,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`

	// Retention settings
	DefaultRetentionDays int  `json:"default_retention_days"`
	GovernanceMode       bool `json:"governance_mode"` // Allow privileged deletion (false = compliance mode)

	// Local WORM simulation (for testing/development)
	LocalPath string `json:"local_path,omitempty"`
}

// ImmutableRecord tracks what was stored in immutable storage.
type ImmutableRecord struct {
	Key           string    `json:"key"`
	Hash          string    `json:"hash"`
	StoredAt      time.Time `json:"stored_at"`
	RetentionEnds time.Time `json:"retention_ends"`
	Provider      string    `json:"provider"`
	Size          int64     `json:"size"`
}

// ErrImmutableLocked is returned when attempting to modify locked data.
var ErrImmutableLocked = errors.New("object is immutably locked")

// ErrRetentionNotExpired is returned when deletion is attempted before retention expires.
var ErrRetentionNotExpired = errors.New("retention period has not expired")

// LocalImmutableStorage implements ImmutableStorage for local testing.
// WARNING: This is NOT truly immutable - it's for development/testing only.
// For production, use a proper WORM storage backend (S3 Object Lock, etc.)
type LocalImmutableStorage struct {
	basePath string
	records  map[string]*ImmutableRecord
}

// NewLocalImmutableStorage creates a local (simulated) immutable storage.
// This is for testing only - does not provide true immutability.
func NewLocalImmutableStorage(basePath string) (*LocalImmutableStorage, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	return &LocalImmutableStorage{
		basePath: basePath,
		records:  make(map[string]*ImmutableRecord),
	}, nil
}

func (s *LocalImmutableStorage) Write(ctx context.Context, key string, data []byte, retention time.Duration) error {
	// In a real implementation, this would use the underlying storage's
	// object lock feature. Here we just track the retention.
	now := time.Now()
	s.records[key] = &ImmutableRecord{
		Key:           key,
		StoredAt:      now,
		RetentionEnds: now.Add(retention),
		Provider:      "local",
		Size:          int64(len(data)),
	}
	// Would actually write to disk here
	return nil
}

func (s *LocalImmutableStorage) Read(ctx context.Context, key string) ([]byte, error) {
	if _, exists := s.records[key]; !exists {
		return nil, errors.New("key not found")
	}
	// Would actually read from disk here
	return nil, errors.New("not implemented")
}

func (s *LocalImmutableStorage) GetRetention(ctx context.Context, key string) (time.Time, error) {
	record, exists := s.records[key]
	if !exists {
		return time.Time{}, errors.New("key not found")
	}
	return record.RetentionEnds, nil
}

func (s *LocalImmutableStorage) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := s.records[key]
	return exists, nil
}

func (s *LocalImmutableStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range s.records {
		if len(prefix) == 0 || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *LocalImmutableStorage) Verify(ctx context.Context, key string, expectedHash string) (bool, error) {
	record, exists := s.records[key]
	if !exists {
		return false, errors.New("key not found")
	}
	return record.Hash == expectedHash, nil
}

// AuditChainImmutableBackup backs up audit chain entries to immutable storage.
type AuditChainImmutableBackup struct {
	storage         ImmutableStorage
	retentionDays   int
	lastBackedUp    uint64
}

// NewAuditChainImmutableBackup creates an immutable backup handler for audit chains.
func NewAuditChainImmutableBackup(storage ImmutableStorage, retentionDays int) *AuditChainImmutableBackup {
	return &AuditChainImmutableBackup{
		storage:       storage,
		retentionDays: retentionDays,
	}
}

// BackupEntries backs up audit entries to immutable storage.
func (b *AuditChainImmutableBackup) BackupEntries(ctx context.Context, entries []ChainedAuditEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Only backup new entries
	var toBackup []ChainedAuditEntry
	for _, e := range entries {
		if e.Sequence > b.lastBackedUp {
			toBackup = append(toBackup, e)
		}
	}

	if len(toBackup) == 0 {
		return nil
	}

	// In a real implementation, we would serialize and store entries
	// with appropriate keys for retrieval
	retention := time.Duration(b.retentionDays) * 24 * time.Hour

	for _, e := range toBackup {
		key := "audit/" + e.ID
		// Would serialize entry here
		data := []byte{} // Placeholder

		if err := b.storage.Write(ctx, key, data, retention); err != nil {
			return err
		}
		b.lastBackedUp = e.Sequence
	}

	return nil
}

// VerifyBackup verifies that backed-up entries match current entries.
func (b *AuditChainImmutableBackup) VerifyBackup(ctx context.Context, entries []ChainedAuditEntry) (bool, []string, error) {
	var mismatches []string

	for _, e := range entries {
		key := "audit/" + e.ID
		exists, err := b.storage.Exists(ctx, key)
		if err != nil {
			return false, nil, err
		}

		if !exists {
			mismatches = append(mismatches, e.ID)
			continue
		}

		// Would verify hash here
		valid, err := b.storage.Verify(ctx, key, e.ContentHash)
		if err != nil {
			return false, nil, err
		}

		if !valid {
			mismatches = append(mismatches, e.ID)
		}
	}

	return len(mismatches) == 0, mismatches, nil
}

// SnapshotImmutableBackup backs up snapshot metadata to immutable storage.
type SnapshotImmutableBackup struct {
	storage       ImmutableStorage
	retentionDays int
}

// NewSnapshotImmutableBackup creates an immutable backup handler for snapshot metadata.
func NewSnapshotImmutableBackup(storage ImmutableStorage, retentionDays int) *SnapshotImmutableBackup {
	return &SnapshotImmutableBackup{
		storage:       storage,
		retentionDays: retentionDays,
	}
}

// RecordSnapshot records snapshot metadata in immutable storage.
// This provides tamper-proof evidence that a snapshot existed.
func (b *SnapshotImmutableBackup) RecordSnapshot(ctx context.Context, snapshotID string, metadata []byte) error {
	key := "snapshots/" + snapshotID
	retention := time.Duration(b.retentionDays) * 24 * time.Hour
	return b.storage.Write(ctx, key, metadata, retention)
}

// VerifySnapshotExisted checks if a snapshot was previously recorded.
// This can detect unauthorized deletions.
func (b *SnapshotImmutableBackup) VerifySnapshotExisted(ctx context.Context, snapshotID string) (bool, time.Time, error) {
	key := "snapshots/" + snapshotID
	exists, err := b.storage.Exists(ctx, key)
	if err != nil {
		return false, time.Time{}, err
	}

	if !exists {
		return false, time.Time{}, nil
	}

	retention, err := b.storage.GetRetention(ctx, key)
	return true, retention, err
}
