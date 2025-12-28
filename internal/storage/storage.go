package storage

import (
	"errors"
	"time"

	"snailbus/internal/models"
)

var (
	// ErrNotFound is returned when a requested resource is not found
	ErrNotFound = errors.New("not found")
)

// Storage defines the interface for storing and retrieving host reports
type Storage interface {
	// SaveHost stores or updates a host's report
	// orgID and uploadedByUserID are required and will be stored with the host
	SaveHost(report *models.Report, orgID, uploadedByUserID string) error

	// GetHost returns the full report data for a specific host by host_id (UUID)
	// Verifies that the host belongs to the specified organization
	GetHost(hostID, orgID string) (*models.Report, error)

	// DeleteHost removes a host by host_id (UUID)
	// Verifies that the host belongs to the specified organization before deletion
	DeleteHost(hostID, orgID string) error

	// ListHosts returns all hosts with summary info for the specified organization
	ListHosts(orgID string) ([]*models.HostSummary, error)

	// GetAllHosts returns all hosts with their full report data for the specified organization
	GetAllHosts(orgID string) ([]*models.Report, error)

	// Close closes the database connection
	Close() error

	// Auth methods
	CreateUser(username, email, passwordHash, orgID, role string) (*models.User, error)
	GetUserByUsername(username string) (*models.User, string, error) // Returns user and password hash
	GetUserByID(userID string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)

	CreateAPIKey(userID, keyHash, keyPrefix, name string, expiresAt *time.Time) (*models.APIKey, error)
	GetAPIKeyByPrefix(keyPrefix string) ([]*models.APIKey, error) // Returns all keys with this prefix
	GetAPIKeysByUserID(userID string) ([]*models.APIKey, error)
	DeleteAPIKey(keyID string) error
	UpdateAPIKeyLastUsed(keyID string) error

	// Organization methods
	CreateOrganization(name string) (*models.Organization, error)
	GetOrganizationByID(orgID string) (*models.Organization, error)
	GetOrganizationByName(name string) (*models.Organization, error)
	CountUsersInOrganization(orgID string) (int, error)

	// User management methods (admin-only)
	ListUsersByOrganization(orgID string) ([]*models.User, error)
	UpdateUserRole(userID, role string) error
	DeleteUser(userID string) error
}
