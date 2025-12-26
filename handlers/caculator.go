package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"xand/services"
)

// CalculatorHandlers manages cost/ROI calculation endpoints
type CalculatorHandlers struct {
	calcService *services.CalculatorService
}

func NewCalculatorHandlers(calcService *services.CalculatorService) *CalculatorHandlers {
	return &CalculatorHandlers{
		calcService: calcService,
	}
}

// CompareCosts godoc
func (ch *CalculatorHandlers) CompareCosts(c echo.Context) error {
	storageTBStr := c.QueryParam("storage_tb")
	if storageTBStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "storage_tb parameter required"})
	}

	storageTB, err := strconv.ParseFloat(storageTBStr, 64)
	if err != nil || storageTB <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid storage_tb value"})
	}

	comparison := ch.calcService.CompareCosts(storageTB)
	return c.JSON(http.StatusOK, comparison)
}

// EstimateROI godoc
func (ch *CalculatorHandlers) EstimateROI(c echo.Context) error {
	storageTBStr := c.QueryParam("storage_tb")
	uptimeStr := c.QueryParam("uptime_percent")

	if storageTBStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "storage_tb parameter required"})
	}

	storageTB, err := strconv.ParseFloat(storageTBStr, 64)
	if err != nil || storageTB <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid storage_tb value"})
	}

	uptime := 99.0 // Default 99%
	if uptimeStr != "" {
		if u, err := strconv.ParseFloat(uptimeStr, 64); err == nil && u > 0 && u <= 100 {
			uptime = u
		}
	}

	estimate := ch.calcService.EstimateROI(storageTB, uptime)
	return c.JSON(http.StatusOK, estimate)
}

// SimulateRedundancy godoc
func (ch *CalculatorHandlers) SimulateRedundancy(c echo.Context) error {
	type SimRequest struct {
		FailedNodes []int `json:"failed_nodes"`
	}

	var req SimRequest
	if err := c.Bind(&req); err != nil {
		// If no body, check query param
		failedParam := c.QueryParam("failed")
		if failedParam != "" {
			// Parse comma-separated list
			// For simplicity, assume single value or empty
			req.FailedNodes = []int{}
		} else {
			req.FailedNodes = []int{}
		}
	}

	simulation := ch.calcService.SimulateRedundancy(req.FailedNodes)
	return c.JSON(http.StatusOK, simulation)
}