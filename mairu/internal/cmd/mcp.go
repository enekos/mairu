package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"log"
)

func init() {
}

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server on stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpServer := server.NewMCPServer("mairu", "0.1.0")

			// Tool: search_memories
			searchMemoriesTool := mcp.NewTool("search_memories",
				mcp.WithDescription("Search agent memories and facts using vector + full-text search."),
				mcp.WithString("query", mcp.Required(), mcp.Description("The search query")),
				mcp.WithString("project", mcp.Description("Project name to scope the search")),
				mcp.WithNumber("k", mcp.Description("Top K results to return (default 5)")),
			)
			mcpServer.AddTool(searchMemoriesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				query, err := request.RequireString("query")
				if err != nil {
					return mcp.NewToolResultError("query is required"), nil
				}
				project := request.GetString("project", "")
				k := request.GetInt("k", 5)

				params := map[string]string{
					"q":       query,
					"type":    "memory",
					"project": project,
					"topK":    fmt.Sprintf("%d", k),
				}

				out, err := ContextGet("/api/search", params)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				// format JSON nicely
				var v any
				if err := json.Unmarshal(out, &v); err == nil {
					if formatted, err := json.MarshalIndent(v, "", "  "); err == nil {
						return mcp.NewToolResultText(string(formatted)), nil
					}
				}
				return mcp.NewToolResultText(string(out)), nil
			})

			// Tool: store_memory
			storeMemoryTool := mcp.NewTool("store_memory",
				mcp.WithDescription("Store a new fact or observation in the agent's memory."),
				mcp.WithString("content", mcp.Required(), mcp.Description("The fact to remember")),
				mcp.WithString("project", mcp.Description("Project namespace")),
				mcp.WithString("category", mcp.Description("Category (default 'observation')")),
				mcp.WithString("owner", mcp.Description("Owner (default 'agent')")),
				mcp.WithNumber("importance", mcp.Description("Importance 1-10 (default 5)")),
			)
			mcpServer.AddTool(storeMemoryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				content, err := request.RequireString("content")
				if err != nil {
					return mcp.NewToolResultError("content is required"), nil
				}
				project := request.GetString("project", "")
				category := request.GetString("category", "observation")
				if category == "" {
					category = "observation"
				}
				owner := request.GetString("owner", "agent")
				if owner == "" {
					owner = "agent"
				}
				importance := request.GetInt("importance", 5)

				out, err := ContextPost("/api/memories", map[string]any{
					"project":    project,
					"content":    content,
					"category":   category,
					"owner":      owner,
					"importance": importance,
				})
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				var v any
				if err := json.Unmarshal(out, &v); err == nil {
					if formatted, err := json.MarshalIndent(v, "", "  "); err == nil {
						return mcp.NewToolResultText(string(formatted)), nil
					}
				}
				return mcp.NewToolResultText(string(out)), nil
			})

			// Tool: search_nodes
			searchNodesTool := mcp.NewTool("search_nodes",
				mcp.WithDescription("Search structured hierarchical context nodes (code files, docs, logic)."),
				mcp.WithString("query", mcp.Required(), mcp.Description("The search query")),
				mcp.WithString("project", mcp.Description("Project name to scope the search")),
				mcp.WithNumber("k", mcp.Description("Top K results to return (default 5)")),
			)
			mcpServer.AddTool(searchNodesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				query, err := request.RequireString("query")
				if err != nil {
					return mcp.NewToolResultError("query is required"), nil
				}
				project := request.GetString("project", "")
				k := request.GetInt("k", 5)

				params := map[string]string{
					"q":       query,
					"type":    "context",
					"project": project,
					"topK":    fmt.Sprintf("%d", k),
				}

				out, err := ContextGet("/api/search", params)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				var v any
				if err := json.Unmarshal(out, &v); err == nil {
					if formatted, err := json.MarshalIndent(v, "", "  "); err == nil {
						return mcp.NewToolResultText(string(formatted)), nil
					}
				}
				return mcp.NewToolResultText(string(out)), nil
			})

			// Tool: vibe_query
			vibeQueryTool := mcp.NewTool("vibe_query",
				mcp.WithDescription("Run an LLM-powered free-form query against the codebase using vibe query."),
				mcp.WithString("prompt", mcp.Required(), mcp.Description("The natural language question about the codebase")),
				mcp.WithString("project", mcp.Description("Project namespace")),
				mcp.WithNumber("k", mcp.Description("Top K results for the underlying context search (default 5)")),
			)
			mcpServer.AddTool(vibeQueryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				prompt, err := request.RequireString("prompt")
				if err != nil {
					return mcp.NewToolResultError("prompt is required"), nil
				}
				project := request.GetString("project", "")
				k := request.GetInt("k", 5)

				out, err := ContextPost("/api/vibe/query", map[string]any{
					"prompt":  prompt,
					"project": project,
					"topK":    k,
				})
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				var v any
				if err := json.Unmarshal(out, &v); err == nil {
					if formatted, err := json.MarshalIndent(v, "", "  "); err == nil {
						return mcp.NewToolResultText(string(formatted)), nil
					}
				}
				return mcp.NewToolResultText(string(out)), nil
			})

			// Tool: vibe_mutation
			vibeMutationTool := mcp.NewTool("vibe_mutation",
				mcp.WithDescription("Suggest and automatically perform database updates (memories, nodes) based on recent facts/instructions."),
				mcp.WithString("prompt", mcp.Required(), mcp.Description("The new facts to commit to memory")),
				mcp.WithString("project", mcp.Description("Project namespace")),
				mcp.WithNumber("k", mcp.Description("Top K context nodes for resolving existing context (default 5)")),
			)
			mcpServer.AddTool(vibeMutationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				prompt, err := request.RequireString("prompt")
				if err != nil {
					return mcp.NewToolResultError("prompt is required"), nil
				}
				project := request.GetString("project", "")
				k := request.GetInt("k", 5)

				planOut, err := ContextPost("/api/vibe/mutation/plan", map[string]any{
					"prompt":  prompt,
					"project": project,
					"topK":    k,
				})
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to generate plan: %v", err)), nil
				}
				var plan struct {
					Operations []map[string]any `json:"operations"`
				}
				if err := json.Unmarshal(planOut, &plan); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to parse plan: %v", err)), nil
				}
				execOut, err := ContextPost("/api/vibe/mutation/execute", map[string]any{
					"project":    project,
					"operations": plan.Operations,
				})
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to execute plan: %v", err)), nil
				}
				var v any
				if err := json.Unmarshal(execOut, &v); err == nil {
					if formatted, err := json.MarshalIndent(v, "", "  "); err == nil {
						return mcp.NewToolResultText(string(formatted)), nil
					}
				}
				return mcp.NewToolResultText(string(execOut)), nil
			})

			if err := server.ServeStdio(mcpServer); err != nil {
				log.Fatalf("Server error: %v", err)
			}
			return nil
		},
	}
	return cmd
}
