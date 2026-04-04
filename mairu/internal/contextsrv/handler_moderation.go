package contextsrv

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) listModerationQueue(c *gin.Context) {
	limit := intParam(c.Query("limit"), 100)
	items, err := h.svc.ListModerationQueue(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) reviewModeration(c *gin.Context) {
	var req struct {
		EventID  int64  `json:"event_id"`
		Decision string `json:"decision"`
		Reviewer string `json:"reviewer"`
		Notes    string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.svc.ReviewModeration(ModerationReviewInput{
		EventID:  req.EventID,
		Decision: req.Decision,
		Reviewer: req.Reviewer,
		Notes:    req.Notes,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
