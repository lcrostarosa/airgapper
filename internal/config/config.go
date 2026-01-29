// Package config manages Airgapper configuration
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Role defines the role of this node
type Role string

const (
	RoleOwner Role = "owner" // Data owner (Alice) - creates backups
	RoleHost  Role = "host"  // Backup host (Bob) - stores data, approves restores
)

// Config represents the Airgapper configuration
type Config struct {
	// Identity
	Name       string `json:"name"`                  // Human-readable name for this node
	Role       Role   `json:"role"`                  // owner or host
	PublicKey  []byte `json:"public_key,omitempty"`  // Ed25519 public key
	PrivateKey []byte `json:"private_key,omitempty"` // Ed25519 private key (encrypted at rest)

	// Repository
	RepoURL  string `json:"repo_url"`           // Restic repository URL (e.g., rest:http://peer:8000/)
	RepoID   string `json:"repo_id,omitempty"`  // Unique repo identifier
	Password string `json:"password,omitempty"` // Full repo password (only for owner, used for backup)

	// Key shares (for restore consensus)
	LocalShare []byte `json:"local_share,omitempty"` // Our share of the repo password
	ShareIndex byte   `json:"share_index,omitempty"` // Our share index (1 or 2)

	// Peer info
	Peer *PeerInfo `json:"peer,omitempty"`

	// API settings
	ListenAddr string `json:"listen_addr,omitempty"` // Address for HTTP API (e.g., :8080)

	// Backup settings (owner only)
	BackupPaths    []string `json:"backup_paths,omitempty"`    // Paths to back up
	BackupSchedule string   `json:"backup_schedule,omitempty"` // Schedule expression (cron or simple)
	BackupExclude  []string `json:"backup_exclude,omitempty"`  // Patterns to exclude

	// Paths
	ConfigDir string `json:"-"` // Not serialized, set at runtime
}

// PeerInfo represents information about the other party
type PeerInfo struct {
	Name      string `json:"name"`
	PublicKey []byte `json:"public_key,omitempty"`
	Address   string `json:"address,omitempty"` // Network address for communication
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
			return nil, errors.New("airgapper not initialized - run 'airgapper init' first")
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

	// Ensure directory exists
	if err := os.MkdirAll(c.ConfigDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(c.ConfigDir, "config.json")
	return os.WriteFile(configPath, data, 0600)
}

// SharePath returns the path to store/load key shares
func (c *Config) SharePath() string {
	return filepath.Join(c.ConfigDir, "share.key")
}

// SaveShare saves the local key share to disk
func (c *Config) SaveShare(share []byte, index byte) error {
	c.LocalShare = share
	c.ShareIndex = index
	return c.Save()
}

// LoadShare loads the local key share
func (c *Config) LoadShare() ([]byte, byte, error) {
	if c.LocalShare == nil {
		return nil, 0, errors.New("no local share found")
	}
	return c.LocalShare, c.ShareIndex, nil
}

// IsOwner returns true if this node is the data owner
func (c *Config) IsOwner() bool {
	return c.Role == RoleOwner
}

// IsHost returns true if this node is the backup host
func (c *Config) IsHost() bool {
	return c.Role == RoleHost
}

// SetSchedule sets the backup schedule
func (c *Config) SetSchedule(schedule string, paths []string) error {
	c.BackupSchedule = schedule
	if len(paths) > 0 {
		c.BackupPaths = paths
	}
	return c.Save()
}
