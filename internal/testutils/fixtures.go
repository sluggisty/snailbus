package testutils

import (
	"encoding/json"
	"fmt"
	"time"

	"snailbus/internal/auth"
	"snailbus/internal/models"
	"snailbus/internal/storage"
)

// CreateTestOrganization creates a test organization in the database.
// Returns the created organization or an error.
func CreateTestOrganization(store storage.Storage, name string) (*models.Organization, error) {
	org, err := store.CreateOrganization(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create test organization: %w", err)
	}
	return org, nil
}

// CreateTestUser creates a test user in the database.
// If password is empty, it defaults to "testpassword123".
// Returns the created user or an error.
func CreateTestUser(store storage.Storage, username, email, password, orgID, role string) (*models.User, error) {
	if password == "" {
		password = "testpassword123"
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := store.CreateUser(username, email, passwordHash, orgID, role)
	if err != nil {
		return nil, fmt.Errorf("failed to create test user: %w", err)
	}

	return user, nil
}

// CreateTestAPIKey creates a test API key for a user.
// Returns the plain API key (to use in requests) and the APIKey model, or an error.
func CreateTestAPIKey(store storage.Storage, userID, name string) (string, *models.APIKey, error) {
	plainKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	apiKey, err := store.CreateAPIKey(userID, keyHash, keyPrefix, name, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create test API key: %w", err)
	}

	return plainKey, apiKey, nil
}

// CreateTestReport creates a test report model (does not save to database).
// Returns a Report model with test data.
func CreateTestReport(hostID, hostname, orgID, userID string) *models.Report {
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

// CreateTestOrganizationWithUser creates a test organization and a test user in that organization.
// This is a convenience function for common test setup.
// Returns the organization, user, and their API key, or an error.
func CreateTestOrganizationWithUser(store storage.Storage, orgName, username, email, password, role string) (*models.Organization, *models.User, string, error) {
	// Create organization
	org, err := CreateTestOrganization(store, orgName)
	if err != nil {
		return nil, nil, "", err
	}

	// Create user
	user, err := CreateTestUser(store, username, email, password, org.ID, role)
	if err != nil {
		return nil, nil, "", err
	}

	// Create API key
	apiKey, _, err := CreateTestAPIKey(store, user.ID, "Test API Key")
	if err != nil {
		return nil, nil, "", err
	}

	return org, user, apiKey, nil
}

// CreateCompleteTestSetup creates a complete test setup with:
// - An organization
// - A user (admin role)
// - An API key for the user
// - A test host report saved to the database
// Returns all created entities or an error.
func CreateCompleteTestSetup(store storage.Storage, orgName, username, email string) (*models.Organization, *models.User, string, *models.Report, error) {
	// Create organization and user
	org, user, apiKey, err := CreateTestOrganizationWithUser(store, orgName, username, email, "", "admin")
	if err != nil {
		return nil, nil, "", nil, err
	}

	// Create and save a test report
	hostID := "test-host-id-12345"
	hostname := "test-host"
	report := CreateTestReport(hostID, hostname, org.ID, user.ID)

	err = store.SaveHost(report, org.ID, user.ID)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("failed to save test host: %w", err)
	}

	return org, user, apiKey, report, nil
}

