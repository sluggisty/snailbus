package testutils

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
	"snailbus/internal/handlers"
	"snailbus/internal/middleware"
	"snailbus/internal/models"
	"snailbus/internal/storage"
)

// TestClient is a helper for making authenticated HTTP requests in tests
type TestClient struct {
	router  *gin.Engine
	apiKey  string
	baseURL string
}

// NewTestClient creates a new test client with an API key for authentication
func NewTestClient(router *gin.Engine, apiKey string) *TestClient {
	return &TestClient{
		router:  router,
		apiKey:   apiKey,
		baseURL:  "",
	}
}

// DoRequest makes an HTTP request with optional authentication
func (tc *TestClient) DoRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	return tc.DoRequestWithHeaders(method, path, body, nil)
}

// DoRequestWithHeaders makes an HTTP request with optional authentication and custom headers
func (tc *TestClient) DoRequestWithHeaders(method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
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

	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	w := httptest.NewRecorder()
	tc.router.ServeHTTP(w, req)
	return w
}

// DoGzipRequest makes a gzip-compressed HTTP request
func (tc *TestClient) DoGzipRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	jsonData, err := json.Marshal(body)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal body: %v", err))
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(jsonData)
	gz.Close()

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	if tc.apiKey != "" {
		req.Header.Set("X-API-Key", tc.apiKey)
	}

	w := httptest.NewRecorder()
	tc.router.ServeHTTP(w, req)
	return w
}

// SetupTestRouter creates a minimal Gin router for testing
// This is a basic router without middleware - useful for unit tests
func SetupTestRouter(h *handlers.Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

// SetupFullTestRouter creates a complete Gin router with all routes and middleware
// This mirrors the setup in main.go and is useful for integration tests
func SetupFullTestRouter(store storage.Storage) *gin.Engine {
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

// CreateAuthenticatedTestClient creates a test client with a user and API key
// This is a convenience function that creates a user, organization, and API key,
// then returns a test client ready to make authenticated requests
func CreateAuthenticatedTestClient(store storage.Storage, router *gin.Engine, orgName, username, email, password, role string) (*TestClient, *models.Organization, *models.User, string, error) {
	org, user, apiKey, err := CreateTestOrganizationWithUser(store, orgName, username, email, password, role)
	if err != nil {
		return nil, nil, nil, "", err
	}

	client := NewTestClient(router, apiKey)
	return client, org, user, apiKey, nil
}

// AssertJSONResponse asserts that a response has the expected JSON structure
func AssertJSONResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, expectedBody interface{}) {
	t.Helper()
	assert.Equal(t, expectedStatus, w.Code, "Unexpected status code. Body: %s", w.Body.String())

	if expectedBody != nil {
		var expectedJSON map[string]interface{}
		expectedBytes, err := json.Marshal(expectedBody)
		if err != nil {
			t.Fatalf("Failed to marshal expected body: %v", err)
		}
		if err := json.Unmarshal(expectedBytes, &expectedJSON); err != nil {
			t.Fatalf("Failed to unmarshal expected body: %v", err)
		}

		var actualJSON map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &actualJSON); err != nil {
			t.Fatalf("Failed to unmarshal response body: %v. Body: %s", err, w.Body.String())
		}

		for key, expectedValue := range expectedJSON {
			actualValue, exists := actualJSON[key]
			assert.True(t, exists, "Missing key in response: %s", key)
			if exists {
				assert.Equal(t, expectedValue, actualValue, "Mismatch for key: %s", key)
			}
		}
	}
}

// AssertHTTPStatus asserts that a response has the expected HTTP status code
func AssertHTTPStatus(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()
	assert.Equal(t, expectedStatus, w.Code, "Unexpected status code. Body: %s", w.Body.String())
}

// AssertTimeEqual asserts that two time values are equal within a tolerance
func AssertTimeEqual(t *testing.T, expected, actual time.Time, tolerance time.Duration) {
	t.Helper()
	diff := expected.Sub(actual)
	if diff < 0 {
		diff = -diff
	}
	assert.LessOrEqual(t, diff, tolerance, "Time difference %v exceeds tolerance %v", diff, tolerance)
}

// AssertTimeWithin asserts that a time is within a certain duration of another time
func AssertTimeWithin(t *testing.T, actual time.Time, reference time.Time, tolerance time.Duration) {
	t.Helper()
	AssertTimeEqual(t, reference, actual, tolerance)
}

// UnmarshalJSONResponse unmarshals a JSON response into the provided type
func UnmarshalJSONResponse(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("Failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}
}

// ReadBody reads the response body as a string
func ReadBody(w *httptest.ResponseRecorder) string {
	return w.Body.String()
}

// ReadBodyAsJSON reads the response body and unmarshals it as JSON
func ReadBodyAsJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response body: %v. Body: %s", err, w.Body.String())
	}
	return result
}

