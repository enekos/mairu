package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mairu/internal/agent"
	"mairu/internal/llm"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

// Server holds the configuration and dependencies for the web server
type Server struct {
	mux            *http.ServeMux
	projectRoot    string
	providerCfg    llm.ProviderConfig
	contextHandler http.Handler
	locator        agent.SymbolLocator
	upgrader       websocket.Upgrader
}

// NewServer creates a new instance of the Server
func NewServer(providerCfg llm.ProviderConfig, contextHandler http.Handler, locator agent.SymbolLocator) (*Server, error) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	s := &Server{
		mux:            http.NewServeMux(),
		projectRoot:    projectRoot,
		providerCfg:    providerCfg,
		contextHandler: contextHandler,
		locator:        locator,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for the dashboard
			},
		},
	}

	s.routes(contextHandler)
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
func (s *Server) routes(contextHandler http.Handler) {
	s.mux.HandleFunc("GET /api/ping", s.handlePing())
	s.mux.HandleFunc("GET /api/chat", s.handleChat())
	s.mux.HandleFunc("GET /api/sessions", s.handleGetSessions())
	s.mux.HandleFunc("POST /api/sessions", s.handleCreateSession())
	s.mux.HandleFunc("GET /api/sessions/:name/messages", s.handleGetSessionMessages())

	if contextHandler != nil {
		s.mux.Handle("/api/", contextHandler)
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

		if s.providerCfg.APIKey == "" {
			ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "API key not set"})
			return
		}

		ag, err := agent.New(s.projectRoot, s.providerCfg)
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
func SetupRouter(providerCfg llm.ProviderConfig, contextHandler http.Handler, locator agent.SymbolLocator) (*http.ServeMux, error) {
	s, err := NewServer(providerCfg, contextHandler, locator)
	if err != nil {
		return nil, err
	}
	return s.mux, nil
}

// StartServer starts the HTTP server on the given port
func StartServer(port int, providerCfg llm.ProviderConfig, contextHandler http.Handler, locator agent.SymbolLocator) error {
	s, err := NewServer(providerCfg, contextHandler, locator)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, s.mux)
}
