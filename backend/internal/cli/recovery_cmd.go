package cli

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
)

// --- Export Share Command ---

var exportShareCmd = &cobra.Command{
	Use:   "export-share",
	Short: "Re-export a specific key share (for lost custodian shares)",
	Long: `Re-export a specific key share for distribution to custodians.

Use this when a custodian has lost their share and needs a new copy.`,
	Example: `  airgapper export-share --index 3`,
	RunE:    runners.OwnerWithPassword().Wrap(runExportShare),
}

func init() {
	exportShareCmd.Flags().Int("index", 0, "Share index to export (required)")
	_ = exportShareCmd.MarkFlagRequired("index")
	rootCmd.AddCommand(exportShareCmd)
}

func runExportShare(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	flags := runner.Flags(cmd)
	shareIndex := flags.Int("index")
	if err := flags.Err(); err != nil {
		return err
	}

	k := ctx.Config.Emergency.GetRecovery().GetThreshold()
	n := ctx.Config.Emergency.GetRecovery().GetTotalShares()

	if shareIndex > n || shareIndex < 1 {
		return fmt.Errorf("share index %d is out of range (1-%d)", shareIndex, n)
	}

	// Regenerate shares
	shares, err := sss.Split([]byte(ctx.Config.Password), k, n)
	if err != nil {
		return fmt.Errorf("failed to regenerate shares: %w", err)
	}

	// Find requested share
	var targetShare *sss.Share
	for i := range shares {
		if shares[i].Index == byte(shareIndex) {
			targetShare = &shares[i]
			break
		}
	}

	if targetShare == nil {
		return fmt.Errorf("share index %d not found", shareIndex)
	}

	logging.Info("Exporting share",
		logging.Int("index", shareIndex),
		logging.String("share", hex.EncodeToString(targetShare.Data)),
		logging.String("repo", ctx.Config.RepoURL))

	logging.Warnf("This share is part of a %d-of-%d scheme. Any %d shares can decrypt your backups - store securely!", k, n, k)

	return nil
}
