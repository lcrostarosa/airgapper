package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status",
	Long:  `Display the current Airgapper configuration and status.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	if cfg == nil {
		return showUninitialized()
	}
	return showStatus()
}

func showUninitialized() error {
	logging.Info("Airgapper status: Not initialized")
	logging.Info("To get started:")
	logging.Info("  As data owner:  airgapper init --name <name> --repo <url>")
	logging.Info("  As backup host: airgapper join --name <name> --repo <url> --share <hex> --index <n>")
	return nil
}

func showStatus() error {
	logging.Info("Airgapper status",
		logging.String("name", cfg.Name),
		logging.String("role", string(cfg.Role)),
		logging.String("repository", cfg.RepoURL))

	// Key info
	if cfg.LocalShare != nil {
		logging.Infof("Key share: Index %d (%d bytes)", cfg.ShareIndex, len(cfg.LocalShare))
	} else {
		logging.Info("Key share: Not configured")
	}

	if cfg.IsOwner() {
		if cfg.Password != "" {
			logging.Info("Password: Stored (can backup)")
		} else {
			logging.Warn("Password: Missing")
		}
	}

	// Peer info
	if cfg.Peer != nil {
		peerInfo := cfg.Peer.Name
		if cfg.Peer.Address != "" {
			peerInfo += " (" + cfg.Peer.Address + ")"
		}
		logging.Info("Peer", logging.String("peer", peerInfo))
	} else {
		logging.Info("Peer: Not configured")
	}

	// Schedule
	if cfg.BackupSchedule != "" {
		logging.Info("Schedule",
			logging.String("schedule", cfg.BackupSchedule),
			logging.String("paths", strings.Join(cfg.BackupPaths, ", ")))
	} else {
		logging.Info("Schedule: Not configured")
	}

	// Restic
	if restic.IsInstalled() {
		ver, _ := restic.Version()
		logging.Info("Restic", logging.String("version", ver))
	} else {
		logging.Warn("Restic: Not installed")
	}

	// Pending requests
	mgr := consent.NewManager(cfg.ConfigDir)
	pending, _ := mgr.ListPending()
	logging.Info("Pending restore requests", logging.Int("count", len(pending)))

	// Emergency features
	if cfg.HasEmergencyConfig() {
		logging.Info("Emergency features:")

		if cfg.Emergency.Recovery != nil && cfg.Emergency.Recovery.Enabled {
			logging.Infof("  Recovery shares: %d-of-%d", cfg.Emergency.Recovery.Threshold, cfg.Emergency.Recovery.TotalShares)
			if len(cfg.Emergency.Recovery.Custodians) > 0 {
				logging.Infof("  Custodians: %d configured", len(cfg.Emergency.Recovery.Custodians))
			}
		}

		if cfg.Emergency.DeadManSwitch != nil && cfg.Emergency.DeadManSwitch.Enabled {
			dms := cfg.Emergency.DeadManSwitch
			logging.Infof("  Dead man's switch: %d days (triggers in %d days)", dms.InactivityDays, dms.DaysUntilTrigger())
			if dms.IsWarning() {
				logging.Warn("  WARNING: Approaching inactivity threshold!")
			}
		}

		if cfg.Emergency.Override != nil && cfg.Emergency.Override.Enabled {
			logging.Infof("  Override key: configured (%d allowed types)", len(cfg.Emergency.Override.AllowedTypes))
		}

		if cfg.Emergency.Notify != nil && cfg.Emergency.Notify.Enabled {
			logging.Infof("  Notifications: %d providers configured", len(cfg.Emergency.Notify.Providers))
		}
	}

	return nil
}
