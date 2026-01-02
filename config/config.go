package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server       ServerConfig       `json:"server"`
	PRPC         PRPCConfig         `json:"prpc"`
	Polling      PollingConfig      `json:"polling"`
	Cache        CacheConfig        `json:"cache"`
	Redis        RedisConfig        `json:"redis"` // NEW
	GeoIP        GeoIPConfig        `json:"geoip"`
	MongoDB      MongoDBConfig      `json:"mongodb"`
	Registration RegistrationConfig `json:"registration"`
}

// NEW: Registration Configuration
type RegistrationConfig struct {
	CSVPath string `json:"csv_path"`
}

type ServerConfig struct {
	Port           int      `json:"port"`
	Host           string   `json:"host"`
	AllowedOrigins []string `json:"allowed_origins"`
	SeedNodes      []string `json:"seed_nodes"`
}

type PRPCConfig struct {
	DefaultPort int `json:"default_port"`
	Timeout     int `json:"timeout_seconds"`
	MaxRetries  int `json:"max_retries"`
}

type PollingConfig struct {
	DiscoveryInterval   int `json:"discovery_interval_seconds"`
	StatsInterval       int `json:"stats_interval_seconds"`
	HealthCheckInterval int `json:"health_check_interval_seconds"`
	StaleThreshold      int `json:"stale_threshold_minutes"`
}

type CacheConfig struct {
	TTL     int `json:"ttl_seconds"`
	MaxSize int `json:"max_size"`
}

// NEW: Redis Configuration
type RedisConfig struct {
	Address  string `json:"address"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	Enabled  bool   `json:"enabled"`
	UseTLS   bool   `json:"use_tls"`
}

type GeoIPConfig struct {
	DBPath string `json:"db_path"`
}

type MongoDBConfig struct {
	URI      string `json:"uri"`
	Database string `json:"database"`
	Enabled  bool   `json:"enabled"`
}

func LoadConfig() (*Config, error) {
	// Load .env file if exists
	_ = godotenv.Load()

	// Default configuration
	cfg := &Config{
		Server: ServerConfig{
			Port:           8080,
			Host:           "0.0.0.0",
			AllowedOrigins: []string{"*"},
			SeedNodes:      []string{},
		},
		PRPC: PRPCConfig{
			DefaultPort: 6000,
			Timeout:     5,
			MaxRetries:  3,
		},
		Polling: PollingConfig{
			DiscoveryInterval:   60,  // Peer discovery every 60s
			StatsInterval:       30,  // Cache refresh every 30s (more frequent due to fast gossip)
			HealthCheckInterval: 120, // Health checks every 2 minutes (less aggressive)
			StaleThreshold:      5,
		},
		Cache: CacheConfig{
			TTL:     30,
			MaxSize: 1000,
		},
		Redis: RedisConfig{
			Address:  "localhost:6379",
			Password: "",
			DB:       0,
			Enabled:  true, // Enable Redis by default
			UseTLS:   true,
		},
		GeoIP: GeoIPConfig{
			DBPath: "",
		},
		MongoDB: MongoDBConfig{
			URI:      "mongodb://localhost:27017",
			Database: "xandeum_analytics",
			Enabled:  true,
		},
		Registration: RegistrationConfig{
			CSVPath: "",
		},
	}

	// Load from config file if exists
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config/config.json"
	}

	if _, err := os.Stat(configPath); err == nil {
		file, err := os.Open(configPath)
		if err == nil {
			defer file.Close()
			decoder := json.NewDecoder(file)
			if err := decoder.Decode(cfg); err != nil {
				fmt.Printf("Warning: Failed to decode config file: %v\n", err)
			}
		}
	}

	// Load from environment variables (overrides config file)
	loadEnv(cfg)

	// Load from command-line flags (overrides everything)
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	var serverPort int
	var serverHost string

	fs.IntVar(&serverPort, "port", 0, "Server port")
	fs.StringVar(&serverHost, "host", "", "Server host")

	_ = fs.Parse(os.Args[1:])

	if isFlagPassed(fs, "port") {
		cfg.Server.Port = serverPort
	}
	if isFlagPassed(fs, "host") {
		cfg.Server.Host = serverHost
	}

	return cfg, nil
}

func isFlagPassed(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func loadEnv(cfg *Config) {
	// Server configuration
	if val := os.Getenv("SERVER_PORT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Server.Port = p
		}
	}
	if val := os.Getenv("SERVER_HOST"); val != "" {
		cfg.Server.Host = val
	}
	if val := os.Getenv("ALLOWED_ORIGINS"); val != "" {
		cfg.Server.AllowedOrigins = strings.Split(val, ",")
	}
	if val := os.Getenv("SEED_NODES"); val != "" {
		parts := strings.Split(val, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		cfg.Server.SeedNodes = parts
	}

	// PRPC configuration
	if val := os.Getenv("PRPC_PORT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.PRPC.DefaultPort = p
		}
	}
	if val := os.Getenv("PRPC_TIMEOUT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.PRPC.Timeout = p
		}
	}
	if val := os.Getenv("PRPC_MAX_RETRIES"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.PRPC.MaxRetries = p
		}
	}

	// Polling configuration
	if val := os.Getenv("DISCOVERY_INTERVAL"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Polling.DiscoveryInterval = p
		}
	}
	if val := os.Getenv("STATS_INTERVAL"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Polling.StatsInterval = p
		}
	}
	if val := os.Getenv("HEALTH_CHECK_INTERVAL"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Polling.HealthCheckInterval = p
		}
	}
	if val := os.Getenv("STALE_THRESHOLD"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Polling.StaleThreshold = p
		}
	}

	// Cache configuration
	if val := os.Getenv("CACHE_TTL"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Cache.TTL = p
		}
	}
	if val := os.Getenv("CACHE_MAX_SIZE"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Cache.MaxSize = p
		}
	}

	// NEW: Redis configuration
	if val := os.Getenv("REDIS_ADDRESS"); val != "" {
		cfg.Redis.Address = val
	}
	if val := os.Getenv("REDIS_PASSWORD"); val != "" {
		cfg.Redis.Password = val
	}
	if val := os.Getenv("REDIS_DB"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Redis.DB = p
		}
	}
	if val := os.Getenv("REDIS_ENABLED"); val != "" {
		cfg.Redis.Enabled = val == "true" || val == "1"
	}

	// GeoIP configuration
	if val := os.Getenv("GEOIP_DB_PATH"); val != "" {
		cfg.GeoIP.DBPath = val
	}

	// MongoDB configuration
	if val := os.Getenv("MONGODB_URI"); val != "" {
		cfg.MongoDB.URI = val
	}
	if val := os.Getenv("MONGODB_DATABASE"); val != "" {
		cfg.MongoDB.Database = val
	}
	if val := os.Getenv("MONGODB_ENABLED"); val != "" {
		cfg.MongoDB.Enabled = val == "true" || val == "1"
	}

	// Registration configuration
	if val := os.Getenv("REGISTRATION_CSV_PATH"); val != "" {
		cfg.Registration.CSVPath = val
	}
}

// Helper methods for duration conversion
func (c *Config) PRPCTimeoutDuration() time.Duration {
	return time.Duration(c.PRPC.Timeout) * time.Second
}

func (c *Config) DiscoveryIntervalDuration() time.Duration {
	return time.Duration(c.Polling.DiscoveryInterval) * time.Second
}

func (c *Config) StatsIntervalDuration() time.Duration {
	return time.Duration(c.Polling.StatsInterval) * time.Second
}

func (c *Config) StaleThresholdDuration() time.Duration {
	return time.Duration(c.Polling.StaleThreshold) * time.Minute
}

func (c *Config) CacheTTLDuration() time.Duration {
	return time.Duration(c.Cache.TTL) * time.Second
}
