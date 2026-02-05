package errors

import (
	"regexp"
	"strings"
)

// Patterns that should be redacted from error messages
var sensitivePatterns = []*regexp.Regexp{
	// File paths (Unix and Windows)
	regexp.MustCompile(`(?i)(/home/[^\s:]+|/Users/[^\s:]+|/root/[^\s:]+|/etc/[^\s:]+|/var/[^\s:]+)`),
	regexp.MustCompile(`(?i)([A-Z]:\\[^\s:]+)`),

	// Passwords and API keys in URLs or strings
	regexp.MustCompile(`(?i)(password|passwd|pwd|secret|api[_-]?key|token|bearer)[=:]["']?[^\s"'&]+`),

	// Environment variables that might contain secrets
	regexp.MustCompile(`(?i)(RESTIC_PASSWORD|API_KEY|SECRET_KEY|AWS_SECRET)[=][^\s]+`),

	// SSH keys or similar
	regexp.MustCompile(`(?i)(ssh-rsa|ssh-ed25519|-----BEGIN [A-Z]+ KEY-----)[^\s]+`),

	// IP addresses (internal)
	regexp.MustCompile(`(?i)(192\.168\.\d+\.\d+|10\.\d+\.\d+\.\d+|172\.(1[6-9]|2[0-9]|3[0-1])\.\d+\.\d+)`),

	// Email addresses
	regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),

	// UUIDs (might be request IDs or other sensitive identifiers)
	regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`),
}

// Generic replacements for common sensitive information types
var replacements = map[string]string{
	"password":    "[REDACTED]",
	"secret":      "[REDACTED]",
	"key":         "[REDACTED]",
	"token":       "[REDACTED]",
	"credentials": "[REDACTED]",
}

// SanitizeError removes sensitive information from error messages
// for display to clients. Internal logging should use the original error.
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeString(err.Error())
}

// SanitizeString removes sensitive information from a string
func SanitizeString(s string) string {
	result := s

	// Apply regex-based redactions
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}

	// Apply keyword-based redactions (case-insensitive)
	lower := strings.ToLower(result)
	for keyword, replacement := range replacements {
		if strings.Contains(lower, keyword) {
			// Only replace if it looks like it's exposing a value
			re := regexp.MustCompile(`(?i)` + keyword + `[=:]["']?[^\s"']+`)
			result = re.ReplaceAllString(result, keyword+"="+replacement)
		}
	}

	return result
}

// SafeError wraps an error with a sanitized message for client-facing use
// while preserving the original error for internal logging
type SafeError struct {
	// Original is the full error (for logging)
	Original error
	// Message is the sanitized message (for clients)
	Message string
}

func (e *SafeError) Error() string {
	return e.Message
}

func (e *SafeError) Unwrap() error {
	return e.Original
}

// NewSafeError creates a client-safe error from an internal error
func NewSafeError(err error) *SafeError {
	if err == nil {
		return nil
	}
	return &SafeError{
		Original: err,
		Message:  SanitizeString(err.Error()),
	}
}

// NewSafeErrorWithMessage creates a safe error with a custom client message
func NewSafeErrorWithMessage(err error, clientMessage string) *SafeError {
	return &SafeError{
		Original: err,
		Message:  clientMessage,
	}
}

// GenericError returns a generic error message suitable for clients
// when the actual error should not be exposed
func GenericError(operation string) string {
	if operation == "" {
		return "An error occurred. Please try again."
	}
	return "An error occurred during " + operation + ". Please try again."
}
