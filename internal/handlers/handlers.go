package handlers

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"snailbus/internal/models"
	"snailbus/internal/storage"
)

// Handlers contains HTTP handlers
type Handlers struct {
	storage storage.Storage
}

// New creates a new Handlers instance
func New(store storage.Storage) *Handlers {
	return &Handlers{storage: store}
}

// Health returns server health status
func (h *Handlers) Health(c *gin.Context) {
	// Check database connection by trying to list hosts (lightweight operation)
	_, err := h.storage.ListHosts()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "error",
			"service":  "snailbus",
			"database": "disconnected",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"service":  "snailbus",
		"database": "connected",
	})
}

// Ingest handles incoming reports from snail-core
func (h *Handlers) Ingest(c *gin.Context) {
	// Handle gzip-compressed requests
	var reader io.Reader = c.Request.Body
	if c.GetHeader("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			log.Printf("Failed to create gzip reader: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decompress request"})
			return
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Parse the request
	var req models.IngestRequest
	if err := json.NewDecoder(reader).Decode(&req); err != nil {
		log.Printf("Failed to parse ingest request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	// Validate required fields
	if req.Meta.Hostname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing hostname in meta"})
		return
	}

	// Create report
	now := time.Now().UTC()
	report := &models.Report{
		ID:         req.Meta.Hostname, // Use hostname as ID
		ReceivedAt: now,
		Meta:       req.Meta,
		Data:       req.Data,
		Errors:     req.Errors,
	}

	// Store the report (replaces any previous data for this host)
	if err := h.storage.SaveHost(report); err != nil {
		log.Printf("Failed to save host data for %s: %v", req.Meta.Hostname, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store host data"})
		return
	}

	log.Printf("Host data updated: hostname=%s, collection_id=%s, errors=%d",
		req.Meta.Hostname, req.Meta.CollectionID, len(req.Errors))

	// Send response
	c.JSON(http.StatusCreated, models.IngestResponse{
		Status:     "ok",
		ReportID:   req.Meta.Hostname,
		ReceivedAt: now.Format(time.RFC3339),
		Message:    "Host data updated successfully",
	})
}

// ListHosts returns a list of all known hosts
func (h *Handlers) ListHosts(c *gin.Context) {
	hosts, err := h.storage.ListHosts()
	if err != nil {
		log.Printf("Failed to list hosts: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve hosts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hosts": hosts,
		"total": len(hosts),
	})
}

// GetHost returns the full data for a specific host
func (h *Handlers) GetHost(c *gin.Context) {
	hostname := c.Param("hostname")
	if hostname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing hostname"})
		return
	}

	report, err := h.storage.GetHost(hostname)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
			return
		}
		log.Printf("Failed to get host %s: %v", hostname, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve host"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// DeleteHost removes a host
func (h *Handlers) DeleteHost(c *gin.Context) {
	hostname := c.Param("hostname")
	if hostname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing hostname"})
		return
	}

	if err := h.storage.DeleteHost(hostname); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
			return
		}
		log.Printf("Failed to delete host %s: %v", hostname, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete host"})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetOpenAPISpecYAML returns the OpenAPI specification in YAML format
func (h *Handlers) GetOpenAPISpecYAML(c *gin.Context) {
	// Read the embedded or file-based OpenAPI spec
	// For now, we'll read from the file system
	// In production, you might want to embed this at build time
	specPath := "openapi.yaml"
	specData, err := os.ReadFile(specPath)
	if err != nil {
		log.Printf("Failed to read OpenAPI spec: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
		return
	}

	c.Data(http.StatusOK, "application/x-yaml", specData)
}

// GetOpenAPISpecJSON returns the OpenAPI specification in JSON format
func (h *Handlers) GetOpenAPISpecJSON(c *gin.Context) {
	// Read YAML and convert to JSON
	specPath := "openapi.yaml"
	specData, err := os.ReadFile(specPath)
	if err != nil {
		log.Printf("Failed to read OpenAPI spec: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
		return
	}

	// Parse YAML
	var spec map[string]interface{}
	if err := yaml.Unmarshal(specData, &spec); err != nil {
		log.Printf("Failed to parse OpenAPI spec: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse OpenAPI specification"})
		return
	}

	c.JSON(http.StatusOK, spec)
}


