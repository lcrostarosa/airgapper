package grpc

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
)

// mapSlice converts a slice of type T to a slice of type R using the provided converter function.
// This is a generic helper to reduce boilerplate in slice conversion functions.
func mapSlice[T, R any](items []T, convert func(T) R) []R {
	result := make([]R, len(items))
	for i, item := range items {
		result[i] = convert(item)
	}
	return result
}

// ============================================================================
// Role Converters
// ============================================================================

func toProtoRole(role config.Role) airgapperv1.Role {
	switch role {
	case config.RoleOwner:
		return airgapperv1.Role_ROLE_OWNER
	case config.RoleHost:
		return airgapperv1.Role_ROLE_HOST
	default:
		return airgapperv1.Role_ROLE_UNSPECIFIED
	}
}

func toProtoOperationMode(cfg *config.Config) airgapperv1.OperationMode {
	if cfg.Consensus != nil {
		return airgapperv1.OperationMode_OPERATION_MODE_CONSENSUS
	}
	if cfg.LocalShare != nil {
		return airgapperv1.OperationMode_OPERATION_MODE_SSS
	}
	return airgapperv1.OperationMode_OPERATION_MODE_NONE
}

// ============================================================================
// Status Converters
// ============================================================================

func toProtoRequestStatus(status consent.RequestStatus) airgapperv1.RequestStatus {
	switch status {
	case consent.StatusPending:
		return airgapperv1.RequestStatus_REQUEST_STATUS_PENDING
	case consent.StatusApproved:
		return airgapperv1.RequestStatus_REQUEST_STATUS_APPROVED
	case consent.StatusDenied:
		return airgapperv1.RequestStatus_REQUEST_STATUS_DENIED
	case consent.StatusExpired:
		return airgapperv1.RequestStatus_REQUEST_STATUS_EXPIRED
	default:
		return airgapperv1.RequestStatus_REQUEST_STATUS_UNSPECIFIED
	}
}

func toProtoDeletionType(dt consent.DeletionType) airgapperv1.DeletionType {
	switch dt {
	case consent.DeletionTypeSnapshot:
		return airgapperv1.DeletionType_DELETION_TYPE_SNAPSHOT
	case consent.DeletionTypePath:
		return airgapperv1.DeletionType_DELETION_TYPE_PATH
	case consent.DeletionTypePrune:
		return airgapperv1.DeletionType_DELETION_TYPE_PRUNE
	case consent.DeletionTypeAll:
		return airgapperv1.DeletionType_DELETION_TYPE_ALL
	default:
		return airgapperv1.DeletionType_DELETION_TYPE_UNSPECIFIED
	}
}

func fromProtoDeletionType(dt airgapperv1.DeletionType) consent.DeletionType {
	switch dt {
	case airgapperv1.DeletionType_DELETION_TYPE_SNAPSHOT:
		return consent.DeletionTypeSnapshot
	case airgapperv1.DeletionType_DELETION_TYPE_PATH:
		return consent.DeletionTypePath
	case airgapperv1.DeletionType_DELETION_TYPE_PRUNE:
		return consent.DeletionTypePrune
	case airgapperv1.DeletionType_DELETION_TYPE_ALL:
		return consent.DeletionTypeAll
	default:
		return ""
	}
}

// ============================================================================
// Key Holder Converters
// ============================================================================

func toProtoKeyHolder(kh *config.KeyHolder) *airgapperv1.KeyHolder {
	if kh == nil {
		return nil
	}
	return &airgapperv1.KeyHolder{
		Id:        kh.ID,
		Name:      kh.Name,
		PublicKey: crypto.EncodePublicKey(kh.PublicKey),
		Address:   kh.Address,
		JoinedAt:  timestamppb.New(kh.JoinedAt),
		IsOwner:   kh.IsOwner,
	}
}

func toProtoKeyHolders(holders []config.KeyHolder) []*airgapperv1.KeyHolder {
	return mapSlice(holders, func(kh config.KeyHolder) *airgapperv1.KeyHolder {
		return toProtoKeyHolder(&kh)
	})
}

func toProtoConsensusInfo(consensus *config.ConsensusConfig) *airgapperv1.ConsensusInfo {
	if consensus == nil {
		return nil
	}
	return &airgapperv1.ConsensusInfo{
		Threshold:       int32(consensus.Threshold),
		TotalKeys:       int32(consensus.TotalKeys),
		KeyHolders:      toProtoKeyHolders(consensus.KeyHolders),
		RequireApproval: consensus.RequireApproval,
	}
}

// ============================================================================
// Approval Converters
// ============================================================================

func toProtoApproval(a consent.Approval) *airgapperv1.Approval {
	return &airgapperv1.Approval{
		KeyHolderId:   a.KeyHolderID,
		KeyHolderName: a.KeyHolderName,
		Signature:     string(a.Signature), // Convert []byte to string
		ApprovedAt:    timestamppb.New(a.ApprovedAt),
	}
}

func toProtoApprovals(approvals []consent.Approval) []*airgapperv1.Approval {
	return mapSlice(approvals, toProtoApproval)
}

// ============================================================================
// Restore Request Converters
// ============================================================================

func toProtoRestoreRequest(req *consent.RestoreRequest) *airgapperv1.RestoreRequest {
	if req == nil {
		return nil
	}

	result := &airgapperv1.RestoreRequest{
		Id:                req.ID,
		Requester:         req.Requester,
		SnapshotId:        req.SnapshotID,
		Paths:             req.Paths,
		Reason:            req.Reason,
		Status:            toProtoRequestStatus(req.Status),
		CreatedAt:         timestamppb.New(req.CreatedAt),
		ExpiresAt:         timestamppb.New(req.ExpiresAt),
		ApprovedBy:        req.ApprovedBy,
		RequiredApprovals: int32(req.RequiredApprovals),
		Approvals:         toProtoApprovals(req.Approvals),
	}

	if req.ApprovedAt != nil {
		result.ApprovedAt = timestamppb.New(*req.ApprovedAt)
	}

	return result
}

func toProtoRestoreRequests(reqs []*consent.RestoreRequest) []*airgapperv1.RestoreRequest {
	return mapSlice(reqs, toProtoRestoreRequest)
}

// ============================================================================
// Deletion Request Converters
// ============================================================================

func toProtoDeletionRequest(del *consent.DeletionRequest) *airgapperv1.DeletionRequest {
	if del == nil {
		return nil
	}

	result := &airgapperv1.DeletionRequest{
		Id:                del.ID,
		Requester:         del.Requester,
		DeletionType:      toProtoDeletionType(del.DeletionType),
		SnapshotIds:       del.SnapshotIDs,
		Paths:             del.Paths,
		Reason:            del.Reason,
		Status:            toProtoRequestStatus(del.Status),
		CreatedAt:         timestamppb.New(del.CreatedAt),
		ExpiresAt:         timestamppb.New(del.ExpiresAt),
		RequiredApprovals: int32(del.RequiredApprovals),
		CurrentApprovals:  int32(len(del.Approvals)),
		Approvals:         toProtoApprovals(del.Approvals),
	}

	if del.ApprovedAt != nil {
		result.ApprovedAt = timestamppb.New(*del.ApprovedAt)
	}
	if del.ExecutedAt != nil {
		result.ExecutedAt = timestamppb.New(*del.ExecutedAt)
	}

	return result
}

func toProtoDeletionRequests(dels []*consent.DeletionRequest) []*airgapperv1.DeletionRequest {
	return mapSlice(dels, toProtoDeletionRequest)
}

// ============================================================================
// Integrity Converters
// ============================================================================

func toProtoIntegrityCheckResult(r *integrity.CheckResult) *airgapperv1.IntegrityCheckResult {
	if r == nil {
		return nil
	}
	return &airgapperv1.IntegrityCheckResult{
		Timestamp:    timestamppb.New(r.Timestamp),
		Passed:       r.Passed,
		TotalFiles:   int32(r.TotalFiles),
		CheckedFiles: int32(r.CheckedFiles),
		CorruptFiles: int32(r.CorruptFiles),
		MissingFiles: int32(r.MissingFiles),
		Duration:     r.Duration,
		Errors:       r.Errors,
	}
}

// ============================================================================
// Timestamp Helpers
// ============================================================================

func timeToTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
