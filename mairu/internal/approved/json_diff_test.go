package approved

import (
	"strings"
	"testing"
)

func TestJSONDiff_Identical(t *testing.T) {
	a := map[string]any{"name": "foo", "count": float64(1)}
	b := map[string]any{"name": "foo", "count": float64(1)}
	got := jsonFieldDiff(a, b)
	if got != "" {
		t.Errorf("expected empty diff for identical JSON, got:\n%s", got)
	}
}

func TestJSONDiff_ChangedField(t *testing.T) {
	a := map[string]any{"name": "foo"}
	b := map[string]any{"name": "bar"}
	got := jsonFieldDiff(a, b)
	if !strings.Contains(got, "name") || !strings.Contains(got, "foo") || !strings.Contains(got, "bar") {
		t.Errorf("expected field change report, got:\n%s", got)
	}
}

func TestJSONDiff_AddedField(t *testing.T) {
	a := map[string]any{"name": "foo"}
	b := map[string]any{"name": "foo", "age": float64(5)}
	got := jsonFieldDiff(a, b)
	if !strings.Contains(got, "age") || !strings.Contains(got, "added") {
		t.Errorf("expected added field, got:\n%s", got)
	}
}

func TestJSONDiff_RemovedField(t *testing.T) {
	a := map[string]any{"name": "foo", "age": float64(5)}
	b := map[string]any{"name": "foo"}
	got := jsonFieldDiff(a, b)
	if !strings.Contains(got, "age") || !strings.Contains(got, "removed") {
		t.Errorf("expected removed field, got:\n%s", got)
	}
}

func TestJSONDiff_NestedChange(t *testing.T) {
	a := map[string]any{"outer": map[string]any{"inner": "old"}}
	b := map[string]any{"outer": map[string]any{"inner": "new"}}
	got := jsonFieldDiff(a, b)
	if !strings.Contains(got, "outer.inner") {
		t.Errorf("expected nested path, got:\n%s", got)
	}
}

func TestJSONDiff_ArrayElementChange(t *testing.T) {
	a := map[string]any{"items": []any{"a", "b"}}
	b := map[string]any{"items": []any{"a", "c"}}
	got := jsonFieldDiff(a, b)
	if !strings.Contains(got, "items[1]") {
		t.Errorf("expected array index path, got:\n%s", got)
	}
}

func TestJSONDiff_ArrayLengthChange(t *testing.T) {
	a := map[string]any{"items": []any{"a"}}
	b := map[string]any{"items": []any{"a", "b", "c"}}
	got := jsonFieldDiff(a, b)
	if !strings.Contains(got, "added") {
		t.Errorf("expected added elements, got:\n%s", got)
	}
}
