package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func LoggerMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Process request
			err := next(c)

			if err != nil {
				c.Error(err)
			}

			// Prepare log data
			stop := time.Now()
			req := c.Request()
			res := c.Response()

			timestamp := stop.Format("2006-01-02 15:04:05")
			method := req.Method
			path := req.URL.Path
			if req.URL.RawQuery != "" {
				path += "?" + req.URL.RawQuery
			}
			status := res.Status
			statusText := http.StatusText(status)
			latency := stop.Sub(start).Milliseconds()
			ip := c.RealIP()

			// [2024-12-13 10:30:15] GET /api/nodes → 200 OK (234ms) from 127.0.0.1
			fmt.Printf("[%s] %s %s -> %d %s (%dms) from %s\n",
				timestamp, method, path, status, statusText, latency, ip)

			return nil
		}
	}
}
