package cmd

import (
	"fmt"

	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/consent"
	"github.com/spf13/cobra"
)

var denyCmd = &cobra.Command{
	Use:   "deny <request-id>",
	Short: "Deny a restore request",
	Long:  `Deny a restore request. The requester will not receive your key share.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDeny,
}

func init() {
	rootCmd.AddCommand(denyCmd)
}

func runDeny(cmd *cobra.Command, args []string) error {
	requestID := args[0]

	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	mgr := consent.NewManager(cfg.ConfigDir)

	if err := mgr.Deny(requestID, cfg.Name); err != nil {
		return err
	}

	fmt.Println("❌ Request denied.")

	return nil
}
