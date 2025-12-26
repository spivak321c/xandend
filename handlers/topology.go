package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"xand/services"
)

// TopologyHandlers manages network topology endpoints
type TopologyHandlers struct {
	topologyService *services.TopologyService
}

func NewTopologyHandlers(topologyService *services.TopologyService) *TopologyHandlers {
	return &TopologyHandlers{
		topologyService: topologyService,
	}
}

// GetTopology godoc
func (th *TopologyHandlers) GetTopology(c echo.Context) error {
	topology := th.topologyService.BuildTopology()
	return c.JSON(http.StatusOK, topology)
}

// GetRegionalClusters godoc
func (th *TopologyHandlers) GetRegionalClusters(c echo.Context) error {
	clusters := th.topologyService.GetRegionalClusters()
	return c.JSON(http.StatusOK, clusters)
}