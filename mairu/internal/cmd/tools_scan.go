package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charlievieth/fastwalk"
	"github.com/spf13/cobra"

	"mairu/internal/fsutil"
)

var scanBudget int
var scanContext int
var scanExtensions string
var scanLimit int
var scanIgnoreCase bool
var scanFilesOnly bool
var scanHeading bool
var scanExclude string
var scanGroup bool
var scanInvert bool
var scanMulti string
var scanFixedStrings bool
var scanSmartCase bool
var scanHidden bool
var scanWordRegexp bool
var scanOnlyMatching bool
var scanBeforeContext int
var scanAfterContext int
var scanMaxCount int
var scanFilesWithoutMatch bool

type scanMatch struct {
	F       string `json:"f"`
	L       int    `json:"l,omitempty"`
	EndL    int    `json:"end_l,omitempty"`
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

func NewScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan <regex> [dir]",
		Short: "AI-optimized semantic search with token budget (JSON)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pattern := args[0]
			dir := "."
			if len(args) > 1 {
				dir = args[1]
			}

			if scanSmartCase {
				hasUppercase := false
				for _, r := range pattern {
					if r >= 'A' && r <= 'Z' {
						hasUppercase = true
						break
					}
				}
				if !hasUppercase {
					scanIgnoreCase = true
				}
			}

			if scanFixedStrings {
				pattern = regexp.QuoteMeta(pattern)
			}

			if scanWordRegexp {
				pattern = `\b` + pattern + `\b`
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
						p = `\b` + p + `\b`
					}

					if scanSmartCase {
						hasUppercase := false
						for _, r := range p {
							if r >= 'A' && r <= 'Z' {
								hasUppercase = true
								break
							}
						}
						if !hasUppercase {
							p = "(?i)" + p
						}
					} else if scanIgnoreCase {
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

			var headingRe *regexp.Regexp
			if scanHeading {
				headingRe = regexp.MustCompile(`^(?:export\s+|public\s+|private\s+|protected\s+)?(?:func|class|type|interface|def|const|let|var)\b|^\s*[a-zA-Z_]\w*\s*\(|^\s*type\s+`)
			}

			ignorer := fsutil.NewProjectIgnorer(dir)

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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var currentBytes int32
			maxBytes := int32(scanBudget * 4)

			var beforeContext, afterContext int
			if scanContext > 0 {
				beforeContext = scanContext
				afterContext = scanContext
			} else {
				beforeContext = scanBeforeContext
				afterContext = scanAfterContext
			}

			type fileJob struct {
				path string
				rel  string
			}

			jobs := make(chan fileJob, 1024)
			results := make(chan interface{}, 1024)

			var wg sync.WaitGroup
			numWorkers := runtime.NumCPU()

			// Start workers
			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for job := range jobs {
						if ctx.Err() != nil {
							return
						}

						if len(multiRes) > 0 {
							content, err := os.ReadFile(job.path)
							if err != nil {
								continue
							}
							allMatch := true
							for _, mre := range multiRes {
								if !mre.Match(content) {
									allMatch = false
									break
								}
							}
							if !allMatch {
								continue
							}
						}

						file, err := os.Open(job.path)
						if err != nil {
							continue
						}

						scanner := bufio.NewScanner(file)

						var currentHeading string
						var ringBuffer []string
						if beforeContext > 0 {
							ringBuffer = make([]string, 0, beforeContext)
						}

						lineNum := 0

						type activeHunk struct {
							startL  int
							endL    int
							lines   []string
							heading string
							matched bool
						}
						var hunk *activeHunk

						fileHasMatch := false
						fileMatchCount := 0

						for scanner.Scan() {
							if ctx.Err() != nil {
								break
							}
							lineNum++
							lineText := scanner.Text()
							cleanLine := strings.ReplaceAll(lineText, "\t", "  ")

							if scanHeading {
								if headingRe.MatchString(cleanLine) || (len(cleanLine) > 0 && (cleanLine[0] != ' ' && cleanLine[0] != '/' && cleanLine[0] != '#')) {
									h := strings.TrimSpace(cleanLine)
									if len(h) > 80 {
										h = h[:77] + "..."
									}
									currentHeading = h
								}
							}

							matched := re.MatchString(cleanLine)
							if scanInvert {
								matched = !matched
							}

							if matched {
								fileHasMatch = true
								fileMatchCount++
								if scanMaxCount > 0 && fileMatchCount > scanMaxCount {
									break
								}
								if scanFilesWithoutMatch {
									break
								}
								if scanOnlyMatching {
									if scanInvert {
										// -o with -v does not make sense conceptually (ripgrep handles this by taking -v precedence or skipping)
										// Let's just output the whole line if inverted
									} else {
										matches := re.FindAllString(cleanLine, -1)
										if len(matches) > 0 {
											cleanLine = strings.Join(matches, "\n")
										}
									}
								}

								if scanFilesOnly || scanFilesWithoutMatch {
									if !fileHasMatch {
										results <- job.rel
										fileHasMatch = true
									}
									break
								}

								if hunk == nil {
									// start new hunk
									hunk = &activeHunk{
										startL:  lineNum - len(ringBuffer),
										endL:    lineNum + afterContext,
										lines:   append([]string(nil), ringBuffer...),
										heading: currentHeading,
									}
									if hunk.startL < 1 {
										hunk.startL = 1
									}
								} else {
									// extend hunk
									hunk.endL = lineNum + afterContext
								}
								hunk.matched = true
							}

							if hunk != nil {
								if lineNum <= hunk.endL {
									hunk.lines = append(hunk.lines, cleanLine)
								}

								if lineNum == hunk.endL {
									if ctx.Err() != nil {
										break
									}
									joined := strings.Join(hunk.lines, "\n")
									matchBytes := int32(len(job.rel) + len(joined) + len(hunk.heading) + 30)

									if atomic.AddInt32(&currentBytes, matchBytes) > maxBytes {
										if ctx.Err() == nil {
											results <- fmt.Errorf("budget hit")
											cancel()
										}
										break
									}

									select {
									case results <- scanMatch{
										F:       job.rel,
										L:       hunk.startL,
										EndL:    hunk.endL,
										C:       joined,
										Heading: hunk.heading,
									}:
									case <-ctx.Done():
									}
									hunk = nil
								}
							}

							if beforeContext > 0 {
								ringBuffer = append(ringBuffer, cleanLine)
								if len(ringBuffer) > beforeContext {
									ringBuffer = ringBuffer[1:]
								}
							}
						}

						// emit trailing hunk if EOF reached
						if hunk != nil && hunk.matched && !scanFilesOnly && ctx.Err() == nil {
							joined := strings.Join(hunk.lines, "\n")
							matchBytes := int32(len(job.rel) + len(joined) + len(hunk.heading) + 30)

							if atomic.AddInt32(&currentBytes, matchBytes) > maxBytes {
								if ctx.Err() == nil {
									results <- fmt.Errorf("budget hit")
									cancel()
								}
							} else {
								select {
								case results <- scanMatch{
									F:       job.rel,
									L:       hunk.startL,
									EndL:    hunk.startL + len(hunk.lines) - 1,
									C:       joined,
									Heading: hunk.heading,
								}:
								case <-ctx.Done():
								}
							}
						}

						file.Close()

						if scanFilesWithoutMatch && !fileHasMatch && ctx.Err() == nil {
							results <- job.rel
						}
					}
				}()
			}

			// Result collector
			var collectorWg sync.WaitGroup
			collectorWg.Add(1)
			go func() {
				defer collectorWg.Done()
				seenFiles := make(map[string]bool)
				for r := range results {
					switch v := r.(type) {
					case error:
						if !res.LimitHit {
							res.BudgetHit = true
						}
					case string:
						if !seenFiles[v] {
							res.Files = append(res.Files, v)
							res.Total++
							seenFiles[v] = true
							if scanLimit > 0 && res.Total >= scanLimit {
								res.LimitHit = true
								cancel()
							}
						}
					case scanMatch:
						if scanLimit > 0 && res.Total >= scanLimit {
							if !res.LimitHit {
								res.LimitHit = true
								cancel()
							}
							continue
						}
						res.Matches = append(res.Matches, v)
						res.Total++
					}
				}
			}()

			// Walk dir
			fastwalk.Walk(nil, dir, func(path string, d fs.DirEntry, err error) error {
				if ctx.Err() != nil {
					return filepath.SkipDir
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
				if ignorer != nil && ignorer.IsIgnored(path) {
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

					select {
					case jobs <- fileJob{path: path, rel: rel}:
					case <-ctx.Done():
						return filepath.SkipDir
					}
				}
				return nil
			})

			close(jobs)
			wg.Wait()
			close(results)
			collectorWg.Wait()

			if outputFormat == "json" {
				if scanGroup {
					grouped := scanGroupedResult{
						BudgetHit: res.BudgetHit,
						LimitHit:  res.LimitHit,
						Total:     res.Total,
						Grouped:   make(map[string][]scanMatch),
					}
					for _, m := range res.Matches {
						grouped.Grouped[m.F] = append(grouped.Grouped[m.F], m)
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
	cmd.Flags().IntVar(&scanBudget, "budget", 3000, "Token budget circuit breaker")
	cmd.Flags().IntVarP(&scanContext, "context", "C", 0, "Number of context lines around match")
	cmd.Flags().IntVarP(&scanBeforeContext, "before-context", "B", 0, "Number of context lines before match")
	cmd.Flags().IntVarP(&scanAfterContext, "after-context", "A", 0, "Number of context lines after match")
	cmd.Flags().StringVarP(&scanExtensions, "ext", "e", "", "Comma-separated extensions to filter (e.g. .go,.ts)")
	cmd.Flags().IntVarP(&scanLimit, "limit", "n", 0, "Max number of matches to return (0 = unlimited)")
	cmd.Flags().IntVarP(&scanMaxCount, "max-count", "m", 0, "Max number of matches per file (0 = unlimited)")
	cmd.Flags().BoolVarP(&scanIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	cmd.Flags().BoolVarP(&scanFilesOnly, "files-with-matches", "l", false, "Only print matching filenames")
	cmd.Flags().BoolVarP(&scanFilesWithoutMatch, "files-without-match", "L", false, "Only print filenames that contain no matches")
	cmd.Flags().BoolVarP(&scanHeading, "heading", "H", false, "Attempt to find nearest function/class heading above match")
	cmd.Flags().StringVarP(&scanExclude, "exclude", "x", "", "Comma-separated glob patterns to exclude (e.g. vendor/*,*_test.go)")
	cmd.Flags().BoolVarP(&scanGroup, "group", "g", false, "Group matches by file")
	cmd.Flags().BoolVarP(&scanInvert, "invert", "v", false, "Invert match (select non-matching lines)")
	cmd.Flags().StringVar(&scanMulti, "multi", "", "Additional patterns that must ALL match in the file (comma-separated, AND logic)")
	cmd.Flags().BoolVarP(&scanFixedStrings, "fixed-strings", "F", false, "Treat the pattern as a literal string instead of a regular expression")
	cmd.Flags().BoolVarP(&scanSmartCase, "smart-case", "S", false, "Search case insensitively if the pattern is all lowercase, case sensitively otherwise")
	cmd.Flags().BoolVar(&scanHidden, "hidden", false, "Search hidden files and directories")
	cmd.Flags().BoolVarP(&scanWordRegexp, "word-regexp", "w", false, "Only show matches surrounded by word boundaries")
	cmd.Flags().BoolVarP(&scanOnlyMatching, "only-matching", "O", false, "Print only the matched (non-empty) parts of a matching line")
	return cmd
}
