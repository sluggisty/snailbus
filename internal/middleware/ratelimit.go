package middleware

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"

	"snailbus/internal/logger"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// General authenticated endpoints (per API key)
	GeneralLimit string

	// Public endpoints
	RegisterLimit string
	LoginLimit    string

	// Ingest endpoint (stricter limit)
	IngestLimit string
}

// getRateLimitConfig loads rate limit configuration from environment variables
func getRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		GeneralLimit:  getEnv("RATE_LIMIT_GENERAL", "100-M"), // 100 requests per minute
		RegisterLimit: getEnv("RATE_LIMIT_REGISTER", "10-M"), // 10 requests per minute
		LoginLimit:    getEnv("RATE_LIMIT_LOGIN", "20-M"),    // 20 requests per minute
		IngestLimit:   getEnv("RATE_LIMIT_INGEST", "50-M"),   // 50 requests per minute
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createLimiter creates a new limiter with the given rate
func createLimiter(rate limiter.Rate) *limiter.Limiter {
	store := memory.NewStore()
	return limiter.New(store, rate)
}

// parseRate extracts the rate from a rate string (e.g., "100-M" -> 100 requests per minute)
func parseRate(rateStr string) limiter.Rate {
	rate, err := limiter.NewRateFromFormatted(rateStr)
	if err != nil {
		logger.Logger.Warn().
			Err(err).
			Str("rate_string", rateStr).
			Msg("Failed to parse rate limit, using default 100-M")
		rate, _ = limiter.NewRateFromFormatted("100-M")
	}
	return rate
}

// IPRateLimitMiddleware creates middleware for IP-based rate limiting
func IPRateLimitMiddleware(rateStr string) gin.HandlerFunc {
	rate := parseRate(rateStr)
	store := memory.NewStore()
	instance := limiter.New(store, rate)

	return mgin.NewMiddleware(instance, mgin.WithLimitReachedHandler(func(c *gin.Context) {
		c.Header("Retry-After", strconv.Itoa(int(rate.Period.Seconds())))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":       "rate limit exceeded",
			"message":     "Too many requests from this IP address",
			"retry_after": int(rate.Period.Seconds()),
			"limit":       rate.Limit,
			"period":      rate.Period.String(),
			"reset_time":  time.Now().Add(rate.Period).Format(time.RFC3339),
		})
		c.Abort()
	}))
}

// APIKeyRateLimitMiddleware creates middleware for API key-based rate limiting
func APIKeyRateLimitMiddleware(rateStr string) gin.HandlerFunc {
	rate := parseRate(rateStr)
	store := memory.NewStore()
	instance := limiter.New(store, rate)

	return func(c *gin.Context) {
		// Get API key from header (same logic as AuthMiddleware)
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Also check Authorization header for backward compatibility
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				// Support both "Bearer <key>" and "ApiKey <key>" formats
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 {
					apiKey = parts[1]
				}
			}
		}

		// If no API key found, use client IP as fallback
		key := apiKey
		if key == "" {
			key = c.ClientIP()
		}

		// Check rate limit
		context, err := instance.Get(c, key)
		if err != nil {
			logger.Logger.Error().Err(err).Str("key", key).Msg("Rate limit check failed")
			c.Next() // Allow request on error
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))

		if context.Reached {
			c.Header("Retry-After", strconv.Itoa(int(rate.Period.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"message":     "Too many requests",
				"retry_after": int(rate.Period.Seconds()),
				"limit":       rate.Limit,
				"period":      rate.Period.String(),
				"reset_time":  time.Now().Add(rate.Period).Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// InitRateLimitMiddleware initializes and returns rate limiting middleware functions
func InitRateLimitMiddleware() (gin.HandlerFunc, gin.HandlerFunc, gin.HandlerFunc, gin.HandlerFunc) {
	config := getRateLimitConfig()

	logger.Logger.Info().
		Str("general_limit", config.GeneralLimit).
		Str("register_limit", config.RegisterLimit).
		Str("login_limit", config.LoginLimit).
		Str("ingest_limit", config.IngestLimit).
		Msg("Initializing rate limiting middleware")

	// Create different limiters for different endpoints
	generalLimiter := APIKeyRateLimitMiddleware(config.GeneralLimit)
	registerLimiter := IPRateLimitMiddleware(config.RegisterLimit)
	loginLimiter := IPRateLimitMiddleware(config.LoginLimit)
	ingestLimiter := APIKeyRateLimitMiddleware(config.IngestLimit)

	return generalLimiter, registerLimiter, loginLimiter, ingestLimiter
}
