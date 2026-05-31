package acpbridge

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SessionStartOptions tunes Session creation.
type SessionStartOptions struct {
	PermissionTimeout time.Duration // forwarded to PermissionMux (default 60s)
	Logger            *slog.Logger  // optional; nil → discard
	StderrTailLines   int           // ring depth for agent stderr (default 64)
}

type Session struct {
	ID    string
	Agent string
	Spec  AgentSpec

	PermissionMux *PermissionMux // optional; nil disables fallback

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	logger *slog.Logger

	ring *Ring

	mu          sync.Mutex
	subscribers map[chan StampedFrame]struct{}
	stderrTail  []string
	stderrCap   int
	closed      bool
	closeErr    error
	doneCh      chan struct{}
}

// StartSession spawns the agent subprocess and starts the stdout pump.
//
// The provided ctx governs only the startup phase (spec validation, pipe
// allocation, process launch); the subprocess itself is detached from ctx and
// lives until Close is called or it exits on its own. This is intentional —
// the HTTP handler typically passes its own request context, and we must not
// let the subprocess die when the response is written.
func StartSession(ctx context.Context, id string, spec AgentSpec, ring *Ring, opts SessionStartOptions) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if spec.Command == "" {
		return nil, errors.New("acpbridge: empty agent command")
	}
	if opts.PermissionTimeout == 0 {
		opts.PermissionTimeout = 60 * time.Second
	}
	if opts.StderrTailLines <= 0 {
		opts.StderrTailLines = 64
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	// Detached from ctx on purpose — see doc comment above.
	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Env = spec.Env
	cmd.Dir = spec.Dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", spec.Command, err)
	}
	s := &Session{
		ID: id, Spec: spec, cmd: cmd,
		stdin: stdin, stdout: stdout, stderr: stderr,
		logger:      logger,
		ring:        ring,
		subscribers: map[chan StampedFrame]struct{}{},
		stderrCap:   opts.StderrTailLines,
		doneCh:      make(chan struct{}),
	}
	go s.readLoop()
	go s.drainStderr()
	go s.waitLoop()
	return s, nil
}

func (s *Session) readLoop() {
	sc := bufio.NewScanner(s.stdout)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		// Copy: scanner reuses its buffer.
		frame := make([]byte, len(line))
		copy(frame, line)
		id := s.ring.Push(frame)
		s.fanout(StampedFrame{ID: id, Frame: frame})
		if s.PermissionMux != nil && bytes.Contains(frame, []byte(`"method":"session/request_permission"`)) {
			if rid := extractRequestID(frame); rid != nil {
				s.PermissionMux.Track(context.Background(), rid, string(frame))
			}
		}
	}
	if err := sc.Err(); err != nil {
		s.logger.Warn("acpbridge: agent stdout read error", "session", s.ID, "err", err)
	}
}

func (s *Session) drainStderr() {
	sc := bufio.NewScanner(s.stderr)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		s.mu.Lock()
		s.stderrTail = append(s.stderrTail, line)
		if len(s.stderrTail) > s.stderrCap {
			s.stderrTail = s.stderrTail[len(s.stderrTail)-s.stderrCap:]
		}
		s.mu.Unlock()
		s.logger.Debug("acpbridge: agent stderr", "session", s.ID, "line", line)
	}
}

// StderrTail returns a snapshot of the most recent agent stderr lines.
func (s *Session) StderrTail() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.stderrTail) == 0 {
		return nil
	}
	out := make([]string, len(s.stderrTail))
	copy(out, s.stderrTail)
	return out
}

// ExitErr returns the error from cmd.Wait, or nil if the agent has not exited
// or exited cleanly.
func (s *Session) ExitErr() error {
	select {
	case <-s.doneCh:
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.closeErr
	default:
		return nil
	}
}

// ExitMessage returns a human-readable summary of how the agent exited, or ""
// if it has not exited yet. Includes the stderr tail when present.
func (s *Session) ExitMessage() string {
	select {
	case <-s.doneCh:
	default:
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	parts := []string{}
	if s.closeErr != nil {
		parts = append(parts, s.closeErr.Error())
	} else {
		parts = append(parts, "agent exited")
	}
	if len(s.stderrTail) > 0 {
		parts = append(parts, "stderr: "+strings.Join(s.stderrTail, " | "))
	}
	return strings.Join(parts, "; ")
}

func (s *Session) waitLoop() {
	err := s.cmd.Wait()
	s.mu.Lock()
	s.closeErr = err
	s.mu.Unlock()
	close(s.doneCh)
	s.closeAllSubscribers()
}

// fanout delivers sf to every current subscriber on a non-blocking basis.
// The session mutex is held for the whole fan-out to serialize against
// Unsubscribe and closeAllSubscribers — so a subscriber channel cannot be
// closed underneath us mid-send. The send is non-blocking (default branch),
// so the lock is held only for as long as a slice walk takes.
func (s *Session) fanout(sf StampedFrame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- sf:
		default:
			// Slow subscriber: drop. Replay via Last-Event-ID will recover.
		}
	}
}

// Subscribe returns a channel that receives every future stamped frame.
// Caller must call Unsubscribe to free resources.
func (s *Session) Subscribe() <-chan StampedFrame {
	ch := make(chan StampedFrame, 64)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func (s *Session) Unsubscribe(ch <-chan StampedFrame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.subscribers {
		if c == ch {
			delete(s.subscribers, c)
			close(c)
			return
		}
	}
}

// Send writes a frame to the agent's stdin. The frame must NOT include a
// trailing newline — Send appends one.
func (s *Session) Send(frame []byte) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("session closed")
	}
	s.mu.Unlock()
	if s.PermissionMux != nil {
		// A response from a client. Best-effort: if it carries an id and no
		// method, treat as permission resolution.
		if !bytes.Contains(frame, []byte(`"method":`)) {
			if id := extractRequestID(frame); id != nil {
				s.PermissionMux.Resolve(id)
			}
		}
	}
	if _, err := s.stdin.Write(frame); err != nil {
		return err
	}
	if _, err := s.stdin.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

func (s *Session) closeAllSubscribers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, ch)
	}
	s.closed = true
}

func (s *Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		<-s.doneCh
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	_ = s.stdin.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	<-s.doneCh
	return nil
}

// Done returns a channel that is closed when the agent process exits.
func (s *Session) Done() <-chan struct{} { return s.doneCh }

// extractRequestID does a best-effort extraction of the JSON-RPC id from
// a frame. It returns nil if no id is found. It does NOT fully parse the
// JSON; this is intentional — frames must be passed through as-is.
func extractRequestID(frame []byte) []byte {
	const key = `"id":`
	i := bytes.Index(frame, []byte(key))
	if i < 0 {
		return nil
	}
	j := i + len(key)
	for j < len(frame) && (frame[j] == ' ' || frame[j] == '\t') {
		j++
	}
	end := j
	depth := 0
	for end < len(frame) {
		c := frame[end]
		if depth == 0 && (c == ',' || c == '}') {
			break
		}
		if c == '{' || c == '[' {
			depth++
		}
		if c == '}' || c == ']' {
			depth--
		}
		end++
	}
	return frame[j:end]
}
