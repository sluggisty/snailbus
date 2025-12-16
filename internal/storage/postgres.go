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
		INSERT INTO hosts (hostname, received_at, collection_id, timestamp, snail_version, data, errors)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (hostname) DO UPDATE SET
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

// GetHost returns the full report data for a specific host
func (ps *PostgresStorage) GetHost(hostname string) (*models.Report, error) {
	query := `
		SELECT hostname, received_at, collection_id, timestamp, snail_version, data, errors
		FROM hosts
		WHERE hostname = $1
	`

	report := &models.Report{}
	var errors []string

	err := ps.db.QueryRow(query, hostname).Scan(
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

	report.ID = hostname // Use hostname as ID
	report.Errors = errors
	return report, nil
}

// DeleteHost removes a host
func (ps *PostgresStorage) DeleteHost(hostname string) error {
	result, err := ps.db.Exec("DELETE FROM hosts WHERE hostname = $1", hostname)
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
		SELECT hostname, received_at
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
		var hostname string
		var receivedAt time.Time

		if err := rows.Scan(&hostname, &receivedAt); err != nil {
			return nil, fmt.Errorf("failed to scan host: %w", err)
		}

		host := &models.HostSummary{
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
		SELECT hostname, received_at, collection_id, timestamp, snail_version, data, errors
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

		report.ID = report.Meta.Hostname
		report.Errors = errors
		reports = append(reports, report)
	}

	return reports, nil
}

// Close closes the database connection
func (ps *PostgresStorage) Close() error {
	return ps.db.Close()
}

