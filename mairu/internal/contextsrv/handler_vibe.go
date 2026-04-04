package contextsrv

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) vibeQuery(c *gin.Context) {
	var req struct {
		Prompt  string `json:"prompt"`
		Project string `json:"project"`
		TopK    int    `json:"topK"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.svc.VibeQuery(req.Prompt, req.Project, req.TopK)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) vibeMutationPlan(c *gin.Context) {
	var req struct {
		Prompt  string `json:"prompt"`
		Project string `json:"project"`
		TopK    int    `json:"topK"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	out, err := h.svc.PlanVibeMutation(req.Prompt, req.Project, req.TopK)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) vibeMutationExecute(c *gin.Context) {
	var req struct {
		Project    string           `json:"project"`
		Operations []VibeMutationOp `json:"operations"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	results, err := h.svc.ExecuteVibeMutation(req.Operations, req.Project)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *Handler) vibeIngest(c *gin.Context) {
	var req struct {
		Text    string `json:"text"`
		BaseURI string `json:"base_uri"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	nodes, err := h.svc.Ingest(req.Text, req.BaseURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}
