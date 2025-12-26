package models

// NetworkTopology represents the gossip network graph
type NetworkTopology struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
	Stats TopologyStats  `json:"stats"`
}

// TopologyNode represents a node in the network graph
type TopologyNode struct {
	ID       string   `json:"id"`
	Address  string   `json:"address"`
	Status   string   `json:"status"`
	Country  string   `json:"country"`
	City     string   `json:"city"`
	Lat      float64  `json:"lat"`
	Lon      float64  `json:"lon"`
	Version  string   `json:"version"`
	PeerCount int     `json:"peer_count"`
	Peers    []string `json:"peers"` // ADD THIS - list of peer IDs
}

// TopologyEdge represents a connection between nodes
type TopologyEdge struct {
	Source   string `json:"source"` // Node ID
	Target   string `json:"target"` // Node ID
	Type     string `json:"type"`   // "local" (same region) or "bridge" (cross-region)
	Strength int    `json:"strength"` // 1-10, based on communication frequency
}

// TopologyStats provides graph metrics
type TopologyStats struct {
	TotalConnections   int     `json:"total_connections"`
	LocalConnections   int     `json:"local_connections"`
	BridgeConnections  int     `json:"bridge_connections"`
	AverageConnections float64 `json:"average_connections_per_node"`
	NetworkDensity     float64 `json:"network_density"` // 0-1
	LargestComponent   int     `json:"largest_component"` // Nodes in biggest connected cluster
}

// RegionalCluster groups nodes by region
type RegionalCluster struct {
	Region     string   `json:"region"`      // "North America", "Europe", etc.
	NodeCount  int      `json:"node_count"`
	NodeIDs    []string `json:"node_ids"`
	InternalEdges int   `json:"internal_edges"` // Connections within region
	ExternalEdges int   `json:"external_edges"` // Connections to other regions
}