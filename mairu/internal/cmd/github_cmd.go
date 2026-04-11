package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func NewGithubCmd() *cobra.Command {
	var project string
	var limit int
	var state string

	cmd := &cobra.Command{
		Use:   "github",
		Short: "GitHub integration commands",
	}
	cmd.PersistentFlags().StringVarP(&project, "project", "P", "", "Project name")

	syncIssuesCmd := &cobra.Command{
		Use:   "sync-issues <owner/repo>",
		Short: "Sync GitHub issues and PRs into context nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				return fmt.Errorf("project (-P) is required")
			}
			repo := args[0]
			return syncGithubIssues(project, repo, limit, state)
		},
	}
	syncIssuesCmd.Flags().IntVarP(&limit, "limit", "l", 50, "Limit the number of issues to fetch")
	syncIssuesCmd.Flags().StringVarP(&state, "state", "s", "all", "State of issues to fetch (open, closed, all)")

	cmd.AddCommand(syncIssuesCmd)
	return cmd
}

type githubIssue struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	PullReq   any    `json:"pull_request"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func syncGithubIssues(project, repo string, limit int, state string) error {
	token := os.Getenv("GITHUB_TOKEN")

	page := 1
	perPage := 100
	if limit < perPage {
		perPage = limit
	}

	fetchedCount := 0
	var allIssues []githubIssue

	client := &http.Client{Timeout: 10 * time.Second}

	for fetchedCount < limit {
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/issues?state=%s&per_page=%d&page=%d",
			repo, url.QueryEscape(state), perPage, page)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to fetch github issues: %w", err)
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("github api returned status %d: %s", resp.StatusCode, string(body))
		}

		var issues []githubIssue
		if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to decode github issues: %w", err)
		}
		resp.Body.Close()

		if len(issues) == 0 {
			break // No more pages
		}

		allIssues = append(allIssues, issues...)
		fetchedCount += len(issues)
		page++

		if len(issues) < perPage {
			break // Last page
		}
	}

	if len(allIssues) > limit {
		allIssues = allIssues[:limit]
	}

	fmt.Printf("Fetched %d issues from %s\n", len(allIssues), repo)

	for _, issue := range allIssues {
		itemType := "Issue"
		if issue.PullReq != nil {
			itemType = "PR"
		}

		labels := make([]string, len(issue.Labels))
		for i, l := range issue.Labels {
			labels[i] = l.Name
		}

		assignees := make([]string, len(issue.Assignees))
		for i, a := range issue.Assignees {
			assignees[i] = a.Login
		}

		// Store as a Context Node
		uri := fmt.Sprintf("contextfs://%s/github/issues/%d", project, issue.Number)
		name := fmt.Sprintf("%s #%d: %s", itemType, issue.Number, issue.Title)

		abstractParts := []string{
			fmt.Sprintf("GitHub %s [%s]", itemType, issue.State),
		}
		if len(labels) > 0 {
			abstractParts = append(abstractParts, fmt.Sprintf("Labels: %s", strings.Join(labels, ", ")))
		}
		abstract := strings.Join(abstractParts, " | ")

		overview := fmt.Sprintf("Title: %s\nURL: %s\nAuthor: %s\nCreated: %s\nUpdated: %s",
			issue.Title, issue.HTMLURL, issue.User.Login, issue.CreatedAt, issue.UpdatedAt)

		if len(assignees) > 0 {
			overview += fmt.Sprintf("\nAssignees: %s", strings.Join(assignees, ", "))
		}

		content := issue.Body
		if len(content) > 5000 {
			content = content[:5000] + "\n...(truncated)"
		}

		// Store node
		_, err := StoreNodeRaw(project, uri, name, abstract, fmt.Sprintf("contextfs://%s/github", project), overview, content)
		if err != nil {
			fmt.Printf("Failed to store node for issue #%d: %v\n", issue.Number, err)
			continue
		}
		fmt.Printf("Synced %s #%d\n", itemType, issue.Number)
	}

	// Store a single summary memory instead of spamming memories per issue
	memContent := fmt.Sprintf("Synced %d GitHub issues and PRs from repository %s into project '%s'.", len(allIssues), repo, project)
	_ = RunMemoryStore(project, memContent, "github_sync", "github", 7)

	return nil
}
