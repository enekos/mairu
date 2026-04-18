package history

import (
	"strings"
	"testing"
	"time"
)

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		path string
		want Format
	}{
		{"/home/me/.zsh_history", FormatZsh},
		{"/tmp/backup.zsh_history", FormatZsh},
		{"/home/me/.zhistory", FormatZsh},
		{"/home/me/.bash_history", FormatBash},
		{"/tmp/random_file", FormatBash},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := DetectFormat(c.path); got != c.want {
				t.Errorf("DetectFormat(%q) = %v; want %v", c.path, got, c.want)
			}
		})
	}
}

func TestParseZshBasic(t *testing.T) {
	in := `: 1700000000:0;echo hello
: 1700000100:2;ls -la
: 1700000200:0;pwd
`
	got, err := ParseZsh(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3: %+v", len(got), got)
	}
	if got[0].Command != "echo hello" {
		t.Errorf("entry 0 command = %q", got[0].Command)
	}
	if got[1].DurationMs != 2000 {
		t.Errorf("entry 1 duration = %d; want 2000", got[1].DurationMs)
	}
	want := time.Unix(1700000100, 0).UTC()
	if !got[1].Timestamp.Equal(want) {
		t.Errorf("entry 1 ts = %v; want %v", got[1].Timestamp, want)
	}
}

func TestParseZshSkipsMalformed(t *testing.T) {
	in := `: 1700000000:0;echo hello
: not-a-number:0;broken
: 1700000100:also-broken;broken
not a zsh line at all
: 1700000200:0;
: 1700000300:0;pwd
`
	got, err := ParseZsh(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Only the first and last lines are well-formed.
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2: %+v", len(got), got)
	}
	if got[0].Command != "echo hello" || got[1].Command != "pwd" {
		t.Errorf("got commands = [%q, %q]", got[0].Command, got[1].Command)
	}
}

func TestParseZshMultiLineCommand(t *testing.T) {
	in := `: 1700000000:0;for i in 1 2 3; do\
  echo $i\
done
: 1700000100:0;echo done
`
	got, err := ParseZsh(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Command, "for i in") || !strings.Contains(got[0].Command, "echo $i") || !strings.Contains(got[0].Command, "done") {
		t.Errorf("multi-line command missing pieces: %q", got[0].Command)
	}
	if got[1].Command != "echo done" {
		t.Errorf("entry 1 = %q", got[1].Command)
	}
}

func TestParseBashWithoutTimestamps(t *testing.T) {
	in := "echo hello\nls -la\npwd\n"
	got, err := ParseBash(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3", len(got))
	}
	for _, e := range got {
		if !e.Timestamp.IsZero() {
			t.Errorf("unexpected timestamp on %q: %v", e.Command, e.Timestamp)
		}
	}
}

func TestParseBashWithTimestamps(t *testing.T) {
	in := "#1700000000\necho hello\n#1700000100\nls -la\npwd\n"
	got, err := ParseBash(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3: %+v", len(got), got)
	}
	want := time.Unix(1700000000, 0).UTC()
	if !got[0].Timestamp.Equal(want) {
		t.Errorf("entry 0 ts = %v; want %v", got[0].Timestamp, want)
	}
	if !got[2].Timestamp.IsZero() {
		t.Errorf("entry 2 should have no timestamp (no preceding #), got %v", got[2].Timestamp)
	}
}

func TestParseBashEmptyLinesSkipped(t *testing.T) {
	in := "echo hello\n\n\nls\n"
	got, err := ParseBash(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2: %+v", len(got), got)
	}
}
