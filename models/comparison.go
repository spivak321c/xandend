package models

import "time"

// CrossChainComparison compares Xandeum vs Solana metrics
type CrossChainComparison struct {
	Timestamp      time.Time         `json:"timestamp"`
	Xandeum        XandeumMetrics    `json:"xandeum"`
	Solana         SolanaMetrics     `json:"solana"`
	PerformanceDelta PerformanceDelta `json:"performance_delta"`
}

// XandeumMetrics for the storage layer
type XandeumMetrics struct {
	StoragePowerIndex  float64 `json:"storage_power_index"` // Custom metric: sqrt(nodes * storage_pb)
	TotalNodes         int     `json:"total_nodes"`
	TotalStoragePB     float64 `json:"total_storage_pb"`
	AverageLatency     int64   `json:"average_latency"`
	NetworkHealth      float64 `json:"network_health"`
	DataAvailability   float64 `json:"data_availability"` // Percentage
}

// SolanaMetrics for the L1 layer
type SolanaMetrics struct {
	StakePowerIndex    float64 `json:"stake_power_index"` // Total active stake
	TotalValidators    int     `json:"total_validators"`
	TotalStake         int64   `json:"total_stake"` // In lamports
	AverageSlotTime    int64   `json:"average_slot_time"` // ms
	TPS                int64   `json:"tps"`
	NetworkHealth      float64 `json:"network_health"`
}

// PerformanceDelta shows differences
type PerformanceDelta struct {
	LatencyDifference  int64   `json:"latency_difference"` // Xandeum - Solana (ms)
	HealthDifference   float64 `json:"health_difference"`  // Xandeum - Solana (%)
	Summary            string  `json:"summary"`
	Interpretation     string  `json:"interpretation"`
}