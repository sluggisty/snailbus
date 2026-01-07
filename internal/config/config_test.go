package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Clear any existing environment variables for clean test
	testEnvVars := []string{
		"DATABASE_URL", "PORT", "METRICS_PORT", "METRICS_BIND_ADDRESS",
		"MIGRATIONS_PATH", "LOG_LEVEL", "GIN_MODE", "CSRF_AUTH_KEY",
		"CONTENT_SECURITY_POLICY", "RATE_LIMIT_GENERAL", "RATE_LIMIT_REGISTER",
		"RATE_LIMIT_LOGIN", "RATE_LIMIT_INGEST",
	}

	// Save original values
	originalValues := make(map[string]string)
	for _, key := range testEnvVars {
		originalValues[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		// Restore original values
		for key, value := range originalValues {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Test with valid configuration
	os.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
	os.Setenv("PORT", "8080")
	os.Setenv("METRICS_PORT", "9090")
	os.Setenv("LOG_LEVEL", "info")
	os.Setenv("GIN_MODE", "debug")

	// Skip database connection test for unit tests
	// config, err := Load()
	// assert.NoError(t, err)
	// assert.NotNil(t, config)
	// assert.Equal(t, "postgres://test:test@localhost:5432/test?sslmode=disable", config.DatabaseURL)
}

func TestValidatePort(t *testing.T) {
	c := &Config{}

	// Valid ports
	assert.NoError(t, c.validatePort("8080", "PORT"))
	assert.NoError(t, c.validatePort("1", "PORT"))
	assert.NoError(t, c.validatePort("65535", "PORT"))

	// Invalid ports
	assert.Error(t, c.validatePort("", "PORT"))
	assert.Error(t, c.validatePort("abc", "PORT"))
	assert.Error(t, c.validatePort("0", "PORT"))
	assert.Error(t, c.validatePort("65536", "PORT"))
}

func TestValidateLogLevel(t *testing.T) {
	c := &Config{}

	// Valid log levels
	validLevels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic"}
	for _, level := range validLevels {
		c.LogLevel = level
		assert.NoError(t, c.validateLogLevel())
	}

	// Case insensitive
	c.LogLevel = "INFO"
	assert.NoError(t, c.validateLogLevel())

	// Invalid log levels
	c.LogLevel = "invalid"
	assert.Error(t, c.validateLogLevel())
}

func TestValidateGinMode(t *testing.T) {
	c := &Config{}

	// Valid modes
	validModes := []string{"debug", "release", "test"}
	for _, mode := range validModes {
		c.GinMode = mode
		assert.NoError(t, c.validateGinMode())
	}

	// Case insensitive
	c.GinMode = "DEBUG"
	assert.NoError(t, c.validateGinMode())

	// Invalid modes
	c.GinMode = "invalid"
	assert.Error(t, c.validateGinMode())
}

func TestValidateRateLimit(t *testing.T) {
	c := &Config{}

	// Valid rate limits
	assert.NoError(t, c.validateRateLimit("100-M", "RATE_LIMIT_GENERAL"))
	assert.NoError(t, c.validateRateLimit("10-S", "RATE_LIMIT_LOGIN"))
	assert.NoError(t, c.validateRateLimit("5-H", "RATE_LIMIT_REGISTER"))

	// Invalid formats
	assert.Error(t, c.validateRateLimit("", "RATE_LIMIT_GENERAL"))
	assert.Error(t, c.validateRateLimit("100", "RATE_LIMIT_GENERAL"))
	assert.Error(t, c.validateRateLimit("abc-M", "RATE_LIMIT_GENERAL"))
	assert.Error(t, c.validateRateLimit("100-X", "RATE_LIMIT_GENERAL"))
	assert.Error(t, c.validateRateLimit("100-M-invalid", "RATE_LIMIT_GENERAL"))
}

func TestValidateBindAddress(t *testing.T) {
	c := &Config{}

	// Valid addresses
	assert.NoError(t, c.validateBindAddress("127.0.0.1"))
	assert.NoError(t, c.validateBindAddress("localhost"))
	assert.NoError(t, c.validateBindAddress("0.0.0.0"))
	assert.NoError(t, c.validateBindAddress("192.168.1.1"))

	// Invalid addresses
	assert.Error(t, c.validateBindAddress(""))
	assert.Error(t, c.validateBindAddress("invalid"))
}

func TestValidateCSRFAuthKey(t *testing.T) {
	c := &Config{}

	// Generate a valid 32-byte base64 key
	validKey := "Y/d8+wuibG279h+uW9lMjtfK+vT4eLRxRGSymI0nT1I=" // 32 bytes when decoded
	c.CSRFAuthKey = validKey
	assert.NoError(t, c.validateCSRFAuthKey())

	// Invalid base64
	c.CSRFAuthKey = "invalid-base64!"
	assert.Error(t, c.validateCSRFAuthKey())

	// Wrong length
	c.CSRFAuthKey = "dGVzdA==" // Only 4 bytes when decoded
	assert.Error(t, c.validateCSRFAuthKey())
}

func TestParseSize(t *testing.T) {
	// Test KB
	assert.Equal(t, int64(1024), parseSize("1KB"))
	assert.Equal(t, int64(2048), parseSize("2KB"))

	// Test MB
	assert.Equal(t, int64(1024*1024), parseSize("1MB"))
	assert.Equal(t, int64(10*1024*1024), parseSize("10MB"))

	// Test GB
	assert.Equal(t, int64(1024*1024*1024), parseSize("1GB"))

	// Test plain numbers (bytes)
	assert.Equal(t, int64(1000), parseSize("1000"))

	// Test empty/invalid
	assert.Equal(t, int64(0), parseSize(""))
	assert.Equal(t, int64(0), parseSize("invalid"))
	assert.Equal(t, int64(0), parseSize("KB"))
}

func TestValidateRequestSizeLimits(t *testing.T) {
	c := &Config{}

	// Valid configuration
	c.MaxRequestSizeIngest = 10 * 1024 * 1024 // 10MB
	c.MaxRequestSizePost = 1 * 1024 * 1024    // 1MB
	c.MaxRequestSizeGet = 100 * 1024          // 100KB
	assert.NoError(t, c.validateRequestSizeLimits())

	// Invalid: negative values
	c.MaxRequestSizeIngest = -1
	assert.Error(t, c.validateRequestSizeLimits())
	c.MaxRequestSizeIngest = 10 * 1024 * 1024

	// Invalid: ingest smaller than post
	c.MaxRequestSizePost = 20 * 1024 * 1024 // 20MB (larger than ingest)
	assert.Error(t, c.validateRequestSizeLimits())
	c.MaxRequestSizePost = 1 * 1024 * 1024

	// Invalid: post smaller than get
	c.MaxRequestSizeGet = 2 * 1024 * 1024 // 2MB (larger than post)
	assert.Error(t, c.validateRequestSizeLimits())
}
