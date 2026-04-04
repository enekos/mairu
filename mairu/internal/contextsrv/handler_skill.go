package contextsrv

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) createSkill(c *gin.Context) {
	var req struct {
		Project     string `json:"project"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.svc.CreateSkill(SkillCreateInput{
		Project:     req.Project,
		Name:        req.Name,
		Description: req.Description,
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

func (h *Handler) listSkills(c *gin.Context) {
	limit := intParam(c.Query("limit"), 200)
	items, err := h.svc.ListSkills(c.Query("project"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) updateSkill(c *gin.Context) {
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.svc.UpdateSkill(SkillUpdateInput{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) deleteSkill(c *gin.Context) {
	if err := h.svc.DeleteSkill(c.Query("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
