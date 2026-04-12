package contextsrv

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) autocomplete(w http.ResponseWriter, r *http.Request) {
	var req AutocompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorString(w, http.StatusBadRequest, "invalid request body")
		return
	}

	out, err := h.svc.Autocomplete(req)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}
