package contextsrv

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.Health())
}

func (h *Handler) cluster(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.ClusterStats())
}

func (h *Handler) dashboard(c *gin.Context) {
	limit := intParam(c.Query("limit"), 200)
	out, err := h.svc.Dashboard(limit, c.Query("project"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}
