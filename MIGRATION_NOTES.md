# Migration Notes: Organization-Based Host Filtering

## Summary

All host repository methods have been updated to filter by `org_id` to ensure organization isolation. Users can only access hosts that belong to their organization.

## Method Signature Changes

### Storage Interface (`internal/storage/storage.go`)

**Before:**
```go
GetHost(hostID string) (*models.Report, error)
DeleteHost(hostID string) error
ListHosts() ([]*models.HostSummary, error)
GetAllHosts() ([]*models.Report, error)
```

**After:**
```go
GetHost(hostID, orgID string) (*models.Report, error)
DeleteHost(hostID, orgID string) error
ListHosts(orgID string) ([]*models.HostSummary, error)
GetAllHosts(orgID string) ([]*models.Report, error)
```

**Unchanged:**
```go
SaveHost(report *models.Report, orgID, uploadedByUserID string) error
// Already accepts orgID and uploadedByUserID parameters
```

## Implementation Details

### `SaveHost`
- **Verification**: Checks if host exists and verifies `org_id` matches before updating
- **Behavior**: Returns `ErrNotFound` if host exists but belongs to a different organization
- **Parameters**: Already accepts `orgID` and `uploadedByUserID` (no signature change needed)

### `GetHost`
- **Filtering**: `WHERE host_id = $1 AND org_id = $2`
- **Verification**: Returns `ErrNotFound` if host doesn't exist or belongs to a different organization
- **Signature Change**: Now requires `orgID` parameter

### `DeleteHost`
- **Filtering**: `DELETE FROM hosts WHERE host_id = $1 AND org_id = $2`
- **Verification**: Only deletes if host belongs to the specified organization
- **Signature Change**: Now requires `orgID` parameter

### `ListHosts`
- **Filtering**: `WHERE org_id = $1`
- **Returns**: Only hosts belonging to the specified organization
- **Signature Change**: Now requires `orgID` parameter

### `GetAllHosts`
- **Filtering**: `WHERE org_id = $1`
- **Returns**: Only hosts belonging to the specified organization
- **Signature Change**: Now requires `orgID` parameter

## Handler Updates

All handlers have been updated to:
1. Extract `orgID` from context using `middleware.GetOrgID(c)`
2. Pass `orgID` to storage methods
3. Return `401 Unauthorized` if `orgID` is not available

### Updated Handlers

- `ListHosts`: Now filters by organization
- `GetHost`: Now verifies organization before returning
- `DeleteHost`: Now verifies organization before deletion
- `Ingest`: Already uses `orgID` from context (no changes needed)
- `Health`: Updated to use a simpler database check (doesn't require org context)

## Example Usage

### In Handlers

```go
import "snailbus/internal/middleware"

func (h *Handlers) ListHosts(c *gin.Context) {
    orgID := middleware.GetOrgID(c)
    if orgID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    hosts, err := h.storage.ListHosts(orgID)
    // ...
}

func (h *Handlers) GetHost(c *gin.Context) {
    hostID := c.Param("host_id")
    orgID := middleware.GetOrgID(c)
    if orgID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    report, err := h.storage.GetHost(hostID, orgID)
    // ...
}
```

## Security Benefits

1. **Organization Isolation**: Users can only see and manage hosts in their own organization
2. **Prevents Cross-Organization Access**: Even if a user knows a host ID from another organization, they cannot access it
3. **Consistent Filtering**: All queries automatically filter by organization
4. **Update Protection**: `SaveHost` verifies organization before allowing updates

## Backward Compatibility

⚠️ **Breaking Changes**: These are breaking changes to the storage interface. Any code that directly calls these methods will need to be updated to pass the `orgID` parameter.

The handlers have been updated to work with the new signatures, so the API endpoints continue to work as expected.



