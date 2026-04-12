package contextsrv

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (h *Handler) createMemory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project    string `json:"project"`
		Content    string `json:"content"`
		Category   string `json:"category"`
		Owner      string `json:"owner"`
		Importance int    `json:"importance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	out, err := h.svc.CreateMemory(MemoryCreateInput{
		Project:    req.Project,
		Content:    req.Content,
		Category:   req.Category,
		Owner:      req.Owner,
		Importance: req.Importance,
	})
	if err != nil {
		if errors.Is(err, ErrModerationRejected) {
			writeJSONError(w, http.StatusUnprocessableEntity, err)
			return
		}
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) listMemories(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r.URL.Query().Get("limit"), 200)
	items, err := h.svc.ListMemories(r.URL.Query().Get("project"), limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(items)
}

func (h *Handler) updateMemory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         string `json:"id"`
		Content    string `json:"content"`
		Category   string `json:"category"`
		Owner      string `json:"owner"`
		Importance int    `json:"importance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	out, err := h.svc.UpdateMemory(MemoryUpdateInput{
		ID:         req.ID,
		Content:    req.Content,
		Category:   req.Category,
		Owner:      req.Owner,
		Importance: req.Importance,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) applyMemoryFeedback(w http.ResponseWriter, r *http.Request) {
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

	out, err := h.svc.ApplyMemoryFeedback(req.ID, req.Reward)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) deleteMemory(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteMemory(r.URL.Query().Get("id")); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
