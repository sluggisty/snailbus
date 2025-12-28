package storage

import (
	"strings"
	"sync"
	"time"

	"snailbus/internal/models"
)

// MockStorage is a mock implementation of the Storage interface for testing
type MockStorage struct {
	mu sync.RWMutex

	// Hosts storage
	hosts map[string]*models.Report // key: hostID
	hostsByOrg map[string][]string // orgID -> []hostID

	// Users storage
	users map[string]*models.User // key: userID
	usersByUsername map[string]string // username -> userID
	usersByEmail map[string]string // email -> userID
	usersByOrg map[string][]string // orgID -> []userID
	passwords map[string]string // userID -> passwordHash

	// API Keys storage
	apiKeys map[string]*models.APIKey // key: apiKeyID
	apiKeysByUser map[string][]string // userID -> []apiKeyID
	apiKeysByPrefix map[string][]string // prefix -> []apiKeyID

	// Organizations storage
	organizations map[string]*models.Organization // key: orgID
	organizationsByName map[string]string // name -> orgID

	// Error injection
	shouldErrorOnSaveHost bool
	shouldErrorOnGetHost bool
	shouldErrorOnDeleteHost bool
	shouldErrorOnListHosts bool
	shouldErrorOnCreateUser bool
	shouldErrorOnGetUser bool
	shouldErrorOnCreateAPIKey bool
	shouldErrorOnCreateOrg bool
}

// NewMockStorage creates a new mock storage instance
func NewMockStorage() *MockStorage {
	return &MockStorage{
		hosts: make(map[string]*models.Report),
		hostsByOrg: make(map[string][]string),
		users: make(map[string]*models.User),
		usersByUsername: make(map[string]string),
		usersByEmail: make(map[string]string),
		usersByOrg: make(map[string][]string),
		passwords: make(map[string]string),
		apiKeys: make(map[string]*models.APIKey),
		apiKeysByUser: make(map[string][]string),
		apiKeysByPrefix: make(map[string][]string),
		organizations: make(map[string]*models.Organization),
		organizationsByName: make(map[string]string),
	}
}

// SaveHost stores or updates a host's report
func (m *MockStorage) SaveHost(report *models.Report, orgID, uploadedByUserID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldErrorOnSaveHost {
		return ErrNotFound
	}

	// Check if host exists and verify org_id matches
	if existing, exists := m.hosts[report.Meta.HostID]; exists {
		// Find which org it belongs to
		for org, hostIDs := range m.hostsByOrg {
			for _, hid := range hostIDs {
				if hid == report.Meta.HostID {
					if org != orgID {
						return ErrNotFound
					}
					break
				}
			}
		}
		_ = existing // Use existing to avoid unused variable
	}

	// Store host
	m.hosts[report.Meta.HostID] = report

	// Update org mapping
	if _, exists := m.hostsByOrg[orgID]; !exists {
		m.hostsByOrg[orgID] = []string{}
	}
	// Remove from old org if exists
	for org, hostIDs := range m.hostsByOrg {
		newHostIDs := []string{}
		for _, hid := range hostIDs {
			if hid != report.Meta.HostID {
				newHostIDs = append(newHostIDs, hid)
			}
		}
		m.hostsByOrg[org] = newHostIDs
	}
	// Add to new org
	found := false
	for _, hid := range m.hostsByOrg[orgID] {
		if hid == report.Meta.HostID {
			found = true
			break
		}
	}
	if !found {
		m.hostsByOrg[orgID] = append(m.hostsByOrg[orgID], report.Meta.HostID)
	}

	return nil
}

// GetHost returns the full report data for a specific host
func (m *MockStorage) GetHost(hostID, orgID string) (*models.Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldErrorOnGetHost {
		return nil, ErrNotFound
	}

	report, exists := m.hosts[hostID]
	if !exists {
		return nil, ErrNotFound
	}

	// Verify org_id
	hostIDs, exists := m.hostsByOrg[orgID]
	if !exists {
		return nil, ErrNotFound
	}
	for _, hid := range hostIDs {
		if hid == hostID {
			return report, nil
		}
	}

	return nil, ErrNotFound
}

// DeleteHost removes a host
func (m *MockStorage) DeleteHost(hostID, orgID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldErrorOnDeleteHost {
		return ErrNotFound
	}

	// Verify org_id
	hostIDs, exists := m.hostsByOrg[orgID]
	if !exists {
		return ErrNotFound
	}
	found := false
	for _, hid := range hostIDs {
		if hid == hostID {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}

	// Delete host
	delete(m.hosts, hostID)

	// Remove from org mapping
	newHostIDs := []string{}
	for _, hid := range hostIDs {
		if hid != hostID {
			newHostIDs = append(newHostIDs, hid)
		}
	}
	m.hostsByOrg[orgID] = newHostIDs

	return nil
}

// ListHosts returns all hosts with summary info for the specified organization
func (m *MockStorage) ListHosts(orgID string) ([]*models.HostSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldErrorOnListHosts {
		return nil, ErrNotFound
	}

	hostIDs, exists := m.hostsByOrg[orgID]
	if !exists {
		return []*models.HostSummary{}, nil
	}

	hosts := []*models.HostSummary{}
	for _, hostID := range hostIDs {
		report, exists := m.hosts[hostID]
		if !exists {
			continue
		}

		host := &models.HostSummary{
			HostID:   report.Meta.HostID,
			Hostname: report.Meta.Hostname,
			LastSeen: report.ReceivedAt,
		}
		hosts = append(hosts, host)
	}

	return hosts, nil
}

// GetAllHosts returns all hosts with their full report data
func (m *MockStorage) GetAllHosts(orgID string) ([]*models.Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hostIDs, exists := m.hostsByOrg[orgID]
	if !exists {
		return []*models.Report{}, nil
	}

	reports := []*models.Report{}
	for _, hostID := range hostIDs {
		report, exists := m.hosts[hostID]
		if exists {
			reports = append(reports, report)
		}
	}

	return reports, nil
}

// Close closes the database connection
func (m *MockStorage) Close() error {
	return nil
}

// CreateUser creates a new user
func (m *MockStorage) CreateUser(username, email, passwordHash, orgID, role string) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldErrorOnCreateUser {
		return nil, ErrNotFound
	}

	// Check if username already exists
	if _, exists := m.usersByUsername[username]; exists {
		return nil, ErrNotFound
	}

	// Check if email already exists
	if _, exists := m.usersByEmail[email]; exists {
		return nil, ErrNotFound
	}

	userID := "user-" + username // Simple ID generation for mock
	user := &models.User{
		ID:        userID,
		Username:  username,
		Email:     email,
		IsActive:  true,
		IsAdmin:   role == "admin",
		OrgID:     orgID,
		Role:      role,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.users[userID] = user
	m.usersByUsername[username] = userID
	m.usersByEmail[email] = userID
	m.passwords[userID] = passwordHash

	if _, exists := m.usersByOrg[orgID]; !exists {
		m.usersByOrg[orgID] = []string{}
	}
	m.usersByOrg[orgID] = append(m.usersByOrg[orgID], userID)

	return user, nil
}

// GetUserByUsername retrieves a user by username
func (m *MockStorage) GetUserByUsername(username string) (*models.User, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldErrorOnGetUser {
		return nil, "", ErrNotFound
	}

	userID, exists := m.usersByUsername[username]
	if !exists {
		return nil, "", ErrNotFound
	}

	user, exists := m.users[userID]
	if !exists {
		return nil, "", ErrNotFound
	}

	passwordHash := m.passwords[userID]
	return user, passwordHash, nil
}

// GetUserByID retrieves a user by ID
func (m *MockStorage) GetUserByID(userID string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldErrorOnGetUser {
		return nil, ErrNotFound
	}

	user, exists := m.users[userID]
	if !exists {
		return nil, ErrNotFound
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (m *MockStorage) GetUserByEmail(email string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldErrorOnGetUser {
		return nil, ErrNotFound
	}

	userID, exists := m.usersByEmail[email]
	if !exists {
		return nil, ErrNotFound
	}

	user, exists := m.users[userID]
	if !exists {
		return nil, ErrNotFound
	}

	return user, nil
}

// CreateAPIKey creates a new API key
func (m *MockStorage) CreateAPIKey(userID, keyHash, keyPrefix, name string, expiresAt *time.Time) (*models.APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldErrorOnCreateAPIKey {
		return nil, ErrNotFound
	}

	// Generate a simple ID (replace spaces to avoid URL issues)
	keyID := "key-" + name
	// Replace spaces with dashes for URL safety
	keyID = strings.ReplaceAll(keyID, " ", "-")
	apiKey := &models.APIKey{
		ID:         keyID,
		UserID:     userID,
		KeyHash:    keyHash,
		KeyPrefix:  keyPrefix,
		Name:       name,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now(),
	}

	m.apiKeys[keyID] = apiKey

	if _, exists := m.apiKeysByUser[userID]; !exists {
		m.apiKeysByUser[userID] = []string{}
	}
	m.apiKeysByUser[userID] = append(m.apiKeysByUser[userID], keyID)

	if _, exists := m.apiKeysByPrefix[keyPrefix]; !exists {
		m.apiKeysByPrefix[keyPrefix] = []string{}
	}
	m.apiKeysByPrefix[keyPrefix] = append(m.apiKeysByPrefix[keyPrefix], keyID)

	return apiKey, nil
}

// GetAPIKeyByPrefix retrieves API keys by prefix
func (m *MockStorage) GetAPIKeyByPrefix(keyPrefix string) ([]*models.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyIDs, exists := m.apiKeysByPrefix[keyPrefix]
	if !exists {
		return []*models.APIKey{}, nil
	}

	keys := []*models.APIKey{}
	for _, keyID := range keyIDs {
		if key, exists := m.apiKeys[keyID]; exists {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// GetAPIKeysByUserID retrieves all API keys for a user
func (m *MockStorage) GetAPIKeysByUserID(userID string) ([]*models.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyIDs, exists := m.apiKeysByUser[userID]
	if !exists {
		return []*models.APIKey{}, nil
	}

	keys := []*models.APIKey{}
	for _, keyID := range keyIDs {
		if key, exists := m.apiKeys[keyID]; exists {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// DeleteAPIKey deletes an API key
func (m *MockStorage) DeleteAPIKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.apiKeys[keyID]
	if !exists {
		return ErrNotFound
	}

	// Remove from user mapping
	keyIDs := m.apiKeysByUser[key.UserID]
	newKeyIDs := []string{}
	for _, kid := range keyIDs {
		if kid != keyID {
			newKeyIDs = append(newKeyIDs, kid)
		}
	}
	m.apiKeysByUser[key.UserID] = newKeyIDs

	// Remove from prefix mapping
	keyIDs = m.apiKeysByPrefix[key.KeyPrefix]
	newKeyIDs = []string{}
	for _, kid := range keyIDs {
		if kid != keyID {
			newKeyIDs = append(newKeyIDs, kid)
		}
	}
	m.apiKeysByPrefix[key.KeyPrefix] = newKeyIDs

	delete(m.apiKeys, keyID)
	return nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp
func (m *MockStorage) UpdateAPIKeyLastUsed(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.apiKeys[keyID]
	if !exists {
		return ErrNotFound
	}

	now := time.Now()
	key.LastUsedAt = &now
	return nil
}

// CreateOrganization creates a new organization
func (m *MockStorage) CreateOrganization(name string) (*models.Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldErrorOnCreateOrg {
		return nil, ErrNotFound
	}

	// Check if name already exists
	if _, exists := m.organizationsByName[name]; exists {
		return nil, ErrNotFound
	}

	orgID := "org-" + name // Simple ID generation
	org := &models.Organization{
		ID:        orgID,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.organizations[orgID] = org
	m.organizationsByName[name] = orgID

	return org, nil
}

// GetOrganizationByID retrieves an organization by ID
func (m *MockStorage) GetOrganizationByID(orgID string) (*models.Organization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	org, exists := m.organizations[orgID]
	if !exists {
		return nil, ErrNotFound
	}

	return org, nil
}

// GetOrganizationByName retrieves an organization by name
func (m *MockStorage) GetOrganizationByName(name string) (*models.Organization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orgID, exists := m.organizationsByName[name]
	if !exists {
		return nil, ErrNotFound
	}

	org, exists := m.organizations[orgID]
	if !exists {
		return nil, ErrNotFound
	}

	return org, nil
}

// CountUsersInOrganization counts the number of users in an organization
func (m *MockStorage) CountUsersInOrganization(orgID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userIDs, exists := m.usersByOrg[orgID]
	if !exists {
		return 0, nil
	}

	return len(userIDs), nil
}

// ListUsersByOrganization lists all users in an organization
func (m *MockStorage) ListUsersByOrganization(orgID string) ([]*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userIDs, exists := m.usersByOrg[orgID]
	if !exists {
		return []*models.User{}, nil
	}

	users := []*models.User{}
	for _, userID := range userIDs {
		if user, exists := m.users[userID]; exists {
			users = append(users, user)
		}
	}

	return users, nil
}

// UpdateUserRole updates a user's role
func (m *MockStorage) UpdateUserRole(userID, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[userID]
	if !exists {
		return ErrNotFound
	}

	user.Role = role
	user.IsAdmin = role == "admin"
	user.UpdatedAt = time.Now()

	return nil
}

// DeleteUser deletes a user
func (m *MockStorage) DeleteUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[userID]
	if !exists {
		return ErrNotFound
	}

	// Remove from username mapping
	delete(m.usersByUsername, user.Username)

	// Remove from email mapping
	delete(m.usersByEmail, user.Email)

	// Remove from org mapping
	userIDs := m.usersByOrg[user.OrgID]
	newUserIDs := []string{}
	for _, uid := range userIDs {
		if uid != userID {
			newUserIDs = append(newUserIDs, uid)
		}
	}
	m.usersByOrg[user.OrgID] = newUserIDs

	delete(m.users, userID)
	delete(m.passwords, userID)

	return nil
}

