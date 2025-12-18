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
// @Summary     Health check
// @Description Returns the health status of the service, including database connectivity. Useful for monitoring and load balancer health checks.
// @Tags        Health
// @Accept      json
// @Produce     json
// @Success     200  {object}  map[string]string  "Service is healthy and database is connected"
// @Success     503  {object}  map[string]string  "Service is unhealthy or database is disconnected"
// @Router      /health [get]
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
// @Summary     Ingest collection report
// @Description Receives a collection report from a snail-core agent and stores it. The report replaces any existing data for the same hostname. Supports gzip-compressed requests via the Content-Encoding: gzip header.
// @Tags        Ingest
// @Accept      json
// @Accept      application/gzip
// @Produce     json
// @Param       request  body      models.IngestRequest  true  "Collection report from snail-core"
// @Success     201      {object}  models.IngestResponse  "Report successfully ingested"
// @Failure     400      {object}  map[string]string     "Invalid request payload"
// @Failure     500      {object}  map[string]string     "Internal server error"
// @Router      /api/v1/ingest [post]
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
// @Summary     List all hosts
// @Description Returns a list of all known hosts with summary information. Each host entry includes the hostname and last seen timestamp.
// @Tags        Hosts
// @Accept      json
// @Produce     json
// @Success     200  {object}  map[string]interface{}  "List of hosts with total count"
// @Failure     500  {object}  map[string]string       "Internal server error"
// @Router      /api/v1/hosts [get]
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
// @Summary     Get host data
// @Description Returns the complete collection report for a specific host, including all collected data and metadata.
// @Tags        Hosts
// @Accept      json
// @Produce     json
// @Param       hostname  path      string  true  "Hostname of the host to retrieve"
// @Success     200       {object}  models.Report  "Host data"
// @Failure     400       {object}  map[string]string  "Missing hostname parameter"
// @Failure     404       {object}  map[string]string  "Host not found"
// @Failure     500       {object}  map[string]string  "Internal server error"
// @Router      /api/v1/hosts/{hostname} [get]
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
// @Summary     Delete host
// @Description Removes a host and all its associated data from the system. This operation cannot be undone.
// @Tags        Hosts
// @Accept      json
// @Produce     json
// @Param       hostname  path      string  true  "Hostname of the host to delete"
// @Success     204       "Host successfully deleted"
// @Failure     400       {object}  map[string]string  "Missing hostname parameter"
// @Failure     404       {object}  map[string]string  "Host not found"
// @Failure     500       {object}  map[string]string  "Internal server error"
// @Router      /api/v1/hosts/{hostname} [delete]
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
// @Summary     OpenAPI specification (YAML)
// @Description Returns the OpenAPI 3.0 specification in YAML format (generated from code annotations)
// @Tags        Health
// @Produce     application/x-yaml
// @Success     200  {string}  string  "OpenAPI specification"
// @Router      /openapi.yaml [get]
func (h *Handlers) GetOpenAPISpecYAML(c *gin.Context) {
	// Try to read generated spec from docs/swagger.yaml first
	specPath := "docs/swagger.yaml"
	specData, err := os.ReadFile(specPath)
	if err != nil {
		// Fallback to openapi.yaml if generated spec doesn't exist
		specPath = "openapi.yaml"
		specData, err = os.ReadFile(specPath)
		if err != nil {
			log.Printf("Failed to read OpenAPI spec: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
			return
		}
	}

	c.Data(http.StatusOK, "application/x-yaml", specData)
}

// GetOpenAPISpecJSON returns the OpenAPI specification in JSON format
// @Summary     OpenAPI specification (JSON)
// @Description Returns the OpenAPI 3.0 specification in JSON format (generated from code annotations)
// @Tags        Health
// @Produce     application/json
// @Success     200  {object}  map[string]interface{}  "OpenAPI specification"
// @Router      /openapi.json [get]
func (h *Handlers) GetOpenAPISpecJSON(c *gin.Context) {
	// Try to read generated spec from docs/swagger.json first
	specPath := "docs/swagger.json"
	specData, err := os.ReadFile(specPath)
	if err != nil {
		// Fallback: try to read YAML and convert
		specPath = "docs/swagger.yaml"
		specData, err = os.ReadFile(specPath)
		if err != nil {
			// Final fallback to openapi.yaml
			specPath = "openapi.yaml"
			specData, err = os.ReadFile(specPath)
			if err != nil {
				log.Printf("Failed to read OpenAPI spec: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
				return
			}
			// Parse YAML and convert to JSON
			var spec map[string]interface{}
			if err := yaml.Unmarshal(specData, &spec); err != nil {
				log.Printf("Failed to parse OpenAPI spec: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse OpenAPI specification"})
				return
			}
			c.JSON(http.StatusOK, spec)
			return
		}
		// Parse YAML and convert to JSON
		var spec map[string]interface{}
		if err := yaml.Unmarshal(specData, &spec); err != nil {
			log.Printf("Failed to parse OpenAPI spec: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse OpenAPI specification"})
			return
		}
		c.JSON(http.StatusOK, spec)
		return
	}

	// Already JSON, just parse and return
	var spec map[string]interface{}
	if err := json.Unmarshal(specData, &spec); err != nil {
		log.Printf("Failed to parse OpenAPI spec JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse OpenAPI specification"})
		return
	}

	c.JSON(http.StatusOK, spec)
}


