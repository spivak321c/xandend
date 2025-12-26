package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// GetStats godoc
// @Summary Get network statistics
// @Description Returns comprehensive network statistics including node counts and health metrics
// @Tags stats
// @Produce json
// @Success 200 {object} NetworkStatsResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/stats [get]
func (h *Handler) GetStats(c echo.Context) error {
	// Try to get fresh stats first
	stats, stale, found := h.Cache.GetNetworkStats(false)

	// Fallback to stale if fresh not available
	if !found {
		stats, stale, found = h.Cache.GetNetworkStats(true)
	}

	// Last resort: trigger immediate refresh
	if !found {
		h.Cache.Refresh()
		stats, stale, found = h.Cache.GetNetworkStats(true)
		if !found {
			return c.JSON(http.StatusServiceUnavailable, ErrorResponse{
				Error: "Network statistics temporarily unavailable",
			})
		}
	}

	// Get all nodes to calculate public/private counts
	nodes, _, nodesFound := h.Cache.GetNodes(true)
	
	var publicNodes, privateNodes int
	if nodesFound {
		for _, node := range nodes {
			if node.IsPublic {
				publicNodes++
			} else {
				privateNodes++
			}
		}
	}

	// Build enhanced response
	response := NetworkStatsResponse{
		TotalNodes:         stats.TotalNodes,
		OnlineNodes:        stats.OnlineNodes,
		WarningNodes:       stats.WarningNodes,
		OfflineNodes:       stats.OfflineNodes,
		PublicNodes:        publicNodes,
		PrivateNodes:       privateNodes,
		TotalStoragePB:     stats.TotalStorage,
		UsedStoragePB:      stats.UsedStorage,
		AverageUptime:      stats.AverageUptime,
		AveragePerformance: stats.AveragePerformance,
		TotalStake:         stats.TotalStake,
		NetworkHealth:      stats.NetworkHealth,
		LastUpdated:        stats.LastUpdated.String(),
	}

	// Set stale header if data is old
	if stale {
		c.Response().Header().Set("X-Data-Stale", "true")
		c.Response().Header().Set("Cache-Control", "max-age=30")
	} else {
		c.Response().Header().Set("Cache-Control", "max-age=60")
	}

	return c.JSON(http.StatusOK, response)
}

// NetworkStatsResponse represents the enhanced stats response
type NetworkStatsResponse struct {
	TotalNodes         int     `json:"total_nodes"`
	OnlineNodes        int     `json:"online_nodes"`
	WarningNodes       int     `json:"warning_nodes"`
	OfflineNodes       int     `json:"offline_nodes"`
	PublicNodes        int     `json:"public_nodes"`
	PrivateNodes       int     `json:"private_nodes"`
	TotalStoragePB     float64 `json:"total_storage_pb"`
	UsedStoragePB      float64 `json:"used_storage_pb"`
	AverageUptime      float64 `json:"average_uptime"`
	AveragePerformance float64 `json:"average_performance"`
	TotalStake         int64   `json:"total_stake"`
	NetworkHealth      float64 `json:"network_health"`
	LastUpdated        string  `json:"last_updated"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}