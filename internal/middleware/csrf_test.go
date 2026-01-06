package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGenerateCSRFToken(t *testing.T) {
	// Test token generation
	token1 := generateCSRFToken()
	token2 := generateCSRFToken()

	// Tokens should be different
	assert.NotEqual(t, token1, token2)

	// Tokens should be base64 encoded (no special characters)
	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
}

func TestValidateCSRFToken(t *testing.T) {
	authKey := []byte("test-key-32-bytes-long-for-testing")

	// Valid tokens should match
	assert.True(t, validateCSRFToken("token123", "token123", authKey))

	// Different tokens should not match
	assert.False(t, validateCSRFToken("token123", "token456", authKey))

	// Empty tokens should not match
	assert.False(t, validateCSRFToken("", "token123", authKey))
	assert.False(t, validateCSRFToken("token123", "", authKey))
}

func TestIsStateChangingMethod(t *testing.T) {
	// State-changing methods
	assert.True(t, IsStateChangingMethod("POST"))
	assert.True(t, IsStateChangingMethod("PUT"))
	assert.True(t, IsStateChangingMethod("PATCH"))
	assert.True(t, IsStateChangingMethod("DELETE"))

	// Case insensitive
	assert.True(t, IsStateChangingMethod("post"))
	assert.True(t, IsStateChangingMethod("Post"))

	// Non-state-changing methods
	assert.False(t, IsStateChangingMethod("GET"))
	assert.False(t, IsStateChangingMethod("HEAD"))
	assert.False(t, IsStateChangingMethod("OPTIONS"))
}

func TestIsAuthEndpoint(t *testing.T) {
	// Auth endpoints should be exempt from CSRF
	assert.True(t, isAuthEndpoint("/api/v1/auth/login"))
	assert.True(t, isAuthEndpoint("/api/v1/auth/register"))
	assert.True(t, isAuthEndpoint("/api/v1/auth/api-key"))

	// Other endpoints should not be exempt
	assert.False(t, isAuthEndpoint("/api/v1/auth/me"))
	assert.False(t, isAuthEndpoint("/api/v1/users"))
	assert.False(t, isAuthEndpoint("/api/v1/hosts"))
	assert.False(t, isAuthEndpoint("/health"))
	assert.False(t, isAuthEndpoint("/"))
}

func TestCSRFTokensMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test router with CSRF middleware
	r := gin.New()
	r.Use(CSRFTokens())

	// Test GET request (should pass without CSRF token)
	r.GET("/test-get", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test POST request without CSRF token (should fail)
	r.POST("/test-post", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test auth endpoints (should pass without CSRF token)
	r.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "login success"})
	})
	r.POST("/api/v1/auth/register", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "register success"})
	})

	// Test GET request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test-get", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test POST request without CSRF token (should fail)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/test-post", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Test auth endpoints (should pass without CSRF token)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/register", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCSRFTokenMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRFTokenMiddleware())

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check if CSRF token cookie is set
	cookies := w.Result().Cookies()
	found := false
	for _, cookie := range cookies {
		if cookie.Name == "csrf_token" {
			found = true
			assert.NotEmpty(t, cookie.Value)
			break
		}
	}
	assert.True(t, found, "CSRF token cookie should be set")
}
