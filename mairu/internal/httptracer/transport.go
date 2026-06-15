// Package httptracer is a stdlib-only wrapper around net/http/httptrace that
// turns the per-request lifecycle (DNS / TCP / TLS / first-byte / body-done)
// into structured slog events.
//
// It is opt-in: callers always wrap their http.Client through Wrap(), but the
// wrapper only attaches a ClientTrace when MAIRU_HTTPTRACE=1 (or when New is
// called with Enabled: true explicitly). Disabled paths return the original
// client untouched — zero allocation, zero hook overhead.
package httptracer

import (
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"sync"
	"time"
)

const envEnable = "MAIRU_HTTPTRACE"

// Options configures a TracingTransport. Logger defaults to slog.Default().
// Component is a short label (e.g. "kimi", "embedder", "ctxclient") emitted as
// a structured field so multiple wrapped clients are distinguishable in logs.
type Options struct {
	Enabled   bool
	Component string
	Logger    *slog.Logger
}

// Wrap returns a *http.Client whose Transport instruments every request with
// httptrace hooks. If base is nil, http.DefaultClient is used as a template
// (its Timeout/CheckRedirect are copied; the Transport is replaced).
//
// When opts.Enabled is false AND the env var is unset, the original client is
// returned unchanged.
func Wrap(base *http.Client, opts Options) *http.Client {
	if !opts.Enabled && !envEnabled() {
		if base == nil {
			return http.DefaultClient
		}
		return base
	}

	src := base
	if src == nil {
		src = http.DefaultClient
	}
	inner := src.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	wrapped := &http.Client{
		Transport:     &TracingTransport{Base: inner, Component: opts.Component, Logger: logger},
		CheckRedirect: src.CheckRedirect,
		Jar:           src.Jar,
		Timeout:       src.Timeout,
	}
	return wrapped
}

func envEnabled() bool {
	v := strings.TrimSpace(os.Getenv(envEnable))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// TracingTransport wraps a RoundTripper and emits one slog record per request
// with DNS / connect / TLS / TTFB / body-done breakdowns.
//
// Caller-attached ClientTraces are preserved: httptrace.WithClientTrace
// composes traces rather than replacing them.
type TracingTransport struct {
	Base      http.RoundTripper
	Component string
	Logger    *slog.Logger
}

type timings struct {
	mu                  sync.Mutex
	start               time.Time
	dnsStart, dnsDone   time.Time
	connStart, connDone time.Time
	tlsStart, tlsDone   time.Time
	gotConn             time.Time
	firstByte           time.Time
	bodyDone            time.Time
	reused              bool
	wasIdle             bool
	idleTime            time.Duration
	dnsErr, connErr     error
}

// RoundTrip implements http.RoundTripper.
func (t *TracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	logger := t.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ts := &timings{start: time.Now()}
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			ts.mu.Lock()
			ts.dnsStart = time.Now()
			ts.mu.Unlock()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			ts.mu.Lock()
			ts.dnsDone = time.Now()
			ts.dnsErr = info.Err
			ts.mu.Unlock()
		},
		ConnectStart: func(string, string) {
			ts.mu.Lock()
			ts.connStart = time.Now()
			ts.mu.Unlock()
		},
		ConnectDone: func(_, _ string, err error) {
			ts.mu.Lock()
			ts.connDone = time.Now()
			ts.connErr = err
			ts.mu.Unlock()
		},
		TLSHandshakeStart: func() {
			ts.mu.Lock()
			ts.tlsStart = time.Now()
			ts.mu.Unlock()
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			ts.mu.Lock()
			ts.tlsDone = time.Now()
			ts.mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			ts.mu.Lock()
			ts.gotConn = time.Now()
			ts.reused = info.Reused
			ts.wasIdle = info.WasIdle
			ts.idleTime = info.IdleTime
			ts.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			ts.mu.Lock()
			ts.firstByte = time.Now()
			ts.mu.Unlock()
		},
	}
	ctx := httptrace.WithClientTrace(req.Context(), trace)
	req = req.WithContext(ctx)

	resp, err := base.RoundTrip(req)
	if err != nil {
		t.emit(logger, req, resp, ts, err, true)
		return resp, err
	}

	resp.Body = &tracedBody{
		ReadCloser: resp.Body,
		onClose: func() {
			ts.mu.Lock()
			ts.bodyDone = time.Now()
			ts.mu.Unlock()
			t.emit(logger, req, resp, ts, nil, false)
		},
	}
	return resp, nil
}

func (t *TracingTransport) emit(logger *slog.Logger, req *http.Request, resp *http.Response, ts *timings, err error, headersOnly bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	attrs := []slog.Attr{
		slog.String("component", t.Component),
		slog.String("method", req.Method),
		slog.String("host", req.URL.Host),
		slog.String("path", req.URL.Path),
	}
	if resp != nil {
		attrs = append(attrs, slog.Int("status", resp.StatusCode))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	if !ts.dnsDone.IsZero() && !ts.dnsStart.IsZero() {
		attrs = append(attrs, slog.Duration("dns_ms", ts.dnsDone.Sub(ts.dnsStart)))
	}
	if !ts.connDone.IsZero() && !ts.connStart.IsZero() {
		attrs = append(attrs, slog.Duration("connect_ms", ts.connDone.Sub(ts.connStart)))
	}
	if !ts.tlsDone.IsZero() && !ts.tlsStart.IsZero() {
		attrs = append(attrs, slog.Duration("tls_ms", ts.tlsDone.Sub(ts.tlsStart)))
	}
	if !ts.firstByte.IsZero() {
		attrs = append(attrs, slog.Duration("ttfb_ms", ts.firstByte.Sub(ts.start)))
	}
	if !ts.bodyDone.IsZero() {
		attrs = append(attrs, slog.Duration("total_ms", ts.bodyDone.Sub(ts.start)))
	} else if headersOnly {
		attrs = append(attrs, slog.Duration("total_ms", time.Since(ts.start)))
	}
	if !ts.gotConn.IsZero() {
		attrs = append(attrs, slog.Bool("reused_conn", ts.reused))
		if ts.wasIdle {
			attrs = append(attrs, slog.Duration("idle_ms", ts.idleTime))
		}
	}

	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelWarn
	}
	logger.LogAttrs(req.Context(), level, "httptrace", attrs...)
}

// tracedBody fires onClose exactly once when the response body is closed,
// regardless of whether the caller drained it. Critical for streaming LLM
// responses where the meaningful "done" moment is Close, not first-byte.
type tracedBody struct {
	io.ReadCloser
	once    sync.Once
	onClose func()
}

func (b *tracedBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.onClose)
	return err
}
