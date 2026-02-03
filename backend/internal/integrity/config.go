// Package integrity provides scheduled verification configuration
package integrity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// VerificationConfig holds settings for scheduled integrity verification
type VerificationConfig struct {
	// Enabled controls whether scheduled verification is active
	Enabled bool `json:"enabled"`

	// Interval between verification checks (e.g., "1h", "24h", "168h" for weekly)
	Interval string `json:"interval"`

	// CheckType: "quick" (file count/merkle only) or "full" (content hash verification)
	CheckType string `json:"checkType"`

	// RepoName is the repository to verify
	RepoName string `json:"repoName,omitempty"`

	// SnapshotID for quick checks (uses latest if empty)
	SnapshotID string `json:"snapshotId,omitempty"`

	// AlertOnCorruption enables corruption alerts
	AlertOnCorruption bool `json:"alertOnCorruption"`

	// AlertWebhook is an optional URL to POST alerts to
	AlertWebhook string `json:"alertWebhook,omitempty"`

	// LastCheck records when verification last ran
	LastCheck *time.Time `json:"lastCheck,omitempty"`

	// LastResult stores the outcome of the last check
	LastResult *CheckResult `json:"lastResult,omitempty"`

	// ConsecutiveFailures tracks failures for escalating alerts
	ConsecutiveFailures int `json:"consecutiveFailures"`
}

// DefaultVerificationConfig returns sensible defaults
func DefaultVerificationConfig() *VerificationConfig {
	return &VerificationConfig{
		Enabled:           false,
		Interval:          "6h", // Every 6 hours by default
		CheckType:         "quick",
		AlertOnCorruption: true,
	}
}

// ParseInterval parses the interval string into a time.Duration
func (c *VerificationConfig) ParseInterval() (time.Duration, error) {
	if c.Interval == "" {
		return 6 * time.Hour, nil
	}
	return time.ParseDuration(c.Interval)
}

// Validate checks that the configuration is valid
func (c *VerificationConfig) Validate() error {
	if c.CheckType != "" && c.CheckType != "quick" && c.CheckType != "full" {
		return fmt.Errorf("invalid checkType: must be 'quick' or 'full'")
	}

	if c.Interval != "" {
		d, err := time.ParseDuration(c.Interval)
		if err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}
		if d < time.Minute {
			return fmt.Errorf("interval must be at least 1 minute")
		}
		if d > 30*24*time.Hour {
			return fmt.Errorf("interval must not exceed 30 days")
		}
	}

	return nil
}

// ConfigManager handles loading/saving verification configuration
type ConfigManager struct {
	basePath   string
	configPath string
	config     *VerificationConfig
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(basePath string) (*ConfigManager, error) {
	cm := &ConfigManager{
		basePath:   basePath,
		configPath: filepath.Join(basePath, ".airgapper-verification-config.json"),
	}

	if err := cm.load(); err != nil {
		// Create default config if none exists
		cm.config = DefaultVerificationConfig()
	}

	return cm, nil
}

// load reads configuration from disk
func (cm *ConfigManager) load() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}

	cm.config = &VerificationConfig{}
	return json.Unmarshal(data, cm.config)
}

// Save writes configuration to disk
func (cm *ConfigManager) Save() error {
	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cm.configPath, data, 0600)
}

// Get returns the current configuration
func (cm *ConfigManager) Get() *VerificationConfig {
	return cm.config
}

// Update updates the configuration with new values
func (cm *ConfigManager) Update(cfg *VerificationConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Preserve last check info if not provided
	if cfg.LastCheck == nil && cm.config.LastCheck != nil {
		cfg.LastCheck = cm.config.LastCheck
	}
	if cfg.LastResult == nil && cm.config.LastResult != nil {
		cfg.LastResult = cm.config.LastResult
	}

	cm.config = cfg
	return cm.Save()
}

// RecordCheck records the result of a verification check
func (cm *ConfigManager) RecordCheck(result *CheckResult) error {
	now := time.Now()
	cm.config.LastCheck = &now
	cm.config.LastResult = result

	if result.Passed {
		cm.config.ConsecutiveFailures = 0
	} else {
		cm.config.ConsecutiveFailures++
	}

	return cm.Save()
}

// ManagedScheduledChecker wraps ScheduledChecker with configuration management
type ManagedScheduledChecker struct {
	checker       *Checker
	configManager *ConfigManager
	scheduler     *ScheduledChecker
}

// NewManagedScheduledChecker creates a managed scheduled checker
func NewManagedScheduledChecker(basePath string) (*ManagedScheduledChecker, error) {
	checker, err := NewChecker(basePath)
	if err != nil {
		return nil, err
	}

	cm, err := NewConfigManager(basePath)
	if err != nil {
		return nil, err
	}

	msc := &ManagedScheduledChecker{
		checker:       checker,
		configManager: cm,
	}

	return msc, nil
}

// GetConfig returns the current verification configuration
func (msc *ManagedScheduledChecker) GetConfig() *VerificationConfig {
	return msc.configManager.Get()
}

// UpdateConfig updates the verification configuration
func (msc *ManagedScheduledChecker) UpdateConfig(cfg *VerificationConfig) error {
	if err := msc.configManager.Update(cfg); err != nil {
		return err
	}

	// Restart scheduler with new config
	msc.restartScheduler()
	return nil
}

// Start starts the scheduled checker based on current config
func (msc *ManagedScheduledChecker) Start() error {
	config := msc.configManager.Get()
	if !config.Enabled {
		return nil
	}

	return msc.startScheduler()
}

// Stop stops the scheduled checker
func (msc *ManagedScheduledChecker) Stop() {
	if msc.scheduler != nil {
		msc.scheduler.Stop()
		msc.scheduler = nil
	}
}

func (msc *ManagedScheduledChecker) startScheduler() error {
	config := msc.configManager.Get()

	interval, err := config.ParseInterval()
	if err != nil {
		return err
	}

	msc.scheduler = NewScheduledChecker(msc.checker, config.RepoName, interval)

	// Set up the callback to record results and trigger alerts
	msc.scheduler.SetCorruptionCallback(func(result *CheckResult) {
		_ = msc.configManager.RecordCheck(result)

		if config.AlertOnCorruption {
			msc.sendAlert(result)
		}
	})

	// Also record successful checks
	origCallback := msc.scheduler.onCorruption
	msc.scheduler.SetCorruptionCallback(func(result *CheckResult) {
		_ = msc.configManager.RecordCheck(result)
		if !result.Passed && origCallback != nil {
			origCallback(result)
		}
	})

	msc.scheduler.Start()
	return nil
}

func (msc *ManagedScheduledChecker) restartScheduler() {
	msc.Stop()
	_ = msc.Start()
}

func (msc *ManagedScheduledChecker) sendAlert(result *CheckResult) {
	config := msc.configManager.Get()

	// Log the alert locally
	logging.Error("Integrity alert: corruption detected",
		logging.String("repoPath", result.RepoPath),
		logging.Int("corruptFiles", result.CorruptFiles),
		logging.Int("missingFiles", result.MissingFiles))

	// If webhook is configured, try to POST to it
	if config.AlertWebhook != "" {
		// In a real implementation, you'd make an HTTP POST here
		// For now, just log
		logging.Warn("Would POST to webhook", logging.String("webhook", config.AlertWebhook))
	}
}

// RunManualCheck performs a manual integrity check
func (msc *ManagedScheduledChecker) RunManualCheck(checkType string) (*CheckResult, error) {
	config := msc.configManager.Get()
	repoName := config.RepoName

	var result *CheckResult
	var err error

	switch checkType {
	case "full":
		result, err = msc.checker.CheckDataIntegrity(repoName)
	case "quick":
		result, err = msc.checker.QuickCheck(repoName, config.SnapshotID)
	default:
		result, err = msc.checker.CheckDataIntegrity(repoName)
	}

	if err != nil {
		return nil, err
	}

	_ = msc.configManager.RecordCheck(result)
	return result, nil
}

// GetChecker returns the underlying Checker for direct access
func (msc *ManagedScheduledChecker) GetChecker() *Checker {
	return msc.checker
}

// GetHistory returns recent check results
func (msc *ManagedScheduledChecker) GetHistory(limit int) []CheckResult {
	return msc.checker.GetHistory(limit)
}
