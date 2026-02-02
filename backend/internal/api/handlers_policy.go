package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lcrostarosa/airgapper/backend/internal/policy"
)

// handleGetPolicy returns the current policy
func (s *Server) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}

	p := s.storageServer.GetPolicy()
	if p == nil {
		jsonResponse(w, http.StatusOK, PolicyStatusDTO{
			HasPolicy: false,
		})
		return
	}

	jsonResponse(w, http.StatusOK, PolicyResponseDTO{
		HasPolicy:     true,
		Policy:        p,
		IsFullySigned: p.IsFullySigned(),
		IsActive:      p.IsActive(),
	})
}

// handleCreatePolicy creates a new policy
func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}

	var body CreatePolicyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if body.OwnerName == "" || body.OwnerKeyID == "" || body.OwnerPubKey == "" {
		jsonError(w, http.StatusBadRequest, "owner information required")
		return
	}
	if body.HostName == "" || body.HostKeyID == "" || body.HostPubKey == "" {
		jsonError(w, http.StatusBadRequest, "host information required")
		return
	}

	// Create policy
	p := policy.NewPolicy(
		body.OwnerName, body.OwnerKeyID, body.OwnerPubKey,
		body.HostName, body.HostKeyID, body.HostPubKey,
	)

	// Set terms
	if body.RetentionDays > 0 {
		p.RetentionDays = body.RetentionDays
	}
	if body.DeletionMode != "" {
		p.DeletionMode = policy.DeletionMode(body.DeletionMode)
	}
	if body.MaxStorageBytes > 0 {
		p.MaxStorageBytes = body.MaxStorageBytes
	}

	// Apply signatures if provided
	if body.OwnerSignature != "" {
		p.OwnerSignature = body.OwnerSignature
	}
	if body.HostSignature != "" {
		p.HostSignature = body.HostSignature
	}

	// If both signatures present, set the policy
	if p.IsFullySigned() {
		if err := s.storageServer.SetPolicy(p); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Return policy data for signing
	policyJSON, _ := p.ToJSON()
	jsonResponse(w, http.StatusCreated, PolicyResponseDTO{
		HasPolicy:     true,
		Policy:        p,
		PolicyJSON:    string(policyJSON),
		IsFullySigned: p.IsFullySigned(),
	})
}

// handlePolicySign handles signing a policy
func (s *Server) handlePolicySign(w http.ResponseWriter, r *http.Request) {
	if !s.requireStorageServer(w) {
		return
	}

	var body PolicySignBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Parse the policy
	p, err := policy.FromJSON([]byte(body.PolicyJSON))
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid policy JSON: %v", err))
		return
	}

	// Apply signature
	switch body.SignerRole {
	case "owner":
		p.OwnerSignature = body.Signature
	case "host":
		p.HostSignature = body.Signature
	default:
		jsonError(w, http.StatusBadRequest, "signerRole must be 'owner' or 'host'")
		return
	}

	// If both signatures present, verify and set
	if p.IsFullySigned() {
		if err := p.Verify(); err != nil {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("signature verification failed: %v", err))
			return
		}
		if err := s.storageServer.SetPolicy(p); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	policyJSON, _ := p.ToJSON()
	jsonResponse(w, http.StatusOK, PolicyResponseDTO{
		HasPolicy:     true,
		Policy:        p,
		PolicyJSON:    string(policyJSON),
		IsFullySigned: p.IsFullySigned(),
	})
}
