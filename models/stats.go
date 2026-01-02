package models

import "time"

// NetworkStats represents aggregated network statistics
type NetworkStats struct {
    // RENAME: TotalNodes → TotalPods (semantic fix)
    TotalPods    int `json:"total_pods"`    // Count of unique pubkeys
    TotalNodes   int `json:"total_nodes"`   // Count of all IPs
    
    OnlineNodes  int `json:"online_nodes"`
    WarningNodes int `json:"warning_nodes"`
    OfflineNodes int `json:"offline_nodes"`

    // NEW: Public/Private breakdown
    TotalPublicNodes  int `json:"total_public_nodes"`   // All public (online+warning+offline)
    TotalPrivateNodes int `json:"total_private_nodes"`  // All private (online+warning+offline)
    OnlinePublicNodes  int `json:"online_public_nodes"`  // NEW
    OnlinePrivateNodes int `json:"online_private_nodes"` // NEW

    // Storage metrics
    TotalStorage float64 `json:"total_storage_bytes"`
    UsedStorage  float64 `json:"used_storage_bytes"`

    // NEW METRIC
    AvgStorageCommittedPerPodBytes float64 `json:"average_storage_committed_per_pod_bytes"`

    AverageUptime      float64 `json:"average_uptime"`
    AveragePerformance float64 `json:"average_performance"`

    TotalStake int64 `json:"total_stake"`

    NetworkHealth float64 `json:"network_health"`

    LastUpdated time.Time `json:"last_updated"`
}

