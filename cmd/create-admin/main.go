package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"snailbus/internal/auth"
	"snailbus/internal/logger"
	"snailbus/internal/storage"
)

func main() {
	// Initialize structured logging
	logger.Init()

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

	adminOrgName := os.Getenv("ADMIN_ORG_NAME")
	if adminOrgName == "" {
		adminOrgName = "Default Organization"
	}

	// Run migrations first
	if err := runMigrations(databaseURL); err != nil {
		logger.Logger.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Initialize storage
	store, err := storage.NewPostgresStorage(databaseURL)
	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer store.Close()

	// Check if admin user already exists
	_, _, err = store.GetUserByUsername(adminUsername)
	if err == nil {
		logger.Logger.Info().Str("username", adminUsername).Msg("Admin user already exists, skipping creation")
		return
	}

	// Check if error is not "not found"
	if err != storage.ErrNotFound {
		logger.Logger.Fatal().Err(err).Str("username", adminUsername).Msg("Error checking for existing user")
	}

	// Check if organization already exists
	org, err := store.GetOrganizationByName(adminOrgName)
	if err != nil && err != storage.ErrNotFound {
		logger.Logger.Fatal().Err(err).Str("org_name", adminOrgName).Msg("Error checking for existing organization")
	}

	var orgID string
	if err == storage.ErrNotFound {
		// Create organization if it doesn't exist
		org, err = store.CreateOrganization(adminOrgName)
		if err != nil {
			logger.Logger.Fatal().Err(err).Str("org_name", adminOrgName).Msg("Failed to create organization")
		}
		logger.Logger.Info().
			Str("org_name", adminOrgName).
			Str("org_id", org.ID).
			Msg("Created organization")
		orgID = org.ID
	} else {
		// Organization exists, check if it has users
		userCount, err := store.CountUsersInOrganization(org.ID)
		if err != nil {
			logger.Logger.Fatal().Err(err).Str("org_id", org.ID).Msg("Error counting users in organization")
		}
		if userCount > 0 {
			logger.Logger.Fatal().
				Str("org_name", adminOrgName).
				Int("user_count", userCount).
				Msg("Organization already has users. Cannot create admin user in existing organization.")
		}
		orgID = org.ID
		logger.Logger.Info().
			Str("org_name", adminOrgName).
			Str("org_id", org.ID).
			Msg("Using existing organization")
	}

	// Hash password
	passwordHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		logger.Logger.Fatal().Err(err).Str("username", adminUsername).Msg("Failed to hash password")
	}

	// Create admin user with admin role
	user, err := store.CreateUser(adminUsername, adminEmail, passwordHash, orgID, "admin")
	if err != nil {
		logger.Logger.Fatal().Err(err).
			Str("username", adminUsername).
			Str("email", adminEmail).
			Str("org_id", orgID).
			Msg("Failed to create admin user")
	}

	logger.Logger.Info().
		Str("username", adminUsername).
		Str("user_id", user.ID).
		Msg("Admin user created successfully")

	// Create an API key for the admin user
	plainKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		logger.Logger.Fatal().Err(err).Str("user_id", user.ID).Msg("Failed to generate API key")
	}

	// Store API key
	_, err = store.CreateAPIKey(user.ID, keyHash, keyPrefix, "Initial Admin API Key", nil)
	if err != nil {
		logger.Logger.Fatal().Err(err).Str("user_id", user.ID).Msg("Failed to create API key")
	}

	logger.Logger.Info().
		Str("api_key", plainKey).
		Str("user_id", user.ID).
		Msg("Admin API key created")
	logger.Logger.Info().
		Str("api_key", plainKey).
		Msg("You can use this API key to authenticate: X-API-Key")
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
		logger.Logger.Info().Msg("Database is up to date, no migrations to run")
	} else {
		logger.Logger.Info().Msg("Database migrations completed successfully")
	}

	return nil
}
