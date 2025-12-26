package models

import "time"

type Node struct {
	// Identity - UPDATED: ID is now always the Pubkey
	ID      string   `json:"id"`      // Always the Pubkey once discovered
	Pubkey  string   `json:"pubkey"`  // Public key of the node
	
	// Multi-address support - NEW
	Addresses []NodeAddress `json:"addresses"` // All known addresses for this node
	
	// Primary address (for backward compatibility and main RPC endpoint)
	IP      string `json:"ip"`      // Primary IP
	Port    int    `json:"port"`    // Primary RPC port
	Address string `json:"address"` // Primary "IP:Port" for RPC
	
	Version string `json:"version"`

	// Status
	IsOnline  bool      `json:"is_online"`
	IsPublic  bool      `json:"is_public"`  // Whether node accepts public connections
	LastSeen  time.Time `json:"last_seen"`
	FirstSeen time.Time `json:"first_seen"`
	Status    string    `json:"status"` // "online", "warning", "offline"

	// Geo Estimation (of primary IP)
	Country string  `json:"country"`
	City    string  `json:"city"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`

	// Stats (Snapshot)
	CPUPercent float64 `json:"cpu_percent"`
	RAMUsed    int64   `json:"ram_used"`
	RAMTotal   int64   `json:"ram_total"`
	
	// Storage
	StorageCapacity     int64   `json:"storage_capacity"`
	StorageUsed         int64   `json:"storage_used"`
	StorageUsagePercent float64 `json:"storage_usage_percent"`

	UptimeSeconds   int64 `json:"uptime_seconds"`
	PacketsReceived int64 `json:"packets_received"`
	PacketsSent     int64 `json:"packets_sent"`

	// Metrics
	UptimeScore      float64 `json:"uptime_score"`
	PerformanceScore float64 `json:"performance_score"`
	ResponseTime     int64   `json:"response_time"` // ms
	Credits          int64   `json:"credits"`
	CreditsRank      int     `json:"credits_rank"`
	CreditsChange    int64   `json:"credits_change"`

	// History
	CallHistory  []bool `json:"-"`
	SuccessCalls int    `json:"-"`
	TotalCalls   int    `json:"-"`

	// Staking
	TotalStake  float64 `json:"total_stake"`
	Commission  float64 `json:"commission"`
	APY         float64 `json:"apy"`
	BoostFactor float64 `json:"boost_factor"`

	VersionStatus   string `json:"version_status"`
	IsUpgradeNeeded bool   `json:"is_upgrade_needed"`
	UpgradeSeverity string `json:"upgrade_severity"`
	UpgradeMessage  string `json:"upgrade_message"`
}

// NodeAddress represents a single address endpoint for a node
type NodeAddress struct {
	Address   string    `json:"address"`    // Full "IP:Port"
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	Type      string    `json:"type"`       // "rpc", "gossip", "unknown"
	IsPublic  bool      `json:"is_public"`
	LastSeen  time.Time `json:"last_seen"`
	IsWorking bool      `json:"is_working"` // Whether this address is currently reachable
}