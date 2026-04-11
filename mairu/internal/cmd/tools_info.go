package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"mairu/internal/fsutil"
)

var infoTop int
var infoExtensions string

type langStat struct {
	Files  int     `json:"files"`
	Tokens int64   `json:"tokens"`
	Lines  int     `json:"lines"`
	Pct    float64 `json:"pct"`
}

type topFile struct {
	P string `json:"p"`
	T int64  `json:"t"`
	L int    `json:"l"`
}

type infoResult struct {
	Files     int                 `json:"files"`
	Tokens    int64               `json:"tokens"`
	Lines     int                 `json:"lines"`
	Languages map[string]langStat `json:"languages"`
	Top       []topFile           `json:"top,omitempty"`
}

var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
	".exe": true, ".bin": true, ".pdf": true, ".mp4": true, ".mp3": true,
	".zip": true, ".tar": true, ".gz": true, ".woff": true, ".woff2": true,
	".ttf": true, ".eot": true, ".so": true, ".dylib": true, ".dll": true,
}

func NewInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [dir]",
		Short: "AI-optimized repository stats (JSON)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			ignorer := fsutil.NewProjectIgnorer(dir)

			allowedExts := make(map[string]bool)
			if infoExtensions != "" {
				for _, ext := range strings.Split(infoExtensions, ",") {
					ext = strings.TrimSpace(ext)
					if !strings.HasPrefix(ext, ".") {
						ext = "." + ext
					}
					allowedExts[strings.ToLower(ext)] = true
				}
			}

			res := infoResult{
				Languages: make(map[string]langStat),
			}
			var allFiles []topFile

			filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || path == dir {
					return nil
				}

				rel, _ := filepath.Rel(dir, path)
				if rel == ".git" || rel == "node_modules" {
					return filepath.SkipDir
				}
				if ignorer != nil && ignorer.IsIgnored(path) {
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

					info, err := d.Info()
					if err != nil {
						return nil
					}

					tokens := info.Size() / 4
					if tokens == 0 && info.Size() > 0 {
						tokens = 1
					}

					lines := 0
					if !binaryExts[ext] {
						if content, err := os.ReadFile(path); err == nil {
							lines = strings.Count(string(content), "\n")
							if len(content) > 0 && content[len(content)-1] != '\n' {
								lines++ // count last line without trailing newline
							}
						}
					}

					res.Files++
					res.Tokens += tokens
					res.Lines += lines

					langKey := ext
					if langKey == "" {
						langKey = "none"
					}
					ls := res.Languages[langKey]
					ls.Files++
					ls.Tokens += tokens
					ls.Lines += lines
					res.Languages[langKey] = ls

					if infoTop > 0 {
						allFiles = append(allFiles, topFile{P: rel, T: tokens, L: lines})
					}
				}
				return nil
			})

			// Calculate percentages
			for k, ls := range res.Languages {
				if res.Tokens > 0 {
					ls.Pct = math.Round(float64(ls.Tokens)/float64(res.Tokens)*100) / 100
				}
				res.Languages[k] = ls
			}

			// Top N
			if infoTop > 0 && len(allFiles) > 0 {
				sort.Slice(allFiles, func(i, j int) bool {
					return allFiles[i].T > allFiles[j].T
				})
				n := infoTop
				if n > len(allFiles) {
					n = len(allFiles)
				}
				res.Top = allFiles[:n]
			}

			if outputFormat == "json" {
				out, _ := json.Marshal(res)
				fmt.Println(string(out))
			} else {
				f := GetFormatter()
				f.PrintTable(
					[]string{"total_files", "total_tokens", "total_lines"},
					[]map[string]string{{
						"total_files":  fmt.Sprintf("%d", res.Files),
						"total_tokens": fmt.Sprintf("%d", res.Tokens),
						"total_lines":  fmt.Sprintf("%d", res.Lines),
					}},
				)

				if len(res.Languages) > 0 {
					fmt.Println("\nBy Extension:")
					var exts []string
					for k := range res.Languages {
						exts = append(exts, k)
					}
					sort.Slice(exts, func(i, j int) bool { return res.Languages[exts[i]].Tokens > res.Languages[exts[j]].Tokens })

					extItems := make([]map[string]any, len(exts))
					for i, ext := range exts {
						st := res.Languages[ext]
						extItems[i] = map[string]any{
							"ext":    ext,
							"files":  st.Files,
							"tokens": st.Tokens,
							"lines":  st.Lines,
							"pct":    st.Pct,
						}
					}
					f.PrintItems(
						[]string{"ext", "files", "tokens", "lines", "pct"},
						extItems,
						func(item map[string]any) map[string]string {
							return map[string]string{
								"ext":    fmt.Sprintf("%v", item["ext"]),
								"files":  fmt.Sprintf("%v", item["files"]),
								"tokens": fmt.Sprintf("%v", item["tokens"]),
								"lines":  fmt.Sprintf("%v", item["lines"]),
								"pct":    fmt.Sprintf("%.1f%%", item["pct"].(float64)*100),
							}
						},
					)
				}

				if len(res.Top) > 0 {
					fmt.Println("\nTop Files:")
					topItems := make([]map[string]any, len(res.Top))
					for i, t := range res.Top {
						topItems[i] = map[string]any{
							"path":   t.P,
							"tokens": t.T,
							"lines":  t.L,
						}
					}
					f.PrintItems(
						[]string{"path", "tokens", "lines"},
						topItems,
						func(item map[string]any) map[string]string {
							return map[string]string{
								"path":   fmt.Sprintf("%v", item["path"]),
								"tokens": fmt.Sprintf("%v", item["tokens"]),
								"lines":  fmt.Sprintf("%v", item["lines"]),
							}
						},
					)
				}
			}
		},
	}
	cmd.Flags().IntVar(&infoTop, "top", 0, "Show top N largest files by token count")
	cmd.Flags().StringVarP(&infoExtensions, "ext", "e", "", "Comma-separated extensions to filter (e.g. .go,.ts)")
	return cmd
}
