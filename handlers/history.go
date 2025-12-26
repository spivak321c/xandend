package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"xand/services"
)

// HistoryHandlers manages historical data endpoints
type HistoryHandlers struct {
	historyService *services.HistoryService
}

func NewHistoryHandlers(historyService *services.HistoryService) *HistoryHandlers {
	return &HistoryHandlers{
		historyService: historyService,
	}
}

// GetNetworkHistory godoc
func (hh *HistoryHandlers) GetNetworkHistory(c echo.Context) error {
	hoursStr := c.QueryParam("hours")
	hours := 24 // Default 24 hours
	
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	snapshots := hh.historyService.GetNetworkHistory(hours)
	return c.JSON(http.StatusOK, snapshots)
}

// GetNodeHistory godoc
func (hh *HistoryHandlers) GetNodeHistory(c echo.Context) error {
	nodeID := c.Param("id")
	
	hoursStr := c.QueryParam("hours")
	hours := 24 // Default 24 hours
	
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	snapshots := hh.historyService.GetNodeHistory(nodeID, hours)
	
	if len(snapshots) == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no history for this node"})
	}

	return c.JSON(http.StatusOK, snapshots)
}
// GetCapacityForecast godoc
// @Summary Get storage capacity forecast
// @Description Predicts when network storage will reach capacity based on historical growth
// @Tags analytics
// @Produce json
// @Success 200 {object} models.CapacityForecast
// @Failure 500 {object} ErrorResponse
// @Router /api/forecast [get]
func (hh *HistoryHandlers) GetCapacityForecast(c echo.Context) error {
	forecast := hh.historyService.GetCapacityForecast()
	
	// Check if we have valid forecast data
	if forecast.CurrentCapacityPB == 0 {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"error":   "Insufficient data for forecast",
			"message": "Need at least 2 data points to calculate growth rate",
			"forecast": forecast,
		})
	}
	
	return c.JSON(http.StatusOK, forecast)
}

// GetLatencyDistribution godoc
func (hh *HistoryHandlers) GetLatencyDistribution(c echo.Context) error {
	distribution := hh.historyService.GetLatencyDistribution()
	return c.JSON(http.StatusOK, distribution)
}