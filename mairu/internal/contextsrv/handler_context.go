package contextsrv

import (
	"encoding/json"
	"errors"
	"net/http"
)

func (h *Handler) createContext(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URI       string  `json:"uri"`
		Project   string  `json:"project"`
		ParentURI *string `json:"parent_uri"`
		Name      string  `json:"name"`
		Abstract  string  `json:"abstract"`
		Overview  string  `json:"overview"`
		Content   string  `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	out, err := h.svc.CreateContextNode(ContextCreateInput{
		URI:       req.URI,
		Project:   req.Project,
		ParentURI: req.ParentURI,
		Name:      req.Name,
		Abstract:  req.Abstract,
		Overview:  req.Overview,
		Content:   req.Content,
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

func (h *Handler) listContext(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r.URL.Query().Get("limit"), 200)
	var parentURI *string
	if v := r.URL.Query().Get("parentUri"); v != "" {
		parentURI = &v
	}
	items, err := h.svc.ListContextNodes(r.URL.Query().Get("project"), parentURI, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(items)
}

func (h *Handler) updateContext(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URI      string `json:"uri"`
		Name     string `json:"name"`
		Abstract string `json:"abstract"`
		Overview string `json:"overview"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}
	out, err := h.svc.UpdateContextNode(ContextUpdateInput{
		URI:      req.URI,
		Name:     req.Name,
		Abstract: req.Abstract,
		Overview: req.Overview,
		Content:  req.Content,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) deleteContext(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteContextNode(r.URL.Query().Get("uri")); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
