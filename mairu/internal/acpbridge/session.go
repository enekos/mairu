package acpbridge

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Session struct {
	ID    string
	Agent string
	Spec  AgentSpec

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	ring *Ring

	mu          sync.Mutex
	subscribers map[chan StampedFrame]struct{}
	closed      bool
	closeErr    error
	doneCh      chan struct{}
}

// StartSession spawns the agent subprocess and starts the stdout pump.
func StartSession(ctx context.Context, id string, spec AgentSpec, ring *Ring) (*Session, error) {
	if spec.Command == "" {
		return nil, errors.New("acpbridge: empty agent command")
	}
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
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
		ring:        ring,
		subscribers: map[chan StampedFrame]struct{}{},
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
	}
}

func (s *Session) drainStderr() {
	// Discard agent stderr for now. When the bridge gets a slog-backed logger,
	// route this there. Errors from the agent are not protocol traffic.
	_, _ = io.Copy(io.Discard, s.stderr)
}

func (s *Session) waitLoop() {
	s.closeErr = s.cmd.Wait()
	close(s.doneCh)
	s.closeAllSubscribers()
}

func (s *Session) fanout(sf StampedFrame) {
	s.mu.Lock()
	subs := make([]chan StampedFrame, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subs = append(subs, ch)
	}
	s.mu.Unlock()
	for _, ch := range subs {
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
