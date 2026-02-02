package service

import (
	"errors"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// HostService handles host-related business logic
type HostService struct {
	cfg           *config.Config
	storageServer *storage.Server
}

// NewHostService creates a new host service
func NewHostService(cfg *config.Config) *HostService {
	return &HostService{cfg: cfg}
}

// SetStorageServer sets the storage server reference
func (s *HostService) SetStorageServer(ss *storage.Server) {
	s.storageServer = ss
}

// InitParams contains parameters for initializing a host
type HostInitParams struct {
	Name          string
	StoragePath   string
	StorageQuota  int64
	AppendOnly    bool
}

// InitResult contains the result of host initialization
type HostInitResult struct {
	Name        string
	KeyID       string
	PublicKey   string
	StoragePath string
}

// Init initializes this node as a backup host
func (s *HostService) Init(params HostInitParams) (*HostInitResult, error) {
	if s.cfg.Name != "" {
		return nil, errors.New("host already initialized")
	}

	// Generate host's key pair
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	// Set up config
	s.cfg.Name = params.Name
	s.cfg.Role = config.RoleHost
	s.cfg.PublicKey = pubKey
	s.cfg.PrivateKey = privKey
	s.cfg.StoragePath = params.StoragePath
	s.cfg.StorageQuotaBytes = params.StorageQuota
	s.cfg.StorageAppendOnly = params.AppendOnly

	if err := s.cfg.Save(); err != nil {
		return nil, err
	}

	return &HostInitResult{
		Name:        s.cfg.Name,
		KeyID:       crypto.KeyID(pubKey),
		PublicKey:   crypto.EncodePublicKey(pubKey),
		StoragePath: params.StoragePath,
	}, nil
}

// StorageStatus represents the current storage server status
type StorageStatus struct {
	Configured     bool
	Running        bool
	BasePath       string
	AppendOnly     bool
	QuotaBytes     int64
	UsedBytes      int64
	RequestCount   int64
	HasPolicy      bool
	PolicyID       string
	DiskUsagePct   int
	DiskFreeBytes  int64
	DiskTotalBytes int64
}

// GetStorageStatus returns the current storage server status
func (s *HostService) GetStorageStatus() StorageStatus {
	if s.storageServer == nil {
		return StorageStatus{Configured: false, Running: false}
	}

	status := s.storageServer.Status()
	return StorageStatus{
		Configured:     true,
		Running:        status.Running,
		BasePath:       status.BasePath,
		AppendOnly:     status.AppendOnly,
		QuotaBytes:     status.QuotaBytes,
		UsedBytes:      status.UsedBytes,
		RequestCount:   status.RequestCount,
		HasPolicy:      status.HasPolicy,
		PolicyID:       status.PolicyID,
		DiskUsagePct:   status.DiskUsagePct,
		DiskFreeBytes:  status.DiskFreeBytes,
		DiskTotalBytes: status.DiskTotalBytes,
	}
}

// StartStorage starts the storage server
func (s *HostService) StartStorage() error {
	if s.storageServer == nil {
		return errors.New("storage server not configured")
	}
	s.storageServer.Start()
	return nil
}

// StopStorage stops the storage server
func (s *HostService) StopStorage() error {
	if s.storageServer == nil {
		return errors.New("storage server not configured")
	}
	s.storageServer.Stop()
	return nil
}

// ReceiveShare stores a key share received from the owner
func (s *HostService) ReceiveShare(share []byte, shareIndex byte, repoURL, peerName string) error {
	s.cfg.LocalShare = share
	s.cfg.ShareIndex = shareIndex
	s.cfg.RepoURL = repoURL
	s.cfg.Peer = &config.PeerInfo{
		Name: peerName,
	}
	return s.cfg.Save()
}

// GetHostKeys returns the host's key ID and private key for signing.
// Returns empty strings/nil if the host is not initialized.
func (s *HostService) GetHostKeys() (keyID string, privateKey []byte) {
	if s.cfg.Role != config.RoleHost || s.cfg.PrivateKey == nil {
		return "", nil
	}
	keyID = crypto.KeyID(s.cfg.PublicKey)
	return keyID, s.cfg.PrivateKey
}

// GetHostPublicKey returns the host's public key.
func (s *HostService) GetHostPublicKey() []byte {
	return s.cfg.PublicKey
}
