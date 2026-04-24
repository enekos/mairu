package tui

import (
	"net/http"

	"mairu/internal/ctxclient"
)

func (m *model) contextAPIBase() string {
	base := ctxclient.BaseURL()
	if base == "" {
		return "http://localhost:8788"
	}
	return base
}

func (m *model) contextToken() string {
	return ctxclient.Token()
}

func doContextRequest(req *http.Request) ([]byte, error) {
	return ctxclient.Do(req)
}
