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

// MetaAgentActivities handles meta-agent specific workflow activities
type MetaAgentActivities struct {
	agentClient *services.AgentClient
	logger      *zap.Logger
}

// NewMetaAgentActivities creates new meta-agent activities instance
func NewMetaAgentActivities(
	agentClient *services.AgentClient,
	logger *zap.Logger,
) *MetaAgentActivities {
	return &MetaAgentActivities{
		agentClient: agentClient,
		logger:      logger,
	}
}

// FindOrCreateAgentForTaskActivity finds a suitable agent or creates one using meta-agent
func (a *MetaAgentActivities) FindOrCreateAgentForTaskActivity(ctx context.Context, task Task) (*AgentInfo, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Finding or creating agent for task", 
		zap.String("taskID", task.ID),
		zap.String("taskType", task.Type))

	// Step 1: Calculate required capabilities for the task
	requiredCapabilities := a.getRequiredCapabilities(task)
	logger.Info("Required capabilities determined", 
		zap.Strings("capabilities", requiredCapabilities))

	// Step 2: Search for existing suitable agents
	agents, err := a.agentClient.ListAgents(ctx, &services.AgentFilters{
		ProjectID: getProjectIDFromContext(ctx),
		Status:    "available",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Step 3: Find best matching existing agent
	bestAgent := a.findBestMatchingAgent(agents.Agents, requiredCapabilities, a.logger)
	
	// If we found a suitable agent (matching at least 60% of capabilities), use it
	if bestAgent != nil {
		logger.Info("Found suitable existing agent", 
			zap.String("agentID", bestAgent.ID),
			zap.String("agentType", bestAgent.Type))

		capNames := make([]string, len(bestAgent.Capabilities))
		for i, cap := range bestAgent.Capabilities {
			capNames[i] = cap.Name
		}
		
		return &AgentInfo{
			ID:           bestAgent.ID,
			Type:         bestAgent.Type,
			Capabilities: capNames,
			Status:       bestAgent.Status,
		}, nil
	}

	// Step 4: No suitable agent found - use meta-agent to create one
	logger.Info("No suitable agent found, using meta-agent for dynamic creation")

	// Find the meta-prompt agent
	metaAgent := a.findMetaPromptAgent(agents.Agents)
	if metaAgent == nil {
		return nil, fmt.Errorf("meta-prompt agent not available")
	}

	logger.Info("Found meta-prompt agent", zap.String("metaAgentID", metaAgent.ID))

	// Step 5: Request meta-agent to design a new specialized agent
	designTask := a.createAgentDesignTask(task, requiredCapabilities)
	
	designResp, err := a.agentClient.ExecuteTask(ctx, metaAgent.ID, &services.ExecuteTaskRequest{
		Type: "design-agent",
		Input: map[string]interface{}{
			"taskDescription": designTask.Description,
			"requirements": map[string]interface{}{
				"capabilities":         requiredCapabilities,
				"complexity":          task.Complexity,
				"estimated_hours":     task.EstimatedHours,
				"technical_requirements": task.TechnicalRequirements,
			},
			"context": map[string]interface{}{
				"project_id":   getProjectIDFromContext(ctx),
				"task_type":    task.Type,
				"priority":     task.Priority,
			},
		},
		Config: map[string]interface{}{
			"timeout_minutes": 5,
			"response_format": "agent_specification",
		},
		Priority:   "high",
		Timeout:    300, // 5 minutes
		MaxRetries: 2,
	})
	if err != nil {
		// Fallback: Use meta-agent directly if design fails
		logger.Warn("Agent design failed, using meta-agent directly", zap.Error(err))
		metaCapNames := make([]string, len(metaAgent.Capabilities))
		for i, cap := range metaAgent.Capabilities {
			metaCapNames[i] = cap.Name
		}
		
		return &AgentInfo{
			ID:           metaAgent.ID,
			Type:         metaAgent.Type,
			Capabilities: metaCapNames,
			Status:       metaAgent.Status,
		}, nil
	}

	// Step 6: Extract design ID from response
	designID, ok := designResp.Output["designId"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid design response: missing designId")
	}

	logger.Info("Agent design completed", zap.String("designID", designID))

	// Step 7: Spawn the designed agent
	spawnResp, err := a.agentClient.ExecuteTask(ctx, metaAgent.ID, &services.ExecuteTaskRequest{
		Type: "spawn-agent",
		Input: map[string]interface{}{
			"designId": designID,
			"taskContext": map[string]interface{}{
				"task_id":       task.ID,
				"project_id":    getProjectIDFromContext(ctx),
				"priority":      task.Priority,
				"estimated_duration": task.EstimatedHours * 3600, // Convert to seconds
			},
			"ttl": 3600000, // 1 hour TTL for dynamic agents
		},
		Config: map[string]interface{}{
			"timeout_minutes": 5,
			"auto_register":   true,
		},
		Priority:   "high",
		Timeout:    300,
		MaxRetries: 2,
	})
	if err != nil {
		// Fallback: Use meta-agent directly if spawn fails
		logger.Warn("Agent spawn failed, using meta-agent directly", zap.Error(err))
		metaCapNames := make([]string, len(metaAgent.Capabilities))
		for i, cap := range metaAgent.Capabilities {
			metaCapNames[i] = cap.Name
		}
		
		return &AgentInfo{
			ID:           metaAgent.ID,
			Type:         metaAgent.Type,
			Capabilities: metaCapNames,
			Status:       metaAgent.Status,
		}, nil
	}

	// Step 8: Extract spawned agent info
	spawnedAgentID, ok := spawnResp.Output["agentId"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid spawn response: missing agentId")
	}

	logger.Info("Dynamic agent spawned successfully", 
		zap.String("spawnedAgentID", spawnedAgentID),
		zap.String("designID", designID))

	// Step 9: Wait for agent to be ready and get its info
	spawnedAgent, err := a.waitForAgentReady(ctx, spawnedAgentID, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("spawned agent not ready: %w", err)
	}

	// Record heartbeat with success
	activity.RecordHeartbeat(ctx, fmt.Sprintf("Dynamic agent %s ready for task %s", spawnedAgentID, task.ID))

	spawnedCapNames := make([]string, len(spawnedAgent.Capabilities))
	for i, cap := range spawnedAgent.Capabilities {
		spawnedCapNames[i] = cap.Name
	}
	
	return &AgentInfo{
		ID:           spawnedAgent.ID,
		Type:         spawnedAgent.Type,
		Capabilities: spawnedCapNames,
		Status:       spawnedAgent.Status,
	}, nil
}

// ExecuteTaskWithAgentActivity executes a task using the selected/created agent
func (a *MetaAgentActivities) ExecuteTaskWithAgentActivity(ctx context.Context, task Task, agent AgentInfo) (*TaskExecutionResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing task with agent", 
		zap.String("taskID", task.ID),
		zap.String("agentID", agent.ID),
		zap.String("agentType", agent.Type))

	startTime := time.Now()

	// Prepare comprehensive task execution request
	execReq := &services.ExecuteTaskRequest{
		Type: a.mapTaskTypeToAgentAction(task.Type),
		Input: map[string]interface{}{
			"task": map[string]interface{}{
				"id":                    task.ID,
				"title":                 task.Title,
				"description":           task.Description,
				"type":                  task.Type,
				"priority":              task.Priority,
				"complexity":            task.Complexity,
				"estimated_hours":       task.EstimatedHours,
				"acceptance_criteria":   task.AcceptanceCriteria,
				"technical_requirements": task.TechnicalRequirements,
				"dependencies":          task.Dependencies,
				"tags":                  task.Tags,
			},
			"context": map[string]interface{}{
				"project_id":        getProjectIDFromContext(ctx),
				"user_id":           getUserIDFromContext(ctx),
				"workflow_id":       getWorkflowIDFromContext(ctx),
				"execution_context": "orchestrated_workflow",
			},
			"requirements": map[string]interface{}{
				"generate_artifacts": true,
				"include_tests":      shouldGenerateTests(task.Type),
				"include_docs":       shouldGenerateDocs(task.Type),
				"quality_checks":     true,
			},
		},
		Config: map[string]interface{}{
			"timeout_minutes":       int(task.EstimatedHours * 60),
			"max_response_tokens":   4000,
			"enable_streaming":      false,
			"quality_threshold":     0.8,
			"include_explanations":  true,
			"code_style":           getCodeStyle(task),
			"target_framework":     getTargetFramework(task),
			"environment":          getEnvironment(task),
		},
		Priority:   task.Priority,
		Timeout:    int(task.EstimatedHours * 3600), // Convert hours to seconds
		MaxRetries: 2,
	}

	// Execute the task
	logger.Info("Sending task execution request to agent")
	execResp, err := a.agentClient.ExecuteTask(ctx, agent.ID, execReq)
	if err != nil {
		// Create failed result
		result := &TaskExecutionResult{
			TaskID:    task.ID,
			AgentID:   agent.ID,
			Status:    "failed",
			Error:     fmt.Sprintf("Task execution failed: %v", err),
			StartTime: startTime,
			EndTime:   time.Now(),
			Duration:  time.Since(startTime),
		}
		return result, fmt.Errorf("task execution failed: %w", err)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	logger.Info("Task execution completed", 
		zap.String("status", execResp.Status),
		zap.Duration("duration", duration))

	// Process execution response
	result := &TaskExecutionResult{
		TaskID:    task.ID,
		AgentID:   agent.ID,
		Status:    execResp.Status,
		Output:    execResp.Output,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
	}

	// Extract and process artifacts
	if artifactsData, ok := execResp.Output["artifacts"]; ok {
		artifacts := a.extractArtifacts(artifactsData, task.ID)
		result.Artifacts = artifacts
		
		logger.Info("Extracted artifacts from task execution", 
			zap.Int("artifact_count", len(artifacts)))
	}

	// Add execution metadata
	if result.Output == nil {
		result.Output = make(map[string]interface{})
	}
	result.Output["execution_metadata"] = map[string]interface{}{
		"agent_type":        agent.Type,
		"capabilities_used": agent.Capabilities,
		"duration_seconds":  duration.Seconds(),
		"timestamp":         endTime.Unix(),
	}

	// Record success heartbeat
	activity.RecordHeartbeat(ctx, fmt.Sprintf("Task %s completed by agent %s in %.2f seconds", 
		task.ID, agent.ID, duration.Seconds()))

	return result, nil
}

// OptimizeAgentPerformanceActivity monitors and optimizes agent performance
func (a *MetaAgentActivities) OptimizeAgentPerformanceActivity(ctx context.Context, agentID string, executionResults []TaskExecutionResult) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Optimizing agent performance", 
		zap.String("agentID", agentID),
		zap.Int("execution_count", len(executionResults)))

	// Calculate performance metrics
	metrics := a.calculatePerformanceMetrics(executionResults)
	
	// If performance is below threshold, request optimization
	if metrics.AverageScore < 0.7 || metrics.FailureRate > 0.2 {
		logger.Info("Agent performance below threshold, requesting optimization",
			zap.Float64("average_score", metrics.AverageScore),
			zap.Float64("failure_rate", metrics.FailureRate))

		// Find meta-prompt agent for optimization
		agents, err := a.agentClient.ListAgents(ctx, &services.AgentFilters{
			Type:   "meta-prompt",
			Status: "available",
		})
		if err != nil {
			return fmt.Errorf("failed to find meta-prompt agent: %w", err)
		}

		metaAgent := a.findMetaPromptAgent(agents.Agents)
		if metaAgent == nil {
			return fmt.Errorf("meta-prompt agent not available for optimization")
		}

		// Request performance optimization
		_, err = a.agentClient.ExecuteTask(ctx, metaAgent.ID, &services.ExecuteTaskRequest{
			Type: "monitor-performance",
			Input: map[string]interface{}{
				"agentId":   agentID,
				"metrics":   metrics,
				"threshold": 0.7,
				"execution_history": executionResults,
			},
			Config: map[string]interface{}{
				"optimization_type": "performance",
				"include_prompt_optimization": true,
			},
			Priority: "normal",
			Timeout:  300,
		})
		if err != nil {
			logger.Warn("Failed to request agent optimization", zap.Error(err))
		} else {
			logger.Info("Agent optimization requested successfully")
		}
	}

	return nil
}

// Helper functions

func (a *MetaAgentActivities) getRequiredCapabilities(task Task) []string {
	capabilities := []string{}

	// Base capabilities from task type
	switch task.Type {
	case "frontend", "ui", "web":
		capabilities = append(capabilities, "frontend", "ui", "javascript", "typescript", "react", "html", "css")
	case "backend", "api", "server":
		capabilities = append(capabilities, "backend", "api", "server", "database", "rest", "graphql")
	case "database", "data":
		capabilities = append(capabilities, "database", "sql", "nosql", "schema", "migration", "query-optimization")
	case "testing", "qa":
		capabilities = append(capabilities, "testing", "unit-test", "integration-test", "e2e-test", "test-automation")
	case "documentation", "docs":
		capabilities = append(capabilities, "documentation", "technical-writing", "api-docs", "markdown")
	case "devops", "infrastructure":
		capabilities = append(capabilities, "devops", "ci-cd", "deployment", "infrastructure", "docker", "kubernetes")
	case "security", "sec":
		capabilities = append(capabilities, "security", "authentication", "authorization", "encryption", "vulnerability-assessment")
	case "mobile", "app":
		capabilities = append(capabilities, "mobile", "ios", "android", "react-native", "flutter")
	default:
		capabilities = append(capabilities, task.Type, "general-purpose")
	}

	// Add capabilities from tags
	capabilities = append(capabilities, task.Tags...)

	// Add capabilities from technical requirements
	if task.TechnicalRequirements != nil {
		if langs, ok := task.TechnicalRequirements["languages"].([]interface{}); ok {
			for _, lang := range langs {
				if langStr, ok := lang.(string); ok {
					capabilities = append(capabilities, strings.ToLower(langStr))
				}
			}
		}
		
		if frameworks, ok := task.TechnicalRequirements["frameworks"].([]interface{}); ok {
			for _, fw := range frameworks {
				if fwStr, ok := fw.(string); ok {
					capabilities = append(capabilities, strings.ToLower(fwStr))
				}
			}
		}
	}

	// Remove duplicates and return
	return a.removeDuplicates(capabilities)
}

func (a *MetaAgentActivities) findBestMatchingAgent(agents []services.Agent, requiredCapabilities []string, logger *zap.Logger) *services.Agent {
	var bestAgent *services.Agent
	bestScore := 0.0
	minThreshold := 0.6 // Agent must match at least 60% of capabilities

	for _, agent := range agents {
		score := a.calculateAgentMatchScore(agent, requiredCapabilities)
		logger.Debug("Agent capability score calculated", 
			zap.String("agentID", agent.ID),
			zap.String("agentType", agent.Type),
			zap.Float64("score", score),
			zap.Any("agentCapabilities", agent.Capabilities))

		if score >= minThreshold && score > bestScore {
			bestScore = score
			bestAgent = &agent
		}
	}

	if bestAgent != nil {
		logger.Info("Best matching agent found", 
			zap.String("agentID", bestAgent.ID),
			zap.Float64("match_score", bestScore))
	}

	return bestAgent
}

func (a *MetaAgentActivities) calculateAgentMatchScore(agent services.Agent, requiredCapabilities []string) float64 {
	if len(requiredCapabilities) == 0 {
		return 1.0
	}

	agentCaps := make(map[string]bool)
	for _, cap := range agent.Capabilities {
		agentCaps[strings.ToLower(cap.Name)] = true
	}

	matches := 0
	for _, required := range requiredCapabilities {
		if agentCaps[strings.ToLower(required)] {
			matches++
		}
	}

	return float64(matches) / float64(len(requiredCapabilities))
}

func (a *MetaAgentActivities) findMetaPromptAgent(agents []services.Agent) *services.Agent {
	for _, agent := range agents {
		if agent.Type == "meta-prompt" && agent.Status == "available" {
			return &agent
		}
	}
	return nil
}

func (a *MetaAgentActivities) createAgentDesignTask(task Task, capabilities []string) AgentDesignTask {
	return AgentDesignTask{
		Description: fmt.Sprintf(`Design a specialized agent for %s tasks.

Task Details:
- Title: %s
- Description: %s
- Complexity: %s
- Estimated Hours: %.1f

Required Capabilities: %s

The agent should be able to:
1. Understand and implement %s requirements
2. Generate high-quality, maintainable code
3. Follow best practices and design patterns
4. Create comprehensive tests and documentation
5. Provide clear explanations of implementations

Please design an agent specification that can effectively handle this type of work.`,
			task.Type,
			task.Title,
			task.Description,
			task.Complexity,
			task.EstimatedHours,
			strings.Join(capabilities, ", "),
			task.Type),
		RequiredCapabilities: capabilities,
		TaskContext: map[string]interface{}{
			"type":        task.Type,
			"complexity":  task.Complexity,
			"priority":    task.Priority,
			"tags":        task.Tags,
		},
	}
}

func (a *MetaAgentActivities) waitForAgentReady(ctx context.Context, agentID string, timeout time.Duration) (*services.Agent, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for agent %s to be ready", agentID)
		case <-ticker.C:
			agent, err := a.agentClient.GetAgent(ctx, agentID)
			if err == nil && (agent.Status == "available" || agent.Status == "active") {
				return agent, nil
			}
		}
	}
}

func (a *MetaAgentActivities) mapTaskTypeToAgentAction(taskType string) string {
	switch taskType {
	case "frontend", "ui", "web":
		return "generate_frontend_code"
	case "backend", "api", "server":
		return "generate_backend_code"
	case "database", "data":
		return "generate_database_schema"
	case "testing", "qa":
		return "generate_tests"
	case "documentation", "docs":
		return "generate_documentation"
	case "devops", "infrastructure":
		return "generate_infrastructure"
	case "security", "sec":
		return "generate_security_code"
	case "mobile", "app":
		return "generate_mobile_code"
	default:
		return "execute_general_task"
	}
}

func (a *MetaAgentActivities) extractArtifacts(artifactsData interface{}, taskID string) []Artifact {
	artifacts := []Artifact{}

	switch data := artifactsData.(type) {
	case []interface{}:
		for i, artifactData := range data {
			if artifactMap, ok := artifactData.(map[string]interface{}); ok {
				artifact := Artifact{
					ID:          fmt.Sprintf("%s-artifact-%d", taskID, i),
					Name:        fmt.Sprintf("%v", artifactMap["name"]),
					Type:        fmt.Sprintf("%v", artifactMap["type"]),
					Content:     fmt.Sprintf("%v", artifactMap["content"]),
					Path:        fmt.Sprintf("%v", artifactMap["path"]),
					ContentType: getContentType(artifactMap),
					CreatedAt:   time.Now(),
				}

				// Calculate size
				if content, ok := artifactMap["content"].(string); ok {
					artifact.Size = int64(len(content))
				}

				artifacts = append(artifacts, artifact)
			}
		}
	case map[string]interface{}:
		artifact := Artifact{
			ID:          fmt.Sprintf("%s-artifact-0", taskID),
			Name:        fmt.Sprintf("%v", data["name"]),
			Type:        fmt.Sprintf("%v", data["type"]),
			Content:     fmt.Sprintf("%v", data["content"]),
			Path:        fmt.Sprintf("%v", data["path"]),
			ContentType: getContentType(data),
			CreatedAt:   time.Now(),
		}

		if content, ok := data["content"].(string); ok {
			artifact.Size = int64(len(content))
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts
}

func (a *MetaAgentActivities) calculatePerformanceMetrics(results []TaskExecutionResult) PerformanceMetrics {
	if len(results) == 0 {
		return PerformanceMetrics{}
	}

	totalDuration := time.Duration(0)
	successCount := 0
	totalScore := 0.0

	for _, result := range results {
		totalDuration += result.Duration
		
		if result.Status == "completed" || result.Status == "succeeded" {
			successCount++
			totalScore += 1.0 // Could be more sophisticated scoring
		}
	}

	return PerformanceMetrics{
		AverageScore:      totalScore / float64(len(results)),
		FailureRate:       float64(len(results)-successCount) / float64(len(results)),
		AverageDuration:   totalDuration / time.Duration(len(results)),
		TotalExecutions:   len(results),
		SuccessfulExecutions: successCount,
	}
}

func (a *MetaAgentActivities) removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

func getContentType(artifactMap map[string]interface{}) string {
	if ct, ok := artifactMap["content_type"].(string); ok {
		return ct
	}
	
	// Infer from file extension or type
	if typ, ok := artifactMap["type"].(string); ok {
		switch typ {
		case "code":
			return "text/plain"
		case "json":
			return "application/json"
		case "yaml":
			return "application/yaml"
		case "markdown":
			return "text/markdown"
		default:
			return "text/plain"
		}
	}
	
	return "application/octet-stream"
}

func getCodeStyle(task Task) string {
	if task.TechnicalRequirements != nil {
		if style, ok := task.TechnicalRequirements["code_style"].(string); ok {
			return style
		}
	}
	return "standard"
}

func getTargetFramework(task Task) string {
	if task.TechnicalRequirements != nil {
		if framework, ok := task.TechnicalRequirements["framework"].(string); ok {
			return framework
		}
		if frameworks, ok := task.TechnicalRequirements["frameworks"].([]interface{}); ok && len(frameworks) > 0 {
			if fw, ok := frameworks[0].(string); ok {
				return fw
			}
		}
	}
	
	// Default frameworks by task type
	switch task.Type {
	case "frontend":
		return "react"
	case "backend":
		return "gin"
	case "mobile":
		return "react-native"
	default:
		return "standard"
	}
}

func getEnvironment(task Task) string {
	if task.TechnicalRequirements != nil {
		if env, ok := task.TechnicalRequirements["environment"].(string); ok {
			return env
		}
	}
	return "development"
}

func getWorkflowIDFromContext(ctx context.Context) string {
	if val := ctx.Value("workflow_id"); val != nil {
		if workflowID, ok := val.(string); ok {
			return workflowID
		}
	}
	return ""
}

// Types for meta-agent activities

type AgentDesignTask struct {
	Description          string                 `json:"description"`
	RequiredCapabilities []string               `json:"required_capabilities"`
	TaskContext          map[string]interface{} `json:"task_context"`
}

type PerformanceMetrics struct {
	AverageScore         float64       `json:"average_score"`
	FailureRate          float64       `json:"failure_rate"`
	AverageDuration      time.Duration `json:"average_duration"`
	TotalExecutions      int           `json:"total_executions"`
	SuccessfulExecutions int           `json:"successful_executions"`
}
