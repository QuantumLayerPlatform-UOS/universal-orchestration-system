package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"orchestrator/internal/models"
)

// WorkflowMonitor monitors workflow status and updates the database
type WorkflowMonitor struct {
	db             *gorm.DB
	temporalClient client.Client
	logger         *zap.Logger
	redis          *redis.Client
	interval       time.Duration
	stopChan       chan struct{}
	wg             sync.WaitGroup
}

// NewWorkflowMonitor creates a new workflow monitor
func NewWorkflowMonitor(db *gorm.DB, temporalClient client.Client, logger *zap.Logger, redisClient *redis.Client, interval time.Duration) *WorkflowMonitor {
	return &WorkflowMonitor{
		db:             db,
		temporalClient: temporalClient,
		logger:         logger,
		redis:          redisClient,
		interval:       interval,
		stopChan:       make(chan struct{}),
	}
}

// Start starts the workflow monitor
func (m *WorkflowMonitor) Start() {
	m.wg.Add(1)
	go m.monitorWorkflows()
	m.logger.Info("Workflow monitor started", zap.Duration("interval", m.interval))
}

// Stop stops the workflow monitor
func (m *WorkflowMonitor) Stop() {
	close(m.stopChan)
	m.wg.Wait()
	m.logger.Info("Workflow monitor stopped")
}

// monitorWorkflows periodically checks workflow statuses
func (m *WorkflowMonitor) monitorWorkflows() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Initial check
	m.checkWorkflows()

	for {
		select {
		case <-ticker.C:
			m.checkWorkflows()
		case <-m.stopChan:
			return
		}
	}
}

// checkWorkflows checks and updates workflow statuses
func (m *WorkflowMonitor) checkWorkflows() {
	ctx := context.Background()

	// Get all running workflows from database
	var workflows []models.Workflow
	if err := m.db.Where("status IN ?", []models.WorkflowStatus{
		models.WorkflowStatusPending,
		models.WorkflowStatusRunning,
	}).Find(&workflows).Error; err != nil {
		m.logger.Error("Failed to fetch running workflows", zap.Error(err))
		return
	}

	m.logger.Debug("Checking workflow statuses", zap.Int("count", len(workflows)))

	// Check each workflow status in Temporal
	for _, workflow := range workflows {
		if workflow.TemporalID == "" || workflow.TemporalRunID == "" {
			continue
		}

		// Describe workflow execution
		resp, err := m.temporalClient.DescribeWorkflowExecution(ctx, workflow.TemporalID, workflow.TemporalRunID)
		if err != nil {
			m.logger.Error("Failed to describe workflow execution",
				zap.String("workflowID", workflow.ID),
				zap.String("temporalID", workflow.TemporalID),
				zap.Error(err))
			continue
		}

		// Update workflow status based on Temporal status
		m.updateWorkflowStatus(&workflow, resp)
	}
}

// updateWorkflowStatus updates workflow status based on Temporal workflow info
func (m *WorkflowMonitor) updateWorkflowStatus(workflow *models.Workflow, info *workflowservice.DescribeWorkflowExecutionResponse) {
	executionInfo := info.WorkflowExecutionInfo
	if executionInfo == nil {
		return
	}

	// Map Temporal status to our workflow status
	var newStatus models.WorkflowStatus
	var errorMsg string

	switch executionInfo.Status {
	case 1: // WORKFLOW_EXECUTION_STATUS_RUNNING
		newStatus = models.WorkflowStatusRunning
	case 2: // WORKFLOW_EXECUTION_STATUS_COMPLETED
		newStatus = models.WorkflowStatusCompleted
	case 3: // WORKFLOW_EXECUTION_STATUS_FAILED
		newStatus = models.WorkflowStatusFailed
		errorMsg = "Workflow execution failed"
	case 4: // WORKFLOW_EXECUTION_STATUS_CANCELED
		newStatus = models.WorkflowStatusCancelled
		errorMsg = "Workflow execution cancelled"
	case 5: // WORKFLOW_EXECUTION_STATUS_TERMINATED
		newStatus = models.WorkflowStatusTerminated
		errorMsg = "Workflow execution terminated"
	case 6: // WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW
		newStatus = models.WorkflowStatusRunning
	case 7: // WORKFLOW_EXECUTION_STATUS_TIMED_OUT
		newStatus = models.WorkflowStatusTimedOut
		errorMsg = "Workflow execution timed out"
	default:
		return // Unknown status
	}

	// Check if status changed
	if workflow.Status == newStatus {
		return
	}

	// Update workflow status
	workflow.Status = newStatus
	if errorMsg != "" {
		workflow.Error = errorMsg
	}

	// Set completion time if workflow is in terminal state
	if workflow.IsTerminal() && workflow.CompletedAt == nil {
		now := time.Now()
		workflow.CompletedAt = &now
	}

	// Extract workflow result if completed
	if newStatus == models.WorkflowStatusCompleted && executionInfo.CloseTime != nil {
		// Try to get workflow result
		result, err := m.getWorkflowResult(workflow.TemporalID, workflow.TemporalRunID)
		if err == nil && result != nil {
			workflow.Output = result
		}
	}

	// Save updated workflow
	if err := m.db.Save(workflow).Error; err != nil {
		m.logger.Error("Failed to update workflow status",
			zap.String("workflowID", workflow.ID),
			zap.String("newStatus", string(newStatus)),
			zap.Error(err))
		return
	}

	// Clear cache for this workflow so the API gets fresh data
	ctx := context.Background()
	cacheKey := fmt.Sprintf("workflow:%s", workflow.ID)
	if err := m.redis.Del(ctx, cacheKey).Err(); err != nil {
		m.logger.Error("Failed to clear workflow cache",
			zap.String("workflowID", workflow.ID),
			zap.Error(err))
	}

	m.logger.Info("Updated workflow status",
		zap.String("workflowID", workflow.ID),
		zap.String("oldStatus", string(workflow.Status)),
		zap.String("newStatus", string(newStatus)))
}

// getWorkflowResult retrieves the result of a completed workflow
func (m *WorkflowMonitor) getWorkflowResult(workflowID, runID string) (json.RawMessage, error) {
	ctx := context.Background()

	// Create a workflow run to get the result
	workflowRun := m.temporalClient.GetWorkflow(ctx, workflowID, runID)

	// Get the result
	var result interface{}
	err := workflowRun.Get(ctx, &result)
	if err != nil {
		return nil, err
	}

	// Convert result to JSON
	resultData, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return resultData, nil
}