package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

var utcpTools = []Tool{
	{
		Name:        "search_memories",
		Description: "Search agent memories and facts using vector + full-text search.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":   map[string]interface{}{"type": "string", "description": "The search query"},
				"project": map[string]interface{}{"type": "string", "description": "Project name to scope the search"},
				"k":       map[string]interface{}{"type": "number", "description": "Top K results to return (default 5)"},
			},
			"required": []string{"query"},
		},
	},
	{
		Name:        "store_memory",
		Description: "Store a new fact or observation in the agent's memory.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content":    map[string]interface{}{"type": "string", "description": "The fact to remember"},
				"project":    map[string]interface{}{"type": "string", "description": "Project namespace"},
				"category":   map[string]interface{}{"type": "string", "description": "Category (default 'observation')"},
				"owner":      map[string]interface{}{"type": "string", "description": "Owner (default 'agent')"},
				"importance": map[string]interface{}{"type": "number", "description": "Importance 1-10 (default 5)"},
			},
			"required": []string{"content"},
		},
	},
	{
		Name:        "search_nodes",
		Description: "Search structured hierarchical context nodes (code files, docs, logic).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":   map[string]interface{}{"type": "string", "description": "The search query"},
				"project": map[string]interface{}{"type": "string", "description": "Project name to scope the search"},
				"k":       map[string]interface{}{"type": "number", "description": "Top K results to return (default 5)"},
			},
			"required": []string{"query"},
		},
	},
	{
		Name:        "vibe_query",
		Description: "Run an LLM-powered free-form query against the codebase using vibe query.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prompt":  map[string]interface{}{"type": "string", "description": "The natural language question about the codebase"},
				"project": map[string]interface{}{"type": "string", "description": "Project namespace"},
				"k":       map[string]interface{}{"type": "number", "description": "Top K results for the underlying context search (default 5)"},
			},
			"required": []string{"prompt"},
		},
	},
	{
		Name:        "vibe_mutation",
		Description: "Suggest and automatically perform database updates (memories, nodes) based on recent facts/instructions.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prompt":  map[string]interface{}{"type": "string", "description": "The new facts to commit to memory"},
				"project": map[string]interface{}{"type": "string", "description": "Project namespace"},
				"k":       map[string]interface{}{"type": "number", "description": "Top K context nodes for resolving existing context (default 5)"},
			},
			"required": []string{"prompt"},
		},
	},
}

func NewUTCPCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "utcp",
		Short: "Start UTCP server for Mairu over HTTP",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := fmt.Sprintf(":%d", port)
			r := http.NewServeMux()
			r.HandleFunc("GET /tools", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := map[string]interface{}{
					"version": "1.0",
					"tools":   utcpTools,
				}
				buf := bufferPool.Get().(*bytes.Buffer)
				defer bufferPool.Put(buf)
				buf.Reset()
				json.NewEncoder(buf).Encode(response)
				w.Write(buf.Bytes())
			})

			r.HandleFunc("POST /tools/{name}/call", func(w http.ResponseWriter, r *http.Request) {
				name := r.PathValue("name")
				var args map[string]interface{}
				if r.ContentLength > 0 {
					if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
						http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
						return
					}
				}

				var result interface{}
				var err error

				switch name {
				case "search_memories":
					query, _ := args["query"].(string)
					project, _ := args["project"].(string)
					k := 5
					if kv, ok := args["k"].(float64); ok {
						k = int(kv)
					}
					params := map[string]string{
						"q":       query,
						"type":    "memory",
						"project": project,
						"topK":    strconv.Itoa(k),
					}
					var out []byte
					out, err = ContextGet("/api/search", params)
					if err == nil {
						var v any
						json.Unmarshal(out, &v)
						result = v
					}
				case "store_memory":
					content, _ := args["content"].(string)
					project, _ := args["project"].(string)
					category, _ := args["category"].(string)
					if category == "" {
						category = "observation"
					}
					owner, _ := args["owner"].(string)
					if owner == "" {
						owner = "agent"
					}
					importance := 5
					if iv, ok := args["importance"].(float64); ok {
						importance = int(iv)
					}
					var out []byte
					out, err = ContextPost("/api/memories", map[string]any{
						"project":    project,
						"content":    content,
						"category":   category,
						"owner":      owner,
						"importance": importance,
					})
					if err == nil {
						var v any
						json.Unmarshal(out, &v)
						result = v
					}
				case "search_nodes":
					query, _ := args["query"].(string)
					project, _ := args["project"].(string)
					k := 5
					if kv, ok := args["k"].(float64); ok {
						k = int(kv)
					}
					params := map[string]string{
						"q":       query,
						"type":    "context",
						"project": project,
						"topK":    strconv.Itoa(k),
					}
					var out []byte
					out, err = ContextGet("/api/search", params)
					if err == nil {
						var v any
						json.Unmarshal(out, &v)
						result = v
					}
				case "vibe_query":
					prompt, _ := args["prompt"].(string)
					project, _ := args["project"].(string)
					k := 5
					if kv, ok := args["k"].(float64); ok {
						k = int(kv)
					}
					var out []byte
					out, err = ContextPost("/api/vibe/query", map[string]any{
						"prompt":  prompt,
						"project": project,
						"topK":    k,
					})
					if err == nil {
						var v any
						json.Unmarshal(out, &v)
						result = v
					}
				case "vibe_mutation":
					prompt, _ := args["prompt"].(string)
					project, _ := args["project"].(string)
					k := 5
					if kv, ok := args["k"].(float64); ok {
						k = int(kv)
					}
					var planOut []byte
					planOut, err = ContextPost("/api/vibe/mutation/plan", map[string]any{
						"prompt":  prompt,
						"project": project,
						"topK":    k,
					})
					if err == nil {
						var plan struct {
							Operations []map[string]any `json:"operations"`
						}
						if pErr := json.Unmarshal(planOut, &plan); pErr == nil {
							var execOut []byte
							execOut, err = ContextPost("/api/vibe/mutation/execute", map[string]any{
								"project":    project,
								"operations": plan.Operations,
							})
							if err == nil {
								var v any
								json.Unmarshal(execOut, &v)
								result = v
							}
						} else {
							err = fmt.Errorf("failed to parse plan: %v", pErr)
						}
					}
				default:
					http.Error(w, "unknown tool: "+name, http.StatusNotFound)
					return
				}

				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				buf := bufferPool.Get().(*bytes.Buffer)
				defer bufferPool.Put(buf)
				buf.Reset()
				json.NewEncoder(buf).Encode(map[string]interface{}{"result": result})
				w.Write(buf.Bytes())
			})

			srv := &http.Server{
				Handler:      r,
				Addr:         addr,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 15 * time.Second,
			}

			log.Printf("Starting UTCP server on %s", addr)
			return srv.ListenAndServe()
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 8081, "Port to listen on")
	return cmd
}
