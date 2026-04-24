package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func NewLinearCmd() *cobra.Command {
	var project string
	var limit int
	var state string

	cmd := &cobra.Command{
		Use:   "linear",
		Short: "Linear integration commands",
	}
	addProjectFlag(cmd, &project)

	syncIssuesCmd := &cobra.Command{
		Use:   "sync-issues <team_key>",
		Short: "Sync Linear issues into context nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				return fmt.Errorf("project (-P) is required")
			}
			teamKey := args[0]
			return syncLinearIssues(project, teamKey, limit, state)
		},
	}
	syncIssuesCmd.Flags().IntVarP(&limit, "limit", "l", 50, "Limit the number of issues to fetch")
	syncIssuesCmd.Flags().StringVarP(&state, "state", "s", "", "Filter by state name (e.g. 'In Progress', 'Done')")

	cmd.AddCommand(syncIssuesCmd)
	return cmd
}

type linearIssueNode struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	State       struct {
		Name string `json:"name"`
	} `json:"state"`
	Assignee struct {
		Name string `json:"name"`
	} `json:"assignee"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func syncLinearIssues(project, teamKey string, limit int, state string) error {
	apiKey := os.Getenv("LINEAR_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("LINEAR_API_KEY environment variable is required")
	}

	query := `
		query($teamKey: String!, $first: Int!, $after: String) {
			issues(filter: { team: { key: { eq: $teamKey } } }, first: $first, after: $after) {
				pageInfo {
					hasNextPage
					endCursor
				}
				nodes {
					identifier
					title
					description
					priority
					state {
						name
					}
					assignee {
						name
					}
					labels {
						nodes {
							name
						}
					}
					url
					createdAt
					updatedAt
				}
			}
		}
	`

	client := &http.Client{Timeout: 10 * time.Second}
	var allIssues []linearIssueNode
	var endCursor *string
	fetchedCount := 0

	for fetchedCount < limit {
		fetchSize := 100
		if limit-fetchedCount < fetchSize {
			fetchSize = limit - fetchedCount
		}

		variables := map[string]any{
			"teamKey": teamKey,
			"first":   fetchSize,
		}
		if endCursor != nil {
			variables["after"] = *endCursor
		}

		reqBody, _ := json.Marshal(map[string]any{
			"query":     query,
			"variables": variables,
		})

		req, err := http.NewRequest("POST", "https://api.linear.app/graphql", bytes.NewBuffer(reqBody))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to fetch linear issues: %w", err)
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("linear api returned status %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Data struct {
				Issues struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []linearIssueNode `json:"nodes"`
				} `json:"issues"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to decode linear issues: %w", err)
		}
		resp.Body.Close()

		if len(result.Errors) > 0 {
			return fmt.Errorf("linear graphql error: %s", result.Errors[0].Message)
		}

		nodes := result.Data.Issues.Nodes
		for _, issue := range nodes {
			if state != "" && !strings.EqualFold(issue.State.Name, state) {
				continue // skip if state filter doesn't match
			}
			allIssues = append(allIssues, issue)
		}

		fetchedCount += len(nodes)
		if !result.Data.Issues.PageInfo.HasNextPage {
			break
		}
		cursor := result.Data.Issues.PageInfo.EndCursor
		endCursor = &cursor
	}

	if len(allIssues) > limit {
		allIssues = allIssues[:limit]
	}

	fmt.Printf("Fetched %d issues from Linear team %s\n", len(allIssues), teamKey)

	for _, issue := range allIssues {
		labels := make([]string, len(issue.Labels.Nodes))
		for i, l := range issue.Labels.Nodes {
			labels[i] = l.Name
		}

		node := integrationIssue{
			Source: "linear",
			ID:     issue.Identifier,
			Name:   fmt.Sprintf("Issue %s: %s", issue.Identifier, issue.Title),
			Abstract: []string{
				fmt.Sprintf("Linear Issue [%s]", issue.State.Name),
			},
			Overview: []string{
				fmt.Sprintf("Title: %s", issue.Title),
				fmt.Sprintf("URL: %s", issue.URL),
				fmt.Sprintf("Created: %s", issue.CreatedAt),
				fmt.Sprintf("Updated: %s", issue.UpdatedAt),
			},
			Content: issue.Description,
		}
		if issue.Priority > 0 {
			node.Abstract = append(node.Abstract, fmt.Sprintf("Priority: %d", issue.Priority))
		}
		if len(labels) > 0 {
			node.Abstract = append(node.Abstract, fmt.Sprintf("Labels: %s", strings.Join(labels, ", ")))
		}
		if issue.Assignee.Name != "" {
			node.Overview = append(node.Overview, fmt.Sprintf("Assignee: %s", issue.Assignee.Name))
		}

		if err := syncIntegrationNode(project, node); err != nil {
			fmt.Printf("Failed to store node for issue %s: %v\n", issue.Identifier, err)
			continue
		}
		fmt.Printf("Synced %s\n", issue.Identifier)
	}

	syncIntegrationSummary(project, "linear",
		fmt.Sprintf("Linear issues from team %s", teamKey), len(allIssues))
	return nil
}
