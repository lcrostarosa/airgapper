package cmd

import (
	"fmt"
	"strings"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/consent"
	"github.com/lcrostarosa/airgapper/internal/restic"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status",
	Long:  `Show the current status of your Airgapper configuration.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		// Not initialized
		fmt.Println("Airgapper Status: Not initialized")
		fmt.Println()
		fmt.Println("To get started:")
		fmt.Println("  As data owner:  airgapper init --name <name> --repo <url>")
		fmt.Println("  As backup host: airgapper join --name <name> --repo <url> --share <hex> --index <n>")
		return nil
	}

	fmt.Println("📊 Airgapper Status")
	fmt.Println("===================")
	fmt.Printf("Name:       %s\n", cfg.Name)
	fmt.Printf("Role:       %s\n", cfg.Role)
	fmt.Printf("Repository: %s\n", cfg.RepoURL)

	// Show threshold info
	if cfg.Threshold > 0 {
		fmt.Printf("Threshold:  %d-of-%d\n", cfg.Threshold, cfg.TotalShares)
	}

	if cfg.LocalShare != nil {
		fmt.Printf("Key Share:  Index %d (%d bytes)\n", cfg.ShareIndex, len(cfg.LocalShare))
	} else {
		fmt.Println("Key Share:  Not configured")
	}

	if cfg.IsOwner() {
		if cfg.Password != "" {
			fmt.Println("Password:   ✅ Stored (can backup)")
		} else {
			fmt.Println("Password:   ❌ Missing")
		}
	}

	if cfg.Peer != nil {
		fmt.Printf("Peer:       %s", cfg.Peer.Name)
		if cfg.Peer.Address != "" {
			fmt.Printf(" (%s)", cfg.Peer.Address)
		}
		fmt.Println()
	} else if len(cfg.PeerShares) > 0 {
		fmt.Printf("Peers:      %d configured\n", len(cfg.PeerShares))
	} else {
		fmt.Println("Peer:       Not configured")
	}

	// Show schedule if configured
	if cfg.BackupSchedule != "" {
		fmt.Printf("Schedule:   %s\n", cfg.BackupSchedule)
		if len(cfg.BackupPaths) > 0 {
			fmt.Printf("Paths:      %s\n", strings.Join(cfg.BackupPaths, ", "))
		}
	} else {
		fmt.Println("Schedule:   Not configured")
	}

	// Check restic
	if restic.IsInstalled() {
		ver, _ := restic.Version()
		fmt.Printf("Restic:     %s\n", ver)
	} else {
		fmt.Println("Restic:     ❌ Not installed")
	}

	// Check pending requests
	mgr := consent.NewManager(cfg.ConfigDir)
	pending, _ := mgr.ListPending()
	fmt.Printf("Pending:    %d restore request(s)\n", len(pending))

	return nil
}
