package contextsrv

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSONErrorString(w, http.StatusBadRequest, "q parameter required")
		return
	}
	topK := intParam(r.URL.Query().Get("topK"), 10)
	store := r.URL.Query().Get("type")
	if store == "" {
		store = "all"
	}
	out, err := h.svc.Search(SearchOptions{
		Query:         query,
		Project:       r.URL.Query().Get("project"),
		Store:         store,
		TopK:          topK,
		MinScore:      floatParam(r.URL.Query().Get("minScore"), 0),
		Highlight:     r.URL.Query().Get("highlight") == "true",
		FieldBoosts:   parseFieldBoosts(r.URL.Query().Get("fieldBoosts")),
		Fuzziness:     r.URL.Query().Get("fuzziness"),
		PhraseBoost:   floatParam(r.URL.Query().Get("phraseBoost"), 0),
		WeightVector:  floatParam(r.URL.Query().Get("weightVector"), 0),
		WeightKeyword: floatParam(r.URL.Query().Get("weightKeyword"), 0),
		WeightRecency: floatParam(r.URL.Query().Get("weightRecency"), 0),
		WeightImp:     floatParam(r.URL.Query().Get("weightImportance"), 0),
		RecencyScale:  r.URL.Query().Get("recencyScale"),
		RecencyDecay:  floatParam(r.URL.Query().Get("recencyDecay"), 0),
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}
