package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"xand/config"
	"xand/models"
)

type MongoDBService struct {
	client   *mongo.Client
	db       *mongo.Database
	enabled  bool
}

const (
	CollectionNetworkSnapshots = "network_snapshots"
	CollectionNodeSnapshots    = "node_snapshots"
	CollectionAlertHistory     = "alert_history"
	CollectionNodeRegistry     = "node_registry" // Track when nodes first appeared
	CollectionAlerts           = "alerts" 
)

func NewMongoDBService(cfg *config.Config) (*MongoDBService, error) {
	if !cfg.MongoDB.Enabled {
		log.Println("MongoDB is disabled in configuration")
		return &MongoDBService{enabled: false}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoDB.URI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(cfg.MongoDB.Database)

	service := &MongoDBService{
		client:  client,
		db:      db,
		enabled: true,
	}

	// Create indexes
	if err := service.createIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	}

	log.Printf("MongoDB connected successfully to database: %s", cfg.MongoDB.Database)
	return service, nil
}

func (m *MongoDBService) createIndexes(ctx context.Context) error {
	if !m.enabled {
		return nil
	}

	// Network Snapshots: Index on timestamp (descending for recent queries)
	_, err := m.db.Collection(CollectionNetworkSnapshots).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "timestamp", Value: -1}},
		Options: options.Index().SetName("timestamp_desc"),
	})
	if err != nil {
		return err
	}

	// Node Snapshots: Compound index on node_id and timestamp
	_, err = m.db.Collection(CollectionNodeSnapshots).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "node_id", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("node_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("timestamp_desc"),
		},
	})
	if err != nil {
		return err
	}

	// Node Registry: Index on node_id
	_, err = m.db.Collection(CollectionNodeRegistry).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "node_id", Value: 1}},
		Options: options.Index().SetName("node_id").SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Alert History: Index on timestamp
	_, err = m.db.Collection(CollectionAlertHistory).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "timestamp", Value: -1}},
		Options: options.Index().SetName("timestamp_desc"),
	})

	return err
}

func (m *MongoDBService) Close() error {
	if !m.enabled || m.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}

// ============================================
// INSERT METHODS
// ============================================

func (m *MongoDBService) InsertNetworkSnapshot(ctx context.Context, snapshot *models.NetworkSnapshot) error {
	if !m.enabled {
		return nil
	}
	_, err := m.db.Collection(CollectionNetworkSnapshots).InsertOne(ctx, snapshot)
	return err
}

func (m *MongoDBService) InsertNodeSnapshot(ctx context.Context, snapshot *models.NodeSnapshot) error {
	if !m.enabled {
		return nil
	}
	_, err := m.db.Collection(CollectionNodeSnapshots).InsertOne(ctx, snapshot)
	return err
}

func (m *MongoDBService) InsertAlertHistory(ctx context.Context, alert *models.AlertHistory) error {
	if !m.enabled {
		return nil
	}
	_, err := m.db.Collection(CollectionAlertHistory).InsertOne(ctx, alert)
	return err
}

func (m *MongoDBService) RegisterNode(ctx context.Context, nodeID string, firstSeen time.Time) error {
	if !m.enabled {
		return nil
	}
	
	filter := bson.M{"node_id": nodeID}
	update := bson.M{
		"$setOnInsert": bson.M{
			"node_id":    nodeID,
			"first_seen": firstSeen,
		},
	}
	opts := options.Update().SetUpsert(true)
	
	_, err := m.db.Collection(CollectionNodeRegistry).UpdateOne(ctx, filter, update, opts)
	return err
}

// ============================================
// QUERY METHODS - These power the analytics!
// ============================================

// 1. "Show me network health for every day in December"
func (m *MongoDBService) GetDailyNetworkHealth(ctx context.Context, year int, month time.Month) ([]models.DailyHealthSnapshot, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	startDate := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0) // First day of next month

	pipeline := mongo.Pipeline{
		// Filter by date range
		{{Key: "$match", Value: bson.M{
			"timestamp": bson.M{
				"$gte": startDate,
				"$lt":  endDate,
			},
		}}},
		// Group by day
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$timestamp"},
				"month": bson.M{"$month": "$timestamp"},
				"day":   bson.M{"$dayOfMonth": "$timestamp"},
			},
			"avg_health":        bson.M{"$avg": "$network_health"},
			"avg_online_nodes":  bson.M{"$avg": "$online_nodes"},
			"avg_total_nodes":   bson.M{"$avg": "$total_nodes"},
			"max_storage_used":  bson.M{"$max": "$used_storage_pb"},
			"min_network_health": bson.M{"$min": "$network_health"},
			"max_network_health": bson.M{"$max": "$network_health"},
			"date":              bson.M{"$first": "$timestamp"},
		}}},
		// Sort by date
		{{Key: "$sort", Value: bson.M{"_id.year": 1, "_id.month": 1, "_id.day": 1}}},
	}

	cursor, err := m.db.Collection(CollectionNetworkSnapshots).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.DailyHealthSnapshot
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// 2. "Find all nodes that had >90% uptime last month"
func (m *MongoDBService) GetHighUptimeNodes(ctx context.Context, minUptime float64, daysBack int) ([]models.NodeUptimeReport, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	startDate := time.Now().AddDate(0, 0, -daysBack)

	pipeline := mongo.Pipeline{
		// Filter by date range
		{{Key: "$match", Value: bson.M{
			"timestamp": bson.M{"$gte": startDate},
		}}},
		// Group by node
		{{Key: "$group", Value: bson.M{
			"_id":          "$node_id",
			"avg_uptime":   bson.M{"$avg": "$uptime_score"},
			"avg_perf":     bson.M{"$avg": "$performance_score"},
			"total_checks": bson.M{"$sum": 1},
			"online_count": bson.M{"$sum": bson.M{"$cond": []interface{}{
				bson.M{"$eq": []interface{}{"$status", "online"}},
				1,
				0,
			}}},
		}}},
		// Filter nodes with high uptime
		{{Key: "$match", Value: bson.M{
			"avg_uptime": bson.M{"$gte": minUptime},
		}}},
		// Sort by uptime descending
		{{Key: "$sort", Value: bson.M{"avg_uptime": -1}}},
	}

	cursor, err := m.db.Collection(CollectionNodeSnapshots).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.NodeUptimeReport
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// 3. "What's the 30-day storage growth rate?"
func (m *MongoDBService) GetStorageGrowthRate(ctx context.Context, days int) (*models.StorageGrowthReport, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	startDate := time.Now().AddDate(0, 0, -days)

	// Get first snapshot in range
	var firstSnapshot models.NetworkSnapshot
	err := m.db.Collection(CollectionNetworkSnapshots).FindOne(ctx, bson.M{
		"timestamp": bson.M{"$gte": startDate},
	}, options.FindOne().SetSort(bson.M{"timestamp": 1})).Decode(&firstSnapshot)
	if err != nil {
		return nil, err
	}

	// Get latest snapshot
	var lastSnapshot models.NetworkSnapshot
	err = m.db.Collection(CollectionNetworkSnapshots).FindOne(ctx, bson.M{},
		options.FindOne().SetSort(bson.M{"timestamp": -1})).Decode(&lastSnapshot)
	if err != nil {
		return nil, err
	}

	// Calculate growth
	storageDiff := lastSnapshot.UsedStoragePB - firstSnapshot.UsedStoragePB
	timeDiff := lastSnapshot.Timestamp.Sub(firstSnapshot.Timestamp).Hours() / 24 // days
	
	var growthRatePerDay float64
	if timeDiff > 0 {
		growthRatePerDay = storageDiff / timeDiff
	}

	var growthPercentage float64
	if firstSnapshot.UsedStoragePB > 0 {
		growthPercentage = (storageDiff / firstSnapshot.UsedStoragePB) * 100
	}

	return &models.StorageGrowthReport{
		StartDate:         firstSnapshot.Timestamp,
		EndDate:           lastSnapshot.Timestamp,
		StartStoragePB:    firstSnapshot.UsedStoragePB,
		EndStoragePB:      lastSnapshot.UsedStoragePB,
		GrowthPB:          storageDiff,
		GrowthPercentage:  growthPercentage,
		GrowthRatePerDay:  growthRatePerDay,
		DaysAnalyzed:      int(timeDiff),
	}, nil
}

// 4. "Which nodes joined the network in the last week?"
func (m *MongoDBService) GetRecentlyJoinedNodes(ctx context.Context, daysBack int) ([]models.NodeRegistryEntry, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	startDate := time.Now().AddDate(0, 0, -daysBack)

	filter := bson.M{
		"first_seen": bson.M{"$gte": startDate},
	}

	cursor, err := m.db.Collection(CollectionNodeRegistry).Find(ctx, filter,
		options.Find().SetSort(bson.M{"first_seen": -1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.NodeRegistryEntry
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// 5. "Compare this week's performance to last week"
func (m *MongoDBService) CompareWeeklyPerformance(ctx context.Context) (*models.WeeklyComparison, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	now := time.Now()
	thisWeekStart := now.AddDate(0, 0, -7)
	lastWeekStart := now.AddDate(0, 0, -14)

	// Get this week's average
	thisWeekStats, err := m.getWeekStats(ctx, thisWeekStart, now)
	if err != nil {
		return nil, err
	}

	// Get last week's average
	lastWeekStats, err := m.getWeekStats(ctx, lastWeekStart, thisWeekStart)
	if err != nil {
		return nil, err
	}

	// Calculate changes
	healthChange := thisWeekStats.AvgHealth - lastWeekStats.AvgHealth
	storageChange := thisWeekStats.AvgStorageUsed - lastWeekStats.AvgStorageUsed
	nodesChange := thisWeekStats.AvgOnlineNodes - lastWeekStats.AvgOnlineNodes

	return &models.WeeklyComparison{
		ThisWeek:      *thisWeekStats,
		LastWeek:      *lastWeekStats,
		HealthChange:  healthChange,
		StorageChange: storageChange,
		NodesChange:   nodesChange,
	}, nil
}

func (m *MongoDBService) getWeekStats(ctx context.Context, start, end time.Time) (*models.WeekStats, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"timestamp": bson.M{
				"$gte": start,
				"$lt":  end,
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":               nil,
			"avg_health":        bson.M{"$avg": "$network_health"},
			"avg_online_nodes":  bson.M{"$avg": "$online_nodes"},
			"avg_total_nodes":   bson.M{"$avg": "$total_nodes"},
			"avg_storage_used":  bson.M{"$avg": "$used_storage_pb"},
			"avg_storage_total": bson.M{"$avg": "$total_storage_pb"},
		}}},
	}

	cursor, err := m.db.Collection(CollectionNetworkSnapshots).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result struct {
		AvgHealth       float64 `bson:"avg_health"`
		AvgOnlineNodes  float64 `bson:"avg_online_nodes"`
		AvgTotalNodes   float64 `bson:"avg_total_nodes"`
		AvgStorageUsed  float64 `bson:"avg_storage_used"`
		AvgStorageTotal float64 `bson:"avg_storage_total"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
	}

	return &models.WeekStats{
		StartDate:       start,
		EndDate:         end,
		AvgHealth:       result.AvgHealth,
		AvgOnlineNodes:  result.AvgOnlineNodes,
		AvgTotalNodes:   result.AvgTotalNodes,
		AvgStorageUsed:  result.AvgStorageUsed,
		AvgStorageTotal: result.AvgStorageTotal,
	}, nil
}





// ============================================
// ADD THESE METHODS TO: backend/services/mongodb_service.go
// ============================================

// GetNetworkSnapshotsRange retrieves network snapshots within a time range
func (m *MongoDBService) GetNetworkSnapshotsRange(ctx context.Context, start, end time.Time) ([]models.NetworkSnapshot, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	filter := bson.M{
		"timestamp": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	opts := options.Find().SetSort(bson.M{"timestamp": 1})
	cursor, err := m.db.Collection(CollectionNetworkSnapshots).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var snapshots []models.NetworkSnapshot
	if err := cursor.All(ctx, &snapshots); err != nil {
		return nil, err
	}

	return snapshots, nil
}

// GetNodeSnapshotsRange retrieves node snapshots within a time range
func (m *MongoDBService) GetNodeSnapshotsRange(ctx context.Context, nodeID string, start, end time.Time) ([]models.NodeSnapshot, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	filter := bson.M{
		"node_id": nodeID,
		"timestamp": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	opts := options.Find().SetSort(bson.M{"timestamp": 1})
	cursor, err := m.db.Collection(CollectionNodeSnapshots).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var snapshots []models.NodeSnapshot
	if err := cursor.All(ctx, &snapshots); err != nil {
		return nil, err
	}

	return snapshots, nil
}

// GetLatestNetworkSnapshot gets the most recent network snapshot
func (m *MongoDBService) GetLatestNetworkSnapshot(ctx context.Context) (*models.NetworkSnapshot, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	var snapshot models.NetworkSnapshot
	opts := options.FindOne().SetSort(bson.M{"timestamp": -1})
	
	err := m.db.Collection(CollectionNetworkSnapshots).FindOne(ctx, bson.M{}, opts).Decode(&snapshot)
	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// GetOldestNetworkSnapshot gets the oldest network snapshot
func (m *MongoDBService) GetOldestNetworkSnapshot(ctx context.Context) (*models.NetworkSnapshot, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	var snapshot models.NetworkSnapshot
	opts := options.FindOne().SetSort(bson.M{"timestamp": 1})
	
	err := m.db.Collection(CollectionNetworkSnapshots).FindOne(ctx, bson.M{}, opts).Decode(&snapshot)
	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// CountNetworkSnapshots returns total number of snapshots stored
func (m *MongoDBService) CountNetworkSnapshots(ctx context.Context) (int64, error) {
	if !m.enabled {
		return 0, fmt.Errorf("MongoDB not enabled")
	}

	count, err := m.db.Collection(CollectionNetworkSnapshots).CountDocuments(ctx, bson.M{})
	return count, err
}

// CountNodeSnapshots returns total number of node snapshots stored
func (m *MongoDBService) CountNodeSnapshots(ctx context.Context) (int64, error) {
	if !m.enabled {
		return 0, fmt.Errorf("MongoDB not enabled")
	}

	count, err := m.db.Collection(CollectionNodeSnapshots).CountDocuments(ctx, bson.M{})
	return count, err
}

// GetAllRegisteredNodes returns all nodes that have ever been seen
func (m *MongoDBService) GetAllRegisteredNodes(ctx context.Context) ([]models.NodeRegistryEntry, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	cursor, err := m.db.Collection(CollectionNodeRegistry).Find(ctx, bson.M{},
		options.Find().SetSort(bson.M{"first_seen": -1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var entries []models.NodeRegistryEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// DeleteOldSnapshots deletes snapshots older than the specified duration
// Useful for data retention policies
func (m *MongoDBService) DeleteOldSnapshots(ctx context.Context, olderThan time.Duration) error {
	if !m.enabled {
		return fmt.Errorf("MongoDB not enabled")
	}

	cutoffTime := time.Now().Add(-olderThan)
	filter := bson.M{
		"timestamp": bson.M{"$lt": cutoffTime},
	}

	// Delete old network snapshots
	netResult, err := m.db.Collection(CollectionNetworkSnapshots).DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	// Delete old node snapshots
	nodeResult, err := m.db.Collection(CollectionNodeSnapshots).DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	log.Printf("Deleted %d network snapshots and %d node snapshots older than %v",
		netResult.DeletedCount, nodeResult.DeletedCount, olderThan)

	return nil
}

// GetDatabaseStats returns statistics about the MongoDB collections
func (m *MongoDBService) GetDatabaseStats(ctx context.Context) (map[string]interface{}, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	stats := make(map[string]interface{})

	// Count documents in each collection
	netCount, _ := m.db.Collection(CollectionNetworkSnapshots).CountDocuments(ctx, bson.M{})
	nodeCount, _ := m.db.Collection(CollectionNodeSnapshots).CountDocuments(ctx, bson.M{})
	alertCount, _ := m.db.Collection(CollectionAlertHistory).CountDocuments(ctx, bson.M{})
	registryCount, _ := m.db.Collection(CollectionNodeRegistry).CountDocuments(ctx, bson.M{})

	stats["network_snapshots_count"] = netCount
	stats["node_snapshots_count"] = nodeCount
	stats["alert_history_count"] = alertCount
	stats["registered_nodes_count"] = registryCount

	// Get oldest and newest snapshots
	oldest, err := m.GetOldestNetworkSnapshot(ctx)
	if err == nil {
		stats["oldest_snapshot"] = oldest.Timestamp
	}

	latest, err := m.GetLatestNetworkSnapshot(ctx)
	if err == nil {
		stats["latest_snapshot"] = latest.Timestamp
	}

	if oldest != nil && latest != nil {
		duration := latest.Timestamp.Sub(oldest.Timestamp)
		stats["data_span_days"] = duration.Hours() / 24
	}

	return stats, nil
}










// GetInactiveNodes returns nodes that haven't been seen in X days
func (m *MongoDBService) GetInactiveNodes(ctx context.Context, inactiveDays int) ([]models.NodeGraveyardEntry, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}

	cutoffTime := time.Now().AddDate(0, 0, -inactiveDays)

	// Use aggregation to find nodes with no recent snapshots
	pipeline := mongo.Pipeline{
		// Get all registered nodes
		{{Key: "$match", Value: bson.M{
			"first_seen": bson.M{"$lt": cutoffTime}, // Only nodes that existed before cutoff
		}}},
		// Lookup recent snapshots
		{{Key: "$lookup", Value: bson.M{
			"from": CollectionNodeSnapshots,
			"let":  bson.M{"nodeId": "$node_id"},
			"pipeline": bson.A{
				bson.M{"$match": bson.M{
					"$expr": bson.M{"$eq": bson.A{"$node_id", "$$nodeId"}},
					"timestamp": bson.M{"$gte": cutoffTime},
				}},
			},
			"as": "recent_snapshots",
		}}},
		// Filter to nodes with no recent activity
		{{Key: "$match", Value: bson.M{
			"recent_snapshots": bson.M{"$size": 0},
		}}},
		// Get their last known snapshot
		{{Key: "$lookup", Value: bson.M{
			"from": CollectionNodeSnapshots,
			"let":  bson.M{"nodeId": "$node_id"},
			"pipeline": bson.A{
				bson.M{"$match": bson.M{
					"$expr": bson.M{"$eq": bson.A{"$node_id", "$$nodeId"}},
				}},
				bson.M{"$sort": bson.M{"timestamp": -1}},
				bson.M{"$limit": 1},
			},
			"as": "last_snapshot",
		}}},
		// Unwind last snapshot
		{{Key: "$unwind", Value: bson.M{
			"path":                       "$last_snapshot",
			"preserveNullAndEmptyArrays": true,
		}}},
		// Sort by last seen
		{{Key: "$sort", Value: bson.M{"last_snapshot.timestamp": -1}}},
	}

	cursor, err := m.db.Collection(CollectionNodeRegistry).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.NodeGraveyardEntry
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}



func (m *MongoDBService) InsertAlert(ctx context.Context, alert *models.Alert) error {
	if !m.enabled {
		return nil
	}
	_, err := m.db.Collection(CollectionAlerts).InsertOne(ctx, alert)
	return err
}

func (m *MongoDBService) UpdateAlert(ctx context.Context, alert *models.Alert) error {
	if !m.enabled {
		return nil
	}
	
	filter := bson.M{"id": alert.ID}
	update := bson.M{"$set": alert}
	
	_, err := m.db.Collection(CollectionAlerts).UpdateOne(ctx, filter, update)
	return err
}

func (m *MongoDBService) DeleteAlert(ctx context.Context, alertID string) error {
	if !m.enabled {
		return nil
	}
	
	filter := bson.M{"id": alertID}
	_, err := m.db.Collection(CollectionAlerts).DeleteOne(ctx, filter)
	return err
}

func (m *MongoDBService) GetAlert(ctx context.Context, alertID string) (*models.Alert, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}
	
	var alert models.Alert
	filter := bson.M{"id": alertID}
	
	err := m.db.Collection(CollectionAlerts).FindOne(ctx, filter).Decode(&alert)
	if err != nil {
		return nil, err
	}
	
	return &alert, nil
}

func (m *MongoDBService) GetAllAlerts(ctx context.Context) ([]*models.Alert, error) {
	if !m.enabled {
		return nil, fmt.Errorf("MongoDB not enabled")
	}
	
	cursor, err := m.db.Collection(CollectionAlerts).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	var alerts []*models.Alert
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, err
	}
	
	return alerts, nil
}