package grpc

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// keyHoldersServer implements the KeyHolderService
type keyHoldersServer struct {
	airgapperv1connect.UnimplementedKeyHolderServiceHandler
	server *Server
}

func newKeyHoldersServer(s *Server) airgapperv1connect.KeyHolderServiceHandler {
	return &keyHoldersServer{server: s}
}

func (k *keyHoldersServer) ListKeyHolders(
	ctx context.Context,
	req *connect.Request[airgapperv1.ListKeyHoldersRequest],
) (*connect.Response[airgapperv1.ListKeyHoldersResponse], error) {
	info, err := k.server.vaultSvc.GetConsensusInfo()
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	return connect.NewResponse(&airgapperv1.ListKeyHoldersResponse{
		Consensus: &airgapperv1.ConsensusInfo{
			Threshold:       int32(info.Threshold),
			TotalKeys:       int32(info.TotalKeys),
			KeyHolders:      toProtoKeyHolders(info.KeyHolders),
			RequireApproval: info.RequireApproval,
		},
	}), nil
}

func (k *keyHoldersServer) GetKeyHolder(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetKeyHolderRequest],
) (*connect.Response[airgapperv1.GetKeyHolderResponse], error) {
	holder, err := k.server.vaultSvc.GetKeyHolder(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&airgapperv1.GetKeyHolderResponse{
		KeyHolder: toProtoKeyHolder(holder),
	}), nil
}

func (k *keyHoldersServer) RegisterKeyHolder(
	ctx context.Context,
	req *connect.Request[airgapperv1.RegisterKeyHolderRequest],
) (*connect.Response[airgapperv1.RegisterKeyHolderResponse], error) {
	params := service.RegisterKeyHolderParams{
		Name:      req.Msg.Name,
		PublicKey: req.Msg.PublicKey,
		Address:   req.Msg.Address,
	}

	result, err := k.server.vaultSvc.RegisterKeyHolder(params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.RegisterKeyHolderResponse{
		Id:       result.ID,
		Name:     result.Name,
		JoinedAt: timestamppb.New(result.JoinedAt),
	}), nil
}
