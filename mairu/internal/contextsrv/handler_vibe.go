package contextsrv

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) vibeQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt  string `json:"prompt"`
		Project string `json:"project"`
		TopK    int    `json:"topK"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	out, err := h.svc.VibeQuery(req.Prompt, req.Project, req.TopK)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) vibeMutationPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prompt  string `json:"prompt"`
		Project string `json:"project"`
		TopK    int    `json:"topK"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	out, err := h.svc.PlanVibeMutation(req.Prompt, req.Project, req.TopK)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) vibeMutationExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project    string           `json:"project"`
		Operations []VibeMutationOp `json:"operations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	results, err := h.svc.ExecuteVibeMutation(req.Operations, req.Project)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"results": results})
}

func (h *Handler) vibeIngest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text    string `json:"text"`
		BaseURI string `json:"base_uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	nodes, err := h.svc.Ingest(req.Text, req.BaseURI)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"nodes": nodes})
}
