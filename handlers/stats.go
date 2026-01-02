package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)




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

	// Get all nodes to aggregate RPC stats
	nodes, _, nodesFound := h.Cache.GetNodes(true)
	
	// Aggregate RPC stats from nodes (CPU, RAM, packets, uptime, etc.)
	var totalCPU, totalRAMUsed, totalRAMTotal float64
	var totalPacketsReceived, totalPacketsSent int64
	var totalUptime int64
	var nodesWithStats int
	
	if nodesFound {
		for _, node := range nodes {
			// Aggregate RPC stats (from get-stats calls stored in node)
			if node.CPUPercent > 0 || node.RAMUsed > 0 {
				totalCPU += node.CPUPercent
				totalRAMUsed += float64(node.RAMUsed)
				totalRAMTotal += float64(node.RAMTotal)
				totalPacketsReceived += node.PacketsReceived
				totalPacketsSent += node.PacketsSent
				totalUptime += node.UptimeSeconds
				nodesWithStats++
			}
		}
	}

	// Calculate averages
	var avgCPU, avgRAMUsed, avgRAMTotal, avgUptime float64
	if nodesWithStats > 0 {
		avgCPU = totalCPU / float64(nodesWithStats)
		avgRAMUsed = totalRAMUsed / float64(nodesWithStats)
		avgRAMTotal = totalRAMTotal / float64(nodesWithStats)
		avgUptime = float64(totalUptime) / float64(nodesWithStats)
	}

	// Build enhanced response
	response := NetworkStatsResponse{
		// Node counts (from aggregation)
		TotalNodes:   stats.TotalNodes,
		TotalPods:    stats.TotalPods,
		OnlineNodes:  stats.OnlineNodes,
		WarningNodes: stats.WarningNodes,
		OfflineNodes: stats.OfflineNodes,
		
		// NEW: Public/Private breakdown
		TotalPublicNodes:   stats.TotalPublicNodes,
		TotalPrivateNodes:  stats.TotalPrivateNodes,
		OnlinePublicNodes:  stats.OnlinePublicNodes,
		OnlinePrivateNodes: stats.OnlinePrivateNodes,
		
		// Storage metrics (from aggregation)
		TotalStorageBytes:              stats.TotalStorage,
		UsedStorageBytes:               stats.UsedStorage,
		AvgStorageCommittedPerPodBytes: stats.AvgStorageCommittedPerPodBytes,
		
		// Performance metrics (from aggregation)
		AverageUptime:      stats.AverageUptime,
		AveragePerformance: stats.AveragePerformance,
		TotalStake:         stats.TotalStake,
		NetworkHealth:      stats.NetworkHealth,
		
		// RPC Stats (from get-stats calls)
		AverageCPUPercent:       avgCPU,
		AverageRAMUsedBytes:     int64(avgRAMUsed),
		AverageRAMTotalBytes:    int64(avgRAMTotal),
		AverageUptimeSeconds:    int64(avgUptime),
		TotalPacketsReceived:    totalPacketsReceived,
		TotalPacketsSent:        totalPacketsSent,
		NodesWithRPCStats:       nodesWithStats,
		
		LastUpdated: stats.LastUpdated.String(),
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





type NetworkStatsResponse struct {
	// Node counts
	TotalNodes   int `json:"total_nodes"`   // All IP addresses
	TotalPods    int `json:"total_pods"`    // Unique pubkeys
	OnlineNodes  int `json:"online_nodes"`
	WarningNodes int `json:"warning_nodes"`
	OfflineNodes int `json:"offline_nodes"`
	
	// NEW: Public/Private breakdown
	TotalPublicNodes   int `json:"total_public_nodes"`
	TotalPrivateNodes  int `json:"total_private_nodes"`
	OnlinePublicNodes  int `json:"online_public_nodes"`  // NEW
	OnlinePrivateNodes int `json:"online_private_nodes"` // NEW
	
	// Storage metrics
	TotalStorageBytes              float64 `json:"total_storage_bytes"`
	UsedStorageBytes               float64 `json:"used_storage_bytes"`
	AvgStorageCommittedPerPodBytes float64 `json:"average_storage_committed_per_pod_bytes"`
	
	// Performance metrics
	AverageUptime      float64 `json:"average_uptime"`
	AveragePerformance float64 `json:"average_performance"`
	TotalStake         int64   `json:"total_stake"`
	NetworkHealth      float64 `json:"network_health"`
	
	// RPC Stats (from get-stats calls)
	AverageCPUPercent       float64 `json:"average_cpu_percent"`
	AverageRAMUsedBytes     int64   `json:"average_ram_used_bytes"`
	AverageRAMTotalBytes    int64   `json:"average_ram_total_bytes"`
	AverageUptimeSeconds    int64   `json:"average_uptime_seconds"`
	TotalPacketsReceived    int64   `json:"total_packets_received"`
	TotalPacketsSent        int64   `json:"total_packets_sent"`
	NodesWithRPCStats       int     `json:"nodes_with_rpc_stats"`
	
	LastUpdated string `json:"last_updated"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}