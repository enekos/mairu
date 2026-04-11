package contextsrv

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) applyBashHistoryFeedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		Reward int    `json:"reward"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": "invalid request body"})
		return
	}
	if req.ID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": "id is required"})
		return
	}

	out, err := h.svc.ApplyBashHistoryFeedback(req.ID, req.Reward)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}
