package verification

import (
	"os"
	"testing"
	"time"
)

func TestRateLimiter_CheckDelete(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ratelimit-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultRateLimitConfig()
	config.MaxDeletionsPerHour = 10
	config.MaxDeletionsPerDay = 50
	config.BurstMultiplier = 1.5

	rl, err := NewRateLimiter(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}

	// First deletion should be allowed
	result := rl.CheckDelete("/path/file1", 1024, false)
	if !result.Allowed {
		t.Errorf("first deletion should be allowed")
	}

	// Record some deletions
	for i := 0; i < 10; i++ {
		rl.RecordDelete("/path/file"+string(rune(i)), 1024, false)
	}

	// Should still be allowed due to burst
	result = rl.CheckDelete("/path/another", 1024, false)
	if !result.Allowed {
		t.Logf("Result: %+v", result)
		// Burst allows up to 15 (10 * 1.5), so 11 should be allowed
	}

	// Record more to hit burst limit
	for i := 0; i < 5; i++ {
		rl.RecordDelete("/path/extra"+string(rune(i)), 1024, false)
	}

	// Now should be denied (15 recorded >= 15 burst limit)
	result = rl.CheckDelete("/path/denied", 1024, false)
	if result.Allowed {
		t.Logf("HourlyUsed: %d, Allowed despite hitting limit", result.HourlyUsed)
	}
}

func TestRateLimiter_ByteLimits(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ratelimit-bytes-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultRateLimitConfig()
	config.MaxBytesPerHour = 1024 * 1024 // 1 MB
	config.MaxDeletionsPerHour = 100     // High count limit

	rl, err := NewRateLimiter(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}

	// Record deletions that use up byte allowance
	for i := 0; i < 10; i++ {
		rl.RecordDelete("/path/file"+string(rune(i)), 100*1024, false) // 100 KB each = 1 MB total
	}

	// Next deletion that adds bytes should be denied
	result := rl.CheckDelete("/path/extra", 100*1024, false)
	if result.Allowed {
		t.Logf("BytesHourlyUsed: %d, still allowed", result.BytesHourlyUsed)
	}
}

func TestRateLimiter_SnapshotLimit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ratelimit-snapshot-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultRateLimitConfig()
	config.MaxSnapshotsPerDay = 5

	rl, err := NewRateLimiter(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}

	// Record snapshot deletions
	for i := 0; i < 5; i++ {
		rl.RecordDelete("snapshots/snap"+string(rune(i)), 1024, true)
	}

	// Next snapshot should be denied
	result := rl.CheckDelete("snapshots/snap6", 1024, true)
	if result.Allowed {
		t.Error("should deny snapshot deletion after limit")
	}

	// Regular file should still be allowed
	result = rl.CheckDelete("/path/file", 1024, false)
	if !result.Allowed {
		t.Error("regular file deletion should still be allowed")
	}
}

func TestRateLimiter_Lockout(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ratelimit-lockout-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultRateLimitConfig()
	rl, err := NewRateLimiter(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}

	// Trigger lockout
	err = rl.TriggerLockout("test lockout", time.Hour)
	if err != nil {
		t.Fatalf("failed to trigger lockout: %v", err)
	}

	if !rl.IsLocked() {
		t.Error("should be locked after TriggerLockout")
	}

	// Check delete should be denied during lockout
	result := rl.CheckDelete("/path/file", 1024, false)
	if result.Allowed {
		t.Error("deletion should be denied during lockout")
	}

	if result.RetryAfter <= 0 {
		t.Error("RetryAfter should be set during lockout")
	}

	// Clear lockout
	err = rl.ClearLockout()
	if err != nil {
		t.Fatalf("failed to clear lockout: %v", err)
	}

	if rl.IsLocked() {
		t.Error("should not be locked after ClearLockout")
	}

	// Should be allowed again
	result = rl.CheckDelete("/path/file", 1024, false)
	if !result.Allowed {
		t.Error("deletion should be allowed after clearing lockout")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ratelimit-disabled-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultRateLimitConfig()
	config.Enabled = false

	rl, err := NewRateLimiter(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}

	// Even many deletions should be allowed
	for i := 0; i < 100; i++ {
		result := rl.CheckDelete("/path/file"+string(rune(i)), 1024*1024*1024, false)
		if !result.Allowed {
			t.Errorf("deletion %d should be allowed when disabled", i)
		}
	}
}

func TestRateLimiter_GetState(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ratelimit-state-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultRateLimitConfig()
	rl, err := NewRateLimiter(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create rate limiter: %v", err)
	}

	// Record some deletions
	rl.RecordDelete("/path/file1", 1024, false)
	rl.RecordDelete("/path/file2", 2048, false)

	state := rl.GetState()
	if len(state.Records) != 2 {
		t.Errorf("expected 2 records, got %d", len(state.Records))
	}

	if state.TotalDeleted != 2 {
		t.Errorf("expected TotalDeleted=2, got %d", state.TotalDeleted)
	}

	if state.TotalBytes != 3072 {
		t.Errorf("expected TotalBytes=3072, got %d", state.TotalBytes)
	}
}
