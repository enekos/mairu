// Package ctxclient is the shared transport for callers of the mairu context
// server. It resolves the base URL and auth token from the environment, builds
// authenticated HTTP requests, and executes them with a default timeout. The
// cmd and tui packages both consume this.
package ctxclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var DefaultTimeout = 20 * time.Second

func BaseURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_URL")), "/")
}

func Token() string {
	return strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_TOKEN"))
}

// Build constructs an authenticated request. Pass params for GET/DELETE query
// strings; pass body (JSON-marshaled) for POST/PUT.
func Build(method, fullURL string, params map[string]string, body any) (*http.Request, error) {
	u, err := url.Parse(fullURL)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			if strings.TrimSpace(v) == "" {
				continue
			}
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, u.String(), rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok := Token(); tok != "" {
		req.Header.Set("X-Context-Token", tok)
	}
	return req, nil
}

// Do executes req and returns the response body. Returns an error for any
// status >= 400 with the body appended.
func Do(req *http.Request) ([]byte, error) {
	client := &http.Client{Timeout: DefaultTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("context server error: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("context server HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
