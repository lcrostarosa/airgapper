package api

import (
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// ============================================================================
// Restore Request DTOs
// ============================================================================

// RestoreRequestDTO is the API representation of a restore request (list view)
type RestoreRequestDTO struct {
	ID         string    `json:"id"`
	Requester  string    `json:"requester"`
	SnapshotID string    `json:"snapshot_id"`
	Paths      []string  `json:"paths"`
	Reason     string    `json:"reason"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// RestoreRequestDetailDTO is the API representation of a restore request (detail view)
type RestoreRequestDetailDTO struct {
	RestoreRequestDTO
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
	ApprovedBy string     `json:"approved_by,omitempty"`
}

// RestoreRequestCreatedDTO is returned after creating a restore request
type RestoreRequestCreatedDTO struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ToRestoreRequestDTO converts a consent.RestoreRequest to a list view DTO
func ToRestoreRequestDTO(req *consent.RestoreRequest) RestoreRequestDTO {
	return RestoreRequestDTO{
		ID:         req.ID,
		Requester:  req.Requester,
		SnapshotID: req.SnapshotID,
		Paths:      req.Paths,
		Reason:     req.Reason,
		Status:     string(req.Status),
		CreatedAt:  req.CreatedAt,
		ExpiresAt:  req.ExpiresAt,
	}
}

// ToRestoreRequestDTOs converts a slice of consent.RestoreRequest to list view DTOs
func ToRestoreRequestDTOs(reqs []*consent.RestoreRequest) []RestoreRequestDTO {
	dtos := make([]RestoreRequestDTO, len(reqs))
	for i, req := range reqs {
		dtos[i] = ToRestoreRequestDTO(req)
	}
	return dtos
}

// ToRestoreRequestDetailDTO converts a consent.RestoreRequest to a detail view DTO
func ToRestoreRequestDetailDTO(req *consent.RestoreRequest) RestoreRequestDetailDTO {
	return RestoreRequestDetailDTO{
		RestoreRequestDTO: ToRestoreRequestDTO(req),
		ApprovedAt:        req.ApprovedAt,
		ApprovedBy:        req.ApprovedBy,
	}
}

// ToRestoreRequestCreatedDTO converts a consent.RestoreRequest to a created DTO
func ToRestoreRequestCreatedDTO(req *consent.RestoreRequest) RestoreRequestCreatedDTO {
	return RestoreRequestCreatedDTO{
		ID:        req.ID,
		Status:    string(req.Status),
		ExpiresAt: req.ExpiresAt,
	}
}

// ============================================================================
// Deletion Request DTOs
// ============================================================================

// DeletionRequestDTO is the API representation of a deletion request (list view)
type DeletionRequestDTO struct {
	ID                string              `json:"id"`
	Requester         string              `json:"requester"`
	DeletionType      consent.DeletionType `json:"deletionType"`
	SnapshotIDs       []string            `json:"snapshotIds"`
	Paths             []string            `json:"paths"`
	Reason            string              `json:"reason"`
	Status            string              `json:"status"`
	CreatedAt         time.Time           `json:"createdAt"`
	ExpiresAt         time.Time           `json:"expiresAt"`
	RequiredApprovals int                 `json:"requiredApprovals"`
	CurrentApprovals  int                 `json:"currentApprovals"`
}

// DeletionRequestDetailDTO is the API representation of a deletion request (detail view)
type DeletionRequestDetailDTO struct {
	ID                string              `json:"id"`
	Requester         string              `json:"requester"`
	DeletionType      consent.DeletionType `json:"deletionType"`
	SnapshotIDs       []string            `json:"snapshotIds"`
	Paths             []string            `json:"paths"`
	Reason            string              `json:"reason"`
	Status            string              `json:"status"`
	CreatedAt         time.Time           `json:"createdAt"`
	ExpiresAt         time.Time           `json:"expiresAt"`
	ApprovedAt        *time.Time          `json:"approvedAt,omitempty"`
	ExecutedAt        *time.Time          `json:"executedAt,omitempty"`
	RequiredApprovals int                 `json:"requiredApprovals"`
	Approvals         []consent.Approval  `json:"approvals"`
}

// DeletionRequestCreatedDTO is returned after creating a deletion request
type DeletionRequestCreatedDTO struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ToDeletionRequestDTO converts a consent.DeletionRequest to a list view DTO
func ToDeletionRequestDTO(del *consent.DeletionRequest) DeletionRequestDTO {
	return DeletionRequestDTO{
		ID:                del.ID,
		Requester:         del.Requester,
		DeletionType:      del.DeletionType,
		SnapshotIDs:       del.SnapshotIDs,
		Paths:             del.Paths,
		Reason:            del.Reason,
		Status:            string(del.Status),
		CreatedAt:         del.CreatedAt,
		ExpiresAt:         del.ExpiresAt,
		RequiredApprovals: del.RequiredApprovals,
		CurrentApprovals:  len(del.Approvals),
	}
}

// ToDeletionRequestDTOs converts a slice of consent.DeletionRequest to list view DTOs
func ToDeletionRequestDTOs(dels []*consent.DeletionRequest) []DeletionRequestDTO {
	dtos := make([]DeletionRequestDTO, len(dels))
	for i, del := range dels {
		dtos[i] = ToDeletionRequestDTO(del)
	}
	return dtos
}

// ToDeletionRequestDetailDTO converts a consent.DeletionRequest to a detail view DTO
func ToDeletionRequestDetailDTO(del *consent.DeletionRequest) DeletionRequestDetailDTO {
	return DeletionRequestDetailDTO{
		ID:                del.ID,
		Requester:         del.Requester,
		DeletionType:      del.DeletionType,
		SnapshotIDs:       del.SnapshotIDs,
		Paths:             del.Paths,
		Reason:            del.Reason,
		Status:            string(del.Status),
		CreatedAt:         del.CreatedAt,
		ExpiresAt:         del.ExpiresAt,
		ApprovedAt:        del.ApprovedAt,
		ExecutedAt:        del.ExecutedAt,
		RequiredApprovals: del.RequiredApprovals,
		Approvals:         del.Approvals,
	}
}

// ToDeletionRequestCreatedDTO converts a consent.DeletionRequest to a created DTO
func ToDeletionRequestCreatedDTO(del *consent.DeletionRequest) DeletionRequestCreatedDTO {
	return DeletionRequestCreatedDTO{
		ID:        del.ID,
		Status:    string(del.Status),
		ExpiresAt: del.ExpiresAt,
	}
}

// ============================================================================
// Key Holder DTOs
// ============================================================================

// KeyHolderDTO is the API representation of a key holder
type KeyHolderDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PublicKey string    `json:"publicKey"`
	Address   string    `json:"address,omitempty"`
	JoinedAt  time.Time `json:"joinedAt"`
	IsOwner   bool      `json:"isOwner,omitempty"`
}

// ConsensusInfoDTO is the API representation of consensus configuration
type ConsensusInfoDTO struct {
	Threshold       int            `json:"threshold"`
	TotalKeys       int            `json:"totalKeys"`
	KeyHolders      []KeyHolderDTO `json:"keyHolders"`
	RequireApproval bool           `json:"requireApproval"`
}

// KeyHolderRegisteredDTO is returned after registering a key holder
type KeyHolderRegisteredDTO struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	JoinedAt time.Time `json:"joinedAt"`
}

// ToKeyHolderDTO converts a config.KeyHolder to a DTO
func ToKeyHolderDTO(kh *config.KeyHolder) KeyHolderDTO {
	return KeyHolderDTO{
		ID:        kh.ID,
		Name:      kh.Name,
		PublicKey: crypto.EncodePublicKey(kh.PublicKey),
		Address:   kh.Address,
		JoinedAt:  kh.JoinedAt,
		IsOwner:   kh.IsOwner,
	}
}

// ToKeyHolderDTOs converts a slice of config.KeyHolder to DTOs
func ToKeyHolderDTOs(holders []config.KeyHolder) []KeyHolderDTO {
	dtos := make([]KeyHolderDTO, len(holders))
	for i := range holders {
		dtos[i] = ToKeyHolderDTO(&holders[i])
	}
	return dtos
}

// ============================================================================
// Approval Progress DTOs
// ============================================================================

// ApprovalProgressDTO represents the progress of a multi-signature approval
type ApprovalProgressDTO struct {
	Status            string `json:"status"`
	CurrentApprovals  int    `json:"currentApprovals"`
	RequiredApprovals int    `json:"requiredApprovals"`
	IsApproved        bool   `json:"isApproved"`
}

// ============================================================================
// Backup History DTOs
// ============================================================================

// BackupResultDTO is the API representation of a backup result
type BackupResultDTO struct {
	ScheduledTime string `json:"scheduled_time"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	DurationMs    int64  `json:"duration_ms"`
	Success       bool   `json:"success"`
	Attempt       int    `json:"attempt"`
	IsRetry       bool   `json:"is_retry"`
	Error         string `json:"error,omitempty"`
}

// BackupHistoryDTO is the API representation of backup history
type BackupHistoryDTO struct {
	History []BackupResultDTO `json:"history"`
	Count   int               `json:"count"`
}

// ToBackupResultDTO converts a scheduler.BackupResult to a DTO
func ToBackupResultDTO(r *scheduler.BackupResult) BackupResultDTO {
	dto := BackupResultDTO{
		ScheduledTime: r.ScheduledTime.Format("2006-01-02T15:04:05Z07:00"),
		StartTime:     r.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		EndTime:       r.EndTime.Format("2006-01-02T15:04:05Z07:00"),
		DurationMs:    r.Duration().Milliseconds(),
		Success:       r.Success,
		Attempt:       r.Attempt,
		IsRetry:       r.IsRetry(),
	}
	if r.Error != nil {
		dto.Error = r.Error.Error()
	}
	return dto
}

// ToBackupResultDTOs converts a slice of scheduler.BackupResult to DTOs
func ToBackupResultDTOs(results []*scheduler.BackupResult) []BackupResultDTO {
	dtos := make([]BackupResultDTO, len(results))
	for i, r := range results {
		dtos[i] = ToBackupResultDTO(r)
	}
	return dtos
}

// ToBackupHistoryDTO creates a backup history response
func ToBackupHistoryDTO(results []*scheduler.BackupResult) BackupHistoryDTO {
	return BackupHistoryDTO{
		History: ToBackupResultDTOs(results),
		Count:   len(results),
	}
}

// ============================================================================
// Simple Response DTOs
// ============================================================================

// StatusMessageDTO is a simple status response
type StatusMessageDTO struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// NewStatusMessage creates a new status message DTO
func NewStatusMessage(status, message string) StatusMessageDTO {
	return StatusMessageDTO{Status: status, Message: message}
}

// ============================================================================
// Integrity DTOs
// ============================================================================

// IntegrityLastCheckDTO represents the last integrity check result
type IntegrityLastCheckDTO struct {
	Timestamp    time.Time `json:"timestamp"`
	Passed       bool      `json:"passed"`
	TotalFiles   int       `json:"totalFiles"`
	CorruptFiles int       `json:"corruptFiles"`
	Duration     string    `json:"duration"`
}

// IntegrityCheckDTO is the response for the integrity check endpoint
type IntegrityCheckDTO struct {
	Status         string                 `json:"status"`
	StorageRunning bool                   `json:"storageRunning"`
	UsedBytes      int64                  `json:"usedBytes"`
	DiskUsagePct   int                    `json:"diskUsagePct"`
	DiskFreeBytes  int64                  `json:"diskFreeBytes"`
	HasPolicy      bool                   `json:"hasPolicy"`
	PolicyID       string                 `json:"policyId,omitempty"`
	RequestCount   int64                  `json:"requestCount"`
	LastCheck      *IntegrityLastCheckDTO `json:"lastCheck,omitempty"`
}

// IntegrityCheckResultDTO represents a detailed integrity check result
type IntegrityCheckResultDTO struct {
	Timestamp    time.Time `json:"timestamp"`
	Passed       bool      `json:"passed"`
	TotalFiles   int       `json:"totalFiles"`
	CheckedFiles int       `json:"checkedFiles"`
	CorruptFiles int       `json:"corruptFiles"`
	MissingFiles int       `json:"missingFiles"`
	Duration     string    `json:"duration"`
	Errors       []string  `json:"errors,omitempty"`
}

// VerificationConfigDTO is the response for the verification config endpoint
type VerificationConfigDTO struct {
	Enabled             bool                     `json:"enabled"`
	Interval            string                   `json:"interval"`
	CheckType           string                   `json:"checkType"`
	RepoName            string                   `json:"repoName"`
	SnapshotID          string                   `json:"snapshotId"`
	AlertOnCorruption   bool                     `json:"alertOnCorruption"`
	AlertWebhook        string                   `json:"alertWebhook"`
	ConsecutiveFailures int                      `json:"consecutiveFailures"`
	LastCheck           *time.Time               `json:"lastCheck,omitempty"`
	LastResult          *IntegrityCheckResultDTO `json:"lastResult,omitempty"`
}

// VerificationConfigUpdatedDTO is the response after updating verification config
type VerificationConfigUpdatedDTO struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Config  *VerificationConfigDTO `json:"config"`
}

// ManualCheckResponseDTO is the response after running a manual integrity check
type ManualCheckResponseDTO struct {
	Status    string                   `json:"status"`
	CheckType string                   `json:"checkType"`
	Result    *IntegrityCheckResultDTO `json:"result"`
}

// RecordAddedDTO is the response after adding a verification record
type RecordAddedDTO struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ToIntegrityCheckDTO creates an IntegrityCheckDTO from storage status and optional check history
func ToIntegrityCheckDTO(status storage.Status, lastCheck *integrity.CheckResult) IntegrityCheckDTO {
	dto := IntegrityCheckDTO{
		Status:         "ok",
		StorageRunning: status.Running,
		UsedBytes:      status.UsedBytes,
		DiskUsagePct:   status.DiskUsagePct,
		DiskFreeBytes:  status.DiskFreeBytes,
		HasPolicy:      status.HasPolicy,
		PolicyID:       status.PolicyID,
		RequestCount:   status.RequestCount,
	}
	if lastCheck != nil {
		dto.LastCheck = &IntegrityLastCheckDTO{
			Timestamp:    lastCheck.Timestamp,
			Passed:       lastCheck.Passed,
			TotalFiles:   lastCheck.TotalFiles,
			CorruptFiles: lastCheck.CorruptFiles,
			Duration:     lastCheck.Duration,
		}
	}
	return dto
}

// ToIntegrityCheckResultDTO converts an integrity.CheckResult to a DTO
func ToIntegrityCheckResultDTO(r *integrity.CheckResult) *IntegrityCheckResultDTO {
	if r == nil {
		return nil
	}
	return &IntegrityCheckResultDTO{
		Timestamp:    r.Timestamp,
		Passed:       r.Passed,
		TotalFiles:   r.TotalFiles,
		CheckedFiles: r.CheckedFiles,
		CorruptFiles: r.CorruptFiles,
		MissingFiles: r.MissingFiles,
		Duration:     r.Duration,
		Errors:       r.Errors,
	}
}

// ToVerificationConfigDTO converts an integrity.VerificationConfig to a DTO
func ToVerificationConfigDTO(cfg *integrity.VerificationConfig) VerificationConfigDTO {
	dto := VerificationConfigDTO{
		Enabled:             cfg.Enabled,
		Interval:            cfg.Interval,
		CheckType:           cfg.CheckType,
		RepoName:            cfg.RepoName,
		SnapshotID:          cfg.SnapshotID,
		AlertOnCorruption:   cfg.AlertOnCorruption,
		AlertWebhook:        cfg.AlertWebhook,
		ConsecutiveFailures: cfg.ConsecutiveFailures,
		LastCheck:           cfg.LastCheck,
		LastResult:          ToIntegrityCheckResultDTO(cfg.LastResult),
	}
	return dto
}

// ============================================================================
// System Status DTOs
// ============================================================================

// PeerDTO represents a peer in the system status
type PeerDTO struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// ConsensusKeyHolderDTO represents a key holder in consensus info (minimal)
type ConsensusKeyHolderDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsOwner bool   `json:"isOwner"`
}

// ConsensusDTO represents consensus configuration in system status
type ConsensusDTO struct {
	Threshold       int                     `json:"threshold"`
	TotalKeys       int                     `json:"totalKeys"`
	KeyHolders      []ConsensusKeyHolderDTO `json:"keyHolders"`
	RequireApproval bool                    `json:"requireApproval"`
}

// SystemStatusDTO is the response for the status endpoint
type SystemStatusDTO struct {
	Name            string        `json:"name"`
	Role            string        `json:"role"`
	RepoURL         string        `json:"repo_url"`
	HasShare        bool          `json:"has_share"`
	ShareIndex      int           `json:"share_index"`
	PendingRequests int           `json:"pending_requests"`
	BackupPaths     []string      `json:"backup_paths"`
	Mode            string        `json:"mode"`
	Peer            *PeerDTO      `json:"peer,omitempty"`
	Consensus       *ConsensusDTO `json:"consensus,omitempty"`
	Scheduler       interface{}   `json:"scheduler,omitempty"`
}

// ============================================================================
// Schedule DTOs
// ============================================================================

// ScheduleUpdatedDTO is the response after updating the schedule
type ScheduleUpdatedDTO struct {
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	HotReloaded *bool  `json:"hot_reloaded,omitempty"`
}

// ============================================================================
// Storage DTOs
// ============================================================================

// StorageStatusDTO is the response for storage status
type StorageStatusDTO struct {
	Configured      bool      `json:"configured"`
	Running         bool      `json:"running"`
	StartTime       time.Time `json:"startTime,omitempty"`
	BasePath        string    `json:"basePath,omitempty"`
	AppendOnly      bool      `json:"appendOnly,omitempty"`
	QuotaBytes      int64     `json:"quotaBytes,omitempty"`
	UsedBytes       int64     `json:"usedBytes,omitempty"`
	RequestCount    int64     `json:"requestCount,omitempty"`
	HasPolicy       bool      `json:"hasPolicy,omitempty"`
	PolicyID        string    `json:"policyId,omitempty"`
	MaxDiskUsagePct int       `json:"maxDiskUsagePct,omitempty"`
	DiskUsagePct    int       `json:"diskUsagePct,omitempty"`
	DiskFreeBytes   int64     `json:"diskFreeBytes,omitempty"`
	DiskTotalBytes  int64     `json:"diskTotalBytes,omitempty"`
}

// ToStorageStatusDTO converts storage.Status to a DTO
func ToStorageStatusDTO(status storage.Status) StorageStatusDTO {
	return StorageStatusDTO{
		Configured:      true,
		Running:         status.Running,
		StartTime:       status.StartTime,
		BasePath:        status.BasePath,
		AppendOnly:      status.AppendOnly,
		QuotaBytes:      status.QuotaBytes,
		UsedBytes:       status.UsedBytes,
		RequestCount:    status.RequestCount,
		HasPolicy:       status.HasPolicy,
		PolicyID:        status.PolicyID,
		MaxDiskUsagePct: status.MaxDiskUsagePct,
		DiskUsagePct:    status.DiskUsagePct,
		DiskFreeBytes:   status.DiskFreeBytes,
		DiskTotalBytes:  status.DiskTotalBytes,
	}
}

// ============================================================================
// Policy DTOs
// ============================================================================

// PolicyStatusDTO is the response when no policy is set
type PolicyStatusDTO struct {
	HasPolicy bool `json:"hasPolicy"`
}

// PolicyResponseDTO is the response with policy data
type PolicyResponseDTO struct {
	HasPolicy     bool        `json:"hasPolicy"`
	Policy        interface{} `json:"policy,omitempty"`
	PolicyJSON    string      `json:"policyJSON,omitempty"`
	IsFullySigned bool        `json:"isFullySigned"`
	IsActive      bool        `json:"isActive,omitempty"`
}

// ============================================================================
// Vault DTOs
// ============================================================================

// VaultInitResponseDTO is the response after initializing a vault
type VaultInitResponseDTO struct {
	Name      string `json:"name"`
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
	Threshold int    `json:"threshold"`
	TotalKeys int    `json:"totalKeys"`
}

// ============================================================================
// Host DTOs
// ============================================================================

// HostInitResponseDTO is the response after initializing a host
type HostInitResponseDTO struct {
	Name        string `json:"name"`
	KeyID       string `json:"keyId"`
	PublicKey   string `json:"publicKey"`
	StorageURL  string `json:"storageUrl"`
	StoragePath string `json:"storagePath"`
}
