package testutils

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// SetupTestDB creates a test database connection and runs migrations.
// Returns the database connection, a cleanup function, and an error.
// The cleanup function should be called in defer statements to ensure proper cleanup.
func SetupTestDB(t *testing.T) (*sql.DB, func(), error) {
	// Get test database URL from environment or use default
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		// Default test database connection string
		databaseURL = "postgres://snail:snail_secret@localhost:5432/snailbus_test?sslmode=disable"
	}

	// Open database connection
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open test database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	// Run migrations
	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Return cleanup function
	cleanup := func() {
		if err := CleanTestData(db); err != nil {
			t.Logf("Warning: failed to clean test data: %v", err)
		}
		db.Close()
	}

	return db, cleanup, nil
}

// RunMigrations runs all migrations on the provided database connection.
func RunMigrations(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Get migrations directory from environment or find it dynamically
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		// Find migrations directory by traversing up from current directory
		// until we find a directory containing "migrations"
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		// Traverse up the directory tree to find migrations folder
		dir := wd
		var migrationsDir string
		for {
			testPath := filepath.Join(dir, "migrations")
			if _, err := os.Stat(testPath); err == nil {
				// Found it!
				migrationsDir = testPath
				break
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				// Reached root, migrations not found
				return fmt.Errorf("migrations directory not found")
			}
			dir = parent
		}

		absPath, err := filepath.Abs(migrationsDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		migrationsPath = "file://" + absPath
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

	return nil
}

// CleanTestData removes all test data from the database.
// This should be called after each test to ensure test isolation.
// Note: This does not drop tables or schema, only deletes data.
// Deletes in order to respect foreign key constraints (children before parents).
func CleanTestData(db *sql.DB) error {
	// Delete in order to respect foreign key constraints (children first)
	// Order: api_keys -> hosts -> users -> organizations
	tables := []string{
		"api_keys",
		"hosts",
		"users",
		"organizations",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s", table)
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to clean table %s: %w", table, err)
		}
	}

	return nil
}

// GetTestDatabaseURL returns the test database URL from environment or a default value.
func GetTestDatabaseURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://snail:snail_secret@localhost:5432/snailbus_test?sslmode=disable"
}
