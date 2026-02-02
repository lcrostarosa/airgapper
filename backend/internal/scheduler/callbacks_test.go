package scheduler

import (
	"sync"
	"testing"
	"time"
)

func TestSchedulerCallbacks_NilSafe(t *testing.T) {
	var c *SchedulerCallbacks

	// None of these should panic
	c.callOnBackupStart(nil)
	c.callOnBackupSuccess(nil)
	c.callOnBackupFailure(nil)
	c.callOnRetryExhausted(nil)
	c.callOnScheduleChange(nil, nil)
}

func TestSchedulerCallbacks_EmptyCallbacks(t *testing.T) {
	c := &SchedulerCallbacks{}

	// None of these should panic
	c.callOnBackupStart(&BackupResult{})
	c.callOnBackupSuccess(&BackupResult{})
	c.callOnBackupFailure(&BackupResult{})
	c.callOnRetryExhausted([]*BackupResult{})
	c.callOnScheduleChange(&Schedule{}, &Schedule{})
}

func TestSchedulerCallbacks_InvokesCallbacks(t *testing.T) {
	var startCalled, successCalled, failureCalled, exhaustedCalled, changeCalled bool

	c := &SchedulerCallbacks{
		OnBackupStart: func(*BackupResult) {
			startCalled = true
		},
		OnBackupSuccess: func(*BackupResult) {
			successCalled = true
		},
		OnBackupFailure: func(*BackupResult) {
			failureCalled = true
		},
		OnRetryExhausted: func([]*BackupResult) {
			exhaustedCalled = true
		},
		OnScheduleChange: func(*Schedule, *Schedule) {
			changeCalled = true
		},
	}

	c.callOnBackupStart(&BackupResult{})
	c.callOnBackupSuccess(&BackupResult{})
	c.callOnBackupFailure(&BackupResult{})
	c.callOnRetryExhausted([]*BackupResult{})
	c.callOnScheduleChange(&Schedule{}, &Schedule{})

	if !startCalled {
		t.Error("OnBackupStart not called")
	}
	if !successCalled {
		t.Error("OnBackupSuccess not called")
	}
	if !failureCalled {
		t.Error("OnBackupFailure not called")
	}
	if !exhaustedCalled {
		t.Error("OnRetryExhausted not called")
	}
	if !changeCalled {
		t.Error("OnScheduleChange not called")
	}
}

func TestLoggingCallbacks(t *testing.T) {
	var logs []string
	var mu sync.Mutex

	logf := func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		logs = append(logs, format)
	}

	c := LoggingCallbacks(logf)

	c.OnBackupStart(&BackupResult{Attempt: 1})
	c.OnBackupSuccess(&BackupResult{
		StartTime: time.Now().Add(-time.Minute),
		EndTime:   time.Now(),
		Attempt:   1,
	})
	c.OnBackupFailure(&BackupResult{Attempt: 1, WillRetry: true})
	c.OnRetryExhausted([]*BackupResult{{}, {}})
	c.OnScheduleChange(&Schedule{Expression: "old"}, &Schedule{Expression: "new"})

	if len(logs) != 5 {
		t.Errorf("expected 5 log entries, got %d", len(logs))
	}
}

func TestChainCallbacks(t *testing.T) {
	var count1, count2 int

	c1 := &SchedulerCallbacks{
		OnBackupStart: func(*BackupResult) { count1++ },
	}
	c2 := &SchedulerCallbacks{
		OnBackupStart: func(*BackupResult) { count2++ },
	}

	chained := ChainCallbacks(c1, c2)
	chained.OnBackupStart(&BackupResult{})

	if count1 != 1 || count2 != 1 {
		t.Errorf("expected both callbacks called, got count1=%d count2=%d", count1, count2)
	}
}

func TestDefaultCallbacks(t *testing.T) {
	c := DefaultCallbacks()

	// Should not panic
	c.callOnBackupStart(&BackupResult{})
}
