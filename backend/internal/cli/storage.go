package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/api"
	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/server"
	"github.com/lcrostarosa/airgapper/backend/internal/storage"
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Storage server management commands",
	Long: `Commands for managing the standalone storage server.

The storage server provides restic-compatible REST backend functionality
with append-only mode, integrity checking, and quota management.`,
}

var storageServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run standalone storage server",
	Long: `Start a standalone storage server without the full API.

This is useful for running just the storage backend on a dedicated host,
separate from the main Airgapper API server.`,
	Example: `  # Start storage server with defaults
  airgapper storage serve --path /data/backups

  # Start with append-only mode and quota
  airgapper storage serve --path /data/backups --append-only --quota 100GB

  # Start on custom address
  airgapper storage serve --path /data/backups --addr :8000`,
	RunE: runners.Uninitialized().Wrap(runStorageServe),
}

var storageStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show storage server status",
	Long:  `Display the current status of the storage server including disk usage, request counts, and policy information.`,
	RunE:  runners.Uninitialized().Wrap(runStorageStatus),
}

func init() {
	// Storage serve flags
	sf := storageServeCmd.Flags()
	sf.StringP("path", "p", "", "Storage base path (required)")
	sf.StringP("addr", "a", ":8000", "Listen address for storage server")
	sf.Bool("append-only", true, "Enable append-only mode (prevents deletions)")
	sf.String("quota", "", "Storage quota (e.g., 100GB, 1TB)")
	sf.Bool("integrity", true, "Enable integrity checking")
	sf.String("integrity-interval", "24h", "Integrity check interval")

	storageServeCmd.MarkFlagRequired("path")

	// Add subcommands
	storageCmd.AddCommand(storageServeCmd)
	storageCmd.AddCommand(storageStatusCmd)

	// Add to root
	rootCmd.AddCommand(storageCmd)
}

func runStorageServe(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	flags := runner.Flags(cmd)
	path := flags.String("path")
	addr := flags.String("addr")
	appendOnly := flags.Bool("append-only")
	quotaStr := flags.String("quota")
	enableIntegrity := flags.Bool("integrity")
	if err := flags.Err(); err != nil {
		return err
	}

	// Parse quota
	var quotaBytes int64
	if quotaStr != "" {
		parsed, err := parseQuota(quotaStr)
		if err != nil {
			return err
		}
		quotaBytes = parsed
	}

	logging.Info("Starting standalone storage server",
		logging.String("path", path),
		logging.String("addr", addr),
		logging.Bool("appendOnly", appendOnly))

	// Create temporary config for storage initialization
	storageCfg := &config.Config{
		StoragePath:       path,
		StorageAppendOnly: appendOnly,
		StorageQuotaBytes: quotaBytes,
	}

	// Initialize storage components
	opts, err := api.InitStorageComponents(storageCfg)
	if err != nil {
		return err
	}

	if opts.StorageServer == nil {
		logging.Error("Failed to initialize storage server")
		return nil
	}

	// Start components
	api.StartStorageComponents(opts)

	// Create a simple HTTP server for the storage endpoint
	mux := http.NewServeMux()

	// Mount storage handler
	mux.Handle("/", storage.WithLogging(opts.StorageServer.Handler()))

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := opts.StorageServer.Status()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"running":` + boolStr(status.Running) +
			`,"appendOnly":` + boolStr(status.AppendOnly) +
			`,"usedBytes":` + int64Str(status.UsedBytes) +
			`,"diskUsagePct":` + intStr(status.DiskUsagePct) + `}`))
	})

	// Integrity check endpoint (if enabled)
	if enableIntegrity && opts.IntegrityChecker != nil {
		mux.HandleFunc("/integrity/check", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			repoName := r.URL.Query().Get("repo")
			if repoName == "" {
				repoName = "default"
			}
			result, err := opts.IntegrityChecker.CheckDataIntegrity(repoName)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"passed":` + boolStr(result.Passed) +
				`,"totalFiles":` + int64Str(int64(result.TotalFiles)) +
				`,"corruptFiles":` + int64Str(int64(result.CorruptFiles)) + `}`))
		})
	}

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logging.Info("Storage server ready",
		logging.String("addr", addr),
		logging.String("path", path))

	return server.RunWithGracefulShutdown(httpServer, func() {
		api.StopStorageComponents(opts)
	})
}

func runStorageStatus(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	if ctx.Config == nil || ctx.Config.StoragePath == "" {
		logging.Info("Storage not configured")
		return nil
	}

	// Initialize storage to get status
	storageCfg := &config.Config{
		StoragePath:       ctx.Config.StoragePath,
		StorageAppendOnly: ctx.Config.StorageAppendOnly,
		StorageQuotaBytes: ctx.Config.StorageQuotaBytes,
	}

	opts, err := api.InitStorageComponents(storageCfg)
	if err != nil {
		return err
	}

	if opts.StorageServer == nil {
		logging.Info("Storage server not available")
		return nil
	}

	status := opts.StorageServer.Status()

	logging.Info("Storage Server Status",
		logging.String("path", status.BasePath),
		logging.Bool("running", status.Running),
		logging.Bool("appendOnly", status.AppendOnly))

	logging.Info("Disk Usage",
		logging.Int64("usedBytes", status.UsedBytes),
		logging.Int64("quotaBytes", status.QuotaBytes),
		logging.Int("diskUsagePct", status.DiskUsagePct),
		logging.Int64("diskFreeBytes", status.DiskFreeBytes))

	if status.HasPolicy {
		logging.Info("Policy",
			logging.String("policyId", status.PolicyID))
	}

	return nil
}

// Helper functions for JSON formatting
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func int64Str(n int64) string {
	return fmt.Sprintf("%d", n)
}

func intStr(n int) string {
	return fmt.Sprintf("%d", n)
}

// parseQuota parses a human-readable quota string (e.g., "100GB", "1TB")
func parseQuota(s string) (int64, error) {
	var multiplier int64 = 1
	var numStr string

	if len(s) >= 2 {
		suffix := s[len(s)-2:]
		switch suffix {
		case "TB", "tb":
			multiplier = 1024 * 1024 * 1024 * 1024
			numStr = s[:len(s)-2]
		case "GB", "gb":
			multiplier = 1024 * 1024 * 1024
			numStr = s[:len(s)-2]
		case "MB", "mb":
			multiplier = 1024 * 1024
			numStr = s[:len(s)-2]
		case "KB", "kb":
			multiplier = 1024
			numStr = s[:len(s)-2]
		default:
			numStr = s
		}
	} else {
		numStr = s
	}

	var num int64
	_, err := parseIntFromString(numStr, &num)
	if err != nil {
		return 0, err
	}

	return num * multiplier, nil
}

func parseIntFromString(s string, result *int64) (bool, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		n = n*10 + int64(c-'0')
	}
	*result = n
	return true, nil
}
