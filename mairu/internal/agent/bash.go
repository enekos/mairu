package agent

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

var ansiPattern = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func StripANSI(str string) string {
	return ansiPattern.ReplaceAllString(str, "")
}

// RunBash executes a shell command with a timeout and returns its output.
func (a *Agent) RunBash(command string, timeoutMs int, outChan chan<- AgentEvent) (string, error) {
	if timeoutMs <= 0 {
		timeoutMs = 30000 // default 30s
	}

	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = a.db.Root() // Run in the project root
	// Make it more resilient by faking CI
	cmd.Env = append(cmd.Environ(), "CI=true", "DEBIAN_FRONTEND=noninteractive", "NONINTERACTIVE=true", "FORCE_COLOR=0")

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var mu sync.Mutex // protect buffers

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
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
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		if err := cmd.Process.Kill(); err != nil {
			return "", fmt.Errorf("command timed out and failed to kill: %w", err)
		}
		return "", fmt.Errorf("command timed out after %dms", timeoutMs)
	case err := <-done:
		mu.Lock()
		outStr := StripANSI(stdoutBuf.String())
		errStr := StripANSI(stderrBuf.String())
		mu.Unlock()

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

		// Truncate if output is too long (max 10000 chars, tail truncation)
		if len(result) > 10000 {
			result = result[:10000] + "\n...[Output truncated, run command redirecting to file to see full output]"
		}

		return result, nil
	}
}
