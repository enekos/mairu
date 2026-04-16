package tui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func (m *model) contextAPIBase() string {
	base := strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_URL"))
	if base == "" {
		base = "http://localhost:8788"
	}
	return strings.TrimRight(base, "/")
}

func (m *model) contextToken() string {
	return strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_TOKEN"))
}

func doContextRequest(req *http.Request) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("context api %s %s failed (%d): %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}
	return body, nil
}
