package acpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func (b *Bridge) Mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", b.handleSessions)
	mux.HandleFunc("/sessions/", b.handleSessionByID)
	mux.HandleFunc("/acp", b.handleWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
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
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			http.Error(w, "bad json: "+err.Error(), 400)
			return
		}
		if body.Agent == "" {
			http.Error(w, "agent required", 400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		opts := SessionStartOptions{
			PermissionTimeout: b.opts.PermissionTimeout,
			Logger:            b.logger,
		}
		id, err := b.registry.Create(ctx, body.Agent, b.opts.Agents, b.opts.RingBufferSize, opts, b.opts.MaxSessions)
		if err != nil {
			if errors.Is(err, ErrSessionCap) {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
				return
			}
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
	switch r.Method {
	case http.MethodGet:
		info, ok := b.registry.Info(id)
		if !ok {
			http.Error(w, "no such session", 404)
			return
		}
		writeJSON(w, 200, info)
	case http.MethodDelete:
		if err := b.registry.Delete(id); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
