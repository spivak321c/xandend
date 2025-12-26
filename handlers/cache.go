package handlers

import (
	"net/http"
	
	"github.com/labstack/echo/v4"
	"xand/services"
)

type CacheHandlers struct {
	cache *services.CacheService
}

func NewCacheHandlers(cache *services.CacheService) *CacheHandlers {
	return &CacheHandlers{
		cache: cache,
	}
}

// GetCacheStatus returns cache health and statistics
func (h *CacheHandlers) GetCacheStatus(c echo.Context) error {
	stats := h.cache.GetCacheStats()
	mode := h.cache.GetCacheMode()
	
	response := map[string]interface{}{
		"mode":    string(mode),
		"healthy": mode == services.CacheModeRedis,
		"stats":   stats,
	}
	
	return c.JSON(http.StatusOK, response)
}

// ClearCache clears all cached data (admin endpoint)
func (h *CacheHandlers) ClearCache(c echo.Context) error {
	err := h.cache.ClearCache()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	
	return c.JSON(http.StatusOK, map[string]string{
		"message": "Cache cleared successfully",
	})
}