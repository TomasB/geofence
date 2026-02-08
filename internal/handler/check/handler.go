package check

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/TomasB/geofence/internal/data"
	"github.com/gin-gonic/gin"
)

// CheckRequest represents the JSON body for a country check.
type CheckRequest struct {
	IP               string   `json:"ip" binding:"required"`
	AllowedCountries []string `json:"allowed_countries" binding:"required,min=1"`
}

// CheckResponse represents the JSON response for a country check.
type CheckResponse struct {
	Allowed bool   `json:"allowed"`
	Country string `json:"country"`
	Error   string `json:"error"`
}

// Handler manages IP geolocation check endpoints.
type Handler struct {
	lookup data.CountryLookup
}

// NewHandler creates a new check handler with the given CountryLookup.
func NewHandler(lookup data.CountryLookup) *Handler {
	return &Handler{lookup: lookup}
}

// Check handles POST /api/v1/check
func (h *Handler) Check(c *gin.Context) {
	var req CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CheckResponse{
			Error: "invalid request: " + err.Error(),
		})
		return
	}

	slog.Debug("check request received", "ip", req.IP, "allowed_countries", req.AllowedCountries)

	ip := net.ParseIP(req.IP)
	if ip == nil {
		c.JSON(http.StatusBadRequest, CheckResponse{
			Error: "invalid IP address",
		})
		return
	}

	country, err := h.lookup.LookupCountry(ip)
	if err != nil {
		slog.Error("country lookup failed", "ip", req.IP, "error", err)
		c.JSON(http.StatusInternalServerError, CheckResponse{
			Error: "lookup failed",
		})
		return
	}

	allowed := false
	for _, ac := range req.AllowedCountries {
		if ac == country {
			allowed = true
			break
		}
	}

	c.JSON(http.StatusOK, CheckResponse{
		Allowed: allowed,
		Country: country,
	})
}
