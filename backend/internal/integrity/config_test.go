package integrity

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerificationConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *VerificationConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &VerificationConfig{
				Enabled:           true,
				Interval:          "1h",
				CheckType:         "quick",
				AlertOnCorruption: true,
			},
			wantErr: false,
		},
		{
			name: "valid full check",
			config: &VerificationConfig{
				Enabled:   true,
				Interval:  "24h",
				CheckType: "full",
			},
			wantErr: false,
		},
		{
			name: "invalid check type",
			config: &VerificationConfig{
				Enabled:   true,
				Interval:  "1h",
				CheckType: "invalid",
			},
			wantErr: true,
		},
		{
			name: "interval too short",
			config: &VerificationConfig{
				Enabled:  true,
				Interval: "30s",
			},
			wantErr: true,
		},
		{
			name: "interval too long",
			config: &VerificationConfig{
				Enabled:  true,
				Interval: "1000h",
			},
			wantErr: true,
		},
		{
			name: "invalid interval format",
			config: &VerificationConfig{
				Enabled:  true,
				Interval: "notaduration",
			},
			wantErr: true,
		},
		{
			name: "empty interval uses default",
			config: &VerificationConfig{
				Enabled:  true,
				Interval: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerificationConfig_ParseInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		want     time.Duration
		wantErr  bool
	}{
		{
			name:     "1 hour",
			interval: "1h",
			want:     time.Hour,
			wantErr:  false,
		},
		{
			name:     "24 hours",
			interval: "24h",
			want:     24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "6 hours",
			interval: "6h",
			want:     6 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "empty defaults to 6h",
			interval: "",
			want:     6 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "30 minutes",
			interval: "30m",
			want:     30 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "invalid",
			interval: "invalid",
			want:     0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &VerificationConfig{Interval: tt.interval}
			got, err := cfg.ParseInterval()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseInterval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultVerificationConfig(t *testing.T) {
	cfg := DefaultVerificationConfig()

	if cfg.Enabled {
		t.Error("expected default config to be disabled")
	}
	if cfg.Interval != "6h" {
		t.Errorf("expected default interval '6h', got '%s'", cfg.Interval)
	}
	if cfg.CheckType != "quick" {
		t.Errorf("expected default checkType 'quick', got '%s'", cfg.CheckType)
	}
	if !cfg.AlertOnCorruption {
		t.Error("expected alertOnCorruption to be true by default")
	}
}

func TestConfigManager_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config manager
	cm, err := NewConfigManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create config manager: %v", err)
	}

	// Update config
	newCfg := &VerificationConfig{
		Enabled:           true,
		Interval:          "2h",
		CheckType:         "full",
		RepoName:          "myrepo",
		AlertOnCorruption: true,
		AlertWebhook:      "https://example.com/webhook",
	}

	if err := cm.Update(newCfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Create new config manager to load from disk
	cm2, err := NewConfigManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create second config manager: %v", err)
	}

	loadedCfg := cm2.Get()

	if !loadedCfg.Enabled {
		t.Error("expected loaded config to be enabled")
	}
	if loadedCfg.Interval != "2h" {
		t.Errorf("expected interval '2h', got '%s'", loadedCfg.Interval)
	}
	if loadedCfg.CheckType != "full" {
		t.Errorf("expected checkType 'full', got '%s'", loadedCfg.CheckType)
	}
	if loadedCfg.RepoName != "myrepo" {
		t.Errorf("expected repoName 'myrepo', got '%s'", loadedCfg.RepoName)
	}
	if loadedCfg.AlertWebhook != "https://example.com/webhook" {
		t.Errorf("unexpected webhook: %s", loadedCfg.AlertWebhook)
	}
}

func TestConfigManager_RecordCheck(t *testing.T) {
	tmpDir := t.TempDir()

	cm, err := NewConfigManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create config manager: %v", err)
	}

	// Record a successful check
	result := &CheckResult{
		Timestamp:    time.Now(),
		RepoPath:     "/test/repo",
		TotalFiles:   100,
		CheckedFiles: 100,
		CorruptFiles: 0,
		MissingFiles: 0,
		Duration:     "1m30s",
		Passed:       true,
	}

	if err := cm.RecordCheck(result); err != nil {
		t.Fatalf("failed to record check: %v", err)
	}

	cfg := cm.Get()
	if cfg.LastCheck == nil {
		t.Fatal("expected lastCheck to be set")
	}
	if cfg.LastResult == nil {
		t.Fatal("expected lastResult to be set")
	}
	if cfg.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures, got %d", cfg.ConsecutiveFailures)
	}

	// Record a failed check
	failedResult := &CheckResult{
		Timestamp:    time.Now(),
		Passed:       false,
		CorruptFiles: 5,
	}

	if err := cm.RecordCheck(failedResult); err != nil {
		t.Fatalf("failed to record check: %v", err)
	}

	cfg = cm.Get()
	if cfg.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", cfg.ConsecutiveFailures)
	}

	// Another failed check
	if err := cm.RecordCheck(failedResult); err != nil {
		t.Fatalf("failed to record check: %v", err)
	}

	cfg = cm.Get()
	if cfg.ConsecutiveFailures != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", cfg.ConsecutiveFailures)
	}

	// Successful check resets counter
	if err := cm.RecordCheck(result); err != nil {
		t.Fatalf("failed to record check: %v", err)
	}

	cfg = cm.Get()
	if cfg.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures after success, got %d", cfg.ConsecutiveFailures)
	}
}

func TestManagedScheduledChecker_ConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	// Create managed checker
	msc, err := NewManagedScheduledChecker(tmpDir)
	if err != nil {
		t.Fatalf("failed to create managed checker: %v", err)
	}

	// Update config
	newCfg := &VerificationConfig{
		Enabled:           true,
		Interval:          "4h",
		CheckType:         "quick",
		RepoName:          "testrepo",
		AlertOnCorruption: true,
	}

	if err := msc.UpdateConfig(newCfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Verify config file exists
	configPath := filepath.Join(tmpDir, ".airgapper-verification-config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("expected config file to exist")
	}

	// Create new managed checker (simulates restart)
	msc2, err := NewManagedScheduledChecker(tmpDir)
	if err != nil {
		t.Fatalf("failed to create second managed checker: %v", err)
	}

	loadedCfg := msc2.GetConfig()
	if !loadedCfg.Enabled {
		t.Error("expected loaded config to be enabled")
	}
	if loadedCfg.Interval != "4h" {
		t.Errorf("expected interval '4h', got '%s'", loadedCfg.Interval)
	}
}

func TestManagedScheduledChecker_RunManualCheck(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	msc, err := NewManagedScheduledChecker(tmpDir)
	if err != nil {
		t.Fatalf("failed to create managed checker: %v", err)
	}

	// Configure with repo name
	cfg := &VerificationConfig{
		Enabled:   false, // Don't start scheduler
		Interval:  "1h",
		CheckType: "full",
		RepoName:  "testrepo",
	}
	if err := msc.UpdateConfig(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Run manual check
	result, err := msc.RunManualCheck("full")
	if err != nil {
		t.Fatalf("manual check failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("expected check to pass, got errors: %v", result.Errors)
	}

	// Verify result was recorded
	loadedCfg := msc.GetConfig()
	if loadedCfg.LastResult == nil {
		t.Error("expected lastResult to be recorded")
	}
	if loadedCfg.LastResult.TotalFiles != 5 {
		t.Errorf("expected 5 total files, got %d", loadedCfg.LastResult.TotalFiles)
	}
}

func TestManagedScheduledChecker_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	msc, err := NewManagedScheduledChecker(tmpDir)
	if err != nil {
		t.Fatalf("failed to create managed checker: %v", err)
	}

	// Configure with valid interval (minimum 1 minute)
	cfg := &VerificationConfig{
		Enabled:           true,
		Interval:          "1m",
		CheckType:         "full",
		RepoName:          "testrepo",
		AlertOnCorruption: true,
	}
	if err := msc.UpdateConfig(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Start - the initial check runs immediately
	if err := msc.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Wait for the initial check to complete
	time.Sleep(100 * time.Millisecond)

	// Stop
	msc.Stop()

	// Verify the initial check was recorded (scheduled checker runs initial check on start)
	history := msc.GetHistory(10)
	if len(history) == 0 {
		t.Error("expected at least one check in history from initial check")
	}
}
