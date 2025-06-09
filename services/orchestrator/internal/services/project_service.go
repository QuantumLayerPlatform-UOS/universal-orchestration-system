package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"orchestrator/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ProjectService handles project management
type ProjectService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewProjectService creates a new project service
func NewProjectService(db *gorm.DB, logger *zap.Logger) *ProjectService {
	return &ProjectService{
		db:     db,
		logger: logger,
	}
}

// CreateProject creates a new project
func (s *ProjectService) CreateProject(ctx context.Context, req *CreateProjectRequest) (*models.Project, error) {
	project := &models.Project{
		Name:           req.Name,
		Description:    req.Description,
		Type:           models.ProjectType(req.Type),
		OwnerID:        req.OwnerID,
		Settings:       req.Settings,
		Tags:           req.Tags,
		Status:         models.ProjectStatusActive,
		CreatedBy:      req.OwnerID,
		UpdatedBy:      req.OwnerID,
	}
	
	// Only set OrganizationID if it's not empty
	if req.OrganizationID != "" {
		project.OrganizationID = &req.OrganizationID
	}

	// Set defaults
	if project.Type == "" {
		project.Type = models.ProjectTypeStandard
	}

	// Create project in database
	if err := s.db.WithContext(ctx).Create(project).Error; err != nil {
		s.logger.Error("failed to create project", zap.Error(err))
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Create default environment
	defaultEnv := &models.Environment{
		ProjectID:   project.ID,
		Name:        "development",
		Type:        "development",
		Description: "Default development environment",
		Status:      "active",
		CreatedBy:   req.OwnerID,
	}
	
	if err := s.db.WithContext(ctx).Create(defaultEnv).Error; err != nil {
		s.logger.Error("failed to create default environment", zap.Error(err))
	}

	// Add owner as project member
	member := &models.ProjectMember{
		ProjectID:   project.ID,
		UserID:      req.OwnerID,
		Role:        "owner",
		Permissions: []string{"*"},
		AddedBy:     req.OwnerID,
		AddedAt:     time.Now(),
	}
	
	if err := s.db.WithContext(ctx).Create(member).Error; err != nil {
		s.logger.Error("failed to add project owner", zap.Error(err))
	}

	s.logger.Info("project created", 
		zap.String("project_id", project.ID),
		zap.String("name", project.Name),
		zap.String("owner", project.OwnerID),
	)

	return project, nil
}

// GetProject retrieves a project by ID
func (s *ProjectService) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	var project models.Project
	
	err := s.db.WithContext(ctx).
		Preload("Members").
		Preload("Environments").
		Preload("Resources").
		Preload("Integrations").
		First(&project, "id = ?", projectID).Error
		
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Update last activity
	go s.updateLastActivity(projectID)

	return &project, nil
}

// ListProjects lists projects with filters
func (s *ProjectService) ListProjects(ctx context.Context, filters *ProjectFilters) ([]*models.Project, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.Project{})

	// Apply filters
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Type != "" {
		query = query.Where("type = ?", filters.Type)
	}
	if filters.OwnerID != "" {
		query = query.Where("owner_id = ?", filters.OwnerID)
	}
	if filters.OrganizationID != "" {
		query = query.Where("organization_id = ?", filters.OrganizationID)
	}
	if len(filters.Tags) > 0 {
		query = query.Where("tags && ?", filters.Tags)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	// Apply sorting
	if filters.SortBy != "" {
		order := "ASC"
		if filters.SortDesc {
			order = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", filters.SortBy, order))
	} else {
		query = query.Order("created_at DESC")
	}

	// Apply pagination
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	// Fetch projects
	var projects []*models.Project
	if err := query.Find(&projects).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list projects: %w", err)
	}

	return projects, total, nil
}

// UpdateProject updates a project
func (s *ProjectService) UpdateProject(ctx context.Context, projectID string, req *UpdateProjectRequest) (*models.Project, error) {
	var project models.Project
	
	// Get existing project
	if err := s.db.WithContext(ctx).First(&project, "id = ?", projectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Update fields
	updates := map[string]interface{}{
		"updated_by": req.UpdatedBy,
		"updated_at": time.Now(),
	}

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Settings != nil {
		updates["settings"] = req.Settings
	}
	if req.Tags != nil {
		updates["tags"] = req.Tags
	}

	// Update project
	if err := s.db.WithContext(ctx).Model(&project).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	// Reload project with associations
	if err := s.db.WithContext(ctx).
		Preload("Members").
		Preload("Environments").
		First(&project, "id = ?", projectID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload project: %w", err)
	}

	s.logger.Info("project updated",
		zap.String("project_id", projectID),
		zap.String("updated_by", req.UpdatedBy),
	)

	return &project, nil
}

// DeleteProject deletes a project (soft delete)
func (s *ProjectService) DeleteProject(ctx context.Context, projectID string) error {
	// Check if project exists
	var project models.Project
	if err := s.db.WithContext(ctx).First(&project, "id = ?", projectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("project not found")
		}
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Soft delete project
	if err := s.db.WithContext(ctx).Delete(&project).Error; err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	s.logger.Info("project deleted", zap.String("project_id", projectID))
	return nil
}

// GetProjectStats retrieves project statistics
func (s *ProjectService) GetProjectStats(ctx context.Context, projectID string) (*models.ProjectStats, error) {
	stats := &models.ProjectStats{
		ProjectID:    projectID,
		CalculatedAt: time.Now(),
	}

	// Count workflows
	s.db.WithContext(ctx).Model(&models.Workflow{}).Where("project_id = ?", projectID).Count(&stats.TotalWorkflows)
	s.db.WithContext(ctx).Model(&models.Workflow{}).Where("project_id = ? AND status IN ?", projectID, 
		[]string{"running", "pending"}).Count(&stats.ActiveWorkflows)

	// Count executions
	s.db.WithContext(ctx).Model(&models.Execution{}).Where("project_id = ?", projectID).Count(&stats.TotalExecutions)
	s.db.WithContext(ctx).Model(&models.Execution{}).Where("project_id = ? AND status = ?", projectID, 
		"succeeded").Count(&stats.SuccessfulExecutions)
	s.db.WithContext(ctx).Model(&models.Execution{}).Where("project_id = ? AND status = ?", projectID, 
		"failed").Count(&stats.FailedExecutions)

	// Count members
	s.db.WithContext(ctx).Model(&models.ProjectMember{}).Where("project_id = ?", projectID).Count(&stats.TotalMembers)

	// Count resources
	s.db.WithContext(ctx).Model(&models.Resource{}).Where("project_id = ?", projectID).Count(&stats.TotalResources)

	// Count integrations
	s.db.WithContext(ctx).Model(&models.Integration{}).Where("project_id = ?", projectID).Count(&stats.TotalIntegrations)

	// Get last activity
	var lastActivity time.Time
	s.db.WithContext(ctx).Model(&models.Project{}).Where("id = ?", projectID).
		Pluck("last_activity_at", &lastActivity)
	stats.LastActivityAt = lastActivity

	return stats, nil
}

// AddProjectMember adds a member to a project
func (s *ProjectService) AddProjectMember(ctx context.Context, projectID string, req *AddProjectMemberRequest) (*models.ProjectMember, error) {
	// Check if project exists
	var project models.Project
	if err := s.db.WithContext(ctx).First(&project, "id = ?", projectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Check if member already exists
	var existingMember models.ProjectMember
	err := s.db.WithContext(ctx).Where("project_id = ? AND user_id = ?", projectID, req.UserID).
		First(&existingMember).Error
	
	if err == nil {
		return nil, fmt.Errorf("user is already a member of this project")
	}

	// Create new member
	member := &models.ProjectMember{
		ProjectID:   projectID,
		UserID:      req.UserID,
		Role:        req.Role,
		Permissions: req.Permissions,
		AddedBy:     req.AddedBy,
		AddedAt:     time.Now(),
	}

	if member.Role == "" {
		member.Role = "viewer"
	}

	if err := s.db.WithContext(ctx).Create(member).Error; err != nil {
		return nil, fmt.Errorf("failed to add project member: %w", err)
	}

	s.logger.Info("project member added",
		zap.String("project_id", projectID),
		zap.String("user_id", req.UserID),
		zap.String("role", member.Role),
	)

	return member, nil
}

// RemoveProjectMember removes a member from a project
func (s *ProjectService) RemoveProjectMember(ctx context.Context, projectID, userID string) error {
	result := s.db.WithContext(ctx).Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&models.ProjectMember{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to remove project member: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("member not found")
	}

	s.logger.Info("project member removed",
		zap.String("project_id", projectID),
		zap.String("user_id", userID),
	)

	return nil
}

// updateLastActivity updates the project's last activity timestamp
func (s *ProjectService) updateLastActivity(projectID string) {
	now := time.Now()
	s.db.Model(&models.Project{}).Where("id = ?", projectID).
		Update("last_activity_at", now)
}

// Request types

type CreateProjectRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Type           string          `json:"type"`
	OwnerID        string          `json:"owner_id"`
	OrganizationID string          `json:"organization_id"`
	Settings       json.RawMessage `json:"settings"`
	Tags           []string        `json:"tags"`
}

type UpdateProjectRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Settings    json.RawMessage `json:"settings"`
	Tags        []string        `json:"tags"`
	UpdatedBy   string          `json:"updated_by"`
}

type ProjectFilters struct {
	Status         string
	Type           string
	OwnerID        string
	OrganizationID string
	Tags           []string
	SortBy         string
	SortDesc       bool
	Limit          int
	Offset         int
}

type AddProjectMemberRequest struct {
	UserID      string   `json:"user_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	AddedBy     string   `json:"added_by"`
}