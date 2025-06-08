package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ExecutionStatus represents the status of an execution
type ExecutionStatus string

const (
	ExecutionStatusPending    ExecutionStatus = "pending"
	ExecutionStatusQueued     ExecutionStatus = "queued"
	ExecutionStatusRunning    ExecutionStatus = "running"
	ExecutionStatusSucceeded  ExecutionStatus = "succeeded"
	ExecutionStatusFailed     ExecutionStatus = "failed"
	ExecutionStatusCancelled  ExecutionStatus = "cancelled"
	ExecutionStatusTimedOut   ExecutionStatus = "timed_out"
	ExecutionStatusSkipped    ExecutionStatus = "skipped"
	ExecutionStatusRetrying   ExecutionStatus = "retrying"
)

// ExecutionType represents different types of executions
type ExecutionType string

const (
	ExecutionTypeCode       ExecutionType = "code"
	ExecutionTypeScript     ExecutionType = "script"
	ExecutionTypeContainer  ExecutionType = "container"
	ExecutionTypeFunction   ExecutionType = "function"
	ExecutionTypeAPI        ExecutionType = "api"
	ExecutionTypeQuery      ExecutionType = "query"
	ExecutionTypePipeline   ExecutionType = "pipeline"
	ExecutionTypeCustom     ExecutionType = "custom"
)

// Execution represents a code or task execution
type Execution struct {
	ID               string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ProjectID        string          `gorm:"type:uuid;not null;index" json:"project_id"`
	WorkflowID       string          `gorm:"type:uuid;index" json:"workflow_id,omitempty"`
	WorkflowStepID   string          `gorm:"type:uuid;index" json:"workflow_step_id,omitempty"`
	AgentID          string          `gorm:"index" json:"agent_id,omitempty"`
	Name             string          `gorm:"not null" json:"name"`
	Type             ExecutionType   `gorm:"not null" json:"type"`
	Status           ExecutionStatus `gorm:"not null;default:'pending';index" json:"status"`
	Language         string          `json:"language,omitempty"`
	Runtime          string          `json:"runtime,omitempty"`
	Code             string          `gorm:"type:text" json:"code,omitempty"`
	Script           string          `gorm:"type:text" json:"script,omitempty"`
	Command          string          `json:"command,omitempty"`
	Arguments        []string        `gorm:"type:text[]" json:"arguments,omitempty"`
	Environment      json.RawMessage `gorm:"type:jsonb" json:"environment,omitempty"`
	Input            json.RawMessage `gorm:"type:jsonb" json:"input,omitempty"`
	Output           json.RawMessage `gorm:"type:jsonb" json:"output,omitempty"`
	Logs             string          `gorm:"type:text" json:"logs,omitempty"`
	Error            string          `gorm:"type:text" json:"error,omitempty"`
	ExitCode         *int            `json:"exit_code,omitempty"`
	StartedAt        *time.Time      `json:"started_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	Duration         int64           `json:"duration,omitempty"` // Duration in milliseconds
	TimeoutSeconds   int             `gorm:"default:300" json:"timeout_seconds"`
	RetryCount       int             `gorm:"default:0" json:"retry_count"`
	MaxRetries       int             `gorm:"default:3" json:"max_retries"`
	RetryDelay       int             `gorm:"default:1000" json:"retry_delay"` // Delay in milliseconds
	ResourceUsage    json.RawMessage `gorm:"type:jsonb" json:"resource_usage,omitempty"`
	Metadata         json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	Tags             []string        `gorm:"type:text[]" json:"tags,omitempty"`
	Priority         int             `gorm:"default:0" json:"priority"`
	QueuedAt         *time.Time      `json:"queued_at,omitempty"`
	ScheduledAt      *time.Time      `json:"scheduled_at,omitempty"`
	CreatedBy        string          `json:"created_by"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Project      *Project      `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Workflow     *Workflow     `gorm:"foreignKey:WorkflowID" json:"workflow,omitempty"`
	WorkflowStep *WorkflowStep `gorm:"foreignKey:WorkflowStepID" json:"workflow_step,omitempty"`
	Artifacts    []Artifact    `gorm:"foreignKey:ExecutionID" json:"artifacts,omitempty"`
	Metrics      []Metric      `gorm:"foreignKey:ExecutionID" json:"metrics,omitempty"`
}

// Artifact represents an artifact produced by an execution
type Artifact struct {
	ID           string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExecutionID  string          `gorm:"type:uuid;not null;index" json:"execution_id"`
	Name         string          `gorm:"not null" json:"name"`
	Type         string          `gorm:"not null" json:"type"`
	Path         string          `json:"path"`
	URL          string          `json:"url,omitempty"`
	Size         int64           `json:"size"`
	Checksum     string          `json:"checksum,omitempty"`
	ContentType  string          `json:"content_type,omitempty"`
	Metadata     json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	DeletedAt    gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Execution *Execution `gorm:"foreignKey:ExecutionID" json:"execution,omitempty"`
}

// Metric represents a metric collected during execution
type Metric struct {
	ID          string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExecutionID string          `gorm:"type:uuid;not null;index" json:"execution_id"`
	Name        string          `gorm:"not null;index" json:"name"`
	Value       float64         `gorm:"not null" json:"value"`
	Unit        string          `json:"unit,omitempty"`
	Type        string          `gorm:"default:'gauge'" json:"type"`
	Tags        json.RawMessage `gorm:"type:jsonb" json:"tags,omitempty"`
	Timestamp   time.Time       `gorm:"not null;index" json:"timestamp"`
	CreatedAt   time.Time       `json:"created_at"`

	// Relationships
	Execution *Execution `gorm:"foreignKey:ExecutionID" json:"execution,omitempty"`
}

// ExecutionLog represents a log entry for an execution
type ExecutionLog struct {
	ID          string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExecutionID string         `gorm:"type:uuid;not null;index" json:"execution_id"`
	Level       string         `gorm:"not null;default:'info'" json:"level"`
	Message     string         `gorm:"type:text;not null" json:"message"`
	Source      string         `json:"source,omitempty"`
	LineNumber  int            `json:"line_number,omitempty"`
	Metadata    json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	Timestamp   time.Time      `gorm:"not null;index" json:"timestamp"`
	CreatedAt   time.Time      `json:"created_at"`

	// Relationships
	Execution *Execution `gorm:"foreignKey:ExecutionID" json:"execution,omitempty"`
}

// ExecutionEvent represents an event that occurred during execution
type ExecutionEvent struct {
	ID          string          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExecutionID string          `gorm:"type:uuid;not null;index" json:"execution_id"`
	Type        string          `gorm:"not null" json:"type"`
	Name        string          `gorm:"not null" json:"name"`
	Data        json.RawMessage `gorm:"type:jsonb" json:"data,omitempty"`
	Timestamp   time.Time       `gorm:"not null;index" json:"timestamp"`
	CreatedAt   time.Time       `json:"created_at"`

	// Relationships
	Execution *Execution `gorm:"foreignKey:ExecutionID" json:"execution,omitempty"`
}

// ResourceUsage represents resource usage during execution
type ResourceUsage struct {
	CPUUsage         float64 `json:"cpu_usage"`          // CPU usage percentage
	MemoryUsage      int64   `json:"memory_usage"`       // Memory usage in bytes
	MemoryLimit      int64   `json:"memory_limit"`       // Memory limit in bytes
	NetworkRxBytes   int64   `json:"network_rx_bytes"`   // Network received bytes
	NetworkTxBytes   int64   `json:"network_tx_bytes"`   // Network transmitted bytes
	DiskReadBytes    int64   `json:"disk_read_bytes"`    // Disk read bytes
	DiskWriteBytes   int64   `json:"disk_write_bytes"`   // Disk write bytes
	GPUUsage         float64 `json:"gpu_usage,omitempty"` // GPU usage percentage
	GPUMemoryUsage   int64   `json:"gpu_memory_usage,omitempty"` // GPU memory usage in bytes
}

// TableName specifies the table name for Execution
func (Execution) TableName() string {
	return "executions"
}

// TableName specifies the table name for Artifact
func (Artifact) TableName() string {
	return "artifacts"
}

// TableName specifies the table name for Metric
func (Metric) TableName() string {
	return "metrics"
}

// TableName specifies the table name for ExecutionLog
func (ExecutionLog) TableName() string {
	return "execution_logs"
}

// TableName specifies the table name for ExecutionEvent
func (ExecutionEvent) TableName() string {
	return "execution_events"
}

// BeforeCreate hook to set default values
func (e *Execution) BeforeCreate(tx *gorm.DB) error {
	if e.Status == "" {
		e.Status = ExecutionStatusPending
	}
	if e.TimeoutSeconds == 0 {
		e.TimeoutSeconds = 300
	}
	if e.MaxRetries == 0 {
		e.MaxRetries = 3
	}
	if e.RetryDelay == 0 {
		e.RetryDelay = 1000
	}
	return nil
}

// BeforeUpdate hook to calculate duration
func (e *Execution) BeforeUpdate(tx *gorm.DB) error {
	if e.StartedAt != nil && e.CompletedAt != nil {
		e.Duration = int64(e.CompletedAt.Sub(*e.StartedAt).Milliseconds())
	}
	return nil
}

// IsTerminal returns true if the execution is in a terminal state
func (e *Execution) IsTerminal() bool {
	return e.Status == ExecutionStatusSucceeded ||
		e.Status == ExecutionStatusFailed ||
		e.Status == ExecutionStatusCancelled ||
		e.Status == ExecutionStatusTimedOut ||
		e.Status == ExecutionStatusSkipped
}

// CanRetry returns true if the execution can be retried
func (e *Execution) CanRetry() bool {
	return (e.Status == ExecutionStatusFailed || e.Status == ExecutionStatusTimedOut) &&
		e.RetryCount < e.MaxRetries
}

// IsRunning returns true if the execution is currently running
func (e *Execution) IsRunning() bool {
	return e.Status == ExecutionStatusRunning || e.Status == ExecutionStatusRetrying
}

// GetDurationString returns a human-readable duration string
func (e *Execution) GetDurationString() string {
	if e.Duration == 0 {
		return "0ms"
	}
	return time.Duration(e.Duration * int64(time.Millisecond)).String()
}

// SetResourceUsage sets the resource usage data
func (e *Execution) SetResourceUsage(usage ResourceUsage) error {
	data, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	e.ResourceUsage = data
	return nil
}

// GetResourceUsage retrieves the resource usage data
func (e *Execution) GetResourceUsage() (*ResourceUsage, error) {
	if e.ResourceUsage == nil {
		return nil, nil
	}
	var usage ResourceUsage
	err := json.Unmarshal(e.ResourceUsage, &usage)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}