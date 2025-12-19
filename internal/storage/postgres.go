package storage

import (
	"database/sql"
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
func (ps *PostgresStorage) SaveHost(report *models.Report) error {
	query := `
		INSERT INTO hosts (host_id, hostname, received_at, collection_id, timestamp, snail_version, data, errors)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (host_id) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			received_at = EXCLUDED.received_at,
			collection_id = EXCLUDED.collection_id,
			timestamp = EXCLUDED.timestamp,
			snail_version = EXCLUDED.snail_version,
			data = EXCLUDED.data,
			errors = EXCLUDED.errors
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
	)

	if err != nil {
		return fmt.Errorf("failed to save host: %w", err)
	}

	return nil
}

// GetHost returns the full report data for a specific host (by host_id UUID)
func (ps *PostgresStorage) GetHost(hostID string) (*models.Report, error) {
	query := `
		SELECT host_id, hostname, received_at, collection_id, timestamp, snail_version, data, errors
		FROM hosts
		WHERE host_id = $1
	`

	report := &models.Report{}
	var errors []string

	err := ps.db.QueryRow(query, hostID).Scan(
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

// GetHostByHostname returns the full report data for a specific host by hostname
// This is useful for backward compatibility or when you only know the hostname
func (ps *PostgresStorage) GetHostByHostname(hostname string) (*models.Report, error) {
	query := `
		SELECT host_id, hostname, received_at, collection_id, timestamp, snail_version, data, errors
		FROM hosts
		WHERE hostname = $1
	`

	report := &models.Report{}
	var errors []string

	err := ps.db.QueryRow(query, hostname).Scan(
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
		return nil, fmt.Errorf("failed to get host by hostname: %w", err)
	}

	report.ID = report.Meta.HostID
	report.Errors = errors
	return report, nil
}

// DeleteHost removes a host by host_id
func (ps *PostgresStorage) DeleteHost(hostID string) error {
	result, err := ps.db.Exec("DELETE FROM hosts WHERE host_id = $1", hostID)
	if err != nil {
		return fmt.Errorf("failed to delete host: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// ListHosts returns all hosts with summary info
func (ps *PostgresStorage) ListHosts() ([]*models.HostSummary, error) {
	query := `
		SELECT host_id, hostname, received_at
		FROM hosts
		ORDER BY received_at DESC
	`

	rows, err := ps.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosts: %w", err)
	}
	defer rows.Close()

	var hosts []*models.HostSummary

	for rows.Next() {
		var hostID string
		var hostname string
		var receivedAt time.Time

		if err := rows.Scan(&hostID, &hostname, &receivedAt); err != nil {
			return nil, fmt.Errorf("failed to scan host: %w", err)
		}

		host := &models.HostSummary{
			HostID:   hostID,
			Hostname: hostname,
			LastSeen: receivedAt,
		}

		hosts = append(hosts, host)
	}

	return hosts, nil
}

// GetAllHosts returns all hosts with their full report data
func (ps *PostgresStorage) GetAllHosts() ([]*models.Report, error) {
	query := `
		SELECT host_id, hostname, received_at, collection_id, timestamp, snail_version, data, errors
		FROM hosts
		ORDER BY received_at DESC
	`

	rows, err := ps.db.Query(query)
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

