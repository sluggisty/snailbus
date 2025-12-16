package models

import (
	"encoding/json"
	"time"
)

// Report represents a collection report from snail-core
type Report struct {
	ID         string          `json:"id"`
	ReceivedAt time.Time       `json:"received_at"`
	Meta       ReportMeta      `json:"meta"`
	Data       json.RawMessage `json:"data"`
	Errors     []string        `json:"errors,omitempty"`
}

// ReportMeta contains metadata about the collection
type ReportMeta struct {
	Hostname     string `json:"hostname"`
	CollectionID string `json:"collection_id"`
	Timestamp    string `json:"timestamp"`
	SnailVersion string `json:"snail_version"`
}

// IngestRequest is the incoming request format from snail-core
type IngestRequest struct {
	Meta   ReportMeta      `json:"meta"`
	Data   json.RawMessage `json:"data"`
	Errors []string        `json:"errors,omitempty"`
}

// IngestResponse is returned after successful ingestion
type IngestResponse struct {
	Status     string `json:"status"`
	ReportID   string `json:"report_id"`
	ReceivedAt string `json:"received_at"`
	Message    string `json:"message,omitempty"`
}

// HostSummary represents summary info about a host
type HostSummary struct {
	Hostname string    `json:"hostname"`
	LastSeen time.Time `json:"last_seen"`
}

