
package services

import (
	"log"
	"time"

	"xand/models"
	"xand/utils"
)

type DataAggregator struct {
	discovery *NodeDiscovery
}

func NewDataAggregator(discovery *NodeDiscovery) *DataAggregator {
	return &DataAggregator{
		discovery: discovery,
	}
}

// Aggregate performs full network aggregation
// func (da *DataAggregator) Aggregate() models.NetworkStats {
// 	nodes := da.discovery.GetNodes()

// 	aggr := models.NetworkStats{
// 		TotalNodes:  len(nodes),
// 		LastUpdated: time.Now(),
// 	}

// 	if len(nodes) == 0 {
// 		log.Println("No nodes available for aggregation")
// 		return aggr
// 	}

// 	var sumUptime float64
// 	var sumPerformance float64
// 	var countPerformance int
// 	var totalCredits int64      // ADD THIS
// 	var nodesWithCredits int    // ADD THIS

// 	for _, node := range nodes {
// 		utils.DetermineStatus(node)
// 		utils.CalculateScore(node)

// 		switch node.Status {
// 		case "online":
// 			aggr.OnlineNodes++
// 		case "warning":
// 			aggr.WarningNodes++
// 		case "offline":
// 			aggr.OfflineNodes++
// 		}

// 		aggr.TotalStorage += float64(node.StorageCapacity) / 1e15
// 		aggr.UsedStorage += float64(node.StorageUsed) / 1e15
// 		aggr.TotalStake += int64(node.TotalStake)

// 		sumUptime += node.UptimeScore
// 		if node.PerformanceScore > 0 {
// 			sumPerformance += node.PerformanceScore
// 			countPerformance++
// 		}
		
// 		// ADD THIS: Track credits
// 		if node.Credits > 0 {
// 			totalCredits += node.Credits
// 			nodesWithCredits++
// 		}
// 	}

// 	if len(nodes) > 0 {
// 		aggr.AverageUptime = sumUptime / float64(len(nodes))
// 	}
	
// 	if countPerformance > 0 {
// 		aggr.AveragePerformance = sumPerformance / float64(countPerformance)
// 	}

// 	if len(nodes) > 0 {
// 		onlineRatio := float64(aggr.OnlineNodes) / float64(aggr.TotalNodes)
// 		aggr.NetworkHealth = (onlineRatio * 80) + (aggr.AverageUptime * 0.2)
		
// 		if aggr.NetworkHealth > 100 {
// 			aggr.NetworkHealth = 100
// 		}
// 	}

// 	log.Printf("Aggregated %d nodes. Health: %.2f%%. Online: %d, Warning: %d, Offline: %d. Storage: %.2f PB / %.2f PB. Credits: %d nodes with avg %d", 
// 		len(nodes), aggr.NetworkHealth, aggr.OnlineNodes, aggr.WarningNodes, aggr.OfflineNodes,
// 		aggr.UsedStorage, aggr.TotalStorage, nodesWithCredits, totalCredits/int64(max(nodesWithCredits, 1)))

// 	return aggr
// }


func (da *DataAggregator) Aggregate() models.NetworkStats {
	nodes := da.discovery.GetNodes()

	aggr := models.NetworkStats{
		TotalNodes:  len(nodes), // Now includes ALL nodes (online, offline, etc.)
		LastUpdated: time.Now(),
	}

	if len(nodes) == 0 {
		log.Println("No nodes available for aggregation")
		return aggr
	}

	var sumUptime float64
	var sumPerformance float64
	var countPerformance int
	var totalCredits int64
	var nodesWithCredits int

	// Process ALL nodes, regardless of status
	for _, node := range nodes {
		// Always determine status and calculate scores for ALL nodes
		utils.DetermineStatus(node)
		utils.CalculateScore(node)

		// Count by status
		switch node.Status {
		case "online":
			aggr.OnlineNodes++
		case "warning":
			aggr.WarningNodes++
		case "offline":
			aggr.OfflineNodes++
		}

		// Aggregate storage from ALL nodes
		aggr.TotalStorage += float64(node.StorageCapacity) / 1e15
		aggr.UsedStorage += float64(node.StorageUsed) / 1e15
		aggr.TotalStake += int64(node.TotalStake)

		// Aggregate metrics from ALL nodes
		sumUptime += node.UptimeScore
		if node.PerformanceScore > 0 {
			sumPerformance += node.PerformanceScore
			countPerformance++
		}
		
		// Track credits from ALL nodes
		if node.Credits > 0 {
			totalCredits += node.Credits
			nodesWithCredits++
		}
	}

	// Calculate averages across ALL nodes
	if len(nodes) > 0 {
		aggr.AverageUptime = sumUptime / float64(len(nodes))
	}
	
	if countPerformance > 0 {
		aggr.AveragePerformance = sumPerformance / float64(countPerformance)
	}

	// Calculate network health (online ratio is key factor)
	if len(nodes) > 0 {
		onlineRatio := float64(aggr.OnlineNodes) / float64(aggr.TotalNodes)
		aggr.NetworkHealth = (onlineRatio * 80) + (aggr.AverageUptime * 0.2)
		
		if aggr.NetworkHealth > 100 {
			aggr.NetworkHealth = 100
		}
	}

	avgCredits := int64(0)
	if nodesWithCredits > 0 {
		avgCredits = totalCredits / int64(nodesWithCredits)
	}

	log.Printf("Aggregated %d nodes (Online: %d, Warning: %d, Offline: %d). Health: %.2f%%. Storage: %.2f/%.2f PB. Credits: %d nodes, avg %d", 
		len(nodes), aggr.OnlineNodes, aggr.WarningNodes, aggr.OfflineNodes,
		aggr.NetworkHealth, aggr.UsedStorage, aggr.TotalStorage, 
		nodesWithCredits, avgCredits)

	return aggr
}

// func max(a, b int) int {
// 	if a > b {
// 		return a
// 	}
// 	return b
// }