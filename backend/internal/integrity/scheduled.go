package integrity

import (
	"sync"
	"time"
)

// ScheduledChecker runs periodic integrity checks
type ScheduledChecker struct {
	checker  *Checker
	repoName string
	interval time.Duration
	stopChan chan struct{}
	running  bool
	mu       sync.Mutex

	// Callback for alerts
	onCorruption func(result *CheckResult)
}

// NewScheduledChecker creates a scheduled checker
func NewScheduledChecker(checker *Checker, repoName string, interval time.Duration) *ScheduledChecker {
	return &ScheduledChecker{
		checker:  checker,
		repoName: repoName,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// SetCorruptionCallback sets a callback to be called when corruption is detected
func (sc *ScheduledChecker) SetCorruptionCallback(cb func(result *CheckResult)) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.onCorruption = cb
}

// Start begins scheduled checking
func (sc *ScheduledChecker) Start() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.running {
		return
	}
	sc.running = true

	go sc.run()
}

// Stop stops scheduled checking
func (sc *ScheduledChecker) Stop() {
	sc.mu.Lock()
	if !sc.running {
		sc.mu.Unlock()
		return
	}
	sc.running = false
	sc.mu.Unlock()

	close(sc.stopChan)
}

func (sc *ScheduledChecker) run() {
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	// Run initial check
	sc.runCheck()

	for {
		select {
		case <-ticker.C:
			sc.runCheck()
		case <-sc.stopChan:
			return
		}
	}
}

func (sc *ScheduledChecker) runCheck() {
	result, err := sc.checker.CheckDataIntegrity(sc.repoName)
	if err != nil {
		return
	}

	if !result.Passed && sc.onCorruption != nil {
		sc.onCorruption(result)
	}
}
