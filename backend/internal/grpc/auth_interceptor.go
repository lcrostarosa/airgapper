package grpc

import (
	"context"
	"crypto/subtle"
	"strings"

	"connectrpc.com/connect"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// APIKey is the required API key for authentication
	APIKey string
	// DevMode disables authentication when true (for development only)
	DevMode bool
}

// authInterceptor validates API key authentication
type authInterceptor struct {
	config *AuthConfig
}

// newAuthInterceptor creates a new authentication interceptor
func newAuthInterceptor(config *AuthConfig) connect.Interceptor {
	return &authInterceptor{config: config}
}

// healthCheckProcedures are exempt from authentication
var healthCheckProcedures = map[string]bool{
	"/airgapper.v1.HealthService/Check": true,
}

func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Skip auth in dev mode
		if i.config.DevMode {
			return next(ctx, req)
		}

		// Skip auth for health checks
		if healthCheckProcedures[req.Spec().Procedure] {
			return next(ctx, req)
		}

		// If no API key configured, deny all requests
		if i.config.APIKey == "" {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				nil,
			)
		}

		// Get API key from header
		apiKey := req.Header().Get("X-API-Key")
		if apiKey == "" {
			// Also check Authorization header with Bearer scheme
			authHeader := req.Header().Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(i.config.APIKey)) != 1 {
			return nil, connect.NewError(
				connect.CodeUnauthenticated,
				nil,
			)
		}

		return next(ctx, req)
	}
}

func (i *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // No streaming RPCs in our API
}

func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		// Skip auth in dev mode
		if i.config.DevMode {
			return next(ctx, conn)
		}

		// If no API key configured, deny all requests
		if i.config.APIKey == "" {
			return connect.NewError(
				connect.CodeUnauthenticated,
				nil,
			)
		}

		// Get API key from header
		apiKey := conn.RequestHeader().Get("X-API-Key")
		if apiKey == "" {
			authHeader := conn.RequestHeader().Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// Constant-time comparison
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(i.config.APIKey)) != 1 {
			return connect.NewError(
				connect.CodeUnauthenticated,
				nil,
			)
		}

		return next(ctx, conn)
	}
}
