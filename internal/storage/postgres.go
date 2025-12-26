package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
	"snailbus/internal/models"
)

// PostgresStorage implements Storage using PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage creates a new PostgreSQL-backed storage
func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ps := &PostgresStorage{
		db: db,
	}

	return ps, nil
}

// SaveHost stores or updates a host's report (replaces any previous report)
// If the host already exists, it verifies that org_id matches before updating
func (ps *PostgresStorage) SaveHost(report *models.Report, orgID, uploadedByUserID string) error {
	// First, check if host exists and verify org_id matches
	var existingOrgID string
	checkQuery := `SELECT org_id FROM hosts WHERE host_id = $1`
	err := ps.db.QueryRow(checkQuery, report.Meta.HostID).Scan(&existingOrgID)
	
	if err == nil {
		// Host exists - verify org_id matches
		if existingOrgID != orgID {
			return fmt.Errorf("host belongs to a different organization: %w", ErrNotFound)
		}
	} else if err != sql.ErrNoRows {
		// Unexpected error
		return fmt.Errorf("failed to check existing host: %w", err)
	}
	// If err == sql.ErrNoRows, host doesn't exist, proceed with insert

	// Use INSERT with ON CONFLICT
	// Note: We've already verified org_id matches above if the host exists
	query := `
		INSERT INTO hosts (host_id, hostname, received_at, collection_id, timestamp, snail_version, data, errors, org_id, uploaded_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (host_id) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			received_at = EXCLUDED.received_at,
			collection_id = EXCLUDED.collection_id,
			timestamp = EXCLUDED.timestamp,
			snail_version = EXCLUDED.snail_version,
			data = EXCLUDED.data,
			errors = EXCLUDED.errors,
			org_id = EXCLUDED.org_id,
			uploaded_by_user_id = EXCLUDED.uploaded_by_user_id
	`

	var errors []string
	if report.Errors != nil {
		errors = report.Errors
	}

	_, err := ps.db.Exec(query,
		report.Meta.HostID,
		report.Meta.Hostname,
		report.ReceivedAt,
		report.Meta.CollectionID,
		report.Meta.Timestamp,
		report.Meta.SnailVersion,
		report.Data,
		pq.Array(errors),
		orgID,
		uploadedByUserID,
	)

	if err != nil {
		return fmt.Errorf("failed to save host: %w", err)
	}

	return nil
}

// GetHost returns the full report data for a specific host (by host_id UUID)
// Verifies that the host belongs to the specified organization
func (ps *PostgresStorage) GetHost(hostID, orgID string) (*models.Report, error) {
	query := `
		SELECT host_id, hostname, received_at, collection_id, timestamp, snail_version, data, errors
		FROM hosts
		WHERE host_id = $1 AND org_id = $2
	`

	report := &models.Report{}
	var errors []string

	err := ps.db.QueryRow(query, hostID, orgID).Scan(
		&report.Meta.HostID,
		&report.Meta.Hostname,
		&report.ReceivedAt,
		&report.Meta.CollectionID,
		&report.Meta.Timestamp,
		&report.Meta.SnailVersion,
		&report.Data,
		pq.Array(&errors),
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	report.ID = report.Meta.HostID // Use host_id as ID
	report.Errors = errors
	return report, nil
}

// DeleteHost removes a host by host_id
// Verifies that the host belongs to the specified organization before deletion
func (ps *PostgresStorage) DeleteHost(hostID, orgID string) error {
	result, err := ps.db.Exec("DELETE FROM hosts WHERE host_id = $1 AND org_id = $2", hostID, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete host: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// ListHosts returns all hosts with summary info for the specified organization
func (ps *PostgresStorage) ListHosts(orgID string) ([]*models.HostSummary, error) {
	query := `
		SELECT host_id, hostname, received_at, data, org_id, uploaded_by_user_id
		FROM hosts
		WHERE org_id = $1
		ORDER BY received_at DESC
	`

	rows, err := ps.db.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %w", err)
	}
	defer rows.Close()

	var hosts []*models.HostSummary

	for rows.Next() {
		var hostID string
		var hostname string
		var receivedAt time.Time
		var dataJSON []byte
		var orgID string
		var uploadedByUserID string

		if err := rows.Scan(&hostID, &hostname, &receivedAt, &dataJSON, &orgID, &uploadedByUserID); err != nil {
			return nil, fmt.Errorf("failed to scan host: %w", err)
		}

		// Extract OS info from JSONB data
		var osName, osVersion, osVersionMajor, osVersionMinor, osVersionPatch string
		var data map[string]interface{}
		if err := json.Unmarshal(dataJSON, &data); err == nil {
			if system, ok := data["system"].(map[string]interface{}); ok {
				if os, ok := system["os"].(map[string]interface{}); ok {
					if name, ok := os["name"].(string); ok {
						osName = name
					}
					if version, ok := os["version"].(string); ok {
						osVersion = version
					}
					// Extract version components
					if major, ok := os["version_major"].(string); ok && major != "" {
						osVersionMajor = major
					}
					if minor, ok := os["version_minor"].(string); ok && minor != "" {
						osVersionMinor = minor
					}
					if patch, ok := os["version_patch"].(string); ok && patch != "" {
						osVersionPatch = patch
					}
				}
			}
		}

		host := &models.HostSummary{
			HostID:           hostID,
			Hostname:         hostname,
			OSName:           osName,
			OSVersion:        osVersion,
			OSVersionMajor:   osVersionMajor,
			OSVersionMinor:   osVersionMinor,
			OSVersionPatch:   osVersionPatch,
			OrgID:            orgID,
			UploadedByUserID: uploadedByUserID,
			LastSeen:         receivedAt,
		}

		hosts = append(hosts, host)
	}

	return hosts, nil
}

// GetAllHosts returns all hosts with their full report data for the specified organization
func (ps *PostgresStorage) GetAllHosts(orgID string) ([]*models.Report, error) {
	query := `
		SELECT host_id, hostname, received_at, collection_id, timestamp, snail_version, data, errors
		FROM hosts
		WHERE org_id = $1
		ORDER BY received_at DESC
	`

	rows, err := ps.db.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all hosts: %w", err)
	}
	defer rows.Close()

	var reports []*models.Report
	for rows.Next() {
		report := &models.Report{}
		var errors []string

		if err := rows.Scan(
			&report.Meta.HostID,
			&report.Meta.Hostname,
			&report.ReceivedAt,
			&report.Meta.CollectionID,
			&report.Meta.Timestamp,
			&report.Meta.SnailVersion,
			&report.Data,
			pq.Array(&errors),
		); err != nil {
			return nil, fmt.Errorf("failed to scan report: %w", err)
		}

		report.ID = report.Meta.HostID
		report.Errors = errors
		reports = append(reports, report)
	}

	return reports, nil
}

// Close closes the database connection
func (ps *PostgresStorage) Close() error {
	return ps.db.Close()
}

// Auth methods

// CreateUser creates a new user
func (ps *PostgresStorage) CreateUser(username, email, passwordHash, orgID, role string) (*models.User, error) {
	query := `
		INSERT INTO users (username, email, password_hash, org_id, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, username, email, is_active, is_admin, org_id, role, created_at, updated_at
	`

	user := &models.User{}
	err := ps.db.QueryRow(query, username, email, passwordHash, orgID, role).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.IsActive,
		&user.IsAdmin,
		&user.OrgID,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByUsername retrieves a user by username and returns the password hash
func (ps *PostgresStorage) GetUserByUsername(username string) (*models.User, string, error) {
	query := `
		SELECT id, username, email, password_hash, is_active, is_admin, org_id, role, created_at, updated_at
		FROM users
		WHERE username = $1
	`

	user := &models.User{}
	var passwordHash string

	err := ps.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&passwordHash,
		&user.IsActive,
		&user.IsAdmin,
		&user.OrgID,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}

	return user, passwordHash, nil
}

// GetUserByID retrieves a user by ID
func (ps *PostgresStorage) GetUserByID(userID string) (*models.User, error) {
	query := `
		SELECT id, username, email, is_active, is_admin, org_id, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	user := &models.User{}
	err := ps.db.QueryRow(query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.IsActive,
		&user.IsAdmin,
		&user.OrgID,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (ps *PostgresStorage) GetUserByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, username, email, is_active, is_admin, org_id, role, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	user := &models.User{}
	err := ps.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.IsActive,
		&user.IsAdmin,
		&user.OrgID,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// CreateAPIKey creates a new API key
func (ps *PostgresStorage) CreateAPIKey(userID, keyHash, keyPrefix, name string, expiresAt *time.Time) (*models.APIKey, error) {
	query := `
		INSERT INTO api_keys (user_id, key_hash, key_prefix, name, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, name, last_used_at, expires_at, created_at
	`

	apiKey := &models.APIKey{}
	err := ps.db.QueryRow(query, userID, keyHash, keyPrefix, name, expiresAt).Scan(
		&apiKey.ID,
		&apiKey.UserID,
		&apiKey.Name,
		&apiKey.LastUsedAt,
		&apiKey.ExpiresAt,
		&apiKey.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

// GetAPIKeyByPrefix retrieves API keys by prefix (for efficient lookup)
func (ps *PostgresStorage) GetAPIKeyByPrefix(keyPrefix string) ([]*models.APIKey, error) {
	query := `
		SELECT id, user_id, key_hash, key_prefix, name, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE key_prefix = $1
	`

	rows, err := ps.db.Query(query, keyPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var apiKeys []*models.APIKey
	for rows.Next() {
		apiKey := &models.APIKey{}
		err := rows.Scan(
			&apiKey.ID,
			&apiKey.UserID,
			&apiKey.KeyHash,
			&apiKey.KeyPrefix,
			&apiKey.Name,
			&apiKey.LastUsedAt,
			&apiKey.ExpiresAt,
			&apiKey.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		apiKeys = append(apiKeys, apiKey)
	}

	return apiKeys, nil
}

// GetAPIKeysByUserID retrieves all API keys for a user
func (ps *PostgresStorage) GetAPIKeysByUserID(userID string) ([]*models.APIKey, error) {
	query := `
		SELECT id, user_id, name, last_used_at, expires_at, created_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := ps.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var apiKeys []*models.APIKey
	for rows.Next() {
		apiKey := &models.APIKey{}
		err := rows.Scan(
			&apiKey.ID,
			&apiKey.UserID,
			&apiKey.Name,
			&apiKey.LastUsedAt,
			&apiKey.ExpiresAt,
			&apiKey.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		apiKeys = append(apiKeys, apiKey)
	}

	return apiKeys, nil
}

// DeleteAPIKey deletes an API key
func (ps *PostgresStorage) DeleteAPIKey(keyID string) error {
	result, err := ps.db.Exec("DELETE FROM api_keys WHERE id = $1", keyID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key
func (ps *PostgresStorage) UpdateAPIKeyLastUsed(keyID string) error {
	_, err := ps.db.Exec(
		"UPDATE api_keys SET last_used_at = NOW() WHERE id = $1",
		keyID,
	)
	if err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}
	return nil
}

// Organization methods

// CreateOrganization creates a new organization
func (ps *PostgresStorage) CreateOrganization(name string) (*models.Organization, error) {
	query := `
		INSERT INTO organizations (name)
		VALUES ($1)
		RETURNING id, name, created_at, updated_at
	`

	org := &models.Organization{}
	err := ps.db.QueryRow(query, name).Scan(
		&org.ID,
		&org.Name,
		&org.CreatedAt,
		&org.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return org, nil
}

// GetOrganizationByID retrieves an organization by ID
func (ps *PostgresStorage) GetOrganizationByID(orgID string) (*models.Organization, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM organizations
		WHERE id = $1
	`

	org := &models.Organization{}
	err := ps.db.QueryRow(query, orgID).Scan(
		&org.ID,
		&org.Name,
		&org.CreatedAt,
		&org.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return org, nil
}

// GetOrganizationByName retrieves an organization by name
func (ps *PostgresStorage) GetOrganizationByName(name string) (*models.Organization, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM organizations
		WHERE name = $1
	`

	org := &models.Organization{}
	err := ps.db.QueryRow(query, name).Scan(
		&org.ID,
		&org.Name,
		&org.CreatedAt,
		&org.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return org, nil
}

// CountUsersInOrganization counts the number of users in an organization
func (ps *PostgresStorage) CountUsersInOrganization(orgID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM users
		WHERE org_id = $1
	`

	var count int
	err := ps.db.QueryRow(query, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users in organization: %w", err)
	}

	return count, nil
}

// ListUsersByOrganization lists all users in an organization
func (ps *PostgresStorage) ListUsersByOrganization(orgID string) ([]*models.User, error) {
	query := `
		SELECT id, username, email, is_active, is_admin, org_id, role, created_at, updated_at
		FROM users
		WHERE org_id = $1
		ORDER BY created_at ASC
	`

	rows, err := ps.db.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.IsActive,
			&user.IsAdmin,
			&user.OrgID,
			&user.Role,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// UpdateUserRole updates a user's role
func (ps *PostgresStorage) UpdateUserRole(userID, role string) error {
	query := `
		UPDATE users
		SET role = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := ps.db.Exec(query, role, userID)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteUser deletes a user by ID
func (ps *PostgresStorage) DeleteUser(userID string) error {
	result, err := ps.db.Exec("DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

