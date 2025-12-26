package handlers

import (
	//"context"
	//"net/http"
	"strconv"
	"time"
	"github.com/labstack/echo/v4"
	"xand/services"
)

type AnalyticsHandlers struct {
	mongo *services.MongoDBService
}

func NewAnalyticsHandlers(mongo *services.MongoDBService) *AnalyticsHandlers {
	return &AnalyticsHandlers{mongo: mongo}
}

// 1. Daily Health
func (h *AnalyticsHandlers) GetDailyHealth(c echo.Context) error {
	year, _ := strconv.Atoi(c.QueryParam("year"))
	month, _ := strconv.Atoi(c.QueryParam("month"))
	
	if year == 0 {
		year = time.Now().Year()
	}
	if month == 0 {
		month = int(time.Now().Month())
	}
	
	results, err := h.mongo.GetDailyNetworkHealth(
		c.Request().Context(), 
		year, 
		time.Month(month),
	)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	
	return c.JSON(200, results)
}

// 2. High Uptime Nodes
func (h *AnalyticsHandlers) GetHighUptimeNodes(c echo.Context) error {
	minUptime, _ := strconv.ParseFloat(c.QueryParam("min_uptime"), 64)
	days, _ := strconv.Atoi(c.QueryParam("days"))
	
	if minUptime == 0 {
		minUptime = 90.0
	}
	if days == 0 {
		days = 30
	}
	
	results, err := h.mongo.GetHighUptimeNodes(
		c.Request().Context(),
		minUptime,
		days,
	)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	
	return c.JSON(200, results)
}

// 3. Storage Growth
func (h *AnalyticsHandlers) GetStorageGrowth(c echo.Context) error {
	days, _ := strconv.Atoi(c.QueryParam("days"))
	if days == 0 {
		days = 30
	}
	
	result, err := h.mongo.GetStorageGrowthRate(
		c.Request().Context(),
		days,
	)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	
	return c.JSON(200, result)
}

// 4. Recently Joined
func (h *AnalyticsHandlers) GetRecentlyJoined(c echo.Context) error {
	days, _ := strconv.Atoi(c.QueryParam("days"))
	if days == 0 {
		days = 7
	}
	
	results, err := h.mongo.GetRecentlyJoinedNodes(
		c.Request().Context(),
		days,
	)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	
	return c.JSON(200, results)
}

// 5. Weekly Comparison
func (h *AnalyticsHandlers) GetWeeklyComparison(c echo.Context) error {
	result, err := h.mongo.CompareWeeklyPerformance(
		c.Request().Context(),
	)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	
	return c.JSON(200, result)
}


// GetNodeGraveyard - GET /api/analytics/node-graveyard?days=30
func (h *AnalyticsHandlers) GetNodeGraveyard(c echo.Context) error {
	daysStr := c.QueryParam("days")
	days := 30 // Default: nodes not seen in 30 days
	
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}
	
	ctx := c.Request().Context()
	graveyard, err := h.mongo.GetInactiveNodes(ctx, days)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	
	// Calculate days since seen and status
	now := time.Now()
	for i := range graveyard {
		if graveyard[i].LastSnapshot != nil {
			daysSince := int(now.Sub(graveyard[i].LastSnapshot.Timestamp).Hours() / 24)
			graveyard[i].DaysSinceSeen = daysSince
			
			if daysSince > 90 {
				graveyard[i].Status = "dead"
			} else {
				graveyard[i].Status = "inactive"
			}
		}
	}
	
	return c.JSON(200, map[string]interface{}{
		"inactive_days_threshold": days,
		"total_inactive_nodes":    len(graveyard),
		"nodes":                   graveyard,
	})
}