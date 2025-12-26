package utils

import (
	"time"

	"xand/models"
)

// DetermineStatus updates the node's status based on latency and uptime
// func DetermineStatus(n *models.Node) {
// 	lastSeen := time.Since(n.LastSeen)

// 	// If node was just discovered (within last 5 minutes), be lenient
// 	justDiscovered := time.Since(n.FirstSeen) < 5*time.Minute

// 	// Check Offline triggers
// 	if lastSeen > 5*time.Minute {
// 		n.Status = "offline"
// 		return
// 	}

// 	// For newly discovered nodes, use relaxed criteria
// 	if justDiscovered {
// 		if n.IsOnline && lastSeen < 2*time.Minute {
// 			n.Status = "online"
// 			return
// 		}
// 	}

// 	// For established nodes, use strict criteria
// 	if n.UptimeScore < 85 {
// 		n.Status = "offline"
// 		return
// 	}

// 	// Check Online conditions
// 	if lastSeen < 2*time.Minute && n.UptimeScore > 95 && n.ResponseTime < 1000 {
// 		n.Status = "online"
// 		return
// 	}

// 	// Default fallback
// 	n.Status = "warning"
// }


// COMPLETELY REPLACE the DetermineStatus function:

// func DetermineStatus(n *models.Node) {
// 	lastSeen := time.Since(n.LastSeen)

// 	// If node was just discovered (within last 5 minutes), be lenient
// 	justDiscovered := time.Since(n.FirstSeen) < 5*time.Minute

// 	// CRITICAL: Check if node hasn't responded recently
// 	// Offline: No response in last 3 minutes
// 	if lastSeen > 3*time.Minute {
// 		n.Status = "offline"
// 		n.IsOnline = false
// 		return
// 	}

// 	// For newly discovered nodes, use relaxed criteria
// 	if justDiscovered {
// 		if n.IsOnline && lastSeen < 2*time.Minute {
// 			n.Status = "online"
// 			return
// 		}
// 	}

// 	// Check call history for reliability
// 	failureRate := 0.0
// 	if len(n.CallHistory) > 0 {
// 		failures := 0
// 		for _, success := range n.CallHistory {
// 			if !success {
// 				failures++
// 			}
// 		}
// 		failureRate = float64(failures) / float64(len(n.CallHistory))
// 	}

// 	// Offline: High failure rate OR very low uptime
// 	if failureRate > 0.5 || n.UptimeScore < 50 {
// 		n.Status = "offline"
// 		n.IsOnline = false
// 		return
// 	}

// 	// Warning: Moderate issues
// 	if failureRate > 0.2 || n.UptimeScore < 85 || n.ResponseTime > 2000 {
// 		n.Status = "warning"
// 		return
// 	}

// 	// Online: Good performance
// 	if lastSeen < 2*time.Minute && n.UptimeScore > 85 && n.ResponseTime < 1000 {
// 		n.Status = "online"
// 		return
// 	}

// 	// Default to warning if unclear
// 	n.Status = "warning"
// }
















func DetermineStatus(n *models.Node) {
	lastSeen := time.Since(n.LastSeen)

	// If node was just discovered (within last 5 minutes), be lenient
	justDiscovered := time.Since(n.FirstSeen) < 5*time.Minute

	// CRITICAL FIX: Be less aggressive about marking offline
	// Nodes in gossip network that haven't been directly contacted may still be online
	// Only mark offline if we've actually tried to contact them and failed
	
	// If we haven't tried to contact the node yet (TotalCalls == 0), use peer data
	if n.TotalCalls == 0 {
		// Node created from peer list, use last_seen from gossip
		if lastSeen > 10*time.Minute {
			n.Status = "offline"
			n.IsOnline = false
		} else if lastSeen > 5*time.Minute {
			n.Status = "warning"
		} else {
			n.Status = "online"
			n.IsOnline = true
		}
		return
	}

	// If we have tried to contact the node, use actual connectivity data
	if lastSeen > 5*time.Minute {
		n.Status = "offline"
		n.IsOnline = false
		return
	}

	// For newly discovered nodes, use relaxed criteria
	if justDiscovered {
		if n.IsOnline && lastSeen < 2*time.Minute {
			n.Status = "online"
			return
		}
	}

	// Check call history for reliability
	failureRate := 0.0
	if len(n.CallHistory) > 0 {
		failures := 0
		for _, success := range n.CallHistory {
			if !success {
				failures++
			}
		}
		failureRate = float64(failures) / float64(len(n.CallHistory))
	}

	// Offline: High failure rate OR very low uptime
	if failureRate > 0.7 || n.UptimeScore < 30 {
		n.Status = "offline"
		n.IsOnline = false
		return
	}

	// Warning: Moderate issues
	if failureRate > 0.3 || n.UptimeScore < 70 || n.ResponseTime > 2000 {
		n.Status = "warning"
		return
	}

	// Online: Good performance
	if lastSeen < 3*time.Minute && n.UptimeScore > 70 && n.ResponseTime < 1500 {
		n.Status = "online"
		return
	}

	// Default to warning if unclear
	n.Status = "warning"
}









// CalculateScore computes the node's performance score (0-100)
func CalculateScore(n *models.Node) {
	// 1. Response Time (40%)
	// <100ms: 40
	// 100-500ms: 30
	// 500-1000ms: 20
	// >1000ms: 10
	var scoreResponse float64
	if n.ResponseTime < 100 {
		scoreResponse = 40
	} else if n.ResponseTime < 500 {
		scoreResponse = 30
	} else if n.ResponseTime < 1000 {
		scoreResponse = 20
	} else {
		scoreResponse = 10
	}

	// 2. Success Rate (30%)
	// (successful_calls / 10) * 30
	successCount := 0
	for _, ok := range n.CallHistory {
		if ok {
			successCount++
		}
	}
	n.SuccessCalls = successCount

	var scoreSuccess float64
	if len(n.CallHistory) > 0 {
		rate := float64(successCount) / float64(len(n.CallHistory))
		scoreSuccess = rate * 30
	}

	// 3. Uptime (30%)
	// (uptime_percentage / 100) * 30
	scoreUptime := (n.UptimeScore / 100.0) * 30

	n.PerformanceScore = scoreResponse + scoreSuccess + scoreUptime

	// Cap at 100
	if n.PerformanceScore > 100 {
		n.PerformanceScore = 100
	}
}
