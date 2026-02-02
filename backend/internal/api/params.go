package api

import (
	"net/http"
)

// RequirePathParam extracts a path parameter and validates it's not empty.
// Returns the parameter value and true if valid, or writes an error response and returns false.
func RequirePathParam(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	value := r.PathValue(name)
	if value == "" {
		jsonError(w, http.StatusBadRequest, name+" required")
		return "", false
	}
	return value, true
}

// RequireQueryParam extracts a query parameter and validates it's not empty.
// Returns the parameter value and true if valid, or writes an error response and returns false.
func RequireQueryParam(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		jsonError(w, http.StatusBadRequest, name+" required")
		return "", false
	}
	return value, true
}

// GetQueryParam extracts a query parameter with a default value.
func GetQueryParam(r *http.Request, name, defaultValue string) string {
	value := r.URL.Query().Get(name)
	if value == "" {
		return defaultValue
	}
	return value
}
