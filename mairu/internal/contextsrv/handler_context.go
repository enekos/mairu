package contextsrv

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) createContext(c *gin.Context) {
	var req struct {
		URI       string  `json:"uri"`
		Project   string  `json:"project"`
		ParentURI *string `json:"parent_uri"`
		Name      string  `json:"name"`
		Abstract  string  `json:"abstract"`
		Overview  string  `json:"overview"`
		Content   string  `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
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
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) listContext(c *gin.Context) {
	limit := intParam(c.Query("limit"), 200)
	var parentURI *string
	if v := c.Query("parentUri"); v != "" {
		parentURI = &v
	}
	items, err := h.svc.ListContextNodes(c.Query("project"), parentURI, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) updateContext(c *gin.Context) {
	var req struct {
		URI      string `json:"uri"`
		Name     string `json:"name"`
		Abstract string `json:"abstract"`
		Overview string `json:"overview"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) deleteContext(c *gin.Context) {
	if err := h.svc.DeleteContextNode(c.Query("uri")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
