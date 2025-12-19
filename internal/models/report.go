package models

import (
	"encoding/json"
	"time"
)

// HealthResponse represents the health check response
// @Description Health check response with service and database status
type HealthResponse struct {
	Status   string `json:"status" example:"ok"`       // Overall service status (ok or error)
	Service  string `json:"service" example:"snailbus"` // Service name
	Database string `json:"database" example:"connected"` // Database connection status (connected or disconnected)
}

// Report represents a collection report from snail-core
// @Description Complete collection report with metadata and collected data
type Report struct {
	ID         string          `json:"id"` // host_id (UUID)
	ReceivedAt time.Time       `json:"received_at"`
	Meta       ReportMeta      `json:"meta"`
	Data       json.RawMessage `json:"data"`
	Errors     []string        `json:"errors,omitempty"`
}

// ReportMeta contains metadata about the collection
// @Description Metadata about the collection including hostname, host_id, collection ID, timestamp, and snail-core version
type ReportMeta struct {
	Hostname     string `json:"hostname"`
	HostID       string `json:"host_id"` // Persistent UUID identifying the host
	CollectionID string `json:"collection_id"`
	Timestamp    string `json:"timestamp"`
	SnailVersion string `json:"snail_version"`
}

// IngestRequest is the incoming request format from snail-core
// @Description Request payload from snail-core containing metadata, collected data, and any errors
type IngestRequest struct {
	Meta   ReportMeta      `json:"meta"`
	Data   json.RawMessage `json:"data"`
	Errors []string        `json:"errors,omitempty"`
}

// IngestResponse is returned after successful ingestion
// @Description Response after successfully ingesting a collection report
type IngestResponse struct {
	Status     string `json:"status"`
	ReportID   string `json:"report_id"`
	ReceivedAt string `json:"received_at"`
	Message    string `json:"message,omitempty"`
}

// HostSummary represents summary info about a host
// @Description Summary information about a host including host_id, hostname and last seen timestamp
type HostSummary struct {
	HostID   string    `json:"host_id"`   // Persistent UUID
	Hostname string    `json:"hostname"`  // Current hostname (may change)
	LastSeen time.Time `json:"last_seen"`
}


