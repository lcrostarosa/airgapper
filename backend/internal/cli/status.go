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
	printInfo("Airgapper Status: Not initialized")
	fmt.Println()
	printInfo("To get started:")
	printInfo("  As data owner:  airgapper init --name <name> --repo <url>")
	printInfo("  As backup host: airgapper join --name <name> --repo <url> --share <hex> --index <n>")
	return nil
}

func showStatus() error {
	printHeader("Airgapper Status")

	// Identity
	printInfo("Name:       %s", cfg.Name)
	printInfo("Role:       %s", cfg.Role)
	printInfo("Repository: %s", cfg.RepoURL)

	// Key info
	if cfg.LocalShare != nil {
		printInfo("Key Share:  Index %d (%d bytes)", cfg.ShareIndex, len(cfg.LocalShare))
	} else {
		printInfo("Key Share:  Not configured")
	}

	if cfg.IsOwner() {
		if cfg.Password != "" {
			printInfo("Password:   ✅ Stored (can backup)")
		} else {
			printInfo("Password:   ❌ Missing")
		}
	}

	// Peer info
	if cfg.Peer != nil {
		peer := cfg.Peer.Name
		if cfg.Peer.Address != "" {
			peer += fmt.Sprintf(" (%s)", cfg.Peer.Address)
		}
		printInfo("Peer:       %s", peer)
	} else {
		printInfo("Peer:       Not configured")
	}

	// Schedule
	if cfg.BackupSchedule != "" {
		printInfo("Schedule:   %s", cfg.BackupSchedule)
		if len(cfg.BackupPaths) > 0 {
			printInfo("Paths:      %s", strings.Join(cfg.BackupPaths, ", "))
		}
	} else {
		printInfo("Schedule:   Not configured")
	}

	// Restic
	if restic.IsInstalled() {
		ver, _ := restic.Version()
		printInfo("Restic:     %s", ver)
	} else {
		printInfo("Restic:     ❌ Not installed")
	}

	// Pending requests
	mgr := consent.NewManager(cfg.ConfigDir)
	pending, _ := mgr.ListPending()
	printInfo("Pending:    %d restore request(s)", len(pending))

	// Emergency features
	if cfg.HasEmergencyConfig() {
		fmt.Println()
		printInfo("Emergency Features:")

		if cfg.Emergency.Recovery != nil && cfg.Emergency.Recovery.Enabled {
			printInfo("  • Recovery shares: %d-of-%d", cfg.Emergency.Recovery.Threshold, cfg.Emergency.Recovery.TotalShares)
			if len(cfg.Emergency.Recovery.Custodians) > 0 {
				printInfo("  • Custodians: %d configured", len(cfg.Emergency.Recovery.Custodians))
			}
		}

		if cfg.Emergency.DeadManSwitch != nil && cfg.Emergency.DeadManSwitch.Enabled {
			dms := cfg.Emergency.DeadManSwitch
			printInfo("  • Dead man's switch: %d days (triggers in %d days)", dms.InactivityDays, dms.DaysUntilTrigger())
			if dms.IsWarning() {
				printWarning("  WARNING: Approaching inactivity threshold!")
			}
		}

		if cfg.Emergency.Override != nil && cfg.Emergency.Override.Enabled {
			printInfo("  • Override key: configured (%d allowed types)", len(cfg.Emergency.Override.AllowedTypes))
		}

		if cfg.Emergency.Notify != nil && cfg.Emergency.Notify.Enabled {
			printInfo("  • Notifications: %d providers configured", len(cfg.Emergency.Notify.Providers))
		}
	}

	return nil
}
