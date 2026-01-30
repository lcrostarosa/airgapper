package cmd

import (
	"fmt"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/consent"
	"github.com/lcrostarosa/airgapper/internal/restic"
	"github.com/lcrostarosa/airgapper/internal/sss"
	"github.com/spf13/cobra"
)

var (
	restoreRequest string
	restoreTarget  string
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore from a snapshot (requires approval)",
	Long: `Restore data from a snapshot after receiving approval from peers.

You need approval from enough peers to meet the threshold before restoring.
For a 2-of-3 setup, you need at least 1 other peer's approval.

Examples:
  airgapper restore --request abc123 --target /restore/path`,
	RunE: runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().StringVarP(&restoreRequest, "request", "r", "", "Request ID (required)")
	restoreCmd.Flags().StringVarP(&restoreTarget, "target", "t", "", "Target directory for restore (required)")

	_ = restoreCmd.MarkFlagRequired("request")
	_ = restoreCmd.MarkFlagRequired("target")
}

func runRestore(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsOwner() {
		return fmt.Errorf("only the data owner can restore data")
	}

	mgr := consent.NewManager(cfg.ConfigDir)
	req, err := mgr.GetRequest(restoreRequest)
	if err != nil {
		return err
	}

	if req.Status != consent.StatusApproved {
		return fmt.Errorf("request is not approved (status: %s)", req.Status)
	}

	// Get all collected shares from the request
	collectedShares := req.CollectedShares
	if collectedShares == nil {
		// Backwards compatibility: check old single share field
		if req.ShareData == nil {
			return fmt.Errorf("approved request missing share data - peer approval may have failed")
		}
		// Convert old format to new
		collectedShares = []consent.CollectedShare{
			{Index: 2, Data: req.ShareData, ApprovedBy: req.ApprovedBy},
		}
	}

	// Load our local share
	localShare, localIndex, err := cfg.LoadShare()
	if err != nil {
		return err
	}

	// Build shares array for reconstruction
	shares := []sss.Share{
		{Index: localIndex, Data: localShare},
	}

	for _, cs := range collectedShares {
		shares = append(shares, sss.Share{Index: cs.Index, Data: cs.Data})
	}

	// Check if we have enough shares
	if len(shares) < cfg.Threshold {
		return fmt.Errorf("not enough shares: have %d, need %d", len(shares), cfg.Threshold)
	}

	fmt.Println("🔓 Reconstructing password from key shares...")
	fmt.Printf("Using %d shares (threshold: %d)\n", len(shares), cfg.Threshold)

	password, err := sss.Combine(shares)
	if err != nil {
		return fmt.Errorf("failed to reconstruct password: %w", err)
	}

	fmt.Println("✅ Password reconstructed successfully")
	fmt.Println()
	fmt.Println("📥 Starting restore...")
	fmt.Printf("Snapshot: %s\n", req.SnapshotID)
	fmt.Printf("Target:   %s\n\n", restoreTarget)

	client := restic.NewClient(cfg.RepoURL, string(password))
	if err := client.Restore(req.SnapshotID, restoreTarget); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Printf("\n✅ Restore complete! Files restored to: %s\n", restoreTarget)

	return nil
}
