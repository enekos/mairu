package contextsrv

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	engine *gin.Engine
	svc    Service
}

func NewHandler(svc Service, authToken string) *Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		if authToken == "" {
			c.Next()
			return
		}
		if c.GetHeader("X-Context-Token") != authToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	})

	h := &Handler{engine: r, svc: svc}

	api := r.Group("/api")
	{
		// System
		api.GET("/health", h.health)
		api.GET("/cluster", h.cluster)
		api.GET("/dashboard", h.dashboard)

		// Memories
		api.POST("/memories", h.createMemory)
		api.GET("/memories", h.listMemories)
		api.PUT("/memories", h.updateMemory)
		api.DELETE("/memories", h.deleteMemory)

		// Skills
		api.POST("/skills", h.createSkill)
		api.GET("/skills", h.listSkills)
		api.PUT("/skills", h.updateSkill)
		api.DELETE("/skills", h.deleteSkill)

		// Context Nodes
		api.POST("/context", h.createContext)
		api.GET("/context", h.listContext)
		api.PUT("/context", h.updateContext)
		api.DELETE("/context", h.deleteContext)

		// Search
		api.GET("/search", h.search)

		// Vibe
		api.POST("/vibe/query", h.vibeQuery)
		api.POST("/vibe/mutation/plan", h.vibeMutationPlan)
		api.POST("/vibe/mutation/execute", h.vibeMutationExecute)
		api.POST("/vibe/ingest", h.vibeIngest)

		// Moderation
		api.GET("/moderation/queue", h.listModerationQueue)
		api.POST("/moderation/review", h.reviewModeration)
	}

	return h
}

func intParam(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func floatParam(raw string, fallback float64) float64 {
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return n
}

func parseFieldBoosts(raw string) map[string]float64 {
	if raw == "" {
		return nil
	}
	out := map[string]float64{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.engine.ServeHTTP(w, r)
}
