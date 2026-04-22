package cmd

import (
	"fmt"
	"log/slog"
	"mairu/internal/config"
	"mairu/internal/logger"
	"os"

	"github.com/spf13/cobra"
)

var (
	debugMode      bool
	outputFormat   string
	verbose        bool
	quiet          bool
	redactBashFlag bool
	appConfig      *config.Config
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
			fmt.Println("No default prompt handler available in this build.")
			os.Exit(1)
		}
		fmt.Println("Welcome to Mairu! Use 'mairu --help' to see available commands.")
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

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json, plain")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Show extra details (timing, weights, query plan)")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Only output results, no status messages")
	rootCmd.PersistentFlags().BoolVar(&redactBashFlag, "redact", false, "Redact agent bash tool output via pii-redact before the model sees it (also settable via [agent] redact_bash_output or MAIRU_REDACT_BASH)")
}
