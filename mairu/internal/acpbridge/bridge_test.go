package acpbridge

import "testing"

func TestNewBridgeRequiresAddr(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error when Addr is empty")
	}
}

func TestNewBridgeOK(t *testing.T) {
	b, err := New(Options{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if b == nil {
		t.Fatal("nil bridge")
	}
}
