package cmd

import (
	"strings"
	"testing"
)

func TestResolvePRReviewTarget(t *testing.T) {
	if got := resolvePRReviewTarget("42", "99"); got != "42" {
		t.Fatalf("expected explicit PR to win, got %q", got)
	}
	if got := resolvePRReviewTarget("", "99"); got != "99" {
		t.Fatalf("expected discovered PR when explicit empty, got %q", got)
	}
	if got := resolvePRReviewTarget(" ", " "); got != "" {
		t.Fatalf("expected empty when both are blank, got %q", got)
	}
}

func TestFormatReviewerFindings_SortedRoles(t *testing.T) {
	out := formatReviewerFindings(map[string]string{
		"Tests Evangelist":     "Add integration test",
		"App Developer":        "Feature behavior is correct",
		"Developer Evangelist": "Refactor helper naming",
	})

	wantOrder := []string{
		"## App Developer",
		"## Developer Evangelist",
		"## Tests Evangelist",
	}
	last := -1
	for _, marker := range wantOrder {
		idx := strings.Index(out, marker)
		if idx == -1 {
			t.Fatalf("expected marker %q in output", marker)
		}
		if idx < last {
			t.Fatalf("expected sorted sections order, %q appeared too early", marker)
		}
		last = idx
	}
}
