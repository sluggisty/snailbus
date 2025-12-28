package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"snailbus/internal/auth"
	"snailbus/internal/models"
	"snailbus/internal/storage"
)

// AuthMiddleware validates API keys from the X-API-Key header
func AuthMiddleware(store storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract API key from header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Also check Authorization header for backward compatibility
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				// Support both "Bearer <key>" and "ApiKey <key>" formats
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 {
					apiKey = parts[1]
				}
			}
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "missing API key",
				"message": "Please provide an API key in the X-API-Key header",
			})
			c.Abort()
			return
		}

		// Extract prefix for efficient lookup
		keyPrefix := auth.GetKeyPrefix(apiKey)

		// Get all API keys with this prefix
		apiKeys, err := store.GetAPIKeyByPrefix(keyPrefix)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "authentication error",
			})
			c.Abort()
			return
		}

		if len(apiKeys) == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			c.Abort()
			return
		}

		// Verify the key against all candidates with matching prefix
		var authenticatedUserID string
		var apiKeyID string
		var matchedKey *models.APIKey

		for _, key := range apiKeys {
			if auth.VerifyAPIKey(apiKey, key.KeyHash) {
				// Check if expired
				if auth.IsExpired(key.ExpiresAt) {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": "API key expired",
					})
					c.Abort()
					return
				}

				matchedKey = key
				authenticatedUserID = key.UserID
				apiKeyID = key.ID
				break
			}
		}

		if matchedKey == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			c.Abort()
			return
		}

		// Get user to check if active
		user, err := store.GetUserByID(authenticatedUserID)
		if err != nil || !user.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "user account is inactive",
			})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", authenticatedUserID)
		c.Set("api_key_id", apiKeyID)
		c.Set("user", user)

		// Update last used timestamp (async, don't wait)
		go store.UpdateAPIKeyLastUsed(apiKeyID)

		c.Next()
	}
}

// AdminMiddleware checks if the authenticated user is an admin
func AdminMiddleware(store storage.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			c.Abort()
			return
		}

		user, err := store.GetUserByID(userID.(string))
		if err != nil || !user.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "admin access required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole creates middleware that checks if the authenticated user has one of the required roles
// This middleware must be used after AuthMiddleware, which sets the "user" in the context
//
// Example usage:
//
//	// Require admin role
//	protected.Use(middleware.RequireRole("admin"))
//
//	// Require either admin or editor role
//	protected.Use(middleware.RequireRole("admin", "editor"))
func RequireRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context (set by AuthMiddleware)
		userValue, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "User not found in context. Ensure AuthMiddleware is applied before RequireRole.",
			})
			c.Abort()
			return
		}

		user, ok := userValue.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal server error",
				"message": "Invalid user type in context",
			})
			c.Abort()
			return
		}

		// Check if user's role matches any of the required roles
		hasRequiredRole := false
		for _, requiredRole := range requiredRoles {
			if user.Role == requiredRole {
				hasRequiredRole = true
				break
			}
		}

		if !hasRequiredRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":          "insufficient role",
				"message":        "Your role does not have permission to access this resource",
				"required_roles": requiredRoles,
				"your_role":      user.Role,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// OrgContextMiddleware extracts organization ID and role from the authenticated user
// and makes them easily accessible in handlers via context keys.
// This middleware must be used after AuthMiddleware, which sets the "user" in the context.
//
// After this middleware, handlers can access:
//   - org_id: via GetOrgID(c) or c.Get("org_id")
//   - role: via GetRole(c) or c.Get("role")
//
// Example usage:
//
//	protected := v1.Group("")
//	protected.Use(middleware.AuthMiddleware(store))
//	protected.Use(middleware.OrgContextMiddleware())
//	{
//	    protected.GET("/hosts", h.ListHosts) // Can use GetOrgID(c) in handler
//	}
func OrgContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context (set by AuthMiddleware)
		userValue, exists := c.Get("user")
		if !exists {
			// If user is not in context, this middleware should not be used
			// or AuthMiddleware was not applied first
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "User not found in context. Ensure AuthMiddleware is applied before OrgContextMiddleware.",
			})
			c.Abort()
			return
		}

		user, ok := userValue.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal server error",
				"message": "Invalid user type in context",
			})
			c.Abort()
			return
		}

		// Extract and set org_id and role for easy access
		c.Set("org_id", user.OrgID)
		c.Set("role", user.Role)

		c.Next()
	}
}

// GetOrgID retrieves the organization ID from the context
// Returns empty string if not found (should not happen if OrgContextMiddleware is used)
func GetOrgID(c *gin.Context) string {
	orgID, exists := c.Get("org_id")
	if !exists {
		return ""
	}
	if orgIDStr, ok := orgID.(string); ok {
		return orgIDStr
	}
	return ""
}

// GetRole retrieves the user's role from the context
// Returns empty string if not found (should not happen if OrgContextMiddleware is used)
func GetRole(c *gin.Context) string {
	role, exists := c.Get("role")
	if !exists {
		return ""
	}
	if roleStr, ok := role.(string); ok {
		return roleStr
	}
	return ""
}

// GetUserID retrieves the user ID from the context
// This is a convenience function for consistency
func GetUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	if userIDStr, ok := userID.(string); ok {
		return userIDStr
	}
	return ""
}
