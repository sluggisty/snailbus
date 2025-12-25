package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"snailbus/internal/auth"
	"snailbus/internal/models"
	"snailbus/internal/storage"
)

// Register handles user registration
// @Summary     Register new user
// @Description Creates a new user account
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      models.RegisterRequest  true  "Registration data"
// @Success     201      {object}  models.User  "User created"
// @Failure     400      {object}  map[string]string  "Invalid request"
// @Failure     409      {object}  map[string]string  "User already exists"
// @Router      /api/v1/auth/register [post]
func (h *Handlers) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if username already exists
	_, _, err := h.storage.GetUserByUsername(req.Username)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}

	// Check if email already exists
	_, err = h.storage.GetUserByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	// Create user
	user, err := h.storage.CreateUser(req.Username, req.Email, passwordHash)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// Login handles user login and returns an API key
// @Summary     Login
// @Description Authenticates a user and returns an API key for this session
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      models.LoginRequest  true  "Login credentials"
// @Success     200      {object}  models.LoginResponse  "Login successful"
// @Failure     401      {object}  map[string]string  "Invalid credentials"
// @Router      /api/v1/auth/login [post]
func (h *Handlers) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user and password hash
	user, passwordHash, err := h.storage.GetUserByUsername(req.Username)
	if err != nil {
		// Don't reveal if user exists
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Check if user is active
	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "account is inactive"})
		return
	}

	// Verify password
	if !auth.CheckPassword(req.Password, passwordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate API key for this session
	plainKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		log.Printf("Failed to generate API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	// Store API key
	_, err = h.storage.CreateAPIKey(user.ID, keyHash, keyPrefix, "Web UI Session", nil)
	if err != nil {
		log.Printf("Failed to store API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{
		User:  user,
		Token: plainKey, // Return the plain API key as "token"
	})
}

// CreateAPIKey creates a new API key for the authenticated user
// @Summary     Create API key
// @Description Creates a new API key for the authenticated user
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Param       request  body      models.CreateAPIKeyRequest  true  "API key details"
// @Success     201      {object}  models.CreateAPIKeyResponse  "API key created"
// @Failure     400      {object}  map[string]string  "Invalid request"
// @Router      /api/v1/api-keys [post]
func (h *Handlers) CreateAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req models.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate API key
	plainKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		log.Printf("Failed to generate API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
		return
	}

	// Store API key
	apiKey, err := h.storage.CreateAPIKey(userID.(string), keyHash, keyPrefix, req.Name, req.ExpiresAt)
	if err != nil {
		log.Printf("Failed to store API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, models.CreateAPIKeyResponse{
		ID:        apiKey.ID,
		Key:       plainKey, // Show plain key only once
		Name:      apiKey.Name,
		ExpiresAt: apiKey.ExpiresAt,
		CreatedAt: apiKey.CreatedAt,
	})
}

// ListAPIKeys lists all API keys for the authenticated user
// @Summary     List API keys
// @Description Returns all API keys for the authenticated user
// @Tags        Auth
// @Produce     json
// @Security    ApiKeyAuth
// @Success     200      {object}  map[string]interface{}  "List of API keys"
// @Router      /api/v1/api-keys [get]
func (h *Handlers) ListAPIKeys(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	apiKeys, err := h.storage.GetAPIKeysByUserID(userID.(string))
	if err != nil {
		log.Printf("Failed to list API keys: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve API keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_keys": apiKeys,
		"total":    len(apiKeys),
	})
}

// DeleteAPIKey deletes an API key
// @Summary     Delete API key
// @Description Deletes an API key by ID
// @Tags        Auth
// @Produce     json
// @Security    ApiKeyAuth
// @Param       id  path      string  true  "API key ID"
// @Success     204  "API key deleted"
// @Failure     404  {object}  map[string]string  "API key not found"
// @Router      /api/v1/api-keys/{id} [delete]
func (h *Handlers) DeleteAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	keyID := c.Param("id")

	// Verify the key belongs to the user
	apiKeys, err := h.storage.GetAPIKeysByUserID(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify ownership"})
		return
	}

	// Check if key belongs to user
	found := false
	for _, key := range apiKeys {
		if key.ID == keyID {
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	// Delete the key
	if err := h.storage.DeleteAPIKey(keyID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
			return
		}
		log.Printf("Failed to delete API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete API key"})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetMe returns the current authenticated user
// @Summary     Get current user
// @Description Returns information about the currently authenticated user
// @Tags        Auth
// @Produce     json
// @Security    ApiKeyAuth
// @Success     200  {object}  models.User  "User information"
// @Router      /api/v1/auth/me [get]
func (h *Handlers) GetMe(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, user)
}


