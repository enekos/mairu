package desktop

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

const (
	meiliVersion    = "v1.12.3"
	meiliMasterKey  = "mairu-desktop-key"
	healthTimeout   = 15 * time.Second
	shutdownTimeout = 5 * time.Second
)

// MeiliManager manages a local Meilisearch process.
type MeiliManager struct {
	BaseDir string // e.g. ~/.mairu/meilisearch
	Port    int
	cmd     *exec.Cmd
}

// NewMeiliManager creates a manager that stores binaries and data under baseDir.
func NewMeiliManager(baseDir string) *MeiliManager {
	return &MeiliManager{BaseDir: baseDir}
}

func (m *MeiliManager) BinPath() string {
	name := "meilisearch"
	if runtime.GOOS == "windows" {
		name = "meilisearch.exe"
	}
	return filepath.Join(m.BaseDir, "bin", name)
}

func (m *MeiliManager) DataDir() string {
	return filepath.Join(m.BaseDir, "data")
}

func (m *MeiliManager) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", m.Port)
}

func (m *MeiliManager) APIKey() string {
	return meiliMasterKey
}

// IsInstalled reports whether the Meilisearch binary exists.
func (m *MeiliManager) IsInstalled() bool {
	_, err := os.Stat(m.BinPath())
	return err == nil
}

// Download fetches the Meilisearch binary for the current platform.
func (m *MeiliManager) Download(ctx context.Context, onProgress func(pct int)) error {
	asset := detectAssetName()
	url := fmt.Sprintf(
		"https://github.com/meilisearch/meilisearch/releases/download/%s/%s",
		meiliVersion, asset,
	)

	if err := os.MkdirAll(filepath.Dir(m.BinPath()), 0o755); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	f, err := os.OpenFile(m.BinPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			written += int64(n)
			if onProgress != nil && total > 0 {
				onProgress(int(written * 100 / total))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// Start launches Meilisearch on a free port and waits for it to become healthy.
func (m *MeiliManager) Start(ctx context.Context) error {
	if err := os.MkdirAll(m.DataDir(), 0o755); err != nil {
		return err
	}

	port, err := freePort()
	if err != nil {
		return fmt.Errorf("find free port: %w", err)
	}
	m.Port = port

	m.cmd = exec.CommandContext(ctx, m.BinPath(),
		"--http-addr", fmt.Sprintf("127.0.0.1:%d", m.Port),
		"--master-key", meiliMasterKey,
		"--db-path", m.DataDir(),
	)
	if runtime.GOOS != "windows" {
		m.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start meilisearch: %w", err)
	}

	return m.waitHealthy(ctx)
}

// Stop sends SIGTERM and waits for graceful shutdown, then SIGKILL as fallback.
func (m *MeiliManager) Stop() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	_ = m.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() { done <- m.cmd.Wait() }()

	select {
	case <-done:
		return nil
	case <-time.After(shutdownTimeout):
		_ = m.cmd.Process.Kill()
		return <-done
	}
}

func (m *MeiliManager) waitHealthy(ctx context.Context) error {
	deadline := time.Now().Add(healthTimeout)
	url := m.URL() + "/health"
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("meilisearch did not become healthy within %s", healthTimeout)
}

func detectAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	switch os + "-" + arch {
	case "darwin-arm64":
		return "meilisearch-macos-apple-silicon"
	case "darwin-amd64":
		return "meilisearch-macos-amd64"
	case "linux-amd64":
		return "meilisearch-linux-amd64"
	case "linux-arm64":
		return "meilisearch-linux-aarch64"
	case "windows-amd64":
		return "meilisearch-windows-amd64.exe"
	default:
		return ""
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
