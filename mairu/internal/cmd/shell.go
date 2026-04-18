package cmd

import (
	"fmt"
	"os"

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
			// Use os.Stdout.WriteString so `go vet`'s printf check doesn't
			// flag the bash snippet (which contains literal `%d` in its
			// embedded awk invocation) as a stray format directive.
			switch args[0] {
			case "zsh":
				_, err := os.Stdout.WriteString(zshHookSnippet)
				return err
			case "bash":
				_, err := os.Stdout.WriteString(bashHookSnippet)
				return err
			case "fish":
				_, err := os.Stdout.WriteString(fishHookSnippet)
				return err
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

const bashHookSnippet = `# mairu shell integration (bash)
# Requires bash 5.0+ (for EPOCHREALTIME). Installs a DEBUG trap to capture
# each command as it starts, and prepends a hook to PROMPT_COMMAND to send
# it to mairu ingestd via MAIRU_INGEST_SOCK (default: ~/.mairu/ingest.sock).
# Set MAIRU_NO_HOOK=1 to disable without uninstalling.

if [[ -n ${MAIRU_NO_HOOK-} ]]; then
    return 0
fi

__mairu_bash_preexec() {
    # DEBUG fires for every simple command, including those inside our own
    # PROMPT_COMMAND hook. Filter those out to avoid recording ourselves.
    case "${BASH_COMMAND}" in
        __mairu_bash_*) return ;;
    esac
    # Skip during tab completion.
    [[ -n "${COMP_LINE-}" ]] && return
    __mairu_last_cmd="${BASH_COMMAND}"
    __mairu_last_start="${EPOCHREALTIME:-0}"
    __mairu_last_cwd="${PWD}"
}

__mairu_bash_precmd() {
    local exit_code=$?
    [[ -z "${__mairu_last_cmd-}" ]] && return
    local duration_ms=0
    if [[ -n "${EPOCHREALTIME-}" && "${__mairu_last_start}" != "0" ]]; then
        duration_ms=$(awk -v s="${__mairu_last_start}" -v e="${EPOCHREALTIME}" \
            'BEGIN{printf "%d", (e - s) * 1000}')
    fi
    mairu ingest record \
        --command "${__mairu_last_cmd}" \
        --exit-code "${exit_code}" \
        --duration-ms "${duration_ms}" \
        --cwd "${__mairu_last_cwd}" >/dev/null 2>&1 &
    disown $! 2>/dev/null
    __mairu_last_cmd=""
}

trap '__mairu_bash_preexec' DEBUG
PROMPT_COMMAND="__mairu_bash_precmd;${PROMPT_COMMAND:-}"
`

const fishHookSnippet = `# mairu shell integration (fish)
# Installs fish_preexec / fish_postexec handlers that send every command to
# mairu ingestd via MAIRU_INGEST_SOCK (default: ~/.mairu/ingest.sock). Set
# MAIRU_NO_HOOK=1 to disable without uninstalling.

function __mairu_fish_preexec --on-event fish_preexec
    set -q MAIRU_NO_HOOK; and return
    set -g __mairu_last_cmd $argv
    set -g __mairu_last_cwd $PWD
end

function __mairu_fish_postexec --on-event fish_postexec
    set -q MAIRU_NO_HOOK; and return
    set -l exit_code $status
    if not set -q __mairu_last_cmd
        return
    end
    # CMD_DURATION is a fish built-in, already in milliseconds.
    mairu ingest record \
        --command "$__mairu_last_cmd" \
        --exit-code "$exit_code" \
        --duration-ms "$CMD_DURATION" \
        --cwd "$__mairu_last_cwd" >/dev/null 2>&1 &
    set -e __mairu_last_cmd
end
`
