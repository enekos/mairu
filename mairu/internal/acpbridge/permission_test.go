package acpbridge

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPermissionTimeoutSynthesizesDenial(t *testing.T) {
	pm := NewPermissionMux(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	gotAgent := make(chan []byte, 1)
	pm.OnTimeout = func(synthetic []byte) { gotAgent <- synthetic }

	pm.Track(ctx, []byte("42"), `{"jsonrpc":"2.0","id":42,"method":"session/request_permission","params":{}}`)

	select {
	case b := <-gotAgent:
		s := string(b)
		if !strings.Contains(s, `"id":42`) || !strings.Contains(s, "outcome") {
			t.Fatalf("synthetic = %s", s)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout never fired")
	}
}

func TestPermissionResolveBeforeTimeout(t *testing.T) {
	pm := NewPermissionMux(time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	timeoutFired := make(chan struct{})
	pm.OnTimeout = func([]byte) { close(timeoutFired) }

	pm.Track(ctx, []byte("7"), `{"jsonrpc":"2.0","id":7,"method":"session/request_permission","params":{}}`)
	pm.Resolve([]byte("7"))
	select {
	case <-timeoutFired:
		t.Fatal("timeout fired despite Resolve")
	case <-time.After(100 * time.Millisecond):
	}
}
