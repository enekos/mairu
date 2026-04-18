package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewShellCmd returns the `mairu shell` top-level command, which currently
// only exposes `init <shell>` — a subcommand that prints a shell-specific
// hook snippet users are expected to `eval` from their rc file.
func NewShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Shell integration snippets (hook installation)",
	}
	cmd.AddCommand(newShellInitCmd())
	return cmd
}

func newShellInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <zsh|bash|fish>",
		Short: "Print a hook snippet to be evaluated in your shell rc file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "zsh":
				fmt.Print(zshHookSnippet)
				return nil
			case "bash", "fish":
				return fmt.Errorf("shell %q: not implemented yet", args[0])
			default:
				return fmt.Errorf("unknown shell %q (want zsh, bash, or fish)", args[0])
			}
		},
	}
}

const zshHookSnippet = `# mairu shell integration (zsh)
# Installs preexec/precmd hooks that send every command to mairu ingestd via
# MAIRU_INGEST_SOCK (default: ~/.mairu/ingest.sock). Set MAIRU_NO_HOOK=1 to
# disable without uninstalling.

if [[ -n ${MAIRU_NO_HOOK-} ]]; then
    return 0
fi

autoload -Uz add-zsh-hook
zmodload zsh/datetime 2>/dev/null

__mairu_preexec() {
    __mairu_last_cmd=$1
    __mairu_last_start=$EPOCHREALTIME
    __mairu_last_cwd=$PWD
}

__mairu_precmd() {
    local exit_code=$?
    [[ -z ${__mairu_last_cmd-} ]] && return
    local duration=$(( (EPOCHREALTIME - __mairu_last_start) * 1000 ))
    mairu ingest record \
        --command "$__mairu_last_cmd" \
        --exit-code "$exit_code" \
        --duration-ms "${duration%.*}" \
        --cwd "$__mairu_last_cwd" >/dev/null 2>&1 &!
    unset __mairu_last_cmd
}

add-zsh-hook preexec __mairu_preexec
add-zsh-hook precmd __mairu_precmd
`
