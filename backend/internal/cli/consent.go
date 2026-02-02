package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/consent"
	"github.com/lcrostarosa/airgapper/backend/internal/crypto"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
)

// --- Request Command ---

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Request restore approval from peer(s)",
	Long:  `Create a new restore request that must be approved by your peer(s).`,
	Example: `  airgapper request --snapshot latest --reason "Need to recover deleted files"
  airgapper request --snapshot abc123 --reason "Testing restore" --peer http://bob:8081`,
	RunE: runners.Owner().Wrap(runRequest),
}

func init() {
	f := requestCmd.Flags()
	f.String("snapshot", "latest", "Snapshot ID to restore")
	f.String("reason", "", "Reason for restore (required)")
	f.String("peer", "", "Peer address to notify")
	requestCmd.MarkFlagRequired("reason")
	rootCmd.AddCommand(requestCmd)
}

func runRequest(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	flags := runner.Flags(cmd)
	snapshotID := flags.String("snapshot")
	reason := flags.String("reason")
	peerAddr := flags.String("peer")
	if err := flags.Err(); err != nil {
		return err
	}

	req, err := ctx.Consent().CreateRequest(ctx.Config.Name, snapshotID, reason, nil)
	if err != nil {
		return err
	}

	logging.Info("Restore request created",
		logging.String("requestID", req.ID),
		logging.String("snapshot", req.SnapshotID),
		logging.String("reason", req.Reason),
		logging.String("expires", req.ExpiresAt.Format("2006-01-02 15:04:05")))

	// Notify peer if address provided
	if peerAddr == "" && ctx.Config.Peer != nil && ctx.Config.Peer.Address != "" {
		peerAddr = ctx.Config.Peer.Address
	}

	if peerAddr != "" {
		notifyPeer(peerAddr, req)
	}

	logging.Info("Waiting for peer approval...")
	logging.Infof("Share request ID with your peer: %s", req.ID)
	logging.Infof("Once approved, run: airgapper restore --request %s --target /restore/path", req.ID)

	return nil
}

func notifyPeer(peerAddr string, req *consent.RestoreRequest) {
	logging.Info("Notifying peer", logging.String("address", peerAddr))

	reqBody := map[string]interface{}{
		"id":          req.ID,
		"requester":   req.Requester,
		"snapshot_id": req.SnapshotID,
		"reason":      req.Reason,
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(peerAddr+"/api/requests", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		logging.Warn("Could not notify peer - share the request ID manually", logging.Err(err))
		return
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		logging.Info("Peer notified successfully")
	} else {
		logging.Warn("Peer returned unexpected status", logging.Int("status", resp.StatusCode))
	}
}

// --- Pending Command ---

var pendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "List pending restore requests",
	Long:  `Show all restore requests waiting for approval.`,
	RunE:  runners.Config().Wrap(runPending),
}

func init() {
	rootCmd.AddCommand(pendingCmd)
}

func runPending(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	requests, err := ctx.Consent().ListPending()
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		logging.Info("No pending restore requests")
		return nil
	}

	logging.Info("Pending restore requests", logging.Int("count", len(requests)))
	for _, req := range requests {
		logging.Info("Request",
			logging.String("id", req.ID),
			logging.String("from", req.Requester),
			logging.String("snapshot", req.SnapshotID),
			logging.String("reason", req.Reason),
			logging.String("expires", req.ExpiresAt.Format("2006-01-02 15:04")))
	}

	logging.Info("To approve: airgapper approve <request-id>")
	logging.Info("To deny:    airgapper deny <request-id>")

	return nil
}

// --- Approve Command ---

var approveCmd = &cobra.Command{
	Use:   "approve <request-id>",
	Short: "Approve a restore request (sign or release share)",
	Long:  `Approve a pending restore request by signing it or releasing your key share.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runners.Config().Wrap(runApprove),
}

func init() {
	rootCmd.AddCommand(approveCmd)
}

func runApprove(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	requestID := args[0]
	mgr := ctx.Consent()

	if ctx.Config.UsesConsensusMode() || ctx.Config.PrivateKey != nil {
		return approveConsensus(ctx, mgr, requestID)
	}

	return approveSSS(ctx, mgr, requestID)
}

func approveSSS(ctx *runner.CommandContext, mgr *consent.Manager, requestID string) error {
	share, shareIndex, err := ctx.Config.LoadShare()
	if err != nil {
		return fmt.Errorf("failed to load share: %w", err)
	}

	logging.Info("Approving request",
		logging.String("requestID", requestID),
		logging.Int("shareIndex", int(shareIndex)))

	if err := mgr.Approve(requestID, ctx.Config.Name, share); err != nil {
		return err
	}

	logging.Info("Request approved - key share released")
	logging.Info("The requester can now restore their data")

	return nil
}

func approveConsensus(ctx *runner.CommandContext, mgr *consent.Manager, requestID string) error {
	if ctx.Config.PrivateKey == nil {
		return fmt.Errorf("no private key found - cannot sign")
	}

	req, err := mgr.GetRequest(requestID)
	if err != nil {
		return err
	}

	keyID := crypto.KeyID(ctx.Config.PublicKey)
	logging.Info("Signing request",
		logging.String("requestID", requestID),
		logging.String("keyID", keyID))

	signature, err := crypto.SignRestoreRequest(
		ctx.Config.PrivateKey,
		req.ID,
		req.Requester,
		req.SnapshotID,
		req.Reason,
		keyID,
		req.Paths,
		req.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	if err := mgr.AddSignature(requestID, keyID, ctx.Config.Name, signature); err != nil {
		return err
	}

	current, required, _ := mgr.GetApprovalProgress(requestID)

	logging.Info("Request signed",
		logging.Int("approvals", current),
		logging.Int("required", required))

	if current >= required {
		logging.Info("Request is now fully approved - the requester can now restore their data")
	} else {
		logging.Infof("Waiting for %d more approval(s)...", required-current)
	}

	return nil
}

// --- Deny Command ---

var denyCmd = &cobra.Command{
	Use:   "deny <request-id>",
	Short: "Deny a restore request",
	Long:  `Deny a pending restore request.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runners.Config().Wrap(runDeny),
}

func init() {
	rootCmd.AddCommand(denyCmd)
}

func runDeny(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	requestID := args[0]

	if err := ctx.Consent().Deny(requestID, ctx.Config.Name); err != nil {
		return err
	}

	logging.Info("Request denied", logging.String("requestID", requestID))
	return nil
}
