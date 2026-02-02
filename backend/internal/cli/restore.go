package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore from a snapshot (requires approval)",
	Long:  `Restore data from a backup snapshot after approval has been granted.`,
	Example: `  airgapper restore --request abc123 --target /restore/path
  airgapper restore --request abc123 --target ~/recovered`,
	RunE: runRestore,
}

func init() {
	f := restoreCmd.Flags()
	f.String("request", "", "Request ID (required)")
	f.String("target", "", "Restore target directory (required)")
	restoreCmd.MarkFlagRequired("request")
	restoreCmd.MarkFlagRequired("target")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	if err := RequireOwner(); err != nil {
		return err
	}

	requestID, _ := cmd.Flags().GetString("request")
	target, _ := cmd.Flags().GetString("target")

	mgr := consent.NewManager(cfg.ConfigDir)

	req, err := mgr.GetRequest(requestID)
	if err != nil {
		return err
	}

	if req.Status != consent.StatusApproved {
		return fmt.Errorf("request is not approved (status: %s)", req.Status)
	}

	if req.ShareData == nil {
		return fmt.Errorf("approved request missing share data")
	}

	// Reconstruct password
	localShare, localIndex, err := cfg.LoadShare()
	if err != nil {
		return err
	}

	peerIndex := byte(1)
	if localIndex == 1 {
		peerIndex = 2
	}

	shares := []sss.Share{
		{Index: localIndex, Data: localShare},
		{Index: peerIndex, Data: req.ShareData},
	}

	fmt.Println("Reconstructing password from key shares...")
	password, err := sss.Combine(shares)
	if err != nil {
		return fmt.Errorf("failed to reconstruct password: %w", err)
	}

	fmt.Println("Password reconstructed successfully")
	fmt.Println("Starting restore...")
	fmt.Printf("Snapshot: %s\n", req.SnapshotID)
	fmt.Printf("Target:   %s\n\n", target)

	client := restic.NewClient(cfg.RepoURL, string(password))
	if err := client.Restore(req.SnapshotID, target); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Printf("Restore complete! Files restored to: %s\n", target)
	return nil
}
