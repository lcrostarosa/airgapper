package grpc

import (
	"context"
	"encoding/json"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
	"github.com/lcrostarosa/airgapper/backend/internal/verification"
)

// verificationServer implements the VerificationService
type verificationServer struct {
	airgapperv1connect.UnimplementedVerificationServiceHandler
	server *Server
}

func newVerificationServer(s *Server) airgapperv1connect.VerificationServiceHandler {
	return &verificationServer{server: s}
}

// ============================================================================
// Verification Status
// ============================================================================

func (v *verificationServer) GetVerificationStatus(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetVerificationStatusRequest],
) (*connect.Response[airgapperv1.GetVerificationStatusResponse], error) {
	cfg := v.server.VerificationConfig()

	status := "disabled"
	initialized := false

	if cfg != nil && cfg.Enabled {
		status = "enabled"
		initialized = true

		// Check individual component status
		if v.server.AuditChain() != nil {
			if result, err := v.server.AuditChain().Verify(); err == nil && result.Valid {
				status = "ok"
			} else {
				status = "warning"
			}
		}
	}

	return connect.NewResponse(&airgapperv1.GetVerificationStatusResponse{
		Status:      status,
		Initialized: initialized,
	}), nil
}

// ============================================================================
// Audit Chain
// ============================================================================

func (v *verificationServer) GetAuditEntries(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetAuditEntriesRequest],
) (*connect.Response[airgapperv1.GetAuditEntriesResponse], error) {
	ac := v.server.AuditChain()
	if ac == nil {
		return connect.NewResponse(&airgapperv1.GetAuditEntriesResponse{
			Entries: []*airgapperv1.AuditEntry{},
			Total:   0,
		}), nil
	}

	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 50
	}
	offset := int(req.Msg.Offset)

	entries := ac.GetEntries(limit, offset, req.Msg.ActionFilter)

	pbEntries := make([]*airgapperv1.AuditEntry, len(entries))
	for i, e := range entries {
		pbEntries[i] = &airgapperv1.AuditEntry{
			Id:        e.ID,
			Action:    e.Operation,
			Actor:     e.HostKeyID,
			Target:    e.Path,
			Details:   e.Details,
			Hash:      e.ContentHash,
			PrevHash:  e.PreviousHash,
			Timestamp: timestamppb.New(e.Timestamp),
		}
	}

	return connect.NewResponse(&airgapperv1.GetAuditEntriesResponse{
		Entries: pbEntries,
		Total:   int32(len(entries)),
	}), nil
}

func (v *verificationServer) VerifyAuditChain(
	ctx context.Context,
	req *connect.Request[airgapperv1.VerifyAuditChainRequest],
) (*connect.Response[airgapperv1.VerifyAuditChainResponse], error) {
	ac := v.server.AuditChain()
	if ac == nil {
		return connect.NewResponse(&airgapperv1.VerifyAuditChainResponse{
			Valid:          true,
			EntriesChecked: 0,
		}), nil
	}

	result, err := ac.Verify()
	if err != nil {
		return connect.NewResponse(&airgapperv1.VerifyAuditChainResponse{
			Valid: false,
			Error: err.Error(),
		}), nil
	}

	resp := &airgapperv1.VerifyAuditChainResponse{
		Valid:          result.Valid,
		EntriesChecked: int32(result.TotalEntries),
	}

	if result.FirstBrokenAt != nil {
		resp.FirstInvalidId = result.Errors[0]
	}

	if len(result.Errors) > 0 {
		resp.Error = result.Errors[0]
	}

	return connect.NewResponse(resp), nil
}

func (v *verificationServer) ExportAuditChain(
	ctx context.Context,
	req *connect.Request[airgapperv1.ExportAuditChainRequest],
) (*connect.Response[airgapperv1.ExportAuditChainResponse], error) {
	ac := v.server.AuditChain()
	if ac == nil {
		return connect.NewResponse(&airgapperv1.ExportAuditChainResponse{
			Data:        []byte("[]"),
			ContentType: "application/json",
			Filename:    "audit-chain.json",
		}), nil
	}

	data, err := ac.Export()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.ExportAuditChainResponse{
		Data:        data,
		ContentType: "application/json",
		Filename:    "audit-chain.json",
	}), nil
}

// ============================================================================
// Challenge-Response
// ============================================================================

func (v *verificationServer) CreateChallenge(
	ctx context.Context,
	req *connect.Request[airgapperv1.CreateChallengeRequest],
) (*connect.Response[airgapperv1.CreateChallengeResponse], error) {
	// This is typically called by the owner to create a challenge
	// For now, return a placeholder - actual implementation would use owner keys
	return connect.NewResponse(&airgapperv1.CreateChallengeResponse{
		Challenge: &airgapperv1.Challenge{
			Status: "not_configured",
		},
	}), nil
}

func (v *verificationServer) ReceiveChallenge(
	ctx context.Context,
	req *connect.Request[airgapperv1.ReceiveChallengeRequest],
) (*connect.Response[airgapperv1.ReceiveChallengeResponse], error) {
	cm := v.server.ChallengeManager()
	if cm == nil {
		return connect.NewResponse(&airgapperv1.ReceiveChallengeResponse{
			Status:  "error",
			Message: "Challenge manager not configured",
		}), nil
	}

	// Convert protobuf to internal type
	challenge := &verification.Challenge{
		ID:        req.Msg.Id,
		OwnerKeyID: req.Msg.ChallengerId,
		ExpiresAt: req.Msg.ExpiresAt.AsTime(),
	}

	if err := cm.ReceiveChallenge(challenge); err != nil {
		return connect.NewResponse(&airgapperv1.ReceiveChallengeResponse{
			Status:  "error",
			Message: err.Error(),
		}), nil
	}

	return connect.NewResponse(&airgapperv1.ReceiveChallengeResponse{
		Status:  "ok",
		Message: "Challenge received",
	}), nil
}

func (v *verificationServer) ListChallenges(
	ctx context.Context,
	req *connect.Request[airgapperv1.ListChallengesRequest],
) (*connect.Response[airgapperv1.ListChallengesResponse], error) {
	cm := v.server.ChallengeManager()
	if cm == nil {
		return connect.NewResponse(&airgapperv1.ListChallengesResponse{
			Challenges: []*airgapperv1.Challenge{},
		}), nil
	}

	pendingOnly := req.Msg.StatusFilter == "pending"
	challenges := cm.ListChallenges(pendingOnly)

	pbChallenges := make([]*airgapperv1.Challenge, len(challenges))
	for i, c := range challenges {
		status := "pending"
		if resp := cm.GetResponse(c.ID); resp != nil {
			status = "responded"
		}
		if time.Now().After(c.ExpiresAt) {
			status = "expired"
		}

		pbChallenges[i] = &airgapperv1.Challenge{
			Id:           c.ID,
			ChallengerId: c.OwnerKeyID,
			Status:       status,
			CreatedAt:    timestamppb.New(c.CreatedAt),
			ExpiresAt:    timestamppb.New(c.ExpiresAt),
		}
	}

	return connect.NewResponse(&airgapperv1.ListChallengesResponse{
		Challenges: pbChallenges,
	}), nil
}

func (v *verificationServer) RespondToChallenge(
	ctx context.Context,
	req *connect.Request[airgapperv1.RespondToChallengeRequest],
) (*connect.Response[airgapperv1.RespondToChallengeResponse], error) {
	cm := v.server.ChallengeManager()
	if cm == nil {
		return connect.NewResponse(&airgapperv1.RespondToChallengeResponse{
			Status:  "error",
			Message: "Challenge manager not configured",
		}), nil
	}

	_, err := cm.RespondToChallenge(req.Msg.Id)
	if err != nil {
		return connect.NewResponse(&airgapperv1.RespondToChallengeResponse{
			Status:  "error",
			Message: err.Error(),
		}), nil
	}

	return connect.NewResponse(&airgapperv1.RespondToChallengeResponse{
		Status:  "ok",
		Message: "Response recorded",
	}), nil
}

func (v *verificationServer) VerifyChallenge(
	ctx context.Context,
	req *connect.Request[airgapperv1.VerifyChallengeRequest],
) (*connect.Response[airgapperv1.VerifyChallengeResponse], error) {
	// This is typically called by the owner to verify a response
	// Would need owner to have the challenge and response
	return connect.NewResponse(&airgapperv1.VerifyChallengeResponse{
		Valid:   false,
		Status:  "not_implemented",
		Message: "Verification must be done by the owner",
	}), nil
}

// ============================================================================
// Deletion Tickets
// ============================================================================

func (v *verificationServer) CreateTicket(
	ctx context.Context,
	req *connect.Request[airgapperv1.CreateTicketRequest],
) (*connect.Response[airgapperv1.CreateTicketResponse], error) {
	// This is typically called by the owner to create a ticket
	// For now, return a placeholder - actual implementation would use owner keys
	return connect.NewResponse(&airgapperv1.CreateTicketResponse{
		Ticket: &airgapperv1.Ticket{
			Id: "",
		},
	}), nil
}

func (v *verificationServer) RegisterTicket(
	ctx context.Context,
	req *connect.Request[airgapperv1.RegisterTicketRequest],
) (*connect.Response[airgapperv1.RegisterTicketResponse], error) {
	tm := v.server.TicketManager()
	if tm == nil {
		return connect.NewResponse(&airgapperv1.RegisterTicketResponse{
			Status:  "error",
			Message: "Ticket manager not configured",
		}), nil
	}

	// Convert protobuf to internal type
	pbTicket := req.Msg.Ticket
	if pbTicket == nil {
		return connect.NewResponse(&airgapperv1.RegisterTicketResponse{
			Status:  "error",
			Message: "Ticket is required",
		}), nil
	}

	ticket := &verification.DeletionTicket{
		ID:             pbTicket.Id,
		OwnerKeyID:     pbTicket.IssuerId,
		Reason:         pbTicket.Purpose,
		CreatedAt:      pbTicket.IssuedAt.AsTime(),
		ExpiresAt:      pbTicket.ExpiresAt.AsTime(),
		OwnerSignature: pbTicket.Signature,
		Target: verification.TicketTarget{
			SnapshotIDs: pbTicket.SnapshotIds,
			Paths:       pbTicket.Paths,
		},
	}

	if len(pbTicket.SnapshotIds) > 0 {
		ticket.Target.Type = verification.TicketTargetSnapshot
	} else if len(pbTicket.Paths) > 0 {
		ticket.Target.Type = verification.TicketTargetFile
	}

	if err := tm.RegisterTicket(ticket); err != nil {
		return connect.NewResponse(&airgapperv1.RegisterTicketResponse{
			Status:  "error",
			Message: err.Error(),
		}), nil
	}

	return connect.NewResponse(&airgapperv1.RegisterTicketResponse{
		Status:  "ok",
		Message: "Ticket registered",
	}), nil
}

func (v *verificationServer) ListTickets(
	ctx context.Context,
	req *connect.Request[airgapperv1.ListTicketsRequest],
) (*connect.Response[airgapperv1.ListTicketsResponse], error) {
	tm := v.server.TicketManager()
	if tm == nil {
		return connect.NewResponse(&airgapperv1.ListTicketsResponse{
			Tickets: []*airgapperv1.Ticket{},
		}), nil
	}

	validOnly := req.Msg.StatusFilter == "active"
	tickets := tm.ListTickets(validOnly)

	pbTickets := make([]*airgapperv1.Ticket, len(tickets))
	for i, t := range tickets {
		pbTickets[i] = ticketToProto(t)
	}

	return connect.NewResponse(&airgapperv1.ListTicketsResponse{
		Tickets: pbTickets,
	}), nil
}

func (v *verificationServer) GetTicket(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetTicketRequest],
) (*connect.Response[airgapperv1.GetTicketResponse], error) {
	tm := v.server.TicketManager()
	if tm == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}

	ticket := tm.GetTicket(req.Msg.Id)
	if ticket == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}

	return connect.NewResponse(&airgapperv1.GetTicketResponse{
		Ticket: ticketToProto(ticket),
	}), nil
}

func (v *verificationServer) GetTicketUsage(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetTicketUsageRequest],
) (*connect.Response[airgapperv1.GetTicketUsageResponse], error) {
	tm := v.server.TicketManager()
	if tm == nil {
		return connect.NewResponse(&airgapperv1.GetTicketUsageResponse{
			Usages: []*airgapperv1.TicketUsage{},
		}), nil
	}

	usages := tm.GetUsageRecords(req.Msg.Id)

	pbUsages := make([]*airgapperv1.TicketUsage, len(usages))
	for i, u := range usages {
		pbUsages[i] = &airgapperv1.TicketUsage{
			TicketId: u.TicketID,
			UsedBy:   u.HostKeyID,
			Action:   "delete",
			UsedAt:   timestamppb.New(u.UsedAt),
		}
	}

	return connect.NewResponse(&airgapperv1.GetTicketUsageResponse{
		Usages: pbUsages,
	}), nil
}

// ticketToProto converts a DeletionTicket to protobuf format
func ticketToProto(t *verification.DeletionTicket) *airgapperv1.Ticket {
	return &airgapperv1.Ticket{
		Id:          t.ID,
		IssuerId:    t.OwnerKeyID,
		Purpose:     string(t.Target.Type),
		IssuedAt:    timestamppb.New(t.CreatedAt),
		ExpiresAt:   timestamppb.New(t.ExpiresAt),
		Signature:   t.OwnerSignature,
		SnapshotIds: t.Target.SnapshotIDs,
		Paths:       t.Target.Paths,
	}
}

// ============================================================================
// Witness Checkpoints
// ============================================================================

func (v *verificationServer) SubmitWitnessCheckpoint(
	ctx context.Context,
	req *connect.Request[airgapperv1.SubmitWitnessCheckpointRequest],
) (*connect.Response[airgapperv1.SubmitWitnessCheckpointResponse], error) {
	wm := v.server.WitnessManager()
	if wm == nil {
		return connect.NewResponse(&airgapperv1.SubmitWitnessCheckpointResponse{
			Status:  "error",
			Message: "Witness manager not configured",
		}), nil
	}

	pbCheckpoint := req.Msg.Checkpoint
	if pbCheckpoint == nil {
		return connect.NewResponse(&airgapperv1.SubmitWitnessCheckpointResponse{
			Status:  "error",
			Message: "Checkpoint is required",
		}), nil
	}

	checkpoint := &verification.WitnessCheckpoint{
		ID:             pbCheckpoint.Id,
		CreatedAt:      pbCheckpoint.CreatedAt.AsTime(),
		HostKeyID:      pbCheckpoint.WitnessId,
		HostSignature:  pbCheckpoint.Signature,
		SnapshotCount:  int(pbCheckpoint.SnapshotCount),
		TotalBytes:     pbCheckpoint.TotalBytes,
	}

	receipts, errs := wm.SubmitToAll(checkpoint)
	if len(errs) > 0 && len(receipts) == 0 {
		return connect.NewResponse(&airgapperv1.SubmitWitnessCheckpointResponse{
			Status:  "error",
			Message: errs[0].Error(),
		}), nil
	}

	return connect.NewResponse(&airgapperv1.SubmitWitnessCheckpointResponse{
		Status:  "ok",
		Message: "Checkpoint submitted",
	}), nil
}

func (v *verificationServer) CreateWitnessCheckpoint(
	ctx context.Context,
	req *connect.Request[airgapperv1.CreateWitnessCheckpointRequest],
) (*connect.Response[airgapperv1.CreateWitnessCheckpointResponse], error) {
	ac := v.server.AuditChain()

	var sequence uint64
	var hash string

	if ac != nil {
		sequence = ac.GetSequence()
		hash = ac.GetLatestHash()
	}

	checkpoint := &airgapperv1.WitnessCheckpoint{
		Id:             generateCheckpointID(),
		CreatedAt:      timestamppb.Now(),
		CheckpointHash: hash,
	}

	// Add audit chain info if available
	if ac != nil {
		checkpointData := map[string]interface{}{
			"sequence": sequence,
			"hash":     hash,
		}
		data, _ := json.Marshal(checkpointData)
		checkpoint.CheckpointHash = string(data)
	}

	return connect.NewResponse(&airgapperv1.CreateWitnessCheckpointResponse{
		Checkpoint: checkpoint,
	}), nil
}

func (v *verificationServer) GetWitnessCheckpoint(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetWitnessCheckpointRequest],
) (*connect.Response[airgapperv1.GetWitnessCheckpointResponse], error) {
	wm := v.server.WitnessManager()
	if wm == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}

	verifications, errs := wm.VerifyFromAll(req.Msg.Id)
	if len(errs) > 0 && len(verifications) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errs[0])
	}

	if len(verifications) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}

	v0 := verifications[0]
	checkpoint := &airgapperv1.WitnessCheckpoint{
		Id:          v0.CheckpointID,
		WitnessName: v0.WitnessName,
	}

	if v0.Checkpoint != nil {
		checkpoint.CreatedAt = timestamppb.New(v0.Checkpoint.CreatedAt)
		checkpoint.CheckpointHash = v0.Checkpoint.AuditChainHash
		checkpoint.SnapshotCount = int64(v0.Checkpoint.SnapshotCount)
		checkpoint.TotalBytes = v0.Checkpoint.TotalBytes
		checkpoint.Signature = v0.Checkpoint.HostSignature
	}

	return connect.NewResponse(&airgapperv1.GetWitnessCheckpointResponse{
		Checkpoint: checkpoint,
	}), nil
}

// generateCheckpointID creates a unique checkpoint ID
func generateCheckpointID() string {
	return "chk-" + time.Now().Format("20060102150405")
}
