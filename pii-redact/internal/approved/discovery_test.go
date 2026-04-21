package approved

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMapInputToApprovedJSON(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"testdata/ts/foo.input.ts", "testdata/ts/foo.approved.json"},
		{"testdata/go/bar.input.go", "testdata/go/bar.approved.json"},
		{"testdata/nl/baz.input.ts", "testdata/nl/baz.approved.json"},
		{"no-input-marker.ts", "no-input-marker.ts.approved.json"},
	}
	for _, tt := range tests {
		got := MapInputToApprovedJSON(tt.input)
		if got != tt.want {
			t.Errorf("MapInputToApprovedJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapInputToApprovedMD(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"testdata/nl/foo.input.ts", "testdata/nl/foo.approved.md"},
		{"testdata/nl/bar.input.go", "testdata/nl/bar.approved.md"},
	}
	for _, tt := range tests {
		got := MapInputToApprovedMD(tt.input)
		if got != tt.want {
			t.Errorf("MapInputToApprovedMD(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDiscoverInputs(t *testing.T) {
	dir := t.TempDir()
	// Create fake input files
	sub := filepath.Join(dir, "ts")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "a.input.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(sub, "b.input.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(sub, "c.other.ts"), []byte(""), 0644)

	got, err := DiscoverInputs(filepath.Join(dir, "*/*.input.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 inputs, got %d: %v", len(got), got)
	}
}

func TestCheckFixtures_AllPresent(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "ts")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "a.input.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(sub, "a.approved.json"), []byte(""), 0644)

	missing, err := CheckFixtures(FixtureRule{
		InputGlob: filepath.Join(dir, "*/*.input.*"),
		MapFunc:   MapInputToApprovedJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing, got: %v", missing)
	}
}

func TestCheckFixtures_Missing(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "ts")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "a.input.ts"), []byte(""), 0644)
	// No approved file

	missing, err := CheckFixtures(FixtureRule{
		InputGlob: filepath.Join(dir, "*/*.input.*"),
		MapFunc:   MapInputToApprovedJSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 {
		t.Errorf("expected 1 missing, got %d: %v", len(missing), missing)
	}
}

func TestCheckFixtures_MultipleRules(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "nl")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "a.input.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(sub, "a.approved.json"), []byte(""), 0644)
	// Missing .approved.md

	missing, err := CheckFixtures(
		FixtureRule{InputGlob: filepath.Join(dir, "*/*.input.*"), MapFunc: MapInputToApprovedJSON},
		FixtureRule{InputGlob: filepath.Join(dir, "nl/*.input.ts"), MapFunc: MapInputToApprovedMD},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 {
		t.Errorf("expected 1 missing, got %d: %v", len(missing), missing)
	}
}
