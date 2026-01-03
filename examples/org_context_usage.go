package main

// This file demonstrates example usage of the OrgContextMiddleware
// It is not meant to be compiled, just for reference

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"snailbus/internal/handlers"
	"snailbus/internal/middleware"
	"snailbus/internal/storage"
)

func exampleOrgContextUsage(store storage.Storage, h *handlers.Handlers) {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	{
		// Public endpoints
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
		}

		// Protected routes with org context
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(store))
		protected.Use(middleware.OrgContextMiddleware()) // Extract org_id and role
		{
			// Example: List hosts filtered by organization
			protected.GET("/hosts", func(c *gin.Context) {
				orgID := middleware.GetOrgID(c)
				// Use orgID to filter hosts
				// hosts, err := h.storage.ListHostsByOrganization(orgID)
				c.JSON(http.StatusOK, gin.H{"org_id": orgID})
			})

			// Example: Create resource with org context
			protected.POST("/resources", func(c *gin.Context) {
				orgID := middleware.GetOrgID(c)
				role := middleware.GetRole(c)

				// Check role permissions
				if role != "admin" && role != "editor" {
					c.JSON(http.StatusForbidden, gin.H{
						"error": "insufficient permissions",
					})
					return
				}

				// Create resource with orgID
				c.JSON(http.StatusCreated, gin.H{
					"org_id":  orgID,
					"message": "Resource created",
				})
			})

			// Example: Access both org_id and role
			protected.GET("/my-org", func(c *gin.Context) {
				orgID := middleware.GetOrgID(c)
				role := middleware.GetRole(c)
				userID := middleware.GetUserID(c)

				c.JSON(http.StatusOK, gin.H{
					"org_id":  orgID,
					"role":    role,
					"user_id": userID,
				})
			})
		}

		// Example: Using with RequireRole
		adminOnly := v1.Group("")
		adminOnly.Use(middleware.AuthMiddleware(store))
		adminOnly.Use(middleware.OrgContextMiddleware())
		adminOnly.Use(middleware.RequireRole("admin"))
		{
			adminOnly.GET("/admin/stats", func(c *gin.Context) {
				orgID := middleware.GetOrgID(c)
				// Admin-only endpoint with org context
				c.JSON(http.StatusOK, gin.H{
					"org_id": orgID,
					"stats":  "admin stats here",
				})
			})
		}
	}
}

// Example handler using org context
func exampleHandler(c *gin.Context) {
	// Method 1: Using helper functions (recommended)
	orgID := middleware.GetOrgID(c)
	role := middleware.GetRole(c)
	userID := middleware.GetUserID(c)

	// Method 2: Direct context access
	orgID2, exists := c.Get("org_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Use the values
	_ = orgID
	_ = role
	_ = userID
	_ = orgID2
}

