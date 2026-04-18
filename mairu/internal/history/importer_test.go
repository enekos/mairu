package history

import (
	"context"
	"strings"
	"testing"

	"mairu/internal/redact"
)

type fakeBashRepo struct {
	calls []insertCall
}

type insertCall struct {
	project    string
	command    string
	durationMs int
}

func (f *fakeBashRepo) InsertBashHistory(_ context.Context, project, command string, _ int, durationMs int, _ string) error {
	f.calls = append(f.calls, insertCall{project, command, durationMs})
	return nil
}

func TestImport_StoresBenignCommands(t *testing.T) {
	in := strings.NewReader("echo hello\nls -la\npwd\n")
	repo := &fakeBashRepo{}
	res, err := Import(context.Background(), in, FormatBash, repo, redact.New(), "demo", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Parsed != 3 || res.Stored != 3 || res.Redacted != 0 || res.Dropped != 0 {
		t.Errorf("got %+v; want Parsed=3 Stored=3", res)
	}
	if len(repo.calls) != 3 {
		t.Fatalf("repo calls = %d; want 3", len(repo.calls))
	}
	if repo.calls[0].command != "echo hello" {
		t.Errorf("call 0 command = %q", repo.calls[0].command)
	}
}

func TestImport_RedactsSecretCommand(t *testing.T) {
	in := strings.NewReader(`curl -H "Authorization: Bearer abc123randomtoken" https://api.example.com/path
ls
`)
	repo := &fakeBashRepo{}
	res, err := Import(context.Background(), in, FormatBash, repo, redact.New(), "demo", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Parsed != 2 || res.Stored != 2 || res.Redacted != 1 {
		t.Errorf("got %+v; want Parsed=2 Stored=2 Redacted=1", res)
	}
	if strings.Contains(repo.calls[0].command, "abc123randomtoken") {
		t.Errorf("token leaked into storage: %q", repo.calls[0].command)
	}
}

func TestImport_SkipsDropped(t *testing.T) {
	// This command has three Layer-1 hits and nothing else — damage cap
	// should trigger, so the entry is dropped, not stored.
	in := strings.NewReader(`curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhYmMifQ.sig" https://user:hunter2supersecret@api.example.com/path?token=ghp_1234567890abcdefghijklmnopqrstuvwxyz
echo safe
`)
	repo := &fakeBashRepo{}
	res, err := Import(context.Background(), in, FormatBash, repo, redact.New(), "demo", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Dropped != 1 {
		t.Errorf("Dropped = %d; want 1 (%+v)", res.Dropped, res)
	}
	if res.Stored != 1 {
		t.Errorf("Stored = %d; want 1", res.Stored)
	}
	if len(repo.calls) != 1 || repo.calls[0].command != "echo safe" {
		t.Errorf("repo calls = %+v", repo.calls)
	}
}

func TestImport_DeduplicatesByHash(t *testing.T) {
	in := strings.NewReader("echo hello\necho hello\nls\necho hello\n")
	repo := &fakeBashRepo{}
	res, err := Import(context.Background(), in, FormatBash, repo, redact.New(), "demo", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Parsed != 4 || res.Stored != 2 || res.DuplicateSkipped != 2 {
		t.Errorf("got %+v; want Parsed=4 Stored=2 DuplicateSkipped=2", res)
	}
	if len(repo.calls) != 2 {
		t.Errorf("repo calls = %d; want 2", len(repo.calls))
	}
}

func TestImport_DryRun(t *testing.T) {
	in := strings.NewReader("echo hello\nls\n")
	repo := &fakeBashRepo{}
	res, err := Import(context.Background(), in, FormatBash, repo, redact.New(), "demo", true)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Stored != 2 {
		t.Errorf("Stored = %d; want 2", res.Stored)
	}
	if len(repo.calls) != 0 {
		t.Errorf("dry-run made repo calls: %+v", repo.calls)
	}
}

func TestImport_ZshFormat(t *testing.T) {
	in := strings.NewReader(`: 1700000000:0;echo hello
: 1700000100:2;ls -la
`)
	repo := &fakeBashRepo{}
	res, err := Import(context.Background(), in, FormatZsh, repo, redact.New(), "demo", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Stored != 2 {
		t.Errorf("Stored = %d; want 2", res.Stored)
	}
	if repo.calls[1].durationMs != 2000 {
		t.Errorf("call 1 duration = %d; want 2000", repo.calls[1].durationMs)
	}
}
