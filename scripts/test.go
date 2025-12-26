package main

import (
	"context"
	"crypto/tls"
	"fmt"
	//"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// Quick test script to verify Redis connection
// Usage: go run test_redis.go

func main() {
	// Get credentials from environment
	addr := os.Getenv("REDIS_ADDRESS")
	password := os.Getenv("REDIS_PASSWORD")
	
	if addr == "" {
		addr = "xavier-zdqu-diig-640923.leapcell.cloud:6379"
	}
	if password == "" {
		password = "Ae00000L1taObYhCEF8Ixd7Dlrc0mQq6NUjQzG3OVgbLd4KeUE49KW7fvHzp71wvU3kl7GB"
	}

	fmt.Println("=== Redis Connection Test ===")
	fmt.Printf("Address: %s\n", addr)
	fmt.Printf("Password: %s...\n", password[:10])
	fmt.Println()

	// Test 1: Without TLS
	fmt.Println("Test 1: Connection WITHOUT TLS...")
	testConnection(addr, password, false)
	fmt.Println()

	// Test 2: With TLS
	fmt.Println("Test 2: Connection WITH TLS...")
	testConnection(addr, password, true)
	fmt.Println()

	// Test 3: With TLS and longer timeout
	fmt.Println("Test 3: Connection WITH TLS (longer timeout)...")
	testConnectionWithTimeout(addr, password, true, 10*time.Second)
	fmt.Println()
}

func testConnection(addr, password string, useTLS bool) {
	options := &redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	if useTLS {
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(options)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test PING
	start := time.Now()
	pong, err := client.Ping(ctx).Result()
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("❌ FAILED: %v (took %v)\n", err, elapsed)
		return
	}

	fmt.Printf("✅ SUCCESS: %s (took %v)\n", pong, elapsed)

	// Test SET
	err = client.Set(ctx, "test_key", "test_value", 10*time.Second).Err()
	if err != nil {
		fmt.Printf("❌ SET failed: %v\n", err)
		return
	}

	// Test GET
	val, err := client.Get(ctx, "test_key").Result()
	if err != nil {
		fmt.Printf("❌ GET failed: %v\n", err)
		return
	}

	fmt.Printf("✅ SET/GET works: got '%s'\n", val)

	// Cleanup
	client.Del(ctx, "test_key")
}

func testConnectionWithTimeout(addr, password string, useTLS bool, timeout time.Duration) {
	options := &redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		DialTimeout:  timeout,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
	}

	if useTLS {
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(options)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	pong, err := client.Ping(ctx).Result()
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("❌ FAILED: %v (took %v)\n", err, elapsed)
		
		// Additional diagnostics
		fmt.Println("\n🔍 Diagnostics:")
		fmt.Printf("   - TLS enabled: %v\n", useTLS)
		fmt.Printf("   - Timeout: %v\n", timeout)
		fmt.Printf("   - Error type: %T\n", err)
		
		return
	}

	fmt.Printf("✅ SUCCESS: %s (took %v)\n", pong, elapsed)

	// Test large payload
	largeData := make([]byte, 1024*100) // 100KB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err = client.Set(ctx, "test_large", largeData, 10*time.Second).Err()
	if err != nil {
		fmt.Printf("❌ Large payload SET failed: %v\n", err)
		return
	}

	fmt.Println("✅ Large payload (100KB) works")

	client.Del(ctx, "test_large")
}