package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// WorkflowStatus represents the status of a workflow
type WorkflowStatus string

const (
	WorkflowStatusPending    WorkflowStatus = "pending"
	WorkflowStatusRunning    WorkflowStatus = "running"
	WorkflowStatusCompleted  WorkflowStatus = "completed"
	WorkflowStatusFailed     WorkflowStatus = "failed"
	WorkflowStatusCancelled  WorkflowStatus = "cancelled"
	WorkflowStatusTerminated WorkflowStatus = "terminated"
	WorkflowStatusTimedOut   WorkflowStatus = "timed_out"
	WorkflowStatusPaused     WorkflowStatus = "paused"
)

// WorkflowType represents different types of workflows
type WorkflowType string

const (
	WorkflowTypeIntent       WorkflowType = "intent_processing"
	WorkflowTypeExecution    WorkflowType = "code_execution"
	WorkflowTypeAnalysis     WorkflowType = "code_analysis"
	WorkflowTypeReview       WorkflowType = "code_review"
	WorkflowTypeDeployment   WorkflowType = "deployment"
	WorkflowTypeTaskExecution WorkflowType = "task_execution"
	WorkflowTypeCustom       WorkflowType = "custom"
)

// WorkflowPriority represents the priority of a workflow
type WorkflowPriority string

const (
	WorkflowPriorityLow      WorkflowPriority = "low"
	WorkflowPriorityMedium   WorkflowPriority = "medium"
	WorkflowPriorityHigh     WorkflowPriority = "high"
	WorkflowPriorityCritical WorkflowPriority = "critical"
)

// Workflow represents a workflow definition
type Workflow struct {
	ID               string           `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name             string           `gorm:"not null" json:"name"`
	Description      string           `json:"description"`
	Type             WorkflowType     `gorm:"not null" json:"type"`
	Priority         WorkflowPriority `gorm:"default:'medium'" json:"priority"`
	ProjectID        string           `gorm:"type:uuid;index" json:"project_id"`
	TemporalID       string           `gorm:"index" json:"temporal_id,omitempty"`
	TemporalRunID    string           `json:"temporal_run_id,omitempty"`
	Status           WorkflowStatus   `gorm:"default:'pending';index" json:"status"`
	Input            json.RawMessage  `gorm:"type:jsonb" json:"input,omitempty"`
	Output           json.RawMessage  `gorm:"type:jsonb" json:"output,omitempty"`
	Metadata         json.RawMessage  `gorm:"type:jsonb" json:"metadata,omitempty"`
	Config           json.RawMessage  `gorm:"type:jsonb" json:"config,omitempty"`
	Error            string           `json:"error,omitempty"`
	StartedAt        *time.Time       `json:"started_at,omitempty"`
	CompletedAt      *time.Time       `json:"completed_at,omitempty"`
	Duration         int64            `json:"duration,omitempty"` // Duration in seconds
	RetryCount       int              `gorm:"default:0" json:"retry_count"`
	MaxRetries       int              `gorm:"default:3" json:"max_retries"`
	TimeoutSeconds   int              `gorm:"default:3600" json:"timeout_seconds"`
	ParentWorkflowID *string          `gorm:"type:uuid" json:"parent_workflow_id,omitempty"`
	CreatedBy        string           `json:"created_by"`
	UpdatedBy        string           `json:"updated_by"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	DeletedAt        gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Project    *Project    `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Executions []Execution `gorm:"foreignKey:WorkflowID" json:"executions,omitempty"`
	Steps      []WorkflowStep `gorm:"foreignKey:WorkflowID" json:"steps,omitempty"`
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	ID              string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	WorkflowID      string          `gorm:"type:uuid;not null;index" json:"workflow_id"`
	Name            string          `gorm:"not null" json:"name"`
	Type            string          `gorm:"not null" json:"type"`
	Order           int             `gorm:"not null" json:"order"`
	Status          WorkflowStatus  `gorm:"default:'pending'" json:"status"`
	Input           json.RawMessage `gorm:"type:jsonb" json:"input,omitempty"`
	Output          json.RawMessage `gorm:"type:jsonb" json:"output,omitempty"`
	Config          json.RawMessage `gorm:"type:jsonb" json:"config,omitempty"`
	Error           string          `json:"error,omitempty"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	Duration        int64           `json:"duration,omitempty"` // Duration in milliseconds
	RetryCount      int             `gorm:"default:0" json:"retry_count"`
	MaxRetries      int             `gorm:"default:3" json:"max_retries"`
	TimeoutSeconds  int             `gorm:"default:300" json:"timeout_seconds"`
	DependsOn       []string        `gorm:"type:text[]" json:"depends_on,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`

	// Relationships
	Workflow *Workflow `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
}

// WorkflowTemplate represents a reusable workflow template
type WorkflowTemplate struct {
	ID          string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name        string          `gorm:"not null;unique" json:"name"`
	Description string          `json:"description"`
	Type        WorkflowType    `gorm:"not null" json:"type"`
	Version     string          `gorm:"not null" json:"version"`
	Schema      json.RawMessage `gorm:"type:jsonb" json:"schema"`
	Config      json.RawMessage `gorm:"type:jsonb" json:"config"`
	Steps       json.RawMessage `gorm:"type:jsonb" json:"steps"`
	Variables   json.RawMessage `gorm:"type:jsonb" json:"variables,omitempty"`
	Tags        []string        `gorm:"type:text[]" json:"tags,omitempty"`
	IsActive    bool            `gorm:"default:true" json:"is_active"`
	IsPublic    bool            `gorm:"default:false" json:"is_public"`
	CreatedBy   string          `json:"created_by"`
	UpdatedBy   string          `json:"updated_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

// WorkflowExecution represents a workflow execution history
type WorkflowExecution struct {
	ID             string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	WorkflowID     string          `gorm:"type:uuid;not null;index" json:"workflow_id"`
	ExecutionID    string          `gorm:"not null;unique;index" json:"execution_id"`
	Status         WorkflowStatus  `gorm:"not null" json:"status"`
	Input          json.RawMessage `gorm:"type:jsonb" json:"input,omitempty"`
	Output         json.RawMessage `gorm:"type:jsonb" json:"output,omitempty"`
	Events         json.RawMessage `gorm:"type:jsonb" json:"events,omitempty"`
	Metrics        json.RawMessage `gorm:"type:jsonb" json:"metrics,omitempty"`
	Error          string          `json:"error,omitempty"`
	StartedAt      time.Time       `json:"started_at"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	Duration       int64           `json:"duration,omitempty"` // Duration in milliseconds
	RetryCount     int             `gorm:"default:0" json:"retry_count"`
	ResourceUsage  json.RawMessage `gorm:"type:jsonb" json:"resource_usage,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`

	// Relationships
	Workflow *Workflow `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
}

// TableName specifies the table name for Workflow
func (Workflow) TableName() string {
	return "workflows"
}

// TableName specifies the table name for WorkflowStep
func (WorkflowStep) TableName() string {
	return "workflow_steps"
}

// TableName specifies the table name for WorkflowTemplate
func (WorkflowTemplate) TableName() string {
	return "workflow_templates"
}

// TableName specifies the table name for WorkflowExecution
func (WorkflowExecution) TableName() string {
	return "workflow_executions"
}

// BeforeCreate hook to set creation timestamp
func (w *Workflow) BeforeCreate(tx *gorm.DB) error {
	if w.Status == "" {
		w.Status = WorkflowStatusPending
	}
	if w.Priority == "" {
		w.Priority = WorkflowPriorityMedium
	}
	return nil
}

// BeforeUpdate hook to calculate duration
func (w *Workflow) BeforeUpdate(tx *gorm.DB) error {
	if w.StartedAt != nil && w.CompletedAt != nil {
		w.Duration = int64(w.CompletedAt.Sub(*w.StartedAt).Seconds())
	}
	return nil
}

// IsTerminal returns true if the workflow is in a terminal state
func (w *Workflow) IsTerminal() bool {
	return w.Status == WorkflowStatusCompleted ||
		w.Status == WorkflowStatusFailed ||
		w.Status == WorkflowStatusCancelled ||
		w.Status == WorkflowStatusTerminated ||
		w.Status == WorkflowStatusTimedOut
}

// CanRetry returns true if the workflow can be retried
func (w *Workflow) CanRetry() bool {
	return (w.Status == WorkflowStatusFailed || w.Status == WorkflowStatusTimedOut) &&
		w.RetryCount < w.MaxRetries
}

// GetDurationString returns a human-readable duration string
func (w *Workflow) GetDurationString() string {
	if w.Duration == 0 {
		return "0s"
	}
	return time.Duration(w.Duration * int64(time.Second)).String()
}