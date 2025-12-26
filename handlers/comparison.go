package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"xand/services"
)

// ComparisonHandlers manages cross-chain comparison endpoints
type ComparisonHandlers struct {
	comparisonService *services.ComparisonService
}

func NewComparisonHandlers(comparisonService *services.ComparisonService) *ComparisonHandlers {
	return &ComparisonHandlers{
		comparisonService: comparisonService,
	}
}

// GetCrossChainComparison godoc
func (ch *ComparisonHandlers) GetCrossChainComparison(c echo.Context) error {
	comparison := ch.comparisonService.GetCrossChainComparison()
	return c.JSON(http.StatusOK, comparison)
}