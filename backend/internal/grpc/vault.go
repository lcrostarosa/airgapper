package grpc

import (
	"context"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
	"github.com/lcrostarosa/airgapper/backend/internal/service"
)

// vaultServer implements the VaultService
type vaultServer struct {
	airgapperv1connect.UnimplementedVaultServiceHandler
	server *Server
}

func newVaultServer(s *Server) airgapperv1connect.VaultServiceHandler {
	return &vaultServer{server: s}
}

func (v *vaultServer) InitVault(
	ctx context.Context,
	req *connect.Request[airgapperv1.InitVaultRequest],
) (*connect.Response[airgapperv1.InitVaultResponse], error) {
	params := service.InitParams{
		Name:            req.Msg.Name,
		RepoURL:         req.Msg.RepoUrl,
		Threshold:       int(req.Msg.Threshold),
		TotalKeys:       int(req.Msg.TotalKeys),
		BackupPaths:     req.Msg.BackupPaths,
		RequireApproval: req.Msg.RequireApproval,
	}

	result, err := v.server.vaultSvc.Init(params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&airgapperv1.InitVaultResponse{
		Name:      result.Name,
		KeyId:     result.KeyID,
		PublicKey: result.PublicKey,
		Threshold: int32(result.Threshold),
		TotalKeys: int32(result.TotalKeys),
	}), nil
}
