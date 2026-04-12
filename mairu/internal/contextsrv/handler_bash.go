package contextsrv

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// createBashHistoryRequest represents a request to create a new bash history entry
type createBashHistoryRequest struct {
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int    `json:"duration_ms"`
	Output     string `json:"output"`
	Project    string `json:"project"`
}

func (h *Handler) createBashHistory(w http.ResponseWriter, r *http.Request) {
	var req createBashHistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Command == "" {
		writeJSONErrorString(w, http.StatusBadRequest, "command is required")
		return
	}

	// Use project from request or extract from auth context
	project := req.Project
	if project == "" {
		project = "default"
	}

	// Get the repository from the service
	// We need to type assert to access the repository
	type repoProvider interface {
		InsertBashHistory(ctx context.Context, project string, command string, exitCode int, durationMs int, output string) error
	}

	// Try to get repository from service
	var repo repoProvider
	if svc, ok := h.svc.(*AppService); ok && svc.repo != nil {
		repo = svc.repo
	}

	if repo == nil {
		writeJSONErrorString(w, http.StatusServiceUnavailable, "bash history storage not available")
		return
	}

	ctx := r.Context()
	err := repo.InsertBashHistory(ctx, project, req.Command, req.ExitCode, req.DurationMs, req.Output)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "created",
		"project":   project,
		"command":   req.Command,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) applyBashHistoryFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Reward int    `json:"reward"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		writeJSONErrorString(w, http.StatusBadRequest, "id is required")
		return
	}

	out, err := h.svc.ApplyBashHistoryFeedback(req.ID, req.Reward)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}
