package testutils

import (
	"encoding/json"
	"time"

	"snailbus/internal/models"
)

// ReportBuilder is a builder for creating test Report objects
type ReportBuilder struct {
	report *models.Report
}

// NewReportBuilder creates a new ReportBuilder with default values
func NewReportBuilder() *ReportBuilder {
	return &ReportBuilder{
		report: &models.Report{
			ID:         "00000000-0000-0000-0000-000000000001",
			ReceivedAt: time.Now().UTC(),
			Meta: models.ReportMeta{
				Hostname:     "test-host",
				HostID:       "00000000-0000-0000-0000-000000000001",
				CollectionID: "test-collection-id",
				Timestamp:    time.Now().UTC().Format(time.RFC3339),
				SnailVersion: "0.2.0",
			},
			Data:   json.RawMessage(`{"system": {"os_name": "Fedora", "os_version": "42"}}`),
			Errors: []string{},
		},
	}
}

// WithHostID sets the host ID
func (b *ReportBuilder) WithHostID(hostID string) *ReportBuilder {
	b.report.ID = hostID
	b.report.Meta.HostID = hostID
	return b
}

// WithHostname sets the hostname
func (b *ReportBuilder) WithHostname(hostname string) *ReportBuilder {
	b.report.Meta.Hostname = hostname
	return b
}

// WithCollectionID sets the collection ID
func (b *ReportBuilder) WithCollectionID(collectionID string) *ReportBuilder {
	b.report.Meta.CollectionID = collectionID
	return b
}

// WithTimestamp sets the timestamp
func (b *ReportBuilder) WithTimestamp(timestamp time.Time) *ReportBuilder {
	b.report.Meta.Timestamp = timestamp.Format(time.RFC3339)
	b.report.ReceivedAt = timestamp
	return b
}

// WithSnailVersion sets the snail version
func (b *ReportBuilder) WithSnailVersion(version string) *ReportBuilder {
	b.report.Meta.SnailVersion = version
	return b
}

// WithData sets the report data
func (b *ReportBuilder) WithData(data json.RawMessage) *ReportBuilder {
	b.report.Data = data
	return b
}

// WithDataJSON sets the report data from a JSON string
func (b *ReportBuilder) WithDataJSON(jsonStr string) *ReportBuilder {
	b.report.Data = json.RawMessage(jsonStr)
	return b
}

// WithErrors sets the errors list
func (b *ReportBuilder) WithErrors(errors []string) *ReportBuilder {
	b.report.Errors = errors
	return b
}

// WithError adds an error to the errors list
func (b *ReportBuilder) WithError(err string) *ReportBuilder {
	b.report.Errors = append(b.report.Errors, err)
	return b
}

// Build returns the built Report
func (b *ReportBuilder) Build() *models.Report {
	return b.report
}

// UserBuilder is a builder for creating test User objects
type UserBuilder struct {
	user *models.User
}

// NewUserBuilder creates a new UserBuilder with default values
func NewUserBuilder() *UserBuilder {
	return &UserBuilder{
		user: &models.User{
			ID:        "00000000-0000-0000-0000-000000000010",
			Username:  "testuser",
			Email:     "test@example.com",
			Role:      "viewer",
			IsActive:  true,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
}

// WithID sets the user ID
func (b *UserBuilder) WithID(id string) *UserBuilder {
	b.user.ID = id
	return b
}

// WithUsername sets the username
func (b *UserBuilder) WithUsername(username string) *UserBuilder {
	b.user.Username = username
	return b
}

// WithEmail sets the email
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.user.Email = email
	return b
}

// WithRole sets the role
func (b *UserBuilder) WithRole(role string) *UserBuilder {
	b.user.Role = role
	return b
}

// WithOrgID sets the organization ID
func (b *UserBuilder) WithOrgID(orgID string) *UserBuilder {
	b.user.OrgID = orgID
	return b
}

// WithActive sets the active status
func (b *UserBuilder) WithActive(active bool) *UserBuilder {
	b.user.IsActive = active
	return b
}

// WithCreatedAt sets the created at timestamp
func (b *UserBuilder) WithCreatedAt(t time.Time) *UserBuilder {
	b.user.CreatedAt = t
	return b
}

// WithUpdatedAt sets the updated at timestamp
func (b *UserBuilder) WithUpdatedAt(t time.Time) *UserBuilder {
	b.user.UpdatedAt = t
	return b
}

// Build returns the built User
func (b *UserBuilder) Build() *models.User {
	return b.user
}

// OrganizationBuilder is a builder for creating test Organization objects
type OrganizationBuilder struct {
	org *models.Organization
}

// NewOrganizationBuilder creates a new OrganizationBuilder with default values
func NewOrganizationBuilder() *OrganizationBuilder {
	return &OrganizationBuilder{
		org: &models.Organization{
			ID:        "00000000-0000-0000-0000-000000000100",
			Name:      "Test Organization",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
}

// WithID sets the organization ID
func (b *OrganizationBuilder) WithID(id string) *OrganizationBuilder {
	b.org.ID = id
	return b
}

// WithName sets the organization name
func (b *OrganizationBuilder) WithName(name string) *OrganizationBuilder {
	b.org.Name = name
	return b
}

// WithCreatedAt sets the created at timestamp
func (b *OrganizationBuilder) WithCreatedAt(t time.Time) *OrganizationBuilder {
	b.org.CreatedAt = t
	return b
}

// WithUpdatedAt sets the updated at timestamp
func (b *OrganizationBuilder) WithUpdatedAt(t time.Time) *OrganizationBuilder {
	b.org.UpdatedAt = t
	return b
}

// Build returns the built Organization
func (b *OrganizationBuilder) Build() *models.Organization {
	return b.org
}

// IngestRequestBuilder is a builder for creating test IngestRequest objects
type IngestRequestBuilder struct {
	request *models.IngestRequest
}

// NewIngestRequestBuilder creates a new IngestRequestBuilder with default values
func NewIngestRequestBuilder() *IngestRequestBuilder {
	return &IngestRequestBuilder{
		request: &models.IngestRequest{
			Meta: models.ReportMeta{
				HostID:       "00000000-0000-0000-0000-000000000001",
				Hostname:     "test-host",
				CollectionID: "test-collection-id",
				Timestamp:    time.Now().UTC().Format(time.RFC3339),
				SnailVersion: "0.2.0",
			},
			Data:   json.RawMessage(`{"system": {"os_name": "Fedora", "os_version": "42"}}`),
			Errors: []string{},
		},
	}
}

// WithHostID sets the host ID
func (b *IngestRequestBuilder) WithHostID(hostID string) *IngestRequestBuilder {
	b.request.Meta.HostID = hostID
	return b
}

// WithHostname sets the hostname
func (b *IngestRequestBuilder) WithHostname(hostname string) *IngestRequestBuilder {
	b.request.Meta.Hostname = hostname
	return b
}

// WithCollectionID sets the collection ID
func (b *IngestRequestBuilder) WithCollectionID(collectionID string) *IngestRequestBuilder {
	b.request.Meta.CollectionID = collectionID
	return b
}

// WithTimestamp sets the timestamp
func (b *IngestRequestBuilder) WithTimestamp(timestamp time.Time) *IngestRequestBuilder {
	b.request.Meta.Timestamp = timestamp.Format(time.RFC3339)
	return b
}

// WithSnailVersion sets the snail version
func (b *IngestRequestBuilder) WithSnailVersion(version string) *IngestRequestBuilder {
	b.request.Meta.SnailVersion = version
	return b
}

// WithData sets the request data
func (b *IngestRequestBuilder) WithData(data json.RawMessage) *IngestRequestBuilder {
	b.request.Data = data
	return b
}

// WithDataJSON sets the request data from a JSON string
func (b *IngestRequestBuilder) WithDataJSON(jsonStr string) *IngestRequestBuilder {
	b.request.Data = json.RawMessage(jsonStr)
	return b
}

// WithErrors sets the errors list
func (b *IngestRequestBuilder) WithErrors(errors []string) *IngestRequestBuilder {
	b.request.Errors = errors
	return b
}

// WithError adds an error to the errors list
func (b *IngestRequestBuilder) WithError(err string) *IngestRequestBuilder {
	b.request.Errors = append(b.request.Errors, err)
	return b
}

// Build returns the built IngestRequest
func (b *IngestRequestBuilder) Build() *models.IngestRequest {
	return b.request
}
