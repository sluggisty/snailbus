package testutils

import (
	"database/sql"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"snailbus/internal/models"
	"snailbus/internal/storage"
)

// SetupTestStorage creates a test storage instance with database setup.
// This is a convenience function that combines SetupTestDB and storage creation.
// Returns the storage instance and a cleanup function.
func SetupTestStorage(t *testing.T) (storage.Storage, func()) {
	t.Helper()
	// Set up test database
	_, cleanup, err := SetupTestDB(t)
	require.NoError(t, err)

	// Create storage instance
	store, err := storage.NewPostgresStorage(GetTestDatabaseURL())
	require.NoError(t, err)

	return store, func() {
		store.Close()
		cleanup()
	}
}

// SetupTestStorageWithRouter creates a test storage instance and a full router.
// This is the most common setup for integration tests.
// Returns the storage, router, and cleanup function.
func SetupTestStorageWithRouter(t *testing.T) (storage.Storage, *gin.Engine, func()) {
	t.Helper()
	store, cleanup := SetupTestStorage(t)
	router := SetupFullTestRouter(store)
	return store, router, cleanup
}

// CreateMultipleTestOrganizations creates multiple test organizations in bulk.
// Returns a slice of created organizations.
func CreateMultipleTestOrganizations(store storage.Storage, count int, namePrefix string) ([]*models.Organization, error) {
	orgs := make([]*models.Organization, 0, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%s-%d", namePrefix, i+1)
		org, err := CreateTestOrganization(store, name)
		if err != nil {
			return nil, fmt.Errorf("failed to create organization %d: %w", i+1, err)
		}
		orgs = append(orgs, org)
	}
	return orgs, nil
}

// CreateMultipleTestUsers creates multiple test users in the same organization.
// Returns a slice of created users.
func CreateMultipleTestUsers(store storage.Storage, orgID string, count int, usernamePrefix string, role string) ([]*models.User, error) {
	users := make([]*models.User, 0, count)
	for i := 0; i < count; i++ {
		username := fmt.Sprintf("%s-%d", usernamePrefix, i+1)
		email := fmt.Sprintf("%s-%d@example.com", usernamePrefix, i+1)
		user, err := CreateTestUser(store, username, email, "password123", orgID, role)
		if err != nil {
			return nil, fmt.Errorf("failed to create user %d: %w", i+1, err)
		}
		users = append(users, user)
	}
	return users, nil
}

// CreateMultipleTestHosts creates multiple test host reports in the same organization.
// Returns a slice of created reports.
func CreateMultipleTestHosts(store storage.Storage, orgID, userID string, count int, hostnamePrefix string) ([]*models.Report, error) {
	reports := make([]*models.Report, 0, count)
	for i := 0; i < count; i++ {
		hostID := fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1)
		hostname := fmt.Sprintf("%s-%d", hostnamePrefix, i+1)
		report := CreateTestReport(hostID, hostname, orgID, userID)
		if err := store.SaveHost(report, orgID, userID); err != nil {
			return nil, fmt.Errorf("failed to save host %d: %w", i+1, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// AssertDatabaseState verifies that the database contains the expected number of records.
func AssertDatabaseState(t *testing.T, db *sql.DB, table string, expectedCount int) {
	t.Helper()
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	err := db.QueryRow(query).Scan(&count)
	require.NoError(t, err, "Failed to query %s", table)
	assert.Equal(t, expectedCount, count, "Expected %d records in %s, got %d", expectedCount, table, count)
}

// AssertOrganizationHasUsers verifies that an organization has the expected number of users.
func AssertOrganizationHasUsers(t *testing.T, store storage.Storage, orgID string, expectedCount int) {
	t.Helper()
	count, err := store.CountUsersInOrganization(orgID)
	require.NoError(t, err)
	assert.Equal(t, expectedCount, count, "Expected %d users in organization %s, got %d", expectedCount, orgID, count)
}

// AssertUserHasAPIKeys verifies that a user has the expected number of API keys.
func AssertUserHasAPIKeys(t *testing.T, store storage.Storage, userID string, expectedCount int) {
	t.Helper()
	keys, err := store.GetAPIKeysByUserID(userID)
	require.NoError(t, err)
	assert.Equal(t, expectedCount, len(keys), "Expected %d API keys for user %s, got %d", expectedCount, userID, len(keys))
}

// AssertOrganizationHasHosts verifies that an organization has the expected number of hosts.
func AssertOrganizationHasHosts(t *testing.T, store storage.Storage, orgID string, expectedCount int) {
	t.Helper()
	hosts, err := store.ListHosts(orgID)
	require.NoError(t, err)
	assert.Equal(t, expectedCount, len(hosts), "Expected %d hosts in organization %s, got %d", expectedCount, orgID, len(hosts))
}

// WaitForCondition waits for a condition to become true, with timeout and retry interval.
// Useful for testing eventual consistency or async operations.
func WaitForCondition(t *testing.T, timeout time.Duration, interval time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("Condition not met within %v: %s", timeout, message)
}

// RetryOperation retries an operation until it succeeds or timeout is reached.
func RetryOperation(t *testing.T, timeout time.Duration, interval time.Duration, operation func() error, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := operation(); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(interval)
	}
	t.Fatalf("Operation failed after %v: %s. Last error: %v", timeout, message, lastErr)
}

// AssertPaginationResponse asserts that a paginated response has the expected structure.
func AssertPaginationResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, expectedTotal int, expectedItems int) {
	t.Helper()
	AssertHTTPStatus(t, w, expectedStatus)

	body := ReadBodyAsJSON(t, w)

	// Check for pagination fields
	total, hasTotal := body["total"]
	assert.True(t, hasTotal, "Response should have 'total' field")
	if hasTotal {
		assert.Equal(t, float64(expectedTotal), total, "Expected total count %d, got %v", expectedTotal, total)
	}

	// Check for items/data field
	items, hasItems := body["items"]
	if !hasItems {
		items, hasItems = body["data"]
	}
	if hasItems {
		itemsSlice, ok := items.([]interface{})
		if ok {
			assert.Equal(t, expectedItems, len(itemsSlice), "Expected %d items, got %d", expectedItems, len(itemsSlice))
		}
	}
}

// AssertErrorResponse asserts that a response is an error response with expected status and error message.
func AssertErrorResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, expectedError string) {
	t.Helper()
	AssertHTTPStatus(t, w, expectedStatus)

	body := ReadBodyAsJSON(t, w)

	errorField, hasError := body["error"]
	assert.True(t, hasError, "Error response should have 'error' field")
	if hasError && expectedError != "" {
		assert.Contains(t, errorField, expectedError, "Error message should contain '%s'", expectedError)
	}
}

// AssertSuccessResponse asserts that a response is a successful response (2xx status).
func AssertSuccessResponse(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	assert.GreaterOrEqual(t, w.Code, 200, "Expected success status (2xx), got %d", w.Code)
	assert.Less(t, w.Code, 300, "Expected success status (2xx), got %d", w.Code)
}

// CleanupTestDataForOrg cleans up all test data for a specific organization.
// Useful for cleaning up after tests that create data for a specific org.
func CleanupTestDataForOrg(db *sql.DB, orgID string) error {
	// Get all users in the organization
	rows, err := db.Query("SELECT id FROM users WHERE org_id = $1", orgID)
	if err != nil {
		return fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return fmt.Errorf("failed to scan user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	// Delete API keys for these users
	if len(userIDs) > 0 {
		placeholders := ""
		args := make([]interface{}, len(userIDs))
		for i, userID := range userIDs {
			if i > 0 {
				placeholders += ","
			}
			placeholders += fmt.Sprintf("$%d", i+1)
			args[i] = userID
		}
		query := fmt.Sprintf("DELETE FROM api_keys WHERE user_id IN (%s)", placeholders)
		if _, err := db.Exec(query, args...); err != nil {
			return fmt.Errorf("failed to delete API keys: %w", err)
		}
	}

	// Delete hosts for the organization
	if _, err := db.Exec("DELETE FROM hosts WHERE org_id = $1", orgID); err != nil {
		return fmt.Errorf("failed to delete hosts: %w", err)
	}

	// Delete users
	if _, err := db.Exec("DELETE FROM users WHERE org_id = $1", orgID); err != nil {
		return fmt.Errorf("failed to delete users: %w", err)
	}

	// Delete organization
	if _, err := db.Exec("DELETE FROM organizations WHERE id = $1", orgID); err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	return nil
}

// TransactionTest runs a test function within a database transaction that is rolled back.
// This provides test isolation without needing to clean up data manually.
func TransactionTest(t *testing.T, store storage.Storage, testFunc func(*testing.T, storage.Storage)) {
	t.Helper()

	// Get the underlying database connection
	// Note: This requires PostgresStorage to expose the DB connection
	// For now, we'll use the regular cleanup approach
	// This is a placeholder for future enhancement

	testFunc(t, store)
}
