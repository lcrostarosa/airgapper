package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/consent"
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
	PrintInfo("Airgapper Status: Not initialized")
	fmt.Println()
	PrintInfo("To get started:")
	PrintInfo("  As data owner:  airgapper init --name <name> --repo <url>")
	PrintInfo("  As backup host: airgapper join --name <name> --repo <url> --share <hex> --index <n>")
	return nil
}

func showStatus() error {
	PrintHeader("Airgapper Status")

	// Identity
	PrintInfo("Name:       %s", cfg.Name)
	PrintInfo("Role:       %s", cfg.Role)
	PrintInfo("Repository: %s", cfg.RepoURL)

	// Key info
	if cfg.LocalShare != nil {
		PrintInfo("Key Share:  Index %d (%d bytes)", cfg.ShareIndex, len(cfg.LocalShare))
	} else {
		PrintInfo("Key Share:  Not configured")
	}

	if cfg.IsOwner() {
		if cfg.Password != "" {
			PrintInfo("Password:   ✅ Stored (can backup)")
		} else {
			PrintInfo("Password:   ❌ Missing")
		}
	}

	// Peer info
	if cfg.Peer != nil {
		peer := cfg.Peer.Name
		if cfg.Peer.Address != "" {
			peer += fmt.Sprintf(" (%s)", cfg.Peer.Address)
		}
		PrintInfo("Peer:       %s", peer)
	} else {
		PrintInfo("Peer:       Not configured")
	}

	// Schedule
	if cfg.BackupSchedule != "" {
		PrintInfo("Schedule:   %s", cfg.BackupSchedule)
		if len(cfg.BackupPaths) > 0 {
			PrintInfo("Paths:      %s", strings.Join(cfg.BackupPaths, ", "))
		}
	} else {
		PrintInfo("Schedule:   Not configured")
	}

	// Restic
	if restic.IsInstalled() {
		ver, _ := restic.Version()
		PrintInfo("Restic:     %s", ver)
	} else {
		PrintInfo("Restic:     ❌ Not installed")
	}

	// Pending requests
	mgr := consent.NewManager(cfg.ConfigDir)
	pending, _ := mgr.ListPending()
	PrintInfo("Pending:    %d restore request(s)", len(pending))

	// Emergency features
	if cfg.HasEmergencyConfig() {
		fmt.Println()
		PrintInfo("Emergency Features:")

		if cfg.Emergency.Recovery != nil && cfg.Emergency.Recovery.Enabled {
			PrintInfo("  • Recovery shares: %d-of-%d", cfg.Emergency.Recovery.Threshold, cfg.Emergency.Recovery.TotalShares)
			if len(cfg.Emergency.Recovery.Custodians) > 0 {
				PrintInfo("  • Custodians: %d configured", len(cfg.Emergency.Recovery.Custodians))
			}
		}

		if cfg.Emergency.DeadManSwitch != nil && cfg.Emergency.DeadManSwitch.Enabled {
			dms := cfg.Emergency.DeadManSwitch
			PrintInfo("  • Dead man's switch: %d days (triggers in %d days)", dms.InactivityDays, dms.DaysUntilTrigger())
			if dms.IsWarning() {
				PrintWarning("  WARNING: Approaching inactivity threshold!")
			}
		}

		if cfg.Emergency.Override != nil && cfg.Emergency.Override.Enabled {
			PrintInfo("  • Override key: configured (%d allowed types)", len(cfg.Emergency.Override.AllowedTypes))
		}

		if cfg.Emergency.Notify != nil && cfg.Emergency.Notify.Enabled {
			PrintInfo("  • Notifications: %d providers configured", len(cfg.Emergency.Notify.Providers))
		}
	}

	return nil
}
