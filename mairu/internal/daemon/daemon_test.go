package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type upsertCall struct {
	URI      string
	Name     string
	Abstract string
	Overview string
	Content  string
	Parent   string
	Project  string
	Metadata map[string]any
}

type managerStub struct {
	mu      sync.Mutex
	upserts []upsertCall
	deletes []string
}

func (m *managerStub) UpsertFileContextNode(_ context.Context, uri, name, abstractText, overviewText, content, parentURI, project string, metadata map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upserts = append(m.upserts, upsertCall{
		URI: uri, Name: name, Abstract: abstractText, Overview: overviewText, Content: content, Parent: parentURI, Project: project, Metadata: metadata,
	})
	return nil
}

func (m *managerStub) DeleteContextNode(_ context.Context, uri string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletes = append(m.deletes, uri)
	return nil
}

func TestProcessFileStoresAbstractOverviewAndContent(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "src", "domain")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "feature.ts")
	src := strings.Join([]string{
		"import { slugify } from './slug';",
		"function normalize(name: string) { return slugify(name); }",
		"export function greet(name: string) { return normalize(name); }",
		"export class UserService { public run(input: string) { return greet(input); } }",
	}, "\n")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := &managerStub{}
	d := New(mgr, "proj", dir, Options{})
	if err := d.ProcessFile(context.Background(), file); err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if len(mgr.upserts) != 1 {
		t.Fatalf("expected one upsert, got %d", len(mgr.upserts))
	}
	call := mgr.upserts[0]
	if call.URI != "contextfs://proj/src/domain/feature.ts" {
		t.Fatalf("unexpected uri: %s", call.URI)
	}
	if !strings.Contains(call.Overview, "File: src/domain/feature.ts") || !strings.Contains(call.Overview, "Symbols:") {
		t.Fatalf("unexpected overview: %s", call.Overview)
	}
	if call.Abstract == "" || call.Content == "" {
		t.Fatalf("expected non-empty abstract/content")
	}
}

func TestSkipsLargeFilesAndIgnoredDirs(t *testing.T) {
	dir := t.TempDir()
	large := filepath.Join(dir, "large.ts")
	if err := os.WriteFile(large, []byte("export const p = \""+strings.Repeat("x", 2048)+"\";"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignored := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(ignored, 0o755); err != nil {
		t.Fatal(err)
	}
	ignoredFile := filepath.Join(ignored, "index.ts")
	if err := os.WriteFile(ignoredFile, []byte("export const x = 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr := &managerStub{}
	d := New(mgr, "proj", dir, Options{MaxFileSizeBytes: 256})
	_ = d.ProcessFile(context.Background(), large)
	_ = d.ProcessFile(context.Background(), ignoredFile)
	if len(mgr.upserts) != 0 {
		t.Fatalf("expected no upserts, got %d", len(mgr.upserts))
	}
}

func TestDeleteClearsCachesAndCallsManager(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "module.ts")
	_ = os.WriteFile(file, []byte("export function ping(){ return 'pong'; }"), 0o644)
	mgr := &managerStub{}
	d := New(mgr, "proj", dir, Options{})
	_ = d.ProcessFile(context.Background(), file)
	if _, ok := d.fileContentHashes[file]; !ok {
		t.Fatal("expected file hash to be cached")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := d.HandleFileDelete(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	if len(mgr.deletes) != 1 {
		t.Fatalf("expected one delete call, got %d", len(mgr.deletes))
	}
	if _, ok := d.fileContentHashes[file]; ok {
		t.Fatal("expected cache cleared")
	}
}

func TestSkipsMtimeOnlyChanges(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "mod.ts")
	_ = os.WriteFile(file, []byte("export function ping(){ return 'pong'; }"), 0o644)
	mgr := &managerStub{}
	d := New(mgr, "proj", dir, Options{})
	_ = d.ProcessFile(context.Background(), file)
	now := time.Now().Add(time.Second)
	_ = os.Chtimes(file, now, now)
	_ = d.ProcessFile(context.Background(), file)
	if len(mgr.upserts) != 1 {
		t.Fatalf("expected one upsert, got %d", len(mgr.upserts))
	}
}

func TestReUpsertsOnContentChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "math.ts")
	_ = os.WriteFile(file, []byte("export function add(a:number,b:number){ return a+b; }"), 0o644)
	mgr := &managerStub{}
	d := New(mgr, "proj", dir, Options{})
	_ = d.ProcessFile(context.Background(), file)
	_ = os.WriteFile(file, []byte("export function add(a:number,b:number){ const s=a+b; return s; }"), 0o644)
	time.Sleep(2 * time.Millisecond)
	now := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(file, now, now)
	_ = d.ProcessFile(context.Background(), file)
	if len(mgr.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(mgr.upserts))
	}
}
