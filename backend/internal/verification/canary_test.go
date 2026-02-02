package verification

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCanaryManager_DeployCanary(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	canaryPath := filepath.Join(tempDir, "test-canary")
	canary, err := cm.DeployCanary(CanaryTypeHidden, canaryPath, nil)
	if err != nil {
		t.Fatalf("failed to deploy canary: %v", err)
	}

	if canary.ID == "" {
		t.Error("canary should have an ID")
	}

	if canary.Status != CanaryStatusActive {
		t.Errorf("expected status active, got %s", canary.Status)
	}

	// Verify file exists
	if _, err := os.Stat(canaryPath); os.IsNotExist(err) {
		t.Error("canary file should exist")
	}
}

func TestCanaryManager_CheckCanary_Modified(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-modified-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	canaryPath := filepath.Join(tempDir, "test-canary")
	canary, err := cm.DeployCanary(CanaryTypeDecoy, canaryPath, nil)
	if err != nil {
		t.Fatalf("failed to deploy canary: %v", err)
	}

	// Modify the canary file
	os.WriteFile(canaryPath, []byte("modified content"), 0644)

	// Check should detect modification
	_, alert, err := cm.CheckCanary(canary.ID)
	if err != nil {
		t.Fatalf("failed to check canary: %v", err)
	}

	if alert == nil {
		t.Fatal("expected alert for modified canary")
	}

	if alert.AlertType != "modified" {
		t.Errorf("expected alert type 'modified', got '%s'", alert.AlertType)
	}

	if alert.Severity != "critical" {
		t.Errorf("expected severity 'critical', got '%s'", alert.Severity)
	}
}

func TestCanaryManager_CheckCanary_Deleted(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-deleted-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	canaryPath := filepath.Join(tempDir, "test-canary")
	canary, err := cm.DeployCanary(CanaryTypeHidden, canaryPath, nil)
	if err != nil {
		t.Fatalf("failed to deploy canary: %v", err)
	}

	// Delete the canary file
	os.Remove(canaryPath)

	// Check should detect deletion
	_, alert, err := cm.CheckCanary(canary.ID)
	if err != nil {
		t.Fatalf("failed to check canary: %v", err)
	}

	if alert == nil {
		t.Fatal("expected alert for deleted canary")
	}

	if alert.AlertType != "deleted" {
		t.Errorf("expected alert type 'deleted', got '%s'", alert.AlertType)
	}
}

func TestCanaryManager_CheckAllCanaries(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-all-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	// Deploy multiple canaries
	for i := 0; i < 3; i++ {
		path := filepath.Join(tempDir, "canary-"+string(rune('a'+i)))
		cm.DeployCanary(CanaryTypeHidden, path, nil)
	}

	// Modify one
	os.WriteFile(filepath.Join(tempDir, "canary-b"), []byte("bad"), 0644)

	// Check all
	alerts, err := cm.CheckAllCanaries()
	if err != nil {
		t.Fatalf("failed to check all canaries: %v", err)
	}

	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}
}

func TestCanaryManager_DecoyContent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-decoy-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	canaryPath := filepath.Join(tempDir, ".credentials")
	_, err = cm.DeployCanary(CanaryTypeDecoy, canaryPath, nil)
	if err != nil {
		t.Fatalf("failed to deploy decoy canary: %v", err)
	}

	// Read content - should look like credentials
	content, err := os.ReadFile(canaryPath)
	if err != nil {
		t.Fatalf("failed to read canary: %v", err)
	}

	contentStr := string(content)
	if len(contentStr) < 50 {
		t.Error("decoy content should be substantial")
	}

	// Should contain credential-like content
	hasKey := false
	keywords := []string{"AWS_ACCESS_KEY", "DATABASE_PASSWORD", "API_KEY"}
	for _, kw := range keywords {
		if contains(contentStr, kw) {
			hasKey = true
			break
		}
	}
	if !hasKey {
		t.Error("decoy should contain credential-like keywords")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCanaryManager_AcknowledgeAlert(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-ack-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	canaryPath := filepath.Join(tempDir, "test-canary")
	canary, _ := cm.DeployCanary(CanaryTypeHidden, canaryPath, nil)

	// Delete to trigger alert
	os.Remove(canaryPath)
	cm.CheckCanary(canary.ID)

	// Get unacknowledged alerts
	alerts := cm.GetAlerts(true, 0)
	if len(alerts) == 0 {
		t.Fatal("expected unacknowledged alerts")
	}

	alertID := alerts[0].ID

	// Acknowledge
	err = cm.AcknowledgeAlert(alertID, "admin")
	if err != nil {
		t.Fatalf("failed to acknowledge alert: %v", err)
	}

	// Should not appear in unacknowledged
	unacked := cm.GetAlerts(true, 0)
	for _, a := range unacked {
		if a.ID == alertID {
			t.Error("acknowledged alert should not appear in unacknowledged list")
		}
	}
}

func TestCanaryManager_GetStats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-stats-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	// Deploy canaries
	cm.DeployCanary(CanaryTypeHidden, filepath.Join(tempDir, "c1"), nil)
	cm.DeployCanary(CanaryTypeDecoy, filepath.Join(tempDir, "c2"), nil)

	stats := cm.GetStats()

	if stats["total_canaries"].(int) != 2 {
		t.Errorf("expected 2 total canaries, got %d", stats["total_canaries"].(int))
	}

	if stats["active"].(int) != 2 {
		t.Errorf("expected 2 active canaries, got %d", stats["active"].(int))
	}
}

func TestCanaryManager_AlertCallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-callback-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	alertReceived := make(chan *CanaryAlert, 1)
	cm.SetAlertCallback(func(alert *CanaryAlert) {
		alertReceived <- alert
	})

	canaryPath := filepath.Join(tempDir, "test-canary")
	canary, _ := cm.DeployCanary(CanaryTypeHidden, canaryPath, nil)

	// Trigger alert
	os.Remove(canaryPath)
	cm.CheckCanary(canary.ID)

	// Wait for callback
	select {
	case alert := <-alertReceived:
		if alert.CanaryID != canary.ID {
			t.Errorf("expected canary ID %s, got %s", canary.ID, alert.CanaryID)
		}
	case <-time.After(time.Second):
		t.Error("alert callback not called")
	}
}

func TestCanaryManager_RemoveCanary(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "canary-remove-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultCanaryConfig()
	cm, err := NewCanaryManager(tempDir, config)
	if err != nil {
		t.Fatalf("failed to create canary manager: %v", err)
	}

	canaryPath := filepath.Join(tempDir, "test-canary")
	canary, _ := cm.DeployCanary(CanaryTypeHidden, canaryPath, nil)

	// Remove with file deletion
	err = cm.RemoveCanary(canary.ID, true)
	if err != nil {
		t.Fatalf("failed to remove canary: %v", err)
	}

	// Should not find canary
	if cm.GetCanary(canary.ID) != nil {
		t.Error("canary should be removed")
	}

	// File should be deleted
	if _, err := os.Stat(canaryPath); !os.IsNotExist(err) {
		t.Error("canary file should be deleted")
	}
}
