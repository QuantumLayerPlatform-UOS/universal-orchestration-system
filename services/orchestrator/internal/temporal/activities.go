package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"orchestrator/internal/models"
	"orchestrator/internal/services"
)

// Activities contains all activity implementations
type Activities struct {
	db           *gorm.DB
	logger       *zap.Logger
	intentClient *services.IntentClient
	agentClient  *services.AgentClient
}

// NewActivities creates new activities instance
func NewActivities(
	db *gorm.DB,
	logger *zap.Logger,
	intentClient *services.IntentClient,
	agentClient *services.AgentClient,
) *Activities {
	return &Activities{
		db:           db,
		logger:       logger,
		intentClient: intentClient,
		agentClient:  agentClient,
	}
}

// Intent Processing Activities

// AnalyzeIntentActivity analyzes an intent
func (a *Activities) AnalyzeIntentActivity(ctx context.Context, intentData IntentData) (*IntentAnalysisResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Analyzing intent", zap.String("type", intentData.Type))

	// Send intent to Intent Processor service
	resp, err := a.intentClient.AnalyzeIntent(ctx, &services.AnalyzeIntentRequest{
		Content:   intentData.Content,
		Context:   convertToStringMap(intentData.Context),
		ProjectID: getProjectIDFromContext(ctx),
		UserID:    getUserIDFromContext(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to analyze intent: %w", err)
	}

	// Convert response to analysis result
	result := &IntentAnalysisResult{
		IntentType: resp.IntentType,
		Confidence: float64(resp.Confidence),
		Actions:    resp.RequiredParams,
		Requirements: map[string]interface{}{
			"required_params": resp.RequiredParams,
			"optional_params": resp.OptionalParams,
			"estimated_time":  resp.EstimatedTime,
			"estimated_cost":  resp.EstimatedCost,
		},
	}

	activity.RecordHeartbeat(ctx, "Intent analysis completed")
	return result, nil
}

// CreateExecutionPlanActivity creates an execution plan
func (a *Activities) CreateExecutionPlanActivity(ctx context.Context, analysis IntentAnalysisResult) (*ExecutionPlan, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating execution plan", zap.String("intent_type", analysis.IntentType))

	// Create plan based on intent type and requirements
	plan := &ExecutionPlan{
		Steps: []ExecutionStep{},
	}

	// Add steps based on actions
	for i, action := range analysis.Actions {
		step := ExecutionStep{
			ID:   fmt.Sprintf("step-%d", i+1),
			Name: action,
			Type: "action",
			Config: map[string]interface{}{
				"action":       action,
				"requirements": analysis.Requirements,
			},
		}
		
		// Set dependencies for sequential execution
		if i > 0 {
			step.DependsOn = []string{fmt.Sprintf("step-%d", i)}
		}
		
		plan.Steps = append(plan.Steps, step)
	}

	activity.RecordHeartbeat(ctx, "Execution plan created")
	return plan, nil
}

// ExecuteStepActivity executes a single step
func (a *Activities) ExecuteStepActivity(ctx context.Context, step ExecutionStep) (*StepResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing step", zap.String("id", step.ID), zap.String("name", step.Name))

	// Record step execution in database
	execution := &models.Execution{
		ProjectID: getProjectIDFromContext(ctx),
		Name:      step.Name,
		Type:      models.ExecutionType(step.Type),
		Status:    models.ExecutionStatusRunning,
		StartedAt: timePtr(time.Now()),
	}
	
	if err := a.db.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	// Execute based on step type
	var output map[string]interface{}
	var err error

	switch step.Type {
	case "action":
		output, err = a.executeAction(ctx, step)
	case "code":
		output, err = a.executeCode(ctx, step)
	case "query":
		output, err = a.executeQuery(ctx, step)
	default:
		err = fmt.Errorf("unknown step type: %s", step.Type)
	}

	// Update execution record
	now := time.Now()
	execution.CompletedAt = &now
	if err != nil {
		execution.Status = models.ExecutionStatusFailed
		execution.Error = err.Error()
	} else {
		execution.Status = models.ExecutionStatusSucceeded
		if outputData, marshalErr := json.Marshal(output); marshalErr == nil {
			execution.Output = outputData
		}
	}
	
	a.db.Save(execution)

	result := &StepResult{
		StepID: step.ID,
		Status: string(execution.Status),
		Output: output,
	}
	
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	activity.RecordHeartbeat(ctx, fmt.Sprintf("Step %s completed", step.ID))
	return result, nil
}

// AggregateResultsActivity aggregates step results
func (a *Activities) AggregateResultsActivity(ctx context.Context, results []StepResult) (*WorkflowResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Aggregating results", zap.Int("count", len(results)))

	// Aggregate all step outputs
	aggregated := make(map[string]interface{})
	allSuccess := true
	
	for _, result := range results {
		aggregated[result.StepID] = result.Output
		if result.Status != "succeeded" {
			allSuccess = false
		}
	}

	workflowResult := &WorkflowResult{
		Status:  "completed",
		Results: aggregated,
		Summary: fmt.Sprintf("Workflow completed with %d steps", len(results)),
	}
	
	if !allSuccess {
		workflowResult.Status = "completed_with_errors"
		workflowResult.Summary = fmt.Sprintf("Workflow completed with errors in some steps")
	}

	return workflowResult, nil
}

// Code Execution Activities

// SelectAgentActivity selects an appropriate agent
func (a *Activities) SelectAgentActivity(ctx context.Context, req CodeExecutionRequest) (*AgentInfo, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Selecting agent", zap.String("language", req.Language))

	// List available agents
	agents, err := a.agentClient.ListAgents(ctx, &services.AgentFilters{
		ProjectID: getProjectIDFromContext(ctx),
		Type:      "code_executor",
		Status:    "active",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Select agent based on language and capabilities
	for _, agent := range agents.Agents {
		for _, capability := range agent.Capabilities {
			if capability == req.Language || capability == "multi-language" {
				return &AgentInfo{
					ID:           agent.ID,
					Type:         agent.Type,
					Capabilities: agent.Capabilities,
					Status:       agent.Status,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no suitable agent found for language: %s", req.Language)
}

// PrepareEnvironmentActivity prepares execution environment
func (a *Activities) PrepareEnvironmentActivity(ctx context.Context, agent AgentInfo, req CodeExecutionRequest) (*EnvironmentInfo, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Preparing environment", zap.String("agent", agent.ID))

	// Send environment preparation request to agent
	taskResp, err := a.agentClient.ExecuteTask(ctx, agent.ID, &services.ExecuteTaskRequest{
		Type: "prepare_environment",
		Input: map[string]interface{}{
			"language":    req.Language,
			"environment": req.Environment,
			"resources":   req.Resources,
		},
		Timeout: 300, // 5 minutes
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare environment: %w", err)
	}

	// Extract environment info from response
	envInfo := &EnvironmentInfo{
		ID:        taskResp.ID,
		Type:      req.Language,
		Resources: taskResp.Output,
	}

	activity.RecordHeartbeat(ctx, "Environment prepared")
	return envInfo, nil
}

// ExecuteCodeActivity executes code
func (a *Activities) ExecuteCodeActivity(ctx context.Context, agent AgentInfo, env EnvironmentInfo, req CodeExecutionRequest) (*ExecutionResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing code", zap.String("agent", agent.ID))

	// Send code execution request to agent
	taskResp, err := a.agentClient.ExecuteTask(ctx, agent.ID, &services.ExecuteTaskRequest{
		Type: "execute_code",
		Input: map[string]interface{}{
			"environment_id": env.ID,
			"language":       req.Language,
			"code":           req.Code,
			"timeout":        600, // 10 minutes
		},
		Timeout: 660, // 11 minutes (includes overhead)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute code: %w", err)
	}

	// Extract execution result
	result := &ExecutionResult{
		Output:   fmt.Sprintf("%v", taskResp.Output["output"]),
		ExitCode: int(taskResp.Output["exit_code"].(float64)),
		Metrics:  taskResp.Output["metrics"].(map[string]interface{}),
	}

	activity.RecordHeartbeat(ctx, "Code execution completed")
	return result, nil
}

// ProcessResultsActivity processes execution results
func (a *Activities) ProcessResultsActivity(ctx context.Context, result ExecutionResult) (*ProcessedResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing results")

	// Process and format results
	processed := &ProcessedResult{
		Success: result.ExitCode == 0,
		Data: map[string]interface{}{
			"output":    result.Output,
			"exit_code": result.ExitCode,
			"metrics":   result.Metrics,
		},
		Summary: "Code execution completed",
	}
	
	if result.ExitCode != 0 {
		processed.Summary = fmt.Sprintf("Code execution failed with exit code %d", result.ExitCode)
	}

	return processed, nil
}

// CleanupEnvironmentActivity cleans up execution environment
func (a *Activities) CleanupEnvironmentActivity(ctx context.Context, env EnvironmentInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Cleaning up environment", zap.String("id", env.ID))

	// This would typically send cleanup request to agent
	// For now, just log the cleanup
	logger.Info("Environment cleanup completed", zap.String("id", env.ID))
	
	return nil
}

// Code Analysis Activities

// FetchCodeActivity fetches code for analysis
func (a *Activities) FetchCodeActivity(ctx context.Context, req CodeAnalysisRequest) (*CodeData, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching code", zap.String("repository", req.Repository))

	// This would typically fetch code from repository
	// For now, return mock data
	codeData := &CodeData{
		Files: []string{"main.go", "handler.go", "service.go"},
		Content: map[string]string{
			"main.go":    "package main\n\nfunc main() {\n\t// Main function\n}",
			"handler.go": "package main\n\nfunc handler() {\n\t// Handler function\n}",
			"service.go": "package main\n\nfunc service() {\n\t// Service function\n}",
		},
		Metadata: map[string]interface{}{
			"repository": req.Repository,
			"branch":     req.Branch,
			"path":       req.Path,
		},
	}

	activity.RecordHeartbeat(ctx, "Code fetched")
	return codeData, nil
}

// RunStaticAnalysisActivity runs static code analysis
func (a *Activities) RunStaticAnalysisActivity(ctx context.Context, code CodeData) (*StaticAnalysisResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Running static analysis")

	// This would run actual static analysis tools
	// For now, return mock results
	result := &StaticAnalysisResult{
		Issues: []interface{}{},
		Metrics: map[string]interface{}{
			"complexity":      10,
			"maintainability": 85,
			"coverage":        75,
		},
		Summary: "Static analysis completed successfully",
	}

	activity.RecordHeartbeat(ctx, "Static analysis completed")
	return result, nil
}

// RunSecurityAnalysisActivity runs security analysis
func (a *Activities) RunSecurityAnalysisActivity(ctx context.Context, code CodeData) (*SecurityAnalysisResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Running security analysis")

	// This would run actual security analysis tools
	// For now, return mock results
	result := &SecurityAnalysisResult{
		Vulnerabilities: []interface{}{},
		RiskScore:       0.2,
		Recommendations: []string{
			"Enable dependency scanning",
			"Add security headers",
		},
	}

	activity.RecordHeartbeat(ctx, "Security analysis completed")
	return result, nil
}

// RunPerformanceAnalysisActivity runs performance analysis
func (a *Activities) RunPerformanceAnalysisActivity(ctx context.Context, code CodeData) (*PerformanceAnalysisResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Running performance analysis")

	// This would run actual performance analysis
	// For now, return mock results
	result := &PerformanceAnalysisResult{
		Bottlenecks:      []interface{}{},
		Optimizations:    []interface{}{},
		PerformanceScore: 0.85,
	}

	activity.RecordHeartbeat(ctx, "Performance analysis completed")
	return result, nil
}

// GenerateAnalysisReportActivity generates analysis report
func (a *Activities) GenerateAnalysisReportActivity(ctx context.Context, static StaticAnalysisResult, security SecurityAnalysisResult, perf PerformanceAnalysisResult) (*AnalysisReport, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Generating analysis report")

	// Calculate overall score
	score := (float64(static.Metrics["maintainability"].(int))/100 + (1-security.RiskScore) + perf.PerformanceScore) / 3

	report := &AnalysisReport{
		Summary:     "Code analysis completed successfully",
		Static:      static,
		Security:    security,
		Performance: perf,
		Score:       score,
	}

	return report, nil
}

// Helper functions

func convertToStringMap(m map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

func getProjectIDFromContext(ctx context.Context) string {
	// Get from activity info or context
	info := activity.GetInfo(ctx)
	if projectID, ok := info.WorkflowExecution.Memo.Get("project_id").(string); ok {
		return projectID
	}
	return ""
}

func getUserIDFromContext(ctx context.Context) string {
	// Get from activity info or context
	info := activity.GetInfo(ctx)
	if userID, ok := info.WorkflowExecution.Memo.Get("user_id").(string); ok {
		return userID
	}
	return ""
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func (a *Activities) executeAction(ctx context.Context, step ExecutionStep) (map[string]interface{}, error) {
	// Execute action based on configuration
	config := step.Config
	action := config["action"].(string)
	
	// Send to intent processor for action execution
	resp, err := a.intentClient.ProcessIntent(ctx, &services.ProcessIntentRequest{
		Type:    "action",
		Content: action,
		Context: convertToStringMap(config),
		ProjectID: getProjectIDFromContext(ctx),
		UserID:    getUserIDFromContext(ctx),
		Async:     false,
		TimeoutSeconds: 300,
	})
	if err != nil {
		return nil, err
	}
	
	return resp.Result, nil
}

func (a *Activities) executeCode(ctx context.Context, step ExecutionStep) (map[string]interface{}, error) {
	// Execute code based on configuration
	config := step.Config
	
	// Extract code execution parameters
	req := CodeExecutionRequest{
		Language:    config["language"].(string),
		Code:        config["code"].(string),
		Environment: make(map[string]string),
		Resources:   config["resources"].(map[string]interface{}),
	}
	
	// Select agent
	agent, err := a.SelectAgentActivity(ctx, req)
	if err != nil {
		return nil, err
	}
	
	// Prepare environment
	env, err := a.PrepareEnvironmentActivity(ctx, *agent, req)
	if err != nil {
		return nil, err
	}
	
	// Execute code
	result, err := a.ExecuteCodeActivity(ctx, *agent, *env, req)
	if err != nil {
		return nil, err
	}
	
	// Process results
	processed, err := a.ProcessResultsActivity(ctx, *result)
	if err != nil {
		return nil, err
	}
	
	return processed.Data, nil
}

func (a *Activities) executeQuery(ctx context.Context, step ExecutionStep) (map[string]interface{}, error) {
	// Execute query based on configuration
	config := step.Config
	query := config["query"].(string)
	
	// This would execute the query against appropriate data source
	// For now, return mock result
	return map[string]interface{}{
		"query":   query,
		"results": []interface{}{},
		"count":   0,
	}, nil
}