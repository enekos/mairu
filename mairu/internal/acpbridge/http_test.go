package acpbridge

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPListEmpty(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var list []SessionInfo
	_ = json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("want empty, got %v", list)
	}
}

func TestHTTPCreateAndDelete(t *testing.T) {
	bin := buildFixture(t)
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{"echo": {Command: bin}}})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"agent": "echo"})
	resp, err := http.Post(srv.URL+"/sessions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("create status %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	if created.ID == "" {
		t.Fatal("no id returned")
	}

	req, _ := http.NewRequest("DELETE", srv.URL+"/sessions/"+created.ID, nil)
	dr, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer dr.Body.Close()
	if dr.StatusCode != 204 {
		t.Fatalf("delete status %d", dr.StatusCode)
	}
}

func TestHTTPCreateUnknownAgent(t *testing.T) {
	b, _ := New(Options{Addr: ":0", Agents: map[string]AgentSpec{}})
	srv := httptest.NewServer(b.Mux())
	defer srv.Close()
	body, _ := json.Marshal(map[string]string{"agent": "nope"})
	resp, _ := http.Post(srv.URL+"/sessions", "application/json", bytes.NewReader(body))
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
