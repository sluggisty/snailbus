package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"snailbus/internal/config"
	"snailbus/internal/logger"
)

// RequestSizeLimit creates middleware that enforces request size limits
func RequestSizeLimit(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var maxSize int64

		// Determine size limit based on method and path
		switch {
		case c.Request.Method == "GET":
			maxSize = cfg.MaxRequestSizeGet
		case c.Request.URL.Path == "/api/v1/ingest":
			maxSize = cfg.MaxRequestSizeIngest
		case c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH":
			maxSize = cfg.MaxRequestSizePost
		default:
			// For other methods, use a reasonable default
			maxSize = cfg.MaxRequestSizePost
		}

		// Check Content-Length header first (for efficiency)
		if c.Request.ContentLength > maxSize {
			logger.Logger.Warn().
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Int64("content_length", c.Request.ContentLength).
				Int64("max_allowed", maxSize).
				Msg("Request size exceeds limit")

			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "Request entity too large",
				"message": "The request body is too large",
				"limit":   maxSize,
			})
			return
		}

		// Set the maximum request body size for Gin's built-in limit
		// This will be enforced during body reading
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		c.Next()
	}
}
