package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for mairu.

To install completions:

  Bash:  mairu completion bash > /etc/bash_completion.d/mairu
  Zsh:   mairu completion zsh > "${fpath[1]}/_mairu"
  Fish:  mairu completion fish > ~/.config/fish/completions/mairu.fish`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			default:
				return cmd.Help()
			}
		},
	}
	return cmd
}

func init() {
}
