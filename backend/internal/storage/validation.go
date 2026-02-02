package storage

import "strings"

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
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
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
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	// Don't allow path traversal
	if strings.Contains(name, "..") {
		return false
	}
	return true
}
