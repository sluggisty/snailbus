package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"snailbus/internal/handlers"
	"snailbus/internal/storage"
)

func main() {
	// Initialize database connection
	databaseURL := getEnv("DATABASE_URL", "postgres://snail:snail_secret@localhost:5432/snailbus?sslmode=disable")

	// Run migrations first
	if err := runMigrations(databaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize storage
	store, err := storage.NewPostgresStorage(databaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Create handlers
	h := handlers.New(store)

	// Create Gin router
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", h.Health)

	// Root endpoint
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to Snailbus API",
			"version": "1.0.0",
		})
	})

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Ingest endpoint - receives data from snail-core
		v1.POST("/ingest", h.Ingest)

		// Host management endpoints
		v1.GET("/hosts", h.ListHosts)
		v1.GET("/hosts/:hostname", h.GetHost)
		v1.DELETE("/hosts/:hostname", h.DeleteHost)
	}

	// Start server on port 8080
	port := getEnv("PORT", "8080")
	log.Printf("Starting Snailbus server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// runMigrations runs database migrations
func runMigrations(databaseURL string) error {
	// Parse database URL to get driver instance
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Get migrations directory from environment or use default
	migrationsPath := getEnv("MIGRATIONS_PATH", "file://migrations")

	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("Database is up to date, no migrations to run")
	} else {
		log.Println("Database migrations completed successfully")
	}

	return nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
