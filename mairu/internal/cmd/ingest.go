package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/enekos/mairu/pii-redact/pkg/redact"
	"github.com/spf13/cobra"
	"mairu/internal/ingest"
)

// resolveSocketPath returns the Unix socket path for the ingest daemon.
// Honours MAIRU_INGEST_SOCK env var; falls back to ~/.mairu/ingest.sock.
// If $HOME cannot be determined and no env var is set, returns an error.
func resolveSocketPath() (string, error) {
	if v := os.Getenv("MAIRU_INGEST_SOCK"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory and MAIRU_INGEST_SOCK is not set: %w", err)
	}
	return filepath.Join(home, ".mairu", "ingest.sock"), nil
}

// NewIngestRecordSubCmd returns the "record" subcommand intended to be added
// to the existing "ingest" cobra command (defined in cmd_misc.go).
func NewIngestRecordSubCmd() *cobra.Command {
	return newIngestRecordCmd()
}

func newIngestRecordCmd() *cobra.Command {
	var (
		command    string
		exitCode   int
		durationMs int
		cwd        string
		project    string
	)

	recordCmd := &cobra.Command{
		Use:   "record",
		Short: "Send a single shell command record to the ingest daemon",
		// SilenceUsage prevents cobra printing usage on RunE error, keeping
		// the shell prompt clean even when something goes wrong.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if command == "" {
				return fmt.Errorf("--command is required")
			}

			// Step 1: resolve socket path.
			path, err := resolveSocketPath()
			if err != nil {
				// Fail silently — daemon may simply not be running.
				return nil //nolint:nilerr
			}

			// Step 2: dial. On ANY error, exit 0 silently.
			conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
			if err != nil {
				return nil //nolint:nilerr
			}

			// Step 3: build record.
			rec := ingest.Record{
				Timestamp:  time.Now().UTC(),
				Command:    command,
				ExitCode:   exitCode,
				DurationMs: durationMs,
				Cwd:        cwd,
				Project:    project,
			}

			// Step 4: encode. On error, exit 0 silently.
			if err := ingest.Encode(conn, rec); err != nil {
				conn.Close()
				return nil //nolint:nilerr
			}

			// Step 5: close.
			conn.Close()
			return nil
		},
	}

	recordCmd.Flags().StringVar(&command, "command", "", "Shell command string (required)")
	recordCmd.Flags().IntVar(&exitCode, "exit-code", 0, "Exit code of the command")
	recordCmd.Flags().IntVar(&durationMs, "duration-ms", 0, "Execution duration in milliseconds")
	recordCmd.Flags().StringVar(&cwd, "cwd", "", "Working directory where the command ran")
	recordCmd.Flags().StringVar(&project, "project", "", "Project name to associate the record with")

	return recordCmd
}

// NewIngestdCmd returns the "ingestd" top-level command with subcommands.
func NewIngestdCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingestd",
		Short: "Ingest daemon — receives shell records over a Unix socket",
	}
	cmd.AddCommand(newIngestdRunCmd())
	return cmd
}

func newIngestdRunCmd() *cobra.Command {
	var socketFlag string

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start the ingest daemon and listen for incoming shell records",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Step 1: resolve socket path.
			var path string
			if socketFlag != "" {
				path = socketFlag
			} else {
				var err error
				path, err = resolveSocketPath()
				if err != nil {
					return err
				}
			}

			// Step 2: ensure the parent directory exists.
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return fmt.Errorf("create socket directory: %w", err)
			}

			// Step 3: build redactor.
			redactor, err := redact.New(redact.Options{})
			if err != nil {
				return fmt.Errorf("build redactor: %w", err)
			}

			// Step 4: get repo.
			app := GetLocalApp()
			repo := app.Repo()
			if repo == nil {
				return fmt.Errorf("repository is not initialized")
			}

			// Step 5: set up signal-aware context.
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			// Step 6: construct server.
			server := ingest.NewServer(path, repo, redactor)

			// Step 7: print banner.
			fmt.Fprintf(os.Stderr, "[ingestd] listening on %s (pid %d)\n", path, os.Getpid())

			// Step 8: run until context is cancelled.
			return server.Run(ctx)
		},
	}

	runCmd.Flags().StringVar(&socketFlag, "socket", "", "Unix socket path (overrides MAIRU_INGEST_SOCK and default)")

	return runCmd
}
