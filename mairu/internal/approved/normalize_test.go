package approved

import (
	"strings"
	"testing"
)

func TestNormalizeJSON_NilSlice(t *testing.T) {
	type S struct {
		Items []string `json:"items"`
	}
	got, err := normalizeJSON(S{Items: nil})
	if err != nil {
		t.Fatal(err)
	}
	expected := "{\n  \"items\": []\n}\n"
	if string(got) != expected {
		t.Errorf("nil slice not normalized.\nExpected:\n%s\nGot:\n%s", expected, string(got))
	}
}

func TestNormalizeJSON_NilMap(t *testing.T) {
	type S struct {
		Meta map[string]string `json:"meta"`
	}
	got, err := normalizeJSON(S{Meta: nil})
	if err != nil {
		t.Fatal(err)
	}
	expected := "{\n  \"meta\": {}\n}\n"
	if string(got) != expected {
		t.Errorf("nil map not normalized.\nExpected:\n%s\nGot:\n%s", expected, string(got))
	}
}

func TestNormalizeJSON_NestedNils(t *testing.T) {
	type Inner struct {
		Tags []string `json:"tags"`
	}
	type Outer struct {
		Items []Inner `json:"items"`
	}
	got, err := normalizeJSON(Outer{Items: []Inner{{Tags: nil}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"tags": []`) {
		t.Errorf("nested nil slice not normalized:\n%s", string(got))
	}
}

func TestNormalizeJSON_TrailingNewline(t *testing.T) {
	got, err := normalizeJSON(map[string]int{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if got[len(got)-1] != '\n' {
		t.Error("missing trailing newline")
	}
}

func TestNormalizeJSON_AlreadyPopulated(t *testing.T) {
	type S struct {
		Items []string `json:"items"`
	}
	got, err := normalizeJSON(S{Items: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"a"`) {
		t.Errorf("populated slice should be preserved:\n%s", string(got))
	}
}
