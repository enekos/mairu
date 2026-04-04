package contextsrv

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q parameter required"})
		return
	}
	topK := intParam(c.Query("topK"), 10)
	store := c.DefaultQuery("type", "all")
	out, err := h.svc.Search(SearchOptions{
		Query:         query,
		Project:       c.Query("project"),
		Store:         store,
		TopK:          topK,
		MinScore:      floatParam(c.Query("minScore"), 0),
		Highlight:     c.Query("highlight") == "true",
		FieldBoosts:   parseFieldBoosts(c.Query("fieldBoosts")),
		Fuzziness:     c.Query("fuzziness"),
		PhraseBoost:   floatParam(c.Query("phraseBoost"), 0),
		WeightVector:  floatParam(c.Query("weightVector"), 0),
		WeightKeyword: floatParam(c.Query("weightKeyword"), 0),
		WeightRecency: floatParam(c.Query("weightRecency"), 0),
		WeightImp:     floatParam(c.Query("weightImportance"), 0),
		RecencyScale:  c.Query("recencyScale"),
		RecencyDecay:  floatParam(c.Query("recencyDecay"), 0),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
