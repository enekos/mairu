package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cobra"
)

var scanBudget int
var scanContext int
var scanExtensions string
var scanLimit int

func init() {
	scanCmd.Flags().IntVar(&scanBudget, "budget", 3000, "Token budget circuit breaker")
	scanCmd.Flags().IntVarP(&scanContext, "context", "C", 0, "Number of context lines around match")
	scanCmd.Flags().StringVarP(&scanExtensions, "ext", "e", "", "Comma-separated extensions to filter (e.g. .go,.ts)")
	scanCmd.Flags().IntVarP(&scanLimit, "limit", "n", 0, "Max number of matches to return (0 = unlimited)")
	rootCmd.AddCommand(scanCmd)
}

type scanMatch struct {
	F string `json:"f"`
	L int    `json:"l"`
	C string `json:"c"`
}

type scanResult struct {
	BudgetHit bool        `json:"budget_hit"`
	LimitHit  bool        `json:"limit_hit"`
	Total     int         `json:"total"`
	Matches   []scanMatch `json:"matches"`
}

var scanCmd = &cobra.Command{
	Use:   "scan <regex> [dir]",
	Short: "AI-optimized semantic search with token budget (JSON)",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pattern := args[0]
		dir := "."
		if len(args) > 1 {
			dir = args[1]
		}

		re, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error compiling regex: %v\n", err)
			os.Exit(1)
		}

		var ignorer *ignore.GitIgnore
		if gi, err := ignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore")); err == nil {
			ignorer = gi
		}

		allowedExts := make(map[string]bool)
		if scanExtensions != "" {
			for _, ext := range strings.Split(scanExtensions, ",") {
				ext = strings.TrimSpace(ext)
				if !strings.HasPrefix(ext, ".") {
					ext = "." + ext
				}
				allowedExts[strings.ToLower(ext)] = true
			}
		}

		res := scanResult{Matches: []scanMatch{}}
		var currentBytes int
		// roughly 4 bytes per token
		maxBytes := scanBudget * 4

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if path == dir {
				return nil
			}

			rel, _ := filepath.Rel(dir, path)
			if rel == ".git" || rel == "node_modules" {
				return filepath.SkipDir
			}
			if ignorer != nil && ignorer.MatchesPath(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !d.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))

				// Apply extension filter if provided
				if len(allowedExts) > 0 && !allowedExts[ext] {
					return nil
				}

				// Standard binary skip
				if ext == ".png" || ext == ".jpg" || ext == ".exe" || ext == ".bin" || ext == ".pdf" || ext == ".mp4" || ext == ".zip" || ext == ".tar" || ext == ".gz" {
					return nil
				}

				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				lines := strings.Split(string(content), "\n")

				// To handle context overlapping correctly within a single file
				var lastMatchEndIdx int = -1

				for i, line := range lines {
					if re.MatchString(line) {
						res.Total++

						// If we hit limits, we still want to count totals but we stop adding to Matches
						if (scanLimit > 0 && len(res.Matches) >= scanLimit) || res.BudgetHit {
							if !res.LimitHit && len(res.Matches) >= scanLimit {
								res.LimitHit = true
							}
							continue
						}

						startIdx := i - scanContext
						if startIdx < 0 {
							startIdx = 0
						}

						// Prevent overlapping context blocks in the same file from duplicating lines
						if startIdx <= lastMatchEndIdx {
							startIdx = lastMatchEndIdx + 1
						}

						if startIdx > i {
							// This match was completely subsumed by the previous match's context
							continue
						}

						endIdx := i + scanContext
						if endIdx >= len(lines) {
							endIdx = len(lines) - 1
						}

						lastMatchEndIdx = endIdx

						var snippet []string
						for j := startIdx; j <= endIdx; j++ {
							// Don't totally trim space, just replace tabs with spaces for compact JSON
							cleanLine := strings.ReplaceAll(lines[j], "\t", "  ")
							snippet = append(snippet, cleanLine)
						}

						joined := strings.Join(snippet, "\n")
						matchBytes := len(rel) + len(joined) + 20

						if currentBytes+matchBytes > maxBytes {
							res.BudgetHit = true
							continue
						}
						currentBytes += matchBytes
						res.Matches = append(res.Matches, scanMatch{
							F: rel,
							L: i + 1,
							C: joined,
						})
					}
				}
			}
			return nil
		})

		out, _ := json.Marshal(res)
		fmt.Println(string(out))
	},
}
