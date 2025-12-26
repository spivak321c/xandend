package models

import "encoding/json"

// JSON-RPC 2.0 Request
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
}

// JSON-RPC 2.0 Response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// JSON-RPC 2.0 Error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ============================================
// get_version response
// ============================================
type VersionResponse struct {
	Version string `json:"version"`
}

// ============================================
// get-stats response - FIXED TO MATCH REAL API
// ============================================
type StatsResponse struct {
	// Storage metadata (flat structure, not nested)
	TotalBytes  int64 `json:"total_bytes"`
	TotalPages  int   `json:"total_pages"`
	LastUpdated int64 `json:"last_updated"`
	FileSize    int64 `json:"file_size"`
	CurrentIndex int  `json:"current_index"`
	
	// System stats (flat structure, not nested)
	CPUPercent      float64 `json:"cpu_percent"`
	RAMUsed         int64   `json:"ram_used"`
	RAMTotal        int64   `json:"ram_total"`
	Uptime          int64   `json:"uptime"`
	PacketsReceived int64   `json:"packets_received"`
	PacketsSent     int64   `json:"packets_sent"`
	ActiveStreams   int     `json:"active_streams"`
}

// Alias for backward compatibility
type PRPCStatsResponse = StatsResponse

// ============================================
// get-pods-with-stats response - FIXED
// ============================================
type PodsWithStatsResponse struct {
	Pods       []PodWithStats `json:"pods"`
	TotalCount int            `json:"total_count"`
}

type PodWithStats struct {
	Address              string  `json:"address"`
	Pubkey               string  `json:"pubkey"`                 // FIXED: was "pod_id"
	RpcPort              int     `json:"rpc_port"`
	IsPublic             bool    `json:"is_public"`
	Version              string  `json:"version"`
	LastSeenTimestamp    int64   `json:"last_seen_timestamp"`
	StorageCommitted     int64   `json:"storage_committed"`
	StorageUsed          int64   `json:"storage_used"`
	StorageUsagePercent  float64 `json:"storage_usage_percent"`
	Uptime               int64   `json:"uptime"`
}

// ============================================
// get-pods response (simpler version)
// ============================================
type PodsResponse struct {
	Pods       []Pod `json:"pods"`
	TotalCount int   `json:"total_count"`
}

type Pod struct {
	Address           string `json:"address"`
	Pubkey            string `json:"pubkey"`            // FIXED: was "pod_id"
	Version           string `json:"version"`
	LastSeenTimestamp int64  `json:"last_seen_timestamp"`
	
	// Optional fields (only in get-pods-with-stats)
	RpcPort             int     `json:"rpc_port,omitempty"`
	IsPublic            bool    `json:"is_public,omitempty"`
	StorageCommitted    int64   `json:"storage_committed,omitempty"`
	StorageUsed         int64   `json:"storage_used,omitempty"`
	StorageUsagePercent float64 `json:"storage_usage_percent,omitempty"`
	Uptime              int64   `json:"uptime,omitempty"`
}