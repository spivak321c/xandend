package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"xand/config"
	"xand/models"
)

type PRPCClient struct {
	config     *config.Config
	httpClient *http.Client
}

func NewPRPCClient(cfg *config.Config) *PRPCClient {
	// Use configured timeout or default to 5 seconds
	timeout := cfg.PRPCTimeoutDuration()
	if timeout > 10*time.Second {
		timeout = 5 * time.Second // Cap at 5 seconds for faster failures
	}
	
	return &PRPCClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
				DisableKeepAlives:   false,
			},
		},
	}
}

func (c *PRPCClient) CallPRPC(nodeIP string, method string, params interface{}) (*models.RPCResponse, error) {
	reqBody := models.RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("http://%s/rpc", nodeIP)

	var resp *http.Response
	delay := 200 * time.Millisecond
	maxRetries := c.config.PRPC.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for i := 0; i < maxRetries; i++ {
		httpReq, reqErr := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if reqErr != nil {
			err = fmt.Errorf("failed to create request: %w", reqErr)
			break
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err = c.httpClient.Do(httpReq)
		if err == nil {
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				resp.Body.Close()
				err = fmt.Errorf("server error: %d", resp.StatusCode)
			} else {
				break
			}
		}

		if i < maxRetries-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %d from %s %s", resp.StatusCode, method, nodeIP)
	}

	var rpcResp models.RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return &rpcResp, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

func (c *PRPCClient) GetVersion(nodeIP string) (*models.VersionResponse, error) {
	resp, err := c.CallPRPC(nodeIP, "get-version", nil)
	if err != nil {
		return nil, err
	}

	var verResp models.VersionResponse
	if err := json.Unmarshal(resp.Result, &verResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version result: %w", err)
	}
	return &verResp, nil
}

func (c *PRPCClient) GetStats(nodeIP string) (*models.StatsResponse, error) {
	resp, err := c.CallPRPCWithRetry(nodeIP, "get-stats", nil)
	if err != nil {
		return nil, fmt.Errorf("get-stats failed for %s: %w", nodeIP, err)
	}

	var statsResp models.StatsResponse
	if err := json.Unmarshal(resp.Result, &statsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats from %s: %w", nodeIP, err)
	}
	
	return &statsResp, nil
}

func (c *PRPCClient) GetPods(nodeIP string) (*models.PodsResponse, error) {
	resp, err := c.CallPRPC(nodeIP, "get-pods-with-stats", nil)
	if err != nil {
		return nil, err
	}

	var podsResp models.PodsResponse
	if err := json.Unmarshal(resp.Result, &podsResp); err != nil {
		log.Printf("ERROR: Failed to unmarshal pods from %s: %v", nodeIP, err)
		return nil, fmt.Errorf("failed to unmarshal pods result: %w", err)
	}
	
	return &podsResp, nil
}

func (c *PRPCClient) CallPRPCWithRetry(nodeIP string, method string, params interface{}) (*models.RPCResponse, error) {
	var lastErr error
	maxRetries := c.config.PRPC.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := c.CallPRPC(nodeIP, method, params)
		if err == nil {
			return resp, nil
		}
		
		lastErr = err
		
		if isNonRetryableError(err) {
			break
		}
		
		if attempt < maxRetries {
			backoff := time.Duration(200*attempt) * time.Millisecond
			time.Sleep(backoff)
		}
	}
	
	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func isNonRetryableError(err error) bool {
	errStr := err.Error()
	nonRetryable := []string{
		"Parse error",
		"Invalid Request",
		"Method not found",
		"connection refused", // Don't retry refused connections
	}
	
	for _, msg := range nonRetryable {
		if strings.Contains(errStr, msg) {
			return true
		}
	}
	return false
}