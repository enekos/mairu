package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cobra"
)

func init() {
}

type infoResult struct {
	Files     int            `json:"files"`
	Tokens    int64          `json:"tokens"`
	Languages map[string]int `json:"languages"`
}

var infoCmd = &cobra.Command{
	Use:   "info [dir]",
	Short: "AI-optimized repository stats (JSON)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		var ignorer *ignore.GitIgnore
		if gi, err := ignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore")); err == nil {
			ignorer = gi
		}

		res := infoResult{
			Languages: make(map[string]int),
		}

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || path == dir {
				return nil
			}

			rel, _ := filepath.Rel(dir, path)
			if rel == ".git" {
				return filepath.SkipDir
			}
			if ignorer != nil && ignorer.MatchesPath(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !d.IsDir() {
				res.Files++
				info, err := d.Info()
				if err == nil {
					// heuristic: 1 token ~ 4 bytes
					res.Tokens += info.Size() / 4
					ext := strings.ToLower(filepath.Ext(path))
					if ext != "" {
						res.Languages[ext]++
					} else {
						res.Languages["none"]++
					}
				}
			}
			return nil
		})

		out, _ := json.Marshal(res)
		fmt.Println(string(out))
	},
}
