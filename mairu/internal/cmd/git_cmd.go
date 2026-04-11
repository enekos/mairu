package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

func NewGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "AI-Optimized Git helpers",
	}
	cmd.AddCommand(NewGitSummaryCmd())
	return cmd
}

func NewGitSummaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Produces a token-dense, strictly parsed git status and history summary for agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			branch, err := getGitBranch()
			if err != nil {
				return fmt.Errorf("failed to get branch: %v", err)
			}

			staged, unstaged, untracked, conflicts, err := getGitStatus()
			if err != nil {
				return fmt.Errorf("failed to get status: %v", err)
			}

			recentCommits, err := getRecentCommits(5)
			if err != nil {
				// Don't fail completely if log fails (e.g., brand new repo)
				recentCommits = []string{"(No commits or error fetching log)"}
			}

			fmt.Printf("%s=== Git Summary ===%s\n", colorCyan, colorReset)
			fmt.Printf("Branch: %s%s%s\n\n", colorBlue, branch, colorReset)

			if len(conflicts) > 0 {
				fmt.Printf("%s[Conflicts - FIX REQUIRED]%s\n", colorRed, colorReset)
				for _, f := range conflicts {
					fmt.Printf("  %s%s%s\n", colorRed, f, colorReset)
				}
				fmt.Println()
			}

			if len(staged) > 0 {
				fmt.Printf("%s[Staged - Ready to commit]%s\n", colorGreen, colorReset)
				printTruncatedList(staged, 50, colorGreen)
				fmt.Println()

				fmt.Printf("%s[Staged Diff]%s\n", colorCyan, colorReset)
				diffCmd := exec.Command("git", "diff", "--cached", "--color=always")
				diffOut, _ := diffCmd.Output()
				if len(diffOut) > 0 {
					fmt.Println(string(diffOut))
				} else {
					fmt.Println("  (No diff output)")
				}
			}

			if len(unstaged) > 0 {
				fmt.Printf("%s[Unstaged - Modified but not added]%s\n", colorYellow, colorReset)
				printTruncatedList(unstaged, 50, colorYellow)
				fmt.Println()
			}

			if len(untracked) > 0 {
				fmt.Printf("%s[Untracked - New files]%s\n", colorGray, colorReset)
				printTruncatedList(untracked, 50, colorGray)
				fmt.Println()
			}

			if len(staged) == 0 && len(unstaged) == 0 && len(untracked) == 0 && len(conflicts) == 0 {
				fmt.Printf("%sWorking tree clean.%s\n\n", colorGreen, colorReset)
			}

			fmt.Printf("%s[Recent Commits]%s\n", colorCyan, colorReset)
			for _, c := range recentCommits {
				// Let's colorize the commit hash slightly different if we want, but it's fine
				fmt.Printf("  %s\n", c)
			}

			return nil
		},
	}
}

func getGitBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getGitStatus() (staged, unstaged, untracked, conflicts []string, err error) {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}

		x := line[0]
		y := line[1]
		file := strings.TrimSpace(line[3:])

		// Check for conflicts
		if (x == 'D' && y == 'D') || (x == 'A' && y == 'U') ||
			(x == 'U' && y == 'D') || (x == 'U' && y == 'A') ||
			(x == 'D' && y == 'U') || (x == 'A' && y == 'A') ||
			(x == 'U' && y == 'U') {
			conflicts = append(conflicts, fmt.Sprintf("%c%c %s", x, y, file))
			continue
		}

		// Untracked
		if x == '?' && y == '?' {
			untracked = append(untracked, file)
			continue
		}

		// Staged
		if x != ' ' && x != '?' {
			staged = append(staged, fmt.Sprintf("%c  %s", x, file))
		}

		// Unstaged
		if y != ' ' && y != '?' {
			unstaged = append(unstaged, fmt.Sprintf(" %c %s", y, file))
		}
	}

	return staged, unstaged, untracked, conflicts, nil
}

func getRecentCommits(n int) ([]string, error) {
	out, err := exec.Command("git", "log", fmt.Sprintf("-n%d", n), "--oneline").Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(bytes.TrimSpace(out)), "\n")
	var commits []string
	for _, line := range lines {
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits, nil
}

func printTruncatedList(list []string, max int, color string) {
	for i, f := range list {
		if i >= max {
			fmt.Printf("  ... and %d more files\n", len(list)-max)
			break
		}
		if color != "" {
			fmt.Printf("  %s%s%s\n", color, f, colorReset)
		} else {
			fmt.Printf("  %s\n", f)
		}
	}
}
