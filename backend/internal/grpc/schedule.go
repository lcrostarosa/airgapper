package grpc

import (
	"context"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
)

// scheduleServer implements the ScheduleService
type scheduleServer struct {
	airgapperv1connect.UnimplementedScheduleServiceHandler
	server *Server
}

func newScheduleServer(s *Server) airgapperv1connect.ScheduleServiceHandler {
	return &scheduleServer{server: s}
}

func (s *scheduleServer) GetSchedule(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetScheduleRequest],
) (*connect.Response[airgapperv1.GetScheduleResponse], error) {
	info := s.server.statusSvc.GetScheduleInfo()

	return connect.NewResponse(&airgapperv1.GetScheduleResponse{
		Schedule:  info.Schedule,
		Paths:     info.Paths,
		Enabled:   info.Enabled,
		LastError: info.LastError,
	}), nil
}

func (s *scheduleServer) UpdateSchedule(
	ctx context.Context,
	req *connect.Request[airgapperv1.UpdateScheduleRequest],
) (*connect.Response[airgapperv1.UpdateScheduleResponse], error) {
	// Update config
	if err := s.server.statusSvc.UpdateSchedule(req.Msg.Schedule, req.Msg.Paths); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.UpdateScheduleResponse{
		Status:  "updated",
		Message: "Schedule updated successfully",
	}), nil
}

func (s *scheduleServer) GetBackupHistory(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetBackupHistoryRequest],
) (*connect.Response[airgapperv1.GetBackupHistoryResponse], error) {
	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 50 // Default limit
	}

	results := s.server.statusSvc.GetBackupHistory(limit)

	protoResults := make([]*airgapperv1.BackupResult, len(results))
	for i, r := range results {
		protoResults[i] = &airgapperv1.BackupResult{
			ScheduledTime: timeToTimestamp(r.ScheduledTime),
			StartTime:     timeToTimestamp(r.StartTime),
			EndTime:       timeToTimestamp(r.EndTime),
			DurationMs:    r.Duration().Milliseconds(),
			Success:       r.Success,
			Attempt:       int32(r.Attempt),
			IsRetry:       r.IsRetry(),
		}
		if r.Error != nil {
			protoResults[i].Error = r.Error.Error()
		}
	}

	return connect.NewResponse(&airgapperv1.GetBackupHistoryResponse{
		History: protoResults,
		Count:   int32(len(protoResults)),
	}), nil
}
