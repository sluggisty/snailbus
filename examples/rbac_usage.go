package main

// This file demonstrates example usage of the RBAC middleware
// It is not meant to be compiled, just for reference

import (
	"github.com/gin-gonic/gin"

	"snailbus/internal/handlers"
	"snailbus/internal/middleware"
	"snailbus/internal/storage"
)

func exampleRBACUsage(store storage.Storage, h *handlers.Handlers) {
	r := gin.Default()

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Public auth endpoints (no authentication required)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
		}

		// Protected routes (require API key authentication)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(store))
		{
			// Auth endpoints - accessible to all authenticated users
			protected.GET("/auth/me", h.GetMe)

			// API key management - accessible to all authenticated users
			protected.GET("/api-keys", h.ListAPIKeys)
			protected.POST("/api-keys", h.CreateAPIKey)
			protected.DELETE("/api-keys/:id", h.DeleteAPIKey)

			// Host management - accessible to all authenticated users (viewers, editors, admins)
			protected.GET("/hosts", h.ListHosts)
			protected.GET("/hosts/:host_id", h.GetHost)

			// Host deletion - requires editor or admin role
			editorOrAdmin := protected.Group("")
			editorOrAdmin.Use(middleware.RequireRole("editor", "admin"))
			{
				editorOrAdmin.DELETE("/hosts/:host_id", h.DeleteHost)
			}

			// Admin-only endpoints - requires admin role
			adminOnly := protected.Group("")
			adminOnly.Use(middleware.RequireRole("admin"))
			{
				// Example: User management endpoints (if you add them)
				// adminOnly.GET("/users", h.ListUsers)
				// adminOnly.POST("/users", h.CreateUser)
				// adminOnly.DELETE("/users/:id", h.DeleteUser)
			}

			// Ingest endpoint - requires editor or admin role (viewers can't upload)
			ingest := v1.Group("")
			ingest.Use(middleware.AuthMiddleware(store))
			ingest.Use(middleware.RequireRole("editor", "admin"))
			{
				ingest.POST("/ingest", h.Ingest)
			}
		}
	}
}

// Alternative example: Using RequireRole directly on routes
func exampleDirectRBACUsage(store storage.Storage, h *handlers.Handlers) {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	{
		// Public endpoints
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
		}

		// All authenticated users can view hosts
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(store))
		{
			protected.GET("/hosts", h.ListHosts)
			protected.GET("/hosts/:host_id", h.GetHost)
		}

		// Only editors and admins can delete hosts
		// Note: AuthMiddleware must be applied first, then RequireRole
		v1.DELETE("/hosts/:host_id",
			middleware.AuthMiddleware(store),
			middleware.RequireRole("editor", "admin"),
			h.DeleteHost,
		)

		// Only admins can access admin endpoints
		v1.GET("/admin/users",
			middleware.AuthMiddleware(store),
			middleware.RequireRole("admin"),
			// h.ListUsers, // if you have this handler
		)
	}
}
