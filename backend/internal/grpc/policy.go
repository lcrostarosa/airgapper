package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
	"github.com/lcrostarosa/airgapper/backend/internal/policy"
)

// Common errors for gRPC handlers
var (
	errStorageNotConfigured   = errors.New("storage server not configured")
	errIntegrityNotConfigured = errors.New("integrity checker not configured")
	errOwnerInfoRequired      = errors.New("owner information required")
	errHostInfoRequired       = errors.New("host information required")
	errInvalidSignerRole      = errors.New("signerRole must be 'owner' or 'host'")
)

// policyServer implements the PolicyService
type policyServer struct {
	airgapperv1connect.UnimplementedPolicyServiceHandler
	server *Server
}

func newPolicyServer(s *Server) airgapperv1connect.PolicyServiceHandler {
	return &policyServer{server: s}
}

func (p *policyServer) GetPolicy(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetPolicyRequest],
) (*connect.Response[airgapperv1.GetPolicyResponse], error) {
	if p.server.storageServer == nil {
		return connect.NewResponse(&airgapperv1.GetPolicyResponse{
			HasPolicy: false,
		}), nil
	}

	pol := p.server.storageServer.GetPolicy()
	if pol == nil {
		return connect.NewResponse(&airgapperv1.GetPolicyResponse{
			HasPolicy: false,
		}), nil
	}

	policyJSON, _ := pol.ToJSON()
	return connect.NewResponse(&airgapperv1.GetPolicyResponse{
		HasPolicy:     true,
		Policy:        policyToProto(pol),
		PolicyJson:    string(policyJSON),
		IsFullySigned: pol.IsFullySigned(),
		IsActive:      pol.IsActive(),
	}), nil
}

func (p *policyServer) CreatePolicy(
	ctx context.Context,
	req *connect.Request[airgapperv1.CreatePolicyRequest],
) (*connect.Response[airgapperv1.CreatePolicyResponse], error) {
	if p.server.storageServer == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errStorageNotConfigured)
	}

	msg := req.Msg

	// Validate required fields
	if msg.OwnerName == "" || msg.OwnerKeyId == "" || msg.OwnerPublicKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errOwnerInfoRequired)
	}
	if msg.HostName == "" || msg.HostKeyId == "" || msg.HostPublicKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errHostInfoRequired)
	}

	// Create policy
	pol := policy.NewPolicy(
		msg.OwnerName, msg.OwnerKeyId, msg.OwnerPublicKey,
		msg.HostName, msg.HostKeyId, msg.HostPublicKey,
	)

	// Set terms
	if msg.RetentionDays > 0 {
		pol.RetentionDays = int(msg.RetentionDays)
	}
	if msg.DeletionMode != airgapperv1.DeletionMode_DELETION_MODE_UNSPECIFIED {
		pol.DeletionMode = deletionModeFromProto(msg.DeletionMode)
	}
	if msg.MaxStorageBytes > 0 {
		pol.MaxStorageBytes = msg.MaxStorageBytes
	}

	// Apply signatures if provided
	if msg.OwnerSignature != "" {
		pol.OwnerSignature = msg.OwnerSignature
	}
	if msg.HostSignature != "" {
		pol.HostSignature = msg.HostSignature
	}

	// If both signatures present, set the policy
	if pol.IsFullySigned() {
		if err := p.server.storageServer.SetPolicy(pol); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	policyJSON, _ := pol.ToJSON()
	return connect.NewResponse(&airgapperv1.CreatePolicyResponse{
		Policy:        policyToProto(pol),
		PolicyJson:    string(policyJSON),
		IsFullySigned: pol.IsFullySigned(),
	}), nil
}

func (p *policyServer) SignPolicy(
	ctx context.Context,
	req *connect.Request[airgapperv1.SignPolicyRequest],
) (*connect.Response[airgapperv1.SignPolicyResponse], error) {
	if p.server.storageServer == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errStorageNotConfigured)
	}

	msg := req.Msg

	// Parse the policy
	pol, err := policy.FromJSON([]byte(msg.PolicyJson))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Apply signature
	switch msg.SignerRole {
	case "owner":
		pol.OwnerSignature = msg.Signature
	case "host":
		pol.HostSignature = msg.Signature
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errInvalidSignerRole)
	}

	// If both signatures present, verify and set
	if pol.IsFullySigned() {
		if err := pol.Verify(); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if err := p.server.storageServer.SetPolicy(pol); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	policyJSON, _ := pol.ToJSON()
	return connect.NewResponse(&airgapperv1.SignPolicyResponse{
		Policy:        policyToProto(pol),
		PolicyJson:    string(policyJSON),
		IsFullySigned: pol.IsFullySigned(),
	}), nil
}

// policyToProto converts a policy.Policy to airgapperv1.Policy
func policyToProto(p *policy.Policy) *airgapperv1.Policy {
	if p == nil {
		return nil
	}

	proto := &airgapperv1.Policy{
		Id:               p.ID,
		Version:          int32(p.Version),
		Name:             p.Name,
		OwnerName:        p.OwnerName,
		OwnerKeyId:       p.OwnerKeyID,
		OwnerPublicKey:   p.OwnerPubKey,
		HostName:         p.HostName,
		HostKeyId:        p.HostKeyID,
		HostPublicKey:    p.HostPubKey,
		RetentionDays:    int32(p.RetentionDays),
		DeletionMode:     deletionModeToProto(p.DeletionMode),
		AppendOnlyLocked: p.AppendOnlyLocked,
		MaxStorageBytes:  p.MaxStorageBytes,
		CreatedAt:        timestamppb.New(p.CreatedAt),
		EffectiveAt:      timestamppb.New(p.EffectiveAt),
		OwnerSignature:   p.OwnerSignature,
		HostSignature:    p.HostSignature,
	}

	if !p.ExpiresAt.IsZero() {
		proto.ExpiresAt = timestamppb.New(p.ExpiresAt)
	}

	return proto
}

// deletionModeToProto converts policy.DeletionMode to airgapperv1.DeletionMode
func deletionModeToProto(mode policy.DeletionMode) airgapperv1.DeletionMode {
	switch mode {
	case policy.DeletionBothRequired:
		return airgapperv1.DeletionMode_DELETION_MODE_BOTH_REQUIRED
	case policy.DeletionOwnerOnly:
		return airgapperv1.DeletionMode_DELETION_MODE_OWNER_ONLY
	case policy.DeletionTimeLockOnly:
		return airgapperv1.DeletionMode_DELETION_MODE_TIME_LOCK_ONLY
	case policy.DeletionNever:
		return airgapperv1.DeletionMode_DELETION_MODE_NEVER
	default:
		return airgapperv1.DeletionMode_DELETION_MODE_UNSPECIFIED
	}
}

// deletionModeFromProto converts airgapperv1.DeletionMode to policy.DeletionMode
func deletionModeFromProto(mode airgapperv1.DeletionMode) policy.DeletionMode {
	switch mode {
	case airgapperv1.DeletionMode_DELETION_MODE_BOTH_REQUIRED:
		return policy.DeletionBothRequired
	case airgapperv1.DeletionMode_DELETION_MODE_OWNER_ONLY:
		return policy.DeletionOwnerOnly
	case airgapperv1.DeletionMode_DELETION_MODE_TIME_LOCK_ONLY:
		return policy.DeletionTimeLockOnly
	case airgapperv1.DeletionMode_DELETION_MODE_NEVER:
		return policy.DeletionNever
	default:
		return policy.DeletionBothRequired
	}
}
