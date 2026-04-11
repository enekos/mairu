package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

type e2eMemory struct {
	ID         string `json:"id"`
	Project    string `json:"project"`
	Content    string `json:"content"`
	Category   string `json:"category"`
	Owner      string `json:"owner"`
	Importance int    `json:"importance"`
}

type e2eSkill struct {
	ID          string `json:"id"`
	Project     string `json:"project"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type e2eNode struct {
	URI       string         `json:"uri"`
	Project   string         `json:"project"`
	ParentURI string         `json:"parent_uri,omitempty"`
	Name      string         `json:"name"`
	Abstract  string         `json:"abstract"`
	Overview  string         `json:"overview,omitempty"`
	Content   string         `json:"content,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type e2eContextAPI struct {
	mu           sync.Mutex
	nextMemoryID int
	nextSkillID  int
	memories     map[string]e2eMemory
	skills       map[string]e2eSkill
	nodes        map[string]e2eNode
	deletedNodes map[string]e2eNode
}

func newE2EContextAPI() *e2eContextAPI {
	return &e2eContextAPI{
		nextMemoryID: 1,
		nextSkillID:  1,
		memories:     map[string]e2eMemory{},
		skills:       map[string]e2eSkill{},
		nodes:        map[string]e2eNode{},
		deletedNodes: map[string]e2eNode{},
	}
}

func (a *e2eContextAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/memories":
		a.handleMemories(w, r)
	case r.URL.Path == "/api/skills":
		a.handleSkills(w, r)
	case r.URL.Path == "/api/context":
		a.handleContext(w, r)
	case r.URL.Path == "/api/context/restore":
		a.handleRestoreNode(w, r)
	case r.URL.Path == "/api/search":
		a.handleSearch(w, r)
	case r.URL.Path == "/api/vibe/query":
		writeJSON(w, http.StatusOK, map[string]any{
			"reasoning": "ok",
			"results":   []map[string]any{},
		})
	case r.URL.Path == "/api/vibe/mutation/plan":
		writeJSON(w, http.StatusOK, map[string]any{
			"operations": []map[string]any{
				{
					"op":          "create_memory",
					"description": "store summary",
					"data": map[string]any{
						"content": "planned memory",
					},
				},
			},
		})
	case r.URL.Path == "/api/vibe/mutation/execute":
		var payload struct {
			Project    string           `json:"project"`
			Operations []map[string]any `json:"operations"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		writeJSON(w, http.StatusOK, map[string]any{
			"results": []map[string]any{
				{"op": "create_memory", "result": "ok"},
			},
		})
	case r.URL.Path == "/api/vibe/ingest":
		var payload struct {
			Text    string `json:"text"`
			BaseURI string `json:"base_uri"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		writeJSON(w, http.StatusOK, map[string]any{
			"nodes": []map[string]any{
				{
					"URI":       payload.BaseURI + "/ingested-node",
					"Name":      "Ingested Node",
					"Abstract":  "Generated from text",
					"Overview":  "",
					"Content":   payload.Text,
					"ParentURI": payload.BaseURI,
				},
			},
		})
	default:
		http.NotFound(w, r)
	}
}

func (a *e2eContextAPI) handleMemories(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch r.Method {
	case http.MethodPost:
		var in e2eMemory
		_ = json.NewDecoder(r.Body).Decode(&in)
		id := fmt.Sprintf("mem_%d", a.nextMemoryID)
		a.nextMemoryID++
		in.ID = id
		a.memories[id] = in
		writeJSON(w, http.StatusCreated, in)
	case http.MethodGet:
		out := make([]e2eMemory, 0, len(a.memories))
		for _, m := range a.memories {
			if sameProject(r.URL.Query(), m.Project) {
				out = append(out, m)
			}
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPut:
		var in e2eMemory
		_ = json.NewDecoder(r.Body).Decode(&in)
		cur, ok := a.memories[in.ID]
		if !ok {
			http.Error(w, "memory not found", http.StatusNotFound)
			return
		}
		if in.Content != "" {
			cur.Content = in.Content
		}
		if in.Category != "" {
			cur.Category = in.Category
		}
		if in.Owner != "" {
			cur.Owner = in.Owner
		}
		if in.Importance > 0 {
			cur.Importance = in.Importance
		}
		a.memories[in.ID] = cur
		writeJSON(w, http.StatusOK, cur)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		delete(a.memories, id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *e2eContextAPI) handleSkills(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch r.Method {
	case http.MethodPost:
		var in e2eSkill
		_ = json.NewDecoder(r.Body).Decode(&in)
		id := fmt.Sprintf("skill_%d", a.nextSkillID)
		a.nextSkillID++
		in.ID = id
		a.skills[id] = in
		writeJSON(w, http.StatusCreated, in)
	case http.MethodGet:
		out := make([]e2eSkill, 0, len(a.skills))
		for _, s := range a.skills {
			if sameProject(r.URL.Query(), s.Project) {
				out = append(out, s)
			}
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		delete(a.skills, id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *e2eContextAPI) handleContext(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch r.Method {
	case http.MethodPost:
		var in e2eNode
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.URI == "" {
			http.Error(w, "uri required", http.StatusBadRequest)
			return
		}
		a.nodes[in.URI] = in
		delete(a.deletedNodes, in.URI)
		writeJSON(w, http.StatusCreated, in)
	case http.MethodGet:
		project := strings.TrimSpace(r.URL.Query().Get("project"))
		parentFilter := strings.TrimSpace(r.URL.Query().Get("parentUri"))
		out := make([]e2eNode, 0, len(a.nodes))
		for _, n := range a.nodes {
			if project != "" && n.Project != project {
				continue
			}
			if parentFilter != "" && n.ParentURI != parentFilter {
				continue
			}
			out = append(out, n)
		}
		slices.SortFunc(out, func(a, b e2eNode) int { return strings.Compare(a.URI, b.URI) })
		writeJSON(w, http.StatusOK, out)
	case http.MethodPut:
		var in e2eNode
		_ = json.NewDecoder(r.Body).Decode(&in)
		cur, ok := a.nodes[in.URI]
		if !ok {
			http.Error(w, "node not found", http.StatusNotFound)
			return
		}
		if in.Name != "" {
			cur.Name = in.Name
		}
		if in.Abstract != "" {
			cur.Abstract = in.Abstract
		}
		if in.Overview != "" {
			cur.Overview = in.Overview
		}
		if in.Content != "" {
			cur.Content = in.Content
		}
		a.nodes[in.URI] = cur
		writeJSON(w, http.StatusOK, cur)
	case http.MethodDelete:
		uri := r.URL.Query().Get("uri")
		if n, ok := a.nodes[uri]; ok {
			a.deletedNodes[uri] = n
		}
		delete(a.nodes, uri)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *e2eContextAPI) handleRestoreNode(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var req struct {
		URI string `json:"uri"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	n, ok := a.deletedNodes[req.URI]
	if !ok {
		http.Error(w, "node not found in deleted set", http.StatusNotFound)
		return
	}
	a.nodes[req.URI] = n
	delete(a.deletedNodes, req.URI)
	writeJSON(w, http.StatusOK, n)
}

func (a *e2eContextAPI) handleSearch(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	project := strings.TrimSpace(r.URL.Query().Get("project"))
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	store := strings.TrimSpace(r.URL.Query().Get("type"))
	if store == "" {
		store = "all"
	}

	memories := []map[string]any{}
	if store == "all" || store == "memory" || store == "memories" {
		for _, m := range a.memories {
			if project != "" && m.Project != project {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(m.Content), query) {
				continue
			}
			memories = append(memories, map[string]any{
				"id":      m.ID,
				"content": m.Content,
			})
		}
	}

	skills := []map[string]any{}
	if store == "all" || store == "skill" || store == "skills" {
		for _, s := range a.skills {
			if project != "" && s.Project != project {
				continue
			}
			match := strings.Contains(strings.ToLower(s.Name), query) || strings.Contains(strings.ToLower(s.Description), query)
			if query != "" && !match {
				continue
			}
			skills = append(skills, map[string]any{
				"id":          s.ID,
				"name":        s.Name,
				"description": s.Description,
			})
		}
	}

	nodes := []map[string]any{}
	if store == "all" || store == "context" {
		for _, n := range a.nodes {
			if project != "" && n.Project != project {
				continue
			}
			match := strings.Contains(strings.ToLower(n.Name), query) ||
				strings.Contains(strings.ToLower(n.Abstract), query) ||
				strings.Contains(strings.ToLower(n.Content), query)
			if query != "" && !match {
				continue
			}
			nodes = append(nodes, map[string]any{
				"uri":      n.URI,
				"name":     n.Name,
				"abstract": n.Abstract,
				"content":  n.Content,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"memories":     memories,
		"skills":       skills,
		"contextNodes": nodes,
	})
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func sameProject(q url.Values, value string) bool {
	want := strings.TrimSpace(q.Get("project"))
	return want == "" || want == value
}

func execCommand(t *testing.T, cmdFactory func() interface {
	SetOut(io.Writer)
	SetErr(io.Writer)
	SetArgs([]string)
	Execute() error
}, args ...string) {
	t.Helper()
	cmd := cmdFactory()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed for args %v: %v", args, err)
	}
}

func Skip_TestContextCommandsEndToEnd(t *testing.T) {
	api := newE2EContextAPI()
	ctxSrv := httptest.NewServer(api)
	defer ctxSrv.Close()

	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Doc</title></head><body><h1>Auth Doc</h1><p>Token rotation details</p></body></html>`))
	}))
	defer pageSrv.Close()

	t.Setenv("MAIRU_CONTEXT_SERVER_URL", ctxSrv.URL)
	t.Setenv("MAIRU_CONTEXT_SERVER_TOKEN", "")
	project := "e2e-demo"

	// memory
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewMemoryCmd()
	}, "-P", project, "store", "remember auth token rotation", "-c", "decision", "--owner", "agent", "-i", "8")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewMemoryCmd()
	}, "-P", project, "add", "remember fallback credentials are disabled")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewMemoryCmd()
	}, "-P", project, "list", "--limit", "20")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewMemoryCmd()
	}, "-P", project, "search", "token", "-k", "5", "--highlight")

	// skill
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewSkillCmd()
	}, "-P", project, "add", "auth-rotation", "Rotate auth tokens every 30 days")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewSkillCmd()
	}, "-P", project, "list")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewSkillCmd()
	}, "-P", project, "search", "rotate")

	// node
	rootURI := "contextfs://e2e/root"
	childURI := "contextfs://e2e/root/auth"
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "store", rootURI, "root", "Root node")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "add", childURI, "auth", "Auth node", "--parent", rootURI, "--content", "Token rotation context")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "ls", rootURI)
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "list", "--limit", "20")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "read", childURI)
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "subtree", rootURI)
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "path", childURI)
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "update", childURI, "--content", "Updated token rotation content")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "search", "token")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "delete", childURI)
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "restore", childURI)

	// vibe and aliases
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewVibeCmd()
	}, "-P", project, "query", "summarize auth context", "-k", "3")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewVibeCmd()
	}, "-P", project, "mutation", "remember auth decision", "-k", "3")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewVibeQueryAliasCmd()
	}, "-P", project, "summarize auth context", "-k", "2")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewVibeMutationAliasCmd()
	}, "-P", project, "remember auth mutation", "-k", "2")

	// ingest and scrape
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewIngestCmd()
	}, "-P", project, "--text", "Auth flow and token rules", "--base-uri", "contextfs://e2e/ingest", "-y")
	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewScrapeWebCmd()
	}, "-P", project, pageSrv.URL+"/docs")

	api.mu.Lock()
	defer api.mu.Unlock()

	if len(api.memories) < 2 {
		t.Fatalf("expected memories to be stored by e2e flow, got %d", len(api.memories))
	}
	if len(api.skills) < 1 {
		t.Fatalf("expected skills to be stored by e2e flow, got %d", len(api.skills))
	}
	if len(api.nodes) < 3 {
		t.Fatalf("expected nodes to include root, restored child, and ingest/scrape nodes, got %d", len(api.nodes))
	}
}

// TestDaemonASTIngestionEndToEnd runs the real AST daemon against a temp TypeScript file,
// upserts via the same /api/context path as production, then queries /api/search (node search).
func TestDaemonASTIngestionEndToEnd(t *testing.T) {
	api := newE2EContextAPI()
	ctxSrv := httptest.NewServer(api)
	defer ctxSrv.Close()

	t.Setenv("MAIRU_CONTEXT_SERVER_URL", ctxSrv.URL)
	t.Setenv("MAIRU_CONTEXT_SERVER_TOKEN", "")
	project := "e2e-ast"

	dir := t.TempDir()
	const tsFile = "ingest_probe.ts"
	src := "export function e2eAstCanary() { return 1; }\n"
	if err := os.WriteFile(filepath.Join(dir, tsFile), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"daemon", dir, "-P", project})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("daemon scan: %v", err)
	}

	wantURI := "contextfs://" + project + "/" + tsFile
	api.mu.Lock()
	n, ok := api.nodes[wantURI]
	api.mu.Unlock()
	if !ok {
		t.Fatalf("expected node %q after daemon ingest, got %d nodes", wantURI, len(api.nodes))
	}
	combined := strings.ToLower(n.Abstract + n.Overview + n.Content)
	if !strings.Contains(combined, "e2eastcanary") {
		t.Fatalf("expected AST output to mention e2eAstCanary; abstract=%q overview=%.120q content=%.200q",
			n.Abstract, n.Overview, n.Content)
	}

	searchRaw, err := ContextGet("/api/search", map[string]string{
		"q":       "e2eastcanary",
		"type":    "context",
		"topK":    "10",
		"project": project,
	})
	if err != nil {
		t.Fatalf("context search: %v", err)
	}
	var searchOut struct {
		ContextNodes []map[string]any `json:"contextNodes"`
	}
	if err := json.Unmarshal(searchRaw, &searchOut); err != nil {
		t.Fatalf("search JSON: %v body=%s", err, string(searchRaw))
	}
	var saw bool
	for _, cn := range searchOut.ContextNodes {
		u, _ := cn["uri"].(string)
		if u == wantURI {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatalf("search results missing %q: %s", wantURI, string(searchRaw))
	}

	execCommand(t, func() interface {
		SetOut(io.Writer)
		SetErr(io.Writer)
		SetArgs([]string)
		Execute() error
	} {
		return NewNodeCmd()
	}, "-P", project, "search", "e2eastcanary", "-k", "5")
}
