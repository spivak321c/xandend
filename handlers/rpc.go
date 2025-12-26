package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"time"

	"xand/models"

	"github.com/labstack/echo/v4"
)

// ProxyRPC godoc
func (h *Handler) ProxyRPC(c echo.Context) error {
	var req models.RPCRequest
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.RPCError{Code: -32700, Message: "Parse error"})
	}

	if err := json.Unmarshal(body, &req); err != nil {
		return c.JSON(http.StatusBadRequest, models.RPCError{Code: -32700, Message: "Parse error"})
	}

	if req.JSONRPC != "2.0" {
		return c.JSON(http.StatusBadRequest, models.RPCError{Code: -32600, Message: "Invalid Request"})
	}

	// Get Nodes (allow stale for proxy finding is better than failure)
	nodes, _, found := h.Cache.GetNodes(true)
	if !found || len(nodes) == 0 {
		return c.JSON(http.StatusServiceUnavailable, models.RPCError{Code: -32000, Message: "No nodes available"})
	}

	var candidates []*models.Node
	for _, n := range nodes {
		if n.Status == "online" {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		for _, n := range nodes {
			if n.Status == "warning" {
				candidates = append(candidates, n)
			}
		}
	}
	if len(candidates) == 0 {
		return c.JSON(http.StatusServiceUnavailable, models.RPCError{Code: -32000, Message: "No reachable nodes"})
	}

	target := candidates[rand.Intn(len(candidates))]

	proxyResp, err := h.forwardRequest(target.Address, body)
	if err != nil {
		if len(candidates) > 1 {
			target2 := candidates[rand.Intn(len(candidates))]
			proxyResp, err = h.forwardRequest(target2.Address, body)
		}
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.RPCError{Code: -32603, Message: "Internal proxied error"})
	}
	defer proxyResp.Body.Close()

	return c.Stream(proxyResp.StatusCode, "application/json", proxyResp.Body)
}

func (h *Handler) forwardRequest(address string, body []byte) (*http.Response, error) {
	url := "http://" + address + "/rpc"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	return client.Do(req)
}
