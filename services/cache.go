// package services

// import (
// 	"context"
// 	"crypto/tls"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"strings"
// 	"sync"
// 	"time"

// 	"github.com/redis/go-redis/v9"

// 	"xand/config"
// 	"xand/models"
// )

// // CacheMode indicates which cache backend is active
// type CacheMode string

// const (
// 	CacheModeRedis     CacheMode = "redis"
// 	CacheModeInMemory  CacheMode = "in-memory"
// 	CacheModeDegraded  CacheMode = "degraded"
// )

// // CacheItem for in-memory fallback
// type CacheItem struct {
// 	Data      interface{}
// 	ExpiresAt time.Time
// }

// type CacheService struct {
// 	cfg        *config.Config
// 	aggregator *DataAggregator

// 	// Redis
// 	redis       *redis.Client
// 	redisCtx    context.Context
// 	redisCancel context.CancelFunc
// 	mode        CacheMode
// 	modeMutex   sync.RWMutex

// 	// In-memory fallback
// 	inMemoryStore sync.Map

// 	stopChan chan struct{}
// }

// func NewCacheService(cfg *config.Config, aggregator *DataAggregator) *CacheService {
// 	ctx, cancel := context.WithCancel(context.Background())
	
// 	cs := &CacheService{
// 		cfg:         cfg,
// 		aggregator:  aggregator,
// 		redisCtx:    ctx,
// 		redisCancel: cancel,
// 		stopChan:    make(chan struct{}),
// 	}

// 	// Try to connect to Redis
// 	cs.connectRedis()

// 	return cs
// }

// // connectRedis attempts to connect to Redis with retry logic
// func (cs *CacheService) connectRedis() {
// 	redisAddr := cs.cfg.Redis.Address
// 	if redisAddr == "" {
// 		redisAddr = "localhost:6379"
// 	}

// 	redisPassword := cs.cfg.Redis.Password
// 	redisDB := cs.cfg.Redis.DB
// 	useTLS := cs.cfg.Redis.UseTLS

// 	// Auto-detect for Leapcell
// 	if strings.Contains(redisAddr, "leapcell") {
// 		useTLS = true
// 		log.Printf("Detected Leapcell Redis, forcing TLS")
// 	}

// 	options := &redis.Options{
// 		Addr:         redisAddr,
// 		Password:     redisPassword,
// 		DB:           redisDB,
// 		DialTimeout:  10 * time.Second,  // Longer for cloud
// 		ReadTimeout:  5 * time.Second,
// 		WriteTimeout: 5 * time.Second,
// 		PoolSize:     5,                 // Reduced pool size
// 		MinIdleConns: 1,                 // Minimal idle connections
// 		MaxRetries:   3,
// 		PoolTimeout:  10 * time.Second,
// 	}

// 	if useTLS {
// 		options.TLSConfig = &tls.Config{
// 			MinVersion:         tls.VersionTLS12,
// 			InsecureSkipVerify: true, // Leapcell uses shared certs
// 		}
// 		log.Printf("TLS enabled for Redis connection")
// 	}

// 	cs.redis = redis.NewClient(options)

// 	// Test with longer timeout
// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	pong, err := cs.redis.Ping(ctx).Result()
// 	if err != nil {
// 		log.Printf("⚠️  Redis connection failed: %v", err)
// 		log.Printf("⚠️  Attempted: %s (TLS: %v, Password: %v)", 
// 			redisAddr, useTLS, redisPassword != "")
// 		log.Printf("⚠️  Running in DEGRADED mode (in-memory cache only)")
// 		cs.setMode(CacheModeInMemory)
// 		return
// 	}

// 	log.Printf("✓ Redis connected successfully at %s (response: %s)", redisAddr, pong)
// 	cs.setMode(CacheModeRedis)
// }

// // setMode safely updates the cache mode
// func (cs *CacheService) setMode(mode CacheMode) {
// 	cs.modeMutex.Lock()
// 	defer cs.modeMutex.Unlock()
	
// 	if cs.mode != mode {
// 		cs.mode = mode
// 		log.Printf("Cache mode changed: %s", mode)
// 	}
// }

// // getMode safely reads the cache mode
// func (cs *CacheService) getMode() CacheMode {
// 	cs.modeMutex.RLock()
// 	defer cs.modeMutex.RUnlock()
// 	return cs.mode
// }

// // StartCacheWarmer starts background tasks
// func (cs *CacheService) StartCacheWarmer() {
// 	log.Println("Starting Cache Warmer...")

// 	// Initial warm
// 	cs.Refresh()

// 	// Start background workers
// 	go cs.runRefreshLoop()
// 	go cs.runHealthCheckLoop()
// }

// func (cs *CacheService) Stop() {
// 	close(cs.stopChan)
// 	cs.redisCancel()
	
// 	if cs.redis != nil {
// 		cs.redis.Close()
// 	}
// }

// // runRefreshLoop periodically refreshes cache
// func (cs *CacheService) runRefreshLoop() {
// 	ticker := time.NewTicker(time.Duration(cs.cfg.Polling.StatsInterval) * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			cs.Refresh()
// 		case <-cs.stopChan:
// 			return
// 		}
// 	}
// }

// // runHealthCheckLoop monitors Redis health and attempts reconnection
// func (cs *CacheService) runHealthCheckLoop() {
// 	ticker := time.NewTicker(30 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ticker.C:
// 			cs.checkRedisHealth()
// 		case <-cs.stopChan:
// 			return
// 		}
// 	}
// }

// // checkRedisHealth verifies Redis is responsive
// func (cs *CacheService) checkRedisHealth() {
// 	mode := cs.getMode()
	
// 	switch mode {
// case CacheModeRedis:
// 		// Check if Redis is still healthy
// 		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
// 		defer cancel()

// 		_, err := cs.redis.Ping(ctx).Result()
// 		if err != nil {
// 			log.Printf("⚠️  Redis health check failed: %v", err)
// 			log.Printf("⚠️  Switching to DEGRADED mode")
// 			cs.setMode(CacheModeInMemory)
// 		}
// 	case CacheModeInMemory:
// 		// Try to reconnect to Redis
// 		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
// 		defer cancel()

// 		_, err := cs.redis.Ping(ctx).Result()
// 		if err == nil {
// 			log.Printf("✓ Redis reconnected! Switching back to Redis mode")
// 			cs.syncInMemoryToRedis()
// 			cs.setMode(CacheModeRedis)
// 		}
// 	}
// }

// // syncInMemoryToRedis copies in-memory cache to Redis on reconnection
// func (cs *CacheService) syncInMemoryToRedis() {
// 	log.Println("Syncing in-memory cache to Redis...")
	
// 	synced := 0
// 	cs.inMemoryStore.Range(func(key, value interface{}) bool {
// 		keyStr := key.(string)
// 		item := value.(*CacheItem)
		
// 		// Calculate remaining TTL
// 		ttl := time.Until(item.ExpiresAt)
// 		if ttl > 0 {
// 			cs.setRedis(keyStr, item.Data, ttl)
// 			synced++
// 		}
// 		return true
// 	})
	
// 	log.Printf("Synced %d items to Redis", synced)
// }

// // Refresh fetches fresh data and updates cache
// func (cs *CacheService) Refresh() {
// 	start := time.Now()
	
// 	// 1. Get aggregated stats
// 	stats := cs.aggregator.Aggregate()

// 	// 2. Get all nodes
// 	nodes := cs.aggregator.discovery.GetNodes()

// 	// 3. Update caches
// 	ttl := time.Duration(cs.cfg.Cache.TTL) * time.Second

// 	cs.Set("stats", stats, ttl)
// 	cs.Set("nodes", nodes, ttl)

// 	// 4. Update individual node caches
// 	for _, n := range nodes {
// 		cs.Set("node:"+n.ID, n, 60*time.Second)
// 	}

// 	elapsed := time.Since(start)
// 	log.Printf("Cache refreshed (%s): %d nodes online, %d total | Mode: %s", 
// 		elapsed, stats.OnlineNodes, len(nodes), cs.getMode())
// }

// // ============================================
// // Generic Set/Get with Redis + In-Memory
// // ============================================

// // Set stores data in the active cache backend
// func (cs *CacheService) Set(key string, data interface{}, ttl time.Duration) {
// 	mode := cs.getMode()
	
// 	if mode == CacheModeRedis {
// 		err := cs.setRedis(key, data, ttl)
// 		if err != nil {
// 			log.Printf("Redis SET failed for key '%s': %v (falling back to in-memory)", key, err)
// 			cs.setInMemory(key, data, ttl)
// 		}
// 	} else {
// 		cs.setInMemory(key, data, ttl)
// 	}
// }

// // Get retrieves data from the active cache backend
// func (cs *CacheService) Get(key string) (interface{}, bool) {
// 	mode := cs.getMode()
	
// 	if mode == CacheModeRedis {
// 		data, found, err := cs.getRedis(key)
// 		if err != nil {
// 			log.Printf("Redis GET failed for key '%s': %v (checking in-memory)", key, err)
// 			return cs.getInMemory(key)
// 		}
// 		return data, found
// 	}
	
// 	return cs.getInMemory(key)
// }

// // GetWithStale retrieves data and stale status
// func (cs *CacheService) GetWithStale(key string) (interface{}, bool, bool) {
// 	mode := cs.getMode()
	
// 	if mode == CacheModeRedis {
// 		data, found, err := cs.getRedis(key)
// 		if err != nil {
// 			log.Printf("Redis GET failed for key '%s': %v", key, err)
// 			data, found := cs.getInMemory(key)
// 			return data, false, found // Can't determine staleness
// 		}
// 		// Redis manages TTL, so if found, it's fresh
// 		return data, false, found
// 	}
	
// 	return cs.getInMemoryWithStale(key)
// }

// // ============================================
// // Redis Operations
// // ============================================

// func (cs *CacheService) setRedis(key string, data interface{}, ttl time.Duration) error {
// 	ctx, cancel := context.WithTimeout(cs.redisCtx, 2*time.Second)
// 	defer cancel()

// 	// Serialize data to JSON
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal data: %w", err)
// 	}

// 	// Store in Redis
// 	err = cs.redis.Set(ctx, key, jsonData, ttl).Err()
// 	if err != nil {
// 		return fmt.Errorf("redis set failed: %w", err)
// 	}

// 	return nil
// }

// func (cs *CacheService) getRedis(key string) (interface{}, bool, error) {
// 	ctx, cancel := context.WithTimeout(cs.redisCtx, 2*time.Second)
// 	defer cancel()

// 	// Get from Redis
// 	jsonData, err := cs.redis.Get(ctx, key).Result()
// 	if err == redis.Nil {
// 		return nil, false, nil // Key doesn't exist
// 	}
// 	if err != nil {
// 		return nil, false, err // Redis error
// 	}

// 	// Deserialize based on key pattern
// 	var data interface{}
	
// 	if key == "stats" {
// 		var stats models.NetworkStats
// 		if err := json.Unmarshal([]byte(jsonData), &stats); err != nil {
// 			return nil, false, err
// 		}
// 		data = stats
// 	} else if key == "nodes" {
// 		var nodes []*models.Node
// 		if err := json.Unmarshal([]byte(jsonData), &nodes); err != nil {
// 			return nil, false, err
// 		}
// 		data = nodes
// 	} else if len(key) > 5 && key[:5] == "node:" {
// 		var node models.Node
// 		if err := json.Unmarshal([]byte(jsonData), &node); err != nil {
// 			return nil, false, err
// 		}
// 		data = &node
// 	} else {
// 		// Generic JSON unmarshaling
// 		if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
// 			return nil, false, err
// 		}
// 	}

// 	return data, true, nil
// }

// // ============================================
// // In-Memory Operations (Fallback)
// // ============================================

// func (cs *CacheService) setInMemory(key string, data interface{}, ttl time.Duration) {
// 	item := &CacheItem{
// 		Data:      data,
// 		ExpiresAt: time.Now().Add(ttl),
// 	}
// 	cs.inMemoryStore.Store(key, item)
// }

// func (cs *CacheService) getInMemory(key string) (interface{}, bool) {
// 	val, ok := cs.inMemoryStore.Load(key)
// 	if !ok {
// 		return nil, false
// 	}

// 	item := val.(*CacheItem)
// 	if time.Now().After(item.ExpiresAt) {
// 		return nil, false
// 	}

// 	return item.Data, true
// }

// func (cs *CacheService) getInMemoryWithStale(key string) (interface{}, bool, bool) {
// 	val, ok := cs.inMemoryStore.Load(key)
// 	if !ok {
// 		return nil, false, false
// 	}

// 	item := val.(*CacheItem)
// 	isStale := time.Now().After(item.ExpiresAt)
// 	return item.Data, isStale, true
// }

// // ============================================
// // Typed Helper Methods
// // ============================================

// func (cs *CacheService) GetNetworkStats(allowStale bool) (*models.NetworkStats, bool, bool) {
// 	data, stale, found := cs.GetWithStale("stats")
// 	if !found {
// 		return nil, false, false
// 	}
// 	if !allowStale && stale {
// 		return nil, false, false
// 	}
	
// 	if stats, ok := data.(models.NetworkStats); ok {
// 		return &stats, stale, true
// 	}
// 	return nil, false, false
// }

// func (cs *CacheService) GetNodes(allowStale bool) ([]*models.Node, bool, bool) {
// 	data, stale, found := cs.GetWithStale("nodes")
// 	if !found {
// 		return nil, false, false
// 	}
// 	if !allowStale && stale {
// 		return nil, false, false
// 	}
	
// 	if nodes, ok := data.([]*models.Node); ok {
// 		return nodes, stale, true
// 	}
// 	return nil, false, false
// }

// func (cs *CacheService) GetNode(id string, allowStale bool) (*models.Node, bool, bool) {
// 	data, stale, found := cs.GetWithStale("node:" + id)
// 	if !found {
// 		return nil, false, false
// 	}
// 	if !allowStale && stale {
// 		return nil, false, false
// 	}
	
// 	if node, ok := data.(*models.Node); ok {
// 		return node, stale, true
// 	}
// 	return nil, false, false
// }

// // ============================================
// // Utility Methods
// // ============================================

// // GetCacheMode returns the current cache mode
// func (cs *CacheService) GetCacheMode() CacheMode {
// 	return cs.getMode()
// }

// // ClearCache clears all cache data
// func (cs *CacheService) ClearCache() error {
// 	mode := cs.getMode()
	
// 	if mode == CacheModeRedis {
// 		ctx, cancel := context.WithTimeout(cs.redisCtx, 5*time.Second)
// 		defer cancel()
		
// 		// Clear only our keys (pattern matching)
// 		iter := cs.redis.Scan(ctx, 0, "node*", 0).Iterator()
// 		for iter.Next(ctx) {
// 			cs.redis.Del(ctx, iter.Val())
// 		}
		
// 		cs.redis.Del(ctx, "stats", "nodes")
// 		log.Println("Redis cache cleared")
// 	}
	
// 	// Always clear in-memory as well
// 	cs.inMemoryStore = sync.Map{}
// 	log.Println("In-memory cache cleared")
	
// 	return nil
// }

// // GetCacheStats returns cache statistics
// func (cs *CacheService) GetCacheStats() map[string]interface{} {
// 	stats := map[string]interface{}{
// 		"mode": string(cs.getMode()),
// 	}
	
// 	mode := cs.getMode()
	
// 	if mode == CacheModeRedis {
// 		ctx, cancel := context.WithTimeout(cs.redisCtx, 2*time.Second)
// 		defer cancel()
		
// 		info, err := cs.redis.Info(ctx, "memory").Result()
// 		if err == nil {
// 			stats["redis_info"] = info
// 		}
		
// 		dbSize, err := cs.redis.DBSize(ctx).Result()
// 		if err == nil {
// 			stats["redis_keys"] = dbSize
// 		}
// 	}
	
// 	// Count in-memory items
// 	inMemCount := 0
// 	cs.inMemoryStore.Range(func(_, _ interface{}) bool {
// 		inMemCount++
// 		return true
// 	})
// 	stats["in_memory_keys"] = inMemCount
	
// 	return stats
// }

// // isLikelyManagedRedis detects if the address looks like a managed Redis service
// func isLikelyManagedRedis(address string) bool {
// 	// Common managed Redis service domains
// 	managedDomains := []string{
// 		"redis.cloud",
// 		"redislabs.com",
// 		".amazonaws.com",
// 		".azure.com",
// 		".digitalocean.com",
// 		".upstash.io",
// 		".rediscloud.com",
// 		".leapcell.cloud", // Your provider
// 		"cache.windows.net",
// 	}
	
// 	for _, domain := range managedDomains {
// 		if strings.Contains(address, domain) {
// 			return true
// 		}
// 	}
	
// 	// If it's not localhost or an IP, it's likely managed
// 	if !strings.Contains(address, "localhost") && 
// 	   !strings.Contains(address, "127.0.0.1") &&
// 	   !strings.HasPrefix(address, "10.") &&
// 	   !strings.HasPrefix(address, "192.168.") {
// 		// Contains a domain name, likely managed
// 		if strings.Contains(address, ".") {
// 			return true
// 		}
// 	}
	
// 	return false
// }


package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"xand/config"
	"xand/models"
)

// CacheMode indicates which cache backend is active
type CacheMode string

const (
	CacheModeRedis    CacheMode = "redis"
	CacheModeInMemory CacheMode = "in-memory"
)

// CacheItem for in-memory fallback
type CacheItem struct {
	Data      interface{}
	ExpiresAt time.Time
}

type CacheService struct {
	cfg        *config.Config
	aggregator *DataAggregator

	// Redis
	redis       *redis.Client
	redisCtx    context.Context
	redisCancel context.CancelFunc
	mode        CacheMode
	modeMutex   sync.RWMutex

	// In-memory fallback
	inMemoryStore sync.Map

	stopChan chan struct{}
}

func NewCacheService(cfg *config.Config, aggregator *DataAggregator) *CacheService {
	ctx, cancel := context.WithCancel(context.Background())
	
	cs := &CacheService{
		cfg:         cfg,
		aggregator:  aggregator,
		redisCtx:    ctx,
		redisCancel: cancel,
		stopChan:    make(chan struct{}),
		mode:        CacheModeInMemory, // Start in memory mode
	}

	// Try to connect to Redis if enabled
	if cfg.Redis.Enabled {
		cs.connectRedis()
	} else {
		log.Println("Redis disabled in config, using in-memory cache only")
	}

	return cs
}

// connectRedis attempts to connect to Redis with improved error handling
func (cs *CacheService) connectRedis() {
	if cs.cfg.Redis.Address == "" {
		log.Println("Redis address not configured, using in-memory cache")
		return
	}

	options := &redis.Options{
		Addr:         cs.cfg.Redis.Address,
		Password:     cs.cfg.Redis.Password,
		DB:           cs.cfg.Redis.DB,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
		PoolTimeout:  10 * time.Second,
	}

	// Enable TLS if configured
	if cs.cfg.Redis.UseTLS {
		options.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, // For cloud providers with shared certs
		}
		log.Printf("TLS enabled for Redis connection")
	}

	cs.redis = redis.NewClient(options)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pong, err := cs.redis.Ping(ctx).Result()
	if err != nil {
		log.Printf("⚠️  Redis connection failed: %v", err)
		log.Printf("⚠️  Running in IN-MEMORY mode")
		cs.setMode(CacheModeInMemory)
		return
	}

	log.Printf("✓ Redis connected successfully (response: %s)", pong)
	cs.setMode(CacheModeRedis)
}

// setMode safely updates the cache mode
func (cs *CacheService) setMode(mode CacheMode) {
	cs.modeMutex.Lock()
	defer cs.modeMutex.Unlock()
	cs.mode = mode
}

// getMode safely reads the cache mode
func (cs *CacheService) getMode() CacheMode {
	cs.modeMutex.RLock()
	defer cs.modeMutex.RUnlock()
	return cs.mode
}

// StartCacheWarmer starts background cache refresh
func (cs *CacheService) StartCacheWarmer() {
	log.Println("Starting Cache Warmer...")

	// Initial warm
	cs.Refresh()

	// Start background workers
	go cs.runRefreshLoop()
	go cs.runHealthCheckLoop()
}

func (cs *CacheService) Stop() {
	close(cs.stopChan)
	cs.redisCancel()
	
	if cs.redis != nil {
		cs.redis.Close()
	}
}

// runRefreshLoop periodically refreshes cache
func (cs *CacheService) runRefreshLoop() {
	interval := time.Duration(cs.cfg.Polling.StatsInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cs.Refresh()
		case <-cs.stopChan:
			return
		}
	}
}

// runHealthCheckLoop monitors Redis health
func (cs *CacheService) runHealthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cs.checkRedisHealth()
		case <-cs.stopChan:
			return
		}
	}
}

// checkRedisHealth verifies Redis is responsive and attempts reconnection
func (cs *CacheService) checkRedisHealth() {
	if !cs.cfg.Redis.Enabled || cs.redis == nil {
		return
	}

	mode := cs.getMode()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := cs.redis.Ping(ctx).Result()

	if mode == CacheModeRedis && err != nil {
		log.Printf("⚠️  Redis health check failed: %v", err)
		log.Printf("⚠️  Switching to IN-MEMORY mode")
		cs.setMode(CacheModeInMemory)
	} else if mode == CacheModeInMemory && err == nil {
		log.Printf("✓ Redis reconnected! Switching back to REDIS mode")
		cs.syncInMemoryToRedis()
		cs.setMode(CacheModeRedis)
	}
}

// syncInMemoryToRedis copies in-memory cache to Redis on reconnection
func (cs *CacheService) syncInMemoryToRedis() {
	log.Println("Syncing in-memory cache to Redis...")
	
	synced := 0
	cs.inMemoryStore.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		item := value.(*CacheItem)
		
		ttl := time.Until(item.ExpiresAt)
		if ttl > 0 {
			if err := cs.setRedis(keyStr, item.Data, ttl); err == nil {
				synced++
			}
		}
		return true
	})
	
	log.Printf("Synced %d items to Redis", synced)
}

// Refresh fetches fresh data and updates cache
func (cs *CacheService) Refresh() {
	start := time.Now()
	
	// Get aggregated stats
	stats := cs.aggregator.Aggregate()

	// Get all nodes
	nodes := cs.aggregator.discovery.GetNodes()

	// Update caches with appropriate TTL
	ttl := time.Duration(cs.cfg.Cache.TTL) * time.Second

	cs.Set("stats", stats, ttl)
	cs.Set("nodes", nodes, ttl)

	// Update individual node caches
	for _, n := range nodes {
		cs.Set("node:"+n.ID, n, 60*time.Second)
	}

	elapsed := time.Since(start)
	log.Printf("Cache refreshed (%s): %d online/%d total nodes | Mode: %s", 
		elapsed, stats.OnlineNodes, len(nodes), cs.getMode())
}

// ============================================
// Generic Set/Get with Redis + In-Memory
// ============================================

// Set stores data in the active cache backend
func (cs *CacheService) Set(key string, data interface{}, ttl time.Duration) {
	mode := cs.getMode()
	
	if mode == CacheModeRedis {
		if err := cs.setRedis(key, data, ttl); err != nil {
			log.Printf("Redis SET failed for '%s': %v (falling back to in-memory)", key, err)
			cs.setInMemory(key, data, ttl)
		}
	} else {
		cs.setInMemory(key, data, ttl)
	}
}

// Get retrieves data from the active cache backend
func (cs *CacheService) Get(key string) (interface{}, bool) {
	mode := cs.getMode()
	
	if mode == CacheModeRedis {
		data, found, err := cs.getRedis(key)
		if err != nil {
			// On Redis error, check in-memory fallback
			return cs.getInMemory(key)
		}
		return data, found
	}
	
	return cs.getInMemory(key)
}

// GetWithStale retrieves data and indicates if it's stale
func (cs *CacheService) GetWithStale(key string) (interface{}, bool, bool) {
	mode := cs.getMode()
	
	if mode == CacheModeRedis {
		data, found, err := cs.getRedis(key)
		if err != nil {
			data, found := cs.getInMemory(key)
			return data, false, found
		}
		// Redis manages TTL, so if found, it's fresh
		return data, false, found
	}
	
	return cs.getInMemoryWithStale(key)
}

// ============================================
// Redis Operations
// ============================================

func (cs *CacheService) setRedis(key string, data interface{}, ttl time.Duration) error {
	if cs.redis == nil {
		return fmt.Errorf("redis client not initialized")
	}

	ctx, cancel := context.WithTimeout(cs.redisCtx, 2*time.Second)
	defer cancel()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	return cs.redis.Set(ctx, key, jsonData, ttl).Err()
}

func (cs *CacheService) getRedis(key string) (interface{}, bool, error) {
	if cs.redis == nil {
		return nil, false, fmt.Errorf("redis client not initialized")
	}

	ctx, cancel := context.WithTimeout(cs.redisCtx, 2*time.Second)
	defer cancel()

	jsonData, err := cs.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	// Deserialize based on key pattern
	var data interface{}
	
	switch {
	case key == "stats":
		var stats models.NetworkStats
		if err := json.Unmarshal([]byte(jsonData), &stats); err != nil {
			return nil, false, err
		}
		data = stats
	case key == "nodes":
		var nodes []*models.Node
		if err := json.Unmarshal([]byte(jsonData), &nodes); err != nil {
			return nil, false, err
		}
		data = nodes
	case strings.HasPrefix(key, "node:"):
		var node models.Node
		if err := json.Unmarshal([]byte(jsonData), &node); err != nil {
			return nil, false, err
		}
		data = &node
	default:
		if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
			return nil, false, err
		}
	}

	return data, true, nil
}

// ============================================
// In-Memory Operations (Fallback)
// ============================================

func (cs *CacheService) setInMemory(key string, data interface{}, ttl time.Duration) {
	item := &CacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	}
	cs.inMemoryStore.Store(key, item)
}

func (cs *CacheService) getInMemory(key string) (interface{}, bool) {
	val, ok := cs.inMemoryStore.Load(key)
	if !ok {
		return nil, false
	}

	item := val.(*CacheItem)
	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item.Data, true
}

func (cs *CacheService) getInMemoryWithStale(key string) (interface{}, bool, bool) {
	val, ok := cs.inMemoryStore.Load(key)
	if !ok {
		return nil, false, false
	}

	item := val.(*CacheItem)
	isStale := time.Now().After(item.ExpiresAt)
	return item.Data, isStale, true
}

// ============================================
// Typed Helper Methods
// ============================================

func (cs *CacheService) GetNetworkStats(allowStale bool) (*models.NetworkStats, bool, bool) {
	data, stale, found := cs.GetWithStale("stats")
	if !found {
		return nil, false, false
	}
	if !allowStale && stale {
		return nil, false, false
	}
	
	if stats, ok := data.(models.NetworkStats); ok {
		return &stats, stale, true
	}
	return nil, false, false
}

func (cs *CacheService) GetNodes(allowStale bool) ([]*models.Node, bool, bool) {
	data, stale, found := cs.GetWithStale("nodes")
	if !found {
		return nil, false, false
	}
	if !allowStale && stale {
		return nil, false, false
	}
	
	if nodes, ok := data.([]*models.Node); ok {
		return nodes, stale, true
	}
	return nil, false, false
}

func (cs *CacheService) GetNode(id string, allowStale bool) (*models.Node, bool, bool) {
	data, stale, found := cs.GetWithStale("node:" + id)
	if !found {
		return nil, false, false
	}
	if !allowStale && stale {
		return nil, false, false
	}
	
	if node, ok := data.(*models.Node); ok {
		return node, stale, true
	}
	return nil, false, false
}

// ============================================
// Utility Methods
// ============================================

func (cs *CacheService) GetCacheMode() CacheMode {
	return cs.getMode()
}

func (cs *CacheService) ClearCache() error {
	mode := cs.getMode()
	
	if mode == CacheModeRedis && cs.redis != nil {
		ctx, cancel := context.WithTimeout(cs.redisCtx, 5*time.Second)
		defer cancel()
		
		// Use SCAN to find and delete our keys
		iter := cs.redis.Scan(ctx, 0, "node:*", 0).Iterator()
		deleted := 0
		for iter.Next(ctx) {
			cs.redis.Del(ctx, iter.Val())
			deleted++
		}
		
		cs.redis.Del(ctx, "stats", "nodes")
		log.Printf("Redis cache cleared (%d node keys deleted)", deleted)
	}
	
	// Clear in-memory
	cs.inMemoryStore = sync.Map{}
	log.Println("In-memory cache cleared")
	
	return nil
}

func (cs *CacheService) GetCacheStats() map[string]interface{} {
	stats := map[string]interface{}{
		"mode":    string(cs.getMode()),
		"enabled": cs.cfg.Redis.Enabled,
	}
	
	mode := cs.getMode()
	
	if mode == CacheModeRedis && cs.redis != nil {
		ctx, cancel := context.WithTimeout(cs.redisCtx, 2*time.Second)
		defer cancel()
		
		dbSize, err := cs.redis.DBSize(ctx).Result()
		if err == nil {
			stats["redis_keys"] = dbSize
		}
	}
	
	// Count in-memory items
	inMemCount := 0
	cs.inMemoryStore.Range(func(_, _ interface{}) bool {
		inMemCount++
		return true
	})
	stats["in_memory_keys"] = inMemCount
	
	return stats
}





func (cs *CacheService) RefreshWithBatch() {
	start := time.Now()
	
	// Get aggregated stats
	stats := cs.aggregator.Aggregate()
	nodes := cs.aggregator.discovery.GetNodes()

	// Batch cache updates
	updates := []struct {
		key  string
		data interface{}
		ttl  time.Duration
	}{
		{"stats", stats, time.Duration(cs.cfg.Cache.TTL) * time.Second},
		{"nodes", nodes, time.Duration(cs.cfg.Cache.TTL) * time.Second},
	}

	// Execute batch
	for _, update := range updates {
		cs.Set(update.key, update.data, update.ttl)
	}

	// Update individual nodes (limit to avoid overwhelming cache)
	maxIndividualNodes := 100
	for i, n := range nodes {
		if i >= maxIndividualNodes {
			break
		}
		cs.Set("node:"+n.ID, n, 60*time.Second)
	}

	elapsed := time.Since(start)
	log.Printf("Cache refreshed (%s): %d nodes | Mode: %s", 
		elapsed, len(nodes), cs.getMode())
}