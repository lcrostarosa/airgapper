package integrity

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			if tt.wantErr {
				assert.Error(t, err, "Validate() should return error")
			} else {
				assert.NoError(t, err, "Validate() should not return error")
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
			if tt.wantErr {
				assert.Error(t, err, "ParseInterval() should return error")
				return
			}
			require.NoError(t, err, "ParseInterval() should not return error")
			assert.Equal(t, tt.want, got, "ParseInterval() returned wrong value")
		})
	}
}

func TestDefaultVerificationConfig(t *testing.T) {
	cfg := DefaultVerificationConfig()

	assert.False(t, cfg.Enabled, "expected default config to be disabled")
	assert.Equal(t, "6h", cfg.Interval, "expected default interval '6h'")
	assert.Equal(t, "quick", cfg.CheckType, "expected default checkType 'quick'")
	assert.True(t, cfg.AlertOnCorruption, "expected alertOnCorruption to be true by default")
}

func TestConfigManager_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config manager
	cm, err := NewConfigManager(tmpDir)
	require.NoError(t, err, "failed to create config manager")

	// Update config
	newCfg := &VerificationConfig{
		Enabled:           true,
		Interval:          "2h",
		CheckType:         "full",
		RepoName:          "myrepo",
		AlertOnCorruption: true,
		AlertWebhook:      "https://example.com/webhook",
	}

	err = cm.Update(newCfg)
	require.NoError(t, err, "failed to update config")

	// Create new config manager to load from disk
	cm2, err := NewConfigManager(tmpDir)
	require.NoError(t, err, "failed to create second config manager")

	loadedCfg := cm2.Get()

	assert.True(t, loadedCfg.Enabled, "expected loaded config to be enabled")
	assert.Equal(t, "2h", loadedCfg.Interval, "expected interval '2h'")
	assert.Equal(t, "full", loadedCfg.CheckType, "expected checkType 'full'")
	assert.Equal(t, "myrepo", loadedCfg.RepoName, "expected repoName 'myrepo'")
	assert.Equal(t, "https://example.com/webhook", loadedCfg.AlertWebhook, "unexpected webhook")
}

func TestConfigManager_RecordCheck(t *testing.T) {
	tmpDir := t.TempDir()

	cm, err := NewConfigManager(tmpDir)
	require.NoError(t, err, "failed to create config manager")

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

	err = cm.RecordCheck(result)
	require.NoError(t, err, "failed to record check")

	cfg := cm.Get()
	require.NotNil(t, cfg.LastCheck, "expected lastCheck to be set")
	require.NotNil(t, cfg.LastResult, "expected lastResult to be set")
	assert.Equal(t, 0, cfg.ConsecutiveFailures, "expected 0 consecutive failures")

	// Record a failed check
	failedResult := &CheckResult{
		Timestamp:    time.Now(),
		Passed:       false,
		CorruptFiles: 5,
	}

	err = cm.RecordCheck(failedResult)
	require.NoError(t, err, "failed to record check")

	cfg = cm.Get()
	assert.Equal(t, 1, cfg.ConsecutiveFailures, "expected 1 consecutive failure")

	// Another failed check
	err = cm.RecordCheck(failedResult)
	require.NoError(t, err, "failed to record check")

	cfg = cm.Get()
	assert.Equal(t, 2, cfg.ConsecutiveFailures, "expected 2 consecutive failures")

	// Successful check resets counter
	err = cm.RecordCheck(result)
	require.NoError(t, err, "failed to record check")

	cfg = cm.Get()
	assert.Equal(t, 0, cfg.ConsecutiveFailures, "expected 0 consecutive failures after success")
}

func TestManagedScheduledChecker_ConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	// Create managed checker
	msc, err := NewManagedScheduledChecker(tmpDir)
	require.NoError(t, err, "failed to create managed checker")

	// Update config
	newCfg := &VerificationConfig{
		Enabled:           true,
		Interval:          "4h",
		CheckType:         "quick",
		RepoName:          "testrepo",
		AlertOnCorruption: true,
	}

	err = msc.UpdateConfig(newCfg)
	require.NoError(t, err, "failed to update config")

	// Verify config file exists
	configPath := filepath.Join(tmpDir, ".airgapper-verification-config.json")
	_, err = os.Stat(configPath)
	assert.False(t, os.IsNotExist(err), "expected config file to exist")

	// Create new managed checker (simulates restart)
	msc2, err := NewManagedScheduledChecker(tmpDir)
	require.NoError(t, err, "failed to create second managed checker")

	loadedCfg := msc2.GetConfig()
	assert.True(t, loadedCfg.Enabled, "expected loaded config to be enabled")
	assert.Equal(t, "4h", loadedCfg.Interval, "expected interval '4h'")
}

func TestManagedScheduledChecker_RunManualCheck(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	msc, err := NewManagedScheduledChecker(tmpDir)
	require.NoError(t, err, "failed to create managed checker")

	// Configure with repo name
	cfg := &VerificationConfig{
		Enabled:   false, // Don't start scheduler
		Interval:  "1h",
		CheckType: "full",
		RepoName:  "testrepo",
	}
	err = msc.UpdateConfig(cfg)
	require.NoError(t, err, "failed to update config")

	// Run manual check
	result, err := msc.RunManualCheck("full")
	require.NoError(t, err, "manual check failed")

	assert.True(t, result.Passed, "expected check to pass, got errors: %v", result.Errors)

	// Verify result was recorded
	loadedCfg := msc.GetConfig()
	require.NotNil(t, loadedCfg.LastResult, "expected lastResult to be recorded")
	assert.Equal(t, 5, loadedCfg.LastResult.TotalFiles, "expected 5 total files")
}

func TestManagedScheduledChecker_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	setupTestRepo(t, tmpDir, "testrepo")

	msc, err := NewManagedScheduledChecker(tmpDir)
	require.NoError(t, err, "failed to create managed checker")

	// Configure with valid interval (minimum 1 minute)
	cfg := &VerificationConfig{
		Enabled:           true,
		Interval:          "1m",
		CheckType:         "full",
		RepoName:          "testrepo",
		AlertOnCorruption: true,
	}
	err = msc.UpdateConfig(cfg)
	require.NoError(t, err, "failed to update config")

	// Start - the initial check runs immediately
	err = msc.Start()
	require.NoError(t, err, "failed to start")

	// Wait for the initial check to complete
	time.Sleep(100 * time.Millisecond)

	// Stop
	msc.Stop()

	// Verify the initial check was recorded (scheduled checker runs initial check on start)
	history := msc.GetHistory(10)
	assert.NotEmpty(t, history, "expected at least one check in history from initial check")
}
