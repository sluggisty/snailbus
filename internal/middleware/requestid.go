package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"snailbus/internal/logger"
)

// RequestIDMiddleware generates a unique request ID for each HTTP request
// and stores it in the Gin context. It also sets the X-Request-ID header
// in the response so clients can track requests.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID is already in header (for distributed tracing)
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate new UUID for request ID
			requestID = uuid.New().String()
		}

		// Store in context
		c.Set(logger.RequestIDKey, requestID)

		// Set response header
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}


