package verification

import (
	"os"
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

func TestHeartbeatMonitor_GenerateHeartbeat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	hb, err := hm.GenerateHeartbeat()
	if err != nil {
		t.Fatalf("failed to generate heartbeat: %v", err)
	}

	if hb.ID == "" {
		t.Error("heartbeat should have an ID")
	}

	if hb.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", hb.Sequence)
	}

	if hb.HostSignature == "" {
		t.Error("heartbeat should be signed")
	}

	if hb.ContentHash == "" {
		t.Error("heartbeat should have content hash")
	}

	if hb.Nonce == "" {
		t.Error("heartbeat should have nonce")
	}
}

func TestHeartbeatMonitor_HeartbeatChain(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-chain-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	// Generate multiple heartbeats
	var lastHash string = "genesis"
	for i := 0; i < 5; i++ {
		hb, err := hm.GenerateHeartbeat()
		if err != nil {
			t.Fatalf("failed to generate heartbeat %d: %v", i, err)
		}

		if hb.PreviousHash != lastHash {
			t.Errorf("heartbeat %d: expected previous hash %s, got %s", i, lastHash, hb.PreviousHash)
		}

		lastHash = hb.ContentHash
	}
}

func TestHeartbeatMonitor_VerifyHeartbeat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-verify-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	hb, _ := hm.GenerateHeartbeat()

	// Verify should pass
	err = VerifyHeartbeat(hb, "genesis", hostPub)
	if err != nil {
		t.Errorf("verification should pass: %v", err)
	}

	// Verify with wrong previous hash should fail
	err = VerifyHeartbeat(hb, "wrong-hash", hostPub)
	if err == nil {
		t.Error("verification should fail with wrong previous hash")
	}
}

func TestHeartbeatMonitor_DeadManSwitch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-dms-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	config.IntervalSeconds = 1 // 1 second for testing
	config.WarningThreshold = 2
	config.CriticalThreshold = 4
	config.DeadManThreshold = 6

	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	// Generate initial heartbeat
	hm.GenerateHeartbeat()

	// Initial status should be healthy
	dms := hm.CheckDeadManSwitch()
	if dms.Status != HeartbeatStatusHealthy {
		t.Errorf("expected healthy status, got %s", dms.Status)
	}

	// Simulate time passing without heartbeats by manually setting last check-in
	hm.mu.Lock()
	hm.deadManSwitch.LastCheckIn = time.Now().Add(-3 * time.Second)
	hm.mu.Unlock()

	// Should be warning
	dms = hm.CheckDeadManSwitch()
	if dms.Status != HeartbeatStatusWarning {
		t.Errorf("expected warning status after 3 missed, got %s", dms.Status)
	}

	// Simulate more time
	hm.mu.Lock()
	hm.deadManSwitch.LastCheckIn = time.Now().Add(-5 * time.Second)
	hm.mu.Unlock()

	// Should be critical
	dms = hm.CheckDeadManSwitch()
	if dms.Status != HeartbeatStatusCritical {
		t.Errorf("expected critical status after 5 missed, got %s", dms.Status)
	}

	// Simulate even more time
	hm.mu.Lock()
	hm.deadManSwitch.LastCheckIn = time.Now().Add(-7 * time.Second)
	hm.mu.Unlock()

	// Should trigger dead man switch
	dms = hm.CheckDeadManSwitch()
	if dms.Status != HeartbeatStatusDead {
		t.Errorf("expected dead status after 7 missed, got %s", dms.Status)
	}

	if dms.RecoveryCode == "" {
		t.Error("dead man switch should have recovery code")
	}
}

func TestHeartbeatMonitor_ResetDeadManSwitch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-reset-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	config.IntervalSeconds = 1
	config.DeadManThreshold = 2

	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	hm.GenerateHeartbeat()

	// Trigger dead man switch
	hm.mu.Lock()
	hm.deadManSwitch.LastCheckIn = time.Now().Add(-10 * time.Second)
	hm.mu.Unlock()

	dms := hm.CheckDeadManSwitch()
	if dms.Status != HeartbeatStatusDead {
		t.Fatal("expected dead man switch to trigger")
	}

	recoveryCode := dms.RecoveryCode

	// Try reset with wrong code
	err = hm.ResetDeadManSwitch("wrong-code")
	if err == nil {
		t.Error("reset should fail with wrong code")
	}

	// Reset with correct code
	err = hm.ResetDeadManSwitch(recoveryCode)
	if err != nil {
		t.Fatalf("reset should succeed with correct code: %v", err)
	}

	dms = hm.GetDeadManSwitchStatus()
	if dms.Status != HeartbeatStatusHealthy {
		t.Errorf("expected healthy after reset, got %s", dms.Status)
	}
}

func TestHeartbeatMonitor_StateProviders(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-providers-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	// Set state providers
	hm.SetStateProviders(
		func() (string, uint64) { return "audit-hash-123", 42 },
		func() (int, int64) { return 10, 1024000 },
		func() string { return "all_ok" },
	)

	hb, err := hm.GenerateHeartbeat()
	if err != nil {
		t.Fatalf("failed to generate heartbeat: %v", err)
	}

	if hb.AuditChainHash != "audit-hash-123" {
		t.Errorf("expected audit hash 'audit-hash-123', got '%s'", hb.AuditChainHash)
	}

	if hb.AuditChainSeq != 42 {
		t.Errorf("expected audit seq 42, got %d", hb.AuditChainSeq)
	}

	if hb.SnapshotCount != 10 {
		t.Errorf("expected snapshot count 10, got %d", hb.SnapshotCount)
	}

	if hb.TotalBytes != 1024000 {
		t.Errorf("expected total bytes 1024000, got %d", hb.TotalBytes)
	}

	if hb.CanaryStatus != "all_ok" {
		t.Errorf("expected canary status 'all_ok', got '%s'", hb.CanaryStatus)
	}
}

func TestHeartbeatMonitor_GetStats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-stats-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	// Generate heartbeats
	for i := 0; i < 5; i++ {
		hm.GenerateHeartbeat()
	}

	stats := hm.GetStats()

	if stats["total_heartbeats"].(int) != 5 {
		t.Errorf("expected 5 heartbeats, got %d", stats["total_heartbeats"].(int))
	}

	if stats["current_sequence"].(uint64) != 5 {
		t.Errorf("expected sequence 5, got %d", stats["current_sequence"].(uint64))
	}

	if stats["dead_man_status"].(HeartbeatStatus) != HeartbeatStatusHealthy {
		t.Errorf("expected healthy status, got %s", stats["dead_man_status"].(HeartbeatStatus))
	}
}

func TestHeartbeatMonitor_AlertCallbacks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-callbacks-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	config.IntervalSeconds = 1
	config.WarningThreshold = 2
	config.CriticalThreshold = 4
	config.DeadManThreshold = 10

	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	warningCalled := make(chan bool, 1)
	criticalCalled := make(chan bool, 1)

	hm.SetAlertCallbacks(
		func(status HeartbeatStatus, missed int) { warningCalled <- true },
		func(status HeartbeatStatus, missed int) { criticalCalled <- true },
		nil,
	)

	hm.GenerateHeartbeat()

	// Trigger warning
	hm.mu.Lock()
	hm.deadManSwitch.LastCheckIn = time.Now().Add(-3 * time.Second)
	hm.mu.Unlock()
	hm.CheckDeadManSwitch()

	select {
	case <-warningCalled:
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Error("warning callback not called")
	}

	// Trigger critical
	hm.mu.Lock()
	hm.deadManSwitch.LastCheckIn = time.Now().Add(-5 * time.Second)
	hm.mu.Unlock()
	hm.CheckDeadManSwitch()

	select {
	case <-criticalCalled:
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Error("critical callback not called")
	}
}

func TestHeartbeatMonitor_Disabled(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "heartbeat-disabled-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hostPub, hostPriv, _ := crypto.GenerateKeyPair()
	hostKeyID := crypto.KeyID(hostPub)

	config := DefaultHeartbeatConfig()
	config.Enabled = false

	hm, err := NewHeartbeatMonitor(tempDir, config, hostPriv, hostPub, hostKeyID)
	if err != nil {
		t.Fatalf("failed to create heartbeat monitor: %v", err)
	}

	_, err = hm.GenerateHeartbeat()
	if err == nil {
		t.Error("should error when disabled")
	}
}
