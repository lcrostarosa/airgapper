// Package api provides filesystem browsing endpoints
package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FilesystemEntry represents a file or directory entry
type FilesystemEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size,omitempty"`
	ModTime time.Time `json:"modTime"`
}

// BrowseResponse is the response for filesystem browse requests
type BrowseResponse struct {
	Path    string            `json:"path"`
	Parent  string            `json:"parent,omitempty"`
	Entries []FilesystemEntry `json:"entries"`
}

// handleFilesystemBrowse handles GET /api/filesystem/browse
func (s *Server) handleFilesystemBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get query parameters
	requestedPath := r.URL.Query().Get("path")
	showHidden := r.URL.Query().Get("showHidden") == "true"

	// Default to home directory if no path specified
	if requestedPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to get home directory")
			return
		}
		requestedPath = home
	}

	// Clean and validate the path
	cleanPath := filepath.Clean(requestedPath)

	// Prevent path traversal attacks
	if !isPathSafe(cleanPath) {
		jsonError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Check if path exists and is a directory
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			jsonError(w, http.StatusNotFound, "path not found")
			return
		}
		if os.IsPermission(err) {
			jsonError(w, http.StatusForbidden, "permission denied")
			return
		}
		jsonError(w, http.StatusInternalServerError, "failed to access path")
		return
	}

	if !info.IsDir() {
		jsonError(w, http.StatusBadRequest, "path is not a directory")
		return
	}

	// Read directory contents
	dirEntries, err := os.ReadDir(cleanPath)
	if err != nil {
		if os.IsPermission(err) {
			jsonError(w, http.StatusForbidden, "permission denied")
			return
		}
		jsonError(w, http.StatusInternalServerError, "failed to read directory")
		return
	}

	entries := make([]FilesystemEntry, 0)
	for _, entry := range dirEntries {
		name := entry.Name()

		// Skip hidden files if not requested
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		entryPath := filepath.Join(cleanPath, name)
		entryInfo, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't stat
		}

		fsEntry := FilesystemEntry{
			Name:    name,
			Path:    entryPath,
			IsDir:   entry.IsDir(),
			ModTime: entryInfo.ModTime(),
		}

		if !entry.IsDir() {
			fsEntry.Size = entryInfo.Size()
		}

		entries = append(entries, fsEntry)
	}

	// Sort: directories first, then alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir // directories first
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	// Calculate parent path
	parent := ""
	if cleanPath != "/" {
		parent = filepath.Dir(cleanPath)
	}

	response := BrowseResponse{
		Path:    cleanPath,
		Parent:  parent,
		Entries: entries,
	}

	jsonResponse(w, http.StatusOK, response)
}

// isPathSafe validates that a path is safe to access
func isPathSafe(path string) bool {
	// Must be an absolute path
	if !filepath.IsAbs(path) {
		return false
	}

	// Check for suspicious patterns
	if strings.Contains(path, "..") {
		// The path should already be cleaned, so ".." shouldn't appear
		return false
	}

	// Don't allow access to certain sensitive directories
	sensitivePatterns := []string{
		"/etc/shadow",
		"/etc/passwd",
		"/proc",
		"/sys",
	}

	for _, pattern := range sensitivePatterns {
		if strings.HasPrefix(path, pattern) {
			return false
		}
	}

	return true
}
