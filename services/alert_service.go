package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"xand/models"
)

type AlertService struct {
	alerts      map[string]*models.Alert
	history     []*models.AlertHistory
	alertsMutex sync.RWMutex
	cache       *CacheService
	mongo       *MongoDBService
	discordBot  *DiscordBotService // NEW

	stopChan    chan struct{}
}

func NewAlertService(cache *CacheService, mongo *MongoDBService, discordBot *DiscordBotService) *AlertService {
	return &AlertService{
		alerts:     make(map[string]*models.Alert),
		history:    make([]*models.AlertHistory, 0),
		cache:      cache,
		mongo:      mongo,
		discordBot: discordBot, // NEW
		stopChan:   make(chan struct{}),
	}
}

func (as *AlertService) Start() {
	log.Println("Starting Alert Service...")
	ticker := time.NewTicker(30 * time.Second)
	
	go func() {
		for {
			select {
			case <-ticker.C:
				as.EvaluateAlerts()
			case <-as.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

func (as *AlertService) Stop() {
	close(as.stopChan)
}

// CreateAlert adds a new alert
func (as *AlertService) CreateAlert(alert *models.Alert) error {
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert_%d", time.Now().UnixNano())
	}
	alert.CreatedAt = time.Now()
	alert.UpdatedAt = time.Now()

	as.alertsMutex.Lock()
	as.alerts[alert.ID] = alert
	as.alertsMutex.Unlock()

	// Persist to MongoDB
	if as.mongo != nil && as.mongo.enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := as.mongo.InsertAlert(ctx, alert); err != nil {
			log.Printf("Failed to persist alert to MongoDB: %v", err)
		}
	}

	return nil
}

// GetAlert retrieves a specific alert
func (as *AlertService) GetAlert(id string) (*models.Alert, bool) {
	as.alertsMutex.RLock()
	defer as.alertsMutex.RUnlock()
	alert, exists := as.alerts[id]
	return alert, exists
}

// ListAlerts returns all alerts
func (as *AlertService) ListAlerts() []*models.Alert {
	as.alertsMutex.RLock()
	defer as.alertsMutex.RUnlock()
	
	alerts := make([]*models.Alert, 0, len(as.alerts))
	for _, a := range as.alerts {
		alerts = append(alerts, a)
	}
	return alerts
}

// UpdateAlert modifies an existing alert
func (as *AlertService) UpdateAlert(id string, updated *models.Alert) error {
	as.alertsMutex.Lock()
	defer as.alertsMutex.Unlock()
	
	if _, exists := as.alerts[id]; !exists {
		return fmt.Errorf("alert not found")
	}
	
	updated.ID = id
	updated.UpdatedAt = time.Now()
	as.alerts[id] = updated
	
	// Update in MongoDB
	if as.mongo != nil && as.mongo.enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := as.mongo.UpdateAlert(ctx, updated); err != nil {
			log.Printf("Failed to update alert in MongoDB: %v", err)
		}
	}
	
	return nil
}

// DeleteAlert removes an alert
func (as *AlertService) DeleteAlert(id string) error {
	as.alertsMutex.Lock()
	defer as.alertsMutex.Unlock()
	
	if _, exists := as.alerts[id]; !exists {
		return fmt.Errorf("alert not found")
	}
	
	delete(as.alerts, id)
	
	// Delete from MongoDB
	if as.mongo != nil && as.mongo.enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := as.mongo.DeleteAlert(ctx, id); err != nil {
			log.Printf("Failed to delete alert from MongoDB: %v", err)
		}
	}
	
	return nil
}

// GetHistory returns alert history
func (as *AlertService) GetHistory(limit int) []*models.AlertHistory {
	as.alertsMutex.RLock()
	defer as.alertsMutex.RUnlock()
	
	if limit <= 0 || limit > len(as.history) {
		limit = len(as.history)
	}
	
	start := len(as.history) - limit
	if start < 0 {
		start = 0
	}
	
	result := make([]*models.AlertHistory, limit)
	copy(result, as.history[start:])
	
	// Reverse to get newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	
	return result
}

// EvaluateAlerts checks all active alerts
func (as *AlertService) EvaluateAlerts() {
	as.alertsMutex.RLock()
	alerts := make([]*models.Alert, 0, len(as.alerts))
	for _, a := range as.alerts {
		if a.Enabled {
			alerts = append(alerts, a)
		}
	}
	as.alertsMutex.RUnlock()

	for _, alert := range alerts {
		// Check cooldown
		if time.Since(alert.LastFired) < time.Duration(alert.Cooldown)*time.Minute {
			continue
		}

		triggered, context := as.evaluateCondition(alert)
		if triggered {
			as.fireAlert(alert, context)
		}
	}
}

func (as *AlertService) evaluateCondition(alert *models.Alert) (bool, map[string]interface{}) {
	context := make(map[string]interface{})

	switch alert.RuleType {
	case "node_status":
		return as.evaluateNodeStatus(alert, context)
	case "network_health":
		return as.evaluateNetworkHealth(alert, context)
	case "storage_threshold":
		return as.evaluateStorageThreshold(alert, context)
	case "latency_spike":
		return as.evaluateLatencySpike(alert, context)
	default:
		return false, context
	}
}

func (as *AlertService) evaluateNodeStatus(alert *models.Alert, context map[string]interface{}) (bool, map[string]interface{}) {
	nodeID, _ := alert.Conditions["node_id"].(string)
	targetStatus, _ := alert.Conditions["status"].(string)

	nodes, _, found := as.cache.GetNodes(true)
	if !found {
		return false, context
	}

	for _, node := range nodes {
		if nodeID != "" && node.ID != nodeID {
			continue
		}
		
		if node.Status == targetStatus {
			context["node_id"] = node.ID
			context["node_address"] = node.Address
			context["status"] = node.Status
			return true, context
		}
	}

	return false, context
}

func (as *AlertService) evaluateNetworkHealth(alert *models.Alert, context map[string]interface{}) (bool, map[string]interface{}) {
	operator, _ := alert.Conditions["operator"].(string)
	threshold, _ := alert.Conditions["threshold"].(float64)

	stats, _, found := as.cache.GetNetworkStats(true)
	if !found {
		return false, context
	}

	context["network_health"] = stats.NetworkHealth
	context["threshold"] = threshold

	switch operator {
	case "lt":
		return stats.NetworkHealth < threshold, context
	case "gt":
		return stats.NetworkHealth > threshold, context
	case "eq":
		return stats.NetworkHealth == threshold, context
	}

	return false, context
}

func (as *AlertService) evaluateStorageThreshold(alert *models.Alert, context map[string]interface{}) (bool, map[string]interface{}) {
	storageType, _ := alert.Conditions["type"].(string)
	operator, _ := alert.Conditions["operator"].(string)
	threshold, _ := alert.Conditions["threshold"].(float64)

	stats, _, found := as.cache.GetNetworkStats(true)
	if !found {
		return false, context
	}

	var value float64
	switch storageType {
	case "total":
		value = stats.TotalStorage
	case "used":
		value = stats.UsedStorage
	case "percent":
		if stats.TotalStorage > 0 {
			value = (stats.UsedStorage / stats.TotalStorage) * 100
		}
	}

	context["storage_type"] = storageType
	context["value"] = value
	context["threshold"] = threshold

	switch operator {
	case "lt":
		return value < threshold, context
	case "gt":
		return value > threshold, context
	}

	return false, context
}

func (as *AlertService) evaluateLatencySpike(alert *models.Alert, context map[string]interface{}) (bool, map[string]interface{}) {
	nodeID, _ := alert.Conditions["node_id"].(string)
	threshold, _ := alert.Conditions["threshold"].(float64)

	nodes, _, found := as.cache.GetNodes(true)
	if !found {
		return false, context
	}

	for _, node := range nodes {
		if nodeID != "" && node.ID != nodeID {
			continue
		}
		
		if node.ResponseTime > int64(threshold) {
			context["node_id"] = node.ID
			context["latency"] = node.ResponseTime
			context["threshold"] = threshold
			return true, context
		}
	}

	return false, context
}

func (as *AlertService) fireAlert(alert *models.Alert, alertContext map[string]interface{}) {
	log.Printf("Alert triggered: %s", alert.Name)

	// Update last fired
	as.alertsMutex.Lock()
	alert.LastFired = time.Now()
	as.alertsMutex.Unlock()

	// Execute actions
	for _, action := range alert.Actions {
		success := as.executeAction(action, alert, alertContext)
		
		// Record history
		history := &models.AlertHistory{
			ID:          fmt.Sprintf("hist_%d", time.Now().UnixNano()),
			AlertID:     alert.ID,
			AlertName:   alert.Name,
			Timestamp:   time.Now(),
			Condition:   alert.RuleType,
			TriggeredBy: alertContext,
			Success:     success,
		}
		
		as.alertsMutex.Lock()
		as.history = append(as.history, history)
		if len(as.history) > 1000 {
			as.history = as.history[len(as.history)-1000:]
		}
		as.alertsMutex.Unlock()
		
		// Persist to MongoDB
		if as.mongo != nil && as.mongo.enabled {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := as.mongo.InsertAlertHistory(ctx, history); err != nil {
				log.Printf("Failed to persist alert history to MongoDB: %v", err)
			}
		}
	}
}

func (as *AlertService) executeAction(action models.AlertAction, alert *models.Alert, alertContext map[string]interface{}) bool {
	switch action.Type {
	case "webhook":
		return as.sendWebhook(action, alert, alertContext)
	case "discord":
		return as.sendDiscordAlert(alert, alertContext)
	// case "telegram":
	// 	return as.sendTelegramAlert(alert, alertContext)
	default:
		log.Printf("Unknown action type: %s", action.Type)
		return false
	}
}

func (as *AlertService) sendWebhook(action models.AlertAction, alert *models.Alert, alertContext map[string]interface{}) bool {
	url, _ := action.Config["url"].(string)
	if url == "" {
		log.Println("Webhook URL not provided")
		return false
	}

	payload := map[string]interface{}{
		"alert_id":    alert.ID,
		"alert_name":  alert.Name,
		"description": alert.Description,
		"rule_type":   alert.RuleType,
		"timestamp":   time.Now(),
		"context":     alertContext,
	}

	jsonData, _ := json.Marshal(payload)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Webhook error: %v", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// NEW: Discord bot integration
func (as *AlertService) sendDiscordAlert(alert *models.Alert, alertContext map[string]interface{}) bool {
	if as.discordBot == nil {
		log.Println("Discord bot not configured")
		return false
	}

	if err := as.discordBot.SendAlert(alert, alertContext); err != nil {
		log.Printf("Discord alert error: %v", err)
		return false
	}

	return true
}

// PLACEHOLDER: Telegram bot integration (commented out)
// func (as *AlertService) sendTelegramAlert(alert *models.Alert, alertContext map[string]interface{}) bool {
// 	if as.telegramBot == nil {
// 		log.Println("Telegram bot not configured")
// 		return false
// 	}
//
// 	if err := as.telegramBot.SendAlert(alert, alertContext); err != nil {
// 		log.Printf("Telegram alert error: %v", err)
// 		return false
// 	}
//
// 	return true
// }

// TestAlert sends a test alert without saving it
func (as *AlertService) TestAlert(alert *models.Alert) error {
	// Create test context based on alert type
	testContext := as.generateTestContext(alert)
	
	// Execute actions with test context
	for _, action := range alert.Actions {
		switch action.Type {
		case "webhook":
			as.sendWebhook(action, alert, testContext)
		case "discord":
			if as.discordBot != nil {
				if err := as.discordBot.SendTestAlert(alert, testContext); err != nil {
					return err
				}
			}
		// case "telegram":
		// 	if as.telegramBot != nil {
		// 		if err := as.telegramBot.SendTestAlert(alert, testContext); err != nil {
		// 			return err
		// 		}
		// 	}
		}
	}
	
	return nil
}

func (as *AlertService) generateTestContext(alert *models.Alert) map[string]interface{} {
	context := make(map[string]interface{})
	
	switch alert.RuleType {
	case "node_status":
		context["node_id"] = "test_node_12345"
		context["node_address"] = "192.168.1.100:6000"
		context["status"] = "offline"
		
	case "network_health":
		context["network_health"] = 85.5
		context["threshold"] = 90.0
		
	case "storage_threshold":
		context["storage_type"] = "percent"
		context["value"] = 92.5
		context["threshold"] = 90.0
		
	case "latency_spike":
		context["node_id"] = "test_node_12345"
		context["latency"] = int64(250)
		context["threshold"] = 200.0
	}
	
	return context
}

func (as *AlertService) LoadAlertsFromDB() error {
	if as.mongo == nil || !as.mongo.enabled {
		return nil
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	alerts, err := as.mongo.GetAllAlerts(ctx)
	if err != nil {
		return fmt.Errorf("failed to load alerts from MongoDB: %w", err)
	}
	
	as.alertsMutex.Lock()
	for _, alert := range alerts {
		as.alerts[alert.ID] = alert
	}
	as.alertsMutex.Unlock()
	
	log.Printf("Loaded %d alerts from MongoDB", len(alerts))
	return nil
}