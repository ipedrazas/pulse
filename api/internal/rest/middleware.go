package rest

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ipedrazas/pulse/api/internal/metrics"
)

// RequestIDMiddleware generates or propagates a request ID via the X-Request-ID header.
// The ID is stored in the Gin context and included in the response header.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		requestID, _ := c.Get("request_id")

		slog.Info("request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"request_id", requestID,
		)
	}
}

// MetricsMiddleware records HTTP request count and latency for Prometheus.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath() // use route pattern, not actual path, to avoid high cardinality
		if path == "" {
			path = "unknown"
		}

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		duration := time.Since(start).Seconds()

		metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
