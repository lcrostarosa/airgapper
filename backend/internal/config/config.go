// Package config manages Airgapper configuration
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/emergency"
	apperrors "github.com/lcrostarosa/airgapper/backend/internal/errors"
	"github.com/lcrostarosa/airgapper/backend/internal/filelock"
	"github.com/lcrostarosa/airgapper/backend/internal/verification"
)

// Role defines the role of this node
type Role string

const (
	RoleOwner Role = "owner" // Data owner (Alice) - creates backups
	RoleHost  Role = "host"  // Backup host (Bob) - stores data, approves restores
)

// KeyHolder represents a participant in the consensus scheme
type KeyHolder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PublicKey []byte    `json:"public_key"`
	Address   string    `json:"address,omitempty"`
	JoinedAt  time.Time `json:"joined_at"`
	IsOwner   bool      `json:"is_owner,omitempty"`
}

// ConsensusConfig defines the m-of-n approval requirements
type ConsensusConfig struct {
	Threshold       int         `json:"threshold"`
	TotalKeys       int         `json:"total_keys"`
	KeyHolders      []KeyHolder `json:"key_holders"`
	RequireApproval bool        `json:"require_approval,omitempty"`
}

// PeerInfo represents information about the other party
type PeerInfo struct {
	Name      string `json:"name"`
	PublicKey []byte `json:"public_key,omitempty"`
	Address   string `json:"address,omitempty"`
}

// Config represents the Airgapper configuration
type Config struct {
	// Identity
	Name       string `json:"name"`
	Role       Role   `json:"role"`
	PublicKey  []byte `json:"public_key,omitempty"`
	PrivateKey []byte `json:"private_key,omitempty"`

	// Repository
	RepoURL  string `json:"repo_url"`
	RepoID   string `json:"repo_id,omitempty"`
	Password string `json:"password,omitempty"`

	// Key shares (for restore consensus - legacy SSS mode)
	LocalShare []byte `json:"local_share,omitempty"`
	ShareIndex byte   `json:"share_index,omitempty"`

	// Consensus configuration (new m-of-n mode)
	Consensus *ConsensusConfig `json:"consensus,omitempty"`

	// Peer info (legacy - for 2-of-2 SSS mode)
	Peer *PeerInfo `json:"peer,omitempty"`

	// API settings
	ListenAddr string `json:"listen_addr,omitempty"`
	APIKey     string `json:"api_key,omitempty"`
	DevMode    bool   `json:"dev_mode,omitempty"` // Disables auth for development

	// Backup settings (owner only)
	BackupPaths    []string `json:"backup_paths,omitempty"`
	BackupSchedule string   `json:"backup_schedule,omitempty"`
	BackupExclude  []string `json:"backup_exclude,omitempty"`

	// Filesystem browsing security
	AllowedBrowseRoots []string `json:"allowed_browse_roots,omitempty"`

	// Storage server settings (host only)
	StoragePath       string `json:"storage_path,omitempty"`
	StorageQuotaBytes int64  `json:"storage_quota_bytes,omitempty"`
	StorageAppendOnly bool   `json:"storage_append_only,omitempty"`
	StoragePort       int    `json:"storage_port,omitempty"`

	// Emergency recovery settings (uses emergency package types)
	Emergency *emergency.Config `json:"emergency,omitempty"`

	// Host verification settings (uses verification package types)
	Verification *verification.VerificationSystemConfig `json:"verification,omitempty"`

	// TLS settings
	TLSCertPath string `json:"tls_cert_path,omitempty"`
	TLSKeyPath  string `json:"tls_key_path,omitempty"`

	// Encrypted secrets (when encryption is enabled)
	EncryptedSecrets *crypto.EncryptedSecrets `json:"encrypted_secrets,omitempty"`

	// Paths (not serialized)
	ConfigDir string `json:"-"`

	// Runtime fields (not serialized)
	encryptionPassphrase string `json:"-"` // Used for encryption/decryption
}

// DefaultConfigDir returns the default config directory
func DefaultConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".airgapper")
}

// Load loads configuration from the config directory
func Load(configDir string) (*Config, error) {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}

	configPath := filepath.Join(configDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperrors.ErrNotInitialized
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.ConfigDir = configDir
	return &cfg, nil
}

// Exists checks if a config exists
func Exists(configDir string) bool {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}
	configPath := filepath.Join(configDir, "config.json")
	_, err := os.Stat(configPath)
	return err == nil
}

// Save saves the configuration to disk
func (c *Config) Save() error {
	if c.ConfigDir == "" {
		c.ConfigDir = DefaultConfigDir()
	}

	if err := os.MkdirAll(c.ConfigDir, 0700); err != nil {
		return err
	}

	configPath := filepath.Join(c.ConfigDir, "config.json")

	// Use file locking to prevent race conditions
	lock := filelock.New(configPath)
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

// --- Role methods ---

func (c *Config) IsOwner() bool { return c.Role == RoleOwner }
func (c *Config) IsHost() bool  { return c.Role == RoleHost }

// --- Share methods ---

func (c *Config) SharePath() string {
	return filepath.Join(c.ConfigDir, "share.key")
}

func (c *Config) SaveShare(share []byte, index byte) error {
	c.LocalShare = share
	c.ShareIndex = index
	return c.Save()
}

func (c *Config) LoadShare() ([]byte, byte, error) {
	if c.LocalShare == nil {
		return nil, 0, apperrors.ErrNoLocalShare
	}
	return c.LocalShare, c.ShareIndex, nil
}

// --- Schedule methods ---

func (c *Config) SetSchedule(schedule string, paths []string) error {
	c.BackupSchedule = schedule
	if len(paths) > 0 {
		c.BackupPaths = paths
	}
	return c.Save()
}

// --- Mode detection ---

func (c *Config) UsesSSSMode() bool       { return c.Consensus == nil && c.LocalShare != nil }
func (c *Config) UsesConsensusMode() bool { return c.Consensus != nil }

// --- Consensus methods ---

func (c *Config) AddKeyHolder(holder KeyHolder) error {
	if c.Consensus == nil {
		return apperrors.ErrConsensusNotConfigured
	}

	for _, kh := range c.Consensus.KeyHolders {
		if kh.ID == holder.ID {
			return apperrors.ErrKeyHolderExists
		}
	}

	c.Consensus.KeyHolders = append(c.Consensus.KeyHolders, holder)
	return c.Save()
}

func (c *Config) GetKeyHolder(id string) *KeyHolder {
	if c.Consensus == nil {
		return nil
	}
	for i := range c.Consensus.KeyHolders {
		if c.Consensus.KeyHolders[i].ID == id {
			return &c.Consensus.KeyHolders[i]
		}
	}
	return nil
}

func (c *Config) CanRestoreDirectly() bool {
	if c.Consensus == nil {
		return false
	}
	return c.Consensus.Threshold == 1 &&
		c.Consensus.TotalKeys == 1 &&
		!c.Consensus.RequireApproval
}

func (c *Config) RequiredApprovals() int {
	if c.Consensus == nil {
		return 2 // Legacy SSS mode
	}
	return c.Consensus.Threshold
}

// HasEmergencyConfig returns true if any emergency features are configured
func (c *Config) HasEmergencyConfig() bool {
	return c.Emergency != nil
}

// EnsureEmergency ensures Emergency config exists
func (c *Config) EnsureEmergency() *emergency.Config {
	if c.Emergency == nil {
		c.Emergency = emergency.NewConfig()
	}
	return c.Emergency
}

// --- Encryption methods ---

// SetEncryptionPassphrase sets the passphrase used for config encryption
func (c *Config) SetEncryptionPassphrase(passphrase string) {
	c.encryptionPassphrase = passphrase
}

// IsEncrypted returns true if sensitive fields are encrypted
func (c *Config) IsEncrypted() bool {
	return crypto.IsEncrypted(c.EncryptedSecrets)
}

// EncryptSecrets encrypts sensitive config fields
// Requires SetEncryptionPassphrase to be called first
func (c *Config) EncryptSecrets() error {
	if c.encryptionPassphrase == "" {
		return apperrors.ErrNoEncryptionPassphrase
	}

	secrets := &crypto.EncryptedSecrets{}

	// Encrypt password
	if c.Password != "" {
		enc, err := crypto.EncryptString(c.Password, c.encryptionPassphrase)
		if err != nil {
			return err
		}
		secrets.Password = enc
		c.Password = "" // Clear plaintext
	}

	// Encrypt private key
	if len(c.PrivateKey) > 0 {
		enc, err := crypto.Encrypt(c.PrivateKey, c.encryptionPassphrase)
		if err != nil {
			return err
		}
		secrets.PrivateKey = enc
		c.PrivateKey = nil // Clear plaintext
	}

	// Encrypt local share
	if len(c.LocalShare) > 0 {
		enc, err := crypto.Encrypt(c.LocalShare, c.encryptionPassphrase)
		if err != nil {
			return err
		}
		secrets.LocalShare = enc
		c.LocalShare = nil // Clear plaintext
	}

	// Encrypt API key
	if c.APIKey != "" {
		enc, err := crypto.EncryptString(c.APIKey, c.encryptionPassphrase)
		if err != nil {
			return err
		}
		secrets.APIKey = enc
		c.APIKey = "" // Clear plaintext
	}

	c.EncryptedSecrets = secrets
	return nil
}

// DecryptSecrets decrypts sensitive config fields
// Requires SetEncryptionPassphrase to be called first
func (c *Config) DecryptSecrets() error {
	if c.EncryptedSecrets == nil {
		return nil // Nothing to decrypt
	}

	if c.encryptionPassphrase == "" {
		return apperrors.ErrNoEncryptionPassphrase
	}

	// Decrypt password
	if c.EncryptedSecrets.Password != nil {
		password, err := crypto.DecryptString(c.EncryptedSecrets.Password, c.encryptionPassphrase)
		if err != nil {
			return err
		}
		c.Password = password
	}

	// Decrypt private key
	if c.EncryptedSecrets.PrivateKey != nil {
		privateKey, err := crypto.Decrypt(c.EncryptedSecrets.PrivateKey, c.encryptionPassphrase)
		if err != nil {
			return err
		}
		c.PrivateKey = privateKey
	}

	// Decrypt local share
	if c.EncryptedSecrets.LocalShare != nil {
		localShare, err := crypto.Decrypt(c.EncryptedSecrets.LocalShare, c.encryptionPassphrase)
		if err != nil {
			return err
		}
		c.LocalShare = localShare
	}

	// Decrypt API key
	if c.EncryptedSecrets.APIKey != nil {
		apiKey, err := crypto.DecryptString(c.EncryptedSecrets.APIKey, c.encryptionPassphrase)
		if err != nil {
			return err
		}
		c.APIKey = apiKey
	}

	return nil
}

// LoadWithPassphrase loads configuration and decrypts secrets if encrypted
func LoadWithPassphrase(configDir, passphrase string) (*Config, error) {
	cfg, err := Load(configDir)
	if err != nil {
		return nil, err
	}

	if cfg.IsEncrypted() {
		if passphrase == "" {
			return nil, apperrors.ErrConfigEncrypted
		}
		cfg.SetEncryptionPassphrase(passphrase)
		if err := cfg.DecryptSecrets(); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// SaveEncrypted saves the configuration with encrypted secrets
func (c *Config) SaveEncrypted() error {
	if c.encryptionPassphrase == "" {
		return c.Save() // Fall back to unencrypted save
	}

	// Encrypt secrets before saving
	if err := c.EncryptSecrets(); err != nil {
		return err
	}

	return c.Save()
}
