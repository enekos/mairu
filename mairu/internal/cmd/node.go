package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func runNodeStore(project, uri, name, abstract, parent, overview, content string) error {
	payload := map[string]any{
		"uri":      uri,
		"project":  project,
		"name":     name,
		"abstract": abstract,
	}
	if overview != "" {
		payload["overview"] = overview
	}
	if content != "" {
		payload["content"] = content
	}
	if parent != "" {
		payload["parent_uri"] = parent
	}
	out, err := ContextPost("/api/context", payload)
	if err != nil {
		return err
	}
	PrintJSON(out)
	return nil
}

func NewNodeCmd() *cobra.Command {
	var project string
	c := &cobra.Command{
		Use:   "node",
		Short: "ContextFS node operations",
	}
	c.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search context nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := ContextGet("/api/search", SearchParamsFromFlags(cmd, args[0], "node", project))
			if err != nil {
				return err
			}
			if outputFormat == "json" || outputFormat == "" {
				PrintJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					PrintJSON(out)
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
							"name":     fmt.Sprintf("%v", item["name"]),
							"abstract": Truncate(fmt.Sprintf("%v", item["abstract"]), 80),
						}
					},
				)
			}
			return nil
		},
	}
	AddCommonSearchFlags(searchCmd)

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
	storeCmd.Flags().StringP("parent", "p", "", "Parent URI")
	storeCmd.Flags().StringP("overview", "o", "", "Overview content")
	storeCmd.Flags().StringP("content", "c", "", "Detailed content")

	addCmd := &cobra.Command{
		Use:    "add <uri> <name> <abstract>",
		Short:  "Alias for node store",
		Args:   cobra.ExactArgs(3),
		Hidden: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			parent, _ := cmd.Flags().GetString("parent")
			overview, _ := cmd.Flags().GetString("overview")
			content, _ := cmd.Flags().GetString("content")
			return runNodeStore(project, args[0], args[1], args[2], parent, overview, content)
		},
	}
	addCmd.Flags().StringP("parent", "p", "", "Parent URI")
	addCmd.Flags().StringP("overview", "o", "", "Overview content")
	addCmd.Flags().StringP("content", "c", "", "Detailed content")

	listCmd := &cobra.Command{
		Use:   "list [parent_uri]",
		Short: "List context nodes (optionally under a parent)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			params := map[string]string{
				"project": project,
				"limit":   fmt.Sprintf("%d", limit),
			}
			if len(args) > 0 {
				params["parent_uri"] = args[0]
			}
			out, err := ContextGet("/api/context", params)
			if err != nil {
				return err
			}
			if outputFormat == "json" || outputFormat == "" {
				PrintJSON(out)
			} else {
				var results []map[string]any
				if err := json.Unmarshal(out, &results); err != nil {
					PrintJSON(out)
					return nil
				}
				f := GetFormatter()
				f.PrintItems(
					[]string{"uri", "name", "abstract"},
					results,
					func(item map[string]any) map[string]string {
						return map[string]string{
							"uri":      fmt.Sprintf("%v", item["uri"]),
							"name":     fmt.Sprintf("%v", item["name"]),
							"abstract": Truncate(fmt.Sprintf("%v", item["abstract"]), 80),
						}
					},
				)
			}
			return nil
		},
	}
	listCmd.Flags().Int("limit", 200, "Maximum items")

	lsCmd := &cobra.Command{
		Use:   "ls [parent_uri]",
		Short: "Alias for list",
		Args:  cobra.MaximumNArgs(1),
		RunE:  listCmd.RunE,
	}
	lsCmd.Flags().AddFlagSet(listCmd.Flags())

	updateCmd := &cobra.Command{
		Use:   "update <uri>",
		Short: "Update a context node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{"uri": args[0]}
			if name, _ := cmd.Flags().GetString("name"); name != "" {
				payload["name"] = name
			}
			if abs, _ := cmd.Flags().GetString("abstract"); abs != "" {
				payload["abstract"] = abs
			}
			if over, _ := cmd.Flags().GetString("overview"); over != "" {
				payload["overview"] = over
			}
			if cont, _ := cmd.Flags().GetString("content"); cont != "" {
				payload["content"] = cont
			}
			out, err := ContextPut("/api/context", payload)
			if err != nil {
				return err
			}
			PrintJSON(out)
			return nil
		},
	}
	updateCmd.Flags().String("name", "", "New name")
	updateCmd.Flags().String("abstract", "", "New abstract")
	updateCmd.Flags().String("overview", "", "New overview")
	updateCmd.Flags().String("content", "", "New content")

	deleteCmd := &cobra.Command{
		Use:   "delete <uri>",
		Short: "Delete context node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := ContextDelete("/api/context", map[string]string{"uri": args[0]})
			if err != nil {
				return err
			}
			PrintJSON(out)
			return nil
		},
	}

	c.AddCommand(searchCmd, storeCmd, addCmd, listCmd, lsCmd, updateCmd, deleteCmd)
	return c
}
