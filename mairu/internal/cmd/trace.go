package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/spf13/cobra"

	"mairu/internal/trace"
)

// NewTraceCmd builds the `mairu trace` command group: list, show, search.
// All subcommands talk directly to the contextfs_llm_traces Meilisearch
// index — they don't need an embedder or the wider App stack.
func NewTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Inspect recorded LLM traces (observability)",
	}
	cmd.AddCommand(
		newTraceListCmd(),
		newTraceSearchCmd(),
		newTraceShowCmd(),
		newTracePromoteCmd(),
	)
	return cmd
}

func meiliClient() (meilisearch.ServiceManager, error) {
	host := os.Getenv("MEILI_URL")
	if host == "" {
		host = "http://localhost:7700"
	}
	key := os.Getenv("MEILI_API_KEY")
	return meilisearch.New(host, meilisearch.WithAPIKey(key)), nil
}

func newTraceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent LLM traces (most recent first)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			project, _ := cmd.Flags().GetString("project")
			operation, _ := cmd.Flags().GetString("operation")
			status, _ := cmd.Flags().GetString("status")
			asJSON, _ := cmd.Flags().GetBool("json")

			client, err := meiliClient()
			if err != nil {
				return err
			}
			req := &meilisearch.SearchRequest{
				Limit: int64(limit),
				Sort:  []string{"created_at:desc"},
			}
			filters := buildTraceFilter(project, operation, status, "")
			if filters != "" {
				req.Filter = filters
			}
			resp, err := client.Index(trace.IndexName).Search("", req)
			if err != nil {
				return fmt.Errorf("meili search: %w", err)
			}
			return renderTraceHits(resp.Hits, asJSON)
		},
	}
	cmd.Flags().IntP("limit", "n", 20, "Max traces to return")
	cmd.Flags().StringP("project", "P", "", "Filter by project")
	cmd.Flags().String("operation", "", "Filter by operation (e.g. router.memory_action)")
	cmd.Flags().String("status", "", "Filter by status: success | error")
	cmd.Flags().Bool("json", false, "Emit raw JSON")
	return cmd
}

func newTraceSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across prompts and responses",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			project, _ := cmd.Flags().GetString("project")
			operation, _ := cmd.Flags().GetString("operation")
			asJSON, _ := cmd.Flags().GetBool("json")

			client, err := meiliClient()
			if err != nil {
				return err
			}
			req := &meilisearch.SearchRequest{Limit: int64(limit)}
			filters := buildTraceFilter(project, operation, "", "")
			if filters != "" {
				req.Filter = filters
			}
			resp, err := client.Index(trace.IndexName).Search(strings.Join(args, " "), req)
			if err != nil {
				return fmt.Errorf("meili search: %w", err)
			}
			return renderTraceHits(resp.Hits, asJSON)
		},
	}
	cmd.Flags().IntP("limit", "n", 20, "Max results")
	cmd.Flags().StringP("project", "P", "", "Filter by project")
	cmd.Flags().String("operation", "", "Filter by operation")
	cmd.Flags().Bool("json", false, "Emit raw JSON")
	return cmd
}

func newTracePromoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Convert a stored trace into an LLM eval case (prints JSON; --append writes to a dataset)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			appendPath, _ := cmd.Flags().GetString("append")
			match, _ := cmd.Flags().GetString("match")

			client, err := meiliClient()
			if err != nil {
				return err
			}
			var doc map[string]any
			if err := client.Index(trace.IndexName).GetDocument(args[0], nil, &doc); err != nil {
				return fmt.Errorf("get trace %s: %w", args[0], err)
			}

			mode := "text"
			if _, ok := doc["schema"].(string); ok && stringOr(doc, "schema") != "" {
				mode = "json"
			}
			// Heuristic: a response that parses as JSON wants json_field by default.
			if match == "" {
				match = "exact"
				if mode == "json" {
					match = "json_field"
				}
			}

			c := promoteCase{
				ID:       args[0],
				Mode:     mode,
				Model:    stringOr(doc, "model"),
				System:   stringOr(doc, "system"),
				User:     stringOr(doc, "prompt"),
				Expected: stringOr(doc, "response"),
				Match:    match,
			}

			if appendPath != "" {
				if err := appendCaseToDataset(appendPath, c); err != nil {
					return err
				}
				fmt.Printf("appended case %s to %s\n", c.ID, appendPath)
				return nil
			}
			b, _ := json.MarshalIndent(c, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	cmd.Flags().String("append", "", "Append the case to an existing LLMDataset JSON file")
	cmd.Flags().String("match", "", "Match strategy: exact|contains|json_field|judge (auto-picked if empty)")
	return cmd
}

type promoteCase struct {
	ID       string `json:"id"`
	Mode     string `json:"mode"`
	Model    string `json:"model,omitempty"`
	System   string `json:"system,omitempty"`
	User     string `json:"user"`
	Expected string `json:"expected"`
	Match    string `json:"match"`
}

// appendCaseToDataset reads the LLMDataset at path, appends c to its cases
// array, and writes it back atomically.
func appendCaseToDataset(path string, c promoteCase) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read dataset: %w", err)
	}
	var ds struct {
		Description string        `json:"description,omitempty"`
		Model       string        `json:"model,omitempty"`
		Cases       []promoteCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &ds); err != nil {
		return fmt.Errorf("parse dataset: %w", err)
	}
	ds.Cases = append(ds.Cases, c)
	out, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

func newTraceShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Print a single trace by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := meiliClient()
			if err != nil {
				return err
			}
			var doc map[string]any
			if err := client.Index(trace.IndexName).GetDocument(args[0], nil, &doc); err != nil {
				return fmt.Errorf("get trace %s: %w", args[0], err)
			}
			b, _ := json.MarshalIndent(doc, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
}

func buildTraceFilter(project, operation, status, parent string) string {
	var parts []string
	if project != "" {
		parts = append(parts, fmt.Sprintf(`project = "%s"`, escapeMeili(project)))
	}
	if operation != "" {
		parts = append(parts, fmt.Sprintf(`operation = "%s"`, escapeMeili(operation)))
	}
	if status != "" {
		parts = append(parts, fmt.Sprintf(`status = "%s"`, escapeMeili(status)))
	}
	if parent != "" {
		parts = append(parts, fmt.Sprintf(`parent_id = "%s"`, escapeMeili(parent)))
	}
	return strings.Join(parts, " AND ")
}

func escapeMeili(v string) string {
	return strings.ReplaceAll(v, `"`, `\"`)
}

func renderTraceHits(hits meilisearch.Hits, asJSON bool) error {
	rows := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		var m map[string]any
		if err := h.DecodeInto(&m); err != nil {
			continue
		}
		rows = append(rows, m)
	}
	if asJSON {
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	if len(rows) == 0 {
		fmt.Println("(no traces)")
		return nil
	}
	for _, m := range rows {
		id, _ := m["id"].(string)
		op, _ := m["operation"].(string)
		status, _ := m["status"].(string)
		latency := m["latency_ms"]
		created, _ := m["created_at"].(string)
		ts := truncTraceTime(created)
		prompt := truncTraceText(stringOr(m, "prompt"), 80)
		fmt.Printf("%s  %-26s  %-7s  %vms  %s  %s\n", id[:min(12, len(id))], op, status, latency, ts, prompt)
	}
	return nil
}

func stringOr(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func truncTraceText(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func truncTraceTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02 15:04:05")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
