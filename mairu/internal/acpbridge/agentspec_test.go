package acpbridge

import "testing"

func TestDefaultAgentSpecsHasMairu(t *testing.T) {
	specs := DefaultAgentSpecs()
	m, ok := specs["mairu"]
	if !ok {
		t.Fatal("mairu spec missing")
	}
	if len(m.Args) == 0 || m.Args[len(m.Args)-1] != "acp" {
		t.Fatalf("mairu spec args = %v", m.Args)
	}
}

func TestDefaultAgentSpecsHasClaudeCodeAndGemini(t *testing.T) {
	specs := DefaultAgentSpecs()
	for _, k := range []string{"claude-code", "gemini"} {
		if _, ok := specs[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
}
