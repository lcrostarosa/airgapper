package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
)

// Validator interface for request body validation
type Validator interface {
	Validate() error
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

// decodeAndValidate decodes JSON body and validates it
func decodeAndValidate(r *http.Request, v Validator) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid request body")
	}
	return v.Validate()
}

// required returns an error if value is empty
func required(field, value string) error {
	if value == "" {
		return ValidationError{Field: field, Message: field + " is required"}
	}
	return nil
}

// requiredInt returns an error if value is < min
func requiredInt(field string, value, min int) error {
	if value < min {
		return ValidationError{Field: field, Message: fmt.Sprintf("%s must be at least %d", field, min)}
	}
	return nil
}

// requireMethod checks HTTP method and returns false if wrong
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

// requireStorageServer checks storage server exists
func (s *Server) requireStorageServer(w http.ResponseWriter) bool {
	if s.storageServer == nil {
		jsonError(w, http.StatusBadRequest, "storage server not configured")
		return false
	}
	return true
}

// requireIntegrityChecker checks integrity checker exists
func (s *Server) requireIntegrityChecker(w http.ResponseWriter) bool {
	if s.integrityChecker == nil {
		jsonError(w, http.StatusBadRequest, "integrity checker not configured")
		return false
	}
	return true
}

// requireScheduledChecker checks managed scheduled checker exists
func (s *Server) requireScheduledChecker(w http.ResponseWriter) bool {
	if s.managedScheduledChecker == nil {
		jsonError(w, http.StatusBadRequest, "scheduled verification not configured")
		return false
	}
	return true
}

// requireConsensus checks consensus mode is configured
func (s *Server) requireConsensus(w http.ResponseWriter) bool {
	if !s.vaultSvc.HasConsensus() {
		jsonError(w, http.StatusBadRequest, "consensus mode not configured")
		return false
	}
	return true
}

// decodeHexSignature decodes a hex-encoded signature
func decodeHexSignature(hexSig string) ([]byte, error) {
	return hex.DecodeString(hexSig)
}

// decodeJSON decodes JSON body without validation
func decodeJSON(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid request body")
	}
	return nil
}
