// Package config manages Airgapper configuration
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config represents the Airgapper configuration
type Config struct {
	// Identity
	Name       string `json:"name"`        // Human-readable name for this node
	PublicKey  []byte `json:"public_key"`  // Ed25519 public key
	PrivateKey []byte `json:"private_key"` // Ed25519 private key (encrypted at rest)

	// Repository
	RepoURL    string `json:"repo_url"`    // Restic repository URL (e.g., rest:http://peer:8000/)
	RepoID     string `json:"repo_id"`     // Unique repo identifier

	// Key shares
	LocalShare  []byte `json:"local_share"`  // Our share of the repo password
	ShareIndex  byte   `json:"share_index"`  // Our share index (1 or 2)

	// Peer info
	Peer *PeerInfo `json:"peer,omitempty"`

	// Paths
	ConfigDir string `json:"-"` // Not serialized, set at runtime
}

// PeerInfo represents information about the other party
type PeerInfo struct {
	Name      string `json:"name"`
	PublicKey []byte `json:"public_key"`
	Address   string `json:"address"` // Network address for communication
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
