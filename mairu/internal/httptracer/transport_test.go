package httptracer

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"testing"
	"time"
)

func newCapturingLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), buf
}

func TestWrap_DisabledReturnsBase(t *testing.T) {
	base := &http.Client{Timeout: 7 * time.Second}
	got := Wrap(base, Options{Enabled: false})
	if got != base {
		t.Fatalf("expected base client when disabled; got %p want %p", got, base)
	}
}

func TestWrap_DisabledWithNilBaseReturnsDefault(t *testing.T) {
	got := Wrap(nil, Options{Enabled: false})
	if got != http.DefaultClient {
		t.Fatalf("expected http.DefaultClient when disabled and base nil")
	}
}

func TestWrap_EnabledInstrumentsAndLogs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	logger, buf := newCapturingLogger()
	client := Wrap(nil, Options{Enabled: true, Component: "test", Logger: logger})
	if client == http.DefaultClient {
		t.Fatal("expected wrapped client, got DefaultClient")
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", srv.URL+"/probe", nil)
	if err != nil {
		t.Fatalf("new req: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected body: %q", body)
	}

	out := buf.String()
	if !strings.Contains(out, `"msg":"httptrace"`) {
		t.Fatalf("expected httptrace log record; got:\n%s", out)
	}
	if !strings.Contains(out, `"component":"test"`) {
		t.Fatalf("expected component=test; got:\n%s", out)
	}
	if !strings.Contains(out, `"path":"/probe"`) {
		t.Fatalf("expected path=/probe; got:\n%s", out)
	}
	if !strings.Contains(out, `"status":200`) {
		t.Fatalf("expected status=200; got:\n%s", out)
	}
	if !strings.Contains(out, `"ttfb_ms":`) {
		t.Fatalf("expected ttfb_ms in record; got:\n%s", out)
	}
	if !strings.Contains(out, `"total_ms":`) {
		t.Fatalf("expected total_ms in record; got:\n%s", out)
	}
}

func TestWrap_PreservesCallerAttachedTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()

	logger, _ := newCapturingLogger()
	client := Wrap(nil, Options{Enabled: true, Component: "compose", Logger: logger})

	var callerFiredFirstByte bool
	callerTrace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() { callerFiredFirstByte = true },
	}
	ctx := httptrace.WithClientTrace(context.Background(), callerTrace)
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if !callerFiredFirstByte {
		t.Fatal("expected caller-attached GotFirstResponseByte to still fire after Wrap composed traces")
	}
}

func TestWrap_ErrorPathStillLogs(t *testing.T) {
	logger, buf := newCapturingLogger()
	client := Wrap(nil, Options{Enabled: true, Component: "err", Logger: logger})
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://127.0.0.1:1/", nil)
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected dial error against port 1")
	}
	if !strings.Contains(buf.String(), `"msg":"httptrace"`) {
		t.Fatalf("expected error-path log record; got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"level":"WARN"`) {
		t.Fatalf("expected WARN level on error; got:\n%s", buf.String())
	}
}

func TestEnvEnabled_TogglesViaEnv(t *testing.T) {
	t.Setenv(envEnable, "1")
	got := Wrap(&http.Client{}, Options{Enabled: false})
	if _, ok := got.Transport.(*TracingTransport); !ok {
		t.Fatalf("expected env-enabled wrap to install TracingTransport, got %T", got.Transport)
	}
}
