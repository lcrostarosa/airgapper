package grpc

import (
	"context"
	"encoding/hex"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// deletionsServer implements the DeletionService
type deletionsServer struct {
	airgapperv1connect.UnimplementedDeletionServiceHandler
	server *Server
}

func newDeletionsServer(s *Server) airgapperv1connect.DeletionServiceHandler {
	return &deletionsServer{server: s}
}

func (d *deletionsServer) ListDeletions(
	ctx context.Context,
	req *connect.Request[airgapperv1.ListDeletionsRequest],
) (*connect.Response[airgapperv1.ListDeletionsResponse], error) {
	deletions, err := d.server.consentSvc.ListPendingDeletions()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.ListDeletionsResponse{
		Deletions: toProtoDeletionRequests(deletions),
	}), nil
}

func (d *deletionsServer) GetDeletion(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetDeletionRequest],
) (*connect.Response[airgapperv1.GetDeletionResponse], error) {
	deletion, err := d.server.consentSvc.GetDeletionRequest(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&airgapperv1.GetDeletionResponse{
		Deletion: toProtoDeletionRequest(deletion),
	}), nil
}

func (d *deletionsServer) CreateDeletion(
	ctx context.Context,
	req *connect.Request[airgapperv1.CreateDeletionRequest],
) (*connect.Response[airgapperv1.CreateDeletionResponse], error) {
	params := service.CreateDeletionRequestParams{
		DeletionType:      fromProtoDeletionType(req.Msg.DeletionType),
		SnapshotIDs:       req.Msg.SnapshotIds,
		Paths:             req.Msg.Paths,
		Reason:            req.Msg.Reason,
		RequiredApprovals: int(req.Msg.RequiredApprovals),
	}

	deletion, err := d.server.consentSvc.CreateDeletionRequest(params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.CreateDeletionResponse{
		Id:        deletion.ID,
		Status:    string(deletion.Status),
		ExpiresAt: timeToTimestamp(deletion.ExpiresAt),
	}), nil
}

func (d *deletionsServer) ApproveDeletion(
	ctx context.Context,
	req *connect.Request[airgapperv1.ApproveDeletionRequest],
) (*connect.Response[airgapperv1.ApproveDeletionResponse], error) {
	// Decode hex signature
	signature, err := hex.DecodeString(req.Msg.Signature)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	progress, err := d.server.consentSvc.ApproveDeletion(
		req.Msg.Id,
		req.Msg.KeyHolderId,
		signature,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.ApproveDeletionResponse{
		Status:            "ok",
		CurrentApprovals:  int32(progress.Current),
		RequiredApprovals: int32(progress.Required),
		IsApproved:        progress.IsApproved,
	}), nil
}

func (d *deletionsServer) DenyDeletion(
	ctx context.Context,
	req *connect.Request[airgapperv1.DenyDeletionRequest],
) (*connect.Response[airgapperv1.DenyDeletionResponse], error) {
	err := d.server.consentSvc.DenyDeletion(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.DenyDeletionResponse{
		Status: "denied",
	}), nil
}
