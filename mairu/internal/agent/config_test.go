package agent

import (
	"context"
	"testing"
)

type noopInterceptor struct{}

func (noopInterceptor) Name() string { return "noop" }

func (noopInterceptor) PreExecute(_ ToolContext, _ string, args map[string]any) (map[string]any, error) {
	return args, nil
}

func TestNormalizeConfig_NoInputReturnsZeroConfig(t *testing.T) {
	got := normalizeConfig()
	if got.SymbolLocator != nil {
		t.Fatalf("expected nil SymbolLocator")
	}
	if len(got.Interceptors) != 0 {
		t.Fatalf("expected empty Interceptors, got %d", len(got.Interceptors))
	}
	if len(got.UTCPProviders) != 0 {
		t.Fatalf("expected empty UTCPProviders, got %d", len(got.UTCPProviders))
	}
	if got.AgentSystemData != nil {
		t.Fatalf("expected nil AgentSystemData")
	}
}

func TestNormalizeConfig_ClonesSlicesAndMaps(t *testing.T) {
	source := Config{
		Unattended:      true,
		HistoryLogger:   nil,
		Interceptors:    []ToolInterceptor{noopInterceptor{}},
		UTCPProviders:   []string{"provider-a"},
		AgentSystemData: map[string]any{"cli_help": "x"},
	}
	got := normalizeConfig(source)

	source.UTCPProviders[0] = "changed"
	source.AgentSystemData["cli_help"] = "y"
	source.Interceptors = nil

	if got.UTCPProviders[0] != "provider-a" {
		t.Fatalf("expected copied UTCPProviders to stay unchanged, got %q", got.UTCPProviders[0])
	}
	if got.AgentSystemData["cli_help"] != "x" {
		t.Fatalf("expected copied AgentSystemData to stay unchanged, got %v", got.AgentSystemData["cli_help"])
	}
	if len(got.Interceptors) != 1 {
		t.Fatalf("expected copied Interceptors to remain populated")
	}
}

func TestChildConfig_ClonesParentConfig(t *testing.T) {
	parent := &Agent{
		Unattended: true,
		interceptors: []ToolInterceptor{
			noopInterceptor{},
		},
		utcpProviders:   []string{"provider-a"},
		AgentSystemData: map[string]any{"k": "v"},
	}

	child := parent.childConfig()
	parent.utcpProviders[0] = "changed"
	parent.AgentSystemData["k"] = "mutated"
	parent.interceptors = nil

	if child.Unattended != true {
		t.Fatalf("expected unattended to propagate")
	}
	if child.UTCPProviders[0] != "provider-a" {
		t.Fatalf("expected copied UTCPProviders, got %q", child.UTCPProviders[0])
	}
	if child.AgentSystemData["k"] != "v" {
		t.Fatalf("expected copied AgentSystemData, got %v", child.AgentSystemData["k"])
	}
	if len(child.Interceptors) != 1 {
		t.Fatalf("expected copied Interceptors to remain populated")
	}

	_, _ = noopInterceptor{}.PreExecute(ToolContext{Context: context.Background()}, "tool", map[string]any{})
}
