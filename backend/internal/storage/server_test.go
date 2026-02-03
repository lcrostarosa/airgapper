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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			if tt.wantErr {
				assert.Error(t, err, "NewServer() should return error")
				return
			}
			require.NoError(t, err, "NewServer() should not return error")
			assert.NotNil(t, s, "NewServer() returned nil server")
		})
	}
}

func TestStorageServer_RepoOperations(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	require.NoError(t, err, "Failed to create server")
	s.Start()
	defer s.Stop()

	handler := s.Handler()

	// Test creating a repository
	t.Run("create repo", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")

		// Verify directories were created
		for _, dir := range []string{"data", "keys", "locks", "snapshots", "index"} {
			path := filepath.Join(tmpDir, "testrepo", dir)
			_, err := os.Stat(path)
			assert.False(t, os.IsNotExist(err), "Directory %s was not created", dir)
		}
	})

	// Test checking if repo exists
	t.Run("head repo exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/testrepo/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
	})

	t.Run("head repo not exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/nonexistent/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code, "Expected status 404")
	})
}

func TestStorageServer_ConfigOperations(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	require.NoError(t, err, "Failed to create server")
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

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200: %s", w.Body.String())
	})

	// Test reading config
	t.Run("get config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/config", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
		assert.True(t, bytes.Equal(w.Body.Bytes(), configData), "Config data mismatch: got %s, want %s", w.Body.String(), string(configData))
	})

	// Test config exists check
	t.Run("head config exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/testrepo/config", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
	})

	// Test creating config again (should fail)
	t.Run("create config again fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/config", bytes.NewReader(configData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, "Expected status 403")
	})

	// Test delete config in append-only mode (should fail)
	t.Run("delete config fails in append-only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/testrepo/config", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, "Expected status 403")
	})
}

func TestStorageServer_DataOperations(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	require.NoError(t, err, "Failed to create server")
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

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200: %s", w.Body.String())
	})

	// Test downloading data
	t.Run("download data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/data/"+hashHex, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
		assert.True(t, bytes.Equal(w.Body.Bytes(), testData), "Data mismatch")
	})

	// Test HEAD for data
	t.Run("head data exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/testrepo/data/"+hashHex, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
	})

	// Test listing data
	t.Run("list data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/data/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
		// Response should contain the file name
		assert.True(t, bytes.Contains(w.Body.Bytes(), []byte(hashHex)), "List should contain uploaded file: %s", w.Body.String())
	})

	// Test delete data in append-only mode (should fail)
	t.Run("delete data fails in append-only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/testrepo/data/"+hashHex, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, "Expected status 403")
	})

	// Test uploading with wrong hash
	t.Run("upload with wrong hash fails", func(t *testing.T) {
		wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
		req := httptest.NewRequest(http.MethodPost, "/testrepo/data/"+wrongHash, bytes.NewReader(testData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "Expected status 400")
	})
}

func TestStorageServer_KeysOperations(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	require.NoError(t, err, "Failed to create server")
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

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200: %s", w.Body.String())
	})

	// Test downloading key
	t.Run("download key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/keys/"+keyName, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
		assert.True(t, bytes.Equal(w.Body.Bytes(), keyData), "Key data mismatch")
	})
}

func TestStorageServer_Quota(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
		QuotaBytes: 100, // Very small quota for testing
	})
	require.NoError(t, err, "Failed to create server")
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

		assert.Equal(t, http.StatusInsufficientStorage, w.Code, "Expected status 507: %s", w.Body.String())
	})

	// Small data should work
	smallData := []byte("small")
	t.Run("upload within quota", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/testrepo/keys/smallkey", bytes.NewReader(smallData))
		req.ContentLength = int64(len(smallData))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200: %s", w.Body.String())
	})

	_ = hashHex // Silence unused warning
}

func TestStorageServer_NotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath: tmpDir,
	})
	require.NoError(t, err, "Failed to create server")
	// Don't start the server

	handler := s.Handler()

	req := httptest.NewRequest(http.MethodGet, "/testrepo/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "Expected status 503")
}

func TestStorageServer_DeleteWithoutAppendOnly(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: false, // Allow deletes
	})
	require.NoError(t, err, "Failed to create server")
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

	require.Equal(t, http.StatusOK, w.Code, "Failed to create key")

	// Delete should work without append-only
	t.Run("delete allowed without append-only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/testrepo/keys/testkey123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected status 200")
	})

	// Verify file is gone
	t.Run("file is deleted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/testrepo/keys/testkey123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code, "Expected status 404")
	})
}

func TestStorageServer_InvalidInputs(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: true,
	})
	require.NoError(t, err, "Failed to create server")
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

			assert.Equal(t, tt.wantStatus, w.Code, "Expected status %d", tt.wantStatus)
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
	require.NoError(t, err, "Failed to create server")

	// Before starting
	status := s.Status()
	assert.False(t, status.Running, "Server should not be running before Start()")

	s.Start()

	// After starting
	status = s.Status()
	assert.True(t, status.Running, "Server should be running after Start()")
	assert.Equal(t, tmpDir, status.BasePath, "BasePath mismatch")
	assert.True(t, status.AppendOnly, "AppendOnly should be true")
	assert.Equal(t, int64(1024), status.QuotaBytes, "QuotaBytes mismatch")

	// Make a request to increment counter
	handler := s.Handler()
	req := httptest.NewRequest(http.MethodPost, "/testrepo/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	status = s.Status()
	assert.Equal(t, int64(1), status.RequestCount, "RequestCount should be 1")

	s.Stop()

	// After stopping
	status = s.Status()
	assert.False(t, status.Running, "Server should not be running after Stop()")
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
			got := isValidRepoName(tt.input)
			assert.Equal(t, tt.want, got, "isValidRepoName(%q)", tt.input)
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
			got := isValidFileName(tt.input)
			assert.Equal(t, tt.want, got, "isValidFileName(%q)", tt.input)
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
		_, _ = io.Copy(io.Discard, w.Body)
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
	require.NoError(t, p.SignAsOwner(ownerPriv), "Failed to sign as owner")
	require.NoError(t, p.SignAsHost(hostPriv), "Failed to sign as host")

	// Create server with policy
	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: false, // Allow deletes (policy will control)
		Policy:     p,
	})
	require.NoError(t, err, "Failed to create server")
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

	require.Equal(t, http.StatusOK, w.Code, "Failed to create key")

	// Try to delete - should fail due to retention period
	t.Run("delete blocked by retention period", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/testrepo/keys/testkey123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, "Expected status 403: %s", w.Body.String())
	})

	// Verify status shows policy
	t.Run("status shows policy", func(t *testing.T) {
		status := s.Status()
		assert.True(t, status.HasPolicy, "Status should show hasPolicy=true")
		assert.Equal(t, p.ID, status.PolicyID, "PolicyID mismatch")
	})
}

// Test setting policy
func TestStorageServer_SetPolicy(t *testing.T) {
	tmpDir := t.TempDir()

	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: false,
	})
	require.NoError(t, err, "Failed to create server")

	// Initially no policy
	assert.Nil(t, s.GetPolicy(), "Should have no policy initially")

	// Create and sign a policy
	ownerPub, ownerPriv, _ := crypto.GenerateKeyPair()
	hostPub, hostPriv, _ := crypto.GenerateKeyPair()

	p := policy.NewPolicy(
		"TestOwner", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
		"TestHost", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
	)
	require.NoError(t, p.SignAsOwner(ownerPriv), "Failed to sign as owner")
	require.NoError(t, p.SignAsHost(hostPriv), "Failed to sign as host")

	// Set policy
	err = s.SetPolicy(p)
	require.NoError(t, err, "SetPolicy failed")

	// Verify policy is set
	got := s.GetPolicy()
	require.NotNil(t, got, "GetPolicy returned nil after SetPolicy")
	assert.Equal(t, p.ID, got.ID, "Policy ID mismatch")

	// Test unsigned policy is rejected
	t.Run("unsigned policy rejected", func(t *testing.T) {
		unsigned := policy.NewPolicy(
			"TestOwner", crypto.KeyID(ownerPub), crypto.EncodePublicKey(ownerPub),
			"TestHost", crypto.KeyID(hostPub), crypto.EncodePublicKey(hostPub),
		)
		err := s.SetPolicy(unsigned)
		assert.Error(t, err, "SetPolicy should reject unsigned policy")
	})

	// Test nil policy rejected
	t.Run("nil policy rejected", func(t *testing.T) {
		err := s.SetPolicy(nil)
		assert.Error(t, err, "SetPolicy should reject nil policy")
	})
}

// Test audit logging
func TestStorageServer_AuditLog(t *testing.T) {
	tmpDir := t.TempDir()

	s, err := NewServer(Config{
		BasePath:   tmpDir,
		AppendOnly: false,
	})
	require.NoError(t, err, "Failed to create server")
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
	assert.NotEmpty(t, entries, "Expected audit entries")

	// Find the delete entry
	foundDelete := false
	for _, e := range entries {
		if e.Operation == "DELETE" {
			foundDelete = true
			assert.True(t, e.Success, "Delete should be marked as successful")
			break
		}
	}
	assert.True(t, foundDelete, "Expected DELETE operation in audit log")
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
	require.NoError(t, p.SignAsOwner(ownerPriv), "Failed to sign as owner")
	require.NoError(t, p.SignAsHost(hostPriv), "Failed to sign as host")

	// Create server and set policy
	s1, _ := NewServer(Config{BasePath: tmpDir})
	require.NoError(t, s1.SetPolicy(p), "Failed to set policy")

	// Create new server pointing to same directory - should load policy
	s2, _ := NewServer(Config{BasePath: tmpDir})

	got := s2.GetPolicy()
	require.NotNil(t, got, "Policy should be loaded from disk")
	assert.Equal(t, 90, got.RetentionDays, "RetentionDays mismatch")
}

// Silence unused variable warnings
var _ = time.Now
