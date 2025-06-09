package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"orchestrator/internal/models"
	"orchestrator/internal/services"
)

// DemoIntentToExecution demonstrates the full flow from intent to task execution
func (h *Handlers) DemoIntentToExecution(c *gin.Context) {
	h.logger.Info("Starting demo: Intent to Execution flow")

	// Step 1: Get the intent analysis result from request body
	var req struct {
		ProjectID    string          `json:"project_id" binding:"required"`
		IntentResult json.RawMessage `json:"intent_result" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Parse the intent result to extract tasks
	var intentResult struct {
		IntentType string `json:"intent_type"`
		Confidence float64 `json:"confidence"`
		Summary    string `json:"summary"`
		Tasks      []struct {
			ID                    string                 `json:"id"`
			Title                 string                 `json:"title"`
			Description           string                 `json:"description"`
			Type                  string                 `json:"type"`
			Priority              string                 `json:"priority"`
			Complexity            string                 `json:"complexity"`
			EstimatedHours        float64                `json:"estimated_hours"`
			Dependencies          []string               `json:"dependencies"`
			Tags                  []string               `json:"tags"`
			AcceptanceCriteria    []string               `json:"acceptance_criteria"`
			TechnicalRequirements map[string]interface{} `json:"technical_requirements"`
		} `json:"tasks"`
		Metadata map[string]interface{} `json:"metadata"`
	}

	if err := json.Unmarshal(req.IntentResult, &intentResult); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid intent result format", err)
		return
	}

	h.logger.Info("Processing intent result",
		zap.String("projectID", req.ProjectID),
		zap.String("intentType", intentResult.IntentType),
		zap.Int("taskCount", len(intentResult.Tasks)))

	// Step 2: Create a task execution workflow
	workflowInput := map[string]interface{}{
		"project_id":    req.ProjectID,
		"intent_result": intentResult,
		"tasks":         intentResult.Tasks,
		"context": map[string]interface{}{
			"demo": true,
			"user": c.GetString("user_id"),
		},
	}

	inputData, err := json.Marshal(workflowInput)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to prepare workflow input", err)
		return
	}

	// Create the workflow
	workflowReq := &services.StartWorkflowRequest{
		Name:           "Demo Task Execution - " + intentResult.Summary,
		Description:    "Automated execution of tasks from intent analysis",
		Type:           string(models.WorkflowTypeTaskExecution),
		Priority:       "high",
		ProjectID:      req.ProjectID,
		UserID:         c.GetString("user_id"),
		Input:          inputData,
		Config:         json.RawMessage(`{}`),
		MaxRetries:     3,
		TimeoutSeconds: 7200, // 2 hours
	}

	// Start the workflow
	h.logger.Info("Starting task execution workflow")
	response, err := h.workflowEngine.StartWorkflow(c.Request.Context(), workflowReq)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to start workflow", err)
		return
	}

	h.logger.Info("Task execution workflow started",
		zap.String("workflowID", response.WorkflowID),
		zap.String("temporalID", response.TemporalID))

	// Return the response
	h.respondSuccess(c, http.StatusCreated, gin.H{
		"message": "Task execution workflow started successfully",
		"workflow": response,
		"summary": gin.H{
			"project_id":      req.ProjectID,
			"intent_type":     intentResult.IntentType,
			"total_tasks":     len(intentResult.Tasks),
			"total_hours":     calculateTotalHours(intentResult.Tasks),
			"workflow_status": "Check workflow status at GET /api/v1/workflows/" + response.WorkflowID,
		},
	})
}

// Helper function to calculate total estimated hours
func calculateTotalHours(tasks []struct {
	ID                    string                 `json:"id"`
	Title                 string                 `json:"title"`
	Description           string                 `json:"description"`
	Type                  string                 `json:"type"`
	Priority              string                 `json:"priority"`
	Complexity            string                 `json:"complexity"`
	EstimatedHours        float64                `json:"estimated_hours"`
	Dependencies          []string               `json:"dependencies"`
	Tags                  []string               `json:"tags"`
	AcceptanceCriteria    []string               `json:"acceptance_criteria"`
	TechnicalRequirements map[string]interface{} `json:"technical_requirements"`
}) float64 {
	total := 0.0
	for _, task := range tasks {
		total += task.EstimatedHours
	}
	return total
}