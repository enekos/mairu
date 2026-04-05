package agent

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

func (a *Agent) FetchURL(url string) (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "MairuAgent/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("HTTP error: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	// For simplicity, just convert to string. If it's huge, we might want to truncate.
	content := string(bodyBytes)
	if len(content) > 50000 {
		content = content[:50000] + "\n... (truncated)"
	}

	// Basic HTML strip could be done here, but LLM usually handles raw HTML reasonably well,
	// though it consumes more tokens. (In a real app you'd use a robust HTML parser like golang.org/x/net/html)

	return content, nil
}
