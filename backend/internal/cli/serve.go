package cli

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/api"
	"github.com/lcrostarosa/airgapper/backend/internal/cli/runner"
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/logging"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
	"github.com/lcrostarosa/airgapper/backend/internal/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run HTTP API server (with optional scheduled backups)",
	Long: `Start the HTTP API server for remote management.

If you're the data owner and have a backup schedule configured,
scheduled backups will run automatically while the server is running.`,
	Example: `  # Start server on default port (8081)
  airgapper serve

  # Start server on custom port
  airgapper serve --addr :8080

  # Override schedule for this session
  airgapper serve --schedule daily --paths ~/Documents,~/Photos`,
	RunE: runners.Uninitialized().Wrap(runServe),
}

func init() {
	f := serveCmd.Flags()
	f.StringP("addr", "a", "", "Listen address (default: :8081 or AIRGAPPER_PORT)")
	f.String("schedule", "", "Override backup schedule for this session")
	f.String("paths", "", "Override backup paths for this session (comma-separated)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(ctx *runner.CommandContext, cmd *cobra.Command, args []string) error {
	serveCfg := ctx.Config
	if serveCfg == nil {
		serveCfg = &config.Config{
			ConfigDir: config.DefaultConfigDir(),
		}
	}

	addr := resolveAddr(cmd)
	serveCfg.ListenAddr = addr

	printServerInfo(serveCfg, addr)

	apiServer := api.NewServer(serveCfg, addr)
	sched := setupScheduler(cmd, serveCfg, apiServer)

	return runServer(apiServer, sched)
}

func resolveAddr(cmd *cobra.Command) string {
	flags := runner.Flags(cmd)
	addr := flags.String("addr")

	if addr != "" {
		return addr
	}

	addr = os.Getenv("AIRGAPPER_PORT")
	if addr == "" {
		addr = ":8081"
	}
	if addr[0] != ':' {
		addr = ":" + addr
	}

	return addr
}

func printServerInfo(serveCfg *config.Config, addr string) {
	logging.Info("Airgapper server starting",
		logging.String("name", serveCfg.Name),
		logging.String("role", string(serveCfg.Role)),
		logging.String("api", "http://localhost"+addr))

	logging.Info("Endpoints available:")
	logging.Info("  GET  /health               - Health check")
	logging.Info("  GET  /api/status           - System status")
	logging.Info("  GET  /api/requests         - List pending requests")
	logging.Info("  POST /api/requests         - Create restore request")
	logging.Info("  POST /api/requests/{id}/approve - Approve request")
	logging.Info("  POST /api/requests/{id}/deny    - Deny request")
}

func setupScheduler(cmd *cobra.Command, serveCfg *config.Config, apiServer *api.Server) *scheduler.Scheduler {
	if !serveCfg.IsOwner() {
		return nil
	}

	scheduleExpr := serveCfg.BackupSchedule
	backupPaths := serveCfg.BackupPaths

	// Allow overrides from flags
	flags := runner.Flags(cmd)
	if override := flags.String("schedule"); flags.Changed("schedule") && override != "" {
		scheduleExpr = override
	}
	if override := flags.String("paths"); flags.Changed("paths") && override != "" {
		backupPaths = strings.Split(override, ",")
	}

	if scheduleExpr == "" || len(backupPaths) == 0 {
		if scheduleExpr == "" {
			logging.Info("No backup schedule configured - configure with: airgapper schedule --set daily ~/Documents")
		}
		return nil
	}

	parsedSched, err := scheduler.ParseSchedule(scheduleExpr)
	if err != nil {
		logging.Warn("Invalid schedule", logging.Err(err))
		return nil
	}

	backupFunc := func() error {
		client := restic.NewClient(serveCfg.RepoURL, serveCfg.Password)
		err := client.Backup(backupPaths, []string{"airgapper", "scheduled"})
		if err == nil {
			serveCfg.RecordActivity()
		}
		return err
	}

	sched := scheduler.NewScheduler(parsedSched, backupFunc)
	apiServer.SetScheduler(sched)

	nextRun := parsedSched.NextRun(time.Now())
	logging.Info("Scheduled backups enabled",
		logging.String("schedule", scheduleExpr),
		logging.String("paths", strings.Join(backupPaths, ", ")),
		logging.String("nextRun", nextRun.Format("2006-01-02 15:04:05")))

	sched.Start()
	return sched
}

func runServer(apiServer *api.Server, sched *scheduler.Scheduler) error {
	logging.Info("Press Ctrl+C to stop")

	httpServer := &http.Server{
		Addr:    apiServer.Addr(),
		Handler: apiServer.Handler(),
	}

	return server.RunWithGracefulShutdown(httpServer, func() {
		if sched != nil {
			sched.Stop()
		}
	})
}
