package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	// RequestIDKey is the key used to store request ID in context
	RequestIDKey = "request_id"
)

var (
	// Logger is the global logger instance
	Logger zerolog.Logger
)

// Init initializes the global logger based on environment
func Init() {
	// Determine log level from environment
	logLevel := getLogLevel()

	// Configure output format based on GIN_MODE
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "release" {
		// Production: JSON output
		Logger = zerolog.New(os.Stdout).
			With().
			Timestamp().
			Logger().
			Level(logLevel)
	} else {
		// Development: Human-readable output with colors
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
		Logger = zerolog.New(output).
			With().
			Timestamp().
			Logger().
			Level(logLevel)
	}

	// Set as global logger
	log.Logger = Logger
}

// getLogLevel returns the log level from environment variable
func getLogLevel() zerolog.Level {
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		// Default to info level
		return zerolog.InfoLevel
	}

	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		// Invalid level, default to info
		return zerolog.InfoLevel
	}

	return level
}

// FromContext creates a logger with context from Gin context
// It extracts request_id, user_id, org_id, and role from the context
func FromContext(c interface {
	Get(any) (any, bool)
}) *zerolog.Event {
	event := Logger.Info()

	// Extract request_id
	if requestID, exists := c.Get(RequestIDKey); exists {
		if id, ok := requestID.(string); ok {
			event = event.Str("request_id", id)
		}
	}

	// Extract user_id
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok {
			event = event.Str("user_id", id)
		}
	}

	// Extract org_id
	if orgID, exists := c.Get("org_id"); exists {
		if id, ok := orgID.(string); ok {
			event = event.Str("org_id", id)
		}
	}

	// Extract role
	if role, exists := c.Get("role"); exists {
		if r, ok := role.(string); ok {
			event = event.Str("role", r)
		}
	}

	return event
}

// WithRequestID creates a logger event with request ID
func WithRequestID(requestID string) *zerolog.Event {
	return Logger.Info().Str("request_id", requestID)
}

// WithUser creates a logger event with user context
func WithUser(userID, orgID, role string) *zerolog.Event {
	event := Logger.Info()
	if userID != "" {
		event = event.Str("user_id", userID)
	}
	if orgID != "" {
		event = event.Str("org_id", orgID)
	}
	if role != "" {
		event = event.Str("role", role)
	}
	return event
}

// WithHost creates a logger event with host context
func WithHost(hostID, hostname string) *zerolog.Event {
	event := Logger.Info()
	if hostID != "" {
		event = event.Str("host_id", hostID)
	}
	if hostname != "" {
		event = event.Str("hostname", hostname)
	}
	return event
}
