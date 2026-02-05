package storage

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrPathTraversal indicates an attempted path traversal attack
var ErrPathTraversal = errors.New("path traversal detected")

// Valid restic file types
var validTypes = map[string]bool{
	"data":      true,
	"keys":      true,
	"locks":     true,
	"snapshots": true,
	"index":     true,
}

func isValidRepoName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	// Explicit path traversal check
	if containsPathTraversal(name) {
		return false
	}
	return true
}

func isValidFileName(name string) bool {
	if name == "" || len(name) > 256 {
		return false
	}
	// Allow alphanumeric, hyphen, underscore, and dot
	// Restic uses various naming conventions for different file types
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' && c != '.' {
			return false
		}
	}
	// Don't allow path traversal
	if containsPathTraversal(name) {
		return false
	}
	return true
}

// containsPathTraversal checks for path traversal attempts
func containsPathTraversal(path string) bool {
	// Check for obvious traversal patterns
	if strings.Contains(path, "..") {
		return true
	}
	if strings.HasPrefix(path, "/") {
		return true
	}
	if strings.HasPrefix(path, "~") {
		return true
	}
	// Check for null bytes (can bypass checks in some systems)
	if strings.Contains(path, "\x00") {
		return true
	}
	return false
}

// ValidateSafePath ensures a path stays within the base directory
// Returns the cleaned absolute path if valid, or an error if path traversal is detected
func ValidateSafePath(basePath, requestedPath string) (string, error) {
	// Clean both paths
	cleanBase := filepath.Clean(basePath)

	// Join and clean the full path
	fullPath := filepath.Join(cleanBase, filepath.Clean(requestedPath))

	// Verify the result is still within the base path
	// Use filepath.Rel to check if the path is relative to base
	rel, err := filepath.Rel(cleanBase, fullPath)
	if err != nil {
		return "", ErrPathTraversal
	}

	// If the relative path starts with "..", it's outside the base
	if strings.HasPrefix(rel, "..") || rel == ".." {
		return "", ErrPathTraversal
	}

	// Additional check: verify the path actually starts with the base path
	if !strings.HasPrefix(fullPath, cleanBase) {
		return "", ErrPathTraversal
	}

	return fullPath, nil
}

// ValidateURLScheme validates that a URL uses an allowed scheme
func ValidateURLScheme(url string) error {
	// Allow HTTPS always
	if strings.HasPrefix(url, "https://") {
		return nil
	}

	// Allow HTTP only for localhost/private networks
	if strings.HasPrefix(url, "http://") {
		host := strings.TrimPrefix(url, "http://")
		// Extract hostname (up to first / or :)
		if idx := strings.IndexAny(host, "/:"); idx != -1 {
			host = host[:idx]
		}

		// Allow localhost and private network ranges
		if host == "localhost" ||
			host == "127.0.0.1" ||
			strings.HasPrefix(host, "192.168.") ||
			strings.HasPrefix(host, "10.") ||
			strings.HasPrefix(host, "172.16.") ||
			strings.HasPrefix(host, "172.17.") ||
			strings.HasPrefix(host, "172.18.") ||
			strings.HasPrefix(host, "172.19.") ||
			strings.HasPrefix(host, "172.20.") ||
			strings.HasPrefix(host, "172.21.") ||
			strings.HasPrefix(host, "172.22.") ||
			strings.HasPrefix(host, "172.23.") ||
			strings.HasPrefix(host, "172.24.") ||
			strings.HasPrefix(host, "172.25.") ||
			strings.HasPrefix(host, "172.26.") ||
			strings.HasPrefix(host, "172.27.") ||
			strings.HasPrefix(host, "172.28.") ||
			strings.HasPrefix(host, "172.29.") ||
			strings.HasPrefix(host, "172.30.") ||
			strings.HasPrefix(host, "172.31.") {
			return nil
		}

		return errors.New("HTTP only allowed for localhost and private networks; use HTTPS for public URLs")
	}

	return errors.New("URL must use http:// or https:// scheme")
}
