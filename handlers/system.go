package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// GetHealth returns OK
func (h *Handler) GetHealth(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

// GetStatus returns backend status
func (h *Handler) GetStatus(c echo.Context) error {
	nodes, _, _ := h.Cache.GetNodes(true)
	// stats, _ := h.Cache.GetNetworkStats(true) // optional

	status := map[string]interface{}{
		"status":      "running",
		"uptime":      "TODO: app start time",
		"knownNodes":  len(nodes),
		"cacheStatus": "active",
		"timestamp":   time.Now(),
	}
	return c.JSON(http.StatusOK, status)
}
