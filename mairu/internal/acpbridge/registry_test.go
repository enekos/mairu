package acpbridge

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testRegistry() *Registry { return NewRegistry(testLogger()) }

func defaultSessionOpts() SessionStartOptions {
	return SessionStartOptions{PermissionTimeout: 60 * time.Second, Logger: testLogger()}
}

func TestRegistryCreateAndGet(t *testing.T) {
	bin := buildFixture(t)
	r := testRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	specs := map[string]AgentSpec{"echo": {Command: bin}}
	id, err := r.Create(ctx, "echo", specs, 16, defaultSessionOpts(), 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s, ok := r.Get(id); !ok || s.ID != id {
		t.Fatal("get failed")
	}
	if list := r.List(); len(list) != 1 {
		t.Fatalf("list = %d", len(list))
	}
}

func TestRegistryDeleteClosesSession(t *testing.T) {
	bin := buildFixture(t)
	r := testRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, _ := r.Create(ctx, "echo", map[string]AgentSpec{"echo": {Command: bin}}, 16, defaultSessionOpts(), 0)
	if err := r.Delete(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := r.Get(id); ok {
		t.Fatal("session still present after delete")
	}
}

func TestRegistryCreateUnknownAgent(t *testing.T) {
	r := testRegistry()
	_, err := r.Create(context.Background(), "nope", map[string]AgentSpec{}, 16, defaultSessionOpts(), 0)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestRegistryRespectsSessionCap(t *testing.T) {
	bin := buildFixture(t)
	r := testRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	specs := map[string]AgentSpec{"echo": {Command: bin}}
	if _, err := r.Create(ctx, "echo", specs, 16, defaultSessionOpts(), 1); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := r.Create(ctx, "echo", specs, 16, defaultSessionOpts(), 1)
	if err == nil {
		t.Fatal("expected cap error on second create")
	}
	if err != ErrSessionCap {
		t.Fatalf("err = %v, want ErrSessionCap", err)
	}
}
