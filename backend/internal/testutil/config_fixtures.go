package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// ConfigFixture represents a test configuration setup
type ConfigFixture struct {
	// Dir is the config directory path
	Dir string
	// Config is the loaded configuration
	Config *config.Config
	// OwnerKey is the owner's key fixture (if applicable)
	OwnerKey *CryptoKeyFixture
	// HostKey is the host's key fixture (if applicable)
	HostKey *CryptoKeyFixture
}

// ConfigFixtureBuilder constructs config fixtures
type ConfigFixtureBuilder struct {
	t            *testing.T
	dir          string
	role         config.Role
	name         string
	repoURL      string
	password     string
	localShare   []byte
	shareIndex   int
	peerName     string
	peerAddress  string
	backupPaths  []string
	storagePath  string
	useConsensus bool
	threshold    int
	totalKeys    int
	keyHolders   []*CryptoKeyFixture
	ownerKey     *CryptoKeyFixture
}

// NewConfigFixture starts building a config fixture
func NewConfigFixture(t *testing.T) *ConfigFixtureBuilder {
	return &ConfigFixtureBuilder{
		t:       t,
		dir:     t.TempDir(),
		role:    config.RoleOwner,
		name:    "test-node",
		repoURL: "rest:http://localhost:8000/",
	}
}

// WithDir sets the config directory
func (b *ConfigFixtureBuilder) WithDir(dir string) *ConfigFixtureBuilder {
	b.dir = dir
	return b
}

// AsOwner configures as data owner
func (b *ConfigFixtureBuilder) AsOwner() *ConfigFixtureBuilder {
	b.role = config.RoleOwner
	return b
}

// AsHost configures as backup host
func (b *ConfigFixtureBuilder) AsHost() *ConfigFixtureBuilder {
	b.role = config.RoleHost
	return b
}

// WithName sets the node name
func (b *ConfigFixtureBuilder) WithName(name string) *ConfigFixtureBuilder {
	b.name = name
	return b
}

// WithRepoURL sets the repository URL
func (b *ConfigFixtureBuilder) WithRepoURL(url string) *ConfigFixtureBuilder {
	b.repoURL = url
	return b
}

// WithPassword sets the backup password
func (b *ConfigFixtureBuilder) WithPassword(password string) *ConfigFixtureBuilder {
	b.password = password
	return b
}

// WithSSSShare sets the local SSS share
func (b *ConfigFixtureBuilder) WithSSSShare(data []byte, index int) *ConfigFixtureBuilder {
	b.localShare = data
	b.shareIndex = index
	return b
}

// WithPeer sets the peer information
func (b *ConfigFixtureBuilder) WithPeer(name, address string) *ConfigFixtureBuilder {
	b.peerName = name
	b.peerAddress = address
	return b
}

// WithBackupPaths sets paths to backup
func (b *ConfigFixtureBuilder) WithBackupPaths(paths ...string) *ConfigFixtureBuilder {
	b.backupPaths = paths
	return b
}

// WithStoragePath sets the storage server path
func (b *ConfigFixtureBuilder) WithStoragePath(path string) *ConfigFixtureBuilder {
	b.storagePath = path
	return b
}

// WithConsensusMode enables consensus mode
func (b *ConfigFixtureBuilder) WithConsensusMode(threshold, totalKeys int) *ConfigFixtureBuilder {
	b.useConsensus = true
	b.threshold = threshold
	b.totalKeys = totalKeys
	return b
}

// WithKeyHolders sets the consensus key holders
func (b *ConfigFixtureBuilder) WithKeyHolders(holders ...*CryptoKeyFixture) *ConfigFixtureBuilder {
	b.keyHolders = holders
	if len(holders) > 0 && b.role == config.RoleOwner {
		b.ownerKey = holders[0]
	}
	return b
}

// Build creates the config fixture, writing files to disk
func (b *ConfigFixtureBuilder) Build() (*ConfigFixture, error) {
	fixture := &ConfigFixture{
		Dir: b.dir,
	}

	// Build config map
	cfgMap := map[string]interface{}{
		"name":     b.name,
		"role":     string(b.role),
		"repo_url": b.repoURL,
	}

	if b.password != "" {
		cfgMap["password"] = b.password
	}

	if b.localShare != nil {
		cfgMap["local_share"] = b.localShare
		cfgMap["share_index"] = b.shareIndex
	}

	if b.peerName != "" {
		cfgMap["peer"] = map[string]interface{}{
			"name":    b.peerName,
			"address": b.peerAddress,
		}
	}

	if len(b.backupPaths) > 0 {
		cfgMap["backup_paths"] = b.backupPaths
	}

	if b.storagePath != "" {
		cfgMap["storage_path"] = b.storagePath
	}

	if b.useConsensus {
		// Generate keys if not provided
		if len(b.keyHolders) == 0 {
			holders, err := NewKeyHoldersFixture("owner", "host")
			if err != nil {
				return nil, err
			}
			b.keyHolders = holders.Holders
			b.ownerKey = holders.Holders[0]
		}

		if b.ownerKey == nil && len(b.keyHolders) > 0 {
			b.ownerKey = b.keyHolders[0]
		}

		fixture.OwnerKey = b.ownerKey
		if len(b.keyHolders) > 1 {
			fixture.HostKey = b.keyHolders[1]
		}

		keyHoldersList := make([]map[string]interface{}, len(b.keyHolders))
		for i, kh := range b.keyHolders {
			keyHoldersList[i] = map[string]interface{}{
				"id":         kh.KeyID,
				"name":       kh.Name,
				"public_key": kh.PublicKey,
				"is_owner":   i == 0,
				"joined_at":  time.Now().Add(-time.Duration(i) * 24 * time.Hour),
			}
			if i > 0 && b.peerAddress != "" {
				keyHoldersList[i]["address"] = b.peerAddress
			}
		}

		cfgMap["public_key"] = b.ownerKey.PublicKey
		cfgMap["consensus"] = map[string]interface{}{
			"threshold":   b.threshold,
			"total_keys":  b.totalKeys,
			"key_holders": keyHoldersList,
		}
	}

	// Write config file
	configPath := filepath.Join(b.dir, "config.json")
	data, err := json.MarshalIndent(cfgMap, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return nil, err
	}

	// Load config with current code
	cfg, err := config.Load(b.dir)
	if err != nil {
		return nil, err
	}
	fixture.Config = cfg

	return fixture, nil
}

// MustBuild creates the fixture or fails the test
func (b *ConfigFixtureBuilder) MustBuild() *ConfigFixture {
	f, err := b.Build()
	if err != nil {
		b.t.Fatalf("Failed to build config fixture: %v", err)
	}
	return f
}

// BuildLegacySSSConfig creates a legacy SSS-mode config fixture
func BuildLegacySSSConfig(t *testing.T, sssFixture *SSSFixture) *ConfigFixture {
	ownerDir := t.TempDir()
	hostDir := t.TempDir()

	// Owner config
	ownerCfg := map[string]interface{}{
		"name":        "owner",
		"role":        "owner",
		"repo_url":    "rest:http://host:8000/",
		"password":    string(sssFixture.Secret),
		"local_share": sssFixture.Shares[0].Data,
		"share_index": int(sssFixture.Shares[0].Index),
		"peer": map[string]interface{}{
			"name":    "host",
			"address": "localhost:8081",
		},
	}
	ownerData, _ := json.MarshalIndent(ownerCfg, "", "  ")
	_ = os.WriteFile(filepath.Join(ownerDir, "config.json"), ownerData, 0600)

	// Host config
	hostCfg := map[string]interface{}{
		"name":        "host",
		"role":        "host",
		"repo_url":    "rest:http://localhost:8000/",
		"local_share": sssFixture.Shares[1].Data,
		"share_index": int(sssFixture.Shares[1].Index),
		"peer": map[string]interface{}{
			"name":    "owner",
			"address": "192.168.1.50:8081",
		},
	}
	hostData, _ := json.MarshalIndent(hostCfg, "", "  ")
	_ = os.WriteFile(filepath.Join(hostDir, "config.json"), hostData, 0600)

	cfg, _ := config.Load(ownerDir)
	return &ConfigFixture{
		Dir:    ownerDir,
		Config: cfg,
	}
}

// BuildConsensusConfig creates a consensus-mode config fixture
func BuildConsensusConfig(t *testing.T, threshold int, holders *KeyHoldersFixture) *ConfigFixture {
	return NewConfigFixture(t).
		AsOwner().
		WithName("alice-consensus").
		WithPassword("consensuspassword").
		WithConsensusMode(threshold, len(holders.Holders)).
		WithKeyHolders(holders.Holders...).
		MustBuild()
}

// GenerateKeyPair wraps crypto.GenerateKeyPair for convenience
func GenerateKeyPair() ([]byte, []byte, error) {
	return crypto.GenerateKeyPair()
}

// KeyID wraps crypto.KeyID for convenience
func KeyID(publicKey []byte) string {
	return crypto.KeyID(publicKey)
}
