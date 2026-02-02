package verification

import (
	"os"
	"testing"
	"time"
)

func TestAnomalyDetector_MassDeletion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anomaly-mass-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultAnomalyConfig()
	config.MassDeletionThresholdCount = 10
	config.MassDeletionThresholdPct = 5.0
	config.EnableTimeBasedDetection = false
	config.EnablePatternLearning = false

	ad, err := NewAnomalyDetector(tempDir, config, nil)
	if err != nil {
		t.Fatalf("failed to create anomaly detector: %v", err)
	}

	// Delete many files to trigger mass deletion alert
	for i := 0; i < 15; i++ {
		anomalies := ad.AnalyzeDeletion("/path/file"+string(rune(i)), 1024, "test", 1000, 1024*1000)
		if i >= config.MassDeletionThresholdCount-1 {
			// Should detect anomaly after threshold
			found := false
			for _, a := range anomalies {
				if a.Type == AnomalyMassDeletion {
					found = true
					break
				}
			}
			if !found && i >= config.MassDeletionThresholdCount {
				t.Errorf("expected mass deletion anomaly at deletion %d", i)
			}
		}
	}

	// Check anomalies were recorded
	allAnomalies := ad.GetAnomalies(false, 0)
	if len(allAnomalies) == 0 {
		t.Error("expected anomalies to be recorded")
	}

	hasMassAnomaly := false
	for _, a := range allAnomalies {
		if a.Type == AnomalyMassDeletion {
			hasMassAnomaly = true
			break
		}
	}
	if !hasMassAnomaly {
		t.Error("expected at least one mass deletion anomaly")
	}
}

func TestAnomalyDetector_RapidDeletion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anomaly-rapid-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultAnomalyConfig()
	config.RapidDeletionWindowMinutes = 5
	config.RapidDeletionThreshold = 5
	config.MassDeletionThresholdCount = 100 // High to avoid triggering
	config.EnableTimeBasedDetection = false
	config.EnablePatternLearning = false

	ad, err := NewAnomalyDetector(tempDir, config, nil)
	if err != nil {
		t.Fatalf("failed to create anomaly detector: %v", err)
	}

	// Delete rapidly to trigger alert
	for i := 0; i < 10; i++ {
		anomalies := ad.AnalyzeDeletion("/path/file"+string(rune(i)), 1024, "test", 1000, 1024*1000)
		if i >= config.RapidDeletionThreshold-1 {
			hasRapid := false
			for _, a := range anomalies {
				if a.Type == AnomalyRapidDeletion {
					hasRapid = true
					break
				}
			}
			if !hasRapid && i >= config.RapidDeletionThreshold {
				t.Logf("No rapid deletion anomaly at iteration %d", i)
			}
		}
	}

	allAnomalies := ad.GetAnomalies(false, 0)
	hasRapidAnomaly := false
	for _, a := range allAnomalies {
		if a.Type == AnomalyRapidDeletion {
			hasRapidAnomaly = true
			break
		}
	}
	if !hasRapidAnomaly {
		t.Error("expected at least one rapid deletion anomaly")
	}
}

func TestAnomalyDetector_UnusualTime(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anomaly-time-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultAnomalyConfig()
	config.EnableTimeBasedDetection = true
	config.NormalOperatingHoursStart = 9  // 9 AM
	config.NormalOperatingHoursEnd = 17   // 5 PM
	config.AllowedDaysOfWeek = []int{1, 2, 3, 4, 5} // Weekdays
	config.MassDeletionThresholdCount = 100
	config.RapidDeletionThreshold = 100
	config.EnablePatternLearning = false

	ad, err := NewAnomalyDetector(tempDir, config, nil)
	if err != nil {
		t.Fatalf("failed to create anomaly detector: %v", err)
	}

	// The test will detect unusual time based on current time
	// This may or may not trigger depending on when test runs
	anomalies := ad.AnalyzeDeletion("/path/file", 1024, "test", 1000, 1024*1000)

	now := time.Now()
	hour := now.Hour()
	dayOfWeek := int(now.Weekday())

	outsideHours := hour < config.NormalOperatingHoursStart || hour >= config.NormalOperatingHoursEnd
	outsideDays := true
	for _, d := range config.AllowedDaysOfWeek {
		if d == dayOfWeek {
			outsideDays = false
			break
		}
	}

	if outsideHours || outsideDays {
		hasTimeAnomaly := false
		for _, a := range anomalies {
			if a.Type == AnomalyUnusualTime {
				hasTimeAnomaly = true
				break
			}
		}
		if !hasTimeAnomaly {
			t.Log("Expected unusual time anomaly since test is running outside normal hours/days")
		}
	}
}

func TestAnomalyDetector_ResolveAnomaly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anomaly-resolve-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultAnomalyConfig()
	config.MassDeletionThresholdCount = 5
	config.EnableTimeBasedDetection = false
	config.EnablePatternLearning = false

	ad, err := NewAnomalyDetector(tempDir, config, nil)
	if err != nil {
		t.Fatalf("failed to create anomaly detector: %v", err)
	}

	// Create anomaly
	for i := 0; i < 10; i++ {
		ad.AnalyzeDeletion("/path/file"+string(rune(i)), 1024, "test", 1000, 1024*1000)
	}

	// Get unresolved anomalies
	unresolved := ad.GetAnomalies(true, 0)
	if len(unresolved) == 0 {
		t.Fatal("expected unresolved anomalies")
	}

	anomalyID := unresolved[0].ID

	// Resolve it
	err = ad.ResolveAnomaly(anomalyID, "admin", "investigated and determined to be legitimate")
	if err != nil {
		t.Fatalf("failed to resolve anomaly: %v", err)
	}

	// Should not appear in unresolved
	unresolved = ad.GetAnomalies(true, 0)
	for _, a := range unresolved {
		if a.ID == anomalyID {
			t.Error("resolved anomaly should not appear in unresolved list")
		}
	}

	// Should appear in all anomalies
	all := ad.GetAnomalies(false, 0)
	found := false
	for _, a := range all {
		if a.ID == anomalyID {
			found = true
			if !a.Resolved {
				t.Error("anomaly should be marked resolved")
			}
			if a.ResolvedBy != "admin" {
				t.Errorf("expected ResolvedBy='admin', got '%s'", a.ResolvedBy)
			}
			break
		}
	}
	if !found {
		t.Error("resolved anomaly should still appear in all anomalies")
	}
}

func TestAnomalyDetector_Disabled(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anomaly-disabled-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultAnomalyConfig()
	config.Enabled = false

	ad, err := NewAnomalyDetector(tempDir, config, nil)
	if err != nil {
		t.Fatalf("failed to create anomaly detector: %v", err)
	}

	// Should not detect anything when disabled
	for i := 0; i < 100; i++ {
		anomalies := ad.AnalyzeDeletion("/path/file"+string(rune(i)), 1024, "test", 100, 1024*100)
		if len(anomalies) > 0 {
			t.Error("should not detect anomalies when disabled")
		}
	}
}

func TestAnomalyDetector_RecordActivityAndBaseline(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anomaly-baseline-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := DefaultAnomalyConfig()
	config.PatternLearningDays = 30

	ad, err := NewAnomalyDetector(tempDir, config, nil)
	if err != nil {
		t.Fatalf("failed to create anomaly detector: %v", err)
	}

	// Record some activity (less than 7 to trigger insufficient data error)
	for i := 0; i < 5; i++ {
		ad.RecordActivity("deletion", i%3+1, int64(i*1024))
	}

	// Try to learn baseline (will fail due to insufficient data - needs at least 7 records)
	err = ad.LearnBaseline()
	if err == nil {
		t.Error("expected error due to insufficient data (need at least 7 records)")
	}

	// Now add more records so baseline learning can succeed
	for i := 0; i < 5; i++ {
		ad.RecordActivity("deletion", i%3+1, int64(i*1024))
	}

	// Now should succeed
	err = ad.LearnBaseline()
	if err != nil {
		t.Errorf("expected baseline learning to succeed with 10 records: %v", err)
	}
}

func TestStatisticalHelpers(t *testing.T) {
	// Test mean
	values := []float64{1, 2, 3, 4, 5}
	m := mean(values)
	if m != 3.0 {
		t.Errorf("expected mean=3.0, got %f", m)
	}

	// Test empty mean
	emptyMean := mean([]float64{})
	if emptyMean != 0 {
		t.Errorf("expected empty mean=0, got %f", emptyMean)
	}

	// Test stdDev
	sd := stdDev(values)
	// Expected stdDev for [1,2,3,4,5] with sample formula is ~1.58
	if sd < 1.5 || sd > 1.7 {
		t.Errorf("expected stdDev ~1.58, got %f", sd)
	}

	// Test single value stdDev (should be 0)
	singleSD := stdDev([]float64{5})
	if singleSD != 0 {
		t.Errorf("expected single value stdDev=0, got %f", singleSD)
	}
}
