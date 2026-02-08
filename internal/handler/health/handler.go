package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler manages health check endpoints
type Handler struct {
	readyFn func() error
}

// NewHandler creates a new health check handler
func NewHandler(readyFn func() error) *Handler {
	return &Handler{readyFn: readyFn}
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
	if h.readyFn != nil {
		if err := h.readyFn(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
