package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"mairu/internal/contextsrv"
	"mairu/internal/crawler"
	"mairu/internal/llm"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newMemoryCmd())
	rootCmd.AddCommand(newSkillCmd())
	rootCmd.AddCommand(newNodeCmd())
	rootCmd.AddCommand(newCodeCmd())
	rootCmd.AddCommand(newVibeCmd())
	rootCmd.AddCommand(newVibeQueryAliasCmd())
	rootCmd.AddCommand(newVibeMutationAliasCmd())
	rootCmd.AddCommand(newIngestCmd())
	rootCmd.AddCommand(newScrapeCmd())
	rootCmd.AddCommand(newSmartScrapeCmd())
	rootCmd.AddCommand(newSearchScrapeCmd())
	rootCmd.AddCommand(newMultiScrapeCmd())
	rootCmd.AddCommand(newScriptScrapeCmd())
	rootCmd.AddCommand(newDepthScrapeCmd())
	rootCmd.AddCommand(newOmniScrapeCmd())
	rootCmd.AddCommand(newSummarizeCmd())
	rootCmd.AddCommand(newFlushCmd())
	rootCmd.AddCommand(newNudgeCmd())
	rootCmd.AddCommand(newImpactCmd())
	rootCmd.AddCommand(newAnalyzeGraphCmd())
	rootCmd.AddCommand(newAnalyzeDiffCmd())
	rootCmd.AddCommand(newGithubCmd())
	rootCmd.AddCommand(newLinearCmd())
}

func contextServerURL() string {
	base := strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_URL"))
	if base == "" {
		base = "http://localhost:8788"
	}
	return strings.TrimRight(base, "/")
}

func contextToken() string {
	return strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_TOKEN"))
}

func contextGet(path string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(contextServerURL() + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range params {
		if strings.TrimSpace(v) == "" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if tok := contextToken(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return doContextRequest(req)
}

func contextPost(path string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, contextServerURL()+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := contextToken(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return doContextRequest(req)
}

func contextPut(path string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, contextServerURL()+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := contextToken(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return doContextRequest(req)
}

func contextDelete(path string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(contextServerURL() + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range params {
		if strings.TrimSpace(v) == "" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if tok := contextToken(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return doContextRequest(req)
}

func doContextRequest(req *http.Request) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s failed (%d): %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}
	return body, nil
}

func printJSON(raw []byte) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Println(string(raw))
		return
	}
	formatted, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(string(raw))
		return
	}
	fmt.Println(string(formatted))
}

func runMemoryStore(project, content, category, owner string, importance int) error {
	out, err := contextPost("/api/memories", map[string]any{
		"project":    project,
		"content":    content,
		"category":   category,
		"owner":      owner,
		"importance": importance,
	})
	if err != nil {
		return err
	}
	printJSON(out)
	return nil
}

func runNodeStore(project, uri, name, abstract, parent, overview, content string) error {
	out, err := storeNodeRaw(project, uri, name, abstract, parent, overview, content)
	if err != nil {
		return err
	}
	printJSON(out)
	return nil
}

func storeNodeRaw(project, uri, name, abstract, parent, overview, content string) ([]byte, error) {
	payload := map[string]any{
		"uri":      uri,
		"project":  project,
		"name":     name,
		"abstract": abstract,
		"overview": overview,
		"content":  content,
	}
	if parent != "" {
		payload["parent_uri"] = parent
	}
	return contextPost("/api/context", payload)
}

func runVibeMutation(project, prompt string, k int) error {
	planOut, err := contextPost("/api/vibe/mutation/plan", map[string]any{
		"prompt":  prompt,
		"project": project,
		"topK":    k,
	})
	if err != nil {
		return err
	}
	var plan struct {
		Operations []map[string]any `json:"operations"`
	}
	if err := json.Unmarshal(planOut, &plan); err != nil {
		return err
	}
	execOut, err := contextPost("/api/vibe/mutation/execute", map[string]any{
		"project":    project,
		"operations": plan.Operations,
	})
	if err != nil {
		return err
	}
	printJSON(execOut)
	return nil
}

func addCommonSearchFlags(cmd *cobra.Command) {
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	cmd.Flags().Float64("minScore", 0, "Hard minimum score cutoff")
	cmd.Flags().Bool("highlight", false, "Include highlighted snippets")
}

func searchParamsFromFlags(cmd *cobra.Command, query, store, project string) map[string]string {
	k, _ := cmd.Flags().GetInt("k")
	minScore, _ := cmd.Flags().GetFloat64("minScore")
	highlight, _ := cmd.Flags().GetBool("highlight")
	params := map[string]string{
		"q":       query,
		"type":    store,
		"topK":    fmt.Sprintf("%d", k),
		"project": project,
	}
	if minScore > 0 {
		params["minScore"] = fmt.Sprintf("%g", minScore)
	}
	if highlight {
		params["highlight"] = "true"
	}
	return params
}

// newMemoryCmd creates the "memory" subcommand which allows users and agents
// to store, search, list, update, and delete atomic facts (memories).
func newMemoryCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "ContextFS memory operations",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search memories",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextGet("/api/search", searchParamsFromFlags(cmd, args[0], "memory", project))
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"score", "category", "content"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"score":    fmt.Sprintf("%.2f", item["_rankingScore"]),
							"category": fmt.Sprintf("%v", item["category"]),
							"content":  truncate(fmt.Sprintf("%v", item["content"]), 80),
						}
					},
				)
			}
			return nil
		},
	}
	addCommonSearchFlags(searchCmd)

	storeCmd := &cobra.Command{
		Use:   "store <content>",
		Short: "Store a memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			category, _ := cmd.Flags().GetString("category")
			owner, _ := cmd.Flags().GetString("owner")
			importance, _ := cmd.Flags().GetInt("importance")
			return runMemoryStore(project, args[0], category, owner, importance)
		},
	}
	storeCmd.Flags().StringP("category", "c", "observation", "Memory category")
	storeCmd.Flags().StringP("owner", "o", "agent", "Memory owner")
	storeCmd.Flags().IntP("importance", "i", 5, "Importance (1-10)")

	addCmd := &cobra.Command{
		Use:    "add <content>",
		Short:  "Alias for memory store",
		Args:   cobra.ExactArgs(1),
		Hidden: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			category, _ := cmd.Flags().GetString("category")
			owner, _ := cmd.Flags().GetString("owner")
			importance, _ := cmd.Flags().GetInt("importance")
			return runMemoryStore(project, args[0], category, owner, importance)
		},
	}
	addCmd.Flags().StringP("category", "c", "observation", "Memory category")
	addCmd.Flags().StringP("owner", "o", "agent", "Memory owner")
	addCmd.Flags().IntP("importance", "i", 5, "Importance (1-10)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List memories",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			out, err := contextGet("/api/memories", map[string]string{
				"project": project,
				"limit":   fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"id", "category", "content"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"id":       fmt.Sprintf("%v", item["id"]),
							"category": fmt.Sprintf("%v", item["category"]),
							"content":  truncate(fmt.Sprintf("%v", item["content"]), 80),
						}
					},
				)
			}
			return nil
		},
	}
	listCmd.Flags().Int("limit", 200, "Maximum items")

	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, _ := cmd.Flags().GetString("content")
			category, _ := cmd.Flags().GetString("category")
			owner, _ := cmd.Flags().GetString("owner")
			importance, _ := cmd.Flags().GetInt("importance")
			out, err := contextPut("/api/memories", map[string]any{
				"id":         args[0],
				"content":    content,
				"category":   category,
				"owner":      owner,
				"importance": importance,
			})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}
	updateCmd.Flags().String("content", "", "New content")
	updateCmd.Flags().String("category", "", "New category")
	updateCmd.Flags().String("owner", "", "New owner")
	updateCmd.Flags().Int("importance", 0, "New importance")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextDelete("/api/memories", map[string]string{"id": args[0]})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}

	feedbackCmd := &cobra.Command{
		Use:   "feedback <id> <reward>",
		Short: "Apply reinforcement learning feedback to a memory (reward 1-10)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reward, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("reward must be an integer between 1 and 10")
			}
			out, err := contextPost("/api/memories/feedback", map[string]any{
				"id":     args[0],
				"reward": reward,
			})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}

	cmd.AddCommand(searchCmd, storeCmd, addCmd, listCmd, updateCmd, deleteCmd, feedbackCmd)
	return cmd
}

// newSkillCmd creates the "skill" subcommand for managing and retrieving agent skills.
func newSkillCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "ContextFS skill operations",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	addCmd := &cobra.Command{
		Use:   "add <name> <description>",
		Short: "Store a skill",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextPost("/api/skills", map[string]any{
				"project":     project,
				"name":        args[0],
				"description": args[1],
			})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search skills",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextGet("/api/search", searchParamsFromFlags(cmd, args[0], "skill", project))
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"score", "name", "description"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"score":       fmt.Sprintf("%.2f", item["_rankingScore"]),
							"name":        fmt.Sprintf("%v", item["name"]),
							"description": truncate(fmt.Sprintf("%v", item["description"]), 80),
						}
					},
				)
			}
			return nil
		},
	}
	addCommonSearchFlags(searchCmd)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			out, err := contextGet("/api/skills", map[string]string{
				"project": project,
				"limit":   fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"id", "name", "description"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"id":          fmt.Sprintf("%v", item["id"]),
							"name":        fmt.Sprintf("%v", item["name"]),
							"description": truncate(fmt.Sprintf("%v", item["description"]), 80),
						}
					},
				)
			}
			return nil
		},
	}
	listCmd.Flags().Int("limit", 200, "Maximum items")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextDelete("/api/skills", map[string]string{"id": args[0]})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}

	cmd.AddCommand(addCmd, searchCmd, listCmd, deleteCmd)
	return cmd
}

// newCodeCmd creates the "code" subcommand, enabling semantic and exact search
// against ingested codebase context nodes.
func newCodeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "code",
		Short: "Semantic code search (bypasses LLM vibe query)",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search code files natively via AST daemon context nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			out, err := contextGet("/api/search", searchParamsFromFlags(cmd, args[0], "context", project))
			if err != nil {
				return err
			}

			if format == "json" {
				printJSON(out)
				return nil
			}

			// Parse response to extract paths
			var resp struct {
				ContextNodes []struct {
					URI      string  `json:"uri"`
					Name     string  `json:"name"`
					Abstract string  `json:"abstract"`
					Score    float64 `json:"_score"` // Meilisearch score if available
				} `json:"contextNodes"`
			}
			if err := json.Unmarshal(out, &resp); err != nil {
				// Fallback to raw if parsing fails
				printJSON(out)
				return nil
			}

			var validNodes []struct {
				URI      string  `json:"uri"`
				Name     string  `json:"name"`
				Abstract string  `json:"abstract"`
				Score    float64 `json:"_score"`
			}

			for _, node := range resp.ContextNodes {
				// strictly scope to daemon-generated files (or project files)
				// file URIs look like: contextfs://<project>/path/to/file.ext
				prefix := "contextfs://"
				if project != "" {
					prefix += project + "/"
				}
				if project != "" && !strings.HasPrefix(node.URI, prefix) {
					continue
				}
				validNodes = append(validNodes, node)
			}

			if len(validNodes) == 0 {
				fmt.Println("No matching code files found.")
				return nil
			}

			for _, node := range validNodes {
				// Convert contextfs://my-project/src/file.ts -> src/file.ts
				path := strings.TrimPrefix(node.URI, "contextfs://")
				if project != "" {
					path = strings.TrimPrefix(path, project+"/")
				} else {
					// If no project specified, extract it from URI
					parts := strings.SplitN(path, "/", 2)
					if len(parts) == 2 {
						path = parts[1]
					}
				}

				if format == "paths" {
					fmt.Println(path)
				} else {
					// Default format: path + abstract
					fmt.Printf("%s\n", path)
					if node.Abstract != "" {
						fmt.Printf("  %s\n", strings.TrimSpace(node.Abstract))
					}
					fmt.Println()
				}
			}

			return nil
		},
	}
	addCommonSearchFlags(searchCmd)
	searchCmd.Flags().String("format", "default", "Output format: default, paths, or json")

	cmd.AddCommand(searchCmd)
	return cmd
}

// newNodeCmd creates the "node" subcommand which provides operations for
// hierarchical context nodes (e.g. documentation, structured knowledge).
func newNodeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "node",
		Short: "ContextFS node operations",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search context nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextGet("/api/search", searchParamsFromFlags(cmd, args[0], "context", project))
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"score", "uri", "name", "abstract"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"score":    fmt.Sprintf("%.2f", item["_rankingScore"]),
							"uri":      fmt.Sprintf("%v", item["uri"]),
							"name":     truncate(fmt.Sprintf("%v", item["name"]), 30),
							"abstract": truncate(fmt.Sprintf("%v", item["abstract"]), 60),
						}
					},
				)
			}
			return nil
		},
	}
	addCommonSearchFlags(searchCmd)

	storeCmd := &cobra.Command{
		Use:   "store <uri> <name> <abstract>",
		Short: "Store a context node",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			parent, _ := cmd.Flags().GetString("parent")
			overview, _ := cmd.Flags().GetString("overview")
			content, _ := cmd.Flags().GetString("content")
			return runNodeStore(project, args[0], args[1], args[2], parent, overview, content)
		},
	}
	storeCmd.Flags().String("parent", "", "Parent URI")
	storeCmd.Flags().String("overview", "", "Node overview")
	storeCmd.Flags().String("content", "", "Node content")

	addCmd := &cobra.Command{
		Use:   "add <uri> <name> <abstract>",
		Short: "Alias for node store",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			parent, _ := cmd.Flags().GetString("parent")
			overview, _ := cmd.Flags().GetString("overview")
			content, _ := cmd.Flags().GetString("content")
			return runNodeStore(project, args[0], args[1], args[2], parent, overview, content)
		},
	}
	addCmd.Flags().String("parent", "", "Parent URI")
	addCmd.Flags().String("overview", "", "Node overview")
	addCmd.Flags().String("content", "", "Node content")

	lsCmd := &cobra.Command{
		Use:   "ls <uri>",
		Short: "List nodes under a parent URI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextGet("/api/context", map[string]string{
				"project":   project,
				"parentUri": args[0],
			})
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"uri", "name", "abstract"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"uri":      fmt.Sprintf("%v", item["uri"]),
							"name":     truncate(fmt.Sprintf("%v", item["name"]), 30),
							"abstract": truncate(fmt.Sprintf("%v", item["abstract"]), 60),
						}
					},
				)
			}
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List context nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			parent, _ := cmd.Flags().GetString("parent")
			limit, _ := cmd.Flags().GetInt("limit")
			params := map[string]string{
				"project": project,
				"limit":   fmt.Sprintf("%d", limit),
			}
			if parent != "" {
				params["parentUri"] = parent
			}
			out, err := contextGet("/api/context", params)
			if err != nil {
				return err
			}

			if outputFormat == "json" || outputFormat == "" {
				printJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					printJSON(out) // fallback
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"uri", "name", "abstract"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"uri":      fmt.Sprintf("%v", item["uri"]),
							"name":     truncate(fmt.Sprintf("%v", item["name"]), 30),
							"abstract": truncate(fmt.Sprintf("%v", item["abstract"]), 60),
						}
					},
				)
			}
			return nil
		},
	}
	listCmd.Flags().String("parent", "", "Parent URI")
	listCmd.Flags().Int("limit", 200, "Maximum items")

	readCmd := &cobra.Command{
		Use:   "read <uri>",
		Short: "Read node by URI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodes, err := fetchAllNodes(project)
			if err != nil {
				return err
			}
			for _, n := range nodes {
				if n["uri"] == args[0] {
					raw, _ := json.Marshal(n)
					printJSON(raw)
					return nil
				}
			}
			return fmt.Errorf("node not found: %s", args[0])
		},
	}

	subtreeCmd := &cobra.Command{
		Use:   "subtree <uri>",
		Short: "List subtree nodes by URI prefix",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodes, err := fetchAllNodes(project)
			if err != nil {
				return err
			}
			root := args[0]
			var filtered []map[string]any
			for _, n := range nodes {
				uri, _ := n["uri"].(string)
				if strings.HasPrefix(uri, root) {
					filtered = append(filtered, n)
				}
			}
			raw, _ := json.Marshal(filtered)
			printJSON(raw)
			return nil
		},
	}

	pathCmd := &cobra.Command{
		Use:   "path <uri>",
		Short: "Build parent path to node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodes, err := fetchAllNodes(project)
			if err != nil {
				return err
			}
			byURI := map[string]map[string]any{}
			for _, n := range nodes {
				if uri, ok := n["uri"].(string); ok {
					byURI[uri] = n
				}
			}
			current := args[0]
			var chain []map[string]any
			for current != "" {
				node, ok := byURI[current]
				if !ok {
					break
				}
				chain = append(chain, node)
				parent, _ := node["parent_uri"].(string)
				current = parent
			}
			for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
				chain[i], chain[j] = chain[j], chain[i]
			}
			raw, _ := json.Marshal(chain)
			printJSON(raw)
			return nil
		},
	}

	updateCmd := &cobra.Command{
		Use:   "update <uri>",
		Short: "Update node fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			abstract, _ := cmd.Flags().GetString("abstract")
			overview, _ := cmd.Flags().GetString("overview")
			content, _ := cmd.Flags().GetString("content")
			out, err := contextPut("/api/context", map[string]any{
				"uri":      args[0],
				"name":     name,
				"abstract": abstract,
				"overview": overview,
				"content":  content,
			})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}
	updateCmd.Flags().String("name", "", "New name")
	updateCmd.Flags().String("abstract", "", "New abstract")
	updateCmd.Flags().String("overview", "", "New overview")
	updateCmd.Flags().String("content", "", "New content")

	deleteCmd := &cobra.Command{
		Use:   "delete <uri>",
		Short: "Delete node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextDelete("/api/context", map[string]string{"uri": args[0]})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}

	restoreCmd := &cobra.Command{
		Use:   "restore <uri>",
		Short: "Restore node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := contextPost("/api/context/restore", map[string]string{"uri": args[0]})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}

	cmd.AddCommand(searchCmd, storeCmd, addCmd, lsCmd, listCmd, readCmd, subtreeCmd, pathCmd, updateCmd, deleteCmd, restoreCmd)
	return cmd
}

func newVibeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "vibe",
		Short: "ContextFS vibe operations",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	queryCmd := &cobra.Command{
		Use:   "query <prompt>",
		Short: "Run vibe query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, _ := cmd.Flags().GetInt("k")
			out, err := contextPost("/api/vibe/query", map[string]any{
				"prompt":  args[0],
				"project": project,
				"topK":    k,
			})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}
	queryCmd.Flags().IntP("k", "k", 5, "Top K results")
	queryCmd.Aliases = []string{"summarize"}

	mutationCmd := &cobra.Command{
		Use:   "mutation [prompt]",
		Short: "Plan and execute vibe mutation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("prompt is required")
			}
			k, _ := cmd.Flags().GetInt("k")
			return runVibeMutation(project, args[0], k)
		},
	}
	mutationCmd.Flags().IntP("k", "k", 5, "Top K results")
	mutationCmd.Aliases = []string{"flush", "nudge"}

	cmd.AddCommand(queryCmd, mutationCmd)
	return cmd
}

func newVibeQueryAliasCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "vibe-query <prompt>",
		Short: "Alias for vibe query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, _ := cmd.Flags().GetInt("k")
			out, err := contextPost("/api/vibe/query", map[string]any{
				"prompt":  args[0],
				"project": project,
				"topK":    k,
			})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	return cmd
}

func newVibeMutationAliasCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "vibe-mutation [prompt]",
		Short: "Alias for vibe mutation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("prompt is required")
			}
			k, _ := cmd.Flags().GetInt("k")
			return runVibeMutation(project, args[0], k)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	return cmd
}

func newSummarizeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "summarize <query>",
		Short: "Summarize using vibe query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, _ := cmd.Flags().GetInt("k")
			out, err := contextPost("/api/vibe/query", map[string]any{
				"prompt":  args[0],
				"project": project,
				"topK":    k,
			})
			if err != nil {
				return err
			}
			printJSON(out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	return cmd
}

func newFlushCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "flush [prompt]",
		Short: "Flush transcript into durable facts",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("prompt is required")
			}
			k, _ := cmd.Flags().GetInt("k")
			return runVibeMutation(project, args[0], k)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	return cmd
}

func newNudgeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "nudge [prompt]",
		Short: "Suggest mutations from transcript",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("prompt is required")
			}
			k, _ := cmd.Flags().GetInt("k")
			return runVibeMutation(project, args[0], k)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "", "Project name")
	cmd.Flags().IntP("k", "k", 5, "Top K results")
	return cmd
}

// newIngestCmd creates the "ingest" subcommand, taking raw text inputs and
// using the LLM to structure them into hierarchical context nodes.
func newIngestCmd() *cobra.Command {
	var project, baseURI, textStr string
	var yes, noRouter bool

	cmd := &cobra.Command{
		Use:   "ingest [file]",
		Short: "Parse an MD file or free text via LLM into context nodes, review, then persist",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && textStr == "" {
				return fmt.Errorf("provide a file path or --text <text>")
			}
			content := textStr
			if len(args) > 0 {
				b, err := os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				content = string(b)
				fmt.Printf("\nRead %d characters from %s\n", len(content), args[0])
			}

			fmt.Println("\nParsing into context nodes via LLM...")
			out, err := contextPost("/api/vibe/ingest", map[string]any{
				"text":     content,
				"base_uri": baseURI,
			})
			if err != nil {
				return err
			}

			var res struct {
				Nodes []map[string]any `json:"nodes"`
			}
			if err := json.Unmarshal(out, &res); err != nil {
				return err
			}

			fmt.Printf("\nProposed %d context node(s):\n\n", len(res.Nodes))
			for _, n := range res.Nodes {
				fmt.Printf("URI: %v\nName: %v\nAbstract: %v\n---\n", n["URI"], n["Name"], n["Abstract"])
			}

			if !yes {
				fmt.Print("Persist these nodes? [y/N]: ")
				var answer string
				fmt.Scanln(&answer)
				if answer != "y" && answer != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			fmt.Printf("\nPersisting %d node(s)...\n", len(res.Nodes))
			for _, n := range res.Nodes {
				uri, _ := n["URI"].(string)
				name, _ := n["Name"].(string)
				abstract, _ := n["Abstract"].(string)
				contentStr, _ := n["Content"].(string)
				overview, _ := n["Overview"].(string)
				parent, _ := n["ParentURI"].(string)

				if err := runNodeStore(project, uri, name, abstract, parent, overview, contentStr); err != nil {
					fmt.Printf("Failed to store %s: %v\n", uri, err)
				} else {
					fmt.Printf("Stored %s\n", uri)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&textStr, "text", "", "Free text to ingest")
	cmd.Flags().StringVar(&baseURI, "base-uri", "contextfs://ingested", "Base URI namespace for generated nodes")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip interactive review and persist all proposed nodes")
	cmd.Flags().BoolVar(&noRouter, "no-router", false, "Skip LLM dedup router when persisting nodes")
	return cmd
}

func newScrapeCmd() *cobra.Command {
	var project string
	var maxPages int
	var maxDepth int
	var concurrency int

	cmd := &cobra.Command{
		Use:   "scrape <url>",
		Short: "Fetch a URL, extract content, summarize, and store as a context node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			urlStr := args[0]
			fmt.Printf("Crawling %s...\n", urlStr)

			opts := crawler.ScrapeOptions{
				Project: project,
				CrawlOptions: crawler.CrawlOptions{
					SeedURL:     urlStr,
					MaxPages:    maxPages,
					MaxDepth:    maxDepth,
					Concurrency: concurrency,
				},
			}

			storeFn := func(ctx context.Context, input contextsrv.ContextCreateInput) error {
				fmt.Printf("Storing node %s...\n", input.URI)
				var parent string
				if input.ParentURI != nil {
					parent = *input.ParentURI
				}
				return runNodeStore(input.Project, input.URI, input.Name, input.Abstract, parent, input.Overview, input.Content)
			}

			apiKey := GetAPIKey()
			res, err := crawler.ScrapeAndIngest(cmd.Context(), opts, storeFn, apiKey)
			if err != nil {
				return err
			}

			fmt.Printf("Scraping complete. Total pages: %d, Stored: %d, Skipped: %d\n", res.PagesTotal, res.PagesStored, res.PagesSkipped)
			if len(res.Errors) > 0 {
				fmt.Printf("Errors encountered: %d\n", len(res.Errors))
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Max pages to crawl")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 3, "Max depth to crawl")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Concurrent requests")
	return cmd
}

func fetchAllNodes(project string) ([]map[string]any, error) {
	out, err := contextGet("/api/context", map[string]string{
		"project": project,
		"limit":   "5000",
	})
	if err != nil {
		return nil, err
	}
	var nodes []map[string]any
	if err := json.Unmarshal(out, &nodes); err != nil {
		return nil, err
	}
	sort.Slice(nodes, func(i, j int) bool {
		left, _ := nodes[i]["uri"].(string)
		right, _ := nodes[j]["uri"].(string)
		return left < right
	})
	return nodes, nil
}

func newSmartScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var useRAG bool
	var refinePrompt bool

	cmd := &cobra.Command{
		Use:   "smart-scrape <url>",
		Short: "Fetch a URL and extract structured data using LLM based on prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for smart-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running smart scrape on %s...\n", targetURL)

			graph := crawler.NewSmartScraperGraph(provider)
			data, err := graph.Run(cmd.Context(), targetURL, prompt)
			if err != nil {
				return fmt.Errorf("scrape failed: %w", err)
			}

			if data == nil {
				fmt.Println("No data extracted.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			fmt.Printf("\nExtracted Data:\n%s\n\n", string(jsonBytes))

			// Store as context node
			uri := fmt.Sprintf("contextfs://scrape/%s", targetURL)
			// basic clean up of URL for URI
			uri = strings.ReplaceAll(uri, "https://", "")
			uri = strings.ReplaceAll(uri, "http://", "")

			content := string(jsonBytes)
			fmt.Printf("Storing extracted data at %s in project '%s'...\n", uri, project)

			return runNodeStore(project, uri, "Extracted Data", "Data extracted via smart-scrape: "+prompt, "", "", content)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract")
	cmd.Flags().BoolVar(&useRAG, "rag", false, "Use vector embeddings to parse massive documents without hitting token limits")
	cmd.Flags().BoolVar(&refinePrompt, "refine", false, "Use LLM to refine the prompt before scraping")
	return cmd
}

func newSearchScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var maxResults int

	cmd := &cobra.Command{
		Use:   "search-scrape <query>",
		Short: "Search web for query and extract structured data from top results using LLM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for search-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running search scrape for query '%s'...\n", query)

			graph := crawler.NewSearchScraperGraph(provider)
			results, err := graph.Run(cmd.Context(), query, prompt, maxResults)
			if err != nil {
				return fmt.Errorf("search scrape failed: %w", err)
			}

			if len(results) == 0 {
				fmt.Println("No data extracted from any search result.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(results, "", "  ")
			fmt.Printf("\nExtracted Data:\n%s\n\n", string(jsonBytes))

			// Store as context node
			uri := fmt.Sprintf("contextfs://search/%s", url.QueryEscape(query))

			content := string(jsonBytes)
			fmt.Printf("Storing extracted data at %s in project '%s'...\n", uri, project)

			return runNodeStore(project, uri, "Search Data: "+query, "Data extracted via search-scrape for query: "+query, "", "", content)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract")
	cmd.Flags().IntVar(&maxResults, "max-results", 3, "Maximum number of search results to process")
	return cmd
}

func newMultiScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var concurrency int

	cmd := &cobra.Command{
		Use:   "multi-scrape <url1> [url2...]",
		Short: "Fetch multiple URLs concurrently and extract structured data using LLM",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for multi-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running multi-scrape on %d URLs...\n", len(args))

			graph := crawler.NewSmartScraperMultiGraph(provider, concurrency)
			data, err := graph.Run(cmd.Context(), args, prompt)
			if err != nil {
				return fmt.Errorf("multi-scrape failed: %w", err)
			}

			if len(data) == 0 {
				fmt.Println("No data extracted.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			fmt.Printf("\nExtracted Data:\n%s\n\n", string(jsonBytes))

			// Optional: iterate and save each as context node
			for urlStr, result := range data {
				uri := fmt.Sprintf("contextfs://scrape/%s", strings.ReplaceAll(strings.ReplaceAll(urlStr, "https://", ""), "http://", ""))
				resBytes, _ := json.Marshal(result)
				runNodeStore(project, uri, "Extracted Data", "Data extracted via multi-scrape: "+prompt, "", "", string(resBytes))
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Concurrent scraping requests")
	return cmd
}

func newScriptScrapeCmd() *cobra.Command {
	var prompt string
	var output string

	cmd := &cobra.Command{
		Use:   "script-scrape <url>",
		Short: "Generate a Go scraping script using goquery tailored for a specific URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for script-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Generating scraper script for %s...\n", targetURL)

			graph := crawler.NewScriptCreatorGraph(provider)
			scriptContent, err := graph.Run(cmd.Context(), targetURL, prompt)
			if err != nil {
				return fmt.Errorf("script generation failed: %w", err)
			}

			if scriptContent == "" {
				fmt.Println("No script generated.")
				return nil
			}

			if output != "" {
				err := os.WriteFile(output, []byte(scriptContent), 0644)
				if err != nil {
					return fmt.Errorf("failed to write script: %w", err)
				}
				fmt.Printf("Script saved to %s\n", output)
			} else {
				fmt.Printf("\n--- Generated Go Script ---\n\n%s\n\n", scriptContent)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct what the script should scrape")
	cmd.Flags().StringVarP(&output, "output", "o", "scraper.go", "Output file for the script (default: scraper.go)")
	return cmd
}

func newDepthScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var maxDepth int
	var concurrency int

	cmd := &cobra.Command{
		Use:   "depth-scrape <url>",
		Short: "Fetch a URL, discover relevant links up to depth K, and extract data concurrently",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			seedURL := args[0]
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for depth-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running depth-scrape (depth: %d) on %s...\n", maxDepth, seedURL)

			graph := crawler.NewDepthSearchScraperGraph(provider, maxDepth, concurrency)
			data, err := graph.Run(cmd.Context(), seedURL, prompt)
			if err != nil {
				return fmt.Errorf("depth-scrape failed: %w", err)
			}

			if len(data) == 0 {
				fmt.Println("No data extracted.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			fmt.Printf("\nExtracted Data:\n%s\n\n", string(jsonBytes))

			// Store each as context node
			for urlStr, result := range data {
				uri := fmt.Sprintf("contextfs://scrape/%s", strings.ReplaceAll(strings.ReplaceAll(urlStr, "https://", ""), "http://", ""))
				resBytes, _ := json.Marshal(result)
				runNodeStore(project, uri, "Extracted Data", "Data extracted via depth-scrape: "+prompt, "", "", string(resBytes))
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract and which links to follow")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 1, "Maximum link depth to traverse (0 = only seed URL)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Concurrent scraping requests")
	return cmd
}

func newOmniScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var concurrency int

	cmd := &cobra.Command{
		Use:   "omni-scrape <url1> [url2...]",
		Short: "Fetch multiple URLs concurrently and merge extracted data into a single summary",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for omni-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running omni-scrape on %d URLs...\n", len(args))

			graph := crawler.NewOmniScraperGraph(provider, concurrency)
			data, err := graph.Run(cmd.Context(), args, prompt)
			if err != nil {
				return fmt.Errorf("omni-scrape failed: %w", err)
			}

			if len(data) == 0 {
				fmt.Println("No data extracted.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			fmt.Printf("\nMerged Extracted Data:\n%s\n\n", string(jsonBytes))

			// Store as context node
			uri := fmt.Sprintf("contextfs://omni-scrape/%s", strings.ReplaceAll(strings.ReplaceAll(args[0], "https://", ""), "http://", ""))
			if len(args) > 1 {
				uri += "-and-others"
			}

			content := string(jsonBytes)
			fmt.Printf("Storing merged data at %s in project '%s'...\n", uri, project)

			return runNodeStore(project, uri, "Merged Omni Data", "Data merged via omni-scrape: "+prompt, "", "", content)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract and merge")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Concurrent scraping requests")
	return cmd
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
