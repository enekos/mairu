package agent

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/enekos/mairu/pii-redact/pkg/redact"
	"mairu/internal/llm"
)

var ansiPattern = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func StripANSI(str string) string {
	return ansiPattern.ReplaceAllString(str, "")
}

func IsDangerousCommand(command string) bool {
	lower := strings.ToLower(command)
	dangerousPrefixes := []string{
		"rm -rf", "rm -r ", "rm ",
		"mv ", "cp ", "wget", "curl", "chmod", "chown", "sudo",
		"docker run", "docker exec", "docker rm", "docker rmi", "docker stop",
		"kill", "pkill",
	}

	// Also check after pipes/ands
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '|' || r == '&' || r == ';'
	})

	for _, p := range parts {
		cmdPart := strings.TrimSpace(p)
		for _, d := range dangerousPrefixes {
			if strings.HasPrefix(cmdPart, d) {
				return true
			}
		}
	}

	return false
}

// RunBash executes a shell command with a timeout and returns its output.
func (a *Agent) RunBash(ctx context.Context, command string, timeoutMs int, outChan chan<- AgentEvent) (string, error) {
	if timeoutMs <= 0 {
		timeoutMs = 30000 // default 30s
	}

	const maxAttempts = 2
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, waitErr, runErr := a.runBashOnce(ctx, command, timeoutMs, outChan)
		if runErr != nil {
			return "", runErr
		}
		if isHangupExit(waitErr, out) && attempt < maxAttempts {
			if outChan != nil {
				outChan <- AgentEvent{Type: "status", Content: "⚠️ Command exited with hangup signal, retrying once..."}
			}
			continue
		}
		return out, nil
	}
	return "", fmt.Errorf("command failed unexpectedly after retries")
}

func (a *Agent) runBashOnce(ctx context.Context, command string, timeoutMs int, outChan chan<- AgentEvent) (string, error, error) {
	// Wrap the command to echo the final PWD at the end, but preserve the exit code
	wrappedCommand := fmt.Sprintf("%s\n__mairu_ret=$?\n__mairu_pwd=$(pwd)\necho \"__MAIRU_PWD_MARKER__${__mairu_pwd}\"\nexit $__mairu_ret", command)

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", wrappedCommand)
	cmd.Dir = a.currentDir // Run in the project root
	// Make it more resilient by faking CI
	cmd.Env = append(cmd.Environ(), "CI=true", "DEBIAN_FRONTEND=noninteractive", "NONINTERACTIVE=true", "FORCE_COLOR=0")
	if runtime.GOOS != "windows" {
		// Isolate child in a new process group so timeout cleanup can kill descendants.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var mu sync.Mutex // protect buffers

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Stream readers
	var wg sync.WaitGroup
	wg.Add(2)

	streamOutput := func(r io.Reader, buf *bytes.Buffer, isErr bool) {
		defer wg.Done()
		reader := bufio.NewReader(r)
		chunk := make([]byte, 1024)
		for {
			n, err := reader.Read(chunk)
			if n > 0 {
				mu.Lock()
				buf.Write(chunk[:n])
				mu.Unlock()

				if outChan != nil {
					cleanChunk := StripANSI(string(chunk[:n]))
					// Emit the streamed chunk. The TUI/WebUI can append this to the bash output view
					outChan <- AgentEvent{Type: "bash_output", Content: cleanChunk}
				}
			}
			if err != nil {
				break
			}
		}
	}

	go streamOutput(stdoutPipe, &stdoutBuf, false)
	go streamOutput(stderrPipe, &stderrBuf, true)

	// Wait with timeout
	done := make(chan error, 1)
	go func() {
		wg.Wait()
		done <- cmd.Wait()
	}()

	select {
	case <-cmdCtx.Done():
		if cmd.Process != nil {
			if runtime.GOOS != "windows" {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			if err := cmd.Process.Kill(); err != nil {
				return "", nil, fmt.Errorf("command cancelled and failed to kill: %w", err)
			}
		}
		if ctx.Err() != nil {
			return "", nil, fmt.Errorf("command interrupted by user")
		}
		return "", nil, fmt.Errorf("command timed out after %dms", timeoutMs)
	case err := <-done:
		mu.Lock()
		outStr := StripANSI(stdoutBuf.String())
		errStr := StripANSI(stderrBuf.String())
		mu.Unlock()

		// Parse the PWD marker from outStr specifically
		pwdMarker := "__MAIRU_PWD_MARKER__"
		if idx := strings.LastIndex(outStr, pwdMarker); idx != -1 {
			lines := strings.Split(outStr[idx+len(pwdMarker):], "\n")
			if len(lines) > 0 {
				newDir := strings.TrimSpace(lines[0])
				if newDir != "" {
					a.mu.Lock()
					a.currentDir = newDir
					a.mu.Unlock()
				}
			}
			// Strip the marker from the stdout
			outStr = strings.TrimSpace(outStr[:idx])
		}

		result := ""
		if outStr != "" {
			result += "STDOUT:\n" + outStr
		}
		if errStr != "" {
			result += "\nSTDERR:\n" + errStr
		}

		if err != nil {
			result += fmt.Sprintf("\nExited with error: %v", err)
		}

		// Dual-axis tail truncation: bash output's signal is at the end.
		if tr := TruncateTail(result, DefaultMaxLines, DefaultMaxBytes); tr.Truncated {
			result = tr.Content + FormatTruncationNote(tr, "tail")
		}

		return strings.TrimSpace(result), err, nil
	}
}

func isHangupExit(waitErr error, output string) bool {
	if waitErr == nil {
		return false
	}
	if ee, ok := waitErr.(*exec.ExitError); ok {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() && ws.Signal() == syscall.SIGHUP {
			return true
		}
	}
	lower := strings.ToLower(output)
	return strings.Contains(lower, "hangup") || strings.Contains(lower, "sighup")
}

type bashTool struct{}

func (t *bashTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "bash",
		Description: "Execute a bash command in the project root directory. Use this to run tests, linters, or explore the file system.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"command":    {Type: llm.TypeString, Description: "The bash command to execute."},
				"timeout_ms": {Type: llm.TypeInteger, Description: "Optional timeout in milliseconds (default 30000)."},
			},
			Required: []string{"command"},
		},
	}
}

func (t *bashTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	command, _ := args["command"].(string)
	timeoutMsFloat, ok := args["timeout_ms"].(float64)
	var timeout int
	if ok {
		timeout = int(timeoutMsFloat)
	}

	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🖥️ Running bash: %s", command)}

	start := time.Now()
	rawOut, err := a.RunBash(ctx, command, timeout, outChan)
	duration := int(time.Since(start).Milliseconds())

	// Emit diff events on the RAW output — the TUI already saw this live.
	if err == nil {
		if strings.HasPrefix(rawOut, "STDOUT:\ndiff ") || strings.Contains(rawOut, "\n--- ") && strings.Contains(rawOut, "\n+++ ") {
			outChan <- AgentEvent{Type: "diff", Content: fmt.Sprintf("```diff\n%s\n```", rawOut)}
		}
	}

	// Redact the output handed back to the model (opt-in via agent config).
	// History logging uses the raw output because the ingest/history pipeline
	// has its own redactor and dedup layer.
	modelOut := rawOut
	if a.redactor != nil && rawOut != "" {
		res := a.redactor.Redact(rawOut, redact.KindText)
		if res.Dropped {
			slog.Debug("agent/bash: redaction damage-cap tripped",
				"findings", len(res.Findings))
			modelOut = fmt.Sprintf("[REDACTED:damage_cap] (bash output had %d redaction findings, body withheld)", len(res.Findings))
		} else {
			modelOut = res.Redacted
		}
	}

	exitCode := 0
	var result map[string]any
	if err != nil {
		result = map[string]any{"error": err.Error(), "output": modelOut}
		exitCode = 1
	} else {
		result = map[string]any{"output": modelOut}
	}

	if a.historyLogger != nil {
		go func() {
			_ = a.historyLogger.InsertBashHistory(context.Background(), a.root, command, exitCode, duration, rawOut)
		}()
	}

	return result, nil
}
