package web

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mairu/internal/agent"
	"mairu/ui"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the dashboard
	},
}

type createSessionRequest struct {
	Name string `json:"name"`
}

func sessionNameFromQuery(c *gin.Context) string {
	name := strings.TrimSpace(c.Query("session"))
	if name == "" {
		return "default"
	}
	return name
}

func StartServer(port int, apiKey, meiliURL, meiliAPIKey string) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	contextServerURL := strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_URL"))
	contextServerToken := strings.TrimSpace(os.Getenv("MAIRU_CONTEXT_SERVER_TOKEN"))

	projectRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	// API routes
	api := r.Group("/api")
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong from mairu"})
		})

		api.GET("/chat", func(c *gin.Context) {
			sessionName := sessionNameFromQuery(c)
			if err := agent.ValidateSessionName(sessionName); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				log.Println("upgrade error:", err)
				return
			}
			defer ws.Close()

			if apiKey == "" {
				ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "GEMINI_API_KEY not set"})
				return
			}

			ag, err := agent.New(projectRoot, apiKey, agent.Config{
				MeiliURL:    meiliURL,
				MeiliAPIKey: meiliAPIKey,
			})
			if err != nil {
				log.Printf("failed to init agent: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize agent: " + err.Error()})
				return
			}
			defer ag.Close()

			if err := ag.LoadSession(sessionName); err != nil {
				ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "Failed to load session: " + err.Error()})
				return
			}

			defer func() {
				if err := ag.SaveSession(sessionName); err != nil {
					log.Printf("failed to save session %q: %v", sessionName, err)
				}
			}()

			for {
				_, msg, err := ws.ReadMessage()
				if err != nil {
					break
				}
				prompt := string(msg)

				outChan := make(chan agent.AgentEvent)
				go ag.RunStream(prompt, outChan)

				for ev := range outChan {
					err := ws.WriteJSON(ev)
					if err != nil {
						break
					}
				}

				if err := ag.SaveSession(sessionName); err != nil {
					_ = ws.WriteJSON(agent.AgentEvent{Type: "error", Content: "Failed to save session: " + err.Error()})
					break
				}
			}
		})

		api.GET("/sessions", func(c *gin.Context) {
			sessions, err := agent.ListSessions(projectRoot)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sessions: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"sessions": sessions})
		})

		api.POST("/sessions", func(c *gin.Context) {
			var req createSessionRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
			if err := agent.ValidateSessionName(req.Name); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			if err := agent.CreateEmptySession(projectRoot, req.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session: " + err.Error()})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"name": req.Name})
		})

		api.GET("/sessions/:name/messages", func(c *gin.Context) {
			sessionName := c.Param("name")
			if err := agent.ValidateSessionName(sessionName); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			saved, err := agent.LoadSavedSessionMessages(projectRoot, sessionName)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					c.JSON(http.StatusOK, gin.H{"messages": []agent.SavedMessage{}})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load session: " + err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"messages": saved})
		})

		// Optional compatibility bridge to centralized context server.
		if contextServerURL != "" {
			forward := func(c *gin.Context) {
				target := strings.TrimSuffix(contextServerURL, "/") + c.Request.URL.Path
				if q := c.Request.URL.RawQuery; q != "" {
					target += "?" + q
				}

				var body io.Reader
				if c.Request.Body != nil {
					raw, _ := io.ReadAll(c.Request.Body)
					body = bytes.NewReader(raw)
				}

				req, err := http.NewRequest(c.Request.Method, target, body)
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"error": "failed to build context upstream request"})
					return
				}
				req.Header.Set("Content-Type", "application/json")
				if contextServerToken != "" {
					req.Header.Set("X-Context-Token", contextServerToken)
				}

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach centralized context server"})
					return
				}
				defer resp.Body.Close()

				payload, _ := io.ReadAll(resp.Body)
				c.Data(resp.StatusCode, "application/json; charset=utf-8", payload)
			}

			api.GET("/search", forward)
			api.GET("/dashboard", forward)
			api.GET("/cluster", forward)
			api.GET("/memories", forward)
			api.POST("/memories", forward)
			api.PUT("/memories", forward)
			api.DELETE("/memories", forward)
			api.GET("/skills", forward)
			api.POST("/skills", forward)
			api.PUT("/skills", forward)
			api.DELETE("/skills", forward)
			api.GET("/context", forward)
			api.POST("/context", forward)
			api.PUT("/context", forward)
			api.DELETE("/context", forward)
			api.POST("/vibe/query", forward)
			api.POST("/vibe/mutation/plan", forward)
			api.POST("/vibe/mutation/execute", forward)
			api.GET("/moderation/queue", forward)
			api.POST("/moderation/review", forward)
		}
	}

	// Serve the embedded Svelte UI
	distFS, err := fs.Sub(ui.FS, "dist")
	if err != nil {
		return fmt.Errorf("failed to load ui assets: %w", err)
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API route not found or context server not configured"})
			return
		}
		// Simple fallback for SPA routing
		f, err := distFS.Open(path[1:])
		if err != nil {
			c.FileFromFS("/", http.FS(distFS))
			return
		}
		defer f.Close()
		c.FileFromFS(path, http.FS(distFS))
	})

	addr := fmt.Sprintf(":%d", port)
	return r.Run(addr)
}
