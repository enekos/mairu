package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cobra"
)

var mapDepth int

func init() {
	mapCmd.Flags().IntVarP(&mapDepth, "depth", "d", 0, "Max depth to map (0 = unlimited)")
}

type mapEntry struct {
	P string `json:"p"`
	T int64  `json:"t"`
}

var mapCmd = &cobra.Command{
	Use:   "map [dir]",
	Short: "AI-optimized directory tree (JSON token-aware)",
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

		var entries []mapEntry
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if path == dir {
				return nil
			}

			rel, _ := filepath.Rel(dir, path)

			// Depth check
			if mapDepth > 0 {
				depth := len(strings.Split(rel, string(os.PathSeparator)))
				if depth > mapDepth {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

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
				info, err := d.Info()
				if err == nil {
					// heuristic: 1 token ~ 4 bytes
					tokens := info.Size() / 4
					if tokens == 0 && info.Size() > 0 {
						tokens = 1
					}
					entries = append(entries, mapEntry{P: rel, T: tokens})
				}
			}
			return nil
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		out, _ := json.Marshal(entries)
		fmt.Println(string(out))
	},
}

type sysEntry struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	NumCPU int    `json:"num_cpu"`
	MemMB  uint64 `json:"mem_mb"`
	// Ports could be implemented later if needed
}

var sysCmd = &cobra.Command{
	Use:   "sys",
	Short: "AI-optimized system health snapshot (JSON)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		info := sysEntry{
			OS:     runtime.GOOS,
			Arch:   runtime.GOARCH,
			NumCPU: runtime.NumCPU(),
			MemMB:  m.Sys / 1024 / 1024,
		}

		out, _ := json.Marshal(info)
		fmt.Println(string(out))
	},
}
