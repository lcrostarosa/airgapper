package grpc

import (
	"context"

	"connectrpc.com/connect"

	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// loggingInterceptor logs RPC calls
type loggingInterceptor struct{}

func newLoggingInterceptor() connect.Interceptor {
	return &loggingInterceptor{}
}

func (i *loggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		logging.Debugf("gRPC call: %s", req.Spec().Procedure)
		resp, err := next(ctx, req)
		if err != nil {
			logging.Debugf("gRPC error: %s: %v", req.Spec().Procedure, err)
		}
		return resp, err
	}
}

func (i *loggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // No streaming RPCs in our API
}

func (i *loggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next // No streaming RPCs in our API
}
