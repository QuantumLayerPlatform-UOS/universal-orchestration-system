package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ProjectStatus represents the status of a project
type ProjectStatus string

const (
	ProjectStatusActive      ProjectStatus = "active"
	ProjectStatusInactive    ProjectStatus = "inactive"
	ProjectStatusArchived    ProjectStatus = "archived"
	ProjectStatusSuspended   ProjectStatus = "suspended"
	ProjectStatusInitializing ProjectStatus = "initializing"
)

// ProjectType represents different types of projects
type ProjectType string

const (
	ProjectTypeStandard    ProjectType = "standard"
	ProjectTypeEnterprise  ProjectType = "enterprise"
	ProjectTypeResearch    ProjectType = "research"
	ProjectTypeEducational ProjectType = "educational"
	ProjectTypePersonal    ProjectType = "personal"
)

// Project represents a project in the system
type Project struct {
	ID               string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name             string          `gorm:"not null;unique" json:"name"`
	Description      string          `json:"description"`
	Type             ProjectType     `gorm:"not null;default:'standard'" json:"type"`
	Status           ProjectStatus   `gorm:"not null;default:'active';index" json:"status"`
	OrganizationID   *string         `gorm:"type:uuid;index" json:"organization_id,omitempty"`
	OwnerID          string          `gorm:"not null;index" json:"owner_id"`
	Tags             []string        `gorm:"type:text[]" json:"tags,omitempty"`
	Metadata         json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	Settings         json.RawMessage `gorm:"type:jsonb" json:"settings,omitempty"`
	ResourceLimits   json.RawMessage `gorm:"type:jsonb" json:"resource_limits,omitempty"`
	Features         []string        `gorm:"type:text[]" json:"features,omitempty"`
	Version          string          `json:"version"`
	Repository       string          `json:"repository,omitempty"`
	DefaultBranch    string          `json:"default_branch,omitempty"`
	Language         string          `json:"language,omitempty"`
	Framework        string          `json:"framework,omitempty"`
	BuildConfig      json.RawMessage `gorm:"type:jsonb" json:"build_config,omitempty"`
	DeploymentConfig json.RawMessage `gorm:"type:jsonb" json:"deployment_config,omitempty"`
	EnvironmentVars  json.RawMessage `gorm:"type:jsonb" json:"environment_vars,omitempty"`
	Secrets          json.RawMessage `gorm:"type:jsonb" json:"secrets,omitempty"`
	CreatedBy        string          `json:"created_by"`
	UpdatedBy        string          `json:"updated_by"`
	LastActivityAt   *time.Time      `json:"last_activity_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Workflows      []Workflow      `gorm:"foreignKey:ProjectID" json:"workflows,omitempty"`
	Executions     []Execution     `gorm:"foreignKey:ProjectID" json:"executions,omitempty"`
	Members        []ProjectMember `gorm:"foreignKey:ProjectID" json:"members,omitempty"`
	Environments   []Environment   `gorm:"foreignKey:ProjectID" json:"environments,omitempty"`
	Resources      []Resource      `gorm:"foreignKey:ProjectID" json:"resources,omitempty"`
	Integrations   []Integration   `gorm:"foreignKey:ProjectID" json:"integrations,omitempty"`
}

// ProjectMember represents a member of a project
type ProjectMember struct {
	ID        string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID string         `gorm:"type:uuid;not null;index" json:"project_id"`
	UserID    string         `gorm:"not null;index" json:"user_id"`
	Role      string         `gorm:"not null;default:'viewer'" json:"role"`
	Permissions []string     `gorm:"type:text[]" json:"permissions,omitempty"`
	AddedBy   string         `json:"added_by"`
	AddedAt   time.Time      `json:"added_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Project *Project `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}

// Environment represents a project environment
type Environment struct {
	ID              string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID       string          `gorm:"type:uuid;not null;index" json:"project_id"`
	Name            string          `gorm:"not null" json:"name"`
	Type            string          `gorm:"not null;default:'development'" json:"type"`
	Description     string          `json:"description"`
	Config          json.RawMessage `gorm:"type:jsonb" json:"config,omitempty"`
	Variables       json.RawMessage `gorm:"type:jsonb" json:"variables,omitempty"`
	Secrets         json.RawMessage `gorm:"type:jsonb" json:"secrets,omitempty"`
	Resources       json.RawMessage `gorm:"type:jsonb" json:"resources,omitempty"`
	Status          string          `gorm:"default:'active'" json:"status"`
	DeploymentURL   string          `json:"deployment_url,omitempty"`
	HealthCheckURL  string          `json:"health_check_url,omitempty"`
	LastDeployedAt  *time.Time      `json:"last_deployed_at,omitempty"`
	LastDeployedBy  string          `json:"last_deployed_by,omitempty"`
	CreatedBy       string          `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	DeletedAt       gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Project *Project `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}

// Resource represents a project resource
type Resource struct {
	ID            string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID     string          `gorm:"type:uuid;not null;index" json:"project_id"`
	Name          string          `gorm:"not null" json:"name"`
	Type          string          `gorm:"not null" json:"type"`
	Provider      string          `json:"provider"`
	Region        string          `json:"region,omitempty"`
	Status        string          `gorm:"default:'provisioning'" json:"status"`
	Config        json.RawMessage `gorm:"type:jsonb" json:"config,omitempty"`
	Metadata      json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	Cost          json.RawMessage `gorm:"type:jsonb" json:"cost,omitempty"`
	Usage         json.RawMessage `gorm:"type:jsonb" json:"usage,omitempty"`
	Limits        json.RawMessage `gorm:"type:jsonb" json:"limits,omitempty"`
	ProvisionedAt *time.Time      `json:"provisioned_at,omitempty"`
	TerminatedAt  *time.Time      `json:"terminated_at,omitempty"`
	CreatedBy     string          `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Project *Project `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}

// Integration represents a third-party integration
type Integration struct {
	ID            string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID     string          `gorm:"type:uuid;not null;index" json:"project_id"`
	Name          string          `gorm:"not null" json:"name"`
	Type          string          `gorm:"not null" json:"type"`
	Provider      string          `gorm:"not null" json:"provider"`
	Status        string          `gorm:"default:'active'" json:"status"`
	Config        json.RawMessage `gorm:"type:jsonb" json:"config,omitempty"`
	Credentials   json.RawMessage `gorm:"type:jsonb" json:"credentials,omitempty"`
	Webhooks      json.RawMessage `gorm:"type:jsonb" json:"webhooks,omitempty"`
	LastSyncedAt  *time.Time      `json:"last_synced_at,omitempty"`
	LastErrorAt   *time.Time      `json:"last_error_at,omitempty"`
	LastError     string          `json:"last_error,omitempty"`
	CreatedBy     string          `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Project *Project `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}

// ProjectStats represents project statistics
type ProjectStats struct {
	ProjectID         string    `json:"project_id"`
	TotalWorkflows    int64     `json:"total_workflows"`
	ActiveWorkflows   int64     `json:"active_workflows"`
	TotalExecutions   int64     `json:"total_executions"`
	SuccessfulExecutions int64  `json:"successful_executions"`
	FailedExecutions  int64     `json:"failed_executions"`
	TotalMembers      int64     `json:"total_members"`
	TotalResources    int64     `json:"total_resources"`
	TotalIntegrations int64     `json:"total_integrations"`
	StorageUsed       int64     `json:"storage_used"`
	ComputeHours      float64   `json:"compute_hours"`
	EstimatedCost     float64   `json:"estimated_cost"`
	LastActivityAt    time.Time `json:"last_activity_at"`
	CalculatedAt      time.Time `json:"calculated_at"`
}

// TableName specifies the table name for Project
func (Project) TableName() string {
	return "projects"
}

// TableName specifies the table name for ProjectMember
func (ProjectMember) TableName() string {
	return "project_members"
}

// TableName specifies the table name for Environment
func (Environment) TableName() string {
	return "environments"
}

// TableName specifies the table name for Resource
func (Resource) TableName() string {
	return "resources"
}

// TableName specifies the table name for Integration
func (Integration) TableName() string {
	return "integrations"
}

// BeforeCreate hook to set default values
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.Status == "" {
		p.Status = ProjectStatusInitializing
	}
	if p.Type == "" {
		p.Type = ProjectTypeStandard
	}
	if p.Version == "" {
		p.Version = "1.0.0"
	}
	now := time.Now()
	p.LastActivityAt = &now
	return nil
}

// UpdateLastActivity updates the last activity timestamp
func (p *Project) UpdateLastActivity(tx *gorm.DB) error {
	now := time.Now()
	p.LastActivityAt = &now
	return tx.Model(p).Update("last_activity_at", now).Error
}

// IsActive returns true if the project is active
func (p *Project) IsActive() bool {
	return p.Status == ProjectStatusActive
}

// HasFeature checks if the project has a specific feature enabled
func (p *Project) HasFeature(feature string) bool {
	for _, f := range p.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// GetMemberRole returns the role of a member in the project
func (p *Project) GetMemberRole(userID string) string {
	for _, member := range p.Members {
		if member.UserID == userID && member.DeletedAt.Time.IsZero() {
			return member.Role
		}
	}
	return ""
}

// HasPermission checks if a user has a specific permission in the project
func (p *Project) HasPermission(userID string, permission string) bool {
	role := p.GetMemberRole(userID)
	if role == "" {
		return false
	}
	
	// Owner has all permissions
	if role == "owner" {
		return true
	}
	
	// Check specific permissions for the role
	for _, member := range p.Members {
		if member.UserID == userID && member.DeletedAt.Time.IsZero() {
			for _, perm := range member.Permissions {
				if perm == permission || perm == "*" {
					return true
				}
			}
		}
	}
	
	return false
}