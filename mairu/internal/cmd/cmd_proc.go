package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	procCmd.AddCommand(procPortsCmd)
	procCmd.AddCommand(procTopCmd)
}

var procCmd = &cobra.Command{
	Use:   "proc",
	Short: "AI-optimized process and port helpers",
}

var procPortsCmd = &cobra.Command{
	Use:   "ports",
	Short: "List active listening ports and their processes",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var out []byte
		var err error

		if runtime.GOOS == "darwin" {
			out, err = exec.CommandContext(ctx, "lsof", "-iTCP", "-sTCP:LISTEN", "-n", "-P").Output()
		} else if runtime.GOOS == "linux" {
			out, err = exec.CommandContext(ctx, "ss", "-ltnp").Output()
		} else {
			fmt.Fprintf(os.Stderr, "OS %s not supported for proc ports yet\n", runtime.GOOS)
			os.Exit(1)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting ports: %v\n", err)
			os.Exit(1)
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var results []map[string]any

		if runtime.GOOS == "darwin" {
			// lsof output parsing
			for i, line := range lines {
				if i == 0 {
					continue // skip header
				}
				parts := strings.Fields(line)
				if len(parts) >= 9 {
					results = append(results, map[string]any{
						"command": parts[0],
						"pid":     parts[1],
						"user":    parts[2],
						"address": parts[8],
					})
				}
			}
		} else if runtime.GOOS == "linux" {
			// ss output parsing
			for i, line := range lines {
				if i == 0 {
					continue // skip header
				}
				parts := strings.Fields(line)
				if len(parts) >= 6 {
					address := parts[3]
					process := ""
					if len(parts) > 6 {
						process = strings.Join(parts[6:], " ")
					}
					results = append(results, map[string]any{
						"address": address,
						"process": process,
					})
				}
			}
		}

		if outputFormat == "json" {
			j, _ := json.Marshal(results)
			fmt.Println(string(j))
		} else {
			f := GetFormatter()
			f.PrintItems(
				[]string{"command", "pid", "address"},
				results,
				func(item map[string]any) map[string]string {
					cmd := fmt.Sprintf("%v", item["command"])
					if cmd == "<nil>" {
						cmd = fmt.Sprintf("%v", item["process"])
					}
					return map[string]string{
						"command": cmd,
						"pid":     fmt.Sprintf("%v", item["pid"]),
						"address": fmt.Sprintf("%v", item["address"]),
					}
				},
			)
		}
	},
}

var procTopCmd = &cobra.Command{
	Use:   "top",
	Short: "Token-budgeted list of highest CPU/Memory processes",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var out []byte
		var err error

		// using ps to get cross platform top processes
		out, err = exec.CommandContext(ctx, "ps", "aux").Output()

		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting processes: %v\n", err)
			os.Exit(1)
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var results []map[string]any

		for i, line := range lines {
			if i == 0 {
				continue // skip header
			}
			parts := strings.Fields(line)
			if len(parts) >= 11 {
				cmdStr := strings.Join(parts[10:], " ")
				if len(cmdStr) > 50 {
					cmdStr = cmdStr[:47] + "..."
				}
				results = append(results, map[string]any{
					"user":    parts[0],
					"pid":     parts[1],
					"cpu":     parts[2],
					"mem":     parts[3],
					"command": cmdStr,
				})
			}
			// Limit to top 10 to keep it AI friendly
			if len(results) >= 10 {
				break
			}
		}

		if outputFormat == "json" {
			j, _ := json.Marshal(results)
			fmt.Println(string(j))
		} else {
			f := GetFormatter()
			f.PrintItems(
				[]string{"pid", "cpu", "mem", "command"},
				results,
				func(item map[string]any) map[string]string {
					return map[string]string{
						"pid":     fmt.Sprintf("%v", item["pid"]),
						"cpu":     fmt.Sprintf("%v", item["cpu"]) + "%",
						"mem":     fmt.Sprintf("%v", item["mem"]) + "%",
						"command": fmt.Sprintf("%v", item["command"]),
					}
				},
			)
		}
	},
}
