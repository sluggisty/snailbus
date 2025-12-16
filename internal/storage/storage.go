package storage

import (
	"errors"
	"snailbus/internal/models"
)

var (
	// ErrNotFound is returned when a requested resource is not found
	ErrNotFound = errors.New("not found")
)

// Storage defines the interface for storing and retrieving host reports
type Storage interface {
	// SaveHost stores or updates a host's report
	SaveHost(report *models.Report) error

	// GetHost returns the full report data for a specific host
	GetHost(hostname string) (*models.Report, error)

	// DeleteHost removes a host
	DeleteHost(hostname string) error

	// ListHosts returns all hosts with summary info
	ListHosts() ([]*models.HostSummary, error)

	// GetAllHosts returns all hosts with their full report data
	GetAllHosts() ([]*models.Report, error)

	// Close closes the database connection
	Close() error
}

