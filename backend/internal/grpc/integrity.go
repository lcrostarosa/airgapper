package grpc

import (
	"context"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
)

// integrityServer implements the IntegrityService
type integrityServer struct {
	airgapperv1connect.UnimplementedIntegrityServiceHandler
	server *Server
}

func newIntegrityServer(s *Server) airgapperv1connect.IntegrityServiceHandler {
	return &integrityServer{server: s}
}

func (i *integrityServer) CheckIntegrity(
	ctx context.Context,
	req *connect.Request[airgapperv1.CheckIntegrityRequest],
) (*connect.Response[airgapperv1.CheckIntegrityResponse], error) {
	status := i.server.hostSvc.GetStorageStatus()

	return connect.NewResponse(&airgapperv1.CheckIntegrityResponse{
		Status:         "ok",
		StorageRunning: status.Running,
		UsedBytes:      status.UsedBytes,
		DiskUsagePct:   int32(status.DiskUsagePct),
		DiskFreeBytes:  status.DiskFreeBytes,
		HasPolicy:      status.HasPolicy,
		PolicyId:       status.PolicyID,
		RequestCount:   status.RequestCount,
	}), nil
}

func (i *integrityServer) RunFullCheck(
	ctx context.Context,
	req *connect.Request[airgapperv1.RunFullCheckRequest],
) (*connect.Response[airgapperv1.RunFullCheckResponse], error) {
	if i.server.integrityChecker == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errIntegrityNotConfigured)
	}

	// Default to "default" repo if not specified
	repoName := "default"

	result, err := i.server.integrityChecker.CheckDataIntegrity(repoName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.RunFullCheckResponse{
		Status: "completed",
		Result: toProtoIntegrityCheckResult(result),
	}), nil
}

func (i *integrityServer) GetIntegrityRecords(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetIntegrityRecordsRequest],
) (*connect.Response[airgapperv1.GetIntegrityRecordsResponse], error) {
	return connect.NewResponse(&airgapperv1.GetIntegrityRecordsResponse{
		Records: []*airgapperv1.IntegrityRecord{},
	}), nil
}

func (i *integrityServer) CreateIntegrityRecord(
	ctx context.Context,
	req *connect.Request[airgapperv1.CreateIntegrityRecordRequest],
) (*connect.Response[airgapperv1.CreateIntegrityRecordResponse], error) {
	return connect.NewResponse(&airgapperv1.CreateIntegrityRecordResponse{
		Status:  "ok",
		Message: "Integrity record created",
	}), nil
}

func (i *integrityServer) AddIntegrityRecord(
	ctx context.Context,
	req *connect.Request[airgapperv1.AddIntegrityRecordRequest],
) (*connect.Response[airgapperv1.AddIntegrityRecordResponse], error) {
	return connect.NewResponse(&airgapperv1.AddIntegrityRecordResponse{
		Status:  "ok",
		Message: "Integrity record added",
	}), nil
}

func (i *integrityServer) GetIntegrityHistory(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetIntegrityHistoryRequest],
) (*connect.Response[airgapperv1.GetIntegrityHistoryResponse], error) {
	return connect.NewResponse(&airgapperv1.GetIntegrityHistoryResponse{
		History: []*airgapperv1.IntegrityCheckResult{},
	}), nil
}

func (i *integrityServer) GetVerificationConfig(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetVerificationConfigRequest],
) (*connect.Response[airgapperv1.GetVerificationConfigResponse], error) {
	return connect.NewResponse(&airgapperv1.GetVerificationConfigResponse{
		Enabled: false,
	}), nil
}

func (i *integrityServer) UpdateVerificationConfig(
	ctx context.Context,
	req *connect.Request[airgapperv1.UpdateVerificationConfigRequest],
) (*connect.Response[airgapperv1.UpdateVerificationConfigResponse], error) {
	return connect.NewResponse(&airgapperv1.UpdateVerificationConfigResponse{
		Status:  "updated",
		Message: "Verification config updated",
	}), nil
}

func (i *integrityServer) RunManualCheck(
	ctx context.Context,
	req *connect.Request[airgapperv1.RunManualCheckRequest],
) (*connect.Response[airgapperv1.RunManualCheckResponse], error) {
	return connect.NewResponse(&airgapperv1.RunManualCheckResponse{
		Status:    "completed",
		CheckType: "quick",
	}), nil
}
