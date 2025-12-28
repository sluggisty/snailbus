package integration

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"snailbus/internal/handlers"
	"snailbus/internal/middleware"
	"snailbus/internal/models"
	"snailbus/internal/storage"
	"snailbus/internal/testutils"
)

// setupTestRouter creates a Gin router with all routes and middleware configured
// This mirrors the setup in main.go
func setupTestRouter(store storage.Storage) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := handlers.New(store)

	// Health check endpoint
	r.GET("/health", h.Health)

	// Root endpoint
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to Snailbus API",
			"version": "1.0.0",
		})
	})

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Public auth endpoints (no authentication required)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
			auth.POST("/api-key", h.GetAPIKeyFromCredentials)
		}

		// Protected routes (require API key authentication)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(store))
		protected.Use(middleware.OrgContextMiddleware())
		{
			// Auth endpoints
			protected.GET("/auth/me", h.GetMe)

			// API key management
			protected.POST("/api-keys", h.CreateAPIKey)
			protected.GET("/api-keys", h.ListAPIKeys)
			protected.DELETE("/api-keys/:id", h.DeleteAPIKey)

			// Host management endpoints - viewing accessible to all authenticated users
			protected.GET("/hosts", h.ListHosts)
			protected.GET("/hosts/:host_id", h.GetHost)

			// Host deletion - requires editor or admin role
			editorOrAdmin := protected.Group("")
			editorOrAdmin.Use(middleware.RequireRole("editor", "admin"))
			{
				editorOrAdmin.DELETE("/hosts/:host_id", h.DeleteHost)
			}

			// User management endpoints - admin only
			adminOnly := protected.Group("")
			adminOnly.Use(middleware.RequireRole("admin"))
			{
				adminOnly.GET("/users", h.ListUsers)
				adminOnly.POST("/users", h.CreateUser)
				adminOnly.PUT("/users/:user_id/role", h.UpdateUserRole)
				adminOnly.DELETE("/users/:user_id", h.DeleteUser)
			}
		}

		// Ingest endpoint - requires editor or admin role
		ingest := v1.Group("")
		ingest.Use(middleware.AuthMiddleware(store))
		ingest.Use(middleware.OrgContextMiddleware())
		ingest.Use(middleware.RequireRole("editor", "admin"))
		{
			ingest.POST("/ingest", h.Ingest)
		}
	}

	return r
}

// testClient is a helper for making authenticated HTTP requests
type testClient struct {
	router   *gin.Engine
	apiKey   string
	baseURL  string
}

// newTestClient creates a new test client with an API key
func newTestClient(router *gin.Engine, apiKey string) *testClient {
	return &testClient{
		router:  router,
		apiKey:   apiKey,
		baseURL:  "",
	}
}

// doRequest makes an HTTP request with authentication
func (tc *testClient) doRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			panic(fmt.Sprintf("Failed to marshal body: %v", err))
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if tc.apiKey != "" {
		req.Header.Set("X-API-Key", tc.apiKey)
	}

	w := httptest.NewRecorder()
	tc.router.ServeHTTP(w, req)
	return w
}

// TestIntegration_UserRegistrationAndLogin tests the complete user registration and login flow
func TestIntegration_UserRegistrationAndLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	router := setupTestRouter(store)

	// Test registration
	registerReq := models.RegisterRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		OrgName:  "Test Organization",
	}

	reqBody, _ := json.Marshal(registerReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var user models.User
	err := json.Unmarshal(w.Body.Bytes(), &user)
	require.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "admin", user.Role)

	// Test login
	loginReq := models.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}

	reqBody, _ = json.Marshal(loginReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Login failed with status %d, body: %s", w.Code, w.Body.String())

	var loginResp models.LoginResponse
	err = json.Unmarshal(w.Body.Bytes(), &loginResp)
	require.NoError(t, err, "Failed to unmarshal login response: %s", w.Body.String())
	require.NotEmpty(t, loginResp.Token, "Login token should not be empty")
	require.NotNil(t, loginResp.User, "Login response should include user")
	assert.Equal(t, user.ID, loginResp.User.ID)

	// Test using the API key from login
	client := newTestClient(router, loginResp.Token)
	w = client.doRequest(http.MethodGet, "/api/v1/auth/me", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var meUser models.User
	err = json.Unmarshal(w.Body.Bytes(), &meUser)
	require.NoError(t, err)
	assert.Equal(t, user.ID, meUser.ID)
}

// TestIntegration_APIKeyCreationAndUsage tests API key creation and usage
func TestIntegration_APIKeyCreationAndUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	router := setupTestRouter(store)

	// Register and login to get initial API key
	_, _, initialAPIKey, err := testutils.CreateTestOrganizationWithUser(store, "Test Org", "testuser", "test@example.com", "password123", "admin")
	require.NoError(t, err)

	client := newTestClient(router, initialAPIKey)

	// Create a new API key
	createKeyReq := models.CreateAPIKeyRequest{
		Name: "Test API Key",
	}

	w := client.doRequest(http.MethodPost, "/api/v1/api-keys", createKeyReq)
	assert.Equal(t, http.StatusCreated, w.Code)

	var keyResp models.CreateAPIKeyResponse
	err = json.Unmarshal(w.Body.Bytes(), &keyResp)
	require.NoError(t, err)
	assert.Equal(t, "Test API Key", keyResp.Name)
	assert.NotEmpty(t, keyResp.Key)

	// Use the new API key
	newClient := newTestClient(router, keyResp.Key)
	w = newClient.doRequest(http.MethodGet, "/api/v1/auth/me", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// List API keys
	w = client.doRequest(http.MethodGet, "/api/v1/api-keys", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var keysResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &keysResp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, keysResp["total"].(float64), float64(2)) // At least 2 keys (login + new one)

	// Delete the API key
	w = client.doRequest(http.MethodDelete, "/api/v1/api-keys/"+keyResp.ID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify it's deleted (try to use it)
	w = newClient.doRequest(http.MethodGet, "/api/v1/auth/me", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestIntegration_HostIngestionFlow tests the complete host data ingestion flow
func TestIntegration_HostIngestionFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	router := setupTestRouter(store)

	// Create test user with editor role
	_, _, apiKey, err := testutils.CreateTestOrganizationWithUser(store, "Test Org", "editor", "editor@example.com", "password123", "editor")
	require.NoError(t, err)

	client := newTestClient(router, apiKey)

	// Ingest host data
	ingestReq := models.IngestRequest{
		Meta: models.ReportMeta{
			HostID:       "00000000-0000-0000-0000-000000000001",
			Hostname:     "test-host",
			CollectionID: "collection-1",
			Timestamp:    time.Now().Format(time.RFC3339),
			SnailVersion: "0.2.0",
		},
		Data:   json.RawMessage(`{"system": {"os_name": "Fedora", "os_version": "42"}}`),
		Errors: []string{},
	}

	w := client.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq)
	assert.Equal(t, http.StatusCreated, w.Code)

	var ingestResp models.IngestResponse
	err = json.Unmarshal(w.Body.Bytes(), &ingestResp)
	require.NoError(t, err)
	assert.Equal(t, "ok", ingestResp.Status)
	assert.Equal(t, ingestReq.Meta.HostID, ingestResp.ReportID)

	// List hosts
	w = client.doRequest(http.MethodGet, "/api/v1/hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var hostsResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &hostsResp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), hostsResp["total"].(float64))

	// Get specific host
	w = client.doRequest(http.MethodGet, "/api/v1/hosts/"+ingestReq.Meta.HostID, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var report models.Report
	err = json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)
	assert.Equal(t, ingestReq.Meta.Hostname, report.Meta.Hostname)

	// Update host data
	ingestReq.Meta.Hostname = "test-host-updated"
	ingestReq.Meta.SnailVersion = "0.3.0"

	w = client.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify update
	w = client.doRequest(http.MethodGet, "/api/v1/hosts/"+ingestReq.Meta.HostID, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &report)
	require.NoError(t, err)
	assert.Equal(t, "test-host-updated", report.Meta.Hostname)
}

// TestIntegration_HostQueryingAndDeletion tests host querying and deletion
func TestIntegration_HostQueryingAndDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	router := setupTestRouter(store)

	// Create test user with admin role
	_, _, apiKey, err := testutils.CreateTestOrganizationWithUser(store, "Test Org", "admin", "admin@example.com", "password123", "admin")
	require.NoError(t, err)

	client := newTestClient(router, apiKey)

	// Create multiple hosts
	hostIDs := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000003",
	}

	for i, hostID := range hostIDs {
		ingestReq := models.IngestRequest{
			Meta: models.ReportMeta{
				HostID:       hostID,
				Hostname:     fmt.Sprintf("host-%d", i+1),
				CollectionID: fmt.Sprintf("collection-%d", i+1),
				Timestamp:    time.Now().Format(time.RFC3339),
				SnailVersion: "0.2.0",
			},
			Data:   json.RawMessage(`{"system": {"os_name": "Fedora"}}`),
			Errors: []string{},
		}

		w := client.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq)
		assert.Equal(t, http.StatusCreated, w.Code)
	}

	// List all hosts
	w := client.doRequest(http.MethodGet, "/api/v1/hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var hostsResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &hostsResp)
	require.NoError(t, err)
	assert.Equal(t, float64(3), hostsResp["total"].(float64))

	// Get specific host
	w = client.doRequest(http.MethodGet, "/api/v1/hosts/"+hostIDs[0], nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete a host
	w = client.doRequest(http.MethodDelete, "/api/v1/hosts/"+hostIDs[0], nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify it's deleted
	w = client.doRequest(http.MethodGet, "/api/v1/hosts/"+hostIDs[0], nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Verify other hosts still exist
	w = client.doRequest(http.MethodGet, "/api/v1/hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &hostsResp)
	require.NoError(t, err)
	assert.Equal(t, float64(2), hostsResp["total"].(float64))
}

// TestIntegration_OrganizationIsolation tests that users from different organizations cannot access each other's data
func TestIntegration_OrganizationIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	router := setupTestRouter(store)

	// Create two organizations with users
	_, _, apiKey1, err := testutils.CreateTestOrganizationWithUser(store, "Org 1", "user1", "user1@example.com", "password123", "admin")
	require.NoError(t, err)

	_, _, apiKey2, err := testutils.CreateTestOrganizationWithUser(store, "Org 2", "user2", "user2@example.com", "password123", "admin")
	require.NoError(t, err)

	client1 := newTestClient(router, apiKey1)
	client2 := newTestClient(router, apiKey2)

	// User1 creates a host
	ingestReq1 := models.IngestRequest{
		Meta: models.ReportMeta{
			HostID:       "00000000-0000-0000-0000-000000000001",
			Hostname:     "org1-host",
			CollectionID: "collection-1",
			Timestamp:    time.Now().Format(time.RFC3339),
			SnailVersion: "0.2.0",
		},
		Data:   json.RawMessage(`{"system": {"os_name": "Fedora"}}`),
		Errors: []string{},
	}

	w := client1.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq1)
	assert.Equal(t, http.StatusCreated, w.Code)

	// User2 creates a host
	ingestReq2 := models.IngestRequest{
		Meta: models.ReportMeta{
			HostID:       "00000000-0000-0000-0000-000000000002",
			Hostname:     "org2-host",
			CollectionID: "collection-2",
			Timestamp:    time.Now().Format(time.RFC3339),
			SnailVersion: "0.2.0",
		},
		Data:   json.RawMessage(`{"system": {"os_name": "Ubuntu"}}`),
		Errors: []string{},
	}

	w = client2.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq2)
	assert.Equal(t, http.StatusCreated, w.Code)

	// User1 can only see their own host
	w = client1.doRequest(http.MethodGet, "/api/v1/hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var hostsResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &hostsResp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), hostsResp["total"].(float64))

	// User2 can only see their own host
	w = client2.doRequest(http.MethodGet, "/api/v1/hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &hostsResp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), hostsResp["total"].(float64))

	// User1 cannot access User2's host
	w = client1.doRequest(http.MethodGet, "/api/v1/hosts/"+ingestReq2.Meta.HostID, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// User2 cannot access User1's host
	w = client2.doRequest(http.MethodGet, "/api/v1/hosts/"+ingestReq1.Meta.HostID, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// User1 cannot delete User2's host
	w = client1.doRequest(http.MethodDelete, "/api/v1/hosts/"+ingestReq2.Meta.HostID, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestIntegration_RoleBasedAccessControl tests role-based access control
func TestIntegration_RoleBasedAccessControl(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	router := setupTestRouter(store)

	// Create organization with users of different roles
	org, _, adminKey, err := testutils.CreateTestOrganizationWithUser(store, "Test Org", "admin", "admin@example.com", "password123", "admin")
	require.NoError(t, err)

	editorUser, err := testutils.CreateTestUser(store, "editor", "editor@example.com", "password123", org.ID, "editor")
	require.NoError(t, err)
	editorKey, _, err := testutils.CreateTestAPIKey(store, editorUser.ID, "Editor Key")
	require.NoError(t, err)

	viewerUser, err := testutils.CreateTestUser(store, "viewer", "viewer@example.com", "password123", org.ID, "viewer")
	require.NoError(t, err)
	viewerKey, _, err := testutils.CreateTestAPIKey(store, viewerUser.ID, "Viewer Key")
	require.NoError(t, err)

	adminClient := newTestClient(router, adminKey)
	editorClient := newTestClient(router, editorKey)
	viewerClient := newTestClient(router, viewerKey)

	// All roles can view hosts (once created)
	// Editor and Admin can ingest
	ingestReq := models.IngestRequest{
		Meta: models.ReportMeta{
			HostID:       "00000000-0000-0000-0000-000000000001",
			Hostname:     "test-host",
			CollectionID: "collection-1",
			Timestamp:    time.Now().Format(time.RFC3339),
			SnailVersion: "0.2.0",
		},
		Data:   json.RawMessage(`{"system": {"os_name": "Fedora"}}`),
		Errors: []string{},
	}

	// Editor can ingest
	w := editorClient.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Admin can ingest
	w = adminClient.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Viewer cannot ingest
	w = viewerClient.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// All can view hosts
	w = viewerClient.doRequest(http.MethodGet, "/api/v1/hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	w = editorClient.doRequest(http.MethodGet, "/api/v1/hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	w = adminClient.doRequest(http.MethodGet, "/api/v1/hosts", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Editor and Admin can delete hosts
	w = editorClient.doRequest(http.MethodDelete, "/api/v1/hosts/"+ingestReq.Meta.HostID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Recreate host for next test
	w = editorClient.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq)
	assert.Equal(t, http.StatusCreated, w.Code)

	w = adminClient.doRequest(http.MethodDelete, "/api/v1/hosts/"+ingestReq.Meta.HostID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Recreate host for viewer test
	w = editorClient.doRequest(http.MethodPost, "/api/v1/ingest", ingestReq)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Viewer cannot delete hosts
	w = viewerClient.doRequest(http.MethodDelete, "/api/v1/hosts/"+ingestReq.Meta.HostID, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Only Admin can manage users
	w = adminClient.doRequest(http.MethodGet, "/api/v1/users", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	w = editorClient.doRequest(http.MethodGet, "/api/v1/users", nil)
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = viewerClient.doRequest(http.MethodGet, "/api/v1/users", nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestIntegration_IngestWithGzip tests gzip-compressed ingestion
func TestIntegration_IngestWithGzip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	store, cleanup := setupTestStorage(t)
	defer cleanup()

	router := setupTestRouter(store)

	_, _, apiKey, err := testutils.CreateTestOrganizationWithUser(store, "Test Org", "editor", "editor@example.com", "password123", "editor")
	require.NoError(t, err)

	ingestReq := models.IngestRequest{
		Meta: models.ReportMeta{
			HostID:       "00000000-0000-0000-0000-000000000001",
			Hostname:     "test-host",
			CollectionID: "collection-1",
			Timestamp:    time.Now().Format(time.RFC3339),
			SnailVersion: "0.2.0",
		},
		Data:   json.RawMessage(`{"system": {"os_name": "Fedora", "os_version": "42"}}`),
		Errors: []string{},
	}

	// Compress the request body
	jsonData, _ := json.Marshal(ingestReq)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(jsonData)
	gz.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("X-API-Key", apiKey)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var ingestResp models.IngestResponse
	err = json.Unmarshal(w.Body.Bytes(), &ingestResp)
	require.NoError(t, err)
	assert.Equal(t, "ok", ingestResp.Status)
}

// setupTestStorage creates a test storage instance with database setup
func setupTestStorage(t *testing.T) (storage.Storage, func()) {
	// Set up test database
	_, cleanup, err := testutils.SetupTestDB(t)
	require.NoError(t, err)

	// Create storage instance
	store, err := storage.NewPostgresStorage(testutils.GetTestDatabaseURL())
	require.NoError(t, err)

	return store, func() {
		store.Close()
		cleanup()
	}
}

