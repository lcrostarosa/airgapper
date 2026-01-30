package cmd

import (
	"fmt"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/consent"
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:   "approve <request-id>",
	Short: "Approve a restore request (releases key share)",
	Long: `Approve a restore request and release your key share.

When you approve a request, your share of the encryption key is released
to the requester, allowing them to restore their data (if they have enough shares).`,
	Args: cobra.ExactArgs(1),
	RunE: runApprove,
}

func init() {
	rootCmd.AddCommand(approveCmd)
}

func runApprove(cmd *cobra.Command, args []string) error {
	requestID := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	mgr := consent.NewManager(cfg.ConfigDir)

	// Load our share
	share, shareIndex, err := cfg.LoadShare()
	if err != nil {
		return fmt.Errorf("failed to load share: %w", err)
	}

	fmt.Printf("Approving request %s...\n", requestID)
	fmt.Printf("Releasing key share (index %d)...\n", shareIndex)

	// Approve and attach our share
	if err := mgr.Approve(requestID, cfg.Name, share); err != nil {
		return err
	}

	fmt.Println("\n✅ Request approved!")
	fmt.Println("Your key share has been released.")
	fmt.Println("The requester can now restore their data (if they have enough shares).")

	return nil
}
