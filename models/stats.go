package models

import "time"

// NetworkStats represents aggregated network statistics
type NetworkStats struct {
	TotalNodes   int `json:"total_nodes"`
	OnlineNodes  int `json:"online_nodes"`
	WarningNodes int `json:"warning_nodes"`
	OfflineNodes int `json:"offline_nodes"`

	TotalStorage float64 `json:"total_storage_pb"` // Petabytes
	UsedStorage  float64 `json:"used_storage_pb"`  // Petabytes

	AverageUptime      float64 `json:"average_uptime"`
	AveragePerformance float64 `json:"average_performance"`

	TotalStake int64 `json:"total_stake"`

	NetworkHealth float64 `json:"network_health"` // 0-100

	LastUpdated time.Time `json:"last_updated"`
}
