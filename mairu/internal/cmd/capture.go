package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"mairu/internal/ingest"
)

// NewCaptureCmd returns the `mairu capture` command: opt-in per-invocation
// output capture that pipes a command's stdout+stderr through this process
// (preserving the user's real-time view) while buffering a copy that is
// sent to mairu ingestd alongside the usual exit-code / duration metadata.
//
// Unlike the always-on shell hook (metadata only), `mairu capture` records
// command OUTPUT — the highest-leakage surface in shell history — so the
// daemon runs it through internal/redact.KindText before persistence.
//
// Usage:
//
//	mairu capture [-- cmd args...]
//	mairu capture --max-output 8192 -- npm test
//	mairu capture --project api -- make deploy
//
// If the daemon isn't running, the client still executes the command and
// exits with the child's exit code; the record is silently dropped.
func NewCaptureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "capture [flags] -- <cmd> [args...]",
		Short:              "Run a command with output capture, send redacted record to ingestd",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: false,
		RunE:               runCapture,
	}
	cmd.Flags().StringP("project", "P", "", "Project name (default: inferred by the daemon from cwd)")
	cmd.Flags().Int("max-output", 64*1024, "Max bytes of captured output to send (0 disables capture)")
	cmd.Flags().Duration("timeout", 0, "Kill the command after this duration (0 = no timeout)")
	return cmd
}

func runCapture(cmd *cobra.Command, args []string) error {
	project, _ := cmd.Flags().GetString("project")
	maxOutput, _ := cmd.Flags().GetInt("max-output")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Resolve the program. Look it up once here so we can report a clean
	// error if it's missing, instead of letting Go format an opaque
	// "exec: <name>: executable file not found" message.
	binPath, lookErr := exec.LookPath(args[0])
	if lookErr != nil {
		return fmt.Errorf("capture: %w", lookErr)
	}

	childCmd := exec.CommandContext(ctx, binPath, args[1:]...)
	childCmd.Stdin = os.Stdin

	var buf bytes.Buffer
	// When maxOutput is 0 the user explicitly opted out of capture — in
	// that case we only record metadata. Otherwise cap the buffer with a
	// limit writer so unbounded output (e.g. `yes`) doesn't blow up.
	if maxOutput > 0 {
		capped := &cappedWriter{limit: maxOutput}
		childCmd.Stdout = io.MultiWriter(os.Stdout, capped)
		childCmd.Stderr = io.MultiWriter(os.Stderr, capped)
		// Assign buf via the cappedWriter's underlying buffer.
		capped.buf = &buf
	} else {
		childCmd.Stdout = os.Stdout
		childCmd.Stderr = os.Stderr
	}

	start := time.Now()
	runErr := childCmd.Run()
	durationMs := int(time.Since(start).Milliseconds())

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = 124 // timeout — convention from GNU `timeout(1)`.
		} else {
			return fmt.Errorf("capture: failed to run: %w", runErr)
		}
	}

	rec := ingest.Record{
		Command:    joinArgs(args),
		ExitCode:   exitCode,
		DurationMs: durationMs,
		Cwd:        cwd,
		Timestamp:  start.UTC(),
		Project:    project,
		Output:     buf.String(),
	}
	sendCaptureRecord(rec)

	// Re-exit with the child's exit code so `mairu capture` is drop-in
	// replaceable in any pipeline that previously invoked the command
	// directly.
	os.Exit(exitCode)
	return nil // unreachable; satisfies cobra's RunE signature.
}

// joinArgs reassembles argv into a displayable command line. This is
// deliberately naive: args with spaces aren't quoted. For retrieval
// purposes an approximate rendering is better than raw argv slices.
func joinArgs(argv []string) string {
	var b bytes.Buffer
	for i, a := range argv {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(a)
	}
	return b.String()
}

// sendCaptureRecord dials the ingest daemon and best-effort sends the
// record. All errors are swallowed — the design contract is that capture
// never surfaces ingest failures to the user.
func sendCaptureRecord(rec ingest.Record) {
	path, err := resolveSocketPath()
	if err != nil {
		return
	}
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = ingest.Encode(conn, rec)
}

// cappedWriter buffers up to `limit` bytes and silently discards the rest.
// Used to bound the buffered copy of child output without truncating what
// the terminal sees.
type cappedWriter struct {
	buf    *bytes.Buffer
	limit  int
	capped bool
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	if c.capped || c.buf == nil {
		return len(p), nil
	}
	remaining := c.limit - c.buf.Len()
	if remaining <= 0 {
		c.capped = true
		return len(p), nil
	}
	if len(p) <= remaining {
		return c.buf.Write(p)
	}
	_, _ = c.buf.Write(p[:remaining])
	c.capped = true
	return len(p), nil
}
