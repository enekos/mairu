package acpbridge

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestBridgeStartShutdown(t *testing.T) {
	b, _ := New(Options{Addr: "127.0.0.1:0"})
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- b.Start(ctx) }()

	deadline := time.After(2 * time.Second)
	for b.ListenAddr() == "" {
		select {
		case <-deadline:
			t.Fatal("Start never bound")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	resp, err := http.Get("http://" + b.ListenAddr() + "/sessions")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Start err: %v", err)
	}
}
