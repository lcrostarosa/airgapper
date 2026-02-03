// Package config tests
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/emergency"
	apperrors "github.com/lcrostarosa/airgapper/backend/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper functions ---

func createTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func writeConfigFile(t *testing.T, dir string, cfg *Config) {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)

	err = os.MkdirAll(dir, 0700)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "config.json"), data, 0600)
	require.NoError(t, err)
}

// --- DefaultConfigDir tests ---

func TestDefaultConfigDir(t *testing.T) {
	dir := DefaultConfigDir()
	assert.NotEmpty(t, dir)
	assert.True(t, filepath.IsAbs(dir))
	assert.Contains(t, dir, ".airgapper")
}

// --- Load tests ---

func TestLoad(t *testing.T) {
	t.Run("loads valid config", func(t *testing.T) {
		dir := createTempConfigDir(t)
		expected := &Config{
			Name:       "test-node",
			Role:       RoleOwner,
			RepoURL:    "rest:http://localhost:8000/",
			ListenAddr: ":8081",
			PublicKey:  []byte{1, 2, 3},
			PrivateKey: []byte{4, 5, 6},
		}
		writeConfigFile(t, dir, expected)

		cfg, err := Load(dir)
		require.NoError(t, err)
		assert.Equal(t, "test-node", cfg.Name)
		assert.Equal(t, RoleOwner, cfg.Role)
		assert.Equal(t, "rest:http://localhost:8000/", cfg.RepoURL)
		assert.Equal(t, ":8081", cfg.ListenAddr)
		assert.Equal(t, []byte{1, 2, 3}, cfg.PublicKey)
		assert.Equal(t, []byte{4, 5, 6}, cfg.PrivateKey)
		assert.Equal(t, dir, cfg.ConfigDir)
	})

	t.Run("returns ErrNotInitialized for missing file", func(t *testing.T) {
		dir := createTempConfigDir(t)
		cfg, err := Load(dir)
		assert.Nil(t, cfg)
		assert.ErrorIs(t, err, apperrors.ErrNotInitialized)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		dir := createTempConfigDir(t)
		err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{invalid json"), 0600)
		require.NoError(t, err)

		cfg, err := Load(dir)
		assert.Nil(t, cfg)
		assert.Error(t, err)
	})

	t.Run("uses default dir when empty string provided", func(t *testing.T) {
		// This will try to load from default dir which likely doesn't exist
		// or has a real config, so we just verify it doesn't panic
		_, _ = Load("")
	})

	t.Run("loads config with consensus settings", func(t *testing.T) {
		dir := createTempConfigDir(t)
		expected := &Config{
			Name:    "consensus-node",
			Role:    RoleOwner,
			RepoURL: "rest:http://localhost:8000/",
			Consensus: &ConsensusConfig{
				Threshold: 2,
				TotalKeys: 3,
				KeyHolders: []KeyHolder{
					{
						ID:        "key1",
						Name:      "Alice",
						PublicKey: []byte{1, 2, 3},
						JoinedAt:  time.Now().UTC().Truncate(time.Second),
						IsOwner:   true,
					},
				},
				RequireApproval: true,
			},
		}
		writeConfigFile(t, dir, expected)

		cfg, err := Load(dir)
		require.NoError(t, err)
		require.NotNil(t, cfg.Consensus)
		assert.Equal(t, 2, cfg.Consensus.Threshold)
		assert.Equal(t, 3, cfg.Consensus.TotalKeys)
		assert.Len(t, cfg.Consensus.KeyHolders, 1)
		assert.Equal(t, "key1", cfg.Consensus.KeyHolders[0].ID)
		assert.True(t, cfg.Consensus.RequireApproval)
	})

	t.Run("loads config with peer info (SSS mode)", func(t *testing.T) {
		dir := createTempConfigDir(t)
		expected := &Config{
			Name:       "sss-node",
			Role:       RoleHost,
			RepoURL:    "rest:http://localhost:8000/",
			LocalShare: []byte{10, 20, 30},
			ShareIndex: 1,
			Peer: &PeerInfo{
				Name:      "Owner",
				PublicKey: []byte{7, 8, 9},
				Address:   "192.168.1.100:8081",
			},
		}
		writeConfigFile(t, dir, expected)

		cfg, err := Load(dir)
		require.NoError(t, err)
		assert.Equal(t, []byte{10, 20, 30}, cfg.LocalShare)
		assert.Equal(t, byte(1), cfg.ShareIndex)
		require.NotNil(t, cfg.Peer)
		assert.Equal(t, "Owner", cfg.Peer.Name)
		assert.Equal(t, "192.168.1.100:8081", cfg.Peer.Address)
	})

	t.Run("loads config with backup settings", func(t *testing.T) {
		dir := createTempConfigDir(t)
		expected := &Config{
			Name:           "backup-node",
			Role:           RoleOwner,
			RepoURL:        "rest:http://localhost:8000/",
			BackupPaths:    []string{"/home/user/documents", "/home/user/photos"},
			BackupSchedule: "0 2 * * *",
			BackupExclude:  []string{"*.tmp", "*.log"},
		}
		writeConfigFile(t, dir, expected)

		cfg, err := Load(dir)
		require.NoError(t, err)
		assert.Equal(t, []string{"/home/user/documents", "/home/user/photos"}, cfg.BackupPaths)
		assert.Equal(t, "0 2 * * *", cfg.BackupSchedule)
		assert.Equal(t, []string{"*.tmp", "*.log"}, cfg.BackupExclude)
	})

	t.Run("loads config with storage settings", func(t *testing.T) {
		dir := createTempConfigDir(t)
		expected := &Config{
			Name:              "host-node",
			Role:              RoleHost,
			RepoURL:           "rest:http://localhost:8000/",
			StoragePath:       "/var/backups",
			StorageQuotaBytes: 1024 * 1024 * 1024,
			StorageAppendOnly: true,
			StoragePort:       8000,
		}
		writeConfigFile(t, dir, expected)

		cfg, err := Load(dir)
		require.NoError(t, err)
		assert.Equal(t, "/var/backups", cfg.StoragePath)
		assert.Equal(t, int64(1024*1024*1024), cfg.StorageQuotaBytes)
		assert.True(t, cfg.StorageAppendOnly)
		assert.Equal(t, 8000, cfg.StoragePort)
	})
}

// --- Exists tests ---

func TestExists(t *testing.T) {
	t.Run("returns true when config exists", func(t *testing.T) {
		dir := createTempConfigDir(t)
		writeConfigFile(t, dir, &Config{Name: "test"})

		assert.True(t, Exists(dir))
	})

	t.Run("returns false when config does not exist", func(t *testing.T) {
		dir := createTempConfigDir(t)
		assert.False(t, Exists(dir))
	})

	t.Run("uses default dir when empty string provided", func(t *testing.T) {
		// Just verify it doesn't panic
		_ = Exists("")
	})
}

// --- Save tests ---

func TestSave(t *testing.T) {
	t.Run("saves config to disk", func(t *testing.T) {
		dir := createTempConfigDir(t)
		cfg := &Config{
			Name:       "test-node",
			Role:       RoleOwner,
			RepoURL:    "rest:http://localhost:8000/",
			ConfigDir:  dir,
			PublicKey:  []byte{1, 2, 3},
			PrivateKey: []byte{4, 5, 6},
		}

		err := cfg.Save()
		require.NoError(t, err)

		// Verify file was created
		configPath := filepath.Join(dir, "config.json")
		assert.FileExists(t, configPath)

		// Verify contents
		data, err := os.ReadFile(configPath)
		require.NoError(t, err)

		var loaded Config
		err = json.Unmarshal(data, &loaded)
		require.NoError(t, err)
		assert.Equal(t, "test-node", loaded.Name)
		assert.Equal(t, RoleOwner, loaded.Role)
		assert.Equal(t, []byte{1, 2, 3}, loaded.PublicKey)
	})

	t.Run("creates directory if it doesn't exist", func(t *testing.T) {
		dir := filepath.Join(createTempConfigDir(t), "nested", "dir")
		cfg := &Config{
			Name:      "test-node",
			Role:      RoleHost,
			ConfigDir: dir,
		}

		err := cfg.Save()
		require.NoError(t, err)

		assert.DirExists(t, dir)
		assert.FileExists(t, filepath.Join(dir, "config.json"))
	})

	t.Run("uses default dir when ConfigDir is empty", func(t *testing.T) {
		cfg := &Config{
			Name: "test-node",
			Role: RoleOwner,
		}

		// This will try to write to the default dir (~/.airgapper)
		// We don't want to pollute the real home dir, so we verify the ConfigDir gets set
		_ = cfg.Save()
		assert.NotEmpty(t, cfg.ConfigDir)
	})

	t.Run("file has correct permissions", func(t *testing.T) {
		dir := createTempConfigDir(t)
		cfg := &Config{
			Name:      "test-node",
			ConfigDir: dir,
		}

		err := cfg.Save()
		require.NoError(t, err)

		info, err := os.Stat(filepath.Join(dir, "config.json"))
		require.NoError(t, err)
		// Check file is not world-readable (0600)
		perm := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0600), perm)
	})
}

// --- Role method tests ---

func TestRoleMethods(t *testing.T) {
	t.Run("IsOwner returns true for owner role", func(t *testing.T) {
		cfg := &Config{Role: RoleOwner}
		assert.True(t, cfg.IsOwner())
		assert.False(t, cfg.IsHost())
	})

	t.Run("IsHost returns true for host role", func(t *testing.T) {
		cfg := &Config{Role: RoleHost}
		assert.False(t, cfg.IsOwner())
		assert.True(t, cfg.IsHost())
	})

	t.Run("both return false for empty role", func(t *testing.T) {
		cfg := &Config{}
		assert.False(t, cfg.IsOwner())
		assert.False(t, cfg.IsHost())
	})
}

// --- Share method tests ---

func TestSharePath(t *testing.T) {
	cfg := &Config{ConfigDir: "/home/user/.airgapper"}
	assert.Equal(t, "/home/user/.airgapper/share.key", cfg.SharePath())
}

func TestSaveShare(t *testing.T) {
	t.Run("saves share to config", func(t *testing.T) {
		dir := createTempConfigDir(t)
		cfg := &Config{
			Name:      "test",
			ConfigDir: dir,
		}
		err := cfg.Save()
		require.NoError(t, err)

		share := []byte{10, 20, 30, 40}
		err = cfg.SaveShare(share, 2)
		require.NoError(t, err)

		assert.Equal(t, share, cfg.LocalShare)
		assert.Equal(t, byte(2), cfg.ShareIndex)

		// Verify it was persisted
		loaded, err := Load(dir)
		require.NoError(t, err)
		assert.Equal(t, share, loaded.LocalShare)
		assert.Equal(t, byte(2), loaded.ShareIndex)
	})
}

func TestLoadShare(t *testing.T) {
	t.Run("returns share when present", func(t *testing.T) {
		cfg := &Config{
			LocalShare: []byte{1, 2, 3, 4},
			ShareIndex: 1,
		}

		share, idx, err := cfg.LoadShare()
		require.NoError(t, err)
		assert.Equal(t, []byte{1, 2, 3, 4}, share)
		assert.Equal(t, byte(1), idx)
	})

	t.Run("returns error when no share", func(t *testing.T) {
		cfg := &Config{}

		share, idx, err := cfg.LoadShare()
		assert.Nil(t, share)
		assert.Equal(t, byte(0), idx)
		assert.ErrorIs(t, err, apperrors.ErrNoLocalShare)
	})
}

// --- Schedule method tests ---

func TestSetSchedule(t *testing.T) {
	t.Run("sets schedule and paths", func(t *testing.T) {
		dir := createTempConfigDir(t)
		cfg := &Config{
			Name:      "test",
			ConfigDir: dir,
		}
		err := cfg.Save()
		require.NoError(t, err)

		err = cfg.SetSchedule("0 2 * * *", []string{"/path/one", "/path/two"})
		require.NoError(t, err)

		assert.Equal(t, "0 2 * * *", cfg.BackupSchedule)
		assert.Equal(t, []string{"/path/one", "/path/two"}, cfg.BackupPaths)

		// Verify persistence
		loaded, err := Load(dir)
		require.NoError(t, err)
		assert.Equal(t, "0 2 * * *", loaded.BackupSchedule)
		assert.Equal(t, []string{"/path/one", "/path/two"}, loaded.BackupPaths)
	})

	t.Run("keeps existing paths when none provided", func(t *testing.T) {
		dir := createTempConfigDir(t)
		cfg := &Config{
			Name:        "test",
			ConfigDir:   dir,
			BackupPaths: []string{"/existing/path"},
		}
		err := cfg.Save()
		require.NoError(t, err)

		err = cfg.SetSchedule("daily", nil)
		require.NoError(t, err)

		assert.Equal(t, "daily", cfg.BackupSchedule)
		assert.Equal(t, []string{"/existing/path"}, cfg.BackupPaths)
	})
}

// --- Mode detection tests ---

func TestModeDetection(t *testing.T) {
	t.Run("UsesSSSMode with local share and no consensus", func(t *testing.T) {
		cfg := &Config{
			LocalShare: []byte{1, 2, 3},
		}
		assert.True(t, cfg.UsesSSSMode())
		assert.False(t, cfg.UsesConsensusMode())
	})

	t.Run("UsesConsensusMode with consensus configured", func(t *testing.T) {
		cfg := &Config{
			Consensus: &ConsensusConfig{
				Threshold: 2,
				TotalKeys: 3,
			},
		}
		assert.False(t, cfg.UsesSSSMode())
		assert.True(t, cfg.UsesConsensusMode())
	})

	t.Run("neither mode when no share and no consensus", func(t *testing.T) {
		cfg := &Config{}
		assert.False(t, cfg.UsesSSSMode())
		assert.False(t, cfg.UsesConsensusMode())
	})

	t.Run("consensus takes precedence over SSS", func(t *testing.T) {
		cfg := &Config{
			LocalShare: []byte{1, 2, 3},
			Consensus: &ConsensusConfig{
				Threshold: 2,
				TotalKeys: 3,
			},
		}
		// Consensus mode wins when both are set
		assert.False(t, cfg.UsesSSSMode())
		assert.True(t, cfg.UsesConsensusMode())
	})
}

// --- Consensus method tests ---

func TestAddKeyHolder(t *testing.T) {
	t.Run("adds key holder successfully", func(t *testing.T) {
		dir := createTempConfigDir(t)
		cfg := &Config{
			Name:      "test",
			ConfigDir: dir,
			Consensus: &ConsensusConfig{
				Threshold: 2,
				TotalKeys: 3,
			},
		}
		err := cfg.Save()
		require.NoError(t, err)

		holder := KeyHolder{
			ID:        "holder1",
			Name:      "Alice",
			PublicKey: []byte{1, 2, 3},
			JoinedAt:  time.Now().UTC(),
			IsOwner:   true,
		}

		err = cfg.AddKeyHolder(holder)
		require.NoError(t, err)

		assert.Len(t, cfg.Consensus.KeyHolders, 1)
		assert.Equal(t, "holder1", cfg.Consensus.KeyHolders[0].ID)
	})

	t.Run("returns error when consensus not configured", func(t *testing.T) {
		cfg := &Config{Name: "test"}

		holder := KeyHolder{ID: "holder1", Name: "Alice"}
		err := cfg.AddKeyHolder(holder)

		assert.ErrorIs(t, err, apperrors.ErrConsensusNotConfigured)
	})

	t.Run("returns error for duplicate key holder", func(t *testing.T) {
		dir := createTempConfigDir(t)
		cfg := &Config{
			Name:      "test",
			ConfigDir: dir,
			Consensus: &ConsensusConfig{
				Threshold: 2,
				TotalKeys: 3,
				KeyHolders: []KeyHolder{
					{ID: "holder1", Name: "Alice"},
				},
			},
		}
		err := cfg.Save()
		require.NoError(t, err)

		holder := KeyHolder{ID: "holder1", Name: "Alice Duplicate"}
		err = cfg.AddKeyHolder(holder)

		assert.ErrorIs(t, err, apperrors.ErrKeyHolderExists)
	})
}

func TestGetKeyHolder(t *testing.T) {
	t.Run("returns key holder when found", func(t *testing.T) {
		cfg := &Config{
			Consensus: &ConsensusConfig{
				KeyHolders: []KeyHolder{
					{ID: "holder1", Name: "Alice"},
					{ID: "holder2", Name: "Bob"},
				},
			},
		}

		holder := cfg.GetKeyHolder("holder2")
		require.NotNil(t, holder)
		assert.Equal(t, "Bob", holder.Name)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		cfg := &Config{
			Consensus: &ConsensusConfig{
				KeyHolders: []KeyHolder{
					{ID: "holder1", Name: "Alice"},
				},
			},
		}

		holder := cfg.GetKeyHolder("nonexistent")
		assert.Nil(t, holder)
	})

	t.Run("returns nil when consensus not configured", func(t *testing.T) {
		cfg := &Config{}
		holder := cfg.GetKeyHolder("holder1")
		assert.Nil(t, holder)
	})
}

func TestCanRestoreDirectly(t *testing.T) {
	t.Run("returns true for 1-of-1 without approval", func(t *testing.T) {
		cfg := &Config{
			Consensus: &ConsensusConfig{
				Threshold:       1,
				TotalKeys:       1,
				RequireApproval: false,
			},
		}
		assert.True(t, cfg.CanRestoreDirectly())
	})

	t.Run("returns false when approval required", func(t *testing.T) {
		cfg := &Config{
			Consensus: &ConsensusConfig{
				Threshold:       1,
				TotalKeys:       1,
				RequireApproval: true,
			},
		}
		assert.False(t, cfg.CanRestoreDirectly())
	})

	t.Run("returns false for m-of-n where m > 1", func(t *testing.T) {
		cfg := &Config{
			Consensus: &ConsensusConfig{
				Threshold:       2,
				TotalKeys:       3,
				RequireApproval: false,
			},
		}
		assert.False(t, cfg.CanRestoreDirectly())
	})

	t.Run("returns false when consensus not configured", func(t *testing.T) {
		cfg := &Config{}
		assert.False(t, cfg.CanRestoreDirectly())
	})
}

func TestRequiredApprovals(t *testing.T) {
	t.Run("returns threshold from consensus config", func(t *testing.T) {
		cfg := &Config{
			Consensus: &ConsensusConfig{
				Threshold: 3,
				TotalKeys: 5,
			},
		}
		assert.Equal(t, 3, cfg.RequiredApprovals())
	})

	t.Run("returns 2 for legacy SSS mode", func(t *testing.T) {
		cfg := &Config{
			LocalShare: []byte{1, 2, 3},
		}
		assert.Equal(t, 2, cfg.RequiredApprovals())
	})
}

// --- Emergency config tests ---

func TestHasEmergencyConfig(t *testing.T) {
	t.Run("returns true when emergency configured", func(t *testing.T) {
		cfg := &Config{
			Emergency: &emergency.Config{},
		}
		assert.True(t, cfg.HasEmergencyConfig())
	})

	t.Run("returns false when emergency not configured", func(t *testing.T) {
		cfg := &Config{}
		assert.False(t, cfg.HasEmergencyConfig())
	})
}

func TestEnsureEmergency(t *testing.T) {
	t.Run("creates emergency config if nil", func(t *testing.T) {
		cfg := &Config{}
		assert.Nil(t, cfg.Emergency)

		em := cfg.EnsureEmergency()
		require.NotNil(t, em)
		assert.NotNil(t, cfg.Emergency)
		assert.Same(t, em, cfg.Emergency)
	})

	t.Run("returns existing emergency config", func(t *testing.T) {
		existing := emergency.NewConfig()
		cfg := &Config{Emergency: existing}

		em := cfg.EnsureEmergency()
		assert.Same(t, existing, em)
	})
}

// --- Role constants tests ---

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, Role("owner"), RoleOwner)
	assert.Equal(t, Role("host"), RoleHost)
}

// --- Full round-trip test ---

func TestConfigRoundTrip(t *testing.T) {
	dir := createTempConfigDir(t)

	original := &Config{
		Name:       "full-test",
		Role:       RoleOwner,
		RepoURL:    "rest:http://localhost:8000/repo1",
		RepoID:     "abc123",
		Password:   "secret-password",
		LocalShare: []byte{1, 2, 3, 4, 5},
		ShareIndex: 1,
		PublicKey:  []byte{10, 20, 30},
		PrivateKey: []byte{40, 50, 60},
		Consensus: &ConsensusConfig{
			Threshold: 2,
			TotalKeys: 3,
			KeyHolders: []KeyHolder{
				{
					ID:        "kh1",
					Name:      "Key Holder 1",
					PublicKey: []byte{7, 8, 9},
					Address:   "192.168.1.100:8081",
					JoinedAt:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					IsOwner:   true,
				},
			},
			RequireApproval: true,
		},
		Peer: &PeerInfo{
			Name:      "Peer Node",
			PublicKey: []byte{11, 12, 13},
			Address:   "192.168.1.200:8081",
		},
		ListenAddr:         ":9000",
		BackupPaths:        []string{"/path/a", "/path/b"},
		BackupSchedule:     "0 3 * * *",
		BackupExclude:      []string{"*.tmp"},
		AllowedBrowseRoots: []string{"/home", "/data"},
		StoragePath:        "/var/storage",
		StorageQuotaBytes:  1073741824,
		StorageAppendOnly:  true,
		StoragePort:        8080,
		ConfigDir:          dir,
	}

	// Save
	err := original.Save()
	require.NoError(t, err)

	// Load
	loaded, err := Load(dir)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, original.Name, loaded.Name)
	assert.Equal(t, original.Role, loaded.Role)
	assert.Equal(t, original.RepoURL, loaded.RepoURL)
	assert.Equal(t, original.RepoID, loaded.RepoID)
	assert.Equal(t, original.Password, loaded.Password)
	assert.Equal(t, original.LocalShare, loaded.LocalShare)
	assert.Equal(t, original.ShareIndex, loaded.ShareIndex)
	assert.Equal(t, original.PublicKey, loaded.PublicKey)
	assert.Equal(t, original.PrivateKey, loaded.PrivateKey)
	assert.Equal(t, original.ListenAddr, loaded.ListenAddr)
	assert.Equal(t, original.BackupPaths, loaded.BackupPaths)
	assert.Equal(t, original.BackupSchedule, loaded.BackupSchedule)
	assert.Equal(t, original.BackupExclude, loaded.BackupExclude)
	assert.Equal(t, original.AllowedBrowseRoots, loaded.AllowedBrowseRoots)
	assert.Equal(t, original.StoragePath, loaded.StoragePath)
	assert.Equal(t, original.StorageQuotaBytes, loaded.StorageQuotaBytes)
	assert.Equal(t, original.StorageAppendOnly, loaded.StorageAppendOnly)
	assert.Equal(t, original.StoragePort, loaded.StoragePort)

	// Verify nested structs
	require.NotNil(t, loaded.Consensus)
	assert.Equal(t, original.Consensus.Threshold, loaded.Consensus.Threshold)
	assert.Equal(t, original.Consensus.TotalKeys, loaded.Consensus.TotalKeys)
	assert.Equal(t, original.Consensus.RequireApproval, loaded.Consensus.RequireApproval)
	require.Len(t, loaded.Consensus.KeyHolders, 1)
	assert.Equal(t, original.Consensus.KeyHolders[0].ID, loaded.Consensus.KeyHolders[0].ID)
	assert.Equal(t, original.Consensus.KeyHolders[0].Name, loaded.Consensus.KeyHolders[0].Name)
	assert.Equal(t, original.Consensus.KeyHolders[0].IsOwner, loaded.Consensus.KeyHolders[0].IsOwner)

	require.NotNil(t, loaded.Peer)
	assert.Equal(t, original.Peer.Name, loaded.Peer.Name)
	assert.Equal(t, original.Peer.PublicKey, loaded.Peer.PublicKey)
	assert.Equal(t, original.Peer.Address, loaded.Peer.Address)
}
