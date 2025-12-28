package models

import "time"

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	IsActive  bool      `json:"is_active"`
	IsAdmin   bool      `json:"is_admin"`
	OrgID     string    `json:"org_id"` // Required foreign key to organizations
	Role      string    `json:"role"`   // Required enum: 'admin', 'editor', 'viewer'
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey represents an API key
type APIKey struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	KeyHash    string     `json:"-"` // Never return the hash
	KeyPrefix  string     `json:"-"` // Never return the prefix
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateAPIKeyRequest is used when creating a new API key
type CreateAPIKeyRequest struct {
	Name      string     `json:"name" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreateAPIKeyResponse is returned when creating a new API key
type CreateAPIKeyResponse struct {
	ID        string     `json:"id"`
	Key       string     `json:"key"` // Plain key, shown only once
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// LoginRequest is used for user login
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest is used for user registration
// When a user registers, a new organization is automatically created and the user is assigned as admin
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	OrgName  string `json:"org_name" binding:"required,min=1,max=100"` // Name for the new organization
}

// LoginResponse is returned after successful login
type LoginResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token"` // API key for this session
}

// CreateUserRequest is used by admins to create new users
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role" binding:"required,oneof=admin editor viewer"`
}

// UpdateUserRoleRequest is used by admins to update a user's role
type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=admin editor viewer"`
}
