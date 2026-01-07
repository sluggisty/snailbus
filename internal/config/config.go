package config

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL driver for validation
)

// Config holds all application configuration with validation
type Config struct {
	// Required configuration
	DatabaseURL string

	// Server configuration
	Port            string
	MetricsPort     string
	MetricsBindAddr string

	// Application paths
	MigrationsPath string

	// Logging configuration
	LogLevel string
	GinMode  string

	// Security configuration
	CSRFAuthKey           string
	ContentSecurityPolicy string

	// Rate limiting configuration
	RateLimitGeneral  string
	RateLimitRegister string
	RateLimitLogin    string
	RateLimitIngest   string
}

// Load loads and validates configuration from environment variables
func Load() (*Config, error) {
	config := &Config{}

	// Load configuration from environment
	if err := config.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// loadFromEnv loads configuration values from environment variables
func (c *Config) loadFromEnv() error {
	c.DatabaseURL = getEnv("DATABASE_URL", "postgres://snail:snail_secret@localhost:5432/snailbus?sslmode=disable")
	c.Port = getEnv("PORT", "8080")
	c.MetricsPort = getEnv("METRICS_PORT", "9090")
	c.MetricsBindAddr = getEnv("METRICS_BIND_ADDRESS", "127.0.0.1")
	c.MigrationsPath = getEnv("MIGRATIONS_PATH", "file://migrations")
	c.LogLevel = getEnv("LOG_LEVEL", "info")
	c.GinMode = getEnv("GIN_MODE", "debug")
	c.CSRFAuthKey = os.Getenv("CSRF_AUTH_KEY") // No default, optional
	c.ContentSecurityPolicy = getEnv("CONTENT_SECURITY_POLICY",
		"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';")

	// Rate limiting configuration
	c.RateLimitGeneral = getEnv("RATE_LIMIT_GENERAL", "100-M")
	c.RateLimitRegister = getEnv("RATE_LIMIT_REGISTER", "5-M")
	c.RateLimitLogin = getEnv("RATE_LIMIT_LOGIN", "10-M")
	c.RateLimitIngest = getEnv("RATE_LIMIT_INGEST", "50-M")

	return nil
}

// validate performs comprehensive validation of all configuration values
func (c *Config) validate() error {
	var errors []string

	// Validate DATABASE_URL
	if err := c.validateDatabaseURL(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate ports
	if err := c.validatePort(c.Port, "PORT"); err != nil {
		errors = append(errors, err.Error())
	}
	if err := c.validatePort(c.MetricsPort, "METRICS_PORT"); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate METRICS_BIND_ADDRESS
	if err := c.validateBindAddress(c.MetricsBindAddr); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate LOG_LEVEL
	if err := c.validateLogLevel(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate GIN_MODE
	if err := c.validateGinMode(); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate CSRF_AUTH_KEY if provided
	if c.CSRFAuthKey != "" {
		if err := c.validateCSRFAuthKey(); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate rate limit formats
	rateLimitFields := map[string]string{
		"RATE_LIMIT_GENERAL":  c.RateLimitGeneral,
		"RATE_LIMIT_REGISTER": c.RateLimitRegister,
		"RATE_LIMIT_LOGIN":    c.RateLimitLogin,
		"RATE_LIMIT_INGEST":   c.RateLimitIngest,
	}

	for fieldName, value := range rateLimitFields {
		if err := c.validateRateLimit(value, fieldName); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation errors:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// validateDatabaseURL validates that DATABASE_URL is a valid PostgreSQL connection string
func (c *Config) validateDatabaseURL() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	// Parse the URL to validate format
	parsedURL, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return fmt.Errorf("DATABASE_URL is not a valid URL: %w", err)
	}

	// Check for PostgreSQL scheme
	if parsedURL.Scheme != "postgres" && parsedURL.Scheme != "postgresql" {
		return fmt.Errorf("DATABASE_URL must use postgres:// or postgresql:// scheme")
	}

	// Try to establish a connection (but don't keep it open)
	db, err := sql.Open("postgres", c.DatabaseURL)
	if err != nil {
		return fmt.Errorf("DATABASE_URL connection test failed: %w", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("DATABASE_URL ping test failed: %w", err)
	}

	return nil
}

// validatePort validates that a port number is in valid range (1-65535)
func (c *Config) validatePort(portStr, fieldName string) error {
	if portStr == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("%s must be a valid number: %s", fieldName, portStr)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535: %d", fieldName, port)
	}

	return nil
}

// validateBindAddress validates that the bind address is a valid IP or hostname
func (c *Config) validateBindAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("METRICS_BIND_ADDRESS cannot be empty")
	}

	// Basic validation - should contain at least one dot or be localhost/127.0.0.1
	if addr != "localhost" && addr != "127.0.0.1" && !strings.Contains(addr, ".") {
		return fmt.Errorf("METRICS_BIND_ADDRESS should be a valid IP address or hostname: %s", addr)
	}

	return nil
}

// validateLogLevel validates that LOG_LEVEL is one of the accepted values
func (c *Config) validateLogLevel() error {
	validLevels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic"}
	level := strings.ToLower(c.LogLevel)

	for _, validLevel := range validLevels {
		if level == validLevel {
			return nil
		}
	}

	return fmt.Errorf("LOG_LEVEL must be one of: %s (got: %s)", strings.Join(validLevels, ", "), c.LogLevel)
}

// validateGinMode validates that GIN_MODE is one of the accepted values
func (c *Config) validateGinMode() error {
	validModes := []string{"debug", "release", "test"}
	mode := strings.ToLower(c.GinMode)

	for _, validMode := range validModes {
		if mode == validMode {
			return nil
		}
	}

	return fmt.Errorf("GIN_MODE must be one of: %s (got: %s)", strings.Join(validModes, ", "), c.GinMode)
}

// validateCSRFAuthKey validates CSRF_AUTH_KEY format if provided
func (c *Config) validateCSRFAuthKey() error {
	// Decode base64 to check if it's valid and 32 bytes
	decoded, err := decodeBase64(c.CSRFAuthKey)
	if err != nil {
		return fmt.Errorf("CSRF_AUTH_KEY must be valid base64: %w", err)
	}

	if len(decoded) != 32 {
		return fmt.Errorf("CSRF_AUTH_KEY must decode to exactly 32 bytes (got %d bytes)", len(decoded))
	}

	return nil
}

// validateRateLimit validates rate limit format (number-unit)
func (c *Config) validateRateLimit(value, fieldName string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return fmt.Errorf("%s must be in format 'number-unit' (e.g., '100-M'): %s", fieldName, value)
	}

	number := parts[0]
	unit := strings.ToUpper(parts[1])

	// Validate number
	if _, err := strconv.Atoi(number); err != nil {
		return fmt.Errorf("%s number must be valid integer: %s", fieldName, number)
	}

	// Validate unit
	validUnits := []string{"S", "M", "H"}
	valid := false
	for _, validUnit := range validUnits {
		if unit == validUnit {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("%s unit must be S, M, or H: %s", fieldName, unit)
	}

	return nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// decodeBase64 decodes a base64 string
func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
