package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mairu/internal/llm"
)

func TestPingRoute(t *testing.T) {
	r, err := SetupRouter(llm.ProviderConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("failed to setup router: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/ping", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["message"] != "pong from mairu" {
		t.Errorf("expected pong message, got %q", resp["message"])
	}
}

func TestSessionNameFromQuery(t *testing.T) {
	r, err := SetupRouter(llm.ProviderConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("failed to setup router: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/sessions", strings.NewReader(`{"name":"test-session"}`))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Errorf("expected route to exist")
	}
}
