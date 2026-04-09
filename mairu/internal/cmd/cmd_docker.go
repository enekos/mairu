package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var dockerLogsLines int
var dockerStatsStream bool

func init() {
	dockerLogsCmd.Flags().IntVarP(&dockerLogsLines, "lines", "n", 50, "Number of lines to tail")
	dockerStatsCmd.Flags().BoolVarP(&dockerStatsStream, "stream", "s", false, "Stream stats (false by default for AI optimization)")

	dockerCmd.AddCommand(dockerPsCmd)
	dockerCmd.AddCommand(dockerLogsCmd)
	dockerCmd.AddCommand(dockerStatsCmd)
}

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "AI-optimized Docker helpers",
	Long:  "Docker utilities designed to provide token-efficient, highly relevant context to AI agents and developers.",
}

var dockerPsCmd = &cobra.Command{
	Use:   "ps",
	Short: "Clean, token-friendly list of running containers",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		format := `{"id":"{{.ID}}","name":"{{.Names}}","image":"{{.Image}}","state":"{{.State}}","status":"{{.Status}}","ports":"{{.Ports}}"}`
		out, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", format).Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error running docker ps: %v\n", err)
			os.Exit(1)
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var containers []map[string]any

		for _, line := range lines {
			if line == "" {
				continue
			}
			var c map[string]any
			if err := json.Unmarshal([]byte(line), &c); err == nil {
				containers = append(containers, c)
			}
		}

		if outputFormat == "json" {
			j, _ := json.Marshal(containers)
			fmt.Println(string(j))
		} else {
			f := GetFormatter()
			f.PrintItems(
				[]string{"name", "image", "state", "ports"},
				containers,
				func(item map[string]any) map[string]string {
					return map[string]string{
						"name":  fmt.Sprintf("%v", item["name"]),
						"image": fmt.Sprintf("%v", item["image"]),
						"state": fmt.Sprintf("%v", item["state"]),
						"ports": fmt.Sprintf("%v", item["ports"]),
					}
				},
			)
		}
	},
}

var dockerLogsCmd = &cobra.Command{
	Use:   "logs <container>",
	Short: "Token-budgeted container logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		container := args[0]
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", dockerLogsLines), container).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting logs for %s: %v\n", container, err)
			os.Exit(1)
		}

		fmt.Println(string(out))
	},
}

var dockerStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Token-friendly snapshot of container resource usage",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		dockerArgs := []string{"stats", "--format", `{"container":"{{.Container}}","name":"{{.Name}}","cpu":"{{.CPUPerc}}","mem":"{{.MemUsage}}","mem_perc":"{{.MemPerc}}","net":"{{.NetIO}}"}`}
		if !dockerStatsStream {
			dockerArgs = append(dockerArgs, "--no-stream")
		}

		out, err := exec.CommandContext(ctx, "docker", dockerArgs...).Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting docker stats: %v\n", err)
			os.Exit(1)
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var stats []map[string]any

		for _, line := range lines {
			if line == "" {
				continue
			}
			var s map[string]any
			if err := json.Unmarshal([]byte(line), &s); err == nil {
				stats = append(stats, s)
			}
		}

		if outputFormat == "json" {
			j, _ := json.Marshal(stats)
			fmt.Println(string(j))
		} else {
			f := GetFormatter()
			f.PrintItems(
				[]string{"name", "cpu", "mem", "net"},
				stats,
				func(item map[string]any) map[string]string {
					return map[string]string{
						"name": fmt.Sprintf("%v", item["name"]),
						"cpu":  fmt.Sprintf("%v", item["cpu"]),
						"mem":  fmt.Sprintf("%v", item["mem"]),
						"net":  fmt.Sprintf("%v", item["net"]),
					}
				},
			)
		}
	},
}
