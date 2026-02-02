package service

import (
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
)

// StatusService provides system status information
type StatusService struct {
	cfg       *config.Config
	scheduler *scheduler.Scheduler
}

// NewStatusService creates a new status service
func NewStatusService(cfg *config.Config) *StatusService {
	return &StatusService{cfg: cfg}
}

// SetScheduler sets the scheduler reference
func (s *StatusService) SetScheduler(sched *scheduler.Scheduler) {
	s.scheduler = sched
}

// SystemStatus represents the overall system status
type SystemStatus struct {
	Name            string
	Role            string
	RepoURL         string
	HasShare        bool
	ShareIndex      byte
	PendingRequests int
	BackupPaths     []string
	Peer            *PeerStatus
	Consensus       *ConsensusStatus
	Mode            string
	Scheduler       *SchedulerStatus
}

// PeerStatus represents peer information
type PeerStatus struct {
	Name    string
	Address string
}

// ConsensusStatus represents consensus configuration
type ConsensusStatus struct {
	Threshold       int
	TotalKeys       int
	KeyHolders      []KeyHolderStatus
	RequireApproval bool
}

// KeyHolderStatus represents a key holder's public info
type KeyHolderStatus struct {
	ID      string
	Name    string
	IsOwner bool
}

// SchedulerStatus represents scheduler state
type SchedulerStatus struct {
	Enabled   bool
	Schedule  string
	Paths     []string
	LastRun   string
	LastError string
	NextRun   string
}

// GetSystemStatus returns the current system status
func (s *StatusService) GetSystemStatus(pendingCount int) *SystemStatus {
	status := &SystemStatus{
		Name:            s.cfg.Name,
		Role:            string(s.cfg.Role),
		RepoURL:         s.cfg.RepoURL,
		HasShare:        s.cfg.LocalShare != nil,
		ShareIndex:      s.cfg.ShareIndex,
		PendingRequests: pendingCount,
		BackupPaths:     s.cfg.BackupPaths,
	}

	// Determine mode
	if s.cfg.Consensus != nil {
		status.Mode = "consensus"
	} else if s.cfg.LocalShare != nil {
		status.Mode = "sss"
	} else {
		status.Mode = "none"
	}

	// Add peer info
	if s.cfg.Peer != nil {
		status.Peer = &PeerStatus{
			Name:    s.cfg.Peer.Name,
			Address: s.cfg.Peer.Address,
		}
	}

	// Add consensus info
	if s.cfg.Consensus != nil {
		holders := make([]KeyHolderStatus, len(s.cfg.Consensus.KeyHolders))
		for i, kh := range s.cfg.Consensus.KeyHolders {
			holders[i] = KeyHolderStatus{
				ID:      kh.ID,
				Name:    kh.Name,
				IsOwner: kh.IsOwner,
			}
		}
		status.Consensus = &ConsensusStatus{
			Threshold:       s.cfg.Consensus.Threshold,
			TotalKeys:       s.cfg.Consensus.TotalKeys,
			KeyHolders:      holders,
			RequireApproval: s.cfg.Consensus.RequireApproval,
		}
	}

	// Add scheduler info
	if s.scheduler != nil {
		lastRun, lastErr, nextRun := s.scheduler.Status()
		schedStatus := &SchedulerStatus{
			Enabled:  true,
			Schedule: s.cfg.BackupSchedule,
			Paths:    s.cfg.BackupPaths,
		}
		if !lastRun.IsZero() {
			schedStatus.LastRun = lastRun.Format("2006-01-02T15:04:05Z07:00")
			if lastErr != nil {
				schedStatus.LastError = lastErr.Error()
			}
		}
		if !nextRun.IsZero() {
			schedStatus.NextRun = nextRun.Format("2006-01-02T15:04:05Z07:00")
		}
		status.Scheduler = schedStatus
	}

	return status
}

// GetScheduleInfo returns current schedule configuration
type ScheduleInfo struct {
	Schedule  string
	Paths     []string
	Enabled   bool
	LastRun   string
	LastError string
	NextRun   string
}

func (s *StatusService) GetScheduleInfo() *ScheduleInfo {
	info := &ScheduleInfo{
		Schedule: s.cfg.BackupSchedule,
		Paths:    s.cfg.BackupPaths,
		Enabled:  s.scheduler != nil,
	}

	if s.scheduler != nil {
		lastRun, lastErr, nextRun := s.scheduler.Status()
		if !lastRun.IsZero() {
			info.LastRun = lastRun.Format("2006-01-02T15:04:05Z07:00")
			if lastErr != nil {
				info.LastError = lastErr.Error()
			}
		}
		if !nextRun.IsZero() {
			info.NextRun = nextRun.Format("2006-01-02T15:04:05Z07:00")
		}
	}

	return info
}

// UpdateSchedule updates the backup schedule
func (s *StatusService) UpdateSchedule(schedule string, paths []string) error {
	return s.cfg.SetSchedule(schedule, paths)
}

// HasScheduler returns true if a scheduler is attached
func (s *StatusService) HasScheduler() bool {
	return s.scheduler != nil
}

// HotReloadSchedule updates the running scheduler's schedule without restart
func (s *StatusService) HotReloadSchedule(schedule *scheduler.Schedule) {
	if s.scheduler != nil {
		s.scheduler.UpdateSchedule(schedule)
	}
}

// GetBackupHistory returns recent backup results from the scheduler
func (s *StatusService) GetBackupHistory(limit int) []*scheduler.BackupResult {
	if s.scheduler == nil {
		return nil
	}
	return s.scheduler.GetHistory(limit)
}

// IsInitialized returns true if the system is initialized
func (s *StatusService) IsInitialized() bool {
	return s.cfg.Name != ""
}

// GetConfigDir returns the config directory path
func (s *StatusService) GetConfigDir() string {
	return s.cfg.ConfigDir
}
