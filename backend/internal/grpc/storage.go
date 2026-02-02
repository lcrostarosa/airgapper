package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
)

// storageServer implements the StorageService
type storageServer struct {
	airgapperv1connect.UnimplementedStorageServiceHandler
	server *Server
}

func newStorageServer(s *Server) airgapperv1connect.StorageServiceHandler {
	return &storageServer{server: s}
}

func (s *storageServer) GetStorageStatus(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetStorageStatusRequest],
) (*connect.Response[airgapperv1.GetStorageStatusResponse], error) {
	status := s.server.hostSvc.GetStorageStatus()

	return connect.NewResponse(&airgapperv1.GetStorageStatusResponse{
		Configured:      status.Configured,
		Running:         status.Running,
		BasePath:        status.BasePath,
		AppendOnly:      status.AppendOnly,
		QuotaBytes:      status.QuotaBytes,
		UsedBytes:       status.UsedBytes,
		RequestCount:    status.RequestCount,
		HasPolicy:       status.HasPolicy,
		PolicyId:        status.PolicyID,
		DiskUsagePct:    int32(status.DiskUsagePct),
		DiskFreeBytes:   status.DiskFreeBytes,
		DiskTotalBytes:  status.DiskTotalBytes,
	}), nil
}

func (s *storageServer) StartStorage(
	ctx context.Context,
	req *connect.Request[airgapperv1.StartStorageRequest],
) (*connect.Response[airgapperv1.StartStorageResponse], error) {
	if err := s.server.hostSvc.StartStorage(); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage server not configured"))
	}

	return connect.NewResponse(&airgapperv1.StartStorageResponse{
		Status: "started",
	}), nil
}

func (s *storageServer) StopStorage(
	ctx context.Context,
	req *connect.Request[airgapperv1.StopStorageRequest],
) (*connect.Response[airgapperv1.StopStorageResponse], error) {
	if err := s.server.hostSvc.StopStorage(); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage server not configured"))
	}

	return connect.NewResponse(&airgapperv1.StopStorageResponse{
		Status: "stopped",
	}), nil
}
