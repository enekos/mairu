//go:build slim

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"mairu/internal/cmd/admincmd"
)

func init() {
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			prompt := strings.Join(args, " ")
			runHeadless(prompt)
			return
		}
		fmt.Println("Welcome to Mairu! Use 'mairu --help' to see available commands.")
		cmd.Help()
	}

	// AI-Optimized Tools (Keep at top level)
	rootCmd.AddCommand(
		NewMapCmd(),
		NewSysCmd(),
		NewEnvCmd(),
		NewInfoCmd(),
		NewOutlineCmd(),
		NewPeekCmd(),
		NewScanCmd(),
		NewDistillCmd(),
		NewSpliceCmd(),
		NewDockerCmd(),
		NewProcCmd(),
		NewDevCmd(),
		NewGitCmd(),
	)

	// Subsystems & Workflows
	rootCmd.AddCommand(
		NewMemoryCmd(),
		NewSkillCmd(),
		NewNodeCmd(),
		NewCodeCmd(),
		NewHistoryCmd(),
		NewSyncCmd(),
		NewVibeCmd(),
		NewScrapeCmd(),
		NewAnalyzeCmd(),
		NewIngestCmd(),
		NewIngestdCmd(),
		NewCaptureCmd(),
		NewShellCmd(),
		NewImpactCmd(),
		NewGithubCmd(),
		NewLinearCmd(),
	)

	// Agent & Servers (no TUI, Web, Telegram, MCP)
	rootCmd.AddCommand(
		NewMinionCmd(),
		NewDaemonCmd(),
		NewContextServerCmd(),
		NewUTCPCmd(),
	)

	// Core / Admin / Misc
	rootCmd.AddCommand(
		admincmd.NewInitCmd(),
		NewConfigCmd(),
		admincmd.NewCompletionCmd(rootCmd),
		NewDoctorCmd(),
		NewSetupCmd(),
		NewEvalCmd(),
		NewEvalLLMCmd(),
		NewTraceCmd(),
	)
}
