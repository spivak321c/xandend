package models

import "time"

// Alert represents a monitoring alert configuration
type Alert struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	RuleType    string                 `json:"rule_type"` // "node_status", "network_health", "storage_threshold", "latency_spike"
	Conditions  map[string]interface{} `json:"conditions"`
	Actions     []AlertAction          `json:"actions"`
	Cooldown    int                    `json:"cooldown_minutes"` // Minimum time between alerts
	LastFired   time.Time              `json:"last_fired"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// AlertAction defines what happens when an alert triggers
type AlertAction struct {
	Type   string                 `json:"type"` // "webhook", "discord", "email"
	Config map[string]interface{} `json:"config"`
}

// AlertHistory tracks when alerts fire
type AlertHistory struct {
	ID          string                 `json:"id"`
	AlertID     string                 `json:"alert_id"`
	AlertName   string                 `json:"alert_name"`
	Timestamp   time.Time              `json:"timestamp"`
	Condition   string                 `json:"condition"`
	TriggeredBy map[string]interface{} `json:"triggered_by"`
	Success     bool                   `json:"success"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
}

// AlertConditions - Common condition structures
type NodeStatusCondition struct {
	NodeID string `json:"node_id,omitempty"` // Empty = any node
	Status string `json:"status"`             // "offline", "warning"
}

type NetworkHealthCondition struct {
	Operator  string  `json:"operator"` // "lt", "gt", "eq"
	Threshold float64 `json:"threshold"`
}

type StorageThresholdCondition struct {
	Type      string  `json:"type"`      // "total", "used", "percent"
	Operator  string  `json:"operator"`  // "lt", "gt"
	Threshold float64 `json:"threshold"` // In PB or percentage
}

type LatencySpikeCondition struct {
	NodeID    string `json:"node_id,omitempty"`
	Threshold int64  `json:"threshold"` // ms
}