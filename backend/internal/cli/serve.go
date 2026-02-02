package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lcrostarosa/airgapper/backend/internal/api"
	"github.com/lcrostarosa/airgapper/backend/internal/config"
	"github.com/lcrostarosa/airgapper/backend/internal/restic"
	"github.com/lcrostarosa/airgapper/backend/internal/scheduler"
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
	RunE: runServe,
}

func init() {
	f := serveCmd.Flags()
	f.StringP("addr", "a", "", "Listen address (default: :8081 or AIRGAPPER_PORT)")
	f.String("schedule", "", "Override backup schedule for this session")
	f.String("paths", "", "Override backup paths for this session (comma-separated)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	serveCfg := cfg
	if serveCfg == nil {
		serveCfg = &config.Config{
			ConfigDir: config.DefaultConfigDir(),
		}
	}

	addr := resolveAddr(cmd)
	serveCfg.ListenAddr = addr

	printServerInfo(serveCfg, addr)

	server := api.NewServer(serveCfg, addr)
	sched := setupScheduler(cmd, serveCfg, server)

	return runServer(server, sched)
}

func resolveAddr(cmd *cobra.Command) string {
	addr, _ := cmd.Flags().GetString("addr")
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
	printHeader("Airgapper Server")
	printInfo("Name: %s", serveCfg.Name)
	printInfo("Role: %s", serveCfg.Role)
	printInfo("API:  http://localhost%s", addr)
	fmt.Println()

	printInfo("Endpoints:")
	printInfo("  GET  /health               - Health check")
	printInfo("  GET  /api/status           - System status")
	printInfo("  GET  /api/requests         - List pending requests")
	printInfo("  POST /api/requests         - Create restore request")
	printInfo("  POST /api/requests/{id}/approve - Approve request")
	printInfo("  POST /api/requests/{id}/deny    - Deny request")
	fmt.Println()
}

func setupScheduler(cmd *cobra.Command, serveCfg *config.Config, server *api.Server) *scheduler.Scheduler {
	if !serveCfg.IsOwner() {
		return nil
	}

	scheduleExpr := serveCfg.BackupSchedule
	backupPaths := serveCfg.BackupPaths

	// Allow overrides from flags
	if override, _ := cmd.Flags().GetString("schedule"); override != "" {
		scheduleExpr = override
	}
	if override, _ := cmd.Flags().GetString("paths"); override != "" {
		backupPaths = strings.Split(override, ",")
	}

	if scheduleExpr == "" || len(backupPaths) == 0 {
		if scheduleExpr == "" {
			printInfo("No backup schedule configured.")
			printInfo("   Configure with: airgapper schedule --set daily ~/Documents")
			fmt.Println()
		}
		return nil
	}

	parsedSched, err := scheduler.ParseSchedule(scheduleExpr)
	if err != nil {
		printWarning("Invalid schedule: %v", err)
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
	server.SetScheduler(sched)

	printInfo("Scheduled Backups:")
	printInfo("  Schedule: %s", scheduleExpr)
	printInfo("  Paths:    %s", strings.Join(backupPaths, ", "))
	nextRun := parsedSched.NextRun(time.Now())
	printInfo("  Next:     %s", nextRun.Format("2006-01-02 15:04:05"))
	fmt.Println()

	sched.Start()
	return sched
}

func runServer(server *api.Server, sched *scheduler.Scheduler) error {
	printInfo("Press Ctrl+C to stop")
	fmt.Println()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-stop
	fmt.Println("\nShutting down...")

	if sched != nil {
		sched.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	printInfo("Server stopped.")
	return nil
}
