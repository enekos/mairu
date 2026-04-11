package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)



func NewDevCmd() *cobra.Command {
	cmd := &cobra.Command{
	Use:   "dev",
	Short: "AI-optimized development utilities",
}
	cmd.AddCommand(NewDevKillPortCmd())
	return cmd
}

func NewDevKillPortCmd() *cobra.Command {
	cmd := &cobra.Command{
	Use:   "kill-port <port>",
	Short: "Kill process running on a specific port",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		port := args[0]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var out []byte
		var err error

		if runtime.GOOS == "darwin" {
			// Find PID on mac
			lsofCmd := exec.CommandContext(ctx, "lsof", "-t", "-i", fmt.Sprintf(":%s", port))
			out, err = lsofCmd.Output()
		} else if runtime.GOOS == "linux" {
			// Find PID on linux
			fuserCmd := exec.CommandContext(ctx, "fuser", fmt.Sprintf("%s/tcp", port))
			out, err = fuserCmd.Output()
		} else {
			fmt.Fprintf(os.Stderr, "OS %s not supported for kill-port yet\n", runtime.GOOS)
			os.Exit(1)
		}

		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			fmt.Printf("No process found listening on port %s\n", port)
			return
		}

		pids := strings.Fields(strings.TrimSpace(string(out)))
		for _, pid := range pids {
			killCmd := exec.CommandContext(ctx, "kill", "-9", pid)
			if err := killCmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to kill process %s: %v\n", pid, err)
			} else {
				fmt.Printf("Killed process %s listening on port %s\n", pid, port)
			}
		}
	},
}
	return cmd
}
