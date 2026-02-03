package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/emergency"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// --- Override Command (parent) ---

var overrideCmd = &cobra.Command{
	Use:   "override",
	Short: "Manage emergency override keys",
	Long:  `Configure and manage emergency override keys for bypassing normal consensus requirements.`,
}

var overrideSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate a new override key",
	Long: `Generate a new emergency override key.

The key will be displayed once and must be stored securely.
It cannot be recovered if lost.`,
	RunE: runners.Config().Wrap(runOverrideSetup),
}

var overrideAllowCmd = &cobra.Command{
	Use:   "allow <type>",
	Short: "Allow a specific override type",
	Long: `Allow a specific type of emergency override.

Available types:
  restore-without-consensus  - Restore without peer approval
  delete-without-consensus   - Delete without peer approval
  bypass-retention           - Bypass retention period
  bypass-dead-man-switch     - Bypass dead man's switch
  force-unlock               - Force unlock any operation`,
	Args: cobra.ExactArgs(1),
	RunE: runners.Config().Wrap(runOverrideAllow),
}

var overrideDenyCmd = &cobra.Command{
	Use:   "deny <type>",
	Short: "Deny a specific override type",
	Args:  cobra.ExactArgs(1),
	RunE:  runners.Config().Wrap(runOverrideDeny),
}

var overrideListCmd = &cobra.Command{
	Use:   "list",
	Short: "List allowed override types",
	RunE:  runners.Config().Wrap(runOverrideList),
}

var overrideAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View override audit log",
	RunE:  runners.Config().Wrap(runOverrideAudit),
}

func init() {
	overrideCmd.AddCommand(overrideSetupCmd)
	overrideCmd.AddCommand(overrideAllowCmd)
	overrideCmd.AddCommand(overrideDenyCmd)
	overrideCmd.AddCommand(overrideListCmd)
	overrideCmd.AddCommand(overrideAuditCmd)
	rootCmd.AddCommand(overrideCmd)
}

func runOverrideSetup(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	if ctx.Config.Emergency.GetOverride().HasKey() {
		logging.Warn("Override key already configured")
		logging.Info("To reset, remove ~/.airgapper/config.json and reinitialize")
		return nil
	}

	e := ctx.Config.EnsureEmergency()
	if e.Override == nil {
		e.WithOverrides()
	}

	key, err := e.Override.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate override key: %w", err)
	}

	if err := ctx.SaveConfig(); err != nil {
		return err
	}

	logging.Info("Override key generated", logging.String("key", key))
	logging.Warn("IMPORTANT: Store this key securely!")
	logging.Info("  - This key is shown ONCE and cannot be recovered")
	logging.Info("  - Store in a safe deposit box, with a lawyer, etc.")
	logging.Info("  - Anyone with this key can bypass security controls")
	logging.Infof("Use with: airgapper restore --override-key %s --reason \"...\"", key)

	return nil
}

func runOverrideAllow(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	overrideType := args[0]

	e := ctx.Config.EnsureEmergency()
	if e.Override == nil {
		e.WithOverrides()
	}

	e.Override.AllowType(emergency.OverrideType(overrideType))

	if err := ctx.SaveConfig(); err != nil {
		return err
	}

	logging.Info("Override type allowed", logging.String("type", overrideType))
	return nil
}

func runOverrideDeny(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	overrideType := args[0]

	override := ctx.Config.Emergency.GetOverride()
	if override == nil {
		logging.Info("No override configuration found")
		return nil
	}

	override.DenyType(emergency.OverrideType(overrideType))

	if err := ctx.SaveConfig(); err != nil {
		return err
	}

	logging.Info("Override type denied", logging.String("type", overrideType))
	return nil
}

func runOverrideList(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	o := ctx.Config.Emergency.GetOverride()
	if !o.IsEnabled() {
		logging.Info("Override system is not configured")
		logging.Info("To enable, run: airgapper override setup")
		return nil
	}

	logging.Info("Override configuration",
		logging.Bool("enabled", o.Enabled),
		logging.Bool("keyConfigured", o.HasKey()),
		logging.Bool("requireReason", o.RequireReason),
		logging.Bool("notifyOnUse", o.NotifyOnUse))

	if len(o.AllowedTypes) == 0 {
		logging.Info("Allowed override types: (none)")
	} else {
		logging.Info("Allowed override types:")
		for _, ot := range o.AllowedTypes {
			logging.Infof("  - %s", ot)
		}
	}
	return nil
}

func runOverrideAudit(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	auditPath := ctx.Config.ConfigDir + "/override-audit.log"
	data, err := os.ReadFile(auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Info("No override audit log found (no overrides have been used)")
			return nil
		}
		return err
	}

	logging.Info("Override audit log")
	logging.Infof("%s", string(data))
	return nil
}
