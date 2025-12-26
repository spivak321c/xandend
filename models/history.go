package models

import "time"

// NetworkSnapshot represents network state at a point in time
type NetworkSnapshot struct {
	Timestamp        time.Time `json:"timestamp"`
	TotalNodes       int       `json:"total_nodes"`
	OnlineNodes      int       `json:"online_nodes"`
	WarningNodes     int       `json:"warning_nodes"`
	OfflineNodes     int       `json:"offline_nodes"`
	TotalStoragePB   float64   `json:"total_storage_pb"`
	UsedStoragePB    float64   `json:"used_storage_pb"`
	AverageLatency   int64     `json:"average_latency"`
	NetworkHealth    float64   `json:"network_health"`
	TotalStake       int64     `json:"total_stake"`
	AverageUptime    float64   `json:"average_uptime"`
	AveragePerf      float64   `json:"average_performance"`
}

// NodeSnapshot represents a single node's state at a point in time
type NodeSnapshot struct {
	Timestamp        time.Time `json:"timestamp"`
	NodeID           string    `json:"node_id"`
	Status           string    `json:"status"`
	ResponseTime     int64     `json:"response_time"`
	CPUPercent       float64   `json:"cpu_percent"`
	RAMUsed          int64     `json:"ram_used"`
	StorageUsed      int64     `json:"storage_used"`
	UptimeScore      float64   `json:"uptime_score"`
	PerformanceScore float64   `json:"performance_score"`
}

// TimeSeriesQuery for fetching historical data
type TimeSeriesQuery struct {
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Interval   string    `json:"interval"`   // "1m", "5m", "1h", "1d"
	Metric     string    `json:"metric"`     // "storage", "latency", "health"
	NodeID     string    `json:"node_id,omitempty"`
	Aggregation string   `json:"aggregation"` // "avg", "max", "min", "sum"
}

// CapacityForecast for predicting storage saturation
type CapacityForecast struct {
	CurrentUsagePB     float64   `json:"current_usage_pb"`
	CurrentCapacityPB  float64   `json:"current_capacity_pb"`
	GrowthRatePBPerDay float64   `json:"growth_rate_pb_per_day"`
	DaysToSaturation   int       `json:"days_to_saturation"`
	SaturationDate     time.Time `json:"saturation_date"`
	Confidence         float64   `json:"confidence"` // 0-100
}