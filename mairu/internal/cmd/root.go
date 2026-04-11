package cmd

import (
	"fmt"
	"log/slog"
	"mairu/internal/agent"
	"mairu/internal/config"

	"mairu/internal/logger"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	debugMode    bool
	outputFormat string
	verbose      bool
	quiet        bool
	appConfig    *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "mairu [prompt]",
	Short: "Mairu - The coding agent that knows your codebase",
	Long:  `Mairu is a graph-powered AI coding agent built for performance and deep context.`,
	Args:  cobra.ArbitraryArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Setup(debugMode)

		cwd, _ := os.Getwd()
		cfg, err := config.Load(cwd)
		if err != nil {
			slog.Warn("Failed to load config, using defaults", "error", err)
			defaults := config.Config{}
			cfg = &defaults
		}
		appConfig = cfg

		// CLI flag overrides for output
		if !cmd.Flags().Changed("output") && appConfig.Output.Format != "" {
			outputFormat = appConfig.Output.Format
		}
		if !cmd.Flags().Changed("verbose") {
			verbose = appConfig.Output.Verbose
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		closeLocalService()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			prompt := strings.Join(args, " ")
			runHeadless(prompt)
			return
		}
		fmt.Println("Welcome to Mairu! Use 'mairu tui' or 'mairu web' to start.")
		cmd.Help()
	},
}

// GetConfig returns the loaded application config. Must be called after
// PersistentPreRun has executed.
func GetConfig() *config.Config {
	return appConfig
}

// GetFormatter returns a Formatter based on the resolved output format.
func GetFormatter() *Formatter {
	return NewFormatter(outputFormat)
}

// GetAgentConfig returns a populated agent.Config using the global config and contextsrv.App
func GetAgentConfig() agent.Config {
	cfg := GetConfig()
	var interceptors []agent.ToolInterceptor
	if cfg != nil {
		interceptors = append(interceptors, &agent.SecurityFilter{
			BlockedCommands: cfg.Security.BlockedCommands,
			BlockedPaths:    cfg.Security.BlockedPaths,
		})
	}

	app := GetLocalApp()
	var repo agent.HistoryLogger
	if app != nil {
		repo = app.Repo()
	}

	return agent.Config{
		SymbolLocator: GetLocalApp().SymbolLocator(),
		HistoryLogger: repo,
		Interceptors:  interceptors,
		UTCPProviders: cfg.Tools.UTCPProviders,
	}
}

func runHeadless(prompt string) {
	apiKey := GetAPIKey()
	if apiKey == "" {
		slog.Error("Gemini API key not found. Please run 'mairu setup' or set GEMINI_API_KEY environment variable.")
		os.Exit(1)
	}

	cwd, _ := os.Getwd()
	a, err := agent.New(cwd, apiKey, GetAgentConfig())
	if err != nil {
		slog.Error("Failed to initialize agent", "error", err)
		os.Exit(1)
	}
	defer a.Close()

	response, err := a.Run(prompt)
	if err != nil {
		slog.Error("Agent error", "error", err)
		os.Exit(1)
	}

	fmt.Println("\n" + response)
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, plain")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Show extra details (timing, weights, query plan)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Only output results, no status messages")
}

func init() {
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
		NewVibeQueryAliasCmd(),
		NewVibeMutationAliasCmd(),
		NewScrapeCmd(),
		NewAnalyzeCmd(),
		NewIngestCmd(),
		NewSummarizeCmd(),
		NewFlushCmd(),
		NewNudgeCmd(),
		NewImpactCmd(),
		NewGithubCmd(),
		NewLinearCmd(),
	)

	// Agent & Servers
	rootCmd.AddCommand(
		NewMinionCmd(),
		NewDaemonCmd(),
		NewContextServerCmd(),
		NewWebCmd(),
		NewTuiCmd(),
		NewTelegramCmd(),
		NewMCPCmd(),
		NewUTCPCmd(),
	)

	// Core / Admin / Misc
	rootCmd.AddCommand(
		NewInitCmd(),
		NewConfigCmd(),
		NewCompletionCmd(),
		NewDoctorCmd(),
		NewSetupCmd(),
		NewSeedCmd(),
		NewEvalCmd(),
	)
}
