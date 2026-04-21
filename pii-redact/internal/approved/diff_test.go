package approved

import (
	"fmt"
	"strings"
	"testing"
)

func TestUnifiedDiff_NoChange(t *testing.T) {
	a := "line1\nline2\nline3\n"
	b := "line1\nline2\nline3\n"
	got := unifiedDiff(a, b, "expected", "actual", 3)
	if got != "" {
		t.Errorf("expected empty diff for identical input, got:\n%s", got)
	}
}

func TestUnifiedDiff_SingleLineChange(t *testing.T) {
	a := "line1\nline2\nline3\n"
	b := "line1\nchanged\nline3\n"
	got := unifiedDiff(a, b, "expected", "actual", 3)

	if !strings.Contains(got, "--- expected") {
		t.Errorf("missing --- header:\n%s", got)
	}
	if !strings.Contains(got, "+++ actual") {
		t.Errorf("missing +++ header:\n%s", got)
	}
	if !strings.Contains(got, "-line2") {
		t.Errorf("missing removed line:\n%s", got)
	}
	if !strings.Contains(got, "+changed") {
		t.Errorf("missing added line:\n%s", got)
	}
}

func TestUnifiedDiff_Addition(t *testing.T) {
	a := "line1\nline2\n"
	b := "line1\nline2\nline3\n"
	got := unifiedDiff(a, b, "expected", "actual", 3)

	if !strings.Contains(got, "+line3") {
		t.Errorf("missing added line:\n%s", got)
	}
}

func TestUnifiedDiff_Deletion(t *testing.T) {
	a := "line1\nline2\nline3\n"
	b := "line1\nline3\n"
	got := unifiedDiff(a, b, "expected", "actual", 3)

	if !strings.Contains(got, "-line2") {
		t.Errorf("missing removed line:\n%s", got)
	}
}

func TestUnifiedDiff_ContextLines(t *testing.T) {
	// 10 lines, change line 5 — context=1 should show lines 4,5,6 only
	var aLines, bLines []string
	for i := 1; i <= 10; i++ {
		aLines = append(aLines, fmt.Sprintf("line%d", i))
		if i == 5 {
			bLines = append(bLines, "changed5")
		} else {
			bLines = append(bLines, fmt.Sprintf("line%d", i))
		}
	}
	a := strings.Join(aLines, "\n") + "\n"
	b := strings.Join(bLines, "\n") + "\n"
	got := unifiedDiff(a, b, "expected", "actual", 1)

	// Should NOT contain line1, line2, line3 (too far from change)
	if strings.Contains(got, " line1") {
		t.Errorf("context=1 should not include line1:\n%s", got)
	}
	// Should contain line4 as context
	if !strings.Contains(got, " line4") {
		t.Errorf("context=1 should include line4:\n%s", got)
	}
}
