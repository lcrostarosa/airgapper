package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore from a snapshot (requires approval)",
	Long:  `Restore data from a backup snapshot after approval has been granted.`,
	Example: `  airgapper restore --request abc123 --target /restore/path
  airgapper restore --request abc123 --target ~/recovered`,
	RunE: runners.Owner().Wrap(runRestore),
}

func init() {
	f := restoreCmd.Flags()
	f.String("request", "", "Request ID (required)")
	f.String("target", "", "Restore target directory (required)")
	_ = restoreCmd.MarkFlagRequired("request")
	_ = restoreCmd.MarkFlagRequired("target")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	flags := runner.Flags(cmd)
	requestID := flags.String("request")
	target := flags.String("target")
	if err := flags.Err(); err != nil {
		return err
	}

	req, err := ctx.Consent().GetRequest(requestID)
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
	localShare, localIndex, err := ctx.Config.LoadShare()
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

	logging.Info("Reconstructing password from key shares")
	password, err := sss.Combine(shares)
	if err != nil {
		return fmt.Errorf("failed to reconstruct password: %w", err)
	}

	logging.Info("Password reconstructed successfully")
	logging.Info("Starting restore",
		logging.String("snapshot", req.SnapshotID),
		logging.String("target", target))

	client := restic.NewClient(ctx.Config.RepoURL, string(password))
	if err := client.Restore(cmd.Context(), req.SnapshotID, target); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	logging.Info("Restore complete", logging.String("target", target))
	return nil
}
