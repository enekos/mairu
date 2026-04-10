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

func newLinearCmd() *cobra.Command {
	var project string
	var limit int
	var state string

	cmd := &cobra.Command{
		Use:   "linear",
		Short: "Linear integration commands",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

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

		uri := fmt.Sprintf("contextfs://%s/linear/issues/%s", project, issue.Identifier)
		name := fmt.Sprintf("Issue %s: %s", issue.Identifier, issue.Title)

		abstractParts := []string{
			fmt.Sprintf("Linear Issue [%s]", issue.State.Name),
		}
		if issue.Priority > 0 {
			abstractParts = append(abstractParts, fmt.Sprintf("Priority: %d", issue.Priority))
		}
		if len(labels) > 0 {
			abstractParts = append(abstractParts, fmt.Sprintf("Labels: %s", strings.Join(labels, ", ")))
		}
		abstract := strings.Join(abstractParts, " | ")

		overview := fmt.Sprintf("Title: %s\nURL: %s\nCreated: %s\nUpdated: %s",
			issue.Title, issue.URL, issue.CreatedAt, issue.UpdatedAt)

		if issue.Assignee.Name != "" {
			overview += fmt.Sprintf("\nAssignee: %s", issue.Assignee.Name)
		}

		content := issue.Description
		if len(content) > 5000 {
			content = content[:5000] + "\n...(truncated)"
		}

		_, err := StoreNodeRaw(project, uri, name, abstract, fmt.Sprintf("contextfs://%s/linear", project), overview, content)
		if err != nil {
			fmt.Printf("Failed to store node for issue %s: %v\n", issue.Identifier, err)
			continue
		}
		fmt.Printf("Synced %s\n", issue.Identifier)
	}

	memContent := fmt.Sprintf("Synced %d Linear issues from team %s into project '%s'.", len(allIssues), teamKey, project)
	_ = RunMemoryStore(project, memContent, "linear_sync", "linear", 7)

	return nil
}
