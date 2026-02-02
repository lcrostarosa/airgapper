package verification

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLocalImmutableStorage_Write(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "immutable-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalImmutableStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	ctx := context.Background()
	err = storage.Write(ctx, "test-key", []byte("test data"), 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Verify key exists
	exists, err := storage.Exists(ctx, "test-key")
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if !exists {
		t.Error("key should exist after write")
	}
}

func TestLocalImmutableStorage_GetRetention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "immutable-retention-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalImmutableStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	ctx := context.Background()
	retention := 48 * time.Hour
	err = storage.Write(ctx, "retention-key", []byte("data"), retention)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	retentionEnd, err := storage.GetRetention(ctx, "retention-key")
	if err != nil {
		t.Fatalf("failed to get retention: %v", err)
	}

	expectedEnd := time.Now().Add(retention)
	diff := retentionEnd.Sub(expectedEnd)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("retention end time off by %v", diff)
	}
}

func TestLocalImmutableStorage_List(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "immutable-list-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalImmutableStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	ctx := context.Background()
	storage.Write(ctx, "audit/entry1", []byte("data1"), time.Hour)
	storage.Write(ctx, "audit/entry2", []byte("data2"), time.Hour)
	storage.Write(ctx, "snapshots/snap1", []byte("data3"), time.Hour)

	// List all
	all, err := storage.List(ctx, "")
	if err != nil {
		t.Fatalf("failed to list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 keys, got %d", len(all))
	}

	// List by prefix
	auditKeys, err := storage.List(ctx, "audit/")
	if err != nil {
		t.Fatalf("failed to list audit: %v", err)
	}
	if len(auditKeys) != 2 {
		t.Errorf("expected 2 audit keys, got %d", len(auditKeys))
	}
}

func TestLocalImmutableStorage_Verify(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "immutable-verify-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalImmutableStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	ctx := context.Background()
	storage.Write(ctx, "verify-key", []byte("data"), time.Hour)

	// Manually set hash in record for test
	storage.records["verify-key"].Hash = "expectedhash"

	valid, err := storage.Verify(ctx, "verify-key", "expectedhash")
	if err != nil {
		t.Fatalf("failed to verify: %v", err)
	}
	if !valid {
		t.Error("verification should pass with correct hash")
	}

	valid, err = storage.Verify(ctx, "verify-key", "wronghash")
	if err != nil {
		t.Fatalf("failed to verify: %v", err)
	}
	if valid {
		t.Error("verification should fail with wrong hash")
	}
}

func TestLocalImmutableStorage_NotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "immutable-notfound-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalImmutableStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	ctx := context.Background()

	exists, err := storage.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if exists {
		t.Error("nonexistent key should not exist")
	}

	_, err = storage.GetRetention(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}

	_, err = storage.Read(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestLocalImmutableStorage_EmptyBasePath(t *testing.T) {
	_, err := NewLocalImmutableStorage("")
	if err == nil {
		t.Error("expected error for empty base path")
	}
}

func TestAuditChainImmutableBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-backup-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalImmutableStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	backup := NewAuditChainImmutableBackup(storage, 365)
	if backup == nil {
		t.Fatal("backup should not be nil")
	}

	ctx := context.Background()
	entries := []ChainedAuditEntry{
		{ID: "entry1", Sequence: 1, ContentHash: "hash1"},
		{ID: "entry2", Sequence: 2, ContentHash: "hash2"},
	}

	err = backup.BackupEntries(ctx, entries)
	if err != nil {
		t.Fatalf("failed to backup entries: %v", err)
	}

	// Backup same entries again (should skip already backed up)
	err = backup.BackupEntries(ctx, entries)
	if err != nil {
		t.Fatalf("failed to backup again: %v", err)
	}

	// Verify backup exists
	exists, err := storage.Exists(ctx, "audit/entry1")
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if !exists {
		t.Error("audit entry should be backed up")
	}
}

func TestAuditChainImmutableBackup_VerifyBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "audit-verify-backup-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalImmutableStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	backup := NewAuditChainImmutableBackup(storage, 365)

	ctx := context.Background()
	entries := []ChainedAuditEntry{
		{ID: "entry1", Sequence: 1, ContentHash: "hash1"},
	}

	// Backup entries
	backup.BackupEntries(ctx, entries)

	// Set hash to match
	storage.records["audit/entry1"].Hash = "hash1"

	// Verify should pass
	valid, mismatches, err := backup.VerifyBackup(ctx, entries)
	if err != nil {
		t.Fatalf("failed to verify backup: %v", err)
	}
	if !valid {
		t.Error("backup verification should pass")
	}
	if len(mismatches) > 0 {
		t.Errorf("unexpected mismatches: %v", mismatches)
	}
}

func TestSnapshotImmutableBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "snapshot-backup-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalImmutableStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	backup := NewSnapshotImmutableBackup(storage, 90)
	if backup == nil {
		t.Fatal("backup should not be nil")
	}

	ctx := context.Background()

	// Record a snapshot
	metadata := []byte(`{"id": "snap123", "time": "2024-01-01T00:00:00Z"}`)
	err = backup.RecordSnapshot(ctx, "snap123", metadata)
	if err != nil {
		t.Fatalf("failed to record snapshot: %v", err)
	}

	// Verify snapshot existed
	existed, retention, err := backup.VerifySnapshotExisted(ctx, "snap123")
	if err != nil {
		t.Fatalf("failed to verify snapshot: %v", err)
	}
	if !existed {
		t.Error("snapshot should exist")
	}
	if retention.IsZero() {
		t.Error("retention time should be set")
	}

	// Non-existent snapshot
	existed, _, err = backup.VerifySnapshotExisted(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("failed to verify nonexistent: %v", err)
	}
	if existed {
		t.Error("nonexistent snapshot should not exist")
	}
}

func TestImmutableStorageConfig(t *testing.T) {
	cfg := &ImmutableStorageConfig{
		Enabled:              true,
		Provider:             "s3",
		Bucket:               "my-bucket",
		Region:               "us-east-1",
		DefaultRetentionDays: 365,
		GovernanceMode:       false,
	}

	if cfg.Provider != "s3" {
		t.Errorf("expected provider s3, got %s", cfg.Provider)
	}
	if cfg.DefaultRetentionDays != 365 {
		t.Errorf("expected 365 days retention, got %d", cfg.DefaultRetentionDays)
	}
	if cfg.GovernanceMode {
		t.Error("expected compliance mode (GovernanceMode=false)")
	}
}
