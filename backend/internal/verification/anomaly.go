package verification

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AnomalyType categorizes detected anomalies.
type AnomalyType string

const (
	AnomalyMassDeletion      AnomalyType = "mass_deletion"       // Unusually high deletion volume
	AnomalyRapidDeletion     AnomalyType = "rapid_deletion"      // Deletions faster than normal
	AnomalyUnusualTime       AnomalyType = "unusual_time"        // Activity at unusual hours
	AnomalyPatternChange     AnomalyType = "pattern_change"      // Deviation from established patterns
	AnomalySnapshotWipe      AnomalyType = "snapshot_wipe"       // Attempt to delete many snapshots
	AnomalyChainTampering    AnomalyType = "chain_tampering"     // Audit chain anomaly
	AnomalyTicketMisuse      AnomalyType = "ticket_misuse"       // Ticket used beyond scope
	AnomalyUnauthorizedAccess AnomalyType = "unauthorized_access" // Access without proper credentials
)

// AnomalySeverity indicates how serious an anomaly is.
type AnomalySeverity string

const (
	SeverityLow      AnomalySeverity = "low"
	SeverityMedium   AnomalySeverity = "medium"
	SeverityHigh     AnomalySeverity = "high"
	SeverityCritical AnomalySeverity = "critical"
)

// Anomaly represents a detected suspicious activity.
type Anomaly struct {
	ID          string          `json:"id"`
	Type        AnomalyType     `json:"type"`
	Severity    AnomalySeverity `json:"severity"`
	DetectedAt  time.Time       `json:"detected_at"`
	Description string          `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Paths       []string        `json:"paths,omitempty"`
	Resolved    bool            `json:"resolved"`
	ResolvedAt  *time.Time      `json:"resolved_at,omitempty"`
	ResolvedBy  string          `json:"resolved_by,omitempty"`
	Resolution  string          `json:"resolution,omitempty"`
}

// AnomalyConfig configures anomaly detection.
type AnomalyConfig struct {
	Enabled bool `json:"enabled"`

	// Thresholds for mass deletion detection
	MassDeletionThresholdPct   float64 `json:"mass_deletion_threshold_pct"`   // Alert if deleting > X% of data
	MassDeletionThresholdCount int     `json:"mass_deletion_threshold_count"` // Alert if deleting > N files at once

	// Rapid deletion detection
	RapidDeletionWindowMinutes int `json:"rapid_deletion_window_minutes"` // Time window for rapid detection
	RapidDeletionThreshold     int `json:"rapid_deletion_threshold"`      // Max deletions in window

	// Time-based anomalies
	EnableTimeBasedDetection bool     `json:"enable_time_based_detection"`
	NormalOperatingHoursStart int     `json:"normal_operating_hours_start"` // e.g., 6 for 6 AM
	NormalOperatingHoursEnd   int     `json:"normal_operating_hours_end"`   // e.g., 22 for 10 PM
	AllowedDaysOfWeek         []int   `json:"allowed_days_of_week"`         // 0=Sunday, 6=Saturday

	// Pattern learning
	EnablePatternLearning    bool `json:"enable_pattern_learning"`
	PatternLearningDays      int  `json:"pattern_learning_days"` // Days to establish baseline
	PatternDeviationMultiple float64 `json:"pattern_deviation_multiple"` // Alert if > N standard deviations

	// Response actions
	AutoLockOnCritical bool `json:"auto_lock_on_critical"` // Auto-trigger lockout on critical anomalies
	AlertWebhookURL    string `json:"alert_webhook_url,omitempty"` // Optional webhook for alerts
}

// DefaultAnomalyConfig returns sensible defaults.
func DefaultAnomalyConfig() *AnomalyConfig {
	return &AnomalyConfig{
		Enabled:                    true,
		MassDeletionThresholdPct:   10.0,  // Alert if deleting > 10% of data
		MassDeletionThresholdCount: 50,    // Alert if deleting > 50 files at once
		RapidDeletionWindowMinutes: 5,     // 5 minute window
		RapidDeletionThreshold:     20,    // 20 deletions in 5 minutes triggers alert
		EnableTimeBasedDetection:   true,
		NormalOperatingHoursStart:  6,     // 6 AM
		NormalOperatingHoursEnd:    22,    // 10 PM
		AllowedDaysOfWeek:          []int{1, 2, 3, 4, 5}, // Weekdays only
		EnablePatternLearning:      true,
		PatternLearningDays:        30,
		PatternDeviationMultiple:   3.0, // 3 standard deviations
		AutoLockOnCritical:         true,
	}
}

// ActivityRecord tracks activity for pattern analysis.
type ActivityRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"` // "deletion", "write", "read"
	Count      int       `json:"count"`
	Bytes      int64     `json:"bytes"`
	Hour       int       `json:"hour"`
	DayOfWeek  int       `json:"day_of_week"`
}

// AnomalyDetector monitors for suspicious activity patterns.
type AnomalyDetector struct {
	basePath    string
	config      *AnomalyConfig
	rateLimiter *RateLimiter // Optional integration

	mu               sync.RWMutex
	anomalies        []*Anomaly
	activityRecords  []ActivityRecord
	baselineStats    *BaselineStats
	recentDeletions  []DeletionEvent
}

// BaselineStats holds learned baseline patterns.
type BaselineStats struct {
	LearnedAt             time.Time `json:"learned_at"`
	DailyDeletionMean     float64   `json:"daily_deletion_mean"`
	DailyDeletionStdDev   float64   `json:"daily_deletion_std_dev"`
	HourlyDeletionMean    float64   `json:"hourly_deletion_mean"`
	HourlyDeletionStdDev  float64   `json:"hourly_deletion_std_dev"`
	DailyBytesMean        float64   `json:"daily_bytes_mean"`
	DailyBytesStdDev      float64   `json:"daily_bytes_std_dev"`
	PeakHours             []int     `json:"peak_hours"`
	PeakDays              []int     `json:"peak_days"`
}

// DeletionEvent tracks a deletion for anomaly analysis.
type DeletionEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
	Bytes     int64     `json:"bytes"`
	Initiator string    `json:"initiator"`
}

// NewAnomalyDetector creates a new anomaly detector.
func NewAnomalyDetector(basePath string, config *AnomalyConfig, rateLimiter *RateLimiter) (*AnomalyDetector, error) {
	if basePath == "" {
		return nil, errors.New("base path required")
	}

	if config == nil {
		config = DefaultAnomalyConfig()
	}

	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create anomaly directory: %w", err)
	}

	ad := &AnomalyDetector{
		basePath:        basePath,
		config:          config,
		rateLimiter:     rateLimiter,
		anomalies:       []*Anomaly{},
		activityRecords: []ActivityRecord{},
		recentDeletions: []DeletionEvent{},
	}

	if err := ad.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load anomaly state: %w", err)
	}

	return ad, nil
}

func (ad *AnomalyDetector) anomalyPath() string {
	return filepath.Join(ad.basePath, "anomalies.json")
}

func (ad *AnomalyDetector) baselinePath() string {
	return filepath.Join(ad.basePath, "baseline.json")
}

func (ad *AnomalyDetector) load() error {
	// Load anomalies
	data, err := os.ReadFile(ad.anomalyPath())
	if err == nil {
		json.Unmarshal(data, &ad.anomalies)
	}

	// Load baseline
	baselineData, err := os.ReadFile(ad.baselinePath())
	if err == nil {
		var baseline BaselineStats
		if json.Unmarshal(baselineData, &baseline) == nil {
			ad.baselineStats = &baseline
		}
	}

	return nil
}

func (ad *AnomalyDetector) save() error {
	// Save anomalies
	data, err := json.MarshalIndent(ad.anomalies, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(ad.anomalyPath(), data, 0600); err != nil {
		return err
	}

	// Save baseline
	if ad.baselineStats != nil {
		baselineData, err := json.MarshalIndent(ad.baselineStats, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(ad.baselinePath(), baselineData, 0600); err != nil {
			return err
		}
	}

	return nil
}

// AnalyzeDeletion checks a deletion for anomalies.
func (ad *AnomalyDetector) AnalyzeDeletion(path string, bytes int64, initiator string, totalFiles int, totalBytes int64) []*Anomaly {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if !ad.config.Enabled {
		return nil
	}

	now := time.Now()
	event := DeletionEvent{
		Timestamp: now,
		Path:      path,
		Bytes:     bytes,
		Initiator: initiator,
	}
	ad.recentDeletions = append(ad.recentDeletions, event)

	// Clean old events (keep last hour)
	hourAgo := now.Add(-time.Hour)
	var recent []DeletionEvent
	for _, e := range ad.recentDeletions {
		if e.Timestamp.After(hourAgo) {
			recent = append(recent, e)
		}
	}
	ad.recentDeletions = recent

	var detected []*Anomaly

	// Check for mass deletion by percentage
	if totalFiles > 0 && ad.config.MassDeletionThresholdPct > 0 {
		deletionPct := (float64(len(ad.recentDeletions)) / float64(totalFiles)) * 100
		if deletionPct >= ad.config.MassDeletionThresholdPct {
			anomaly := ad.createAnomaly(
				AnomalyMassDeletion,
				SeverityHigh,
				fmt.Sprintf("Mass deletion detected: %.1f%% of files deleted in last hour", deletionPct),
				map[string]interface{}{
					"deletion_percentage": deletionPct,
					"files_deleted":       len(ad.recentDeletions),
					"total_files":         totalFiles,
				},
			)
			detected = append(detected, anomaly)
		}
	}

	// Check for mass deletion by count
	if len(ad.recentDeletions) >= ad.config.MassDeletionThresholdCount {
		anomaly := ad.createAnomaly(
			AnomalyMassDeletion,
			SeverityMedium,
			fmt.Sprintf("High deletion volume: %d files deleted in last hour", len(ad.recentDeletions)),
			map[string]interface{}{
				"files_deleted": len(ad.recentDeletions),
				"threshold":     ad.config.MassDeletionThresholdCount,
			},
		)
		detected = append(detected, anomaly)
	}

	// Check for rapid deletion
	windowStart := now.Add(-time.Duration(ad.config.RapidDeletionWindowMinutes) * time.Minute)
	var recentWindowCount int
	for _, e := range ad.recentDeletions {
		if e.Timestamp.After(windowStart) {
			recentWindowCount++
		}
	}
	if recentWindowCount >= ad.config.RapidDeletionThreshold {
		anomaly := ad.createAnomaly(
			AnomalyRapidDeletion,
			SeverityHigh,
			fmt.Sprintf("Rapid deletion detected: %d deletions in %d minutes",
				recentWindowCount, ad.config.RapidDeletionWindowMinutes),
			map[string]interface{}{
				"deletions_in_window": recentWindowCount,
				"window_minutes":      ad.config.RapidDeletionWindowMinutes,
				"threshold":           ad.config.RapidDeletionThreshold,
			},
		)
		detected = append(detected, anomaly)
	}

	// Check for unusual time
	if ad.config.EnableTimeBasedDetection {
		hour := now.Hour()
		dayOfWeek := int(now.Weekday())

		outsideHours := hour < ad.config.NormalOperatingHoursStart ||
			hour >= ad.config.NormalOperatingHoursEnd

		outsideDays := true
		for _, d := range ad.config.AllowedDaysOfWeek {
			if d == dayOfWeek {
				outsideDays = false
				break
			}
		}

		if outsideHours || outsideDays {
			anomaly := ad.createAnomaly(
				AnomalyUnusualTime,
				SeverityLow,
				fmt.Sprintf("Deletion at unusual time: %s (hour %d, day %d)",
					now.Format(time.RFC3339), hour, dayOfWeek),
				map[string]interface{}{
					"hour":       hour,
					"day_of_week": dayOfWeek,
					"outside_hours": outsideHours,
					"outside_days":  outsideDays,
				},
			)
			detected = append(detected, anomaly)
		}
	}

	// Check against baseline pattern
	if ad.config.EnablePatternLearning && ad.baselineStats != nil {
		hourlyDeletions := float64(len(ad.recentDeletions))
		if ad.baselineStats.HourlyDeletionStdDev > 0 {
			zScore := (hourlyDeletions - ad.baselineStats.HourlyDeletionMean) / ad.baselineStats.HourlyDeletionStdDev
			if math.Abs(zScore) > ad.config.PatternDeviationMultiple {
				severity := SeverityMedium
				if zScore > ad.config.PatternDeviationMultiple*2 {
					severity = SeverityHigh
				}
				anomaly := ad.createAnomaly(
					AnomalyPatternChange,
					severity,
					fmt.Sprintf("Deletion rate deviates from baseline: %.1f standard deviations", zScore),
					map[string]interface{}{
						"z_score":           zScore,
						"hourly_deletions":  hourlyDeletions,
						"baseline_mean":     ad.baselineStats.HourlyDeletionMean,
						"baseline_std_dev":  ad.baselineStats.HourlyDeletionStdDev,
					},
				)
				detected = append(detected, anomaly)
			}
		}
	}

	// Handle critical anomalies
	for _, a := range detected {
		ad.anomalies = append(ad.anomalies, a)
		if a.Severity == SeverityCritical && ad.config.AutoLockOnCritical && ad.rateLimiter != nil {
			ad.rateLimiter.TriggerLockout(
				fmt.Sprintf("Auto-lock due to critical anomaly: %s", a.Description),
				time.Duration(ad.rateLimiter.config.LockoutDurationMinutes)*time.Minute,
			)
		}
	}

	if len(detected) > 0 {
		ad.save()
	}

	return detected
}

func (ad *AnomalyDetector) createAnomaly(aType AnomalyType, severity AnomalySeverity, description string, details map[string]interface{}) *Anomaly {
	return &Anomaly{
		ID:          fmt.Sprintf("anm-%d", time.Now().UnixNano()),
		Type:        aType,
		Severity:    severity,
		DetectedAt:  time.Now(),
		Description: description,
		Details:     details,
	}
}

// RecordActivity records activity for baseline learning.
func (ad *AnomalyDetector) RecordActivity(actType string, count int, bytes int64) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	now := time.Now()
	record := ActivityRecord{
		Timestamp: now,
		Type:      actType,
		Count:     count,
		Bytes:     bytes,
		Hour:      now.Hour(),
		DayOfWeek: int(now.Weekday()),
	}

	ad.activityRecords = append(ad.activityRecords, record)

	// Keep last 30 days
	cutoff := now.Add(-time.Duration(ad.config.PatternLearningDays) * 24 * time.Hour)
	var recent []ActivityRecord
	for _, r := range ad.activityRecords {
		if r.Timestamp.After(cutoff) {
			recent = append(recent, r)
		}
	}
	ad.activityRecords = recent
}

// LearnBaseline analyzes historical data to establish baseline patterns.
func (ad *AnomalyDetector) LearnBaseline() error {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if len(ad.activityRecords) < 7 { // Need at least a week of data
		return errors.New("insufficient data for baseline learning")
	}

	// Calculate daily and hourly statistics
	dailyDeletions := make(map[string]int)
	hourlyDeletions := make(map[int][]int)
	dailyBytes := make(map[string]int64)

	for _, r := range ad.activityRecords {
		if r.Type == "deletion" {
			day := r.Timestamp.Format("2006-01-02")
			dailyDeletions[day] += r.Count
			dailyBytes[day] += r.Bytes
			hourlyDeletions[r.Hour] = append(hourlyDeletions[r.Hour], r.Count)
		}
	}

	// Calculate means and standard deviations
	ad.baselineStats = &BaselineStats{
		LearnedAt: time.Now(),
	}

	// Daily deletion stats
	var dailyValues []float64
	for _, v := range dailyDeletions {
		dailyValues = append(dailyValues, float64(v))
	}
	if len(dailyValues) > 0 {
		ad.baselineStats.DailyDeletionMean = mean(dailyValues)
		ad.baselineStats.DailyDeletionStdDev = stdDev(dailyValues)
	}

	// Hourly deletion stats
	var allHourlyValues []float64
	for _, values := range hourlyDeletions {
		for _, v := range values {
			allHourlyValues = append(allHourlyValues, float64(v))
		}
	}
	if len(allHourlyValues) > 0 {
		ad.baselineStats.HourlyDeletionMean = mean(allHourlyValues)
		ad.baselineStats.HourlyDeletionStdDev = stdDev(allHourlyValues)
	}

	// Daily bytes stats
	var bytesValues []float64
	for _, v := range dailyBytes {
		bytesValues = append(bytesValues, float64(v))
	}
	if len(bytesValues) > 0 {
		ad.baselineStats.DailyBytesMean = mean(bytesValues)
		ad.baselineStats.DailyBytesStdDev = stdDev(bytesValues)
	}

	return ad.save()
}

// GetAnomalies returns detected anomalies, optionally filtered.
func (ad *AnomalyDetector) GetAnomalies(unresolvedOnly bool, limit int) []*Anomaly {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	var result []*Anomaly
	for i := len(ad.anomalies) - 1; i >= 0 && (limit <= 0 || len(result) < limit); i-- {
		a := ad.anomalies[i]
		if unresolvedOnly && a.Resolved {
			continue
		}
		result = append(result, a)
	}
	return result
}

// ResolveAnomaly marks an anomaly as resolved.
func (ad *AnomalyDetector) ResolveAnomaly(id, resolvedBy, resolution string) error {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	for _, a := range ad.anomalies {
		if a.ID == id {
			now := time.Now()
			a.Resolved = true
			a.ResolvedAt = &now
			a.ResolvedBy = resolvedBy
			a.Resolution = resolution
			return ad.save()
		}
	}
	return fmt.Errorf("anomaly %s not found", id)
}

// Helper functions for statistics

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	var sumSquares float64
	for _, v := range values {
		diff := v - m
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)-1))
}
