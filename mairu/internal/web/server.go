package web

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mairu/internal/agent"
	"mairu/ui"
	"net/http"
	"os"
	"strings"

	"encoding/json"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the dashboard
	},
}

type createSessionRequest struct {
	Name string `json:"name"`
}

func sessionNameFromQuery(r *http.Request) string {
	name := strings.TrimSpace(r.URL.Query().Get("session"))
	if name == "" {
		return "default"
	}
	return name
}

func SetupRouter(apiKey, meiliURL, meiliAPIKey string) (*http.ServeMux, error) {
	r := http.NewServeMux()
	contextServerURL := strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_URL"))
	contextServerToken := strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_TOKEN"))

	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// API routes

	r.HandleFunc("GET /api/ping", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"message": "pong from mairu"})
	})

	r.HandleFunc("GET /api/chat", func(w http.ResponseWriter, req *http.Request) {
		sessionName := sessionNameFromQuery(req)
		if err := agent.ValidateSessionName(sessionName); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		ws, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			slog.Error("upgrade error", "error", err)
			return
		}
		defer ws.Close()

		if apiKey == "" {
			ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "GEMINI_API_KEY not set"})
			return
		}

		ag, err := agent.New(projectRoot, apiKey, agent.Config{
			MeiliURL:    meiliURL,
			MeiliAPIKey: meiliAPIKey,
		})
		if err != nil {
			slog.Error("failed to init agent", "error", err)
			ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "failed to initialize agent: " + err.Error()})
			return
		}
		defer ag.Close()

		if err := ag.LoadSession(sessionName); err != nil {
			ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "Failed to load session: " + err.Error()})
			return
		}

		defer func() {
			if err := ag.SaveSession(sessionName); err != nil {
				slog.Error("failed to save session", "session", sessionName, "error", err)
			}
		}()

		promptChan := make(chan string)
		go func() {
			for {
				_, msg, err := ws.ReadMessage()
				if err != nil {
					close(promptChan)
					break
				}
				prompt := string(msg)
				cmd := strings.TrimSpace(prompt)
				if cmd == "/approve" {
					ag.ApproveAction(true)
				} else if cmd == "/deny" {
					ag.ApproveAction(false)
				} else {
					promptChan <- prompt
				}
			}
		}()

		for prompt := range promptChan {
			outChan := make(chan agent.AgentEvent)
			go ag.RunStream(prompt, outChan)

			for ev := range outChan {
				err := ws.WriteJSON(ev)
				if err != nil {
					break
				}
			}

			if err := ag.SaveSession(sessionName); err != nil {
				_ = ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "Failed to save session: " + err.Error()})
				break
			}
		}
	})

	r.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, req *http.Request) {
		sessions, err := agent.ListSessions(projectRoot)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": "failed to list sessions: " + err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"sessions": sessions})
	})

	r.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, req *http.Request) {
		var payload createSessionRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": "invalid request body"})
			return
		}
		if err := agent.ValidateSessionName(payload.Name); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		if err := agent.CreateEmptySession(projectRoot, payload.Name); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": "failed to create session: " + err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"name": payload.Name})
	})

	r.HandleFunc("GET /api/sessions/:name/messages", func(w http.ResponseWriter, req *http.Request) {
		sessionName := req.PathValue("name")
		if err := agent.ValidateSessionName(sessionName); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		saved, err := agent.LoadSavedSessionMessages(projectRoot, sessionName)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{"messages": []agent.SavedMessage{}})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{"error": "failed to load session: " + err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"messages": saved})
	})

	// Optional compatibility bridge to centralized context server.
	if contextServerURL != "" {
		forward := func(w http.ResponseWriter, req *http.Request) {
			target := strings.TrimSuffix(contextServerURL, "/") + req.URL.Path
			if q := req.URL.RawQuery; q != "" {
				target += "?" + q
			}

			var body io.Reader
			if req.Body != nil {
				raw, _ := io.ReadAll(req.Body)
				body = bytes.NewReader(raw)
			}

			req, err := http.NewRequest(req.Method, target, body)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				json.NewEncoder(w).Encode(map[string]any{"error": "failed to build context upstream request"})
				return
			}
			req.Header.Set("Content-Type", "application/json")
			if contextServerToken != "" {
				req.Header.Set("X-Context-Token", contextServerToken)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				json.NewEncoder(w).Encode(map[string]any{"error": "failed to reach centralized context server"})
				return
			}
			defer resp.Body.Close()

			payload, _ := io.ReadAll(resp.Body)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(resp.StatusCode)
			w.Write(payload)
		}

		r.HandleFunc("GET /api/search", forward)
		r.HandleFunc("GET /api/dashboard", forward)
		r.HandleFunc("GET /api/cluster", forward)
		r.HandleFunc("GET /api/memories", forward)
		r.HandleFunc("POST /api/memories", forward)
		r.HandleFunc("PUT /api/memories", forward)
		r.HandleFunc("DELETE /api/memories", forward)
		r.HandleFunc("GET /api/skills", forward)
		r.HandleFunc("POST /api/skills", forward)
		r.HandleFunc("PUT /api/skills", forward)
		r.HandleFunc("DELETE /api/skills", forward)
		r.HandleFunc("GET /api/context", forward)
		r.HandleFunc("POST /api/context", forward)
		r.HandleFunc("PUT /api/context", forward)
		r.HandleFunc("DELETE /api/context", forward)
		r.HandleFunc("POST /api/context/restore", forward)
		r.HandleFunc("POST /api/vibe/query", forward)
		r.HandleFunc("POST /api/vibe/mutation/plan", forward)
		r.HandleFunc("POST /api/vibe/mutation/execute", forward)
		r.HandleFunc("GET /api/moderation/queue", forward)
		r.HandleFunc("POST /api/moderation/review", forward)
	}

	// Serve the embedded Svelte UI
	distFS, err := fs.Sub(ui.FS, "dist")
	if err != nil {
		return nil, fmt.Errorf("failed to load ui assets: %w", err)
	}

	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if strings.HasPrefix(path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"error": "API route not found or context server not configured"})
			return
		}
		f, err := distFS.Open(path[1:])
		if err != nil {
			http.FileServer(http.FS(distFS)).ServeHTTP(w, req)
			return
		}
		defer f.Close()
		http.FileServer(http.FS(distFS)).ServeHTTP(w, req)
	})

	return r, nil
}

func StartServer(port int, apiKey, meiliURL, meiliAPIKey string) error {
	r, err := SetupRouter(apiKey, meiliURL, meiliAPIKey)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, r)
}
