package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/quantumlayer/uos/services/orchestrator/internal/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Mock clients
type MockTemporalClient struct {
	mock.Mock
	client.Client
}

type MockIntentClient struct {
	mock.Mock
}

type MockAgentClient struct {
	mock.Mock
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Auto-migrate test models
	err = db.AutoMigrate(&models.Workflow{}, &models.WorkflowStep{}, &models.Execution{})
	assert.NoError(t, err)

	return db
}

func setupTestRedis() *redis.Client {
	// Use redis mock or miniredis for testing
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use a separate DB for testing
	})
}

func TestWorkflowEngine_StartWorkflow(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	redisClient := setupTestRedis()
	logger := zap.NewNop()
	
	mockTemporalClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)
	
	mockIntentClient := &MockIntentClient{}
	mockAgentClient := &MockAgentClient{}
	
	config := &WorkflowConfig{
		TaskQueue:               "test-queue",
		MaxConcurrentWorkflows:  10,
		MaxConcurrentActivities: 10,
		WorkflowTimeout:         time.Hour,
		ActivityTimeout:         time.Minute * 5,
	}
	
	engine := NewWorkflowEngine(
		db,
		redisClient,
		mockTemporalClient,
		logger,
		mockIntentClient,
		mockAgentClient,
		config,
	)

	// Test data
	req := &StartWorkflowRequest{
		Name:           "Test Workflow",
		Description:    "Test Description",
		Type:           "intent_processing",
		Priority:       "medium",
		ProjectID:      "test-project-id",
		UserID:         "test-user-id",
		Input:          json.RawMessage(`{"test": "data"}`),
		Config:         json.RawMessage(`{"key": "value"}`),
		MaxRetries:     3,
		TimeoutSeconds: 3600,
	}

	// Mock expectations
	mockWorkflowRun.On("GetID").Return("temporal-workflow-id")
	mockWorkflowRun.On("GetRunID").Return("temporal-run-id")
	
	mockTemporalClient.On("ExecuteWorkflow",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(mockWorkflowRun, nil)

	// Execute
	resp, err := engine.StartWorkflow(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.WorkflowID)
	assert.Equal(t, "temporal-workflow-id", resp.TemporalID)
	assert.Equal(t, "temporal-run-id", resp.TemporalRunID)
	assert.Equal(t, "running", resp.Status)

	// Verify workflow was saved to database
	var workflow models.Workflow
	err = db.First(&workflow, "name = ?", "Test Workflow").Error
	assert.NoError(t, err)
	assert.Equal(t, req.Name, workflow.Name)
	assert.Equal(t, models.WorkflowStatusRunning, workflow.Status)
}

func TestWorkflowEngine_GetWorkflow(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	redisClient := setupTestRedis()
	logger := zap.NewNop()
	
	engine := &WorkflowEngine{
		db:     db,
		redis:  redisClient,
		logger: logger,
	}

	// Create test workflow
	workflow := &models.Workflow{
		ID:          "test-workflow-id",
		Name:        "Test Workflow",
		Type:        models.WorkflowTypeIntent,
		Status:      models.WorkflowStatusRunning,
		ProjectID:   "test-project-id",
		CreatedBy:   "test-user",
		UpdatedBy:   "test-user",
	}
	err := db.Create(workflow).Error
	assert.NoError(t, err)

	// Execute
	result, err := engine.GetWorkflow(context.Background(), workflow.ID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, workflow.ID, result.ID)
	assert.Equal(t, workflow.Name, result.Name)
}

func TestWorkflowEngine_CancelWorkflow(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	redisClient := setupTestRedis()
	logger := zap.NewNop()
	
	mockTemporalClient := new(mocks.Client)
	
	engine := &WorkflowEngine{
		db:             db,
		redis:          redisClient,
		temporalClient: mockTemporalClient,
		logger:         logger,
	}

	// Create test workflow
	workflow := &models.Workflow{
		ID:            "test-workflow-id",
		Name:          "Test Workflow",
		Type:          models.WorkflowTypeIntent,
		Status:        models.WorkflowStatusRunning,
		ProjectID:     "test-project-id",
		TemporalID:    "temporal-id",
		TemporalRunID: "temporal-run-id",
		CreatedBy:     "test-user",
		UpdatedBy:     "test-user",
	}
	err := db.Create(workflow).Error
	assert.NoError(t, err)

	// Mock expectations
	mockTemporalClient.On("CancelWorkflow",
		mock.Anything,
		"temporal-id",
		"temporal-run-id",
	).Return(nil)

	// Execute
	err = engine.CancelWorkflow(context.Background(), workflow.ID, "Test cancellation")

	// Assert
	assert.NoError(t, err)

	// Verify workflow status was updated
	var updatedWorkflow models.Workflow
	err = db.First(&updatedWorkflow, "id = ?", workflow.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, models.WorkflowStatusCancelled, updatedWorkflow.Status)
	assert.Equal(t, "Test cancellation", updatedWorkflow.Error)
	assert.NotNil(t, updatedWorkflow.CompletedAt)
}

func TestWorkflowEngine_ListWorkflows(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	redisClient := setupTestRedis()
	logger := zap.NewNop()
	
	engine := &WorkflowEngine{
		db:     db,
		redis:  redisClient,
		logger: logger,
	}

	// Create test workflows
	projectID := "test-project-id"
	workflows := []models.Workflow{
		{
			Name:      "Workflow 1",
			Type:      models.WorkflowTypeIntent,
			Status:    models.WorkflowStatusRunning,
			ProjectID: projectID,
			CreatedBy: "user1",
			UpdatedBy: "user1",
		},
		{
			Name:      "Workflow 2",
			Type:      models.WorkflowTypeExecution,
			Status:    models.WorkflowStatusCompleted,
			ProjectID: projectID,
			CreatedBy: "user2",
			UpdatedBy: "user2",
		},
		{
			Name:      "Workflow 3",
			Type:      models.WorkflowTypeIntent,
			Status:    models.WorkflowStatusFailed,
			ProjectID: projectID,
			CreatedBy: "user1",
			UpdatedBy: "user1",
		},
	}

	for _, w := range workflows {
		err := db.Create(&w).Error
		assert.NoError(t, err)
	}

	// Test cases
	tests := []struct {
		name          string
		filters       *WorkflowFilters
		expectedCount int
	}{
		{
			name: "Filter by project ID",
			filters: &WorkflowFilters{
				ProjectID: projectID,
			},
			expectedCount: 3,
		},
		{
			name: "Filter by status",
			filters: &WorkflowFilters{
				ProjectID: projectID,
				Status:    "running",
			},
			expectedCount: 1,
		},
		{
			name: "Filter by type",
			filters: &WorkflowFilters{
				ProjectID: projectID,
				Type:      "intent_processing",
			},
			expectedCount: 2,
		},
		{
			name: "Filter by creator",
			filters: &WorkflowFilters{
				ProjectID: projectID,
				CreatedBy: "user1",
			},
			expectedCount: 2,
		},
		{
			name: "With pagination",
			filters: &WorkflowFilters{
				ProjectID: projectID,
				Limit:     2,
				Offset:    0,
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute
			results, total, err := engine.ListWorkflows(context.Background(), tt.filters)

			// Assert
			assert.NoError(t, err)
			assert.Len(t, results, tt.expectedCount)
			if tt.filters.Limit == 0 {
				assert.Equal(t, int64(tt.expectedCount), total)
			} else {
				assert.Equal(t, int64(3), total) // Total count ignores pagination
			}
		})
	}
}

func TestWorkflowEngine_GetWorkflowMetrics(t *testing.T) {
	// Setup
	db := setupTestDB(t)
	redisClient := setupTestRedis()
	logger := zap.NewNop()
	
	engine := &WorkflowEngine{
		db:     db,
		redis:  redisClient,
		logger: logger,
	}

	// Create test workflow with steps
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()
	
	workflow := &models.Workflow{
		ID:          "test-workflow-id",
		Name:        "Test Workflow",
		Type:        models.WorkflowTypeIntent,
		Status:      models.WorkflowStatusCompleted,
		ProjectID:   "test-project-id",
		StartedAt:   &startTime,
		CompletedAt: &endTime,
		Duration:    3600, // 1 hour in seconds
		RetryCount:  1,
		CreatedBy:   "test-user",
		UpdatedBy:   "test-user",
	}
	err := db.Create(workflow).Error
	assert.NoError(t, err)

	// Create workflow steps
	steps := []models.WorkflowStep{
		{
			WorkflowID:  workflow.ID,
			Name:        "Step 1",
			Type:        "action",
			Status:      models.WorkflowStatusCompleted,
			StartedAt:   &startTime,
			CompletedAt: &endTime,
			Duration:    1800000, // 30 minutes in milliseconds
		},
		{
			WorkflowID:  workflow.ID,
			Name:        "Step 2",
			Type:        "action",
			Status:      models.WorkflowStatusCompleted,
			StartedAt:   &startTime,
			CompletedAt: &endTime,
			Duration:    1800000, // 30 minutes in milliseconds
		},
	}
	
	for _, step := range steps {
		err := db.Create(&step).Error
		assert.NoError(t, err)
	}

	// Execute
	metrics, err := engine.GetWorkflowMetrics(context.Background(), workflow.ID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.Equal(t, workflow.ID, metrics.WorkflowID)
	assert.Equal(t, "completed", metrics.Status)
	assert.Equal(t, int64(3600), metrics.Duration)
	assert.Len(t, metrics.StepMetrics, 2)
}