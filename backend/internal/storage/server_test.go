package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/policy"
)

func TestNewServer(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				BasePath:   filepath.Join(tmpDir, "storage"),
				AppendOnly: true,
			},
			wantErr: false,
		},
		{
			name: "missing base path",
			cfg: Config{
				BasePath: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewServer(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && s == nil {
				t.Error("NewServer() returned nil server")
			}
		})
	}
}

func TestStorageServer_RepoOperations(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()
	defer s.Stop()

	handler := s.Handler()

	// Test creating a repository
	t.Run("create repo", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Verify directories were created
		for _, dir := range []string{"data", "keys", "locks", "snapshots", "index"} {
			path := filepath.Join(tmpDir, "testrepo", dir)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Directory %s was not created", dir)
			}
		}
	})

	// Test checking if repo exists
	t.Run("head repo exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/testrepo/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("head repo not exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/nonexistent/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func TestStorageServer_ConfigOperations(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()

	handler := s.Handler()

	// Create repo first
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	configData := []byte(`{"version":2,"id":"test123"}`)

	// Test creating config
	t.Run("create config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/config", bytes.NewReader(configData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test reading config
	t.Run("get config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/config", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if !bytes.Equal(w.Body.Bytes(), configData) {
			t.Errorf("Config data mismatch: got %s, want %s", w.Body.String(), string(configData))
		}
	})

	// Test config exists check
	t.Run("head config exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/testrepo/config", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	// Test creating config again (should fail)
	t.Run("create config again fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/config", bytes.NewReader(configData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})

	// Test delete config in append-only mode (should fail)
	t.Run("delete config fails in append-only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/testrepo/config", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})
}

func TestStorageServer_DataOperations(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()

	handler := s.Handler()

	// Create repo first
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Create test data with known hash
	testData := []byte("hello world test data for restic backup")
	hash := sha256.Sum256(testData)
	hashHex := hex.EncodeToString(hash[:])

	// Test uploading data
	t.Run("upload data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/data/"+hashHex, bytes.NewReader(testData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test downloading data
	t.Run("download data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/data/"+hashHex, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if !bytes.Equal(w.Body.Bytes(), testData) {
			t.Errorf("Data mismatch")
		}
	})

	// Test HEAD for data
	t.Run("head data exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/testrepo/data/"+hashHex, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	// Test listing data
	t.Run("list data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/data/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		// Response should contain the file name
		if !bytes.Contains(w.Body.Bytes(), []byte(hashHex)) {
			t.Errorf("List should contain uploaded file: %s", w.Body.String())
		}
	})

	// Test delete data in append-only mode (should fail)
	t.Run("delete data fails in append-only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/testrepo/data/"+hashHex, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})

	// Test uploading with wrong hash
	t.Run("upload with wrong hash fails", func(t *testing.T) {
		wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
		req := httptest.NewRequest(http.MethodPost, "/testrepo/data/"+wrongHash, bytes.NewReader(testData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestStorageServer_KeysOperations(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()

	handler := s.Handler()

	// Create repo first
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	keyData := []byte(`{"created":"2024-01-01T00:00:00Z","username":"test"}`)
	keyName := "abc123def456abc123def456abc123def456abc123def456abc123def456abcd"

	// Test uploading key
	t.Run("upload key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/keys/"+keyName, bytes.NewReader(keyData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test downloading key
	t.Run("download key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/keys/"+keyName, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if !bytes.Equal(w.Body.Bytes(), keyData) {
			t.Errorf("Key data mismatch")
		}
	})
}

func TestStorageServer_Quota(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
		QuotaBytes: 100, // Very small quota for testing
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()

	handler := s.Handler()

	// Create repo first
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Create large data that exceeds quota
	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	hash := sha256.Sum256(largeData)
	hashHex := hex.EncodeToString(hash[:])

	t.Run("upload exceeds quota", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/keys/testkey", bytes.NewReader(largeData))
		req.ContentLength = int64(len(largeData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInsufficientStorage {
			t.Errorf("Expected status 507, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Small data should work
	smallData := []byte("small")
	t.Run("upload within quota", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/keys/smallkey", bytes.NewReader(smallData))
		req.ContentLength = int64(len(smallData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	_ = hashHex // Silence unused warning
}

func TestStorageServer_NotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	// Don't start the server

	handler := s.Handler()

	req := httptest.NewRequest(http.MethodGet, "/testrepo/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestStorageServer_DeleteWithoutAppendOnly(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: false, // Allow deletes
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()

	handler := s.Handler()

	// Create repo
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Create a key
	keyData := []byte("test key data")
	req = httptest.NewRequest(http.MethodPost, "/testrepo/keys/testkey123", bytes.NewReader(keyData))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create key: %d", w.Code)
	}

	// Delete should work without append-only
	t.Run("delete allowed without append-only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/testrepo/keys/testkey123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	// Verify file is gone
	t.Run("file is deleted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/keys/testkey123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func TestStorageServer_InvalidInputs(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()

	handler := s.Handler()

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "invalid repo name with slash",
			path:       "/repo/with/slash/config",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid file type",
			path:       "/testrepo/invalid/file",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty repo name",
			path:       "//config",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestStorageServer_Status(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
		QuotaBytes: 1024,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Before starting
	status := s.Status()
	if status.Running {
		t.Error("Server should not be running before Start()")
	}

	s.Start()

	// After starting
	status = s.Status()
	if !status.Running {
		t.Error("Server should be running after Start()")
	}
	if status.BasePath != tmpDir {
		t.Errorf("BasePath mismatch: got %s, want %s", status.BasePath, tmpDir)
	}
	if !status.AppendOnly {
		t.Error("AppendOnly should be true")
	}
	if status.QuotaBytes != 1024 {
		t.Errorf("QuotaBytes mismatch: got %d, want 1024", status.QuotaBytes)
	}

	// Make a request to increment counter
	handler := s.Handler()
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	status = s.Status()
	if status.RequestCount != 1 {
		t.Errorf("RequestCount should be 1, got %d", status.RequestCount)
	}

	s.Stop()

	// After stopping
	status = s.Status()
	if status.Running {
		t.Error("Server should not be running after Stop()")
	}
}

func TestIsValidRepoName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase", "myrepo", true},
		{"valid with numbers", "repo123", true},
		{"valid with hyphen", "my-repo", true},
		{"valid with underscore", "my_repo", true},
		{"valid mixed", "My-Repo_123", true},
		{"empty", "", false},
		{"with slash", "repo/path", false},
		{"with dot", "repo.name", false},
		{"with space", "repo name", false},
		{"too long", string(make([]byte, 65)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidRepoName(tt.input); got != tt.want {
				t.Errorf("isValidRepoName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidFileName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid hex", "abc123def456", true},
		{"valid with hyphen", "abc-123", true},
		{"valid with underscore", "abc_123", true},
		{"valid uppercase", "ABC123", true},
		{"valid with dot", "file.json", true},
		{"empty", "", false},
		{"with slash", "abc/123", false},
		{"path traversal", "abc..def", false},
		{"too long", string(make([]byte, 257)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidFileName(tt.input); got != tt.want {
				t.Errorf("isValidFileName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Benchmark for storage operations
func BenchmarkStorageServer_Upload(b *testing.B) {
	tmpDir := b.TempDir()
	s, _ := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	s.Start()

	handler := s.Handler()

	// Create repo
	req := httptest.NewRequest(http.MethodPost, "/benchrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Prepare test data
	testData := make([]byte, 1024) // 1KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique key for each iteration
		keyName := hex.EncodeToString([]byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)})
		keyName = keyName + "00000000000000000000000000000000000000000000000000000000"

		req := httptest.NewRequest(http.MethodPost, "/benchrepo/keys/"+keyName, bytes.NewReader(testData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkStorageServer_Download(b *testing.B) {
	tmpDir := b.TempDir()
	s, _ := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	s.Start()

	handler := s.Handler()

	// Create repo and upload test data
	req := httptest.NewRequest(http.MethodPost, "/benchrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	testData := make([]byte, 1024)
	keyName := "benchkey00000000000000000000000000000000000000000000000000000000"
	req = httptest.NewRequest(http.MethodPost, "/benchrepo/keys/"+keyName, bytes.NewReader(testData))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/benchrepo/keys/"+keyName, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		io.Copy(io.Discard, w.Body)
	}
}

// Test policy enforcement for deletion
func TestStorageServer_PolicyEnforcement(t *testing.T) {
	tmpDir := t.TempDir()

	// Create keys for owner and host
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()

	// Create a policy with 30 day retention
	p := policy.NewPolicy(
		"TestOwner", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"TestHost", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)
	p.RetentionDays = 30
	p.DeletionMode = policy.DeletionTimeLockOnly
	p.AppendOnlyLocked = false // Allow testing deletion

	// Sign the policy
	p.SignAsOwner(ownerPriv)
	p.SignAsHost(hostPriv)

	// Create server with policy
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: false, // Allow deletes (policy will control)
		Policy:     p,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()

	handler := s.Handler()

	// Create repo
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Create a file
	keyData := []byte("test key data")
	req = httptest.NewRequest(http.MethodPost, "/testrepo/keys/testkey123", bytes.NewReader(keyData))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create key: %d", w.Code)
	}

	// Try to delete - should fail due to retention period
	t.Run("delete blocked by retention period", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/testrepo/keys/testkey123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Verify status shows policy
	t.Run("status shows policy", func(t *testing.T) {
		status := s.Status()
		if !status.HasPolicy {
			t.Error("Status should show hasPolicy=true")
		}
		if status.PolicyID != p.ID {
			t.Errorf("PolicyID mismatch: got %s, want %s", status.PolicyID, p.ID)
		}
	})
}

// Test setting policy
func TestStorageServer_SetPolicy(t *testing.T) {
	tmpDir := t.TempDir()

	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: false,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initially no policy
	if s.GetPolicy() != nil {
		t.Error("Should have no policy initially")
	}

	// Create and sign a policy
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()

	p := policy.NewPolicy(
		"TestOwner", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"TestHost", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)
	p.SignAsOwner(ownerPriv)
	p.SignAsHost(hostPriv)

	// Set policy
	if err := s.SetPolicy(p); err != nil {
		t.Fatalf("SetPolicy failed: %v", err)
	}

	// Verify policy is set
	got := s.GetPolicy()
	if got == nil {
		t.Fatal("GetPolicy returned nil after SetPolicy")
	}
	if got.ID != p.ID {
		t.Errorf("Policy ID mismatch: got %s, want %s", got.ID, p.ID)
	}

	// Test unsigned policy is rejected
	t.Run("unsigned policy rejected", func(t *testing.T) {
		unsigned := policy.NewPolicy(
			"TestOwner", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
			"TestHost", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
		)
		if err := s.SetPolicy(unsigned); err == nil {
			t.Error("SetPolicy should reject unsigned policy")
		}
	})

	// Test nil policy rejected
	t.Run("nil policy rejected", func(t *testing.T) {
		if err := s.SetPolicy(nil); err == nil {
			t.Error("SetPolicy should reject nil policy")
		}
	})
}

// Test audit logging
func TestStorageServer_AuditLog(t *testing.T) {
	tmpDir := t.TempDir()

	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: false,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	s.Start()

	handler := s.Handler()

	// Create repo
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Create and delete a file to generate audit entries
	keyData := []byte("test")
	req = httptest.NewRequest(http.MethodPost, "/testrepo/keys/auditkey", bytes.NewReader(keyData))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	req = httptest.NewRequest(http.MethodDelete, "/testrepo/keys/auditkey", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check audit log
	entries := s.GetAuditLog(10)
	if len(entries) == 0 {
		t.Error("Expected audit entries")
	}

	// Find the delete entry
	foundDelete := false
	for _, e := range entries {
		if e.Operation == "DELETE" {
			foundDelete = true
			if !e.Success {
				t.Error("Delete should be marked as successful")
			}
			break
		}
	}
	if !foundDelete {
		t.Error("Expected DELETE operation in audit log")
	}
}

// Test policy persistence
func TestStorageServer_PolicyPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()

	p := policy.NewPolicy(
		"TestOwner", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"TestHost", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)
	p.RetentionDays = 90
	p.SignAsOwner(ownerPriv)
	p.SignAsHost(hostPriv)

	// Create server and set policy
	s1, _ := NewServer(Config{BasePath: tmpDir})
	s1.SetPolicy(p)

	// Create new server pointing to same directory - should load policy
	s2, _ := NewServer(Config{BasePath: tmpDir})

	got := s2.GetPolicy()
	if got == nil {
		t.Fatal("Policy should be loaded from disk")
	}
	if got.RetentionDays != 90 {
		t.Errorf("RetentionDays mismatch: got %d, want 90", got.RetentionDays)
	}
}

// Silence unused variable warnings
var _ = time.Now
