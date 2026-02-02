package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status",
	Long:  `Display the current Airgapper configuration and status.`,
	RunE:  runners.Uninitialized().Wrap(runStatus),
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	if ctx.Config == nil {
		return showUninitialized()
	}
	return showStatus(ctx)
}

func showUninitialized() error {
	logging.Info("Airgapper status: Not initialized")
	logging.Info("To get started:")
	logging.Info("  As data owner:  airgapper init --name <name> --repo <url>")
	logging.Info("  As backup host: airgapper join --name <name> --repo <url> --share <hex> --index <n>")
	return nil
}

func showStatus(ctx *runner.CommandContext) error {
	logging.Info("Airgapper status",
		logging.String("name", ctx.Config.Name),
		logging.String("role", string(ctx.Config.Role)),
		logging.String("repository", ctx.Config.RepoURL))

	// Key info
	if ctx.Config.LocalShare != nil {
		logging.Infof("Key share: Index %d (%d bytes)", ctx.Config.ShareIndex, len(ctx.Config.LocalShare))
	} else {
		logging.Info("Key share: Not configured")
	}

	if ctx.Config.IsOwner() {
		if ctx.Config.Password != "" {
			logging.Info("Password: Stored (can backup)")
		} else {
			logging.Warn("Password: Missing")
		}
	}

	// Peer info
	if ctx.Config.Peer != nil {
		peerInfo := ctx.Config.Peer.Name
		if ctx.Config.Peer.Address != "" {
			peerInfo += " (" + ctx.Config.Peer.Address + ")"
		}
		logging.Info("Peer", logging.String("peer", peerInfo))
	} else {
		logging.Info("Peer: Not configured")
	}

	// Schedule
	if ctx.Config.BackupSchedule != "" {
		logging.Info("Schedule",
			logging.String("schedule", ctx.Config.BackupSchedule),
			logging.String("paths", strings.Join(ctx.Config.BackupPaths, ", ")))
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
	pending, _ := ctx.Consent().ListPending()
	logging.Info("Pending restore requests", logging.Int("count", len(pending)))

	// Emergency features
	if ctx.Config.HasEmergencyConfig() {
		logging.Info("Emergency features:")

		if ctx.Config.Emergency.Recovery != nil && ctx.Config.Emergency.Recovery.Enabled {
			logging.Infof("  Recovery shares: %d-of-%d", ctx.Config.Emergency.Recovery.Threshold, ctx.Config.Emergency.Recovery.TotalShares)
			if len(ctx.Config.Emergency.Recovery.Custodians) > 0 {
				logging.Infof("  Custodians: %d configured", len(ctx.Config.Emergency.Recovery.Custodians))
			}
		}

		if ctx.Config.Emergency.DeadManSwitch != nil && ctx.Config.Emergency.DeadManSwitch.Enabled {
			dms := ctx.Config.Emergency.DeadManSwitch
			logging.Infof("  Dead man's switch: %d days (triggers in %d days)", dms.InactivityDays, dms.DaysUntilTrigger())
			if dms.IsWarning() {
				logging.Warn("  WARNING: Approaching inactivity threshold!")
			}
		}

		if ctx.Config.Emergency.Override != nil && ctx.Config.Emergency.Override.Enabled {
			logging.Infof("  Override key: configured (%d allowed types)", len(ctx.Config.Emergency.Override.AllowedTypes))
		}

		if ctx.Config.Emergency.Notify != nil && ctx.Config.Emergency.Notify.Enabled {
			logging.Infof("  Notifications: %d providers configured", len(ctx.Config.Emergency.Notify.Providers))
		}
	}

	return nil
}
