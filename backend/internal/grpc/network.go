package grpc

import (
	"context"
	"net"

	"connectrpc.com/connect"

	airgapperv1 "github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1"
	"github.com/lcrostarosa/airgapper/backend/gen/airgapper/v1/airgapperv1connect"
)

// networkServer implements the NetworkService
type networkServer struct {
	airgapperv1connect.UnimplementedNetworkServiceHandler
	server *Server
}

func newNetworkServer(s *Server) airgapperv1connect.NetworkServiceHandler {
	return &networkServer{server: s}
}

func (n *networkServer) GetLocalIP(
	ctx context.Context,
	req *connect.Request[airgapperv1.GetLocalIPRequest],
) (*connect.Response[airgapperv1.GetLocalIPResponse], error) {
	ip := getOutboundIP()
	return connect.NewResponse(&airgapperv1.GetLocalIPResponse{
		Ip: ip,
	}), nil
}

// getOutboundIP returns the preferred outbound IP of this machine
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer func() { _ = conn.Close() }()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
