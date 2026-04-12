package contextsrv

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) listModerationQueue(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r.URL.Query().Get("limit"), 100)
	items, err := h.svc.ListModerationQueue(limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (h *Handler) reviewModeration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventID  int64  `json:"event_id"`
		Decision string `json:"decision"`
		Reviewer string `json:"reviewer"`
		Notes    string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.ReviewModeration(ModerationReviewInput{
		EventID:  req.EventID,
		Decision: req.Decision,
		Reviewer: req.Reviewer,
		Notes:    req.Notes,
	}); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
