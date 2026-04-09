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
var scanBeforeContext int
var scanAfterContext int
var scanExtensions string
var scanLimit int
var scanMaxCount int
var scanIgnoreCase bool
var scanFilesOnly bool
var scanFilesWithoutMatch bool
var scanHeading bool
var scanExclude string
var scanGroup bool
var scanInvert bool
var scanMulti string
var scanWordRegexp bool
var scanFixedStrings bool
var scanHidden bool

func init() {
	scanCmd.Flags().IntVar(&scanBudget, "budget", 3000, "Token budget circuit breaker")
	scanCmd.Flags().IntVarP(&scanContext, "context", "C", 0, "Number of context lines around match")
	scanCmd.Flags().IntVarP(&scanBeforeContext, "before-context", "B", 0, "Number of context lines before match")
	scanCmd.Flags().IntVarP(&scanAfterContext, "after-context", "A", 0, "Number of context lines after match")
	scanCmd.Flags().StringVarP(&scanExtensions, "ext", "e", "", "Comma-separated extensions to filter (e.g. .go,.ts)")
	scanCmd.Flags().IntVarP(&scanLimit, "limit", "n", 0, "Max number of matches to return (0 = unlimited)")
	scanCmd.Flags().IntVarP(&scanMaxCount, "max-count", "m", 0, "Max number of matches per file (0 = unlimited)")
	scanCmd.Flags().BoolVarP(&scanIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	scanCmd.Flags().BoolVarP(&scanFilesOnly, "files-with-matches", "l", false, "Only print matching filenames")
	scanCmd.Flags().BoolVarP(&scanFilesWithoutMatch, "files-without-match", "L", false, "Only print filenames that contain no matches")
	scanCmd.Flags().BoolVarP(&scanHeading, "heading", "H", false, "Attempt to find nearest function/class heading above match")
	scanCmd.Flags().StringVarP(&scanExclude, "exclude", "x", "", "Comma-separated glob patterns to exclude (e.g. vendor/*,*_test.go)")
	scanCmd.Flags().BoolVarP(&scanGroup, "group", "g", false, "Group matches by file")
	scanCmd.Flags().BoolVarP(&scanInvert, "invert", "v", false, "Invert match (select non-matching lines)")
	scanCmd.Flags().StringVar(&scanMulti, "multi", "", "Additional patterns that must ALL match in the file (comma-separated, AND logic)")
	scanCmd.Flags().BoolVarP(&scanWordRegexp, "word-regexp", "w", false, "Only match whole words")
	scanCmd.Flags().BoolVarP(&scanFixedStrings, "fixed-strings", "F", false, "Treat pattern as a literal string instead of a regular expression")
	scanCmd.Flags().BoolVar(&scanHidden, "hidden", false, "Search hidden files and directories")
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

type scanGroupedResult struct {
	BudgetHit bool                   `json:"budget_hit"`
	LimitHit  bool                   `json:"limit_hit"`
	Total     int                    `json:"total"`
	Grouped   map[string][]scanMatch `json:"grouped"`
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

		if scanFixedStrings {
			pattern = regexp.QuoteMeta(pattern)
		}
		if scanWordRegexp {
			pattern = "\\b" + pattern + "\\b"
		}
		if scanIgnoreCase {
			pattern = "(?i)" + pattern
		}

		re, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error compiling regex: %v\n", err)
			os.Exit(1)
		}

		var multiRes []*regexp.Regexp
		if scanMulti != "" {
			for _, p := range strings.Split(scanMulti, ",") {
				p = strings.TrimSpace(p)
				if scanFixedStrings {
					p = regexp.QuoteMeta(p)
				}
				if scanWordRegexp {
					p = "\\b" + p + "\\b"
				}
				if scanIgnoreCase {
					p = "(?i)" + p
				}
				mre, err := regexp.Compile(p)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error compiling multi regex: %v\n", err)
					os.Exit(1)
				}
				multiRes = append(multiRes, mre)
			}
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

		var excludePatterns []string
		if scanExclude != "" {
			for _, p := range strings.Split(scanExclude, ",") {
				excludePatterns = append(excludePatterns, strings.TrimSpace(p))
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

		var beforeContext, afterContext int
		if scanContext > 0 {
			beforeContext = scanContext
			afterContext = scanContext
		} else {
			beforeContext = scanBeforeContext
			afterContext = scanAfterContext
		}

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if path == dir {
				return nil
			}

			rel, _ := filepath.Rel(dir, path)

			if !scanHidden && strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if rel == ".git" || rel == "node_modules" {
				return filepath.SkipDir
			}
			if ignorer != nil && ignorer.MatchesPath(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if len(excludePatterns) > 0 {
				for _, pat := range excludePatterns {
					if matched, _ := filepath.Match(pat, rel); matched {
						if d.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
				}
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

				if len(multiRes) > 0 {
					allMatch := true
					for _, mre := range multiRes {
						if !mre.Match(content) {
							allMatch = false
							break
						}
					}
					if !allMatch {
						return nil
					}
				}

				lines := strings.Split(string(content), "\n")
				var lastMatchEndIdx int = -1
				fileHasMatch := false
				fileMatchCount := 0

				for i, line := range lines {
					matched := re.MatchString(line)
					if scanInvert {
						matched = !matched
					}
					if matched {
						fileHasMatch = true
						fileMatchCount++

						if scanFilesWithoutMatch {
							break
						}

						if scanFilesOnly {
							res.Files = append(res.Files, rel)
							res.Total++
							break
						}

						if scanMaxCount > 0 && fileMatchCount > scanMaxCount {
							break
						}

						res.Total++

						if (scanLimit > 0 && len(res.Matches) >= scanLimit) || res.BudgetHit {
							if !res.LimitHit && len(res.Matches) >= scanLimit {
								res.LimitHit = true
							}
							continue
						}

						startIdx := i - beforeContext
						if startIdx < 0 {
							startIdx = 0
						}

						if startIdx <= lastMatchEndIdx {
							startIdx = lastMatchEndIdx + 1
						}

						if startIdx > i {
							continue
						}

						endIdx := i + afterContext
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

				if scanFilesWithoutMatch && !fileHasMatch {
					res.Files = append(res.Files, rel)
					res.Total++
				}
			}
			return nil
		})

		if outputFormat == "json" {
			if scanGroup {
				grouped := scanGroupedResult{
					BudgetHit: res.BudgetHit,
					LimitHit:  res.LimitHit,
					Total:     res.Total,
					Grouped:   make(map[string][]scanMatch),
				}
				for _, m := range res.Matches {
					inner := scanMatch{L: m.L, C: m.C, Heading: m.Heading}
					grouped.Grouped[m.F] = append(grouped.Grouped[m.F], inner)
				}
				out, _ := json.Marshal(grouped)
				fmt.Println(string(out))
				return
			}
			out, _ := json.Marshal(res)
			fmt.Println(string(out))
		} else {
			f := GetFormatter()
			if res.BudgetHit {
				fmt.Println("Warning: Token budget hit, results truncated.")
			}
			if res.LimitHit {
				fmt.Printf("Warning: Match limit hit (%d matches).\n", scanLimit)
			}

			if scanFilesOnly || scanFilesWithoutMatch {
				var files []string
				seen := make(map[string]bool)
				for _, f := range res.Files {
					if !seen[f] {
						seen[f] = true
						files = append(files, f)
					}
				}
				items := make([]map[string]any, len(files))
				for i, file := range files {
					items[i] = map[string]any{"file": file}
				}
				f.PrintItems([]string{"file"}, items, func(item map[string]any) map[string]string {
					return map[string]string{"file": fmt.Sprintf("%v", item["file"])}
				})
			} else if scanGroup {
				grouped := make(map[string][]scanMatch)
				for _, m := range res.Matches {
					grouped[m.F] = append(grouped[m.F], m)
				}
				for file, matches := range grouped {
					fmt.Printf("\n%s:\n", file)
					items := make([]map[string]any, len(matches))
					for i, m := range matches {
						items[i] = map[string]any{
							"line":    m.L,
							"content": m.C,
							"heading": m.Heading,
						}
					}
					cols := []string{"line", "content"}
					if scanHeading {
						cols = []string{"line", "heading", "content"}
					}
					f.PrintItems(cols, items, func(item map[string]any) map[string]string {
						res := map[string]string{
							"line":    fmt.Sprintf("%v", item["line"]),
							"content": fmt.Sprintf("%v", item["content"]),
						}
						if scanHeading {
							res["heading"] = fmt.Sprintf("%v", item["heading"])
						}
						return res
					})
				}
			} else {
				items := make([]map[string]any, len(res.Matches))
				for i, m := range res.Matches {
					items[i] = map[string]any{
						"file":    m.F,
						"line":    m.L,
						"content": m.C,
						"heading": m.Heading,
					}
				}
				cols := []string{"file", "line", "content"}
				if scanHeading {
					cols = []string{"file", "line", "heading", "content"}
				}
				f.PrintItems(cols, items, func(item map[string]any) map[string]string {
					res := map[string]string{
						"file":    fmt.Sprintf("%v", item["file"]),
						"line":    fmt.Sprintf("%v", item["line"]),
						"content": fmt.Sprintf("%v", item["content"]),
					}
					if scanHeading {
						res["heading"] = fmt.Sprintf("%v", item["heading"])
					}
					return res
				})
			}
		}
	},
}
