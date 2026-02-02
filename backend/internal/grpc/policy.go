package grpc

import (
	"context"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
)

// policyServer implements the PolicyService
type policyServer struct {
	airgapperv1connect.UnimplementedPolicyServiceHandler
	server *Server
}

func newPolicyServer(s *Server) airgapperv1connect.PolicyServiceHandler {
	return &policyServer{server: s}
}

func (p *policyServer) GetPolicy(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetPolicyRequest],
) (*connect.Response[airgapperv1.GetPolicyResponse], error) {
	// TODO: Implement once policy service is available
	return connect.NewResponse(&airgapperv1.GetPolicyResponse{
		HasPolicy: false,
	}), nil
}

func (p *policyServer) CreatePolicy(
	ctx context.Context,
	req *connect.Request[airgapperv1.CreatePolicyRequest],
) (*connect.Response[airgapperv1.CreatePolicyResponse], error) {
	// TODO: Implement once policy service is available
	return connect.NewResponse(&airgapperv1.CreatePolicyResponse{
		IsFullySigned: false,
	}), nil
}

func (p *policyServer) SignPolicy(
	ctx context.Context,
	req *connect.Request[airgapperv1.SignPolicyRequest],
) (*connect.Response[airgapperv1.SignPolicyResponse], error) {
	// TODO: Implement once policy service is available
	return connect.NewResponse(&airgapperv1.SignPolicyResponse{
		IsFullySigned: false,
	}), nil
}
