// Package errors provides sentinel errors for the airgapper application.
package errors

import "errors"

// Configuration errors
var (
	// ErrNotInitialized is returned when airgapper has not been initialized.
	ErrNotInitialized = errors.New("airgapper not initialized")

	// ErrNoLocalShare is returned when no local key share is found.
	ErrNoLocalShare = errors.New("no local share found")

	// ErrConsensusNotConfigured is returned when consensus is required but not configured.
	ErrConsensusNotConfigured = errors.New("consensus not configured")
)

// Key holder errors
var (
	// ErrKeyHolderExists is returned when trying to add a key holder that already exists.
	ErrKeyHolderExists = errors.New("key holder already registered")

	// ErrKeyHolderNotFound is returned when a key holder is not found.
	ErrKeyHolderNotFound = errors.New("key holder not found")
)

// Request errors
var (
	// ErrRequestNotFound is returned when a request (restore or deletion) is not found.
	ErrRequestNotFound = errors.New("request not found")

	// ErrRequestNotPending is returned when an operation requires a pending request.
	ErrRequestNotPending = errors.New("request is not pending")

	// ErrRequestExpired is returned when a request has expired.
	ErrRequestExpired = errors.New("request has expired")

	// ErrRequestNotApproved is returned when an operation requires an approved request.
	ErrRequestNotApproved = errors.New("request is not approved")

	// ErrAlreadyApproved is returned when a key holder has already approved a request.
	ErrAlreadyApproved = errors.New("key holder already approved this request")

	// ErrInsufficientApprovals is returned when there aren't enough approvals.
	ErrInsufficientApprovals = errors.New("insufficient approvals")
)

// Role errors
var (
	// ErrInvalidRole is returned when an operation is attempted with an invalid role.
	ErrInvalidRole = errors.New("invalid role for this operation")
)
