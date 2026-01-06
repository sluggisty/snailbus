package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"snailbus/internal/logger"
)

// CSRF token generation and validation middleware
func CSRFTokens() gin.HandlerFunc {
	// Get CSRF auth key from environment or generate a random one
	authKey := getCSRFAuthKey()

	return func(c *gin.Context) {
		// Skip CSRF validation for safe methods (GET, HEAD, OPTIONS)
		if !IsStateChangingMethod(c.Request.Method) {
			c.Next()
			return
		}

		// Skip CSRF validation for authentication endpoints (login, register, api-key)
		// Users don't have CSRF tokens yet when they authenticate
		if isAuthEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Get CSRF token from header
		tokenFromHeader := c.GetHeader("X-CSRF-Token")
		if tokenFromHeader == "" {
			logger.Logger.Warn().
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Msg("Missing X-CSRF-Token header for state-changing request")
			c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token validation failed"})
			c.Abort()
			return
		}

		// Get expected token from cookie
		tokenFromCookie, err := c.Cookie("csrf_token")
		if err != nil || tokenFromCookie == "" {
			logger.Logger.Warn().
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Msg("Missing CSRF token cookie")
			c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token validation failed"})
			c.Abort()
			return
		}

		// Validate tokens match
		if !validateCSRFToken(tokenFromHeader, tokenFromCookie, authKey) {
			logger.Logger.Warn().
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Msg("CSRF token validation failed - tokens don't match")
			c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token validation failed"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getCSRFAuthKey gets the CSRF authentication key from environment or generates a random one
func getCSRFAuthKey() []byte {
	// Try to get from environment variable
	if key := os.Getenv("CSRF_AUTH_KEY"); key != "" {
		// Decode base64 key if provided
		if decoded, err := base64.StdEncoding.DecodeString(key); err == nil && len(decoded) == 32 {
			return decoded
		}
	}

	// Generate a random 32-byte key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		logger.Logger.Fatal().Err(err).Msg("Failed to generate CSRF auth key")
	}

	logger.Logger.Info().Msg("Generated random CSRF auth key (consider setting CSRF_AUTH_KEY environment variable)")
	return key
}

// GetCSRFToken returns the current CSRF token for the request
func GetCSRFToken(c *gin.Context) string {
	// Try to get token from cookie first
	if token, err := c.Cookie("csrf_token"); err == nil && token != "" {
		return token
	}

	// Generate a new token
	return generateCSRFToken()
}

// SetCSRFCookie sets the CSRF token as a cookie for frontend access
func SetCSRFCookie(c *gin.Context) {
	token := GetCSRFToken(c)
	isSecure := os.Getenv("GIN_MODE") == "release"
	c.SetCookie("csrf_token", token, 86400*7, "/", "", isSecure, false) // 7 days, not httpOnly, secure in production
}

// CSRFTokenMiddleware sets the CSRF token cookie on every response
func CSRFTokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set CSRF token cookie for frontend access
		SetCSRFCookie(c)
		c.Next()
	}
}

// IsStateChangingMethod checks if the HTTP method changes state (requires CSRF protection)
func IsStateChangingMethod(method string) bool {
	stateChangingMethods := []string{"POST", "PUT", "PATCH", "DELETE"}
	for _, m := range stateChangingMethods {
		if strings.ToUpper(method) == m {
			return true
		}
	}
	return false
}

// generateCSRFToken generates a new CSRF token
func generateCSRFToken() string {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		logger.Logger.Fatal().Err(err).Msg("Failed to generate CSRF token")
	}
	return base64.StdEncoding.EncodeToString(token)
}

// validateCSRFToken validates that the provided token matches the expected token
func validateCSRFToken(providedToken, expectedToken string, authKey []byte) bool {
	// Simple comparison for now - in production, you might want to use HMAC validation
	return providedToken == expectedToken && providedToken != ""
}

// isAuthEndpoint checks if the request path is an authentication endpoint that should be exempt from CSRF validation
func isAuthEndpoint(path string) bool {
	authEndpoints := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/api-key",
	}

	for _, endpoint := range authEndpoints {
		if path == endpoint {
			return true
		}
	}
	return false
}
