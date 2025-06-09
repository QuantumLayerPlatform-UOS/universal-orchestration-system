package temporal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.uber.org/zap"

	"orchestrator/internal/services"
)

// FindOrCreateAgentForTaskActivity finds a suitable agent or requests creation of a new one
func (a *Activities) FindOrCreateAgentForTaskActivity(ctx context.Context, task Task) (*AgentInfo, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Finding agent for task", 
		zap.String("taskID", task.ID),
		zap.String("taskType", task.Type))

	// Map task types to agent capabilities
	requiredCapabilities := a.getRequiredCapabilities(task)
	logger.Info("Required capabilities", zap.Strings("capabilities", requiredCapabilities))

	// First, try to find an existing agent with required capabilities
	agents, err := a.agentClient.ListAgents(ctx, &services.AgentFilters{
		Status: "available",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Find best matching agent
	var bestAgent *services.Agent
	bestScore := 0

	for _, agent := range agents.Agents {
		score := a.calculateAgentScore(agent, requiredCapabilities)
		logger.Debug("Agent capability score", 
			zap.String("agentID", agent.ID),
			zap.Int("score", score),
			zap.Any("agentCapabilities", agent.Capabilities))

		if score > bestScore {
			bestScore = score
			bestAgent = &agent
		}
	}

	// If we found a suitable agent (matching at least 50% of capabilities)
	if bestAgent != nil && bestScore >= len(requiredCapabilities)/2 {
		logger.Info("Found suitable existing agent", 
			zap.String("agentID", bestAgent.ID),
			zap.Int("score", bestScore))

		bestCapNames := make([]string, len(bestAgent.Capabilities))
		for i, cap := range bestAgent.Capabilities {
			bestCapNames[i] = cap.Name
		}
		
		return &AgentInfo{
			ID:           bestAgent.ID,
			Type:         bestAgent.Type,
			Capabilities: bestCapNames,
			Status:       bestAgent.Status,
		}, nil
	}

	// No suitable agent found - request dynamic agent creation
	logger.Info("No suitable agent found, requesting dynamic agent creation")

	// Create agent specification based on task requirements
	agentSpec := a.createAgentSpec(task, requiredCapabilities)

	// Request meta-prompt agent to create a new specialized agent
	createResp, err := a.agentClient.CreateAgent(ctx, &services.CreateAgentRequest{
		Name:         fmt.Sprintf("%s-agent-%s", task.Type, task.ID[:8]),
		Type:         "dynamic",
		ProjectID:    getProjectIDFromContext(ctx),
		Capabilities: requiredCapabilities,
		Config: map[string]interface{}{
			"task_type":        task.Type,
			"task_description": task.Description,
			"complexity":       task.Complexity,
			"spec":             agentSpec,
		},
	})
	if err != nil {
		// If dynamic creation fails, try to use meta-prompt agent directly
		logger.Warn("Dynamic agent creation failed, falling back to meta-prompt agent", zap.Error(err))
		
		// Find meta-prompt agent
		for _, agent := range agents.Agents {
			if agent.Type == "meta-prompt" {
				agentCapNames := make([]string, len(agent.Capabilities))
				for i, cap := range agent.Capabilities {
					agentCapNames[i] = cap.Name
				}
				
				return &AgentInfo{
					ID:           agent.ID,
					Type:         agent.Type,
					Capabilities: agentCapNames,
					Status:       agent.Status,
				}, nil
			}
		}

		return nil, fmt.Errorf("failed to create agent and no fallback available: %w", err)
	}

	createCapNames := make([]string, len(createResp.Capabilities))
	for i, cap := range createResp.Capabilities {
		createCapNames[i] = cap.Name
	}
	
	return &AgentInfo{
		ID:           createResp.ID,
		Type:         createResp.Type,
		Capabilities: createCapNames,
		Status:       createResp.Status,
	}, nil
}

// ExecuteTaskWithAgentActivity executes a task using the selected agent
func (a *Activities) ExecuteTaskWithAgentActivity(ctx context.Context, task Task, agent AgentInfo) (*TaskExecutionResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing task with agent", 
		zap.String("taskID", task.ID),
		zap.String("agentID", agent.ID))

	startTime := time.Now()

	// Prepare task execution request
	execReq := &services.ExecuteTaskRequest{
		Type: task.Type,
		Input: map[string]interface{}{
			"task_id":               task.ID,
			"title":                 task.Title,
			"description":           task.Description,
			"acceptance_criteria":   task.AcceptanceCriteria,
			"technical_requirements": task.TechnicalRequirements,
			"context": map[string]interface{}{
				"project_id": getProjectIDFromContext(ctx),
				"priority":   task.Priority,
				"complexity": task.Complexity,
			},
		},
		Config: map[string]interface{}{
			"timeout_minutes":  int(task.EstimatedHours * 60),
			"generate_tests":   shouldGenerateTests(task.Type),
			"generate_docs":    shouldGenerateDocs(task.Type),
			"code_style":       "standard",
			"target_language":  getTargetLanguage(task),
		},
		Priority:   task.Priority,
		Timeout:    int(task.EstimatedHours * 3600), // Convert hours to seconds
		MaxRetries: 2,
	}

	// Execute task
	execResp, err := a.agentClient.ExecuteTask(ctx, agent.ID, execReq)
	if err != nil {
		return &TaskExecutionResult{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			Status:    "failed",
			Error:     err.Error(),
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
		}, err
	}

	// Process execution response
	result := &TaskExecutionResult{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		Status:    execResp.Status,
		Output:    execResp.Output,
		StartTime: startTime,
		EndTime:   time.Now(),
		Duration:  time.Since(startTime),
	}

	// Extract artifacts from output
	if artifactsData, ok := execResp.Output["artifacts"]; ok {
		if artifacts, ok := artifactsData.([]interface{}); ok {
			for _, artifactData := range artifacts {
				if artifactMap, ok := artifactData.(map[string]interface{}); ok {
					artifact := Artifact{
						ID:          fmt.Sprintf("%s-%s", task.ID, artifactMap["name"]),
						Name:        fmt.Sprintf("%v", artifactMap["name"]),
						Type:        fmt.Sprintf("%v", artifactMap["type"]),
						Content:     fmt.Sprintf("%v", artifactMap["content"]),
						Path:        fmt.Sprintf("%v", artifactMap["path"]),
						ContentType: fmt.Sprintf("%v", artifactMap["content_type"]),
						CreatedAt:   time.Now(),
					}
					
					// Calculate size if content is provided
					if content, ok := artifactMap["content"].(string); ok {
						artifact.Size = int64(len(content))
					}
					
					result.Artifacts = append(result.Artifacts, artifact)
				}
			}
		}
	}

	// Record heartbeat with progress
	activity.RecordHeartbeat(ctx, fmt.Sprintf("Task %s completed by agent %s", task.ID, agent.ID))

	return result, nil
}

// AggregateTaskResultsActivity aggregates results from all task executions
func (a *Activities) AggregateTaskResultsActivity(ctx context.Context, results []TaskExecutionResult) (*AggregatedTaskResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Aggregating task results", zap.Int("count", len(results)))

	aggregated := &AggregatedTaskResult{
		TotalTasks:  len(results),
		TaskResults: results,
		Artifacts:   []Artifact{},
		Metadata:    make(map[string]interface{}),
	}

	// Count successes and failures
	for _, result := range results {
		if result.Status == "completed" || result.Status == "succeeded" {
			aggregated.SuccessfulTasks++
		} else {
			aggregated.FailedTasks++
		}

		// Collect all artifacts
		aggregated.Artifacts = append(aggregated.Artifacts, result.Artifacts...)
	}

	// Determine overall status
	if aggregated.FailedTasks == 0 {
		aggregated.Status = "completed"
		aggregated.Summary = fmt.Sprintf("All %d tasks completed successfully", aggregated.TotalTasks)
	} else if aggregated.SuccessfulTasks == 0 {
		aggregated.Status = "failed"
		aggregated.Summary = fmt.Sprintf("All %d tasks failed", aggregated.TotalTasks)
	} else {
		aggregated.Status = "partial_success"
		aggregated.Summary = fmt.Sprintf("%d of %d tasks completed successfully", 
			aggregated.SuccessfulTasks, aggregated.TotalTasks)
	}

	// Add metadata
	aggregated.Metadata["total_artifacts"] = len(aggregated.Artifacts)
	aggregated.Metadata["completion_rate"] = float64(aggregated.SuccessfulTasks) / float64(aggregated.TotalTasks)

	return aggregated, nil
}

// StoreArtifactsActivity stores generated artifacts
func (a *Activities) StoreArtifactsActivity(ctx context.Context, projectID string, artifacts []Artifact) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Storing artifacts", 
		zap.String("projectID", projectID),
		zap.Int("count", len(artifacts)))

	// This would typically store artifacts in a storage service
	// For now, we'll just log them
	for _, artifact := range artifacts {
		logger.Info("Storing artifact",
			zap.String("id", artifact.ID),
			zap.String("name", artifact.Name),
			zap.String("type", artifact.Type),
			zap.Int64("size", artifact.Size))
		
		// TODO: Implement actual artifact storage
		// - Store in object storage (S3, Azure Blob, etc.)
		// - Save metadata in database
		// - Generate download URLs
	}

	activity.RecordHeartbeat(ctx, fmt.Sprintf("Stored %d artifacts", len(artifacts)))
	return nil
}

// Helper functions

func (a *Activities) getRequiredCapabilities(task Task) []string {
	capabilities := []string{}

	// Map task type to capabilities
	switch task.Type {
	case "frontend":
		capabilities = append(capabilities, "frontend", "ui", "javascript", "react", "html", "css")
	case "backend":
		capabilities = append(capabilities, "backend", "api", "database", "server")
	case "api":
		capabilities = append(capabilities, "api", "rest", "graphql", "openapi")
	case "database":
		capabilities = append(capabilities, "database", "sql", "nosql", "schema")
	case "testing":
		capabilities = append(capabilities, "testing", "unit-test", "integration-test", "e2e-test")
	case "documentation":
		capabilities = append(capabilities, "documentation", "technical-writing", "api-docs")
	case "devops":
		capabilities = append(capabilities, "devops", "ci-cd", "deployment", "infrastructure")
	case "security":
		capabilities = append(capabilities, "security", "authentication", "authorization", "encryption")
	default:
		capabilities = append(capabilities, task.Type)
	}

	// Add capabilities from tags
	capabilities = append(capabilities, task.Tags...)

	// Add capabilities from technical requirements
	if task.TechnicalRequirements != nil {
		if langs, ok := task.TechnicalRequirements["languages"].([]string); ok {
			capabilities = append(capabilities, langs...)
		}
		if frameworks, ok := task.TechnicalRequirements["frameworks"].([]string); ok {
			capabilities = append(capabilities, frameworks...)
		}
	}

	return capabilities
}

func (a *Activities) calculateAgentScore(agent services.Agent, requiredCapabilities []string) int {
	score := 0
	agentCaps := make(map[string]bool)

	// Create a map for faster lookup
	for _, cap := range agent.Capabilities {
		agentCaps[strings.ToLower(cap.Name)] = true
	}

	// Count matching capabilities
	for _, required := range requiredCapabilities {
		if agentCaps[strings.ToLower(required)] {
			score++
		}
	}

	return score
}

func (a *Activities) createAgentSpec(task Task, capabilities []string) map[string]interface{} {
	return map[string]interface{}{
		"name":         fmt.Sprintf("%s-specialist", task.Type),
		"description":  fmt.Sprintf("Specialized agent for %s tasks", task.Type),
		"capabilities": capabilities,
		"system_prompt": fmt.Sprintf(`You are a specialized %s agent capable of:
- Understanding and implementing %s requirements
- Writing high-quality, maintainable code
- Following best practices and design patterns
- Generating comprehensive tests and documentation
- Providing clear explanations of your implementations

Task context: %s`, task.Type, task.Type, task.Description),
		"tools": getToolsForTaskType(task.Type),
		"config": map[string]interface{}{
			"temperature":     0.7,
			"max_tokens":      4000,
			"response_format": "structured",
		},
	}
}

func getToolsForTaskType(taskType string) []string {
	baseTools := []string{"code_generator", "test_generator", "documentation_generator"}

	switch taskType {
	case "frontend":
		return append(baseTools, "ui_component_generator", "style_generator", "accessibility_checker")
	case "backend":
		return append(baseTools, "api_generator", "database_schema_generator", "validation_generator")
	case "api":
		return append(baseTools, "openapi_generator", "endpoint_generator", "request_validator")
	case "database":
		return append(baseTools, "schema_generator", "migration_generator", "query_optimizer")
	case "testing":
		return []string{"unit_test_generator", "integration_test_generator", "test_data_generator", "mock_generator"}
	case "devops":
		return append(baseTools, "dockerfile_generator", "ci_pipeline_generator", "k8s_manifest_generator")
	default:
		return baseTools
	}
}

func shouldGenerateTests(taskType string) bool {
	// Generate tests for most task types except documentation
	return taskType != "documentation"
}

func shouldGenerateDocs(taskType string) bool {
	// Always generate documentation
	return true
}

func getTargetLanguage(task Task) string {
	// Extract target language from technical requirements
	if task.TechnicalRequirements != nil {
		if lang, ok := task.TechnicalRequirements["primary_language"].(string); ok {
			return lang
		}
		if langs, ok := task.TechnicalRequirements["languages"].([]string); ok && len(langs) > 0 {
			return langs[0]
		}
	}

	// Default languages based on task type
	switch task.Type {
	case "frontend":
		return "typescript"
	case "backend", "api":
		return "go"
	default:
		return "python"
	}
}