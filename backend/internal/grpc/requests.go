package grpc

import (
	"context"
	"encoding/hex"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// requestsServer implements the RestoreRequestService
type requestsServer struct {
	airgapperv1connect.UnimplementedRestoreRequestServiceHandler
	server *Server
}

func newRequestsServer(s *Server) airgapperv1connect.RestoreRequestServiceHandler {
	return &requestsServer{server: s}
}

func (r *requestsServer) ListRequests(
	ctx context.Context,
	req *connect.Request[airgapperv1.ListRequestsRequest],
) (*connect.Response[airgapperv1.ListRequestsResponse], error) {
	requests, err := r.server.consentSvc.ListPendingRequests()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.ListRequestsResponse{
		Requests: toProtoRestoreRequests(requests),
	}), nil
}

func (r *requestsServer) GetRequest(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetRequestRequest],
) (*connect.Response[airgapperv1.GetRequestResponse], error) {
	request, err := r.server.consentSvc.GetRequest(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&airgapperv1.GetRequestResponse{
		Request: toProtoRestoreRequest(request),
	}), nil
}

func (r *requestsServer) CreateRequest(
	ctx context.Context,
	req *connect.Request[airgapperv1.CreateRequestRequest],
) (*connect.Response[airgapperv1.CreateRequestResponse], error) {
	params := service.CreateRestoreRequestParams{
		SnapshotID: req.Msg.SnapshotId,
		Paths:      req.Msg.Paths,
		Reason:     req.Msg.Reason,
	}

	request, err := r.server.consentSvc.CreateRestoreRequest(params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.CreateRequestResponse{
		Id:        request.ID,
		Status:    string(request.Status),
		ExpiresAt: timeToTimestamp(request.ExpiresAt),
	}), nil
}

func (r *requestsServer) ApproveRequest(
	ctx context.Context,
	req *connect.Request[airgapperv1.ApproveRequestRequest],
) (*connect.Response[airgapperv1.ApproveRequestResponse], error) {
	err := r.server.consentSvc.ApproveRequest(req.Msg.Id, req.Msg.Share)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.ApproveRequestResponse{
		Status:  "approved",
		Message: "Request approved successfully",
	}), nil
}

func (r *requestsServer) SignRequest(
	ctx context.Context,
	req *connect.Request[airgapperv1.SignRequestRequest],
) (*connect.Response[airgapperv1.SignRequestResponse], error) {
	// Decode hex signature
	signature, err := hex.DecodeString(req.Msg.Signature)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	params := service.SignRequestParams{
		RequestID:   req.Msg.Id,
		KeyHolderID: req.Msg.KeyHolderId,
		Signature:   signature,
	}

	progress, err := r.server.consentSvc.SignRequest(params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.SignRequestResponse{
		Status:            "ok",
		CurrentApprovals:  int32(progress.Current),
		RequiredApprovals: int32(progress.Required),
		IsApproved:        progress.IsApproved,
	}), nil
}

func (r *requestsServer) DenyRequest(
	ctx context.Context,
	req *connect.Request[airgapperv1.DenyRequestRequest],
) (*connect.Response[airgapperv1.DenyRequestResponse], error) {
	err := r.server.consentSvc.DenyRequest(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.DenyRequestResponse{
		Status: "denied",
	}), nil
}
