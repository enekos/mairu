package acpbridge

import (
	"context"
	"testing"
	"time"
)

func TestRegistryCreateAndGet(t *testing.T) {
	bin := buildFixture(t)
	r := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	specs := map[string]AgentSpec{"echo": {Command: bin}}
	id, err := r.Create(ctx, "echo", specs, 16)
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
	r := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, _ := r.Create(ctx, "echo", map[string]AgentSpec{"echo": {Command: bin}}, 16)
	if err := r.Delete(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := r.Get(id); ok {
		t.Fatal("session still present after delete")
	}
}

func TestRegistryCreateUnknownAgent(t *testing.T) {
	r := NewRegistry()
	_, err := r.Create(context.Background(), "nope", map[string]AgentSpec{}, 16)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}
