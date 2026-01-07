package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"snailbus/internal/config"
)

func TestRequestSizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create test config with size limits
	cfg := &config.Config{
		MaxRequestSizeIngest: 10 * 1024 * 1024, // 10MB
		MaxRequestSizePost:   1 * 1024 * 1024,  // 1MB
		MaxRequestSizeGet:    100 * 1024,       // 100KB
	}

	// Create router with size limit middleware
	r := gin.New()
	r.Use(RequestSizeLimit(cfg))

	// Test GET request within limit
	r.GET("/test-get", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test POST request within limit
	r.POST("/test-post", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test ingest endpoint within limit
	r.POST("/api/v1/ingest", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test GET request - should pass
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test-get", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test POST request with small body - should pass
	smallBody := bytes.Repeat([]byte("a"), 1024) // 1KB
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/test-post", bytes.NewReader(smallBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(smallBody))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test POST request exceeding limit
	largeBody := bytes.Repeat([]byte("a"), 2*1024*1024) // 2MB (exceeds 1MB limit)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/test-post", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(largeBody))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	// Test GET request exceeding limit
	largeGetBody := bytes.Repeat([]byte("a"), 200*1024) // 200KB (exceeds 100KB limit)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test-get", bytes.NewReader(largeGetBody))
	req.ContentLength = int64(len(largeGetBody))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	// Test ingest endpoint with large body - should pass (10MB limit)
	ingestBody := bytes.Repeat([]byte("a"), 5*1024*1024) // 5MB (within 10MB limit)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(ingestBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(ingestBody))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test ingest endpoint exceeding limit
	hugeBody := bytes.Repeat([]byte("a"), 15*1024*1024) // 15MB (exceeds 10MB limit)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(hugeBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(hugeBody))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}
