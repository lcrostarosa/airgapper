package cmd

import (
	"fmt"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/consent"
	"github.com/spf13/cobra"
)

var pendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "List pending restore requests",
	Long:  `List all pending restore requests that need approval.`,
	RunE:  runPending,
}

func init() {
	rootCmd.AddCommand(pendingCmd)
}

func runPending(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	mgr := consent.NewManager(cfg.ConfigDir)
	requests, err := mgr.ListPending()
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		fmt.Println("No pending restore requests.")
		return nil
	}

	fmt.Println("📋 Pending Restore Requests")
	fmt.Println("===========================")
	for _, req := range requests {
		fmt.Printf("\nID: %s\n", req.ID)
		fmt.Printf("  From:     %s\n", req.Requester)
		fmt.Printf("  Snapshot: %s\n", req.SnapshotID)
		fmt.Printf("  Reason:   %s\n", req.Reason)
		fmt.Printf("  Expires:  %s\n", req.ExpiresAt.Format("2006-01-02 15:04"))
	}

	fmt.Println("\nTo approve: airgapper approve <request-id>")
	fmt.Println("To deny:    airgapper deny <request-id>")

	return nil
}
