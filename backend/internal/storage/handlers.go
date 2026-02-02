package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requestCount++
	running := s.running
	s.mu.Unlock()

	if !running {
		http.Error(w, "Storage server not running", http.StatusServiceUnavailable)
		return
	}

	// Parse the path: /{repo}/{type}/{name} or /{repo}/{type}/ or /{repo}/config
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "Repository name required", http.StatusBadRequest)
		return
	}

	repo := parts[0]

	// Validate repo name (alphanumeric, hyphens, underscores only)
	if !isValidRepoName(repo) {
		http.Error(w, "Invalid repository name", http.StatusBadRequest)
		return
	}

	// Handle different path patterns
	if len(parts) == 1 || (len(parts) == 2 && parts[1] == "") {
		// /{repo}/ - Repository root
		s.handleRepo(w, r, repo)
		return
	}

	if parts[1] == "config" {
		// /{repo}/config - Repository config file
		s.handleConfig(w, r, repo)
		return
	}

	fileType := parts[1]
	if !validTypes[fileType] {
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 || (len(parts) == 3 && parts[2] == "") {
		// /{repo}/{type}/ - List files
		s.handleList(w, r, repo, fileType)
		return
	}

	// /{repo}/{type}/{name} - Individual file
	fileName := parts[2]
	s.handleFile(w, r, repo, fileType, fileName)
}

func (s *Server) handleRepo(w http.ResponseWriter, r *http.Request, repo string) {
	repoPath := filepath.Join(s.basePath, repo)

	switch r.Method {
	case http.MethodPost:
		// Create repository
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			http.Error(w, "Failed to create repository", http.StatusInternalServerError)
			return
		}
		// Create subdirectories
		for fileType := range validTypes {
			if err := os.MkdirAll(filepath.Join(repoPath, fileType), 0755); err != nil {
				http.Error(w, "Failed to create repository", http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodHead:
		// Check if repo exists
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			http.Error(w, "Repository not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request, repo string) {
	repoPath := filepath.Join(s.basePath, repo)
	configPath := filepath.Join(repoPath, "config")

	switch r.Method {
	case http.MethodHead:
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			http.Error(w, "Config not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		data, err := os.ReadFile(configPath)
		if os.IsNotExist(err) {
			http.Error(w, "Config not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to read config", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(data)

	case http.MethodPost:
		// Config can only be created once
		if _, err := os.Stat(configPath); err == nil {
			http.Error(w, "Config already exists", http.StatusForbidden)
			return
		}

		// Ensure repo directory exists
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			http.Error(w, "Failed to create repository", http.StatusInternalServerError)
			return
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			http.Error(w, "Failed to write config", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		allowed, reason := s.checkDeleteAllowed(configPath)
		if !allowed {
			s.audit("DELETE_DENIED", configPath, reason, false, reason)
			http.Error(w, reason, http.StatusForbidden)
			return
		}
		if err := os.Remove(configPath); err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Config not found", http.StatusNotFound)
				return
			}
			s.audit("DELETE", configPath, "", false, err.Error())
			http.Error(w, "Failed to delete config", http.StatusInternalServerError)
			return
		}
		s.audit("DELETE", configPath, "config deleted", true, "")
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request, repo, fileType string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dirPath := filepath.Join(s.basePath, repo, fileType)

	// For data directory, we need to look in subdirectories
	type fileEntry struct {
		name string
		size int64
	}
	var files []fileEntry

	if fileType == "data" {
		// Data files are stored in subdirectories by first 2 chars of hash
		filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				files = append(files, fileEntry{name: info.Name(), size: info.Size()})
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(dirPath)
		if os.IsNotExist(err) {
			// Return empty list for missing directory
			w.Header().Set("Content-Type", "application/vnd.x.restic.rest.v2")
			fmt.Fprint(w, "[]")
			return
		}
		if err != nil {
			http.Error(w, "Failed to list directory", http.StatusInternalServerError)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileEntry{name: entry.Name(), size: info.Size()})
		}
	}

	// Build JSON response
	w.Header().Set("Content-Type", "application/vnd.x.restic.rest.v2")
	fmt.Fprint(w, "[")
	for i, f := range files {
		if i > 0 {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, `{"name":%q,"size":%d}`, f.name, f.size)
	}
	fmt.Fprint(w, "]")
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request, repo, fileType, fileName string) {
	// Validate filename (should be hex for most types)
	if !isValidFileName(fileName) {
		http.Error(w, "Invalid file name", http.StatusBadRequest)
		return
	}

	// For data files, use subdirectory structure (first 2 chars)
	var filePath string
	if fileType == "data" && len(fileName) >= 2 {
		subdir := fileName[:2]
		filePath = filepath.Join(s.basePath, repo, fileType, subdir, fileName)
	} else {
		filePath = filepath.Join(s.basePath, repo, fileType, fileName)
	}

	switch r.Method {
	case http.MethodHead:
		info, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to stat file", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		file, err := os.Open(filePath)
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Failed to open file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		info, _ := file.Stat()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		io.Copy(w, file)

	case http.MethodPost:
		contentLength := r.ContentLength
		if contentLength < 0 {
			contentLength = 0 // Unknown size
		}

		// Check system disk space first
		if ok, reason := s.checkDiskSpace(contentLength); !ok {
			s.audit("WRITE_DENIED", filePath, reason, false, reason)
			http.Error(w, reason, http.StatusInsufficientStorage)
			return
		}

		// Check per-repo quota
		if s.quotaBytes > 0 && contentLength > 0 {
			currentUsed := s.calculateUsedSpace()
			if currentUsed+contentLength > s.quotaBytes {
				http.Error(w, "Storage quota exceeded", http.StatusInsufficientStorage)
				return
			}
		}

		// Ensure directory exists
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}

		// Write to temp file first, then rename (atomic write)
		tmpPath := filePath + ".tmp"
		file, err := os.Create(tmpPath)
		if err != nil {
			http.Error(w, "Failed to create file", http.StatusInternalServerError)
			return
		}

		hash := sha256.New()
		written, err := io.Copy(io.MultiWriter(file, hash), r.Body)
		file.Close()

		if err != nil {
			os.Remove(tmpPath)
			http.Error(w, "Failed to write file", http.StatusInternalServerError)
			return
		}

		// For data blobs, verify the hash matches the filename
		if fileType == "data" {
			expectedHash := fileName
			actualHash := hex.EncodeToString(hash.Sum(nil))
			if actualHash != expectedHash {
				os.Remove(tmpPath)
				http.Error(w, "Hash mismatch", http.StatusBadRequest)
				return
			}
		}

		// Rename temp file to final name
		if err := os.Rename(tmpPath, filePath); err != nil {
			os.Remove(tmpPath)
			http.Error(w, "Failed to finalize file", http.StatusInternalServerError)
			return
		}

		s.mu.Lock()
		s.totalBytes += written
		s.mu.Unlock()

		// Audit file creation for snapshots (to track what backups exist)
		if fileType == "snapshots" {
			s.audit("SNAPSHOT_CREATE", filePath, fmt.Sprintf("snapshot %s created (%d bytes)", fileName, written), true, "")
		}

		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		allowed, reason := s.checkDeleteAllowed(filePath)
		if !allowed {
			s.audit("DELETE_DENIED", filePath, reason, false, reason)
			http.Error(w, reason, http.StatusForbidden)
			return
		}

		if err := os.Remove(filePath); err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			s.audit("DELETE", filePath, "", false, err.Error())
			http.Error(w, "Failed to delete file", http.StatusInternalServerError)
			return
		}
		s.audit("DELETE", filePath, fmt.Sprintf("%s/%s deleted", fileType, fileName), true, "")
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
