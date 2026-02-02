// Package runner provides an interceptor-based command execution framework for CLI commands.
// It mirrors the pattern used by Connect-RPC interceptors, providing consistent middleware
// semantics for CLI command handlers.
package runner

import "errors"

// Standard errors returned by interceptors
var (
	// ErrNotInitialized is returned when airgapper is not initialized
	ErrNotInitialized = errors.New("airgapper not initialized - run 'airgapper init' first")

	// ErrNotOwner is returned when a command requires owner role but caller is not owner
	ErrNotOwner = errors.New("only the data owner can run this command")

	// ErrNotHost is returned when a command requires host role but caller is not host
	ErrNotHost = errors.New("only the backup host can run this command")

	// ErrNoPassword is returned when password is required but not available
	ErrNoPassword = errors.New("no password found - this config may be corrupted")

	// ErrNoPrivateKey is returned when private key is required but not available
	ErrNoPrivateKey = errors.New("no private key found - cannot sign")

	// ErrNoShare is returned when a key share is required but not available
	ErrNoShare = errors.New("no key share found")
)
