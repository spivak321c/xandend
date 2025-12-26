package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"xand/models"
)

type ComparisonService struct {
	cache      *CacheService
	httpClient *http.Client
}

func NewComparisonService(cache *CacheService) *ComparisonService {
	return &ComparisonService{
		cache: cache,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetCrossChainComparison compares Xandeum vs Solana
func (cs *ComparisonService) GetCrossChainComparison() models.CrossChainComparison {
	comparison := models.CrossChainComparison{
		Timestamp: time.Now(),
	}

	// Get Xandeum metrics
	comparison.Xandeum = cs.getXandeumMetrics()

	// Get Solana metrics
	comparison.Solana = cs.getSolanaMetrics()

	// Calculate deltas
	comparison.PerformanceDelta = cs.calculateDelta(comparison.Xandeum, comparison.Solana)

	return comparison
}

func (cs *ComparisonService) getXandeumMetrics() models.XandeumMetrics {
	stats, _, found := cs.cache.GetNetworkStats(true)
	if !found {
		return models.XandeumMetrics{}
	}

	nodes, _, _ := cs.cache.GetNodes(true)

	// Calculate average latency
	var totalLatency int64
	var count int
	for _, node := range nodes {
		if node.ResponseTime > 0 {
			totalLatency += node.ResponseTime
			count++
		}
	}
	var avgLatency int64
	if count > 0 {
		avgLatency = totalLatency / int64(count)
	}

	// Storage Power Index: sqrt(nodes * storage_pb)
	// This is a custom metric to quantify storage network strength
	storagePowerIndex := math.Sqrt(float64(stats.TotalNodes) * stats.TotalStorage)

	// Data availability: percentage of data that can be retrieved
	// Simplified: (online_nodes / total_nodes) * 100
	dataAvailability := 0.0
	if stats.TotalNodes > 0 {
		dataAvailability = (float64(stats.OnlineNodes) / float64(stats.TotalNodes)) * 100
	}

	return models.XandeumMetrics{
		StoragePowerIndex: storagePowerIndex,
		TotalNodes:        stats.TotalNodes,
		TotalStoragePB:    stats.TotalStorage,
		AverageLatency:    avgLatency,
		NetworkHealth:     stats.NetworkHealth,
		DataAvailability:  dataAvailability,
	}
}

func (cs *ComparisonService) getSolanaMetrics() models.SolanaMetrics {
	// Attempt to fetch real Solana data from public RPC
	// Fallback to mock data if unavailable

	metrics := cs.fetchSolanaRPC()
	if metrics != nil {
		return *metrics
	}

	// Return an empty struct if fetching fails
	return models.SolanaMetrics{}
}

func (cs *ComparisonService) fetchSolanaRPC() *models.SolanaMetrics {
	// Try to fetch from a public Solana RPC endpoint
	// Using mainnet-beta public endpoint
	url := "https://api.mainnet-beta.solana.com"

	// Get cluster nodes
	nodesReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getClusterNodes",
	}

	nodesData, err := cs.callSolanaRPC(url, nodesReq)
	if err != nil {
		return nil
	}

	var nodesResp struct {
		Result []map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal(nodesData, &nodesResp); err != nil {
		return nil
	}

	// Get recent performance samples
	perfReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getRecentPerformanceSamples",
		"params":  []interface{}{10},
	}

	perfData, err := cs.callSolanaRPC(url, perfReq)
	if err != nil {
		return nil
	}

	var perfResp struct {
		Result []struct {
			NumSlots         int64 `json:"numSlots"`
			NumTransactions  int64 `json:"numTransactions"`
			SamplePeriodSecs int64 `json:"samplePeriodSecs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(perfData, &perfResp); err != nil {
		return nil
	}

	// Calculate metrics
	totalValidators := len(nodesResp.Result)

	var avgSlotTime int64 = 400 // Default
	var tps int64 = 0
	if len(perfResp.Result) > 0 {
		sample := perfResp.Result[0]
		if sample.NumSlots > 0 {
			avgSlotTime = (sample.SamplePeriodSecs * 1000) / sample.NumSlots
			tps = sample.NumTransactions / sample.SamplePeriodSecs
		}
	}

	return &models.SolanaMetrics{
		StakePowerIndex: 500000000, // Would need staking info endpoint
		TotalValidators: totalValidators,
		TotalStake:      500000000000000000,
		AverageSlotTime: avgSlotTime,
		TPS:             tps,
		NetworkHealth:   99.5,
	}
}

func (cs *ComparisonService) callSolanaRPC(url string, payload map[string]interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := cs.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

func (cs *ComparisonService) calculateDelta(xandeum models.XandeumMetrics, solana models.SolanaMetrics) models.PerformanceDelta {
	delta := models.PerformanceDelta{
		LatencyDifference: xandeum.AverageLatency - solana.AverageSlotTime,
		HealthDifference:  xandeum.NetworkHealth - solana.NetworkHealth,
	}

	// Generate summary
	if delta.LatencyDifference > 0 {
		delta.Summary = fmt.Sprintf("Xandeum storage layer has %.0fms higher latency than Solana consensus layer", float64(delta.LatencyDifference))
	} else {
		delta.Summary = fmt.Sprintf("Xandeum storage layer has %.0fms lower latency than Solana consensus layer", float64(-delta.LatencyDifference))
	}

	// Interpretation
	if math.Abs(float64(delta.LatencyDifference)) < 100 {
		delta.Interpretation = "Latency difference is minimal. Both layers perform comparably."
	} else if delta.LatencyDifference > 500 {
		delta.Interpretation = "Storage layer latency is significantly higher, which is expected for distributed storage operations."
	} else if delta.LatencyDifference < -100 {
		delta.Interpretation = "Storage layer is performing exceptionally well with lower latency than consensus layer."
	} else {
		delta.Interpretation = "Latency difference is within acceptable range for a storage layer."
	}

	return delta
}
