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
// @Description Creates a new user account with a new organization. The user is automatically assigned as admin role.
// @Description Only one user can be registered per organization (registration is only allowed once per organization).
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      models.RegisterRequest  true  "Registration data"
// @Success     201      {object}  models.User  "User created"
// @Failure     400      {object}  map[string]string  "Invalid request"
// @Failure     409      {object}  map[string]string  "User already exists or organization already has a user"
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

	// Check if organization name already exists
	existingOrg, err := h.storage.GetOrganizationByName(req.OrgName)
	if err == nil {
		// Organization exists, check if it has any users
		userCount, err := h.storage.CountUsersInOrganization(existingOrg.ID)
		if err != nil {
			log.Printf("Failed to count users in organization: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check organization"})
			return
		}
		if userCount > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error": "organization already has a user",
				"message": "Registration is only allowed once per organization. This organization already has a registered user.",
			})
			return
		}
		// Organization exists but has no users - reject registration
		// Users cannot join existing organizations, they must create new ones
		c.JSON(http.StatusConflict, gin.H{
			"error": "organization name already exists",
			"message": "This organization name is already taken. Please choose a different name.",
		})
		return
	}

	// Create a new organization for this user
	org, err := h.storage.CreateOrganization(req.OrgName)
	if err != nil {
		log.Printf("Failed to create organization: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create organization"})
		return
	}
	orgID := org.ID

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	// Create user with admin role automatically
	user, err := h.storage.CreateUser(req.Username, req.Email, passwordHash, orgID, "admin")
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

// GetAPIKeyFromCredentials returns an API key for a user given username and password
// @Summary     Get API key from credentials
// @Description Authenticates with username/password and returns an API key
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      models.LoginRequest  true  "Login credentials"
// @Success     200      {object}  models.CreateAPIKeyResponse  "API key created"
// @Failure     401      {object}  map[string]string  "Invalid credentials"
// @Router      /api/v1/auth/api-key [post]
func (h *Handlers) GetAPIKeyFromCredentials(c *gin.Context) {
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

	// Generate API key
	plainKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		log.Printf("Failed to generate API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
		return
	}

	// Store API key
	apiKey, err := h.storage.CreateAPIKey(user.ID, keyHash, keyPrefix, "Auto-generated from credentials", nil)
	if err != nil {
		log.Printf("Failed to store API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
		return
	}

	c.JSON(http.StatusOK, models.CreateAPIKeyResponse{
		ID:        apiKey.ID,
		Key:       plainKey, // Return the plain API key
		Name:      apiKey.Name,
		ExpiresAt: apiKey.ExpiresAt,
		CreatedAt: apiKey.CreatedAt,
	})
}

// ListUsers lists all users in the current organization (admin-only)
// @Summary     List users in organization
// @Description Returns all users in the authenticated user's organization
// @Tags        Users
// @Produce     json
// @Security    ApiKeyAuth
// @Success     200      {object}  map[string]interface{}  "List of users"
// @Failure     403      {object}  map[string]string  "Forbidden - admin role required"
// @Router      /api/v1/users [get]
func (h *Handlers) ListUsers(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userObj := user.(*models.User)

	users, err := h.storage.ListUsersByOrganization(userObj.OrgID)
	if err != nil {
		log.Printf("Failed to list users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": len(users),
	})
}

// CreateUser creates a new user in the current organization (admin-only)
// @Summary     Create user
// @Description Creates a new user in the authenticated user's organization
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Param       request  body      models.CreateUserRequest  true  "User creation data"
// @Success     201      {object}  models.User  "User created"
// @Failure     400      {object}  map[string]string  "Invalid request"
// @Failure     403      {object}  map[string]string  "Forbidden - admin role required"
// @Failure     409      {object}  map[string]string  "User already exists"
// @Router      /api/v1/users [post]
func (h *Handlers) CreateUser(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userObj := user.(*models.User)

	var req models.CreateUserRequest
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

	// Create user in the current organization
	newUser, err := h.storage.CreateUser(req.Username, req.Email, passwordHash, userObj.OrgID, req.Role)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, newUser)
}

// UpdateUserRole updates a user's role (admin-only)
// @Summary     Update user role
// @Description Updates the role of a user in the current organization. Admins cannot update their own role.
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Param       user_id  path      string  true  "User ID"
// @Param       request  body      models.UpdateUserRoleRequest  true  "Role update data"
// @Success     200      {object}  models.User  "User updated"
// @Failure     400      {object}  map[string]string  "Invalid request"
// @Failure     403      {object}  map[string]string  "Forbidden - admin role required or cannot update own role"
// @Failure     404      {object}  map[string]string  "User not found"
// @Router      /api/v1/users/{user_id}/role [put]
func (h *Handlers) UpdateUserRole(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	currentUser := user.(*models.User)
	userID := c.Param("user_id")

	// Prevent users from updating their own role
	if currentUser.ID == userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "cannot update own role",
			"message": "You cannot update your own role. Ask another admin to update it for you.",
		})
		return
	}

	// Verify the target user is in the same organization
	targetUser, err := h.storage.GetUserByID(userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		log.Printf("Failed to get user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user"})
		return
	}

	if targetUser.OrgID != currentUser.OrgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "user not in your organization",
			"message": "You can only update users in your own organization.",
		})
		return
	}

	var req models.UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update the user's role
	if err := h.storage.UpdateUserRole(userID, req.Role); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		log.Printf("Failed to update user role: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user role"})
		return
	}

	// Fetch updated user
	updatedUser, err := h.storage.GetUserByID(userID)
	if err != nil {
		log.Printf("Failed to get updated user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve updated user"})
		return
	}

	c.JSON(http.StatusOK, updatedUser)
}

// DeleteUser deletes a user from the current organization (admin-only)
// @Summary     Delete user
// @Description Deletes a user from the current organization. Admins cannot delete themselves.
// @Tags        Users
// @Produce     json
// @Security    ApiKeyAuth
// @Param       user_id  path      string  true  "User ID"
// @Success     204      "User deleted"
// @Failure     403      {object}  map[string]string  "Forbidden - admin role required or cannot delete self"
// @Failure     404      {object}  map[string]string  "User not found"
// @Router      /api/v1/users/{user_id} [delete]
func (h *Handlers) DeleteUser(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	currentUser := user.(*models.User)
	userID := c.Param("user_id")

	// Prevent users from deleting themselves
	if currentUser.ID == userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "cannot delete yourself",
			"message": "You cannot delete your own account. Ask another admin to delete it for you.",
		})
		return
	}

	// Verify the target user is in the same organization
	targetUser, err := h.storage.GetUserByID(userID)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		log.Printf("Failed to get user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user"})
		return
	}

	if targetUser.OrgID != currentUser.OrgID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "user not in your organization",
			"message": "You can only delete users in your own organization.",
		})
		return
	}

	// Delete the user
	if err := h.storage.DeleteUser(userID); err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		log.Printf("Failed to delete user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}

	c.Status(http.StatusNoContent)
}


