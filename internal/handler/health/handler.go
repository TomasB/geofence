package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler manages health check endpoints
type Handler struct{}

// NewHandler creates a new health check handler
func NewHandler() *Handler {
	return &Handler{}
}

// Health is the liveness probe endpoint
// GET /health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// Ready is the readiness probe endpoint
// GET /ready
func (h *Handler) Ready(c *gin.Context) {
	// Placeholder for future readiness checks
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
