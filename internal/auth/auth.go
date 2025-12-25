package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// APIKeyLength is the length of generated API keys (in bytes, before base64 encoding)
	APIKeyLength = 32
	// BcryptCost is the cost factor for bcrypt password hashing
	BcryptCost = 12
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a password against a hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateAPIKey generates a new API key
// Returns the plain key (to show to user once), the hash (to store in DB), and prefix (for lookup)
func GenerateAPIKey() (plainKey string, keyHash string, keyPrefix string, err error) {
	// Generate random bytes
	keyBytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random key: %w", err)
	}

	// Encode as base64 for the plain key (user-friendly)
	plainKey = base64.URLEncoding.EncodeToString(keyBytes)

	// Extract prefix (first 8 characters) for efficient lookup
	if len(plainKey) >= 8 {
		keyPrefix = plainKey[:8]
	} else {
		keyPrefix = plainKey
	}

	// Hash the key for storage (using bcrypt)
	keyHashBytes, err := bcrypt.GenerateFromPassword(keyBytes, BcryptCost)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to hash API key: %w", err)
	}
	keyHash = string(keyHashBytes)

	return plainKey, keyHash, keyPrefix, nil
}

// VerifyAPIKey verifies an API key against a stored hash
func VerifyAPIKey(plainKey, keyHash string) bool {
	// Decode the base64 plain key
	keyBytes, err := base64.URLEncoding.DecodeString(plainKey)
	if err != nil {
		return false
	}

	// Compare using bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(keyHash), keyBytes)
	return err == nil
}

// HashAPIKey hashes an API key for storage (used when importing existing keys)
// Returns hash and prefix
func HashAPIKey(plainKey string) (keyHash string, keyPrefix string, err error) {
	keyBytes, err := base64.URLEncoding.DecodeString(plainKey)
	if err != nil {
		return "", "", fmt.Errorf("invalid API key format: %w", err)
	}

	// Extract prefix
	if len(plainKey) >= 8 {
		keyPrefix = plainKey[:8]
	} else {
		keyPrefix = plainKey
	}

	keyHashBytes, err := bcrypt.GenerateFromPassword(keyBytes, BcryptCost)
	if err != nil {
		return "", "", fmt.Errorf("failed to hash API key: %w", err)
	}

	return string(keyHashBytes), keyPrefix, nil
}

// GetKeyPrefix extracts the prefix from a plain API key
func GetKeyPrefix(plainKey string) string {
	if len(plainKey) >= 8 {
		return plainKey[:8]
	}
	return plainKey
}

// ConstantTimeCompare performs a constant-time comparison of two strings
// This helps prevent timing attacks
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// IsExpired checks if a timestamp has passed
func IsExpired(expiresAt *time.Time) bool {
	if expiresAt == nil {
		return false // No expiration
	}
	return time.Now().After(*expiresAt)
}

