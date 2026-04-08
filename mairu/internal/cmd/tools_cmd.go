package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	ignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cobra"
)

var mapDepth int
var mapExtensions string
var mapMin int64
var mapSort string
var mapDirs bool

func init() {
	mapCmd.Flags().IntVarP(&mapDepth, "depth", "d", 0, "Max depth to map (0 = unlimited)")
	mapCmd.Flags().StringVarP(&mapExtensions, "ext", "e", "", "Comma-separated extensions to filter (e.g. .go,.ts)")
	mapCmd.Flags().Int64Var(&mapMin, "min", 0, "Only show files with >= N tokens")
	mapCmd.Flags().StringVar(&mapSort, "sort", "", "Sort order: 'size' for descending token count (default: path order)")
	mapCmd.Flags().BoolVar(&mapDirs, "dirs", false, "Include directory entries with aggregated token counts")
}

type mapEntry struct {
	P   string `json:"p"`
	T   int64  `json:"t"`
	Dir bool   `json:"d,omitempty"`
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

		allowedExts := make(map[string]bool)
		if mapExtensions != "" {
			for _, ext := range strings.Split(mapExtensions, ",") {
				ext = strings.TrimSpace(ext)
				if !strings.HasPrefix(ext, ".") {
					ext = "." + ext
				}
				allowedExts[strings.ToLower(ext)] = true
			}
		}

		var entries []mapEntry
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || path == dir {
				return nil
			}

			rel, _ := filepath.Rel(dir, path)

			if mapDepth > 0 {
				depth := len(strings.Split(rel, string(os.PathSeparator)))
				if depth > mapDepth {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
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
				if mapMin > 0 && tokens < mapMin {
					return nil
				}
				entries = append(entries, mapEntry{P: rel, T: tokens})
			}
			return nil
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		// Directory aggregates
		if mapDirs {
			dirTotals := make(map[string]int64)
			for _, e := range entries {
				d := filepath.Dir(e.P)
				for d != "." && d != "" {
					dirTotals[d] += e.T
					d = filepath.Dir(d)
				}
			}
			for d, t := range dirTotals {
				entries = append(entries, mapEntry{P: d, T: t, Dir: true})
			}
		}

		// Sort
		if mapSort == "size" {
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].T > entries[j].T
			})
		}

		out, _ := json.Marshal(entries)
		fmt.Println(string(out))
	},
}

type sysEntry struct {
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
	NumCPU      int     `json:"num_cpu"`
	MemMB       uint64  `json:"mem_mb"`
	DiskFreeGB  float64 `json:"disk_free_gb"`
	DiskTotalGB float64 `json:"disk_total_gb"`
	GoVersion   string  `json:"go_version"`
	Docker      bool    `json:"docker"`
}

var sysCmd = &cobra.Command{
	Use:   "sys",
	Short: "AI-optimized system health snapshot (JSON)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		info := sysEntry{
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			NumCPU:    runtime.NumCPU(),
			MemMB:     m.Sys / 1024 / 1024,
			GoVersion: runtime.Version(),
		}

		// Disk usage
		var stat syscall.Statfs_t
		cwd, _ := os.Getwd()
		if err := syscall.Statfs(cwd, &stat); err == nil {
			info.DiskFreeGB = math.Round(float64(stat.Bavail)*float64(stat.Bsize)/1e9*10) / 10
			info.DiskTotalGB = math.Round(float64(stat.Blocks)*float64(stat.Bsize)/1e9*10) / 10
		}

		// Docker status
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "docker", "info").Run(); err == nil {
			info.Docker = true
		}

		out, _ := json.Marshal(info)
		fmt.Println(string(out))
	},
}
