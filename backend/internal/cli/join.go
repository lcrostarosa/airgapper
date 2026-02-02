package cli

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
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
	RunE: runJoin,
}

func init() {
	f := joinCmd.Flags()

	f.StringP("name", "n", "", "Your name/identifier")
	f.StringP("repo", "r", "", "Restic repository URL")
	joinCmd.MarkFlagRequired("name")
	joinCmd.MarkFlagRequired("repo")

	// SSS mode
	f.StringP("share", "s", "", "Hex-encoded key share from owner")
	f.IntP("index", "i", 0, "Share index (usually 2)")

	// Consensus mode
	f.Bool("consensus", false, "Join in consensus mode (generate key pair)")

	rootCmd.AddCommand(joinCmd)
}

func runJoin(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	repoURL, _ := cmd.Flags().GetString("repo")

	if config.Exists("") {
		return fmt.Errorf("already initialized. Remove ~/.airgapper to reinitialize")
	}

	consensus, _ := cmd.Flags().GetBool("consensus")
	if consensus {
		return joinConsensus(name, repoURL)
	}

	return joinSSS(cmd, name, repoURL)
}

func joinSSS(cmd *cobra.Command, name, repoURL string) error {
	shareHex, _ := cmd.Flags().GetString("share")
	shareIndex, _ := cmd.Flags().GetInt("index")

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

	PrintHeader("Airgapper Join (Backup Host) - SSS Mode")
	PrintInfo("Name:  %s", name)
	PrintInfo("Repo:  %s", repoURL)
	PrintInfo("Share: %d bytes, index %d", len(share), shareIndex)
	fmt.Println()

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

	PrintSuccess("Joined as backup host!")
	fmt.Println()
	PrintInfo("Commands available to you:")
	PrintInfo("  airgapper pending  - List pending restore requests")
	PrintInfo("  airgapper approve  - Approve a restore request")
	PrintInfo("  airgapper deny     - Deny a restore request")
	PrintInfo("  airgapper serve    - Run HTTP API for remote management")

	return nil
}

func joinConsensus(name, repoURL string) error {
	PrintHeader("Airgapper Join (Key Holder) - Consensus Mode")
	PrintInfo("Name: %s", name)
	PrintInfo("Repo: %s", repoURL)
	fmt.Println()

	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}
	keyID := crypto.KeyID(pubKey)

	PrintInfo("1. Generated Ed25519 key pair")
	PrintInfo("   Your Key ID: %s", keyID)

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

	PrintInfo("2. Configuration saved to ~/.airgapper/")
	fmt.Println()
	PrintDivider()
	PrintWarning("IMPORTANT: Register with the vault owner")
	PrintDivider()
	fmt.Println()
	PrintInfo("Share your public key with the vault owner so they can register you:")
	fmt.Println()
	PrintInfo("  Public Key: %s", crypto.EncodePublicKey(pubKey))
	PrintInfo("  Key ID:     %s", keyID)
	PrintDivider()

	PrintSuccess("Joined as key holder!")
	return nil
}
