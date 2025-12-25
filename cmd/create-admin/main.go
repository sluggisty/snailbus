package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"snailbus/internal/auth"
	"snailbus/internal/storage"
)

func main() {
	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://snail:snail_secret@localhost:5432/snailbus?sslmode=disable"
	}

	// Get admin credentials from environment or use defaults
	adminUsername := os.Getenv("ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = "admin"
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "change me"
	}

	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@localhost"
	}

	// Run migrations first
	if err := runMigrations(databaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize storage
	store, err := storage.NewPostgresStorage(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	// Check if admin user already exists
	_, _, err = store.GetUserByUsername(adminUsername)
	if err == nil {
		log.Printf("Admin user '%s' already exists, skipping creation", adminUsername)
		return
	}

	// Check if error is not "not found"
	if err != storage.ErrNotFound {
		log.Fatalf("Error checking for existing user: %v", err)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create admin user
	user, err := store.CreateUser(adminUsername, adminEmail, passwordHash)
	if err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	log.Printf("Admin user '%s' created successfully with ID: %s", adminUsername, user.ID)

	// Create an API key for the admin user
	plainKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		log.Fatalf("Failed to generate API key: %v", err)
	}

	// Store API key
	_, err = store.CreateAPIKey(user.ID, keyHash, keyPrefix, "Initial Admin API Key", nil)
	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	log.Printf("Admin API key created: %s", plainKey)
	log.Printf("You can use this API key to authenticate: X-API-Key: %s", plainKey)
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
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "file:///app/migrations"
	}

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

