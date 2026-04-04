package contextsrv

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) createMemory(c *gin.Context) {
	var req struct {
		Project    string `json:"project"`
		Content    string `json:"content"`
		Category   string `json:"category"`
		Owner      string `json:"owner"`
		Importance int    `json:"importance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
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
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) listMemories(c *gin.Context) {
	limit := intParam(c.Query("limit"), 200)
	items, err := h.svc.ListMemories(c.Query("project"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) updateMemory(c *gin.Context) {
	var req struct {
		ID         string `json:"id"`
		Content    string `json:"content"`
		Category   string `json:"category"`
		Owner      string `json:"owner"`
		Importance int    `json:"importance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) deleteMemory(c *gin.Context) {
	if err := h.svc.DeleteMemory(c.Query("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
