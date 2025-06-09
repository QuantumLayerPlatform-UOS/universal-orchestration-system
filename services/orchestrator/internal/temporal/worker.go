package temporal

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/api/workflowservice/v1"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"orchestrator/internal/config"
	"orchestrator/internal/services"
)

// Worker represents a Temporal worker
type Worker struct {
	client            client.Client
	worker            worker.Worker
	logger            *zap.Logger
	config            *config.TemporalConfig
	workflows         *WorkflowEngine
	activities        *Activities
	metaAgentActivities *MetaAgentActivities
}

// NewWorker creates a new Temporal worker
func NewWorker(
	cfg *config.TemporalConfig,
	db *gorm.DB,
	logger *zap.Logger,
	intentClient *services.IntentClient,
	agentClient *services.AgentClient,
) (*Worker, error) {
	// Create Temporal client
	temporalClient, err := createTemporalClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Temporal client: %w", err)
	}

	// Create workflow engine
	workflowEngine := NewWorkflowEngine(logger)

	// Create activities
	activities := NewActivities(db, logger, intentClient, agentClient)

	// Create meta-agent activities
	metaAgentActivities := NewMetaAgentActivities(agentClient, logger)

	// Create worker
	w := worker.New(temporalClient, cfg.TaskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize:      cfg.WorkerOptions.MaxConcurrentActivityExecutionSize,
		MaxConcurrentWorkflowTaskExecutionSize:  cfg.WorkerOptions.MaxConcurrentWorkflowTaskExecutionSize,
		MaxConcurrentLocalActivityExecutionSize: cfg.WorkerOptions.MaxConcurrentLocalActivityExecutionSize,
		WorkerActivitiesPerSecond:               cfg.WorkerOptions.WorkerActivitiesPerSecond,
		TaskQueueActivitiesPerSecond:            cfg.WorkerOptions.TaskQueueActivitiesPerSecond,
		WorkerLocalActivitiesPerSecond:          cfg.WorkerOptions.WorkerLocalActivitiesPerSecond,
		EnableLoggingInReplay:                   true,
		DisableWorkflowWorker:                   false,
		LocalActivityWorkerOnly:                 false,
		Identity:                                "orchestrator-worker",
		DeadlockDetectionTimeout:                0, // Use default
		MaxHeartbeatThrottleInterval:            0, // Use default
		DefaultHeartbeatThrottleInterval:        0, // Use default
	})

	// Register workflows
	registerWorkflows(w, workflowEngine)

	// Register activities
	registerActivities(w, activities, metaAgentActivities)

	return &Worker{
		client:              temporalClient,
		worker:              w,
		logger:              logger,
		config:              cfg,
		workflows:           workflowEngine,
		activities:          activities,
		metaAgentActivities: metaAgentActivities,
	}, nil
}

// Start starts the worker
func (w *Worker) Start() error {
	w.logger.Info("Starting Temporal worker with meta-agent integration",
		zap.String("task_queue", w.config.TaskQueue),
		zap.String("namespace", w.config.Namespace),
	)

	// Start worker
	err := w.worker.Start()
	if err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	w.logger.Info("Temporal worker started successfully with meta-agent capabilities")
	return nil
}

// Stop stops the worker
func (w *Worker) Stop() {
	w.logger.Info("Stopping Temporal worker")
	w.worker.Stop()
	w.client.Close()
	w.logger.Info("Temporal worker stopped")
}

// GetClient returns the Temporal client
func (w *Worker) GetClient() client.Client {
	return w.client
}

// createTemporalClient creates a new Temporal client
func createTemporalClient(cfg *config.TemporalConfig, logger *zap.Logger) (client.Client, error) {
	// Configure client options
	clientOptions := client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
		Logger:    NewTemporalLogger(logger),
		ConnectionOptions: client.ConnectionOptions{
			TLS:                          nil, // Configure TLS if needed
			EnableKeepAliveCheck:         cfg.ClientOptions.EnableKeepAlive,
			KeepAliveTime:                0, // Use default
			KeepAliveTimeout:             0, // Use default
			KeepAlivePermitWithoutStream: cfg.ClientOptions.KeepAlivePermitWithoutStream,
		},
	}

	// Add metrics if enabled
	if cfg.EnableMetrics {
		// Configure metrics scope
		// This would typically integrate with Prometheus
		// For now, we'll use the default metrics
	}

	// Create client
	c, err := client.Dial(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create Temporal client: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = c.WorkflowService().GetSystemInfo(ctx, &workflowservice.GetSystemInfoRequest{})
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to verify Temporal connection: %w", err)
	}

	logger.Info("Connected to Temporal",
		zap.String("host", cfg.HostPort),
		zap.String("namespace", cfg.Namespace),
	)

	return c, nil
}

// registerWorkflows registers all workflows with the worker
func registerWorkflows(w worker.Worker, engine *WorkflowEngine) {
	w.RegisterWorkflow(engine.IntentProcessingWorkflow)
	w.RegisterWorkflow(engine.CodeExecutionWorkflow)
	w.RegisterWorkflow(engine.CodeAnalysisWorkflow)
	w.RegisterWorkflow(engine.CodeReviewWorkflow)
	w.RegisterWorkflow(engine.DeploymentWorkflow)
	w.RegisterWorkflow(engine.TaskExecutionWorkflow)
	w.RegisterWorkflow(engine.CustomWorkflow)
}

// registerActivities registers all activities with the worker
func registerActivities(w worker.Worker, activities *Activities, metaAgentActivities *MetaAgentActivities) {
	// Intent processing activities
	w.RegisterActivity(activities.AnalyzeIntentActivity)
	w.RegisterActivity(activities.CreateExecutionPlanActivity)
	w.RegisterActivity(activities.ExecuteStepActivity)
	w.RegisterActivity(activities.AggregateResultsActivity)

	// Code execution activities
	w.RegisterActivity(activities.SelectAgentActivity)
	w.RegisterActivity(activities.PrepareEnvironmentActivity)
	w.RegisterActivity(activities.ExecuteCodeActivity)
	w.RegisterActivity(activities.ProcessResultsActivity)
	w.RegisterActivity(activities.CleanupEnvironmentActivity)

	// Original task execution activities (kept for compatibility)
	w.RegisterActivity(activities.FindOrCreateAgentForTaskActivity)
	w.RegisterActivity(activities.ExecuteTaskWithAgentActivity)
	w.RegisterActivity(activities.AggregateTaskResultsActivity)
	w.RegisterActivity(activities.StoreArtifactsActivity)

	// 🚀 NEW: Meta-Agent Integration Activities
	w.RegisterActivityWithOptions(
		metaAgentActivities.FindOrCreateAgentForTaskActivity,
		activity.RegisterOptions{Name: "MetaAgentFindOrCreateAgentForTaskActivity"},
	)
	w.RegisterActivityWithOptions(
		metaAgentActivities.ExecuteTaskWithAgentActivity,
		activity.RegisterOptions{Name: "MetaAgentExecuteTaskWithAgentActivity"},
	)
	w.RegisterActivityWithOptions(
		metaAgentActivities.OptimizeAgentPerformanceActivity,
		activity.RegisterOptions{Name: "MetaAgentOptimizeAgentPerformanceActivity"},
	)

	// Code analysis activities
	w.RegisterActivity(activities.FetchCodeActivity)
	w.RegisterActivity(activities.RunStaticAnalysisActivity)
	w.RegisterActivity(activities.RunSecurityAnalysisActivity)
	w.RegisterActivity(activities.RunPerformanceAnalysisActivity)
	w.RegisterActivity(activities.GenerateAnalysisReportActivity)

	// Code review activities
	w.RegisterActivity(FetchCodeChangesActivity)
	w.RegisterActivity(RunAutomatedChecksActivity)
	w.RegisterActivity(RunAIReviewActivity)
	w.RegisterActivity(GenerateReviewSummaryActivity)
	w.RegisterActivity(PostReviewCommentsActivity)

	// Deployment activities
	w.RegisterActivity(ValidateDeploymentActivity)
	w.RegisterActivity(BuildArtifactsActivity)
	w.RegisterActivity(RunDeploymentTestsActivity)
	w.RegisterActivity(DeployToStagingActivity)
	w.RegisterActivity(RunSmokeTestsActivity)
	w.RegisterActivity(DeployToProductionActivity)
	w.RegisterActivity(RunHealthCheckActivity)
	w.RegisterActivity(RollbackDeploymentActivity)
	w.RegisterActivity(UpdateDeploymentStatusActivity)

	// Custom workflow activities
	w.RegisterActivity(ExecuteCustomStepActivity)
}

// TemporalLogger adapts zap.Logger to Temporal's logger interface
type TemporalLogger struct {
	logger *zap.Logger
}

// NewTemporalLogger creates a new Temporal logger
func NewTemporalLogger(logger *zap.Logger) *TemporalLogger {
	return &TemporalLogger{logger: logger}
}

// Debug logs at debug level
func (l *TemporalLogger) Debug(msg string, keyvals ...interface{}) {
	l.logger.Debug(msg, l.fieldsFromKeyvals(keyvals)...)
}

// Info logs at info level
func (l *TemporalLogger) Info(msg string, keyvals ...interface{}) {
	l.logger.Info(msg, l.fieldsFromKeyvals(keyvals)...)
}

// Warn logs at warn level
func (l *TemporalLogger) Warn(msg string, keyvals ...interface{}) {
	l.logger.Warn(msg, l.fieldsFromKeyvals(keyvals)...)
}

// Error logs at error level
func (l *TemporalLogger) Error(msg string, keyvals ...interface{}) {
	l.logger.Error(msg, l.fieldsFromKeyvals(keyvals)...)
}

// fieldsFromKeyvals converts keyvals to zap fields
func (l *TemporalLogger) fieldsFromKeyvals(keyvals []interface{}) []zap.Field {
	if len(keyvals)%2 != 0 {
		return []zap.Field{zap.Any("keyvals", keyvals)}
	}

	fields := make([]zap.Field, 0, len(keyvals)/2)
	for i := 0; i < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", keyvals[i])
		}
		fields = append(fields, zap.Any(key, keyvals[i+1]))
	}
	return fields
}

// Placeholder activity functions - these would normally be in separate files

func FetchCodeChangesActivity(ctx context.Context, req CodeReviewRequest) (*CodeChanges, error) {
	return &CodeChanges{
		Files:     []string{"file1.go", "file2.go"},
		Additions: 100,
		Deletions: 50,
		Diff:      "diff content",
	}, nil
}

func RunAutomatedChecksActivity(ctx context.Context, changes CodeChanges) (*AutomatedCheckResults, error) {
	return &AutomatedCheckResults{
		Passed:   true,
		Checks:   []interface{}{},
		Coverage: 0.85,
	}, nil
}

func RunAIReviewActivity(ctx context.Context, changes CodeChanges, checks AutomatedCheckResults) (*AIReviewResult, error) {
	return &AIReviewResult{
		Issues:      []interface{}{},
		Suggestions: []interface{}{},
		CodeQuality: 0.9,
	}, nil
}

func GenerateReviewSummaryActivity(ctx context.Context, checks AutomatedCheckResults, ai AIReviewResult) (*ReviewSummary, error) {
	return &ReviewSummary{
		Approved: true,
		Comments: []interface{}{},
		Score:    0.88,
	}, nil
}

func PostReviewCommentsActivity(ctx context.Context, summary ReviewSummary) error {
	return nil
}

func ValidateDeploymentActivity(ctx context.Context, req DeploymentRequest) (*DeploymentValidation, error) {
	return &DeploymentValidation{
		IsValid: true,
		Errors:  []string{},
	}, nil
}

func BuildArtifactsActivity(ctx context.Context, req DeploymentRequest) (*BuildResult, error) {
	return &BuildResult{
		ArtifactID: "artifact-123",
		Version:    req.Version,
		Size:       1024 * 1024 * 50, // 50MB
	}, nil
}

func RunDeploymentTestsActivity(ctx context.Context, build BuildResult) (*TestResult, error) {
	return &TestResult{
		Passed:   true,
		Tests:    100,
		Failures: 0,
		Coverage: 0.85,
	}, nil
}

func DeployToStagingActivity(ctx context.Context, build BuildResult) (*DeploymentResult, error) {
	return &DeploymentResult{
		DeploymentID: "deploy-staging-123",
		Environment:  "staging",
		Version:      build.Version,
		URL:          "https://staging.example.com",
		Timestamp:    time.Now(),
	}, nil
}

func RunSmokeTestsActivity(ctx context.Context, deployment DeploymentResult) (*TestResult, error) {
	return &TestResult{
		Passed:   true,
		Tests:    20,
		Failures: 0,
		Coverage: 0.0,
	}, nil
}

func DeployToProductionActivity(ctx context.Context, build BuildResult) (*DeploymentResult, error) {
	return &DeploymentResult{
		DeploymentID: "deploy-prod-123",
		Environment:  "production",
		Version:      build.Version,
		URL:          "https://api.example.com",
		Timestamp:    time.Now(),
	}, nil
}

func RunHealthCheckActivity(ctx context.Context, deployment DeploymentResult) (*HealthCheckResult, error) {
	return &HealthCheckResult{
		IsHealthy: true,
		Checks: map[string]bool{
			"api":      true,
			"database": true,
			"cache":    true,
		},
	}, nil
}

func RollbackDeploymentActivity(ctx context.Context, deployment DeploymentResult) error {
	return nil
}

func UpdateDeploymentStatusActivity(ctx context.Context, deployment DeploymentResult) error {
	return nil
}

func ExecuteCustomStepActivity(ctx context.Context, step CustomStep) (interface{}, error) {
	return map[string]interface{}{
		"step":   step.Name,
		"status": "completed",
	}, nil
}

// Type definitions for placeholder activities
type CodeReviewRequest struct {
	Repository    string `json:"repository"`
	Branch        string `json:"branch"`
	CommitHash    string `json:"commit_hash"`
	PostComments  bool   `json:"post_comments"`
}

type CodeChanges struct {
	Files     []string `json:"files"`
	Additions int      `json:"additions"`
	Deletions int      `json:"deletions"`
	Diff      string   `json:"diff"`
}

type AutomatedCheckResults struct {
	Passed   bool          `json:"passed"`
	Checks   []interface{} `json:"checks"`
	Coverage float64       `json:"coverage"`
}

type AIReviewResult struct {
	Issues      []interface{} `json:"issues"`
	Suggestions []interface{} `json:"suggestions"`
	CodeQuality float64       `json:"code_quality"`
}

type ReviewSummary struct {
	Approved bool          `json:"approved"`
	Comments []interface{} `json:"comments"`
	Score    float64       `json:"score"`
}

type DeploymentRequest struct {
	Version         string `json:"version"`
	Environment     string `json:"environment"`
	Repository      string `json:"repository"`
	DeployToStaging bool   `json:"deploy_to_staging"`
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
	Passed   bool    `json:"passed"`
	Tests    int     `json:"tests"`
	Failures int     `json:"failures"`
	Coverage float64 `json:"coverage"`
}

type DeploymentResult struct {
	DeploymentID string    `json:"deployment_id"`
	Environment  string    `json:"environment"`
	Version      string    `json:"version"`
	URL          string    `json:"url"`
	Timestamp    time.Time `json:"timestamp"`
}

type HealthCheckResult struct {
	IsHealthy bool            `json:"is_healthy"`
	Checks    map[string]bool `json:"checks"`
}

type CustomStep struct {
	Name            string                 `json:"name"`
	Config          map[string]interface{} `json:"config"`
	TimeoutSeconds  int                    `json:"timeout_seconds"`
	MaxRetries      int                    `json:"max_retries"`
	ContinueOnError bool                   `json:"continue_on_error"`
}
