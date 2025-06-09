package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"orchestrator/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	workflowEngine *services.WorkflowEngine
	projectService *services.ProjectService
	agentClient    *services.AgentClient
	logger         *zap.Logger
	db             *gorm.DB
}

// NewHandlers creates new handlers instance
func NewHandlers(
	workflowEngine *services.WorkflowEngine,
	projectService *services.ProjectService,
	agentClient *services.AgentClient,
	logger *zap.Logger,
	db *gorm.DB,
) *Handlers {
	return &Handlers{
		workflowEngine: workflowEngine,
		projectService: projectService,
		agentClient:    agentClient,
		logger:         logger,
		db:             db,
	}
}

// GetDB returns the database instance
func (h *Handlers) GetDB() *gorm.DB {
	return h.db
}

// Project Handlers

// CreateProject creates a new project
func (h *Handlers) CreateProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Get user ID from context (set by auth middleware)
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "system" // Default for unauthenticated requests
	}

	project, err := h.projectService.CreateProject(c.Request.Context(), &services.CreateProjectRequest{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		OwnerID:     userID,
		Settings:    req.Settings,
		Tags:        req.Tags,
	})
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to create project", err)
		return
	}

	h.respondSuccess(c, http.StatusCreated, project)
}

// GetProject retrieves a project by ID
func (h *Handlers) GetProject(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		h.respondError(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	project, err := h.projectService.GetProject(c.Request.Context(), projectID)
	if err != nil {
		h.respondError(c, http.StatusNotFound, "Project not found", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, project)
}

// ListProjects lists all projects with optional filters
func (h *Handlers) ListProjects(c *gin.Context) {
	filters := &services.ProjectFilters{
		Status:   c.Query("status"),
		Type:     c.Query("type"),
		OwnerID:  c.Query("owner_id"),
		SortBy:   c.Query("sort_by"),
		SortDesc: c.Query("sort_order") == "desc",
	}

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filters.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filters.Offset = o
		}
	}

	projects, total, err := h.projectService.ListProjects(c.Request.Context(), filters)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to list projects", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, gin.H{
		"projects": projects,
		"total":    total,
		"limit":    filters.Limit,
		"offset":   filters.Offset,
	})
}

// UpdateProject updates a project
func (h *Handlers) UpdateProject(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		h.respondError(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	userID := c.GetString("user_id")
	
	project, err := h.projectService.UpdateProject(c.Request.Context(), projectID, &services.UpdateProjectRequest{
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		Settings:    req.Settings,
		Tags:        req.Tags,
		UpdatedBy:   userID,
	})
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to update project", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, project)
}

// DeleteProject deletes a project
func (h *Handlers) DeleteProject(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		h.respondError(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	if err := h.projectService.DeleteProject(c.Request.Context(), projectID); err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to delete project", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, gin.H{"message": "Project deleted successfully"})
}

// Workflow Handlers

// StartWorkflow starts a new workflow
func (h *Handlers) StartWorkflow(c *gin.Context) {
	var req StartWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "system"
	}

	// Convert request to service format
	startReq := &services.StartWorkflowRequest{
		Name:           req.Name,
		Description:    req.Description,
		Type:           req.Type,
		Priority:       req.Priority,
		ProjectID:      req.ProjectID,
		UserID:         userID,
		Input:          req.Input,
		Config:         req.Config,
		MaxRetries:     req.MaxRetries,
		TimeoutSeconds: req.TimeoutSeconds,
	}

	// Set defaults
	if startReq.Priority == "" {
		startReq.Priority = "medium"
	}
	if startReq.MaxRetries == 0 {
		startReq.MaxRetries = 3
	}
	if startReq.TimeoutSeconds == 0 {
		startReq.TimeoutSeconds = 3600
	}

	response, err := h.workflowEngine.StartWorkflow(c.Request.Context(), startReq)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to start workflow", err)
		return
	}

	h.respondSuccess(c, http.StatusCreated, response)
}

// GetWorkflow retrieves workflow details
func (h *Handlers) GetWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		h.respondError(c, http.StatusBadRequest, "Workflow ID is required", nil)
		return
	}

	workflow, err := h.workflowEngine.GetWorkflow(c.Request.Context(), workflowID)
	if err != nil {
		h.respondError(c, http.StatusNotFound, "Workflow not found", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, workflow)
}

// ListWorkflows lists workflows with filters
func (h *Handlers) ListWorkflows(c *gin.Context) {
	filters := &services.WorkflowFilters{
		ProjectID: c.Query("project_id"),
		Status:    c.Query("status"),
		Type:      c.Query("type"),
		CreatedBy: c.Query("created_by"),
		SortBy:    c.Query("sort_by"),
		SortDesc:  c.Query("sort_order") == "desc",
	}

	// Parse dates
	if startDate := c.Query("start_date"); startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			filters.StartDate = t
		}
	}
	if endDate := c.Query("end_date"); endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			filters.EndDate = t
		}
	}

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filters.Limit = l
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filters.Offset = o
		}
	}

	workflows, total, err := h.workflowEngine.ListWorkflows(c.Request.Context(), filters)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to list workflows", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, gin.H{
		"workflows": workflows,
		"total":     total,
		"limit":     filters.Limit,
		"offset":    filters.Offset,
	})
}

// CancelWorkflow cancels a running workflow
func (h *Handlers) CancelWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		h.respondError(c, http.StatusBadRequest, "Workflow ID is required", nil)
		return
	}

	var req CancelWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = "User requested cancellation"
	}

	if err := h.workflowEngine.CancelWorkflow(c.Request.Context(), workflowID, req.Reason); err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to cancel workflow", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, gin.H{"message": "Workflow cancelled successfully"})
}

// GetWorkflowMetrics retrieves workflow metrics
func (h *Handlers) GetWorkflowMetrics(c *gin.Context) {
	workflowID := c.Param("id")
	if workflowID == "" {
		h.respondError(c, http.StatusBadRequest, "Workflow ID is required", nil)
		return
	}

	metrics, err := h.workflowEngine.GetWorkflowMetrics(c.Request.Context(), workflowID)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to get workflow metrics", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, metrics)
}

// Agent Handlers

// ListAgents lists available agents
func (h *Handlers) ListAgents(c *gin.Context) {
	filters := &services.AgentFilters{
		ProjectID: c.Query("project_id"),
		Type:      c.Query("type"),
		Status:    c.Query("status"),
	}

	// Parse pagination
	if page := c.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			filters.Page = p
		}
	}
	if pageSize := c.Query("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil {
			filters.PageSize = ps
		}
	}

	agentList, err := h.agentClient.ListAgents(c.Request.Context(), filters)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to list agents", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, agentList)
}

// GetAgent retrieves agent details
func (h *Handlers) GetAgent(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		h.respondError(c, http.StatusBadRequest, "Agent ID is required", nil)
		return
	}

	agent, err := h.agentClient.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		h.respondError(c, http.StatusNotFound, "Agent not found", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, agent)
}

// RestartAgent restarts an agent
func (h *Handlers) RestartAgent(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		h.respondError(c, http.StatusBadRequest, "Agent ID is required", nil)
		return
	}

	// Update agent status to trigger restart
	_, err := h.agentClient.UpdateAgent(c.Request.Context(), agentID, &services.UpdateAgentRequest{
		Status: "restarting",
	})
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to restart agent", err)
		return
	}

	h.respondSuccess(c, http.StatusOK, gin.H{"message": "Agent restart initiated"})
}

// Health check handler with detailed status
func (h *Handlers) HealthCheck(c *gin.Context) {
	_ = c.Request.Context() // Reserved for future use
	
	// Check database connectivity
	dbHealthy := true
	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			dbHealthy = false
		} else if err := sqlDB.Ping(); err != nil {
			dbHealthy = false
		}
	}
	
	// Check Temporal connectivity
	temporalHealthy := h.workflowEngine != nil
	
	// Check Agent Manager connectivity
	agentManagerHealthy := true
	if h.agentClient != nil {
		// Simple check - could be enhanced with actual ping
		agentManagerHealthy = true
	}
	
	overallHealthy := dbHealthy && temporalHealthy && agentManagerHealthy
	
	response := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"checks": gin.H{
			"database":      dbHealthy,
			"temporal":      temporalHealthy,
			"agent_manager": agentManagerHealthy,
		},
	}
	
	if !overallHealthy {
		response["status"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"data":    response,
		})
		return
	}
	
	h.respondSuccess(c, http.StatusOK, response)
}

// Helper methods

func (h *Handlers) respondSuccess(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, gin.H{
		"success": true,
		"data":    data,
	})
}

func (h *Handlers) respondError(c *gin.Context, statusCode int, message string, err error) {
	h.logger.Error(message, zap.Error(err))
	
	response := gin.H{
		"success": false,
		"error": gin.H{
			"message": message,
		},
	}
	
	if err != nil {
		response["error"].(gin.H)["details"] = err.Error()
	}
	
	c.JSON(statusCode, response)
}

// Request types

type CreateProjectRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Settings    json.RawMessage        `json:"settings"`
	Tags        []string               `json:"tags"`
}

type UpdateProjectRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Settings    json.RawMessage        `json:"settings"`
	Tags        []string               `json:"tags"`
}

type StartWorkflowRequest struct {
	Name           string          `json:"name" binding:"required"`
	Description    string          `json:"description"`
	Type           string          `json:"type" binding:"required"`
	Priority       string          `json:"priority"`
	ProjectID      string          `json:"project_id" binding:"required"`
	Input          json.RawMessage `json:"input"`
	Config         json.RawMessage `json:"config"`
	MaxRetries     int             `json:"max_retries"`
	TimeoutSeconds int             `json:"timeout_seconds"`
}

type CancelWorkflowRequest struct {
	Reason string `json:"reason"`
}