// Optimized data_aggregator.go - Remove status recalculation

package services

import (
	"log"
	"time"

	"xand/models"
)

type DataAggregator struct {
	discovery *NodeDiscovery
}

func NewDataAggregator(discovery *NodeDiscovery) *DataAggregator {
	return &DataAggregator{
		discovery: discovery,
	}
}


func (da *DataAggregator) Aggregate() models.NetworkStats {
	allNodes := da.discovery.GetAllNodes()
	
	log.Printf("Aggregating stats from %d nodes", len(allNodes))
	
	uniquePubkeys := make(map[string]bool)
	var totalCommittedStoragePerPod float64
	
	aggr := models.NetworkStats{
		TotalNodes:  len(allNodes),
		LastUpdated: time.Now(),
	}
	
	if len(allNodes) == 0 {
		return aggr
	}
	
	var sumUptime float64
	var sumPerformance float64
	var countPerformance int
	var totalCredits int64
	var nodesWithCredits int
	
	// Aggregate with public/private tracking
	for _, node := range allNodes {
		// Track unique pubkeys
		if node.Pubkey != "" && node.Pubkey != "unknown" {
			if !uniquePubkeys[node.Pubkey] {
				uniquePubkeys[node.Pubkey] = true
				totalCommittedStoragePerPod += float64(node.StorageCapacity)
			}
		}
		
		// Count public/private totals
		if node.IsPublic {
			aggr.TotalPublicNodes++
		} else {
			aggr.TotalPrivateNodes++
		}
		
		// Use existing status (don't recalculate)
		switch node.Status {
		case "online":
			aggr.OnlineNodes++
			// NEW: Track online public/private split
			if node.IsPublic {
				aggr.OnlinePublicNodes++
			} else {
				aggr.OnlinePrivateNodes++
			}
		case "warning":
			aggr.WarningNodes++
		case "offline":
			aggr.OfflineNodes++
		}
		
		// Aggregate metrics
		aggr.TotalStorage += float64(node.StorageCapacity)
		aggr.UsedStorage += float64(node.StorageUsed)
		aggr.TotalStake += int64(node.TotalStake)
		
		sumUptime += node.UptimeScore
		if node.PerformanceScore > 0 {
			sumPerformance += node.PerformanceScore
			countPerformance++
		}
		
		if node.Credits > 0 {
			totalCredits += node.Credits
			nodesWithCredits++
		}
	}
	
	// Calculate aggregates
	aggr.TotalPods = len(uniquePubkeys)
	
	if aggr.TotalPods > 0 {
		aggr.AvgStorageCommittedPerPodBytes = totalCommittedStoragePerPod / float64(aggr.TotalPods)
	}
	
	if len(allNodes) > 0 {
		aggr.AverageUptime = sumUptime / float64(len(allNodes))
	}
	
	if countPerformance > 0 {
		aggr.AveragePerformance = sumPerformance / float64(countPerformance)
	}
	
	// Network health
	if len(allNodes) > 0 {
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
	
	log.Printf("Aggregated: %d IPs, %d pods. Online=%d (Public=%d, Private=%d), Warning=%d, Offline=%d. Health=%.1f%%. Avg credits=%d",
		aggr.TotalNodes, aggr.TotalPods,
		aggr.OnlineNodes, aggr.OnlinePublicNodes, aggr.OnlinePrivateNodes,
		aggr.WarningNodes, aggr.OfflineNodes,
		aggr.NetworkHealth, avgCredits)
	
	return aggr
}