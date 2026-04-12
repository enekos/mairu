package contextsrv

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Handler struct {
	engine *http.ServeMux
	svc    Service
}

func NewHandler(svc Service, authToken string) *Handler {
	mux := http.NewServeMux()

	h := &Handler{engine: mux, svc: svc}

	authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if authToken != "" {
				if r.Header.Get("X-Context-Token") != authToken {
					writeJSONErrorString(w, http.StatusUnauthorized, "unauthorized")
					return
				}
			}
			next(w, r)
		}
	}

	// System
	mux.HandleFunc("GET /api/health", authMiddleware(h.health))
	mux.HandleFunc("GET /api/cluster", authMiddleware(h.cluster))
	mux.HandleFunc("GET /api/dashboard", authMiddleware(h.dashboard))

	// Memories
	mux.HandleFunc("POST /api/memories", authMiddleware(h.createMemory))
	mux.HandleFunc("GET /api/memories", authMiddleware(h.listMemories))
	mux.HandleFunc("PUT /api/memories", authMiddleware(h.updateMemory))
	mux.HandleFunc("DELETE /api/memories", authMiddleware(h.deleteMemory))
	mux.HandleFunc("POST /api/memories/feedback", authMiddleware(h.applyMemoryFeedback))

	// Bash History
	mux.HandleFunc("POST /api/bash/history", authMiddleware(h.createBashHistory))
	mux.HandleFunc("POST /api/bash/feedback", authMiddleware(h.applyBashHistoryFeedback))

	// Skills
	mux.HandleFunc("POST /api/skills", authMiddleware(h.createSkill))
	mux.HandleFunc("GET /api/skills", authMiddleware(h.listSkills))
	mux.HandleFunc("PUT /api/skills", authMiddleware(h.updateSkill))
	mux.HandleFunc("DELETE /api/skills", authMiddleware(h.deleteSkill))

	// Context Nodes
	mux.HandleFunc("POST /api/context", authMiddleware(h.createContext))
	mux.HandleFunc("GET /api/context", authMiddleware(h.listContext))
	mux.HandleFunc("PUT /api/context", authMiddleware(h.updateContext))
	mux.HandleFunc("DELETE /api/context", authMiddleware(h.deleteContext))

	// Search
	mux.HandleFunc("GET /api/search", authMiddleware(h.search))

	// Vibe
	mux.HandleFunc("POST /api/vibe/query", authMiddleware(h.vibeQuery))
	mux.HandleFunc("POST /api/vibe/mutation/plan", authMiddleware(h.vibeMutationPlan))
	mux.HandleFunc("POST /api/vibe/mutation/execute", authMiddleware(h.vibeMutationExecute))
	mux.HandleFunc("POST /api/vibe/ingest", authMiddleware(h.vibeIngest))
	mux.HandleFunc("POST /api/autocomplete", authMiddleware(h.autocomplete))

	// Moderation
	mux.HandleFunc("GET /api/moderation/queue", authMiddleware(h.listModerationQueue))
	mux.HandleFunc("POST /api/moderation/review", authMiddleware(h.reviewModeration))

	return h
}

func intParam(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func floatParam(raw string, fallback float64) float64 {
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return n
}

func parseFieldBoosts(raw string) map[string]float64 {
	if raw == "" {
		return nil
	}
	out := map[string]float64{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.engine.ServeHTTP(w, r)
}
