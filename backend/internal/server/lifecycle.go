// Package server provides HTTP server utilities
package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// ShutdownTimeout is the default timeout for graceful shutdown
const ShutdownTimeout = 5 * time.Second

// GracefulServer wraps an http.Server with graceful shutdown capabilities
type GracefulServer struct {
	server       *http.Server
	beforeStop   func()
	shutdownHook func()
	tlsCertFile  string
	tlsKeyFile   string
}

// GracefulServerOptions configures a GracefulServer
type GracefulServerOptions struct {
	// BeforeStop is called before initiating shutdown (e.g., stop scheduler)
	BeforeStop func()
	// ShutdownHook is called after server shutdown completes
	ShutdownHook func()
	// TLSCertFile is the path to the TLS certificate file (enables HTTPS if set)
	TLSCertFile string
	// TLSKeyFile is the path to the TLS private key file
	TLSKeyFile string
}

// NewGracefulServer creates a server wrapper with graceful shutdown
func NewGracefulServer(server *http.Server, opts *GracefulServerOptions) *GracefulServer {
	gs := &GracefulServer{server: server}
	if opts != nil {
		gs.beforeStop = opts.BeforeStop
		gs.shutdownHook = opts.ShutdownHook
		gs.tlsCertFile = opts.TLSCertFile
		gs.tlsKeyFile = opts.TLSKeyFile
	}
	return gs
}

// ListenAndServe starts the server and handles graceful shutdown on SIGINT/SIGTERM.
// This is a blocking call that returns when the server has been shut down.
// If TLS is configured, the server will use HTTPS.
func (gs *GracefulServer) ListenAndServe() error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		var err error
		if gs.tlsCertFile != "" && gs.tlsKeyFile != "" {
			logging.Info("Starting HTTPS server with TLS")
			err = gs.server.ListenAndServeTLS(gs.tlsCertFile, gs.tlsKeyFile)
		} else {
			err = gs.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		logging.Error("Server error", logging.Err(err))
		return err
	case <-stop:
		return gs.Shutdown()
	}
}

// ListenAndServeTLS starts the server with TLS and handles graceful shutdown.
func (gs *GracefulServer) ListenAndServeTLS(certFile, keyFile string) error {
	gs.tlsCertFile = certFile
	gs.tlsKeyFile = keyFile
	return gs.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (gs *GracefulServer) Shutdown() error {
	logging.Info("Shutting down...")

	if gs.beforeStop != nil {
		gs.beforeStop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	if err := gs.server.Shutdown(ctx); err != nil {
		return err
	}

	if gs.shutdownHook != nil {
		gs.shutdownHook()
	}

	logging.Info("Server stopped")
	return nil
}

// RunWithGracefulShutdown starts an HTTP server and handles shutdown signals.
// beforeStop is called before shutdown begins (can be nil).
// This is a convenience function for simple use cases.
func RunWithGracefulShutdown(server *http.Server, beforeStop func()) error {
	gs := NewGracefulServer(server, &GracefulServerOptions{
		BeforeStop: beforeStop,
	})
	return gs.ListenAndServe()
}

// RunWithGracefulShutdownTLS starts an HTTPS server with TLS and handles shutdown signals.
func RunWithGracefulShutdownTLS(server *http.Server, certFile, keyFile string, beforeStop func()) error {
	gs := NewGracefulServer(server, &GracefulServerOptions{
		BeforeStop:  beforeStop,
		TLSCertFile: certFile,
		TLSKeyFile:  keyFile,
	})
	return gs.ListenAndServe()
}
