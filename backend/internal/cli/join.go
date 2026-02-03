package cli

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join as backup host / key holder",
	Long: `Join an existing Airgapper vault as a backup host or key holder.

In SSS mode, you receive a key share from the data owner.
In consensus mode, you generate your own key pair and register with the owner.`,
	Example: `  # Join as backup host (SSS mode)
  airgapper join --name bob --repo rest:http://localhost:8000/backup \
    --share abc123... --index 2

  # Join as key holder (consensus mode)
  airgapper join --name bob --repo rest:http://localhost:8000/backup --consensus`,
	RunE: runners.Uninitialized().Wrap(runJoin),
}

func init() {
	f := joinCmd.Flags()

	f.StringP("name", "n", "", "Your name/identifier")
	f.StringP("repo", "r", "", "Restic repository URL")
	_ = joinCmd.MarkFlagRequired("name")
	_ = joinCmd.MarkFlagRequired("repo")

	// SSS mode
	f.StringP("share", "s", "", "Hex-encoded key share from owner")
	f.IntP("index", "i", 0, "Share index (usually 2)")

	// Consensus mode
	f.Bool("consensus", false, "Join in consensus mode (generate key pair)")

	rootCmd.AddCommand(joinCmd)
}

func runJoin(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	flags := runner.Flags(cmd)
	name := flags.String("name")
	repoURL := flags.String("repo")
	consensus := flags.Bool("consensus")
	if err := flags.Err(); err != nil {
		return err
	}

	if config.Exists("") {
		return fmt.Errorf("already initialized. Remove ~/.airgapper to reinitialize")
	}

	if consensus {
		return joinConsensus(name, repoURL)
	}

	return joinSSS(cmd, name, repoURL)
}

func joinSSS(cmd *cobra.Command, name, repoURL string) error {
	flags := runner.Flags(cmd)
	shareHex := flags.String("share")
	shareIndex := flags.Int("index")
	if err := flags.Err(); err != nil {
		return err
	}

	if shareHex == "" {
		return fmt.Errorf("--share is required (hex-encoded share from owner)")
	}
	if shareIndex == 0 {
		return fmt.Errorf("--index is required (share index, usually 2)")
	}

	share, err := hex.DecodeString(shareHex)
	if err != nil {
		return fmt.Errorf("invalid share (must be hex): %w", err)
	}

	logging.Info("Airgapper join (Backup Host) - SSS Mode",
		logging.String("name", name),
		logging.String("repo", repoURL),
		logging.Int("shareBytes", len(share)),
		logging.Int("index", shareIndex))

	newCfg := &config.Config{
		Name:       name,
		Role:       config.RoleHost,
		RepoURL:    repoURL,
		LocalShare: share,
		ShareIndex: byte(shareIndex),
	}

	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	logging.Info("Joined as backup host")
	logging.Info("Commands available to you:")
	logging.Info("  airgapper pending  - List pending restore requests")
	logging.Info("  airgapper approve  - Approve a restore request")
	logging.Info("  airgapper deny     - Deny a restore request")
	logging.Info("  airgapper serve    - Run HTTP API for remote management")

	return nil
}

func joinConsensus(name, repoURL string) error {
	logging.Info("Airgapper join (Key Holder) - Consensus Mode",
		logging.String("name", name),
		logging.String("repo", repoURL))

	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}
	keyID := crypto.KeyID(pubKey)

	logging.Info("Generated Ed25519 key pair", logging.String("keyID", keyID))

	newCfg := &config.Config{
		Name:       name,
		Role:       config.RoleHost,
		RepoURL:    repoURL,
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}

	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	logging.Info("Configuration saved to ~/.airgapper/")
	logging.Warn("IMPORTANT: Register with the vault owner")
	logging.Info("Share your public key with the vault owner so they can register you:")
	logging.Infof("  Public Key: %s", crypto.EncodePublicKey(pubKey))
	logging.Infof("  Key ID:     %s", keyID)
	logging.Info("Joined as key holder")
	return nil
}
