package verification

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CanaryType defines the type of canary file.
type CanaryType string

const (
	CanaryTypeHidden    CanaryType = "hidden"    // Hidden files that shouldn't be accessed
	CanaryTypeDecoy     CanaryType = "decoy"     // Decoy files that look valuable
	CanaryTypeTripwire  CanaryType = "tripwire"  // Files that trigger on any access
	CanaryTypeHoneypot  CanaryType = "honeypot"  // Directories with fake sensitive data
)

// CanaryStatus represents the state of a canary.
type CanaryStatus string

const (
	CanaryStatusActive    CanaryStatus = "active"    // Canary is deployed and intact
	CanaryStatusTriggered CanaryStatus = "triggered" // Canary was accessed/modified
	CanaryStatusMissing   CanaryStatus = "missing"   // Canary file was deleted
	CanaryStatusCorrupted CanaryStatus = "corrupted" // Canary content was modified
)

// Canary represents a canary file or directory.
type Canary struct {
	ID           string       `json:"id"`
	Type         CanaryType   `json:"type"`
	Path         string       `json:"path"`          // Path where canary is deployed
	ContentHash  string       `json:"content_hash"`  // SHA256 of original content
	Size         int64        `json:"size"`
	CreatedAt    time.Time    `json:"created_at"`
	LastChecked  time.Time    `json:"last_checked"`
	Status       CanaryStatus `json:"status"`
	TriggerCount int          `json:"trigger_count"` // Number of times triggered
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// CanaryAlert is generated when a canary is triggered.
type CanaryAlert struct {
	ID          string       `json:"id"`
	CanaryID    string       `json:"canary_id"`
	CanaryPath  string       `json:"canary_path"`
	AlertType   string       `json:"alert_type"` // "accessed", "modified", "deleted"
	DetectedAt  time.Time    `json:"detected_at"`
	Details     string       `json:"details"`
	Severity    string       `json:"severity"` // "warning", "critical"
	Acknowledged bool        `json:"acknowledged"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string    `json:"acknowledged_by,omitempty"`
}

// CanaryConfig configures the canary system.
type CanaryConfig struct {
	Enabled            bool          `json:"enabled"`
	CheckIntervalSecs  int           `json:"check_interval_secs"`  // How often to check canaries
	AutoDeployCount    int           `json:"auto_deploy_count"`    // Number of canaries to auto-deploy
	AlertOnAccess      bool          `json:"alert_on_access"`      // Alert on file access (requires inotify/fsnotify)
	DeployPaths        []string      `json:"deploy_paths"`         // Directories where canaries can be deployed
	DecoyFileNames     []string      `json:"decoy_file_names"`     // Names for decoy files
	HoneypotDirNames   []string      `json:"honeypot_dir_names"`   // Names for honeypot directories
}

// DefaultCanaryConfig returns sensible defaults.
func DefaultCanaryConfig() *CanaryConfig {
	return &CanaryConfig{
		Enabled:           true,
		CheckIntervalSecs: 300, // 5 minutes
		AutoDeployCount:   5,
		AlertOnAccess:     false, // Requires additional setup
		DeployPaths:       []string{},
		DecoyFileNames: []string{
			".credentials", ".secrets", "passwords.txt", ".ssh_backup",
			"private_key.pem", ".env.backup", "database.sql", ".aws_credentials",
		},
		HoneypotDirNames: []string{
			".backup_keys", ".recovery", ".admin", ".secret",
		},
	}
}

// CanaryManager manages canary files and monitors for triggers.
type CanaryManager struct {
	basePath string
	config   *CanaryConfig

	mu       sync.RWMutex
	canaries map[string]*Canary
	alerts   []*CanaryAlert

	// Callback for alerts
	onAlert func(*CanaryAlert)
}

// NewCanaryManager creates a new canary manager.
func NewCanaryManager(basePath string, config *CanaryConfig) (*CanaryManager, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if config == nil {
		config = DefaultCanaryConfig()
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create canary directory: %w", err)
	}

	cm := &CanaryManager{
		basePath: basePath,
		config:   config,
		canaries: make(map[string]*Canary),
		alerts:   []*CanaryAlert{},
	}

	if err := cm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load canary state: %w", err)
	}

	return cm, nil
}

func (cm *CanaryManager) statePath() string {
	return filepath.Join(cm.basePath, "canaries.json")
}

func (cm *CanaryManager) alertsPath() string {
	return filepath.Join(cm.basePath, "canary-alerts.json")
}

func (cm *CanaryManager) load() error {
	// Load canaries
	data, err := os.ReadFile(cm.statePath())
	if err == nil {
		var canaries []*Canary
		if json.Unmarshal(data, &canaries) == nil {
			for _, c := range canaries {
				cm.canaries[c.ID] = c
			}
		}
	}

	// Load alerts
	alertData, err := os.ReadFile(cm.alertsPath())
	if err == nil {
		json.Unmarshal(alertData, &cm.alerts)
	}

	return nil
}

func (cm *CanaryManager) save() error {
	// Save canaries
	canaries := make([]*Canary, 0, len(cm.canaries))
	for _, c := range cm.canaries {
		canaries = append(canaries, c)
	}

	data, err := json.MarshalIndent(canaries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(cm.statePath(), data, 0600); err != nil {
		return err
	}

	// Save alerts
	alertData, err := json.MarshalIndent(cm.alerts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cm.alertsPath(), alertData, 0600)
}

// SetAlertCallback sets a callback function for canary alerts.
func (cm *CanaryManager) SetAlertCallback(callback func(*CanaryAlert)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onAlert = callback
}

// DeployCanary creates and deploys a new canary file.
func (cm *CanaryManager) DeployCanary(canaryType CanaryType, targetPath string, metadata map[string]string) (*Canary, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Generate canary content based on type
	content, err := cm.generateCanaryContent(canaryType)
	if err != nil {
		return nil, fmt.Errorf("failed to generate canary content: %w", err)
	}

	// Write canary file
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create canary directory: %w", err)
	}

	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write canary file: %w", err)
	}

	// Compute hash
	hash := sha256.Sum256(content)

	canary := &Canary{
		ID:          generateCanaryID(),
		Type:        canaryType,
		Path:        targetPath,
		ContentHash: hex.EncodeToString(hash[:]),
		Size:        int64(len(content)),
		CreatedAt:   time.Now(),
		LastChecked: time.Now(),
		Status:      CanaryStatusActive,
		Metadata:    metadata,
	}

	cm.canaries[canary.ID] = canary

	if err := cm.save(); err != nil {
		return nil, err
	}

	return canary, nil
}

// generateCanaryContent creates appropriate content for each canary type.
func (cm *CanaryManager) generateCanaryContent(canaryType CanaryType) ([]byte, error) {
	switch canaryType {
	case CanaryTypeHidden:
		// Small hidden marker file
		marker := make([]byte, 32)
		rand.Read(marker)
		return marker, nil

	case CanaryTypeDecoy:
		// Fake credentials file
		return []byte(fmt.Sprintf(`# Backup Credentials - DO NOT DELETE
# Generated: %s
AWS_ACCESS_KEY_ID=AKIA%s
AWS_SECRET_ACCESS_KEY=%s
DATABASE_PASSWORD=%s
API_KEY=%s
`, time.Now().Format(time.RFC3339),
			randomHex(16), randomHex(40), randomHex(32), randomHex(32))), nil

	case CanaryTypeTripwire:
		// Binary file that looks like a key
		content := make([]byte, 256)
		rand.Read(content)
		return content, nil

	case CanaryTypeHoneypot:
		// JSON config that looks sensitive
		return []byte(fmt.Sprintf(`{
  "type": "backup_encryption_keys",
  "version": "2.0",
  "created": "%s",
  "keys": {
    "master": "%s",
    "recovery": "%s",
    "admin": "%s"
  },
  "warning": "DO NOT MODIFY - System will detect tampering"
}`, time.Now().Format(time.RFC3339),
			randomHex(64), randomHex(64), randomHex(64))), nil

	default:
		return nil, fmt.Errorf("unknown canary type: %s", canaryType)
	}
}

func generateCanaryID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "cnr-" + hex.EncodeToString(b)
}

func randomHex(length int) string {
	b := make([]byte, length/2)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CheckCanary verifies a single canary's integrity.
func (cm *CanaryManager) CheckCanary(id string) (*Canary, *CanaryAlert, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	canary, exists := cm.canaries[id]
	if !exists {
		return nil, nil, fmt.Errorf("canary %s not found", id)
	}

	return cm.checkCanaryInternal(canary)
}

// checkCanaryInternal checks a canary (caller must hold lock).
func (cm *CanaryManager) checkCanaryInternal(canary *Canary) (*Canary, *CanaryAlert, error) {
	canary.LastChecked = time.Now()

	// Check if file exists
	info, err := os.Stat(canary.Path)
	if os.IsNotExist(err) {
		if canary.Status != CanaryStatusMissing {
			canary.Status = CanaryStatusMissing
			alert := cm.createAlert(canary, "deleted", "Canary file was deleted", "critical")
			return canary, alert, nil
		}
		return canary, nil, nil
	}
	if err != nil {
		return canary, nil, fmt.Errorf("failed to stat canary: %w", err)
	}

	// Check size
	if info.Size() != canary.Size {
		if canary.Status != CanaryStatusCorrupted {
			canary.Status = CanaryStatusCorrupted
			alert := cm.createAlert(canary, "modified",
				fmt.Sprintf("Canary size changed: %d -> %d", canary.Size, info.Size()), "critical")
			return canary, alert, nil
		}
		return canary, nil, nil
	}

	// Check content hash
	content, err := os.ReadFile(canary.Path)
	if err != nil {
		return canary, nil, fmt.Errorf("failed to read canary: %w", err)
	}

	hash := sha256.Sum256(content)
	currentHash := hex.EncodeToString(hash[:])

	if currentHash != canary.ContentHash {
		if canary.Status != CanaryStatusCorrupted {
			canary.Status = CanaryStatusCorrupted
			alert := cm.createAlert(canary, "modified", "Canary content was modified", "critical")
			return canary, alert, nil
		}
		return canary, nil, nil
	}

	// Canary is intact
	if canary.Status != CanaryStatusActive {
		canary.Status = CanaryStatusActive
	}

	return canary, nil, nil
}

// createAlert creates and records a canary alert.
func (cm *CanaryManager) createAlert(canary *Canary, alertType, details, severity string) *CanaryAlert {
	now := time.Now()
	canary.TriggerCount++
	canary.LastTriggeredAt = &now

	alert := &CanaryAlert{
		ID:         fmt.Sprintf("calt-%d", time.Now().UnixNano()),
		CanaryID:   canary.ID,
		CanaryPath: canary.Path,
		AlertType:  alertType,
		DetectedAt: now,
		Details:    details,
		Severity:   severity,
	}

	cm.alerts = append(cm.alerts, alert)
	cm.save()

	// Call alert callback if set
	if cm.onAlert != nil {
		go cm.onAlert(alert)
	}

	return alert
}

// CheckAllCanaries verifies all deployed canaries.
func (cm *CanaryManager) CheckAllCanaries() ([]*CanaryAlert, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var alerts []*CanaryAlert

	for _, canary := range cm.canaries {
		_, alert, err := cm.checkCanaryInternal(canary)
		if err != nil {
			continue // Log error but continue checking others
		}
		if alert != nil {
			alerts = append(alerts, alert)
		}
	}

	if err := cm.save(); err != nil {
		return alerts, err
	}

	return alerts, nil
}

// GetCanary retrieves a canary by ID.
func (cm *CanaryManager) GetCanary(id string) *Canary {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.canaries[id]
}

// ListCanaries returns all canaries.
func (cm *CanaryManager) ListCanaries() []*Canary {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*Canary, 0, len(cm.canaries))
	for _, c := range cm.canaries {
		result = append(result, c)
	}
	return result
}

// GetAlerts returns canary alerts.
func (cm *CanaryManager) GetAlerts(unacknowledgedOnly bool, limit int) []*CanaryAlert {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*CanaryAlert
	for i := len(cm.alerts) - 1; i >= 0 && (limit <= 0 || len(result) < limit); i-- {
		alert := cm.alerts[i]
		if unacknowledgedOnly && alert.Acknowledged {
			continue
		}
		result = append(result, alert)
	}
	return result
}

// AcknowledgeAlert marks an alert as acknowledged.
func (cm *CanaryManager) AcknowledgeAlert(alertID, acknowledgedBy string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, alert := range cm.alerts {
		if alert.ID == alertID {
			now := time.Now()
			alert.Acknowledged = true
			alert.AcknowledgedAt = &now
			alert.AcknowledgedBy = acknowledgedBy
			return cm.save()
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

// RemoveCanary removes a canary and optionally deletes the file.
func (cm *CanaryManager) RemoveCanary(id string, deleteFile bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	canary, exists := cm.canaries[id]
	if !exists {
		return fmt.Errorf("canary %s not found", id)
	}

	if deleteFile {
		os.Remove(canary.Path)
	}

	delete(cm.canaries, id)
	return cm.save()
}

// AutoDeploy automatically deploys canaries to configured paths.
func (cm *CanaryManager) AutoDeploy(storagePath string) ([]*Canary, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.config.Enabled {
		return nil, nil
	}

	var deployed []*Canary
	deployCount := cm.config.AutoDeployCount

	// Deploy hidden canaries in storage subdirectories
	hiddenPaths := []string{
		filepath.Join(storagePath, ".canary"),
		filepath.Join(storagePath, "data", ".check"),
		filepath.Join(storagePath, "snapshots", ".verify"),
	}

	for i, path := range hiddenPaths {
		if i >= deployCount {
			break
		}
		// Unlock for deployment, then relock
		cm.mu.Unlock()
		canary, err := cm.DeployCanary(CanaryTypeHidden, path, map[string]string{"auto": "true"})
		cm.mu.Lock()
		if err == nil {
			deployed = append(deployed, canary)
		}
	}

	// Deploy decoy files with attractive names
	for i, name := range cm.config.DecoyFileNames {
		if len(deployed) >= deployCount {
			break
		}
		path := filepath.Join(storagePath, name)
		cm.mu.Unlock()
		canary, err := cm.DeployCanary(CanaryTypeDecoy, path, map[string]string{"auto": "true", "name": name})
		cm.mu.Lock()
		if err == nil {
			deployed = append(deployed, canary)
		}
		if i >= 2 { // Limit decoy files
			break
		}
	}

	return deployed, nil
}

// GetStats returns canary system statistics.
func (cm *CanaryManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	active := 0
	triggered := 0
	missing := 0
	corrupted := 0

	for _, c := range cm.canaries {
		switch c.Status {
		case CanaryStatusActive:
			active++
		case CanaryStatusTriggered:
			triggered++
		case CanaryStatusMissing:
			missing++
		case CanaryStatusCorrupted:
			corrupted++
		}
	}

	unackedAlerts := 0
	for _, a := range cm.alerts {
		if !a.Acknowledged {
			unackedAlerts++
		}
	}

	return map[string]interface{}{
		"total_canaries":        len(cm.canaries),
		"active":                active,
		"triggered":             triggered,
		"missing":               missing,
		"corrupted":             corrupted,
		"total_alerts":          len(cm.alerts),
		"unacknowledged_alerts": unackedAlerts,
	}
}
