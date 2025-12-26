package models

import "time"

// DailyHealthSnapshot represents aggregated daily statistics
type DailyHealthSnapshot struct {
	Date             time.Time `bson:"date" json:"date"`
	AvgHealth        float64   `bson:"avg_health" json:"avg_health"`
	AvgOnlineNodes   float64   `bson:"avg_online_nodes" json:"avg_online_nodes"`
	AvgTotalNodes    float64   `bson:"avg_total_nodes" json:"avg_total_nodes"`
	MaxStorageUsed   float64   `bson:"max_storage_used" json:"max_storage_used"`
	MinNetworkHealth float64   `bson:"min_network_health" json:"min_network_health"`
	MaxNetworkHealth float64   `bson:"max_network_health" json:"max_network_health"`
}

// NodeUptimeReport represents a node's uptime statistics
type NodeUptimeReport struct {
	NodeID      string  `bson:"_id" json:"node_id"`
	AvgUptime   float64 `bson:"avg_uptime" json:"avg_uptime"`
	AvgPerf     float64 `bson:"avg_perf" json:"avg_performance"`
	TotalChecks int     `bson:"total_checks" json:"total_checks"`
	OnlineCount int     `bson:"online_count" json:"online_count"`
}

// StorageGrowthReport shows storage growth over time
type StorageGrowthReport struct {
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	StartStoragePB   float64   `json:"start_storage_pb"`
	EndStoragePB     float64   `json:"end_storage_pb"`
	GrowthPB         float64   `json:"growth_pb"`
	GrowthPercentage float64   `json:"growth_percentage"`
	GrowthRatePerDay float64   `json:"growth_rate_per_day"`
	DaysAnalyzed     int       `json:"days_analyzed"`
}

// NodeRegistryEntry tracks when nodes first appeared
type NodeRegistryEntry struct {
	NodeID    string    `bson:"node_id" json:"node_id"`
	FirstSeen time.Time `bson:"first_seen" json:"first_seen"`
}

// WeekStats represents statistics for a week period
type WeekStats struct {
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	AvgHealth       float64   `json:"avg_health"`
	AvgOnlineNodes  float64   `json:"avg_online_nodes"`
	AvgTotalNodes   float64   `json:"avg_total_nodes"`
	AvgStorageUsed  float64   `json:"avg_storage_used"`
	AvgStorageTotal float64   `json:"avg_storage_total"`
}

// WeeklyComparison compares two weeks
type WeeklyComparison struct {
	ThisWeek      WeekStats `json:"this_week"`
	LastWeek      WeekStats `json:"last_week"`
	HealthChange  float64   `json:"health_change"`
	StorageChange float64   `json:"storage_change"`
	NodesChange   float64   `json:"nodes_change"`
}



// NodeGraveyardEntry represents an inactive/dead node
type NodeGraveyardEntry struct {
	NodeID       string                `bson:"node_id" json:"node_id"`
	FirstSeen    time.Time             `bson:"first_seen" json:"first_seen"`
	LastSnapshot *NodeSnapshot         `bson:"last_snapshot" json:"last_snapshot,omitempty"`
	DaysSinceSeen int                  `json:"days_since_seen"`
	Status       string                `json:"status"` // "inactive", "dead"
}