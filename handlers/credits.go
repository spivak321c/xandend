
package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"xand/services"
)

type CreditsHandlers struct {
	creditsService *services.CreditsService
}

func NewCreditsHandlers(creditsService *services.CreditsService) *CreditsHandlers {
	return &CreditsHandlers{
		creditsService: creditsService,
	}
}

// GetAllCredits - GET /api/credits
func (ch *CreditsHandlers) GetAllCredits(c echo.Context) error {
	credits := ch.creditsService.GetAllCredits()
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"total": len(credits),
		"credits": credits,
	})
}

// GetTopCredits - GET /api/credits/top?limit=20
func (ch *CreditsHandlers) GetTopCredits(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	limit := 20 // Default
	
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	credits := ch.creditsService.GetTopCredits(limit)
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"limit": limit,
		"count": len(credits),
		"top_credits": credits,
	})
}

// GetNodeCredits - GET /api/credits/:pubkey
func (ch *CreditsHandlers) GetNodeCredits(c echo.Context) error {
	pubkey := c.Param("pubkey")
	
	if pubkey == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "pubkey parameter required",
		})
	}
	
	credits, exists := ch.creditsService.GetCredits(pubkey)
	if !exists {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "credits not found for this pubkey",
			"pubkey": pubkey,
		})
	}

	return c.JSON(http.StatusOK, credits)
}

// GetCreditsByRank - GET /api/credits/rank?start=1&end=50
func (ch *CreditsHandlers) GetCreditsByRank(c echo.Context) error {
	startStr := c.QueryParam("start")
	endStr := c.QueryParam("end")
	
	start := 1
	end := 50
	
	if startStr != "" {
		if s, err := strconv.Atoi(startStr); err == nil && s > 0 {
			start = s
		}
	}
	
	if endStr != "" {
		if e, err := strconv.Atoi(endStr); err == nil && e > 0 {
			end = e
		}
	}
	
	if start > end {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "start rank must be less than or equal to end rank",
		})
	}
	
	credits := ch.creditsService.GetCreditsByRank(start, end)
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"start_rank": start,
		"end_rank": end,
		"count": len(credits),
		"credits": credits,
	})
}

// GetCreditsStats - GET /api/credits/stats
func (ch *CreditsHandlers) GetCreditsStats(c echo.Context) error {
	stats := ch.creditsService.GetCreditsStats()
	return c.JSON(http.StatusOK, stats)
}