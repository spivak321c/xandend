package handlers

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/labstack/echo/v4"

	"xand/config"
	"xand/models"
	"xand/services"
)

type Handler struct {
	Cfg       *config.Config
	Cache     *services.CacheService
	Discovery *services.NodeDiscovery
	PRPC      *services.PRPCClient
}

func NewHandler(cfg *config.Config, cache *services.CacheService, discovery *services.NodeDiscovery, prpc *services.PRPCClient) *Handler {
	return &Handler{
		Cfg:       cfg,
		Cache:     cache,
		Discovery: discovery,
		PRPC:      prpc,
	}
}

// GetNodes godoc
// GetNodes godoc
// @Summary Get all nodes with pagination
// @Description Returns a paginated list of all discovered nodes
// @Tags nodes
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 50, max: 500)"
// @Param status query string false "Filter by status (online, warning, offline)"
// @Param sort query string false "Sort field (uptime, performance, credits, storage)"
// @Param order query string false "Sort order (asc, desc) (default: desc)"
// @Success 200 {object} NodesResponse
// @Router /api/nodes [get]
func (h *Handler) GetNodes(c echo.Context) error {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	// Parse filter parameters
	statusFilter := c.QueryParam("status")
	sortField := c.QueryParam("sort")
	sortOrder := c.QueryParam("order")
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Get nodes from cache (allow stale for better UX)
	nodes, stale, found := h.Cache.GetNodes(false)
	if !found {
		nodes, stale, found = h.Cache.GetNodes(true)
	}

	if !found {
		nodes = h.Discovery.GetNodes()
	}

	// Filter out incomplete nodes (no pubkey)
	completeNodes := make([]*models.Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Pubkey != "" {
			completeNodes = append(completeNodes, node)
		}
	}

		includeOffline := true
	if c.QueryParam("include_offline") == "false" {
		includeOffline = false
	}

	// Apply status filter
	// if statusFilter != "" {
	// 	filteredNodes := make([]*models.Node, 0)
	// 	for _, node := range completeNodes {
	// 		if node.Status == statusFilter {
	// 			filteredNodes = append(filteredNodes, node)
	// 		}
	// 	}
	// 	completeNodes = filteredNodes
	// }
	if statusFilter != "" {
		filteredNodes := make([]*models.Node, 0)
		for _, node := range completeNodes {
			if node.Status == statusFilter {
				filteredNodes = append(filteredNodes, node)
			}
		}
		completeNodes = filteredNodes
	} else if !includeOffline {
		// If not showing offline and no specific status filter, exclude offline nodes
		filteredNodes := make([]*models.Node, 0)
		for _, node := range completeNodes {
			if node.Status != "offline" {
				filteredNodes = append(filteredNodes, node)
			}
		}
		completeNodes = filteredNodes
	}

	// Sort nodes
	h.sortNodes(completeNodes, sortField, sortOrder)

	// Calculate pagination
	totalNodes := len(completeNodes)
	totalPages := (totalNodes + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	// Calculate slice bounds
	startIdx := (page - 1) * limit
	endIdx := startIdx + limit

	if startIdx >= totalNodes {
		startIdx = 0
		endIdx = 0
		page = 1
	}

	if endIdx > totalNodes {
		endIdx = totalNodes
	}

	// Slice the results
	paginatedNodes := make([]*models.Node, 0)
	if startIdx < endIdx {
		paginatedNodes = completeNodes[startIdx:endIdx]
	}

	// Build response
	response := NodesResponse{
		Nodes: paginatedNodes,
		Pagination: PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalItems: totalNodes,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
	}

	// Set headers
	if stale {
		c.Response().Header().Set("X-Data-Stale", "true")
	}

	return c.JSON(http.StatusOK, response)
}

// GetNode godoc
// @Summary Get a single node by ID
// @Description Returns detailed information about a specific node
// @Tags nodes
// @Produce json
// @Param id path string true "Node ID (pubkey or address)"
// @Success 200 {object} models.Node
// @Failure 404 {object} ErrorResponse
// @Router /api/nodes/{id} [get]
func (h *Handler) GetNode(c echo.Context) error {
	id := c.Param("id")

	// Try cache first (fresh)
	node, stale, found := h.Cache.GetNode(id, false)

	// Fallback to stale
	if !found {
		node, stale, found = h.Cache.GetNode(id, true)
	}

	// Fallback to searching in nodes list
	if !found {
		nodes, _, listFound := h.Cache.GetNodes(true)
		if listFound {
			for _, n := range nodes {
				if n.ID == id || n.Pubkey == id || n.Address == id {
					node = n
					found = true
					break
				}
			}
		}
	}

	if !found {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Node not found",
		})
	}

	if stale {
		c.Response().Header().Set("X-Data-Stale", "true")
	}

	return c.JSON(http.StatusOK, node)
}

// sortNodes sorts nodes based on the specified field and order
func (h *Handler) sortNodes(nodes []*models.Node, field, order string) {
	asc := order == "asc"

	sort.Slice(nodes, func(i, j int) bool {
		var less bool

		switch field {
		case "uptime":
			less = nodes[i].UptimeScore < nodes[j].UptimeScore
		case "performance":
			less = nodes[i].PerformanceScore < nodes[j].PerformanceScore
		case "credits":
			less = nodes[i].Credits < nodes[j].Credits
		case "storage":
			less = nodes[i].StorageUsed < nodes[j].StorageUsed
		case "latency":
			less = nodes[i].ResponseTime < nodes[j].ResponseTime
		default:
			// Default: sort by status weight, then uptime
			statusWeight := func(s string) int {
				switch s {
				case "online":
					return 3
				case "warning":
					return 2
				case "offline":
					return 1
				default:
					return 0
				}
			}

			if statusWeight(nodes[i].Status) != statusWeight(nodes[j].Status) {
				less = statusWeight(nodes[i].Status) < statusWeight(nodes[j].Status)
			} else {
				less = nodes[i].UptimeScore < nodes[j].UptimeScore
			}
		}

		if asc {
			return less
		}
		return !less
	})
}

// NodesResponse represents the paginated nodes response
type NodesResponse struct {
	Nodes      []*models.Node `json:"nodes"`
	Pagination PaginationMeta `json:"pagination"`
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	TotalItems int  `json:"total_items"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}