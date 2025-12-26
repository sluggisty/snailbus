# Middleware Package

This package provides authentication and authorization middleware for the Snailbus API.

## Middleware

### AuthMiddleware

Validates API keys from the `X-API-Key` header and sets the authenticated user in the context.

**Usage:**
```go
protected := v1.Group("")
protected.Use(middleware.AuthMiddleware(store))
{
    protected.GET("/hosts", h.ListHosts)
}
```

**Context Values Set:**
- `user_id` (string): The authenticated user's ID
- `api_key_id` (string): The API key ID used for authentication
- `user` (*models.User): The full user object

### RequireRole

Checks if the authenticated user has one of the required roles. **Must be used after AuthMiddleware.**

**Signature:**
```go
func RequireRole(requiredRoles ...string) gin.HandlerFunc
```

**Parameters:**
- `requiredRoles`: One or more role names (e.g., "admin", "editor", "viewer")

**Returns:**
- `403 Forbidden` if the user's role doesn't match any of the required roles
- Response includes:
  - `error`: "insufficient role"
  - `message`: Human-readable error message
  - `required_roles`: Array of required roles
  - `your_role`: The user's current role

**Usage Examples:**

1. **Single Role:**
```go
// Require admin role
adminOnly := protected.Group("")
adminOnly.Use(middleware.RequireRole("admin"))
{
    adminOnly.DELETE("/users/:id", h.DeleteUser)
}
```

2. **Multiple Roles (OR logic):**
```go
// Require either editor or admin role
editorOrAdmin := protected.Group("")
editorOrAdmin.Use(middleware.RequireRole("editor", "admin"))
{
    editorOrAdmin.DELETE("/hosts/:host_id", h.DeleteHost)
    editorOrAdmin.POST("/ingest", h.Ingest)
}
```

3. **Direct Route Usage:**
```go
// Apply middleware directly to a route
v1.DELETE("/hosts/:host_id",
    middleware.AuthMiddleware(store),
    middleware.RequireRole("editor", "admin"),
    h.DeleteHost,
)
```

4. **Nested Groups:**
```go
protected := v1.Group("")
protected.Use(middleware.AuthMiddleware(store))
{
    // All authenticated users can view
    protected.GET("/hosts", h.ListHosts)
    
    // Only editors and admins can modify
    editorOrAdmin := protected.Group("")
    editorOrAdmin.Use(middleware.RequireRole("editor", "admin"))
    {
        editorOrAdmin.POST("/hosts", h.CreateHost)
        editorOrAdmin.PUT("/hosts/:id", h.UpdateHost)
        editorOrAdmin.DELETE("/hosts/:id", h.DeleteHost)
    }
    
    // Only admins can manage users
    adminOnly := protected.Group("")
    adminOnly.Use(middleware.RequireRole("admin"))
    {
        adminOnly.GET("/users", h.ListUsers)
        adminOnly.POST("/users", h.CreateUser)
    }
}
```

## Error Responses

### 401 Unauthorized
Returned when:
- API key is missing
- API key is invalid
- API key is expired
- User account is inactive

### 403 Forbidden
Returned by `RequireRole` when:
- User's role doesn't match any of the required roles

**Example 403 Response:**
```json
{
  "error": "insufficient role",
  "message": "Your role does not have permission to access this resource",
  "required_roles": ["admin", "editor"],
  "your_role": "viewer"
}
```

## Best Practices

1. **Always use AuthMiddleware first:** `RequireRole` depends on the user being set in context by `AuthMiddleware`.

2. **Order matters:** Apply middleware in the correct order:
   ```go
   // ✅ Correct
   protected.Use(middleware.AuthMiddleware(store))
   protected.Use(middleware.RequireRole("admin"))
   
   // ❌ Wrong - RequireRole will fail without AuthMiddleware
   protected.Use(middleware.RequireRole("admin"))
   protected.Use(middleware.AuthMiddleware(store))
   ```

3. **Use groups for organization:** Group routes by permission level for cleaner code:
   ```go
   protected := v1.Group("")
   protected.Use(middleware.AuthMiddleware(store))
   
   // Viewer access
   protected.GET("/hosts", h.ListHosts)
   
   // Editor/Admin access
   editorOrAdmin := protected.Group("")
   editorOrAdmin.Use(middleware.RequireRole("editor", "admin"))
   {
       editorOrAdmin.POST("/hosts", h.CreateHost)
   }
   ```

4. **Role hierarchy:** Consider implementing role hierarchy if needed (e.g., admin can do everything editor can do). Currently, you must explicitly list all allowed roles.

## Available Roles

- `admin`: Full access to all resources
- `editor`: Can create, update, and delete resources
- `viewer`: Read-only access

### OrgContextMiddleware

Extracts organization ID and role from the authenticated user and makes them easily accessible in handlers.

**Signature:**
```go
func OrgContextMiddleware() gin.HandlerFunc
```

**Usage:**
```go
protected := v1.Group("")
protected.Use(middleware.AuthMiddleware(store))
protected.Use(middleware.OrgContextMiddleware())
{
    protected.GET("/hosts", h.ListHosts)
}
```

**Context Values Set:**
- `org_id` (string): The authenticated user's organization ID
- `role` (string): The authenticated user's role

**Helper Functions:**

Handlers can use these convenience functions to access context values:

```go
import "snailbus/internal/middleware"

func (h *Handlers) MyHandler(c *gin.Context) {
    orgID := middleware.GetOrgID(c)
    role := middleware.GetRole(c)
    userID := middleware.GetUserID(c)
    
    // Use orgID, role, userID in your handler logic
}
```

**Example Handler Usage:**

```go
func (h *Handlers) ListHosts(c *gin.Context) {
    orgID := middleware.GetOrgID(c)
    if orgID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    
    // Filter hosts by organization
    hosts, err := h.storage.ListHostsByOrganization(orgID)
    // ...
}

func (h *Handlers) CreateResource(c *gin.Context) {
    orgID := middleware.GetOrgID(c)
    role := middleware.GetRole(c)
    
    // Only admins and editors can create
    if role != "admin" && role != "editor" {
        c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
        return
    }
    
    // Create resource with orgID
    // ...
}
```

**Direct Context Access:**

You can also access values directly from context:

```go
orgID, _ := c.Get("org_id")
role, _ := c.Get("role")
```

**Note:** `OrgContextMiddleware` must be used **after** `AuthMiddleware`, as it depends on the `user` object being set in the context.

