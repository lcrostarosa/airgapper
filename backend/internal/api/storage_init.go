package api

import (
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/integrity"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

// ServerOptions contains optional components that can be injected into the server
type ServerOptions struct {
	// StorageServer is an optional pre-initialized storage server
	StorageServer *storage.Server

	// IntegrityChecker is an optional pre-initialized integrity checker
	IntegrityChecker *integrity.Checker

	// ScheduledChecker is an optional pre-initialized scheduled integrity checker
	ScheduledChecker *integrity.ManagedScheduledChecker
}

// InitStorageComponents initializes storage-related components from config.
// Returns a ServerOptions struct ready to be passed to NewServer.
// This can be called independently to set up storage for standalone operation.
func InitStorageComponents(cfg *config.Config) (*ServerOptions, error) {
	opts := &ServerOptions{}

	if cfg.StoragePath == "" {
		return opts, nil
	}

	// Initialize storage server
	storageServer, err := storage.NewServer(storage.Config{
		BasePath:   cfg.StoragePath,
		AppendOnly: cfg.StorageAppendOnly,
		QuotaBytes: cfg.StorageQuotaBytes,
	})
	if err != nil {
		logging.Warnf("failed to initialize storage server: %v", err)
		return opts, nil
	}
	opts.StorageServer = storageServer

	// Initialize integrity checker
	integrityChecker, err := integrity.NewChecker(cfg.StoragePath)
	if err != nil {
		logging.Warnf("failed to initialize integrity checker: %v", err)
	} else {
		opts.IntegrityChecker = integrityChecker
		logging.Info("Integrity checker initialized")
	}

	// Initialize managed scheduled checker for scheduled verification
	managedChecker, err := integrity.NewManagedScheduledChecker(cfg.StoragePath)
	if err != nil {
		logging.Warnf("failed to initialize scheduled checker: %v", err)
	} else {
		opts.ScheduledChecker = managedChecker
	}

	return opts, nil
}

// StartStorageComponents starts storage-related components.
// Call this after InitStorageComponents to begin serving storage requests.
func StartStorageComponents(opts *ServerOptions) {
	if opts.StorageServer != nil {
		opts.StorageServer.Start()
		logging.Info("Storage server started")
	}

	if opts.ScheduledChecker != nil {
		if err := opts.ScheduledChecker.Start(); err != nil {
			logging.Warnf("failed to start scheduled verification: %v", err)
		} else {
			verifyConfig := opts.ScheduledChecker.GetConfig()
			if verifyConfig.Enabled {
				logging.Infof("Scheduled verification started (interval: %s, type: %s)",
					verifyConfig.Interval, verifyConfig.CheckType)
			}
		}
	}
}

// StopStorageComponents gracefully stops storage-related components.
func StopStorageComponents(opts *ServerOptions) {
	if opts.StorageServer != nil {
		opts.StorageServer.Stop()
		logging.Info("Storage server stopped")
	}

	if opts.ScheduledChecker != nil {
		opts.ScheduledChecker.Stop()
		logging.Info("Scheduled verification stopped")
	}
}
