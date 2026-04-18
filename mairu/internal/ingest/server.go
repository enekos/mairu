package ingest

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"

	"mairu/internal/redact"
)

// BashRepo is the persistence interface for bash history records.
// Task 4 will wire a concrete implementation.
type BashRepo interface {
	InsertBashHistory(ctx context.Context, project, command string, exitCode, durationMs int, output string) error
}

// Server listens on a Unix socket and dispatches ingest records.
type Server struct {
	path     string
	repo     BashRepo
	redactor *redact.Redactor

	// testHook, if non-nil, replaces the default processRecord path.
	// It is unexported so only tests within this package can set it.
	testHook func(context.Context, Record)

	wg sync.WaitGroup
}

// NewServer constructs a Server that will listen at path.
func NewServer(path string, repo BashRepo, redactor *redact.Redactor) *Server {
	return &Server{path: path, repo: repo, redactor: redactor}
}

// Run listens on the Unix socket at s.path and serves record streams until
// ctx is cancelled or an unrecoverable error occurs. On shutdown it closes
// the listener and removes the socket file.
func (s *Server) Run(ctx context.Context) error {
	// Remove any stale socket file from a previous run.
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	listener, err := net.Listen("unix", s.path)
	if err != nil {
		return err
	}

	// Restrict access to the current user only.
	if err := os.Chmod(s.path, 0o600); err != nil {
		listener.Close()
		return err
	}

	acceptErrCh := make(chan error, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					acceptErrCh <- nil
					return
				}
				acceptErrCh <- err
				return
			}
			s.wg.Add(1)
			go s.handleConn(ctx, conn)
		}
	}()

	select {
	case <-ctx.Done():
		// Graceful shutdown path.
	case err := <-acceptErrCh:
		// Unexpected accept error — clean up and propagate.
		listener.Close()
		s.wg.Wait()
		os.Remove(s.path) //nolint:errcheck
		return err
	}

	listener.Close()
	s.wg.Wait()
	os.Remove(s.path) //nolint:errcheck
	return nil
}

// handleConn reads records from conn until EOF or a decode error, then exits.
func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	r := bufio.NewReader(conn)
	for {
		rec, err := Decode(r)
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			slog.Debug("ingest: decode error", "err", err)
			return
		}
		s.processRecord(ctx, rec)
	}
}

// processRecord dispatches rec. If testHook is set it takes priority;
// otherwise redacts the command and persists it via repo.
func (s *Server) processRecord(ctx context.Context, rec Record) {
	if s.testHook != nil {
		s.testHook(ctx, rec)
		return
	}
	project := rec.Project
	if project == "" {
		project = ResolveProject(rec.Cwd)
	}

	result := s.redactor.Redact(rec.Command, redact.KindCommand)
	if result.Dropped {
		slog.Debug("ingest: dropped record after redaction", "cwd", rec.Cwd)
		return
	}
	if err := s.repo.InsertBashHistory(ctx, project, result.Redacted, rec.ExitCode, rec.DurationMs, ""); err != nil {
		slog.Error("ingest: insert failed", "err", err, "project", project)
	}
}
