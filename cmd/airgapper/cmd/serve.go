package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lcrostarosa/airgapper/internal/api"
	"github.com/lcrostarosa/airgapper/internal/config"
	"github.com/lcrostarosa/airgapper/internal/restic"
	"github.com/lcrostarosa/airgapper/internal/scheduler"
	"github.com/spf13/cobra"
)

var (
	serveAddr     string
	serveSchedule string
	servePaths    string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run HTTP API server (with optional scheduled backups)",
	Long: `Start the Airgapper HTTP API server.

The server provides REST endpoints for:
  - System status and health checks
  - Restore request management
  - Schedule configuration

For data owners, it can also run scheduled backups.

Examples:
  airgapper serve --addr :8080
  airgapper serve --schedule daily --paths ~/Documents,~/Pictures`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&serveAddr, "addr", "a", ":8080", "Listen address")
	serveCmd.Flags().StringVarP(&serveSchedule, "schedule", "s", "", "Override backup schedule")
	serveCmd.Flags().StringVarP(&servePaths, "paths", "p", "", "Override backup paths (comma-separated)")
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	// Update config with listen address
	cfg.ListenAddr = serveAddr

	fmt.Println("🌐 Airgapper Server")
	fmt.Println("===================")
	fmt.Printf("Name: %s\n", cfg.Name)
	fmt.Printf("Role: %s\n", cfg.Role)
	fmt.Printf("API:  http://localhost%s\n\n", serveAddr)

	fmt.Println("Endpoints:")
	fmt.Println("  GET  /health               - Health check")
	fmt.Println("  GET  /api/status           - System status")
	fmt.Println("  GET  /api/requests         - List pending requests")
	fmt.Println("  POST /api/requests         - Create restore request")
	fmt.Println("  GET  /api/requests/{id}    - Get request details")
	fmt.Println("  POST /api/requests/{id}/approve - Approve request")
	fmt.Println("  POST /api/requests/{id}/deny    - Deny request")
	fmt.Println("  GET  /api/schedule         - Get schedule info")
	fmt.Println("  POST /api/schedule         - Update schedule")
	fmt.Println()

	server := api.NewServer(cfg, serveAddr)

	// Set up scheduler if owner and schedule is configured
	var sched *scheduler.Scheduler
	if cfg.IsOwner() {
		scheduleExpr := cfg.BackupSchedule
		backupPaths := cfg.BackupPaths

		// Override from command line
		if serveSchedule != "" {
			scheduleExpr = serveSchedule
		}
		if servePaths != "" {
			backupPaths = strings.Split(servePaths, ",")
		}

		if scheduleExpr != "" && len(backupPaths) > 0 {
			parsedSched, err := scheduler.ParseSchedule(scheduleExpr)
			if err != nil {
				return fmt.Errorf("invalid schedule: %w", err)
			}

			// Create backup function
			backupFunc := func() error {
				client := restic.NewClient(cfg.RepoURL, cfg.Password)
				return client.Backup(backupPaths, []string{"airgapper", "scheduled"})
			}

			sched = scheduler.NewScheduler(parsedSched, backupFunc)
			server.SetScheduler(sched)

			fmt.Println("📅 Scheduled Backups:")
			fmt.Printf("  Schedule: %s\n", scheduleExpr)
			fmt.Printf("  Paths:    %s\n", strings.Join(backupPaths, ", "))
			nextRun := parsedSched.NextRun(time.Now())
			fmt.Printf("  Next:     %s\n", nextRun.Format("2006-01-02 15:04:05"))
			fmt.Println()

			sched.Start()
		} else if cfg.IsOwner() {
			if scheduleExpr == "" {
				fmt.Println("📅 No backup schedule configured.")
				fmt.Println("   Configure with: airgapper schedule --set daily ~/Documents")
			} else if len(backupPaths) == 0 {
				fmt.Println("📅 Schedule configured but no paths specified.")
				fmt.Println("   Add paths with: airgapper schedule --set daily ~/Documents")
			}
			fmt.Println()
		}
	}

	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Handle shutdown
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

	// Stop scheduler
	if sched != nil {
		sched.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	fmt.Println("Server stopped.")
	return nil
}
