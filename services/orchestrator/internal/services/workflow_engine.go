package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"orchestrator/internal/models"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WorkflowEngine manages workflow orchestration
type WorkflowEngine struct {
	db             *gorm.DB
	redis          *redis.Client
	temporalClient client.Client
	logger         *zap.Logger
	intentClient   *IntentClient
	agentClient    *AgentClient
	config         *WorkflowConfig
}

// WorkflowConfig holds workflow engine configuration
type WorkflowConfig struct {
	TaskQueue               string
	MaxConcurrentWorkflows  int
	MaxConcurrentActivities int
	WorkflowTimeout         time.Duration
	ActivityTimeout         time.Duration
	RetryPolicy             *temporal.RetryPolicy
	EnableMetrics           bool
	EnableTracing           bool
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(
	db *gorm.DB,
	redis *redis.Client,
	temporalClient client.Client,
	logger *zap.Logger,
	intentClient *IntentClient,
	agentClient *AgentClient,
	config *WorkflowConfig,
) *WorkflowEngine {
	return &WorkflowEngine{
		db:             db,
		redis:          redis,
		temporalClient: temporalClient,
		logger:         logger,
		intentClient:   intentClient,
		agentClient:    agentClient,
		config:         config,
	}
}

// StartWorkflow starts a new workflow execution
func (e *WorkflowEngine) StartWorkflow(ctx context.Context, req *StartWorkflowRequest) (*StartWorkflowResponse, error) {
	// Create workflow record in database
	workflow := &models.Workflow{
		Name:           req.Name,
		Description:    req.Description,
		Type:           models.WorkflowType(req.Type),
		Priority:       models.WorkflowPriority(req.Priority),
		ProjectID:      req.ProjectID,
		Status:         models.WorkflowStatusPending,
		Input:          req.Input,
		Config:         req.Config,
		MaxRetries:     req.MaxRetries,
		TimeoutSeconds: req.TimeoutSeconds,
		CreatedBy:      req.UserID,
		UpdatedBy:      req.UserID,
	}

	if err := e.db.Create(workflow).Error; err != nil {
		return nil, fmt.Errorf("failed to create workflow record: %w", err)
	}

	// Prepare workflow options
	workflowOptions := client.StartWorkflowOptions{
		ID:                       workflow.ID,
		TaskQueue:                e.config.TaskQueue,
		WorkflowExecutionTimeout: time.Duration(workflow.TimeoutSeconds) * time.Second,
		WorkflowTaskTimeout:      10 * time.Minute,
		RetryPolicy:              e.config.RetryPolicy,
	}

	// Start Temporal workflow
	workflowRun, err := e.temporalClient.ExecuteWorkflow(
		ctx,
		workflowOptions,
		e.getWorkflowFunction(workflow.Type),
		workflow,
	)
	if err != nil {
		// Update workflow status to failed
		workflow.Status = models.WorkflowStatusFailed
		workflow.Error = err.Error()
		e.db.Save(workflow)
		return nil, fmt.Errorf("failed to start temporal workflow: %w", err)
	}

	// Update workflow with Temporal IDs
	workflow.TemporalID = workflowRun.GetID()
	workflow.TemporalRunID = workflowRun.GetRunID()
	workflow.Status = models.WorkflowStatusRunning
	now := time.Now()
	workflow.StartedAt = &now

	if err := e.db.Save(workflow).Error; err != nil {
		e.logger.Error("failed to update workflow with temporal IDs", zap.Error(err))
	}

	// Store workflow state in Redis for quick access
	e.cacheWorkflowState(ctx, workflow)

	// Emit workflow started event
	e.emitWorkflowEvent(ctx, workflow, "started", nil)

	return &StartWorkflowResponse{
		WorkflowID:    workflow.ID,
		TemporalID:    workflow.TemporalID,
		TemporalRunID: workflow.TemporalRunID,
		Status:        string(workflow.Status),
	}, nil
}

// GetWorkflow retrieves workflow details
func (e *WorkflowEngine) GetWorkflow(ctx context.Context, workflowID string) (*models.Workflow, error) {
	// Try to get from cache first
	cached, err := e.getCachedWorkflow(ctx, workflowID)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Get from database
	var workflow models.Workflow
	if err := e.db.Preload("Steps").Preload("Executions").First(&workflow, "id = ?", workflowID).Error; err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	// Update cache
	e.cacheWorkflowState(ctx, &workflow)

	return &workflow, nil
}

// CancelWorkflow cancels a running workflow
func (e *WorkflowEngine) CancelWorkflow(ctx context.Context, workflowID string, reason string) error {
	workflow, err := e.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	if workflow.IsTerminal() {
		return fmt.Errorf("workflow is already in terminal state: %s", workflow.Status)
	}

	// Cancel Temporal workflow
	if err := e.temporalClient.CancelWorkflow(ctx, workflow.TemporalID, workflow.TemporalRunID); err != nil {
		return fmt.Errorf("failed to cancel temporal workflow: %w", err)
	}

	// Update workflow status
	workflow.Status = models.WorkflowStatusCancelled
	workflow.Error = reason
	now := time.Now()
	workflow.CompletedAt = &now

	if err := e.db.Save(workflow).Error; err != nil {
		return fmt.Errorf("failed to update workflow status: %w", err)
	}

	// Update cache
	e.cacheWorkflowState(ctx, workflow)

	// Emit workflow cancelled event
	e.emitWorkflowEvent(ctx, workflow, "cancelled", map[string]interface{}{"reason": reason})

	return nil
}

// ListWorkflows lists workflows with filters
func (e *WorkflowEngine) ListWorkflows(ctx context.Context, filters *WorkflowFilters) ([]*models.Workflow, int64, error) {
	query := e.db.Model(&models.Workflow{})

	// Apply filters
	if filters.ProjectID != "" {
		query = query.Where("project_id = ?", filters.ProjectID)
	}
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Type != "" {
		query = query.Where("type = ?", filters.Type)
	}
	if filters.CreatedBy != "" {
		query = query.Where("created_by = ?", filters.CreatedBy)
	}
	if !filters.StartDate.IsZero() {
		query = query.Where("created_at >= ?", filters.StartDate)
	}
	if !filters.EndDate.IsZero() {
		query = query.Where("created_at <= ?", filters.EndDate)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count workflows: %w", err)
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

	// Fetch workflows
	var workflows []*models.Workflow
	if err := query.Preload("Project").Find(&workflows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list workflows: %w", err)
	}

	return workflows, total, nil
}

// GetWorkflowMetrics retrieves workflow metrics
func (e *WorkflowEngine) GetWorkflowMetrics(ctx context.Context, workflowID string) (*WorkflowMetrics, error) {
	workflow, err := e.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	metrics := &WorkflowMetrics{
		WorkflowID:     workflowID,
		Status:         string(workflow.Status),
		StartedAt:      workflow.StartedAt,
		CompletedAt:    workflow.CompletedAt,
		Duration:       workflow.Duration,
		RetryCount:     workflow.RetryCount,
		StepMetrics:    make([]*StepMetric, 0),
		ResourceUsage:  make(map[string]interface{}),
	}

	// Get step metrics
	for _, step := range workflow.Steps {
		stepMetric := &StepMetric{
			StepID:      step.ID,
			Name:        step.Name,
			Status:      string(step.Status),
			StartedAt:   step.StartedAt,
			CompletedAt: step.CompletedAt,
			Duration:    step.Duration,
			RetryCount:  step.RetryCount,
		}
		metrics.StepMetrics = append(metrics.StepMetrics, stepMetric)
	}

	// Get execution metrics
	var executions []models.Execution
	if err := e.db.Where("workflow_id = ?", workflowID).Find(&executions).Error; err == nil {
		var totalCPU, totalMemory float64
		for _, exec := range executions {
			if exec.ResourceUsage != nil {
				var usage models.ResourceUsage
				if err := json.Unmarshal(exec.ResourceUsage, &usage); err == nil {
					totalCPU += usage.CPUUsage
					totalMemory += float64(usage.MemoryUsage)
				}
			}
		}
		metrics.ResourceUsage["total_cpu_usage"] = totalCPU
		metrics.ResourceUsage["total_memory_usage"] = totalMemory
		metrics.ResourceUsage["execution_count"] = len(executions)
	}

	return metrics, nil
}

// getWorkflowFunction returns the appropriate workflow function name based on type
func (e *WorkflowEngine) getWorkflowFunction(workflowType models.WorkflowType) interface{} {
	// Return workflow function names as strings
	// The actual workflow functions will be registered separately with the Temporal worker
	switch workflowType {
	case models.WorkflowTypeIntent:
		return "IntentProcessingWorkflow"
	case models.WorkflowTypeExecution:
		return "CodeExecutionWorkflow"
	case models.WorkflowTypeAnalysis:
		return "CodeAnalysisWorkflow"
	case models.WorkflowTypeReview:
		return "CodeReviewWorkflow"
	case models.WorkflowTypeDeployment:
		return "DeploymentWorkflow"
	default:
		return "CustomWorkflow"
	}
}

// cacheWorkflowState caches workflow state in Redis
func (e *WorkflowEngine) cacheWorkflowState(ctx context.Context, workflow *models.Workflow) {
	key := fmt.Sprintf("workflow:%s", workflow.ID)
	data, err := json.Marshal(workflow)
	if err != nil {
		e.logger.Error("failed to marshal workflow for cache", zap.Error(err))
		return
	}

	if err := e.redis.Set(ctx, key, data, 5*time.Minute).Err(); err != nil {
		e.logger.Error("failed to cache workflow state", zap.Error(err))
	}
}

// getCachedWorkflow retrieves workflow from cache
func (e *WorkflowEngine) getCachedWorkflow(ctx context.Context, workflowID string) (*models.Workflow, error) {
	key := fmt.Sprintf("workflow:%s", workflowID)
	data, err := e.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var workflow models.Workflow
	if err := json.Unmarshal([]byte(data), &workflow); err != nil {
		return nil, err
	}

	return &workflow, nil
}

// emitWorkflowEvent emits a workflow event
func (e *WorkflowEngine) emitWorkflowEvent(ctx context.Context, workflow *models.Workflow, eventType string, data map[string]interface{}) {
	event := map[string]interface{}{
		"workflow_id": workflow.ID,
		"project_id":  workflow.ProjectID,
		"type":        workflow.Type,
		"status":      workflow.Status,
		"event_type":  eventType,
		"timestamp":   time.Now(),
		"data":        data,
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		e.logger.Error("failed to marshal workflow event", zap.Error(err))
		return
	}

	// Publish event to Redis
	channel := fmt.Sprintf("workflow:events:%s", workflow.ProjectID)
	if err := e.redis.Publish(ctx, channel, eventData).Err(); err != nil {
		e.logger.Error("failed to publish workflow event", zap.Error(err))
	}
}

// StartWorkflowRequest represents a request to start a workflow
type StartWorkflowRequest struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Type           string          `json:"type"`
	Priority       string          `json:"priority"`
	ProjectID      string          `json:"project_id"`
	UserID         string          `json:"user_id"`
	Input          json.RawMessage `json:"input"`
	Config         json.RawMessage `json:"config"`
	MaxRetries     int             `json:"max_retries"`
	TimeoutSeconds int             `json:"timeout_seconds"`
}

// StartWorkflowResponse represents a response from starting a workflow
type StartWorkflowResponse struct {
	WorkflowID    string `json:"workflow_id"`
	TemporalID    string `json:"temporal_id"`
	TemporalRunID string `json:"temporal_run_id"`
	Status        string `json:"status"`
}

// WorkflowFilters represents filters for listing workflows
type WorkflowFilters struct {
	ProjectID string
	Status    string
	Type      string
	CreatedBy string
	StartDate time.Time
	EndDate   time.Time
	SortBy    string
	SortDesc  bool
	Limit     int
	Offset    int
}

// WorkflowMetrics represents workflow metrics
type WorkflowMetrics struct {
	WorkflowID    string                 `json:"workflow_id"`
	Status        string                 `json:"status"`
	StartedAt     *time.Time             `json:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at"`
	Duration      int64                  `json:"duration"`
	RetryCount    int                    `json:"retry_count"`
	StepMetrics   []*StepMetric          `json:"step_metrics"`
	ResourceUsage map[string]interface{} `json:"resource_usage"`
}

// StepMetric represents metrics for a workflow step
type StepMetric struct {
	StepID      string     `json:"step_id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Duration    int64      `json:"duration"`
	RetryCount  int        `json:"retry_count"`
}