// Package service contains business logic separated from HTTP and data access concerns
package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
)

// VaultService handles vault-related business logic
type VaultService struct {
	cfg *config.Config
}

// NewVaultService creates a new vault service
func NewVaultService(cfg *config.Config) *VaultService {
	return &VaultService{cfg: cfg}
}

// InitParams contains parameters for initializing a vault
type InitParams struct {
	Name            string
	RepoURL         string
	Threshold       int
	TotalKeys       int
	BackupPaths     []string
	RequireApproval bool
}

// InitResult contains the result of vault initialization
type InitResult struct {
	Name      string
	KeyID     string
	PublicKey string
	Threshold int
	TotalKeys int
}

// Init initializes a new vault as the owner
func (s *VaultService) Init(params InitParams) (*InitResult, error) {
	if s.cfg.Name != "" {
		return nil, errors.New("vault already initialized")
	}

	// Generate owner's key pair
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	// Generate repository password
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return nil, err
	}
	password := hex.EncodeToString(passwordBytes)

	// Set up config
	s.cfg.Name = params.Name
	s.cfg.Role = config.RoleOwner
	s.cfg.RepoURL = params.RepoURL
	s.cfg.PublicKey = pubKey
	s.cfg.PrivateKey = privKey
	s.cfg.Password = password
	s.cfg.BackupPaths = params.BackupPaths

	// Set up consensus
	ownerKeyHolder := config.KeyHolder{
		ID:        crypto.KeyID(pubKey),
		Name:      params.Name,
		PublicKey: pubKey,
		JoinedAt:  time.Now(),
		IsOwner:   true,
	}

	s.cfg.Consensus = &config.ConsensusConfig{
		Threshold:       params.Threshold,
		TotalKeys:       params.TotalKeys,
		KeyHolders:      []config.KeyHolder{ownerKeyHolder},
		RequireApproval: params.RequireApproval,
	}

	if err := s.cfg.Save(); err != nil {
		return nil, err
	}

	return &InitResult{
		Name:      s.cfg.Name,
		KeyID:     ownerKeyHolder.ID,
		PublicKey: crypto.EncodePublicKey(pubKey),
		Threshold: params.Threshold,
		TotalKeys: params.TotalKeys,
	}, nil
}

// RegisterKeyHolderParams contains parameters for registering a key holder
type RegisterKeyHolderParams struct {
	Name      string
	PublicKey string // Hex encoded
	Address   string
}

// RegisterKeyHolderResult contains the result of registration
type RegisterKeyHolderResult struct {
	ID       string
	Name     string
	JoinedAt time.Time
}

// RegisterKeyHolder adds a new key holder to the consensus scheme
func (s *VaultService) RegisterKeyHolder(params RegisterKeyHolderParams) (*RegisterKeyHolderResult, error) {
	if s.cfg.Consensus == nil {
		return nil, errors.New("consensus mode not configured")
	}

	// Decode public key
	pubKey, err := crypto.DecodePublicKey(params.PublicKey)
	if err != nil {
		return nil, err
	}

	// Check capacity
	if len(s.cfg.Consensus.KeyHolders) >= s.cfg.Consensus.TotalKeys {
		return nil, errors.New("maximum number of key holders reached")
	}

	// Create key holder
	holder := config.KeyHolder{
		ID:        crypto.KeyID(pubKey),
		Name:      params.Name,
		PublicKey: pubKey,
		Address:   params.Address,
		JoinedAt:  time.Now(),
		IsOwner:   false,
	}

	if err := s.cfg.AddKeyHolder(holder); err != nil {
		return nil, err
	}

	return &RegisterKeyHolderResult{
		ID:       holder.ID,
		Name:     holder.Name,
		JoinedAt: holder.JoinedAt,
	}, nil
}

// GetKeyHolders returns all registered key holders
func (s *VaultService) GetKeyHolders() ([]config.KeyHolder, error) {
	if s.cfg.Consensus == nil {
		return nil, errors.New("consensus mode not configured")
	}
	return s.cfg.Consensus.KeyHolders, nil
}

// GetKeyHolder returns a specific key holder by ID
func (s *VaultService) GetKeyHolder(id string) (*config.KeyHolder, error) {
	holder := s.cfg.GetKeyHolder(id)
	if holder == nil {
		return nil, errors.New("key holder not found")
	}
	return holder, nil
}

// ConsensusInfo returns consensus configuration
type ConsensusInfo struct {
	Threshold       int
	TotalKeys       int
	KeyHolders      []config.KeyHolder
	RequireApproval bool
}

// GetConsensusInfo returns consensus configuration
func (s *VaultService) GetConsensusInfo() (*ConsensusInfo, error) {
	if s.cfg.Consensus == nil {
		return nil, errors.New("consensus mode not configured")
	}
	return &ConsensusInfo{
		Threshold:       s.cfg.Consensus.Threshold,
		TotalKeys:       s.cfg.Consensus.TotalKeys,
		KeyHolders:      s.cfg.Consensus.KeyHolders,
		RequireApproval: s.cfg.Consensus.RequireApproval,
	}, nil
}

// HasConsensus returns true if consensus mode is configured
func (s *VaultService) HasConsensus() bool {
	return s.cfg.Consensus != nil
}

// IsInitialized returns true if the vault is initialized
func (s *VaultService) IsInitialized() bool {
	return s.cfg.Name != ""
}
