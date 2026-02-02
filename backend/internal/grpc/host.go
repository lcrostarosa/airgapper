package grpc

import (
	"context"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// hostServer implements the HostService
type hostServer struct {
	airgapperv1connect.UnimplementedHostServiceHandler
	server *Server
}

func newHostServer(s *Server) airgapperv1connect.HostServiceHandler {
	return &hostServer{server: s}
}

func (h *hostServer) InitHost(
	ctx context.Context,
	req *connect.Request[airgapperv1.InitHostRequest],
) (*connect.Response[airgapperv1.InitHostResponse], error) {
	params := service.HostInitParams{
		Name:         req.Msg.Name,
		StoragePath:  req.Msg.StoragePath,
		StorageQuota: req.Msg.StorageQuotaBytes,
		AppendOnly:   req.Msg.AppendOnly,
	}

	result, err := h.server.hostSvc.Init(params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.InitHostResponse{
		Name:        result.Name,
		KeyId:       result.KeyID,
		PublicKey:   result.PublicKey,
		StoragePath: result.StoragePath,
	}), nil
}

func (h *hostServer) ReceiveShare(
	ctx context.Context,
	req *connect.Request[airgapperv1.ReceiveShareRequest],
) (*connect.Response[airgapperv1.ReceiveShareResponse], error) {
	err := h.server.hostSvc.ReceiveShare(
		req.Msg.Share,
		byte(req.Msg.ShareIndex),
		req.Msg.RepoUrl,
		req.Msg.PeerName,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.ReceiveShareResponse{
		Status:  "ok",
		Message: "Share received successfully",
	}), nil
}

func (h *hostServer) ListSnapshots(
	ctx context.Context,
	req *connect.Request[airgapperv1.ListSnapshotsRequest],
) (*connect.Response[airgapperv1.ListSnapshotsResponse], error) {
	// TODO: Implement snapshot listing via restic
	return connect.NewResponse(&airgapperv1.ListSnapshotsResponse{
		Snapshots: []*airgapperv1.Snapshot{},
	}), nil
}
