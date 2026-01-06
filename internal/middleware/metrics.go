package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"snailbus/internal/metrics"
)

// MetricsMiddleware tracks HTTP request metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start).Seconds()

		// Get route path (use the matched route if available, otherwise use the request path)
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}

		// Get status code
		statusCode := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		// Record metrics
		metrics.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, endpoint, statusCode).Observe(duration)
	}
}
