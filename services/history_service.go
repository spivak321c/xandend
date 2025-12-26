package services

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"xand/models"
)

type HistoryService struct {
	cache     *CacheService
	mongo     *MongoDBService
	stopChan  chan struct{}
	mutex     sync.RWMutex
	
	// Keep hot data in memory for quick access (last 1 hour)
	recentNetworkSnapshots []models.NetworkSnapshot
	recentNodeSnapshots    map[string][]models.NodeSnapshot
}

func NewHistoryService(cache *CacheService, mongo *MongoDBService) *HistoryService {
	return &HistoryService{
		cache:                  cache,
		mongo:                  mongo,
		stopChan:               make(chan struct{}),
		recentNetworkSnapshots: make([]models.NetworkSnapshot, 0),
		recentNodeSnapshots:    make(map[string][]models.NodeSnapshot),
	}
}

func (hs *HistoryService) Start() {
	log.Println("Starting History Service with MongoDB...")
	
	// Collect snapshots every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	
	go func() {
		// Immediate first collection
		hs.collectSnapshot()
		
		for {
			select {
			case <-ticker.C:
				hs.collectSnapshot()
			case <-hs.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

func (hs *HistoryService) Stop() {
	close(hs.stopChan)
}

func (hs *HistoryService) collectSnapshot() {
	ctx := context.Background()
	
	stats, _, found := hs.cache.GetNetworkStats(true)
	if !found {
		return
	}

	nodes, _, _ := hs.cache.GetNodes(true)

	// Calculate average latency
	var totalLatency int64
	var latencyCount int
	for _, node := range nodes {
		if node.ResponseTime > 0 {
			totalLatency += node.ResponseTime
			latencyCount++
		}
	}
	var avgLatency int64
	if latencyCount > 0 {
		avgLatency = totalLatency / int64(latencyCount)
	}

	// Create network snapshot
	netSnap := models.NetworkSnapshot{
		Timestamp:      time.Now(),
		TotalNodes:     stats.TotalNodes,
		OnlineNodes:    stats.OnlineNodes,
		WarningNodes:   stats.WarningNodes,
		OfflineNodes:   stats.OfflineNodes,
		TotalStoragePB: stats.TotalStorage,
		UsedStoragePB:  stats.UsedStorage,
		AverageLatency: avgLatency,
		NetworkHealth:  stats.NetworkHealth,
		TotalStake:     stats.TotalStake,
		AverageUptime:  stats.AverageUptime,
		AveragePerf:    stats.AveragePerformance,
	}

	// Save to MongoDB
	if hs.mongo != nil && hs.mongo.enabled {
		if err := hs.mongo.InsertNetworkSnapshot(ctx, &netSnap); err != nil {
			log.Printf("Error saving network snapshot to MongoDB: %v", err)
		}
	}

	// Save to in-memory cache (last 1 hour = 12 snapshots at 5min intervals)
	hs.mutex.Lock()
	hs.recentNetworkSnapshots = append(hs.recentNetworkSnapshots, netSnap)
	if len(hs.recentNetworkSnapshots) > 12 {
		hs.recentNetworkSnapshots = hs.recentNetworkSnapshots[len(hs.recentNetworkSnapshots)-12:]
	}
	hs.mutex.Unlock()

	// Save node snapshots
	for _, node := range nodes {
		nodeSnap := models.NodeSnapshot{
			Timestamp:        time.Now(),
			NodeID:           node.ID,
			Status:           node.Status,
			ResponseTime:     node.ResponseTime,
			CPUPercent:       node.CPUPercent,
			RAMUsed:          node.RAMUsed,
			StorageUsed:      node.StorageUsed,
			UptimeScore:      node.UptimeScore,
			PerformanceScore: node.PerformanceScore,
		}
		
		// Save to MongoDB
		if hs.mongo != nil && hs.mongo.enabled {
			if err := hs.mongo.InsertNodeSnapshot(ctx, &nodeSnap); err != nil {
				log.Printf("Error saving node snapshot for %s to MongoDB: %v", node.ID, err)
			}
			
			// Register node in registry (tracks first_seen)
			if err := hs.mongo.RegisterNode(ctx, node.ID, node.FirstSeen); err != nil {
				log.Printf("Error registering node %s: %v", node.ID, err)
			}
		}
		
		// Save to in-memory cache (last 1 hour per node)
		hs.mutex.Lock()
		if _, exists := hs.recentNodeSnapshots[node.ID]; !exists {
			hs.recentNodeSnapshots[node.ID] = make([]models.NodeSnapshot, 0)
		}
		hs.recentNodeSnapshots[node.ID] = append(hs.recentNodeSnapshots[node.ID], nodeSnap)
		if len(hs.recentNodeSnapshots[node.ID]) > 12 {
			hs.recentNodeSnapshots[node.ID] = hs.recentNodeSnapshots[node.ID][len(hs.recentNodeSnapshots[node.ID])-12:]
		}
		hs.mutex.Unlock()
	}

	log.Printf("Collected snapshot: %d nodes, %.2f PB used (saved to MongoDB)", stats.TotalNodes, stats.UsedStorage)
}

// ============================================
// LEGACY METHODS (for backward compatibility)
// These use in-memory cache for recent data, MongoDB for older data
// ============================================

// GetNetworkHistory returns network snapshots
// If MongoDB is available, uses it. Otherwise falls back to in-memory.
func (hs *HistoryService) GetNetworkHistory(hours int) []models.NetworkSnapshot {
	if hours <= 0 {
		hours = 24
	}

	// If MongoDB is available and we need more than 1 hour of data
	if hs.mongo != nil && hs.mongo.enabled && hours > 1 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		startTime := time.Now().Add(-time.Duration(hours) * time.Hour)
		
		// Query MongoDB
		snapshots, err := hs.mongo.GetNetworkSnapshotsRange(ctx, startTime, time.Now())
		if err != nil {
			log.Printf("Error fetching network history from MongoDB: %v", err)
			// Fallback to in-memory
			return hs.getInMemoryNetworkHistory(hours)
		}
		
		return snapshots
	}

	// Use in-memory cache for recent data
	return hs.getInMemoryNetworkHistory(hours)
}

func (hs *HistoryService) getInMemoryNetworkHistory(hours int) []models.NetworkSnapshot {
	hs.mutex.RLock()
	defer hs.mutex.RUnlock()

	// Calculate how many snapshots to return (12 per hour at 5min intervals)
	count := hours * 12
	if count > len(hs.recentNetworkSnapshots) {
		count = len(hs.recentNetworkSnapshots)
	}

	start := len(hs.recentNetworkSnapshots) - count
	if start < 0 {
		start = 0
	}

	result := make([]models.NetworkSnapshot, count)
	copy(result, hs.recentNetworkSnapshots[start:])
	return result
}

// GetNodeHistory returns snapshots for a specific node
func (hs *HistoryService) GetNodeHistory(nodeID string, hours int) []models.NodeSnapshot {
	if hours <= 0 {
		hours = 24
	}

	// If MongoDB is available and we need more than 1 hour of data
	if hs.mongo != nil && hs.mongo.enabled && hours > 1 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		startTime := time.Now().Add(-time.Duration(hours) * time.Hour)
		
		// Query MongoDB
		snapshots, err := hs.mongo.GetNodeSnapshotsRange(ctx, nodeID, startTime, time.Now())
		if err != nil {
			log.Printf("Error fetching node history from MongoDB: %v", err)
			// Fallback to in-memory
			return hs.getInMemoryNodeHistory(nodeID, hours)
		}
		
		return snapshots
	}

	// Use in-memory cache for recent data
	return hs.getInMemoryNodeHistory(nodeID, hours)
}

func (hs *HistoryService) getInMemoryNodeHistory(nodeID string, hours int) []models.NodeSnapshot {
	hs.mutex.RLock()
	defer hs.mutex.RUnlock()

	snapshots, exists := hs.recentNodeSnapshots[nodeID]
	if !exists {
		return []models.NodeSnapshot{}
	}

	count := hours * 12
	if count > len(snapshots) {
		count = len(snapshots)
	}

	start := len(snapshots) - count
	if start < 0 {
		start = 0
	}

	result := make([]models.NodeSnapshot, count)
	copy(result, snapshots[start:])
	return result
}

// GetCapacityForecast predicts storage saturation
func (hs *HistoryService) GetCapacityForecast() models.CapacityForecast {
	forecast := models.CapacityForecast{
		CurrentUsagePB:    0,
		CurrentCapacityPB: 0,
		GrowthRatePBPerDay: 0,
		DaysToSaturation:  -1,
		Confidence:        0,
	}

	// Try MongoDB first for more accurate forecast with more data
	if hs.mongo != nil && hs.mongo.enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Get 30 days of data for better forecast
		result, err := hs.mongo.GetStorageGrowthRate(ctx, 30)
		if err == nil {
			forecast.CurrentUsagePB = result.EndStoragePB
			forecast.GrowthRatePBPerDay = result.GrowthRatePerDay
			
			// Get current capacity from latest snapshot
			stats, _, found := hs.cache.GetNetworkStats(true)
			if found {
				forecast.CurrentCapacityPB = stats.TotalStorage
				
				if forecast.GrowthRatePBPerDay > 0 {
					remaining := forecast.CurrentCapacityPB - forecast.CurrentUsagePB
					if remaining > 0 {
						forecast.DaysToSaturation = int(math.Ceil(remaining / forecast.GrowthRatePBPerDay))
						forecast.SaturationDate = time.Now().AddDate(0, 0, forecast.DaysToSaturation)
					}
				}
				
				forecast.Confidence = 85.0 // High confidence with 30 days of data
			}
			
			return forecast
		}
	}

	// Fallback to in-memory calculation (less accurate, only 1 hour of data)
	hs.mutex.RLock()
	snapshots := hs.recentNetworkSnapshots
	hs.mutex.RUnlock()

	if len(snapshots) < 2 {
		return forecast
	}

	// Get latest snapshot
	latest := snapshots[len(snapshots)-1]
	forecast.CurrentUsagePB = latest.UsedStoragePB
	forecast.CurrentCapacityPB = latest.TotalStoragePB

	// Get oldest snapshot in memory
	oldest := snapshots[0]
	
	hoursDiff := latest.Timestamp.Sub(oldest.Timestamp).Hours()
	if hoursDiff > 0 {
		usageDiff := latest.UsedStoragePB - oldest.UsedStoragePB
		forecast.GrowthRatePBPerDay = (usageDiff / hoursDiff) * 24

		// Calculate days to saturation
		if forecast.GrowthRatePBPerDay > 0 {
			remaining := forecast.CurrentCapacityPB - forecast.CurrentUsagePB
			if remaining > 0 {
				forecast.DaysToSaturation = int(math.Ceil(remaining / forecast.GrowthRatePBPerDay))
				forecast.SaturationDate = time.Now().AddDate(0, 0, forecast.DaysToSaturation)
			}
		}

		// Low confidence with only 1 hour of data
		forecast.Confidence = 30.0
	}

	return forecast
}

// GetLatencyDistribution returns latency histogram data
func (hs *HistoryService) GetLatencyDistribution() map[string]int {
	nodes, _, found := hs.cache.GetNodes(true)
	if !found {
		return map[string]int{}
	}

	distribution := map[string]int{
		"0-50ms":      0,
		"50-100ms":    0,
		"100-250ms":   0,
		"250-500ms":   0,
		"500-1000ms":  0,
		"1000-2000ms": 0,
		"2000ms+":     0,
	}

	for _, node := range nodes {
		latency := node.ResponseTime
		switch {
		case latency <= 50:
			distribution["0-50ms"]++
		case latency <= 100:
			distribution["50-100ms"]++
		case latency <= 250:
			distribution["100-250ms"]++
		case latency <= 500:
			distribution["250-500ms"]++
		case latency <= 1000:
			distribution["500-1000ms"]++
		case latency <= 2000:
			distribution["1000-2000ms"]++
		default:
			distribution["2000ms+"]++
		}
	}

	return distribution
}