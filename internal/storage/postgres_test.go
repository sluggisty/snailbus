package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"snailbus/internal/auth"
	"snailbus/internal/models"
)

// Test UUIDs for predictable testing
const (
	testHostID1    = "00000000-0000-0000-0000-000000000001"
	testHostID2    = "00000000-0000-0000-0000-000000000002"
	testUserID1    = "00000000-0000-0000-0000-000000000010"
	testUserID2    = "00000000-0000-0000-0000-000000000011"
	testOrgID1     = "00000000-0000-0000-0000-000000000100"
	testOrgID2     = "00000000-0000-0000-0000-000000000200"
	testAPIKeyID1  = "00000000-0000-0000-0000-000000001000"
	testAPIKeyID2  = "00000000-0000-0000-0000-000000002000"
)

func getTestDatabaseURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://snail:snail_secret@localhost:5432/snailbus_test?sslmode=disable"
}

func setupTestDB(t *testing.T) (*sql.DB, func(), error) {
	databaseURL := getTestDatabaseURL()
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open test database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to create migration driver: %w", err)
	}

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		// Find migrations directory by traversing up from current directory
		// until we find a directory containing "migrations"
		wd, err := os.Getwd()
		if err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("failed to get working directory: %w", err)
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
				db.Close()
				return nil, nil, fmt.Errorf("migrations directory not found")
			}
			dir = parent
		}

		absPath, err := filepath.Abs(migrationsDir)
		if err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("failed to get absolute path: %w", err)
		}

		migrationsPath = "file://" + absPath
	}

	m, err := migrate.NewWithDatabaseInstance(migrationsPath, "postgres", driver)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		db.Close()
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	cleanup := func() {
		// Clean test data in order to respect foreign key constraints
		// Order: api_keys -> hosts -> users -> organizations
		tables := []string{"api_keys", "hosts", "users", "organizations"}
		for _, table := range tables {
			if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
				t.Logf("Warning: failed to clean table %s: %v", table, err)
			}
		}
		db.Close()
	}

	return db, cleanup, nil
}

func setupTestStorage(t *testing.T) (Storage, func()) {
	_, cleanup, err := setupTestDB(t)
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}

	store, err := NewPostgresStorage(getTestDatabaseURL())
	if err != nil {
		cleanup()
		t.Fatalf("Failed to create test storage: %v", err)
	}

	return store, func() {
		store.Close()
		cleanup()
	}
}

// Helper functions to avoid import cycle with testutils
func createTestOrg(store Storage, name string) (*models.Organization, error) {
	return store.CreateOrganization(name)
}

func createTestUser(store Storage, username, email, password, orgID, role string) (*models.User, error) {
	if password == "" {
		password = "testpassword123"
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	return store.CreateUser(username, email, passwordHash, orgID, role)
}

func createTestAPIKey(store Storage, userID, name string) (string, *models.APIKey, error) {
	plainKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		return "", nil, err
	}
	apiKey, err := store.CreateAPIKey(userID, keyHash, keyPrefix, name, nil)
	if err != nil {
		return "", nil, err
	}
	return plainKey, apiKey, nil
}

func createTestReport(hostID, hostname string) *models.Report {
	testData := json.RawMessage(`{
		"system": {
			"os_name": "Fedora",
			"os_version": "42",
			"os_version_major": "42",
			"os_version_minor": "0",
			"os_version_patch": "0"
		}
	}`)
	return &models.Report{
		ID:         hostID,
		ReceivedAt: time.Now().UTC(),
		Meta: models.ReportMeta{
			Hostname:     hostname,
			HostID:       hostID,
			CollectionID: "test-collection-id",
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
			SnailVersion: "0.2.0",
		},
		Data:   testData,
		Errors: []string{},
	}
}

// ============================================================================
// Host CRUD Operations Tests
// ============================================================================

func TestPostgresStorage_SaveHost(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create test organization and user
	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name    string
		report  *models.Report
		orgID   string
		userID   string
		wantErr bool
	}{
		{
			name: "create new host",
			report: &models.Report{
				ID:         testHostID1,
				ReceivedAt: time.Now().UTC(),
				Meta: models.ReportMeta{
					Hostname:     "test-host-1",
					HostID:       testHostID1,
					CollectionID: "collection-1",
					Timestamp:    time.Now().UTC().Format(time.RFC3339),
					SnailVersion: "0.2.0",
				},
				Data:   json.RawMessage(`{"system": {"os_name": "Fedora"}}`),
				Errors: []string{},
			},
			orgID:   org.ID,
			userID:  user.ID,
			wantErr: false,
		},
		{
			name: "update existing host",
			report: &models.Report{
				ID:         testHostID1,
				ReceivedAt: time.Now().UTC(),
				Meta: models.ReportMeta{
					Hostname:     "test-host-1-updated",
					HostID:       testHostID1,
					CollectionID: "collection-2",
					Timestamp:    time.Now().UTC().Format(time.RFC3339),
					SnailVersion: "0.3.0",
				},
				Data:   json.RawMessage(`{"system": {"os_name": "Ubuntu"}}`),
				Errors: []string{},
			},
			orgID:   org.ID,
			userID:  user.ID,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.SaveHost(tt.report, tt.orgID, tt.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the host was saved
				saved, err := store.GetHost(tt.report.Meta.HostID, tt.orgID)
				if err != nil {
					t.Errorf("GetHost() error = %v", err)
					return
				}
				if saved.Meta.Hostname != tt.report.Meta.Hostname {
					t.Errorf("Hostname = %v, want %v", saved.Meta.Hostname, tt.report.Meta.Hostname)
				}
			}
		})
	}
}

func TestPostgresStorage_SaveHost_OrganizationIsolation(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create two organizations
	org1, err := createTestOrg(store, "Org 1")
	if err != nil {
		t.Fatalf("Failed to create org1: %v", err)
	}

	org2, err := createTestOrg(store, "Org 2")
	if err != nil {
		t.Fatalf("Failed to create org2: %v", err)
	}

	user1, err := createTestUser(store, "user1", "user1@example.com", "", org1.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2, err := createTestUser(store, "user2", "user2@example.com", "", org2.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Save host for org1
	report := createTestReport(testHostID1, "host1")
	err = store.SaveHost(report, org1.ID, user1.ID)
	if err != nil {
		t.Fatalf("Failed to save host for org1: %v", err)
	}

	// Try to update host from org2 (should fail)
	report.Meta.Hostname = "hacked-hostname"
	err = store.SaveHost(report, org2.ID, user2.ID)
	if err == nil {
		t.Error("SaveHost() should fail when trying to update host from different organization")
	}
}

func TestPostgresStorage_GetHost(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Save a host first
	report := createTestReport(testHostID1, "test-host")
	err = store.SaveHost(report, org.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	tests := []struct {
		name    string
		hostID  string
		orgID   string
		wantErr bool
		want    string
	}{
		{
			name:    "get existing host",
			hostID:  testHostID1,
			orgID:   org.ID,
			wantErr: false,
			want:    "test-host",
		},
		{
			name:    "get non-existent host",
			hostID:  "00000000-0000-0000-0000-000000000999",
			orgID:   org.ID,
			wantErr: true,
		},
		{
			name:    "get host from wrong organization",
			hostID:  testHostID1,
			orgID:   "00000000-0000-0000-0000-000000000999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetHost(tt.hostID, tt.orgID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Meta.Hostname != tt.want {
					t.Errorf("GetHost() hostname = %v, want %v", got.Meta.Hostname, tt.want)
				}
			} else {
				if err != ErrNotFound && err != nil {
					t.Errorf("GetHost() expected ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPostgresStorage_DeleteHost(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Save a host first
	report := createTestReport(testHostID1, "test-host")
	err = store.SaveHost(report, org.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	tests := []struct {
		name    string
		hostID  string
		orgID   string
		wantErr bool
	}{
		{
			name:    "delete existing host",
			hostID:  testHostID1,
			orgID:   org.ID,
			wantErr: false,
		},
		{
			name:    "delete non-existent host",
			hostID:  "00000000-0000-0000-0000-000000000999",
			orgID:   org.ID,
			wantErr: true,
		},
		{
			name:    "delete host from wrong organization",
			hostID:  testHostID1,
			orgID:   "00000000-0000-0000-0000-000000000999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.DeleteHost(tt.hostID, tt.orgID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify host was deleted
				_, err := store.GetHost(tt.hostID, tt.orgID)
				if err != ErrNotFound {
					t.Errorf("GetHost() after delete should return ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPostgresStorage_ListHosts(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create two organizations
	org1, err := createTestOrg(store, "Org 1")
	if err != nil {
		t.Fatalf("Failed to create org1: %v", err)
	}

	org2, err := createTestOrg(store, "Org 2")
	if err != nil {
		t.Fatalf("Failed to create org2: %v", err)
	}

	user1, err := createTestUser(store, "user1", "user1@example.com", "", org1.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2, err := createTestUser(store, "user2", "user2@example.com", "", org2.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Save hosts for org1
	report1 := createTestReport(testHostID1, "host1")
	report2 := createTestReport(testHostID2, "host2")
	err = store.SaveHost(report1, org1.ID, user1.ID)
	if err != nil {
		t.Fatalf("Failed to save host1: %v", err)
	}
	err = store.SaveHost(report2, org1.ID, user1.ID)
	if err != nil {
		t.Fatalf("Failed to save host2: %v", err)
	}

	// Save host for org2
	report3 := createTestReport("00000000-0000-0000-0000-000000000003", "host3")
	err = store.SaveHost(report3, org2.ID, user2.ID)
	if err != nil {
		t.Fatalf("Failed to save host3: %v", err)
	}

	// List hosts for org1
	hosts, err := store.ListHosts(org1.ID)
	if err != nil {
		t.Fatalf("ListHosts() error = %v", err)
	}

	if len(hosts) != 2 {
		t.Errorf("ListHosts() returned %d hosts, want 2", len(hosts))
	}

	// Verify organization isolation
	hosts2, err := store.ListHosts(org2.ID)
	if err != nil {
		t.Fatalf("ListHosts() for org2 error = %v", err)
	}

	if len(hosts2) != 1 {
		t.Errorf("ListHosts() for org2 returned %d hosts, want 1", len(hosts2))
	}
}

// ============================================================================
// User Management Tests
// ============================================================================

func TestPostgresStorage_CreateUser(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	tests := []struct {
		name      string
		username  string
		email     string
		password  string
		orgID     string
		role      string
		wantErr   bool
		checkRole string
	}{
		{
			name:      "create admin user",
			username:  "admin",
			email:     "admin@example.com",
			password:  "password123",
			orgID:     org.ID,
			role:      "admin",
			wantErr:   false,
			checkRole: "admin",
		},
		{
			name:      "create editor user",
			username:  "editor",
			email:     "editor@example.com",
			password:  "password123",
			orgID:     org.ID,
			role:      "editor",
			wantErr:   false,
			checkRole: "editor",
		},
		{
			name:      "create viewer user",
			username:  "viewer",
			email:     "viewer@example.com",
			password:  "password123",
			orgID:     org.ID,
			role:      "viewer",
			wantErr:   false,
			checkRole: "viewer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := createTestUser(store, tt.username, tt.email, tt.password, tt.orgID, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if user.Username != tt.username {
					t.Errorf("CreateUser() username = %v, want %v", user.Username, tt.username)
				}
				if user.Email != tt.email {
					t.Errorf("CreateUser() email = %v, want %v", user.Email, tt.email)
				}
				if user.Role != tt.checkRole {
					t.Errorf("CreateUser() role = %v, want %v", user.Role, tt.checkRole)
				}
				if user.OrgID != tt.orgID {
					t.Errorf("CreateUser() orgID = %v, want %v", user.OrgID, tt.orgID)
				}
			}
		})
	}
}

func TestPostgresStorage_GetUserByUsername(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "password123", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name    string
		username string
		wantErr bool
		wantID  string
	}{
		{
			name:     "get existing user",
			username: "testuser",
			wantErr:  false,
			wantID:   user.ID,
		},
		{
			name:     "get non-existent user",
			username: "nonexistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, passwordHash, err := store.GetUserByUsername(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserByUsername() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.ID != tt.wantID {
					t.Errorf("GetUserByUsername() ID = %v, want %v", got.ID, tt.wantID)
				}
				if passwordHash == "" {
					t.Error("GetUserByUsername() passwordHash should not be empty")
				}
			} else {
				if err != ErrNotFound {
					t.Errorf("GetUserByUsername() expected ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPostgresStorage_GetUserByID(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name    string
		userID  string
		wantErr bool
		want    string
	}{
		{
			name:    "get existing user",
			userID:  user.ID,
			wantErr: false,
			want:    "testuser",
		},
		{
			name:    "get non-existent user",
			userID:  "00000000-0000-0000-0000-000000000999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetUserByID(tt.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.Username != tt.want {
					t.Errorf("GetUserByID() username = %v, want %v", got.Username, tt.want)
				}
			} else {
				if err != ErrNotFound {
					t.Errorf("GetUserByID() expected ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPostgresStorage_GetUserByEmail(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	_, err = createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name    string
		email   string
		wantErr bool
		want    string
	}{
		{
			name:    "get existing user",
			email:   "test@example.com",
			wantErr: false,
			want:    "testuser",
		},
		{
			name:    "get non-existent user",
			email:   "nonexistent@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetUserByEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserByEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.Username != tt.want {
					t.Errorf("GetUserByEmail() username = %v, want %v", got.Username, tt.want)
				}
			} else {
				if err != ErrNotFound {
					t.Errorf("GetUserByEmail() expected ErrNotFound, got %v", err)
				}
			}
		})
	}
}

// ============================================================================
// API Key Management Tests
// ============================================================================

func TestPostgresStorage_CreateAPIKey(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	plainKey, apiKey, err := createTestAPIKey(store, user.ID, "Test Key")
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	if apiKey.UserID != user.ID {
		t.Errorf("CreateAPIKey() UserID = %v, want %v", apiKey.UserID, user.ID)
	}

	if apiKey.Name != "Test Key" {
		t.Errorf("CreateAPIKey() Name = %v, want Test Key", apiKey.Name)
	}

	if plainKey == "" {
		t.Error("CreateAPIKey() plainKey should not be empty")
	}
}

func TestPostgresStorage_GetAPIKeyByPrefix(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create two API keys with the same prefix (shouldn't happen in practice, but test it)
	plainKey1, apiKey1, err := createTestAPIKey(store, user.ID, "Key 1")
	if err != nil {
		t.Fatalf("Failed to create API key 1: %v", err)
	}

	// Get keys by prefix
	prefix := plainKey1[:8]
	keys, err := store.GetAPIKeyByPrefix(prefix)
	if err != nil {
		t.Fatalf("GetAPIKeyByPrefix() error = %v", err)
	}

	if len(keys) == 0 {
		t.Error("GetAPIKeyByPrefix() should return at least one key")
	}

	found := false
	for _, key := range keys {
		if key.ID == apiKey1.ID {
			found = true
			break
		}
	}

	if !found {
		t.Error("GetAPIKeyByPrefix() should return the created key")
	}
}

func TestPostgresStorage_GetAPIKeysByUserID(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create multiple API keys
	_, _, err = createTestAPIKey(store, user.ID, "Key 1")
	if err != nil {
		t.Fatalf("Failed to create API key 1: %v", err)
	}

	_, _, err = createTestAPIKey(store, user.ID, "Key 2")
	if err != nil {
		t.Fatalf("Failed to create API key 2: %v", err)
	}

	keys, err := store.GetAPIKeysByUserID(user.ID)
	if err != nil {
		t.Fatalf("GetAPIKeysByUserID() error = %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("GetAPIKeysByUserID() returned %d keys, want 2", len(keys))
	}
}

func TestPostgresStorage_DeleteAPIKey(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	_, apiKey, err := createTestAPIKey(store, user.ID, "Test Key")
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	tests := []struct {
		name    string
		keyID   string
		wantErr bool
	}{
		{
			name:    "delete existing key",
			keyID:   apiKey.ID,
			wantErr: false,
		},
		{
			name:    "delete non-existent key",
			keyID:   "00000000-0000-0000-0000-000000000999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.DeleteAPIKey(tt.keyID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteAPIKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify key was deleted
				keys, err := store.GetAPIKeysByUserID(user.ID)
				if err != nil {
					t.Fatalf("GetAPIKeysByUserID() error = %v", err)
				}

				for _, key := range keys {
					if key.ID == tt.keyID {
						t.Error("DeleteAPIKey() key should be deleted")
					}
				}
			}
		})
	}
}

func TestPostgresStorage_UpdateAPIKeyLastUsed(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	user, err := createTestUser(store, "testuser", "test@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	_, apiKey, err := createTestAPIKey(store, user.ID, "Test Key")
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Update last used
	err = store.UpdateAPIKeyLastUsed(apiKey.ID)
	if err != nil {
		t.Fatalf("UpdateAPIKeyLastUsed() error = %v", err)
	}

	// Verify it was updated
	keys, err := store.GetAPIKeysByUserID(user.ID)
	if err != nil {
		t.Fatalf("GetAPIKeysByUserID() error = %v", err)
	}

	found := false
	for _, key := range keys {
		if key.ID == apiKey.ID {
			found = true
			if key.LastUsedAt == nil {
				t.Error("UpdateAPIKeyLastUsed() LastUsedAt should be set")
			}
			break
		}
	}

	if !found {
		t.Error("UpdateAPIKeyLastUsed() key not found")
	}
}

// ============================================================================
// Organization Tests
// ============================================================================

func TestPostgresStorage_CreateOrganization(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	tests := []struct {
		name    string
		orgName string
		wantErr bool
	}{
		{
			name:    "create organization",
			orgName: "Test Organization",
			wantErr: false,
		},
		{
			name:    "create another organization",
			orgName: "Another Organization",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, err := createTestOrg(store, tt.orgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateOrganization() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if org.Name != tt.orgName {
					t.Errorf("CreateOrganization() Name = %v, want %v", org.Name, tt.orgName)
				}
				if org.ID == "" {
					t.Error("CreateOrganization() ID should not be empty")
				}
			}
		})
	}
}

func TestPostgresStorage_GetOrganizationByID(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	tests := []struct {
		name    string
		orgID   string
		wantErr bool
		want    string
	}{
		{
			name:    "get existing organization",
			orgID:   org.ID,
			wantErr: false,
			want:    "Test Org",
		},
		{
			name:    "get non-existent organization",
			orgID:   "00000000-0000-0000-0000-000000000999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetOrganizationByID(tt.orgID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOrganizationByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.Name != tt.want {
					t.Errorf("GetOrganizationByID() Name = %v, want %v", got.Name, tt.want)
				}
			} else {
				if err != ErrNotFound {
					t.Errorf("GetOrganizationByID() expected ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPostgresStorage_GetOrganizationByName(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	tests := []struct {
		name    string
		orgName string
		wantErr bool
		wantID  string
	}{
		{
			name:    "get existing organization",
			orgName: "Test Org",
			wantErr: false,
			wantID:  org.ID,
		},
		{
			name:    "get non-existent organization",
			orgName: "Non-existent Org",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetOrganizationByName(tt.orgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOrganizationByName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.ID != tt.wantID {
					t.Errorf("GetOrganizationByName() ID = %v, want %v", got.ID, tt.wantID)
				}
			} else {
				if err != ErrNotFound {
					t.Errorf("GetOrganizationByName() expected ErrNotFound, got %v", err)
				}
			}
		})
	}
}

func TestPostgresStorage_CountUsersInOrganization(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	org, err := createTestOrg(store, "Test Org")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	// Create users
	_, err = createTestUser(store, "user1", "user1@example.com", "", org.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	_, err = createTestUser(store, "user2", "user2@example.com", "", org.ID, "editor")
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	count, err := store.CountUsersInOrganization(org.ID)
	if err != nil {
		t.Fatalf("CountUsersInOrganization() error = %v", err)
	}

	if count != 2 {
		t.Errorf("CountUsersInOrganization() = %v, want 2", count)
	}
}

// ============================================================================
// Organization Isolation Tests
// ============================================================================

func TestPostgresStorage_OrganizationIsolation(t *testing.T) {
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create two organizations
	org1, err := createTestOrg(store, "Org 1")
	if err != nil {
		t.Fatalf("Failed to create org1: %v", err)
	}

	org2, err := createTestOrg(store, "Org 2")
	if err != nil {
		t.Fatalf("Failed to create org2: %v", err)
	}

	// Create users in each org
	user1, err := createTestUser(store, "user1", "user1@example.com", "", org1.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2, err := createTestUser(store, "user2", "user2@example.com", "", org2.ID, "admin")
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Save hosts for each org
	report1 := createTestReport(testHostID1, "host1")
	report2 := createTestReport(testHostID2, "host2")

	err = store.SaveHost(report1, org1.ID, user1.ID)
	if err != nil {
		t.Fatalf("Failed to save host1: %v", err)
	}

	err = store.SaveHost(report2, org2.ID, user2.ID)
	if err != nil {
		t.Fatalf("Failed to save host2: %v", err)
	}

	// Verify org1 can only see its own hosts
	hosts1, err := store.ListHosts(org1.ID)
	if err != nil {
		t.Fatalf("ListHosts() for org1 error = %v", err)
	}
	if len(hosts1) != 1 {
		t.Errorf("ListHosts() for org1 returned %d hosts, want 1", len(hosts1))
	}

	// Verify org2 can only see its own hosts
	hosts2, err := store.ListHosts(org2.ID)
	if err != nil {
		t.Fatalf("ListHosts() for org2 error = %v", err)
	}
	if len(hosts2) != 1 {
		t.Errorf("ListHosts() for org2 returned %d hosts, want 1", len(hosts2))
	}

	// Verify org1 cannot access org2's host
	_, err = store.GetHost(testHostID2, org1.ID)
	if err != ErrNotFound {
		t.Errorf("GetHost() should return ErrNotFound when accessing other org's host, got %v", err)
	}

	// Verify users are isolated
	users1, err := store.ListUsersByOrganization(org1.ID)
	if err != nil {
		t.Fatalf("ListUsersByOrganization() for org1 error = %v", err)
	}
	if len(users1) != 1 {
		t.Errorf("ListUsersByOrganization() for org1 returned %d users, want 1", len(users1))
	}

	users2, err := store.ListUsersByOrganization(org2.ID)
	if err != nil {
		t.Fatalf("ListUsersByOrganization() for org2 error = %v", err)
	}
	if len(users2) != 1 {
		t.Errorf("ListUsersByOrganization() for org2 returned %d users, want 1", len(users2))
	}
}

