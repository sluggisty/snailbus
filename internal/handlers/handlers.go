package handlers

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"snailbus/internal/middleware"
	"snailbus/internal/models"
	"snailbus/internal/storage"
)

// Handlers contains HTTP handlers
type Handlers struct {
	storage storage.Storage
}

// Auth handlers are in auth.go

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
	// Check database connection by trying a simple query
	// Note: Health check doesn't require org context, so we use a simple query
	_, err := h.storage.GetOrganizationByID("00000000-0000-0000-0000-000000000000")
	if err != nil && err != storage.ErrNotFound {
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
	if req.Meta.HostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing host_id in meta"})
		return
	}
	if req.Meta.Hostname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing hostname in meta"})
		return
	}

	// Get user_id and org_id from context (set by AuthMiddleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userObj := user.(*models.User)

	// Create report
	now := time.Now().UTC()
	report := &models.Report{
		ID:         req.Meta.HostID, // Use host_id (UUID) as primary identifier
		ReceivedAt: now,
		Meta:       req.Meta,
		Data:       req.Data,
		Errors:     req.Errors,
	}

	// Store the report (replaces any previous data for this host)
	// Associate the host with the authenticated user's organization and user ID
	if err := h.storage.SaveHost(report, userObj.OrgID, userID.(string)); err != nil {
		log.Printf("Failed to save host data for %s: %v", req.Meta.Hostname, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store host data"})
		return
	}

	log.Printf("Host data updated: host_id=%s, hostname=%s, collection_id=%s, errors=%d",
		req.Meta.HostID, req.Meta.Hostname, req.Meta.CollectionID, len(req.Errors))

	// Send response
	c.JSON(http.StatusCreated, models.IngestResponse{
		Status:     "ok",
		ReportID:   req.Meta.HostID, // Return host_id instead of hostname
		ReceivedAt: now.Format(time.RFC3339),
		Message:    "Host data updated successfully",
	})
}

// ListHosts returns a list of all known hosts in the current organization
// @Summary     List all hosts
// @Description Returns a list of all known hosts with summary information for the authenticated user's organization. Each host entry includes the hostname and last seen timestamp.
// @Tags        Hosts
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Success     200  {object}  map[string]interface{}  "List of hosts with total count"
// @Failure     401  {object}  map[string]string       "Unauthorized"
// @Failure     500  {object}  map[string]string       "Internal server error"
// @Router      /api/v1/hosts [get]
func (h *Handlers) ListHosts(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	hosts, err := h.storage.ListHosts(orgID)
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
// @Description Returns the complete collection report for a specific host in the authenticated user's organization, including all collected data and metadata, identified by its host ID.
// @Tags        Hosts
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Param       host_id  path      string  true  "Unique identifier (UUID) of the host to retrieve"
// @Success     200       {object}  models.Report  "Host data"
// @Failure     400       {object}  map[string]string  "Missing host_id parameter"
// @Failure     401       {object}  map[string]string  "Unauthorized"
// @Failure     404       {object}  map[string]string  "Host not found"
// @Failure     500       {object}  map[string]string  "Internal server error"
// @Router      /api/v1/hosts/{host_id} [get]
func (h *Handlers) GetHost(c *gin.Context) {
	hostID := c.Param("host_id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing host_id"})
		return
	}

	orgID := middleware.GetOrgID(c)
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	report, err := h.storage.GetHost(hostID, orgID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
			return
		}
		log.Printf("Failed to get host %s: %v", hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve host"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// DeleteHost removes a host
// @Summary     Delete host
// @Description Removes a host and all its associated data from the authenticated user's organization. This operation cannot be undone. Uses host_id (UUID) as the identifier.
// @Tags        Hosts
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Param       host_id  path      string  true  "Host ID (UUID) of the host to delete"
// @Success     204       "Host successfully deleted"
// @Failure     400       {object}  map[string]string  "Missing host_id parameter"
// @Failure     401       {object}  map[string]string  "Unauthorized"
// @Failure     404       {object}  map[string]string  "Host not found"
// @Failure     500       {object}  map[string]string  "Internal server error"
// @Router      /api/v1/hosts/{host_id} [delete]
func (h *Handlers) DeleteHost(c *gin.Context) {
	hostID := c.Param("host_id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing host_id"})
		return
	}

	orgID := middleware.GetOrgID(c)
	if orgID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.storage.DeleteHost(hostID, orgID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
			return
		}
		log.Printf("Failed to delete host %s: %v", hostID, err)
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
	// Try to find the spec file relative to the executable or current working directory
	specPath := h.findSpecFile("docs/swagger.yaml")
	if specPath == "" {
		specPath = h.findSpecFile("openapi.yaml")
	}
	
	if specPath == "" {
		log.Printf("Failed to find OpenAPI spec file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
		return
	}
	
	specData, err := os.ReadFile(specPath)
	if err != nil {
		log.Printf("Failed to read OpenAPI spec from %s: %v", specPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
		return
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
	// Try to find the spec file relative to the executable or current working directory
	specPath := h.findSpecFile("docs/swagger.json")
	if specPath == "" {
		// Fallback: try to read YAML and convert
		specPath = h.findSpecFile("docs/swagger.yaml")
		if specPath == "" {
			specPath = h.findSpecFile("openapi.yaml")
		}
		
		if specPath == "" {
			log.Printf("Failed to find OpenAPI spec file")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
			return
		}
		
		// Read and parse YAML, then convert to JSON
		specData, err := os.ReadFile(specPath)
		if err != nil {
			log.Printf("Failed to read OpenAPI spec from %s: %v", specPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
			return
		}
		
		var spec map[string]interface{}
		if err := yaml.Unmarshal(specData, &spec); err != nil {
			log.Printf("Failed to parse OpenAPI spec: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse OpenAPI specification"})
			return
		}
		c.JSON(http.StatusOK, spec)
		return
	}
	
	// Read JSON file
	specData, err := os.ReadFile(specPath)
	if err != nil {
		log.Printf("Failed to read OpenAPI spec from %s: %v", specPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OpenAPI specification"})
		return
	}

	// Parse and return JSON
	var spec map[string]interface{}
	if err := json.Unmarshal(specData, &spec); err != nil {
		log.Printf("Failed to parse OpenAPI spec JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse OpenAPI specification"})
		return
	}

	c.JSON(http.StatusOK, spec)
}

// findSpecFile tries to locate a spec file in multiple possible locations
func (h *Handlers) findSpecFile(relativePath string) string {
	// Try current working directory first
	if _, err := os.Stat(relativePath); err == nil {
		return relativePath
	}
	
	// Try relative to executable location
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		absPath := filepath.Join(execDir, relativePath)
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}
	
	// Try relative to current working directory with absolute path
	wd, err := os.Getwd()
	if err == nil {
		absPath := filepath.Join(wd, relativePath)
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}
	
	return ""
}


