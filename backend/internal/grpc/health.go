package grpc

import (
	"context"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
)

// healthServer implements the HealthService
type healthServer struct {
	airgapperv1connect.UnimplementedHealthServiceHandler
	server *Server
}

func newHealthServer(s *Server) airgapperv1connect.HealthServiceHandler {
	return &healthServer{server: s}
}

func (h *healthServer) Check(
	ctx context.Context,
	req *connect.Request[airgapperv1.CheckRequest],
) (*connect.Response[airgapperv1.CheckResponse], error) {
	return connect.NewResponse(&airgapperv1.CheckResponse{
		Status: "ok",
	}), nil
}

func (h *healthServer) GetStatus(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetStatusRequest],
) (*connect.Response[airgapperv1.GetStatusResponse], error) {
	// Get pending request count
	pendingCount := 0
	if pending, err := h.server.consentSvc.ListPendingRequests(); err == nil {
		pendingCount = len(pending)
	}

	status := h.server.statusSvc.GetSystemStatus(pendingCount)
	cfg := h.server.cfg

	resp := &airgapperv1.GetStatusResponse{
		Name:            status.Name,
		Role:            toProtoRole(cfg.Role),
		RepoUrl:         status.RepoURL,
		HasShare:        status.HasShare,
		ShareIndex:      int32(status.ShareIndex),
		PendingRequests: int32(status.PendingRequests),
		BackupPaths:     status.BackupPaths,
		Mode:            toProtoOperationMode(cfg),
		Consensus:       toProtoConsensusInfo(cfg.Consensus),
	}

	// Add peer info if available
	if status.Peer != nil {
		resp.Peer = &airgapperv1.Peer{
			Name:    status.Peer.Name,
			Address: status.Peer.Address,
		}
	}

	// Add scheduler info
	if status.Scheduler != nil {
		resp.Scheduler = &airgapperv1.SchedulerInfo{
			Enabled:   status.Scheduler.Enabled,
			Schedule:  status.Scheduler.Schedule,
			Paths:     status.Scheduler.Paths,
			LastError: status.Scheduler.LastError,
		}
	}

	return connect.NewResponse(resp), nil
}
