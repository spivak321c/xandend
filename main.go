// package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"os"
// 	"os/signal"
// 	"syscall"
// 	"time"

// 	"github.com/labstack/echo/v4"

// 	"xand/config"
// 	"xand/handlers"
// 	"xand/middleware"
// 	"xand/services"
// 	"xand/utils"
// )

// func main() {
// 	// 1. Config
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Fatalf("Failed to load config: %v", err)
// 	}

// 	log.Println("=== Configuration ===")
// 	log.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
// 	log.Printf("Redis: %s", cfg.Redis.Address)
// 	log.Printf("MongoDB: %s", cfg.MongoDB.Database)

// 	// 2. Core Services
// 	// GeoIP
// 	geo, err := utils.NewGeoResolver(cfg.GeoIP.DBPath)
// 	if err != nil {
// 		log.Printf("⚠️  GeoIP DB not found at %s: %v", cfg.GeoIP.DBPath, err)
// 	}
// 	defer geo.Close()

// 	// MongoDB
// 	mongoService, err := services.NewMongoDBService(cfg)
// 	if err != nil {
// 		log.Printf("⚠️  MongoDB connection failed: %v", err)
// 		log.Println("Analytics features will be disabled")
// 		mongoService = nil
// 	}
// 	if mongoService != nil {
// 		defer mongoService.Close()
// 	}

// 	// PRPC Client
// 	prpc := services.NewPRPCClient(cfg)

// 	// Credits Service
// 	creditsService := services.NewCreditsService()

// 	// Node Discovery
// 	discovery := services.NewNodeDiscovery(cfg, prpc, geo, creditsService)

// 	// Data Aggregator
// 	aggregator := services.NewDataAggregator(discovery)

// 	// Cache Service (with Redis + In-Memory fallback)
// 	cache := services.NewCacheService(cfg, aggregator)

// 	// Feature Services
// 	alertService := services.NewAlertService(cache)
// 	historyService := services.NewHistoryService(cache, mongoService)
// 	calculatorService := services.NewCalculatorService()
// 	topologyService := services.NewTopologyService(cache, discovery)
// 	comparisonService := services.NewComparisonService(cache)

// 	// 3. Start Background Services
// 	log.Println("=== Starting Services ===")
	
// 	// Start Credits Service
// 	creditsService.Start()
// 	log.Println("✓ Credits Service started")
	
// 	// Start Node Discovery
// 	discovery.Start()
// 	log.Println("✓ Node Discovery started")
	
// 	// Wait for initial discovery
// 	log.Println("⏳ Waiting for initial node discovery...")
// 	time.Sleep(15 * time.Second)
	
// 	// Start Cache Warmer (will use Redis or fall back to in-memory)
// 	cache.StartCacheWarmer()
// 	log.Println("✓ Cache Service started")
// 	log.Printf("   Mode: %s", cache.GetCacheMode())
	
// 	// Start History Service
// 	historyService.Start()
// 	log.Println("✓ History Service started")
	
// 	// Start Alert Service
// 	alertService.Start()
// 	log.Println("✓ Alert Service started")
	
// 	log.Println("=== All Services Running ===")

// 	// 4. Web Server
// 	e := echo.New()
// 	e.HideBanner = true

// 	// Middleware
// 	e.Use(middleware.LoggerMiddleware())
// 	e.Use(middleware.CORSMiddleware(cfg.Server.AllowedOrigins))
// 	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
// 		return func(c echo.Context) error {
// 			defer func() {
// 				if r := recover(); r != nil {
// 					log.Printf("Recovered from panic: %v", r)
// 					c.Error(fmt.Errorf("internal server error"))
// 				}
// 			}()
// 			return next(c)
// 		}
// 	})

// 	// 5. Handlers
// 	h := handlers.NewHandler(cfg, cache, discovery, prpc)
// 	alertHandlers := handlers.NewAlertHandlers(alertService)
// 	historyHandlers := handlers.NewHistoryHandlers(historyService)
// 	calculatorHandlers := handlers.NewCalculatorHandlers(calculatorService)
// 	topologyHandlers := handlers.NewTopologyHandlers(topologyService)
// 	comparisonHandlers := handlers.NewComparisonHandlers(comparisonService)
// 	creditsHandlers := handlers.NewCreditsHandlers(creditsService)
// 	analyticsHandlers := handlers.NewAnalyticsHandlers(mongoService)
// 	cacheHandlers := handlers.NewCacheHandlers(cache) // NEW

// 	// 6. Routes
// 	// System
// 	e.GET("/health", h.GetHealth)
// 	e.GET("/cache/status", cacheHandlers.GetCacheStatus) // NEW
// 	e.POST("/cache/clear", cacheHandlers.ClearCache)     // NEW (admin)

// 	api := e.Group("/api")
	
// 	// Core endpoints
// 	api.GET("/status", h.GetStatus)
// 	api.GET("/nodes", h.GetNodes)
// 	api.GET("/nodes/:id", h.GetNode)
// 	api.GET("/stats", h.GetStats)
// 	api.POST("/rpc", h.ProxyRPC)

// 	// Alert endpoints
// 	alerts := api.Group("/alerts")
// 	alerts.POST("", alertHandlers.CreateAlert)
// 	alerts.GET("", alertHandlers.ListAlerts)
// 	alerts.GET("/:id", alertHandlers.GetAlert)
// 	alerts.PUT("/:id", alertHandlers.UpdateAlert)
// 	alerts.DELETE("/:id", alertHandlers.DeleteAlert)
// 	alerts.GET("/history", alertHandlers.GetAlertHistory)
// 	alerts.POST("/test", alertHandlers.TestAlert)

// 	// History endpoints
// 	history := api.Group("/history")
// 	history.GET("/network", historyHandlers.GetNetworkHistory)
// 	history.GET("/nodes/:id", historyHandlers.GetNodeHistory)
// 	history.GET("/forecast", historyHandlers.GetCapacityForecast)
// 	history.GET("/latency-distribution", historyHandlers.GetLatencyDistribution)

// 	// Calculator endpoints
// 	calculator := api.Group("/calculator")
// 	calculator.GET("/costs", calculatorHandlers.CompareCosts)
// 	calculator.GET("/roi", calculatorHandlers.EstimateROI)
// 	calculator.POST("/redundancy", calculatorHandlers.SimulateRedundancy)
// 	calculator.GET("/redundancy", calculatorHandlers.SimulateRedundancy)

// 	// Credits endpoints
// 	credits := api.Group("/credits")
// 	credits.GET("", creditsHandlers.GetAllCredits)
// 	credits.GET("/top", creditsHandlers.GetTopCredits)
// 	credits.GET("/:pubkey", creditsHandlers.GetNodeCredits)

// 	// Analytics endpoints
// 	analytics := api.Group("/analytics")
// 	analytics.GET("/daily-health", analyticsHandlers.GetDailyHealth)
// 	analytics.GET("/high-uptime", analyticsHandlers.GetHighUptimeNodes)
// 	analytics.GET("/storage-growth", analyticsHandlers.GetStorageGrowth)
// 	analytics.GET("/recently-joined", analyticsHandlers.GetRecentlyJoined)
// 	analytics.GET("/weekly-comparison", analyticsHandlers.GetWeeklyComparison)
// 	analytics.GET("/node-graveyard", analyticsHandlers.GetNodeGraveyard)

// 	// Topology endpoints
// 	topology := api.Group("/topology")
// 	topology.GET("", topologyHandlers.GetTopology)
// 	topology.GET("/regions", topologyHandlers.GetRegionalClusters)

// 	// Comparison endpoints
// 	api.GET("/comparison", comparisonHandlers.GetCrossChainComparison)

// 	// 7. Start Server with Graceful Shutdown
// 	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

// 	go func() {
// 		log.Printf("🚀 Server running on http://%s", serverAddr)
		
// 		if err := e.Start(serverAddr); err != nil && err != http.ErrServerClosed {
// 			log.Fatalf("shutting down the server: %v", err)
// 		}
// 	}()

// 	// Wait for interrupt signal
// 	quit := make(chan os.Signal, 1)
// 	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
// 	<-quit
// 	log.Println("⏳ Graceful shutdown initiated...")

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	// Stop Background Services
// 	log.Println("Stopping services...")
// 	alertService.Stop()
// 	historyService.Stop()
// 	cache.Stop() // Will close Redis connection
// 	discovery.Stop()
// 	creditsService.Stop()
// 	log.Println("✓ All services stopped")

// 	if err := e.Shutdown(ctx); err != nil {
// 		e.Logger.Fatal(err)
// 	}
// 	log.Println("✓ Server exited cleanly")
// }






















package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"xand/config"
	"xand/handlers"
	"xand/middleware"
	"xand/services"
	"xand/utils"
)

// func main() {
// 	// 1. Config
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Fatalf("Failed to load config: %v", err)
// 	}

// 	log.Println("=== Configuration ===")
// 	log.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
// 	log.Printf("Redis: %s", cfg.Redis.Address)
// 	log.Printf("MongoDB: %s", cfg.MongoDB.Database)

// 	// 2. Core Services
// 	// GeoIP
// 	geo, err := utils.NewGeoResolver(cfg.GeoIP.DBPath)
// 	if err != nil {
// 		log.Printf("⚠️  GeoIP DB not found at %s: %v", cfg.GeoIP.DBPath, err)
// 	}
// 	defer geo.Close()

// 	// MongoDB
// 	mongoService, err := services.NewMongoDBService(cfg)
// 	if err != nil {
// 		log.Printf("⚠️  MongoDB connection failed: %v", err)
// 		log.Println("Analytics features will be disabled")
// 		mongoService = nil
// 	}
// 	if mongoService != nil {
// 		defer mongoService.Close()
// 	}






// 		discordToken := os.Getenv("DISCORD_BOT_TOKEN")
// 	discordChannelID := os.Getenv("DISCORD_CHANNEL_ID")
// 	discordBot, err := services.NewDiscordBotService(discordToken, discordChannelID)
// 	if err != nil {
// 		log.Printf("⚠️  Discord bot initialization failed: %v", err)
// 		log.Println("Discord notifications will be disabled")
// 		discordBot = nil
// 	} else if discordBot != nil {
// 		defer discordBot.Close()
// 		log.Println("✓ Discord Bot connected")
// 	}

// 	// Telegram Bot Service (PLACEHOLDER - commented out)
// 	// telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
// 	// telegramChatID := os.Getenv("TELEGRAM_CHAT_ID")
// 	// var telegramChatIDInt int64
// 	// if telegramChatID != "" {
// 	// 	telegramChatIDInt, _ = strconv.ParseInt(telegramChatID, 10, 64)
// 	// }
// 	// telegramBot, err := services.NewTelegramBotService(telegramToken, telegramChatIDInt)
// 	// if err != nil {
// 	// 	log.Printf("⚠️  Telegram bot initialization failed: %v", err)
// 	// 	log.Println("Telegram notifications will be disabled")
// 	// 	telegramBot = nil
// 	// } else if telegramBot != nil {
// 	// 	log.Println("✓ Telegram Bot connected")
// 	// }

	

// 	// PRPC Client
// 	prpc := services.NewPRPCClient(cfg)

// 	// Credits Service
// 	creditsService := services.NewCreditsService()

// 	// Node Discovery
// 	discovery := services.NewNodeDiscovery(cfg, prpc, geo, creditsService)

// 	// Data Aggregator
// 	aggregator := services.NewDataAggregator(discovery)

// 	// Cache Service (with Redis + In-Memory fallback)
// 	cache := services.NewCacheService(cfg, aggregator)

// 	// Feature Services
// 	//alertService := services.NewAlertService(cache)

// 	//alertService := services.NewAlertService(cache, mongoService)
// 		alertService := services.NewAlertService(cache, mongoService, discordBot) // UPDATED


// // ADD after alertService.Start() (around line 90):

// 	// Load alerts from MongoDB
// 	if err := alertService.LoadAlertsFromDB(); err != nil {
// 		log.Printf("Warning: Failed to load alerts from MongoDB: %v", err)
// 	} else {
// 		log.Println("✓ Alerts loaded from MongoDB")
// 	}
// 	historyService := services.NewHistoryService(cache, mongoService)
// 	calculatorService := services.NewCalculatorService()
// 	topologyService := services.NewTopologyService(cache, discovery)
// 	comparisonService := services.NewComparisonService(cache)

// 	// 3. Start Background Services
// 	log.Println("=== Starting Services ===")
	
// 	// Start Credits Service
// 	creditsService.Start()
// 	log.Println("✓ Credits Service started")
	
// 	// Start Node Discovery
// 	discovery.Start()
// 	log.Println("✓ Node Discovery started")
	
// 	// Wait for initial discovery
// 	log.Println("⏳ Waiting for initial node discovery...")
// 	time.Sleep(30 * time.Second)
	
// 	// Start Cache Warmer (will use Redis or fall back to in-memory)
// 	cache.StartCacheWarmer()
// 	log.Println("✓ Cache Service started")
// 	log.Printf("   Mode: %s", cache.GetCacheMode())
	
// 	// Start History Service
// 	historyService.Start()
// 	log.Println("✓ History Service started")
	
// 	// Start Alert Service
// 	alertService.Start()
// 	log.Println("✓ Alert Service started")
	
// 	log.Println("=== All Services Running ===")

// 	// 4. Web Server
// 	e := echo.New()
// 	e.HideBanner = true

// 	// Middleware
// 	e.Use(middleware.LoggerMiddleware())
// 	e.Use(middleware.CORSMiddleware(cfg.Server.AllowedOrigins))
// 	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
// 		return func(c echo.Context) error {
// 			defer func() {
// 				if r := recover(); r != nil {
// 					log.Printf("Recovered from panic: %v", r)
// 					c.Error(fmt.Errorf("internal server error"))
// 				}
// 			}()
// 			return next(c)
// 		}
// 	})
// e.GET("/kaithheathcheck", func(c echo.Context) error {
// 	return c.NoContent(http.StatusOK)
// })

// 	// 5. Handlers
// 	h := handlers.NewHandler(cfg, cache, discovery, prpc)
// 	alertHandlers := handlers.NewAlertHandlers(alertService)
// 	historyHandlers := handlers.NewHistoryHandlers(historyService)
// 	calculatorHandlers := handlers.NewCalculatorHandlers(calculatorService)
// 	topologyHandlers := handlers.NewTopologyHandlers(topologyService)
// 	comparisonHandlers := handlers.NewComparisonHandlers(comparisonService)
// 	creditsHandlers := handlers.NewCreditsHandlers(creditsService)
// 	analyticsHandlers := handlers.NewAnalyticsHandlers(mongoService)
// 	cacheHandlers := handlers.NewCacheHandlers(cache)

// 	// 6. Routes
// 	// System
// 	e.GET("/health", h.GetHealth)
// 	e.GET("/cache/status", cacheHandlers.GetCacheStatus)
// 	e.POST("/cache/clear", cacheHandlers.ClearCache)

// 	api := e.Group("/api")
	
// 	// Core endpoints
// 	api.GET("/status", h.GetStatus)
// 	api.GET("/nodes", h.GetNodes)
// 	api.GET("/nodes/:id", h.GetNode)
// 	api.GET("/stats", h.GetStats)
// 	api.POST("/rpc", h.ProxyRPC)

// 	// Alert endpoints
// 	alerts := api.Group("/alerts")
// 	alerts.POST("", alertHandlers.CreateAlert)
// 	alerts.GET("", alertHandlers.ListAlerts)
// 	alerts.GET("/:id", alertHandlers.GetAlert)
// 	alerts.PUT("/:id", alertHandlers.UpdateAlert)
// 	alerts.DELETE("/:id", alertHandlers.DeleteAlert)
// 	alerts.GET("/history", alertHandlers.GetAlertHistory)
// 	alerts.POST("/test", alertHandlers.TestAlert)

// 	// History endpoints
// 	history := api.Group("/history")
// 	history.GET("/network", historyHandlers.GetNetworkHistory)
// 	history.GET("/nodes/:id", historyHandlers.GetNodeHistory)
// 	history.GET("/latency-distribution", historyHandlers.GetLatencyDistribution)

// 	// Forecast endpoint (FIXED)
// 	api.GET("/forecast", historyHandlers.GetCapacityForecast)

// 	// Calculator endpoints
// 	calculator := api.Group("/calculator")
// 	calculator.GET("/costs", calculatorHandlers.CompareCosts)
// 	calculator.GET("/roi", calculatorHandlers.EstimateROI)
// 	calculator.POST("/redundancy", calculatorHandlers.SimulateRedundancy)
// 	calculator.GET("/redundancy", calculatorHandlers.SimulateRedundancy)

// 	// Credits endpoints
// 	credits := api.Group("/credits")
// 	credits.GET("", creditsHandlers.GetAllCredits)
// 	credits.GET("/top", creditsHandlers.GetTopCredits)
// 	credits.GET("/:pubkey", creditsHandlers.GetNodeCredits)
// 	credits.GET("/rank", creditsHandlers.GetCreditsByRank)
// 	credits.GET("/stats", creditsHandlers.GetCreditsStats)

// 	// Analytics endpoints
// 	analytics := api.Group("/analytics")
// 	analytics.GET("/daily-health", analyticsHandlers.GetDailyHealth)
// 	analytics.GET("/high-uptime", analyticsHandlers.GetHighUptimeNodes)
// 	analytics.GET("/storage-growth", analyticsHandlers.GetStorageGrowth)
// 	analytics.GET("/recently-joined", analyticsHandlers.GetRecentlyJoined)
// 	analytics.GET("/weekly-comparison", analyticsHandlers.GetWeeklyComparison)
// 	analytics.GET("/node-graveyard", analyticsHandlers.GetNodeGraveyard)

// 	// Topology endpoints
// 	topology := api.Group("/topology")
// 	topology.GET("", topologyHandlers.GetTopology)
// 	topology.GET("/regions", topologyHandlers.GetRegionalClusters)

// 	// Comparison endpoints
// 	api.GET("/comparison", comparisonHandlers.GetCrossChainComparison)

// 	// 7. Start Server with Graceful Shutdown
// 	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

// 	go func() {
// 		log.Printf("🚀 Server running on http://%s", serverAddr)
		
// 		if err := e.Start(serverAddr); err != nil && err != http.ErrServerClosed {
// 			log.Fatalf("shutting down the server: %v", err)
// 		}
// 	}()

// 	// Wait for interrupt signal
// 	quit := make(chan os.Signal, 1)
// 	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
// 	<-quit
// 	log.Println("⏳ Graceful shutdown initiated...")

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	// Stop Background Services
// 	log.Println("Stopping services...")
// 	alertService.Stop()
// 	historyService.Stop()
// 	cache.Stop()
// 	discovery.Stop()
// 	creditsService.Stop()
// 	log.Println("✓ All services stopped")

// 	if err := e.Shutdown(ctx); err != nil {
// 		e.Logger.Fatal(err)
// 	}
// 	log.Println("✓ Server exited cleanly")
// }


func main() {
	// 1. Config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Println("=== Configuration ===")
	log.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Redis: %s", cfg.Redis.Address)
	log.Printf("MongoDB: %s", cfg.MongoDB.Database)

	// 2. Core Services - Initialize
	geo, err := utils.NewGeoResolver(cfg.GeoIP.DBPath)
	if err != nil {
		log.Printf("⚠️  GeoIP DB not found at %s: %v", cfg.GeoIP.DBPath, err)
	}
	defer geo.Close()

	mongoService, err := services.NewMongoDBService(cfg)
	if err != nil {
		log.Printf("⚠️  MongoDB connection failed: %v", err)
		log.Println("Analytics features will be disabled")
		mongoService = nil
	}
	if mongoService != nil {
		defer mongoService.Close()
	}

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	discordChannelID := os.Getenv("DISCORD_CHANNEL_ID")
	discordBot, err := services.NewDiscordBotService(discordToken, discordChannelID)
	if err != nil {
		log.Printf("⚠️  Discord bot initialization failed: %v", err)
		log.Println("Discord notifications will be disabled")
		discordBot = nil
	} else if discordBot != nil {
		defer discordBot.Close()
		log.Println("✓ Discord Bot connected")
	}

	prpc := services.NewPRPCClient(cfg)
	creditsService := services.NewCreditsService()
	discovery := services.NewNodeDiscovery(cfg, prpc, geo, creditsService)
	aggregator := services.NewDataAggregator(discovery)
	cache := services.NewCacheService(cfg, aggregator)
	alertService := services.NewAlertService(cache, mongoService, discordBot)
	historyService := services.NewHistoryService(cache, mongoService)
	calculatorService := services.NewCalculatorService()
	topologyService := services.NewTopologyService(cache, discovery)
	comparisonService := services.NewComparisonService(cache)

	// 3. Start CRITICAL services immediately (before HTTP server)
	log.Println("=== Starting Critical Services ===")
	
	creditsService.Start()
	log.Println("✓ Credits Service started")
	
	discovery.Start()
	log.Println("✓ Node Discovery started")

	// 4. Web Server Setup
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.LoggerMiddleware())
	e.Use(middleware.CORSMiddleware(cfg.Server.AllowedOrigins))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic: %v", r)
					c.Error(fmt.Errorf("internal server error"))
				}
			}()
			return next(c)
		}
	})

	// FIX THE TYPO - both spellings for compatibility
	e.GET("/kaithhealthcheck", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})
	e.GET("/kaithheathcheck", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// 5. Handlers
	h := handlers.NewHandler(cfg, cache, discovery, prpc)
	alertHandlers := handlers.NewAlertHandlers(alertService)
	historyHandlers := handlers.NewHistoryHandlers(historyService)
	calculatorHandlers := handlers.NewCalculatorHandlers(calculatorService)
	topologyHandlers := handlers.NewTopologyHandlers(topologyService)
	comparisonHandlers := handlers.NewComparisonHandlers(comparisonService)
	creditsHandlers := handlers.NewCreditsHandlers(creditsService)
	analyticsHandlers := handlers.NewAnalyticsHandlers(mongoService)
	cacheHandlers := handlers.NewCacheHandlers(cache)

	// 6. Routes
	e.GET("/health", h.GetHealth)
	e.GET("/cache/status", cacheHandlers.GetCacheStatus)
	e.POST("/cache/clear", cacheHandlers.ClearCache)

	api := e.Group("/api")
	
	api.GET("/status", h.GetStatus)
	api.GET("/nodes", h.GetNodes)
	api.GET("/nodes/:id", h.GetNode)
	api.GET("/stats", h.GetStats)
	api.POST("/rpc", h.ProxyRPC)

	alerts := api.Group("/alerts")
	alerts.POST("", alertHandlers.CreateAlert)
	alerts.GET("", alertHandlers.ListAlerts)
	alerts.GET("/:id", alertHandlers.GetAlert)
	alerts.PUT("/:id", alertHandlers.UpdateAlert)
	alerts.DELETE("/:id", alertHandlers.DeleteAlert)
	alerts.GET("/history", alertHandlers.GetAlertHistory)
	alerts.POST("/test", alertHandlers.TestAlert)

	history := api.Group("/history")
	history.GET("/network", historyHandlers.GetNetworkHistory)
	history.GET("/nodes/:id", historyHandlers.GetNodeHistory)
	history.GET("/latency-distribution", historyHandlers.GetLatencyDistribution)
	api.GET("/forecast", historyHandlers.GetCapacityForecast)

	calculator := api.Group("/calculator")
	calculator.GET("/costs", calculatorHandlers.CompareCosts)
	calculator.GET("/roi", calculatorHandlers.EstimateROI)
	calculator.POST("/redundancy", calculatorHandlers.SimulateRedundancy)
	calculator.GET("/redundancy", calculatorHandlers.SimulateRedundancy)

	credits := api.Group("/credits")
	credits.GET("", creditsHandlers.GetAllCredits)
	credits.GET("/top", creditsHandlers.GetTopCredits)
	credits.GET("/:pubkey", creditsHandlers.GetNodeCredits)
	credits.GET("/rank", creditsHandlers.GetCreditsByRank)
	credits.GET("/stats", creditsHandlers.GetCreditsStats)

	analytics := api.Group("/analytics")
	analytics.GET("/daily-health", analyticsHandlers.GetDailyHealth)
	analytics.GET("/high-uptime", analyticsHandlers.GetHighUptimeNodes)
	analytics.GET("/storage-growth", analyticsHandlers.GetStorageGrowth)
	analytics.GET("/recently-joined", analyticsHandlers.GetRecentlyJoined)
	analytics.GET("/weekly-comparison", analyticsHandlers.GetWeeklyComparison)
	analytics.GET("/node-graveyard", analyticsHandlers.GetNodeGraveyard)

	topology := api.Group("/topology")
	topology.GET("", topologyHandlers.GetTopology)
	topology.GET("/regions", topologyHandlers.GetRegionalClusters)

	api.GET("/comparison", comparisonHandlers.GetCrossChainComparison)

	// 7. Start HTTP Server
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	
	go func() {
		log.Printf("🚀 Server running on http://%s", serverAddr)
		if err := e.Start(serverAddr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("shutting down the server: %v", err)
		}
	}()

	// Small delay to ensure server is listening
	time.Sleep(200 * time.Millisecond)
	log.Println("✓ HTTP Server ready")

	// 8. Start remaining services in background
	go func() {
		log.Println("⏳ Initializing remaining services...")
		
		// Reduced wait time - adjust based on your needs
		time.Sleep(10 * time.Second)
		
		cache.StartCacheWarmer()
		log.Println("✓ Cache Service started")
		log.Printf("   Mode: %s", cache.GetCacheMode())
		
		historyService.Start()
		log.Println("✓ History Service started")
		
		alertService.Start()
		log.Println("✓ Alert Service started")
		
		if err := alertService.LoadAlertsFromDB(); err != nil {
			log.Printf("Warning: Failed to load alerts from MongoDB: %v", err)
		} else {
			log.Println("✓ Alerts loaded from MongoDB")
		}
		
		log.Println("=== All Services Running ===")
	}()

	// 9. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("⏳ Graceful shutdown initiated...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Stopping services...")
	alertService.Stop()
	historyService.Stop()
	cache.Stop()
	discovery.Stop()
	creditsService.Stop()
	log.Println("✓ All services stopped")

	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
	log.Println("✓ Server exited cleanly")
}
