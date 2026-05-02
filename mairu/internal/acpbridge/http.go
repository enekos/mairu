package acpbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (b *Bridge) Mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", b.handleSessions)
	mux.HandleFunc("/sessions/", b.handleSessionByID)
	mux.HandleFunc("/acp", b.handleWS)
	return mux
}

func (b *Bridge) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, b.registry.List())
	case http.MethodPost:
		var body struct {
			Agent string `json:"agent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		id, err := b.registry.Create(ctx, body.Agent, b.opts.Agents, b.opts.RingBufferSize)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, 201, map[string]string{"id": id})
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (b *Bridge) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/sessions/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := b.registry.Delete(id); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	w.WriteHeader(204)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
