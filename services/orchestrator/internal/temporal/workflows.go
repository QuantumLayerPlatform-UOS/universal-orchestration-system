package temporal

import (
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"

	"orchestrator/internal/models"
	"orchestrator/internal/services"
)

// WorkflowEngine implements Temporal workflows
type WorkflowEngine struct {
	logger *zap.Logger
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(logger *zap.Logger) *WorkflowEngine {
	return &WorkflowEngine{
		logger: logger,
	}
}

// IntentProcessingWorkflow handles intent processing workflow
func (w *WorkflowEngine) IntentProcessingWorkflow(ctx workflow.Context, wf *models.Workflow) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting intent processing workflow", "workflowID", wf.ID)

	// Set workflow options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Parse and validate intent
	var intentData IntentData
	if err := json.Unmarshal(wf.Input, &intentData); err != nil {
		return fmt.Errorf("failed to parse intent data: %w", err)
	}

	var analysisResult IntentAnalysisResult
	err := workflow.ExecuteActivity(ctx, AnalyzeIntentActivity, intentData).Get(ctx, &analysisResult)
	if err != nil {
		return fmt.Errorf("intent analysis failed: %w", err)
	}

	// Step 2: Create execution plan
	var executionPlan ExecutionPlan
	err = workflow.ExecuteActivity(ctx, CreateExecutionPlanActivity, analysisResult).Get(ctx, &executionPlan)
	if err != nil {
		return fmt.Errorf("failed to create execution plan: %w", err)
	}

	// Step 3: Execute plan steps in parallel or sequence based on dependencies
	var results []StepResult
	for _, step := range executionPlan.Steps {
		if len(step.DependsOn) == 0 {
			// Execute independent steps in parallel
			var stepResult StepResult
			err = workflow.ExecuteActivity(ctx, ExecuteStepActivity, step).Get(ctx, &stepResult)
			if err != nil {
				return fmt.Errorf("step execution failed: %w", err)
			}
			results = append(results, stepResult)
		}
	}

	// Step 4: Aggregate results
	var finalResult WorkflowResult
	err = workflow.ExecuteActivity(ctx, AggregateResultsActivity, results).Get(ctx, &finalResult)
	if err != nil {
		return fmt.Errorf("failed to aggregate results: %w", err)
	}

	// Update workflow output
	outputData, _ := json.Marshal(finalResult)
	wf.Output = outputData

	logger.Info("Intent processing workflow completed", "workflowID", wf.ID)
	return nil
}

// CodeExecutionWorkflow handles code execution workflow
func (w *WorkflowEngine) CodeExecutionWorkflow(ctx workflow.Context, wf *models.Workflow) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting code execution workflow", "workflowID", wf.ID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    1 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Parse execution request
	var execRequest CodeExecutionRequest
	if err := json.Unmarshal(wf.Input, &execRequest); err != nil {
		return fmt.Errorf("failed to parse execution request: %w", err)
	}

	// Step 2: Select appropriate agent
	var agent AgentInfo
	err := workflow.ExecuteActivity(ctx, SelectAgentActivity, execRequest).Get(ctx, &agent)
	if err != nil {
		return fmt.Errorf("failed to select agent: %w", err)
	}

	// Step 3: Prepare execution environment
	var envInfo EnvironmentInfo
	err = workflow.ExecuteActivity(ctx, PrepareEnvironmentActivity, agent, execRequest).Get(ctx, &envInfo)
	if err != nil {
		return fmt.Errorf("failed to prepare environment: %w", err)
	}

	// Step 4: Execute code
	var execResult ExecutionResult
	err = workflow.ExecuteActivity(ctx, ExecuteCodeActivity, agent, envInfo, execRequest).Get(ctx, &execResult)
	if err != nil {
		return fmt.Errorf("code execution failed: %w", err)
	}

	// Step 5: Process results
	var processedResult ProcessedResult
	err = workflow.ExecuteActivity(ctx, ProcessResultsActivity, execResult).Get(ctx, &processedResult)
	if err != nil {
		return fmt.Errorf("failed to process results: %w", err)
	}

	// Step 6: Cleanup environment
	err = workflow.ExecuteActivity(ctx, CleanupEnvironmentActivity, envInfo).Get(ctx, nil)
	if err != nil {
		// Log but don't fail workflow for cleanup errors
		logger.Error("Failed to cleanup environment", zap.Error(err))
	}

	// Update workflow output
	outputData, _ := json.Marshal(processedResult)
	wf.Output = outputData

	logger.Info("Code execution workflow completed", "workflowID", wf.ID)
	return nil
}

// CodeAnalysisWorkflow handles code analysis workflow
func (w *WorkflowEngine) CodeAnalysisWorkflow(ctx workflow.Context, wf *models.Workflow) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting code analysis workflow", "workflowID", wf.ID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Parse analysis request
	var analysisRequest CodeAnalysisRequest
	if err := json.Unmarshal(wf.Input, &analysisRequest); err != nil {
		return fmt.Errorf("failed to parse analysis request: %w", err)
	}

	// Step 2: Fetch code
	var codeData CodeData
	err := workflow.ExecuteActivity(ctx, FetchCodeActivity, analysisRequest).Get(ctx, &codeData)
	if err != nil {
		return fmt.Errorf("failed to fetch code: %w", err)
	}

	// Step 3: Run multiple analyses in parallel
	selector := workflow.NewSelector(ctx)

	// Static analysis
	staticFuture := workflow.ExecuteActivity(ctx, RunStaticAnalysisActivity, codeData)
	var staticResult StaticAnalysisResult
	selector.AddFuture(staticFuture, func(f workflow.Future) {
		f.Get(ctx, &staticResult)
	})

	// Security analysis
	securityFuture := workflow.ExecuteActivity(ctx, RunSecurityAnalysisActivity, codeData)
	var securityResult SecurityAnalysisResult
	selector.AddFuture(securityFuture, func(f workflow.Future) {
		f.Get(ctx, &securityResult)
	})

	// Performance analysis
	perfFuture := workflow.ExecuteActivity(ctx, RunPerformanceAnalysisActivity, codeData)
	var perfResult PerformanceAnalysisResult
	selector.AddFuture(perfFuture, func(f workflow.Future) {
		f.Get(ctx, &perfResult)
	})

	// Wait for all analyses to complete
	for i := 0; i < 3; i++ {
		selector.Select(ctx)
	}

	// Step 4: Generate report
	var report AnalysisReport
	err = workflow.ExecuteActivity(ctx, GenerateAnalysisReportActivity, 
		staticResult, securityResult, perfResult).Get(ctx, &report)
	if err != nil {
		return fmt.Errorf("failed to generate analysis report: %w", err)
	}

	// Update workflow output
	outputData, _ := json.Marshal(report)
	wf.Output = outputData

	logger.Info("Code analysis workflow completed", "workflowID", wf.ID)
	return nil
}

// CodeReviewWorkflow handles code review workflow
func (w *WorkflowEngine) CodeReviewWorkflow(ctx workflow.Context, wf *models.Workflow) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting code review workflow", "workflowID", wf.ID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 20 * time.Minute,
		HeartbeatTimeout:    3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Parse review request
	var reviewRequest CodeReviewRequest
	if err := json.Unmarshal(wf.Input, &reviewRequest); err != nil {
		return fmt.Errorf("failed to parse review request: %w", err)
	}

	// Step 2: Fetch code changes
	var codeChanges CodeChanges
	err := workflow.ExecuteActivity(ctx, FetchCodeChangesActivity, reviewRequest).Get(ctx, &codeChanges)
	if err != nil {
		return fmt.Errorf("failed to fetch code changes: %w", err)
	}

	// Step 3: Run automated checks
	var automatedChecks AutomatedCheckResults
	err = workflow.ExecuteActivity(ctx, RunAutomatedChecksActivity, codeChanges).Get(ctx, &automatedChecks)
	if err != nil {
		return fmt.Errorf("failed to run automated checks: %w", err)
	}

	// Step 4: AI-powered review
	var aiReview AIReviewResult
	err = workflow.ExecuteActivity(ctx, RunAIReviewActivity, codeChanges, automatedChecks).Get(ctx, &aiReview)
	if err != nil {
		return fmt.Errorf("failed to run AI review: %w", err)
	}

	// Step 5: Generate review summary
	var reviewSummary ReviewSummary
	err = workflow.ExecuteActivity(ctx, GenerateReviewSummaryActivity, automatedChecks, aiReview).Get(ctx, &reviewSummary)
	if err != nil {
		return fmt.Errorf("failed to generate review summary: %w", err)
	}

	// Step 6: Post review comments (if configured)
	if reviewRequest.PostComments {
		err = workflow.ExecuteActivity(ctx, PostReviewCommentsActivity, reviewSummary).Get(ctx, nil)
		if err != nil {
			logger.Error("Failed to post review comments", zap.Error(err))
		}
	}

	// Update workflow output
	outputData, _ := json.Marshal(reviewSummary)
	wf.Output = outputData

	logger.Info("Code review workflow completed", "workflowID", wf.ID)
	return nil
}

// DeploymentWorkflow handles deployment workflow
func (w *WorkflowEngine) DeploymentWorkflow(ctx workflow.Context, wf *models.Workflow) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting deployment workflow", "workflowID", wf.ID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Parse deployment request
	var deployRequest DeploymentRequest
	if err := json.Unmarshal(wf.Input, &deployRequest); err != nil {
		return fmt.Errorf("failed to parse deployment request: %w", err)
	}

	// Step 2: Validate deployment
	var validation DeploymentValidation
	err := workflow.ExecuteActivity(ctx, ValidateDeploymentActivity, deployRequest).Get(ctx, &validation)
	if err != nil {
		return fmt.Errorf("deployment validation failed: %w", err)
	}

	if !validation.IsValid {
		return fmt.Errorf("deployment validation failed: %s", validation.Errors)
	}

	// Step 3: Build artifacts
	var buildResult BuildResult
	err = workflow.ExecuteActivity(ctx, BuildArtifactsActivity, deployRequest).Get(ctx, &buildResult)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Step 4: Run tests
	var testResult TestResult
	err = workflow.ExecuteActivity(ctx, RunDeploymentTestsActivity, buildResult).Get(ctx, &testResult)
	if err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	// Step 5: Deploy to staging (if configured)
	if deployRequest.DeployToStaging {
		var stagingResult DeploymentResult
		err = workflow.ExecuteActivity(ctx, DeployToStagingActivity, buildResult).Get(ctx, &stagingResult)
		if err != nil {
			return fmt.Errorf("staging deployment failed: %w", err)
		}

		// Run smoke tests on staging
		var smokeTestResult TestResult
		err = workflow.ExecuteActivity(ctx, RunSmokeTestsActivity, stagingResult).Get(ctx, &smokeTestResult)
		if err != nil {
			// Rollback staging
			workflow.ExecuteActivity(ctx, RollbackDeploymentActivity, stagingResult).Get(ctx, nil)
			return fmt.Errorf("staging smoke tests failed: %w", err)
		}
	}

	// Step 6: Deploy to production
	var prodResult DeploymentResult
	err = workflow.ExecuteActivity(ctx, DeployToProductionActivity, buildResult).Get(ctx, &prodResult)
	if err != nil {
		return fmt.Errorf("production deployment failed: %w", err)
	}

	// Step 7: Health check
	var healthCheck HealthCheckResult
	err = workflow.ExecuteActivity(ctx, RunHealthCheckActivity, prodResult).Get(ctx, &healthCheck)
	if err != nil || !healthCheck.IsHealthy {
		// Rollback production
		workflow.ExecuteActivity(ctx, RollbackDeploymentActivity, prodResult).Get(ctx, nil)
		return fmt.Errorf("health check failed: %w", err)
	}

	// Step 8: Update deployment status
	err = workflow.ExecuteActivity(ctx, UpdateDeploymentStatusActivity, prodResult).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to update deployment status", zap.Error(err))
	}

	// Update workflow output
	outputData, _ := json.Marshal(prodResult)
	wf.Output = outputData

	logger.Info("Deployment workflow completed", "workflowID", wf.ID)
	return nil
}

// CustomWorkflow handles custom workflow types
func (w *WorkflowEngine) CustomWorkflow(ctx workflow.Context, wf *models.Workflow) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting custom workflow", "workflowID", wf.ID)

	// Parse custom workflow definition
	var customDef CustomWorkflowDefinition
	if err := json.Unmarshal(wf.Config, &customDef); err != nil {
		return fmt.Errorf("failed to parse custom workflow definition: %w", err)
	}

	// Execute custom steps based on definition
	for _, step := range customDef.Steps {
		ao := workflow.ActivityOptions{
			StartToCloseTimeout: time.Duration(step.TimeoutSeconds) * time.Second,
			HeartbeatTimeout:    30 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    time.Second,
				BackoffCoefficient: 2.0,
				MaximumInterval:    time.Minute,
				MaximumAttempts:    int32(step.MaxRetries),
			},
		}
		stepCtx := workflow.WithActivityOptions(ctx, ao)

		var stepResult interface{}
		err := workflow.ExecuteActivity(stepCtx, ExecuteCustomStepActivity, step).Get(stepCtx, &stepResult)
		if err != nil {
			if step.ContinueOnError {
				logger.Warn("Custom step failed but continuing", 
					zap.String("step", step.Name), 
					zap.Error(err))
			} else {
				return fmt.Errorf("custom step %s failed: %w", step.Name, err)
			}
		}
	}

	logger.Info("Custom workflow completed", "workflowID", wf.ID)
	return nil
}

// Helper types for workflows

type IntentData struct {
	Type        string                 `json:"type"`
	Content     string                 `json:"content"`
	Context     map[string]interface{} `json:"context"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type IntentAnalysisResult struct {
	IntentType   string                 `json:"intent_type"`
	Confidence   float64                `json:"confidence"`
	Actions      []string               `json:"actions"`
	Requirements map[string]interface{} `json:"requirements"`
}

type ExecutionPlan struct {
	Steps []ExecutionStep `json:"steps"`
}

type ExecutionStep struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	DependsOn []string `json:"depends_on"`
	Config    map[string]interface{} `json:"config"`
}

type StepResult struct {
	StepID  string                 `json:"step_id"`
	Status  string                 `json:"status"`
	Output  map[string]interface{} `json:"output"`
	Error   string                 `json:"error,omitempty"`
}

type WorkflowResult struct {
	Status  string                 `json:"status"`
	Results map[string]interface{} `json:"results"`
	Summary string                 `json:"summary"`
}

type CodeExecutionRequest struct {
	Language    string                 `json:"language"`
	Code        string                 `json:"code"`
	Environment map[string]string      `json:"environment"`
	Resources   map[string]interface{} `json:"resources"`
}

type AgentInfo struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
}

type EnvironmentInfo struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Resources map[string]interface{} `json:"resources"`
}

type ExecutionResult struct {
	Output   string                 `json:"output"`
	ExitCode int                    `json:"exit_code"`
	Metrics  map[string]interface{} `json:"metrics"`
}

type ProcessedResult struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
	Summary string                 `json:"summary"`
}

type CodeAnalysisRequest struct {
	Repository string   `json:"repository"`
	Branch     string   `json:"branch"`
	Path       string   `json:"path"`
	Types      []string `json:"types"`
}

type CodeData struct {
	Files    []string               `json:"files"`
	Content  map[string]string      `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

type StaticAnalysisResult struct {
	Issues   []interface{} `json:"issues"`
	Metrics  map[string]interface{} `json:"metrics"`
	Summary  string `json:"summary"`
}

type SecurityAnalysisResult struct {
	Vulnerabilities []interface{} `json:"vulnerabilities"`
	RiskScore       float64       `json:"risk_score"`
	Recommendations []string      `json:"recommendations"`
}

type PerformanceAnalysisResult struct {
	Bottlenecks     []interface{} `json:"bottlenecks"`
	Optimizations   []interface{} `json:"optimizations"`
	PerformanceScore float64      `json:"performance_score"`
}

type AnalysisReport struct {
	Summary     string                 `json:"summary"`
	Static      StaticAnalysisResult   `json:"static"`
	Security    SecurityAnalysisResult `json:"security"`
	Performance PerformanceAnalysisResult `json:"performance"`
	Score       float64                `json:"score"`
}

type CodeReviewRequest struct {
	PullRequestID string `json:"pull_request_id"`
	Repository    string `json:"repository"`
	BaseBranch    string `json:"base_branch"`
	HeadBranch    string `json:"head_branch"`
	PostComments  bool   `json:"post_comments"`
}

type CodeChanges struct {
	Files    []string          `json:"files"`
	Additions int              `json:"additions"`
	Deletions int              `json:"deletions"`
	Diff      string           `json:"diff"`
}

type AutomatedCheckResults struct {
	Passed   bool              `json:"passed"`
	Checks   []interface{}     `json:"checks"`
	Coverage float64           `json:"coverage"`
}

type AIReviewResult struct {
	Issues       []interface{}     `json:"issues"`
	Suggestions  []interface{}     `json:"suggestions"`
	CodeQuality  float64           `json:"code_quality"`
}

type ReviewSummary struct {
	Approved     bool              `json:"approved"`
	Comments     []interface{}     `json:"comments"`
	Score        float64           `json:"score"`
}

type DeploymentRequest struct {
	Environment     string            `json:"environment"`
	Version         string            `json:"version"`
	DeployToStaging bool              `json:"deploy_to_staging"`
	Config          map[string]interface{} `json:"config"`
}

type DeploymentValidation struct {
	IsValid bool     `json:"is_valid"`
	Errors  []string `json:"errors"`
}

type BuildResult struct {
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version"`
	Size       int64  `json:"size"`
}

type TestResult struct {
	Passed   bool              `json:"passed"`
	Tests    int               `json:"tests"`
	Failures int               `json:"failures"`
	Coverage float64           `json:"coverage"`
}

type DeploymentResult struct {
	DeploymentID string    `json:"deployment_id"`
	Environment  string    `json:"environment"`
	Version      string    `json:"version"`
	URL          string    `json:"url"`
	Timestamp    time.Time `json:"timestamp"`
}

type HealthCheckResult struct {
	IsHealthy bool              `json:"is_healthy"`
	Checks    map[string]bool   `json:"checks"`
}

type CustomWorkflowDefinition struct {
	Steps []CustomStep `json:"steps"`
}

type CustomStep struct {
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	TimeoutSeconds  int                    `json:"timeout_seconds"`
	MaxRetries      int                    `json:"max_retries"`
	ContinueOnError bool                   `json:"continue_on_error"`
	Config          map[string]interface{} `json:"config"`
}