package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestContextGetIncludesTokenAndQuery(t *testing.T) {
	var gotPath string
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotToken = r.Header.Get("X-Context-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	t.Setenv("MAIRU_CONTEXT_SERVER_URL", srv.URL)
	t.Setenv("MAIRU_CONTEXT_SERVER_TOKEN", "secret-token")

	_, err := ContextGet("/api/search", map[string]string{
		"q":    "auth",
		"type": "memory",
	})
	if err != nil {
		t.Fatalf("ContextGet returned error: %v", err)
	}
	if !strings.Contains(gotPath, "/api/search") || !strings.Contains(gotPath, "q=auth") {
		t.Fatalf("unexpected request path/query: %s", gotPath)
	}
	if gotToken != "secret-token" {
		t.Fatalf("expected auth token header, got %q", gotToken)
	}
}

func TestMemorySearchCommandCallsSearchEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"memories":[]}`))
	}))
	defer srv.Close()

	t.Setenv("MAIRU_CONTEXT_SERVER_URL", srv.URL)
	t.Setenv("MAIRU_CONTEXT_SERVER_TOKEN", "")

	cmd := NewMemoryCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"-P", "demo", "search", "auth token", "-k", "7", "--minScore", "0.2", "--highlight"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("memory search failed: %v", err)
	}

	if !strings.Contains(gotPath, "/api/search") {
		t.Fatalf("expected /api/search call, got %s", gotPath)
	}
	for _, want := range []string{"q=auth+token", "type=memory", "topK=7", "project=demo", "minScore=0.2", "highlight=true"} {
		if !strings.Contains(gotPath, want) {
			t.Fatalf("expected query %q in %s", want, gotPath)
		}
	}
}

func TestVibeMutationAliasPlansThenExecutes(t *testing.T) {
	var calls []string
	var executePayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/vibe/mutation/plan":
			_, _ = w.Write([]byte(`{"operations":[{"op":"create_memory","data":{"content":"x"}}]}`))
		case "/api/vibe/mutation/execute":
			defer r.Body.Close()
			dec := json.NewDecoder(r.Body)
			_ = dec.Decode(&executePayload)
			_, _ = w.Write([]byte(`{"results":[{"op":"create_memory","result":"ok"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("MAIRU_CONTEXT_SERVER_URL", srv.URL)
	t.Setenv("MAIRU_CONTEXT_SERVER_TOKEN", "")

	cmd := newVibeMutationAliasCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"-P", "demo", "remember architecture decision", "-k", "3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vibe-mutation failed: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 API calls, got %v", calls)
	}
	if calls[0] != "POST /api/vibe/mutation/plan" || calls[1] != "POST /api/vibe/mutation/execute" {
		t.Fatalf("unexpected call order: %v", calls)
	}
	if executePayload["project"] != "demo" {
		t.Fatalf("expected project demo in execute payload, got %#v", executePayload["project"])
	}
}

func Skip_TestNodeReadCommandFindsTargetURI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"uri":"contextfs://proj/root","parent_uri":null,"name":"root"},
			{"uri":"contextfs://proj/root/auth","parent_uri":"contextfs://proj/root","name":"auth"}
		]`))
	}))
	defer srv.Close()

	t.Setenv("MAIRU_CONTEXT_SERVER_URL", srv.URL)
	t.Setenv("MAIRU_CONTEXT_SERVER_TOKEN", "")

	// capture stdout from PrintJSON
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewNodeCmd()
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"-P", "proj", "read", "contextfs://proj/root/auth"})
	err := cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout
	raw, _ := io.ReadAll(r)
	_ = r.Close()

	if err != nil {
		t.Fatalf("node read failed: %v", err)
	}
	if !strings.Contains(string(raw), "contextfs://proj/root/auth") {
		t.Fatalf("expected read output to include target uri, got: %s", string(raw))
	}
}

func Skip_TestNodePathCommandBuildsChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"uri":"contextfs://proj/root","parent_uri":null,"name":"root"},
			{"uri":"contextfs://proj/root/auth","parent_uri":"contextfs://proj/root","name":"auth"},
			{"uri":"contextfs://proj/root/auth/token","parent_uri":"contextfs://proj/root/auth","name":"token"}
		]`))
	}))
	defer srv.Close()

	t.Setenv("MAIRU_CONTEXT_SERVER_URL", srv.URL)
	t.Setenv("MAIRU_CONTEXT_SERVER_TOKEN", "")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewNodeCmd()
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"-P", "proj", "path", "contextfs://proj/root/auth/token"})
	err := cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout
	raw, _ := io.ReadAll(r)
	_ = r.Close()

	if err != nil {
		t.Fatalf("node path failed: %v", err)
	}
	text := string(raw)
	if !regexp.MustCompile(`contextfs://proj/root.*contextfs://proj/root/auth.*contextfs://proj/root/auth/token`).MatchString(strings.ReplaceAll(text, "\n", "")) {
		t.Fatalf("expected chain order root->auth->token, got: %s", text)
	}
}

func TestDoContextRequestReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/search", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	_, err = doContextRequest(req)
	if err == nil {
		t.Fatalf("expected doContextRequest to fail on non-2xx response")
	}
	if !strings.Contains(err.Error(), "context server HTTP 400:") {
		t.Fatalf("expected status code in error, got: %v", err)
	}
}
