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
var scanIgnoreCase bool
var scanFilesOnly bool
var scanHeading bool

func init() {
	scanCmd.Flags().IntVar(&scanBudget, "budget", 3000, "Token budget circuit breaker")
	scanCmd.Flags().IntVarP(&scanContext, "context", "C", 0, "Number of context lines around match")
	scanCmd.Flags().StringVarP(&scanExtensions, "ext", "e", "", "Comma-separated extensions to filter (e.g. .go,.ts)")
	scanCmd.Flags().IntVarP(&scanLimit, "limit", "n", 0, "Max number of matches to return (0 = unlimited)")
	scanCmd.Flags().BoolVarP(&scanIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	scanCmd.Flags().BoolVarP(&scanFilesOnly, "files-with-matches", "l", false, "Only print matching filenames")
	scanCmd.Flags().BoolVarP(&scanHeading, "heading", "H", false, "Attempt to find nearest function/class heading above match")
	rootCmd.AddCommand(scanCmd)
}

type scanMatch struct {
	F       string `json:"f"`
	L       int    `json:"l,omitempty"`
	C       string `json:"c,omitempty"`
	Heading string `json:"heading,omitempty"`
}

type scanResult struct {
	BudgetHit bool        `json:"budget_hit"`
	LimitHit  bool        `json:"limit_hit"`
	Total     int         `json:"total"`
	Files     []string    `json:"files,omitempty"`
	Matches   []scanMatch `json:"matches,omitempty"`
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

		if scanIgnoreCase {
			pattern = "(?i)" + pattern
		}

		re, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error compiling regex: %v\n", err)
			os.Exit(1)
		}

		// Pre-compile heading regex if needed
		// This matches typical top-level declarations like `func ...`, `class ...`, `export const ...`
		var headingRe *regexp.Regexp
		if scanHeading {
			headingRe = regexp.MustCompile(`^(?:export\s+|public\s+|private\s+|protected\s+)?(?:func|class|type|interface|def|const|let|var)\b|^\s*[a-zA-Z_]\w*\s*\(|^\s*type\s+`)
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

		res := scanResult{}
		if !scanFilesOnly {
			res.Matches = []scanMatch{}
		} else {
			res.Files = []string{}
		}

		var currentBytes int
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

				if len(allowedExts) > 0 && !allowedExts[ext] {
					return nil
				}

				if ext == ".png" || ext == ".jpg" || ext == ".exe" || ext == ".bin" || ext == ".pdf" || ext == ".mp4" || ext == ".zip" || ext == ".tar" || ext == ".gz" {
					return nil
				}

				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				lines := strings.Split(string(content), "\n")
				var lastMatchEndIdx int = -1
				fileHasMatch := false

				for i, line := range lines {
					if re.MatchString(line) {
						if scanFilesOnly {
							if !fileHasMatch {
								res.Files = append(res.Files, rel)
								res.Total++
								fileHasMatch = true
							}
							break // Skip the rest of this file since we just need the filename
						}

						res.Total++

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

						if startIdx <= lastMatchEndIdx {
							startIdx = lastMatchEndIdx + 1
						}

						if startIdx > i {
							continue
						}

						endIdx := i + scanContext
						if endIdx >= len(lines) {
							endIdx = len(lines) - 1
						}

						lastMatchEndIdx = endIdx

						var snippet []string
						for j := startIdx; j <= endIdx; j++ {
							cleanLine := strings.ReplaceAll(lines[j], "\t", "  ")
							snippet = append(snippet, cleanLine)
						}
						joined := strings.Join(snippet, "\n")

						// Extract Heading if requested
						foundHeading := ""
						if scanHeading {
							for k := i; k >= 0; k-- {
								hLine := lines[k]
								// Fast check: look for our regex match, or a line with zero indentation that's not empty
								if headingRe.MatchString(hLine) || (len(hLine) > 0 && (hLine[0] != ' ' && hLine[0] != '\t' && hLine[0] != '/' && hLine[0] != '#')) {
									foundHeading = strings.TrimSpace(hLine)
									// truncate if insanely long
									if len(foundHeading) > 80 {
										foundHeading = foundHeading[:77] + "..."
									}
									break
								}
							}
						}

						matchBytes := len(rel) + len(joined) + len(foundHeading) + 30

						if currentBytes+matchBytes > maxBytes {
							res.BudgetHit = true
							continue
						}
						currentBytes += matchBytes

						match := scanMatch{
							F: rel,
							L: i + 1,
							C: joined,
						}
						if scanHeading && foundHeading != "" {
							match.Heading = foundHeading
						}

						res.Matches = append(res.Matches, match)
					}
				}
			}
			return nil
		})

		out, _ := json.Marshal(res)
		fmt.Println(string(out))
	},
}
