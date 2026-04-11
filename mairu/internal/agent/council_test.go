package agent

import (
	"strings"
	"testing"
)

func TestCouncilConfig_WithDefaults(t *testing.T) {
	got := (CouncilConfig{}).withDefaults()
	if len(got.Roles) != 3 {
		t.Fatalf("expected 3 default council roles, got %d", len(got.Roles))
	}
}

func TestFormatCouncilFeedback_SortedStableOutput(t *testing.T) {
	formatted := formatCouncilFeedback(map[string]string{
		"Tests Evangelist":     "Add integration tests",
		"App Developer":        "Feature toggle missing",
		"Developer Evangelist": "Architecture is acceptable",
	})

	expectedOrder := []string{
		"## App Developer",
		"## Developer Evangelist",
		"## Tests Evangelist",
	}
	lastIdx := -1
	for _, marker := range expectedOrder {
		idx := strings.Index(formatted, marker)
		if idx == -1 {
			t.Fatalf("expected marker %q in formatted output", marker)
		}
		if idx < lastIdx {
			t.Fatalf("expected sorted output order, got %q before previous marker", marker)
		}
		lastIdx = idx
	}
}

func TestBuildCouncilExecutionPrompt_IncludesTaskSynthesisAndFeedback(t *testing.T) {
	prompt := buildCouncilExecutionPrompt(
		"Implement feature X",
		"Prioritize safety checks",
		map[string]string{"App Developer": "Looks good"},
	)

	assertContains := []string{
		"## Original Task",
		"Implement feature X",
		"## Product Lead Guidance",
		"Prioritize safety checks",
		"## Expert Reviews",
		"## App Developer",
		"Looks good",
	}
	for _, needle := range assertContains {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("expected prompt to contain %q", needle)
		}
	}
}
