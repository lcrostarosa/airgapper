package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/emergency"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
	"github.com/lcrostarosa/airgapper/backend/internal/sss"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize as data owner (creates repo, splits key)",
	Long: `Initialize a new Airgapper vault as the data owner.

This creates a restic repository and splits the encryption password
using Shamir's Secret Sharing. You keep one share, and give the
other to your backup host.`,
	Example: `  # Standard 2-of-2 initialization
  airgapper init --name alice --repo rest:http://bob-nas:8000/backup

  # With recovery shares (2-of-4 scheme)
  airgapper init --name alice --repo rest:http://bob-nas:8000/backup \
    --recovery-shares 4 --recovery-threshold 2 \
    --custodian "Lawyer" --custodian "Family"

  # With consensus mode (m-of-n key holders)
  airgapper init --name alice --repo rest:http://bob-nas:8000/backup \
    --threshold 2 --holders 3`,
	RunE: runners.Uninitialized().Wrap(runInit),
}

func init() {
	f := initCmd.Flags()

	// Required
	f.StringP("name", "n", "", "Your name/identifier")
	f.StringP("repo", "r", "", "Restic repository URL")
	_ = initCmd.MarkFlagRequired("name")
	_ = initCmd.MarkFlagRequired("repo")

	// SSS mode options
	f.Int("recovery-shares", 2, "Total shares to create")
	f.Int("recovery-threshold", 2, "Shares needed to restore")
	f.StringSlice("custodian", nil, "Custodian name (can specify multiple)")

	// Consensus mode options
	f.Int("threshold", 0, "Approval threshold (enables consensus mode)")
	f.Int("holders", 0, "Total key holders")

	// Emergency options
	f.String("dead-man-switch", "", "Days of inactivity before trigger (e.g., 180d)")
	f.StringSlice("escalation-contact", nil, "Escalation contact (can specify multiple)")
	f.Bool("enable-overrides", false, "Enable emergency override keys")

	rootCmd.AddCommand(initCmd)
}

func runInit(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	if !restic.IsInstalled() {
		return fmt.Errorf("restic is not installed - please install it first: https://restic.net")
	}

	flags := runner.Flags(cmd)
	name := flags.String("name")
	repoURL := flags.String("repo")
	threshold := flags.Int("threshold")
	holders := flags.Int("holders")
	if err := flags.Err(); err != nil {
		return err
	}

	if config.Exists("") {
		return fmt.Errorf("already initialized. Remove ~/.airgapper to reinitialize")
	}

	if threshold > 0 || holders > 0 {
		return initConsensus(cmd, name, repoURL, threshold, holders)
	}

	return initSSS(cmd, name, repoURL)
}

func initSSS(cmd *cobra.Command, name, repoURL string) error {
	flags := runner.Flags(cmd)
	recoveryShares := flags.Int("recovery-shares")
	recoveryThreshold := flags.Int("recovery-threshold")
	custodians := flags.StringSlice("custodian")
	deadManSwitch := flags.String("dead-man-switch")
	enableOverrides := flags.Bool("enable-overrides")
	escalationContacts := flags.StringSlice("escalation-contact")
	if err := flags.Err(); err != nil {
		return err
	}

	deadManDays := parseDays(deadManSwitch)

	if recoveryThreshold > recoveryShares {
		return fmt.Errorf("recovery threshold (%d) cannot exceed total shares (%d)", recoveryThreshold, recoveryShares)
	}

	logging.Info("Airgapper initialization (Data Owner) - SSS Mode",
		logging.String("name", name),
		logging.String("repo", repoURL))
	if recoveryShares > 2 {
		logging.Infof("Recovery: %d-of-%d shares", recoveryThreshold, recoveryShares)
	}

	// Generate password
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}
	password := hex.EncodeToString(passwordBytes)
	logging.Info("Generated secure repository password")

	// Split using SSS
	shares, err := sss.Split([]byte(password), recoveryThreshold, recoveryShares)
	if err != nil {
		return fmt.Errorf("failed to split password: %w", err)
	}
	logging.Infof("Split password into %d shares (%d-of-%d required)", recoveryShares, recoveryThreshold, recoveryShares)

	// Initialize restic repo
	logging.Info("Initializing restic repository...")
	client := restic.NewClient(repoURL, password)
	if err := client.Init(cmd.Context()); err != nil {
		return fmt.Errorf("failed to init repo: %w", err)
	}
	logging.Info("Repository initialized successfully")

	// Build config
	newCfg := &config.Config{
		Name:       name,
		Role:       config.RoleOwner,
		RepoURL:    repoURL,
		Password:   password,
		LocalShare: shares[0].Data,
		ShareIndex: shares[0].Index,
	}

	// Configure emergency features
	if recoveryShares > 2 || deadManDays > 0 || enableOverrides {
		newCfg.Emergency = emergency.NewConfig()

		if recoveryShares > 2 {
			newCfg.Emergency.WithRecovery(recoveryThreshold, recoveryShares, custodians)
		}

		if deadManDays > 0 {
			newCfg.Emergency.WithDeadManSwitch(deadManDays, escalationContacts)
		}

		if enableOverrides {
			newCfg.Emergency.WithOverrides()
		}
	}

	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	logging.Info("Configuration saved to ~/.airgapper/")

	// Output shares
	printShareInfo(shares, repoURL, recoveryThreshold, recoveryShares, custodians)

	if newCfg.Emergency != nil {
		printEmergencyFeatures(newCfg.Emergency)
	}

	logging.Info("Initialization complete")
	printInitNextSteps(recoveryShares > 2)

	return nil
}

func initConsensus(cmd *cobra.Command, name, repoURL string, threshold, holders int) error {
	if threshold < 1 {
		threshold = 1
	}
	if holders < threshold {
		holders = threshold
	}

	logging.Info("Airgapper initialization (Data Owner) - Consensus Mode",
		logging.String("name", name),
		logging.String("repo", repoURL),
		logging.Int("threshold", threshold),
		logging.Int("holders", holders))

	// Generate password
	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}
	password := hex.EncodeToString(passwordBytes)
	logging.Info("Generated secure repository password")

	// Generate Ed25519 key pair
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}
	keyID := crypto.KeyID(pubKey)
	logging.Info("Generated Ed25519 key pair", logging.String("keyID", keyID))

	// Initialize restic repo
	logging.Info("Initializing restic repository...")
	client := restic.NewClient(repoURL, password)
	if err := client.Init(cmd.Context()); err != nil {
		return fmt.Errorf("failed to init repo: %w", err)
	}
	logging.Info("Repository initialized successfully")

	// Build config
	ownerHolder := config.KeyHolder{
		ID:        keyID,
		Name:      name,
		PublicKey: pubKey,
		JoinedAt:  time.Now(),
		IsOwner:   true,
	}

	newCfg := &config.Config{
		Name:       name,
		Role:       config.RoleOwner,
		RepoURL:    repoURL,
		Password:   password,
		PublicKey:  pubKey,
		PrivateKey: privKey,
		Consensus: &config.ConsensusConfig{
			Threshold:       threshold,
			TotalKeys:       holders,
			KeyHolders:      []config.KeyHolder{ownerHolder},
			RequireApproval: threshold > 1 || holders > 1,
		},
	}

	if err := newCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	logging.Info("Configuration saved to ~/.airgapper/")

	if holders > 1 {
		logging.Warn("IMPORTANT: Invite other key holders to join",
			logging.Int("needed", holders-1),
			logging.Int("threshold", threshold),
			logging.Int("total", holders))
		logging.Infof("They should run: airgapper join --name <their-name> --repo '%s' --consensus", repoURL)
	}

	logging.Info("Initialization complete")
	return nil
}

func printShareInfo(shares []sss.Share, repoURL string, k, n int, custodians []string) {
	logging.Warn("IMPORTANT: Share this with your backup host")
	peerShare := hex.EncodeToString(shares[1].Data)
	logging.Infof("Share: %s", peerShare)
	logging.Infof("Index: %d", shares[1].Index)
	logging.Infof("Repo: %s", repoURL)
	logging.Infof("They should run: airgapper join --name <their-name> --repo '%s' --share %s --index %d", repoURL, peerShare, shares[1].Index)

	if n > 2 {
		logging.Info("RECOVERY CUSTODIAN SHARES")
		logging.Infof("These shares can be used to recover if you lose access to share 1. Any %d of %d shares can reconstruct the password.", k, n)

		for i := 2; i < n; i++ {
			custName := fmt.Sprintf("Custodian %d", i-1)
			if len(custodians) > i-2 {
				custName = custodians[i-2]
			}
			custShare := hex.EncodeToString(shares[i].Data)
			logging.Infof("Share %d (%s): %s", shares[i].Index, custName, custShare)
		}
		logging.Warn("Store these shares securely! They can decrypt your backups!")
	}
}

func printEmergencyFeatures(e *emergency.Config) {
	logging.Info("Emergency features enabled")
	if e.Recovery != nil && e.Recovery.Enabled {
		logging.Infof("Recovery shares: %d-of-%d", e.Recovery.Threshold, e.Recovery.TotalShares)
	}
	if e.DeadManSwitch != nil && e.DeadManSwitch.Enabled {
		logging.Infof("Dead man's switch: %d days", e.DeadManSwitch.InactivityDays)
	}
	if e.Override != nil && e.Override.Enabled {
		logging.Info("Override key: run 'airgapper override setup' to generate")
	}
}

func printInitNextSteps(hasRecovery bool) {
	logging.Info("Next steps:")
	logging.Info("  1. Give the host share above to your backup host")
	if hasRecovery {
		logging.Info("  2. Securely distribute custodian shares to trusted parties")
	}
	logging.Info("  3. Configure backup schedule: airgapper schedule --set daily ~/Documents")
	logging.Info("  4. Run: airgapper backup <paths>  (or start server for scheduled backups)")
}

func parseDays(s string) int {
	if s == "" {
		return 0
	}
	s = strings.TrimSuffix(s, "d")
	var days int
	_, _ = fmt.Sscanf(s, "%d", &days)
	return days
}
