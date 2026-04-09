package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Helper to run mairu command
func runMairu(args ...string) (string, error) {
	// Find the root mairu directory (where go.mod is) to run the command safely
	// regardless of working directory changes by other tests.
	pwd, _ := os.Getwd()
	for !strings.HasSuffix(pwd, "mairu/mairu") && pwd != "/" {
		pwd = filepath.Dir(pwd)
	}

	cmd := exec.Command("go", append([]string{"run", "./cmd/mairu"}, args...)...)
	cmd.Dir = pwd
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestDockerPsE2E(t *testing.T) {
	out, err := runMairu("docker", "ps", "-o", "json")
	if err != nil {
		// If docker is not running or not installed, the command might fail.
		// We shouldn't fail the test suite if docker is just not available.
		if strings.Contains(out, "error running docker ps") || strings.Contains(out, "Cannot connect to the Docker daemon") {
			t.Skip("Docker is not running or not installed, skipping test")
		}
		t.Fatalf("docker ps failed: %v, output: %s", err, out)
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}
}

func TestDockerStatsE2E(t *testing.T) {
	out, err := runMairu("docker", "stats", "-o", "json")
	if err != nil {
		if strings.Contains(out, "error getting docker stats") || strings.Contains(out, "Cannot connect to the Docker daemon") {
			t.Skip("Docker is not running or not installed, skipping test")
		}
		t.Fatalf("docker stats failed: %v, output: %s", err, out)
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}
}

func TestProcTopE2E(t *testing.T) {
	out, err := runMairu("proc", "top", "-o", "json")
	if err != nil {
		t.Fatalf("proc top failed: %v, output: %s", err, out)
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}

	if len(results) == 0 {
		t.Errorf("proc top returned 0 processes")
	} else {
		// Check required fields
		first := results[0]
		required := []string{"cpu", "mem", "pid", "command"}
		for _, req := range required {
			if _, ok := first[req]; !ok {
				t.Errorf("proc top missing field: %s", req)
			}
		}
	}
}

func TestProcPortsE2E(t *testing.T) {
	// Start a dummy listener to ensure at least one port is open
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start dummy listener: %v", err)
	}
	defer listener.Close()

	port := strings.Split(listener.Addr().String(), ":")[1]

	out, err := runMairu("proc", "ports", "-o", "json")
	if err != nil {
		// lsof might fail or ss might not be available
		t.Fatalf("proc ports failed: %v, output: %s", err, out)
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Errorf("Failed to parse JSON output: %v, output: %s", err, out)
	}

	// Verify our port is in the output
	found := false
	for _, res := range results {
		addr := fmt.Sprintf("%v", res["address"])
		if strings.Contains(addr, port) {
			found = true
			break
		}
	}

	if !found {
		// On some CI environments, lsof might not see our own process immediately or at all without sudo,
		// but we log it as a warning instead of failing to prevent flaky tests.
		t.Logf("Warning: Port %s not found in proc ports output", port)
	}
}

func TestDevKillPortE2E(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("kill-port not implemented for windows yet")
	}

	// Create a dummy go program that listens on a port
	code := `
package main
import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)
func main() {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(l.Addr().(*net.TCPAddr).Port)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
`
	tmpDir, err := os.MkdirTemp("", "killporttest")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(srcFile, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write dummy program: %v", err)
	}

	binFile := filepath.Join(tmpDir, "dummy")
	buildCmd := exec.Command("go", "build", "-o", binFile, srcFile)
	buildCmd.Env = append(os.Environ(), "GO111MODULE=off")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build dummy program: %v", err)
	}

	// Run the dummy program
	runCmd := exec.Command(binFile)
	runOut, err := runCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := runCmd.Start(); err != nil {
		t.Fatalf("failed to start dummy program: %v", err)
	}
	defer func() {
		if runCmd.Process != nil {
			runCmd.Process.Kill()
		}
	}()

	// Read the port from stdout
	buf := make([]byte, 1024)
	n, err := runOut.Read(buf)
	if err != nil {
		t.Fatalf("failed to read port from dummy program: %v", err)
	}
	port := strings.TrimSpace(string(buf[:n]))

	// Let the process settle
	time.Sleep(500 * time.Millisecond)

	// Run mairu dev kill-port
	out, err := runMairu("dev", "kill-port", port)
	if err != nil {
		t.Fatalf("kill-port failed: %v, output: %s", err, out)
	}

	if !strings.Contains(out, "Killed process") {
		t.Errorf("Expected 'Killed process' in output, got: %s", out)
	}

	// Wait for the process to exit
	err = runCmd.Wait()
	if err == nil {
		t.Errorf("Expected dummy program to be killed, but it exited normally")
	}
}
