package cmd

import (
	"encoding/hex"
	"fmt"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/spf13/cobra"
)

var (
	joinName       string
	joinRepo       string
	joinShare      string
	joinShareIndex int
	joinThreshold  int
	joinTotalShares int
)

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join as backup host (receive key share from owner)",
	Long: `Join an Airgapper backup as a backup host.

You'll need the share data and index provided by the data owner.
As a backup host, you:
  - Store encrypted backup data
  - Hold a key share required for restore
  - Must approve restore requests

Examples:
  airgapper join --name bob --repo rest:http://localhost:8000/alice-backup \
    --share abc123... --index 2

  airgapper join --name charlie --repo rest:http://server:8000/backup \
    --share def456... --index 3 --threshold 2 --total 3`,
	RunE: runJoin,
}

func init() {
	rootCmd.AddCommand(joinCmd)

	joinCmd.Flags().StringVarP(&joinName, "name", "n", "", "Your name/identifier (required)")
	joinCmd.Flags().StringVarP(&joinRepo, "repo", "r", "", "Restic repository URL (required)")
	joinCmd.Flags().StringVarP(&joinShare, "share", "s", "", "Hex-encoded key share from owner (required)")
	joinCmd.Flags().IntVarP(&joinShareIndex, "index", "i", 0, "Share index provided by owner (required)")
	joinCmd.Flags().IntVarP(&joinThreshold, "threshold", "t", 2, "Number of shares required to restore")
	joinCmd.Flags().IntVarP(&joinTotalShares, "total", "T", 2, "Total number of shares")

	joinCmd.MarkFlagRequired("name")
	joinCmd.MarkFlagRequired("repo")
	joinCmd.MarkFlagRequired("share")
	joinCmd.MarkFlagRequired("index")
}

func runJoin(cmd *cobra.Command, args []string) error {
	if joinShareIndex == 0 {
		return fmt.Errorf("--index is required (share index, usually 2+)")
	}

	// Check if already initialized
	if config.Exists("") {
		return fmt.Errorf("already initialized. Remove ~/.airgapper to reinitialize")
	}

	// Decode share
	share, err := hex.DecodeString(joinShare)
	if err != nil {
		return fmt.Errorf("invalid share (must be hex): %w", err)
	}

	fmt.Println("🔐 Airgapper Join (Backup Host)")
	fmt.Println("================================")
	fmt.Printf("Name:      %s\n", joinName)
	fmt.Printf("Repo:      %s\n", joinRepo)
	fmt.Printf("Share:     %d bytes, index %d\n", len(share), joinShareIndex)
	fmt.Printf("Threshold: %d-of-%d\n\n", joinThreshold, joinTotalShares)

	// Save config
	cfg := &config.Config{
		Name:        joinName,
		Role:        config.RoleHost,
		RepoURL:     joinRepo,
		LocalShare:  share,
		ShareIndex:  byte(joinShareIndex),
		Threshold:   joinThreshold,
		TotalShares: joinTotalShares,
		// Note: Host does NOT have the password, only the share
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✅ Joined as backup host!")
	fmt.Println()
	fmt.Println("You are now a key holder for this backup repository.")
	fmt.Printf("The owner needs %d shares to restore (you have share #%d).\n", joinThreshold, joinShareIndex)
	fmt.Println()
	fmt.Println("Commands available to you:")
	fmt.Println("  airgapper pending  - List pending restore requests")
	fmt.Println("  airgapper approve  - Approve a restore request")
	fmt.Println("  airgapper deny     - Deny a restore request")
	fmt.Println("  airgapper serve    - Run HTTP API for remote management")

	return nil
}
