package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

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
