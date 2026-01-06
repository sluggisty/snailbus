package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"snailbus/internal/logger"
)

// SecurityHeadersMiddleware adds security headers to all HTTP responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	// Get CSP from environment variable, with a reasonable default
	csp := getEnv("CONTENT_SECURITY_POLICY",
		"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';")

	logger.Logger.Info().
		Str("csp_policy", csp).
		Msg("Initializing security headers middleware")

	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking attacks
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS filtering
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer Policy - only send origin for cross-origin requests
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy
		c.Header("Content-Security-Policy", csp)

		// HTTP Strict Transport Security (HSTS) - only for HTTPS connections
		if c.Request.TLS != nil || strings.HasPrefix(c.Request.Header.Get("X-Forwarded-Proto"), "https") {
			// 1 year max-age for HSTS
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		c.Next()
	}
}
