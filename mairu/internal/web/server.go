package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mairu/internal/agent"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

// Server holds the configuration and dependencies for the web server
type Server struct {
	mux                *http.ServeMux
	projectRoot        string
	apiKey             string
	meiliURL           string
	meiliAPIKey        string
	contextServerURL   string
	contextServerToken string
	upgrader           websocket.Upgrader
}

// NewServer creates a new instance of the Server
func NewServer(apiKey, meiliURL, meiliAPIKey string) (*Server, error) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	s := &Server{
		mux:                http.NewServeMux(),
		projectRoot:        projectRoot,
		apiKey:             apiKey,
		meiliURL:           meiliURL,
		meiliAPIKey:        meiliAPIKey,
		contextServerURL:   strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_URL")),
		contextServerToken: strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_TOKEN")),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for the dashboard
			},
		},
	}

	s.routes()
	return s, nil
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

// writeJSON writes a JSON response with the given status code
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

// writeError writes a JSON error response with the given status code
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// routes registers all the HTTP handlers
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/ping", s.handlePing())
	s.mux.HandleFunc("GET /api/chat", s.handleChat())
	s.mux.HandleFunc("GET /api/sessions", s.handleGetSessions())
	s.mux.HandleFunc("POST /api/sessions", s.handleCreateSession())
	s.mux.HandleFunc("GET /api/sessions/:name/messages", s.handleGetSessionMessages())

	if s.contextServerURL != "" {
		s.registerForwardingRoutes()
	}

	s.mux.HandleFunc("/", s.handleStaticFiles())
}

func (s *Server) handlePing() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"message": "pong from mairu"})
	}
}

func (s *Server) handleGetSessions() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sessions, err := agent.ListSessions(s.projectRoot)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list sessions: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	}
}

func (s *Server) handleCreateSession() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var payload createSessionRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := agent.ValidateSessionName(payload.Name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := agent.CreateEmptySession(s.projectRoot, payload.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create session: "+err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"name": payload.Name})
	}
}

func (s *Server) handleGetSessionMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sessionName := req.PathValue("name")
		if err := agent.ValidateSessionName(sessionName); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		saved, err := agent.LoadSavedSessionMessages(s.projectRoot, sessionName)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeJSON(w, http.StatusOK, map[string]any{"messages": []agent.SavedMessage{}})
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load session: "+err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"messages": saved})
	}
}

func (s *Server) handleChat() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		sessionName := sessionNameFromQuery(req)
		if err := agent.ValidateSessionName(sessionName); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		ws, err := s.upgrader.Upgrade(w, req, nil)
		if err != nil {
			slog.Error("upgrade error", "error", err)
			return
		}
		defer ws.Close()

		if s.apiKey == "" {
			ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "GEMINI_API_KEY not set"})
			return
		}

		ag, err := agent.New(s.projectRoot, s.apiKey, agent.Config{
			MeiliURL:    s.meiliURL,
			MeiliAPIKey: s.meiliAPIKey,
		})
		if err != nil {
			slog.Error("failed to init agent", "error", err)
			ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "failed to initialize agent: " + err.Error()})
			return
		}

		modelQuery := req.URL.Query().Get("model")
		if modelQuery != "" {
			ag.SetModel(modelQuery)
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

		s.processWebSocketChat(ws, ag, sessionName)
	}
}

func (s *Server) processWebSocketChat(ws *websocket.Conn, ag *agent.Agent, sessionName string) {
	promptChan := make(chan string)
	go func() {
		defer close(promptChan)
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
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
}

func (s *Server) registerForwardingRoutes() {
	routes := []string{
		"GET /api/search",
		"GET /api/dashboard",
		"GET /api/cluster",
		"GET /api/memories",
		"POST /api/memories",
		"PUT /api/memories",
		"DELETE /api/memories",
		"GET /api/skills",
		"POST /api/skills",
		"PUT /api/skills",
		"DELETE /api/skills",
		"GET /api/context",
		"POST /api/context",
		"PUT /api/context",
		"DELETE /api/context",
		"POST /api/context/restore",
		"POST /api/vibe/query",
		"POST /api/vibe/mutation/plan",
		"POST /api/vibe/mutation/execute",
		"POST /api/autocomplete",
		"GET /api/moderation/queue",
		"POST /api/moderation/review",
	}

	for _, route := range routes {
		s.mux.HandleFunc(route, s.handleForward())
	}
}

func (s *Server) handleForward() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		target := strings.TrimSuffix(s.contextServerURL, "/") + req.URL.Path
		if q := req.URL.RawQuery; q != "" {
			target += "?" + q
		}

		var body io.Reader
		if req.Body != nil {
			raw, _ := io.ReadAll(req.Body)
			body = bytes.NewReader(raw)
		}

		proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, target, body)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to build context upstream request")
			return
		}
		proxyReq.Header.Set("Content-Type", "application/json")
		if s.contextServerToken != "" {
			proxyReq.Header.Set("X-Context-Token", s.contextServerToken)
		}

		resp, err := http.DefaultClient.Do(proxyReq)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to reach centralized context server")
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func (s *Server) handleStaticFiles() http.HandlerFunc {
	distFS := os.DirFS("ui/dist")
	fileServer := http.FileServer(http.FS(distFS))

	return func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if strings.HasPrefix(path, "/api/") {
			writeError(w, http.StatusNotFound, "API route not found or context server not configured")
			return
		}
		f, err := distFS.Open(path[1:])
		if err != nil {
			fileServer.ServeHTTP(w, req)
			return
		}
		defer f.Close()
		fileServer.ServeHTTP(w, req)
	}
}

// SetupRouter is kept for backwards compatibility and easy setup
func SetupRouter(apiKey, meiliURL, meiliAPIKey string) (*http.ServeMux, error) {
	s, err := NewServer(apiKey, meiliURL, meiliAPIKey)
	if err != nil {
		return nil, err
	}
	return s.mux, nil
}

// StartServer starts the HTTP server on the given port
func StartServer(port int, apiKey, meiliURL, meiliAPIKey string) error {
	s, err := NewServer(apiKey, meiliURL, meiliAPIKey)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, s.mux)
}
